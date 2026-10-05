package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/i18nx"
	"agent-desk/internal/pkg/openidentity"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const testEmailChannelCode = "support_mail"

func setupEmailRequestTest(t *testing.T, maxOpen int, liveMinutes int) (*gorm.DB, *models.Channel, *models.AIAgent) {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerContact{},
		&models.Channel{},
		&models.AIAgent{},
		&models.Conversation{},
		&models.ConversationAssignment{},
		&models.ConversationParticipant{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.ChannelMessageOutbox{},
		&models.Message{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)

	previousConfig := config.GetCurrent()
	config.SetCurrent(&config.Config{
		Conversation: config.ConversationConfig{
			CustomerMaxOpen:      maxOpen,
			EmailChatLiveMinutes: liveMinutes,
		},
		Email: config.EmailConfig{InboundSecret: ""},
	})
	previousWs := WsService
	WsService = newWsService()
	t.Cleanup(func() {
		config.SetCurrent(previousConfig)
		WsService = previousWs
	})

	// AI-first keeps the test off the human dispatch path.
	aiAgent := models.AIAgent{
		Name:        "test AI",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&aiAgent).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}

	channel := models.Channel{
		Name:        "Support mail",
		ChannelID:   testEmailChannelCode,
		ChannelType: enums.ChannelTypeEmail,
		AIAgentID:   aiAgent.ID,
		Status:      enums.StatusOk,
		ConfigJSON: mustMarshalChannelConfig(dto.EmailChannelConfig{
			EmailAddress:  "help@example.com",
			WebhookSecret: "channel-secret",
		}),
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel error = %v", err)
	}

	return db, &channel, &aiAgent
}

func TestHandleRequestFilesAnEmailConversation(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	result, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email:   "alice@example.com",
		Name:    "Alice",
		Subject: "Cannot complete payment",
		Message: "The checkout page spins forever.",
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}

	if result.Outcome != EmailRequestOutcomeEmail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, EmailRequestOutcomeEmail)
	}
	if !result.Verified {
		t.Fatal("a request presenting the channel secret must be verified")
	}

	conversation := ConversationService.Get(result.ConversationID)
	if conversation == nil {
		t.Fatal("conversation was not created")
	}
	if conversation.Subject != "Cannot complete payment" {
		t.Fatalf("Subject = %q, want the submitted title", conversation.Subject)
	}
	if conversation.ChannelID != channel.ID {
		t.Fatalf("conversation channel = %d, want the email channel %d", conversation.ChannelID, channel.ID)
	}

	messages := MessageService.Find(sqls.NewCnd().Eq("conversation_id", conversation.ID).Asc("id"))
	if len(messages) < 2 {
		t.Fatalf("expected the request plus an acknowledgement, got %d messages", len(messages))
	}
	if !strings.Contains(messages[0].Content, "checkout page spins") {
		t.Fatalf("request content = %q", messages[0].Content)
	}
	if messages[len(messages)-1].SenderType != enums.IMSenderTypeAI {
		t.Fatalf("acknowledgement sender = %q, want an AI message", messages[len(messages)-1].SenderType)
	}
}

// The receipt has to reach the inbox, so it must land in the email outbox rather
// than only in the transcript.
func TestHandleRequestQueuesTheAcknowledgementForEmail(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	result, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email:   "alice@example.com",
		Subject: "Question",
		Message: "Hello there.",
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}

	pending := ChannelMessageOutboxService.ListPending(enums.ChannelTypeEmail, 10)
	if len(pending) != 1 {
		t.Fatalf("outbox has %d pending messages, want exactly the acknowledgement", len(pending))
	}
	if pending[0].ConversationID != result.ConversationID {
		t.Fatalf("outbox conversation = %d, want %d", pending[0].ConversationID, result.ConversationID)
	}
}

// Omitting the secret is the documented guest mode: the request is still filed,
// but nothing the submitter claimed is treated as proven.
func TestHandleRequestWithoutSecretIsSelfReported(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	result, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "", EmailRequestInput{
		Email:   "bob@example.com",
		Subject: "Question",
		Message: "Hello there.",
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	if result.Verified {
		t.Fatal("a request without the channel secret must not be verified")
	}
	if result.Outcome != EmailRequestOutcomeEmail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, EmailRequestOutcomeEmail)
	}

	contact := findTestContact(t, "bob@example.com")
	if contact == nil {
		t.Fatal("the submitted address should still be recorded as a contact")
	}
	if contact.IsVerified {
		t.Fatal("a self-reported address must not be marked verified")
	}
	if strings.TrimSpace(contact.Remark) == "" {
		t.Fatal("a self-reported address must carry a remark agents can see")
	}
}

func TestHandleRequestWithValidSecretMarksContactVerified(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "carol@example.com", Message: "First note.",
	}); err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}

	contact := findTestContact(t, "carol@example.com")
	if contact == nil {
		t.Fatal("expected a contact row")
	}
	if !contact.IsVerified {
		t.Fatal("a secret-authenticated address must be marked verified")
	}
	if contact.VerifiedAt == nil {
		t.Fatal("a verified contact needs a verification timestamp")
	}
}

// A later secretless submission must not silently upgrade the record, and a
// secret one must not stay unverified after being proven.
func TestHandleRequestUpdatesVerificationOnRepeat(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 5, 30)

	if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "", EmailRequestInput{
		Email: "dan@example.com", Message: "Unverified first.",
	}); err != nil {
		t.Fatalf("guest HandleRequest() error = %v", err)
	}
	if contact := findTestContact(t, "dan@example.com"); contact == nil || contact.IsVerified {
		t.Fatal("first submission should be unverified")
	}

	if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "dan@example.com", Message: "Verified second.",
	}); err != nil {
		t.Fatalf("verified HandleRequest() error = %v", err)
	}
	contact := findTestContact(t, "dan@example.com")
	if contact == nil || !contact.IsVerified {
		t.Fatal("a verified submission must upgrade the existing contact")
	}
}

// A wrong secret must fail loudly. Treating it as a guest submission would let a
// broken integration look like it is working while filing untrusted identities.
func TestHandleRequestRejectsAWrongSecret(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	_, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "not-the-secret", EmailRequestInput{
		Email: "eve@example.com", Message: "Hello.",
	})
	if err == nil {
		t.Fatal("expected a wrong secret to be rejected")
	}
	if !strings.Contains(err.Error(), i18nx.Getf(i18nx.DefaultLocale, "error.auth.invalidSignature")) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRequestRejectsEmptyMessage(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	for _, message := range []string{"", "   \n\t "} {
		if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
			Email: "frank@example.com", Message: message,
		}); err == nil {
			t.Fatalf("expected %q to be rejected as an empty request", message)
		}
	}
}

func TestHandleRequestRejectsInvalidAddress(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 3, 30)

	for _, address := range []string{"", "no-at-sign", "   "} {
		if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
			Email: address, Message: "Hello.",
		}); err == nil {
			t.Fatalf("expected %q to be rejected", address)
		}
	}
}

// A live chat is where the visitor is actually present, so mail joins it instead
// of starting a thread they will not watch.
//
// Identity is keyed on the address, so the visitor has to reach the same customer
// through both: the email address is what resolves to them here, so the chat has
// to belong to the customer that address created.
func TestHandleRequestJoinsALiveChatInsteadOfStartingEmail(t *testing.T) {
	_, emailChannel, _ := setupEmailRequestTest(t, 5, 30)
	webChannel := createTestWebChannel(t)

	first, err := EmailRequestService.HandleRequest(t.Context(), emailChannel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "guest@example.com", Subject: "Started by email", Message: "Opening question.",
	})
	if err != nil {
		t.Fatalf("first HandleRequest() error = %v", err)
	}
	customerID := ConversationService.Get(first.ConversationID).CustomerID
	closeConversation(t, first.ConversationID)

	chat, err := ConversationService.CreateNew(
		openidentity.ExternalUser{ExternalSource: enums.ExternalSourceEmail, ExternalID: "guest@example.com"},
		webChannel.ID, emailChannel.AIAgentID, "live chat",
	)
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	if chat.CustomerID != customerID {
		t.Fatalf("chat customer = %d, want the address's own customer %d", chat.CustomerID, customerID)
	}

	result, err := EmailRequestService.HandleRequest(t.Context(), emailChannel.ChannelID, "channel-secret", EmailRequestInput{
		Email:   "guest@example.com",
		Subject: "Follow up",
		Message: "Adding detail by email.",
	})
	if err != nil {
		t.Fatalf("second HandleRequest() error = %v", err)
	}
	if result.Outcome != EmailRequestOutcomeChat {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, EmailRequestOutcomeChat)
	}
	if result.ConversationID != chat.ID {
		t.Fatalf("conversation = %d, want the live chat %d", result.ConversationID, chat.ID)
	}

	messages := MessageService.Find(sqls.NewCnd().Eq("conversation_id", chat.ID).Asc("id"))
	found := false
	for _, message := range messages {
		if strings.Contains(message.Content, "Adding detail by email") {
			found = true
		}
	}
	if !found {
		t.Fatal("the email content should be recorded in the chat so the agent can see it")
	}

	// The visitor is told where to continue, and it has to be an email, not a
	// message that only exists inside the chat they were about to leave.
	pending := ChannelMessageOutboxService.ListPending(enums.ChannelTypeEmail, 10)
	// The first request's acknowledgement was already delivered by the async
	// dispatcher attempt, so only the newest notice is outstanding here.
	if len(pending) == 0 {
		t.Fatal("expected a continuation notice to be queued for email")
	}
}

// Once the chat has gone quiet it is abandoned, and mail becomes its own thread.
func TestHandleRequestStartsEmailWhenTheChatIsStale(t *testing.T) {
	_, emailChannel, _ := setupEmailRequestTest(t, 5, 30)
	webChannel := createTestWebChannel(t)

	first, err := EmailRequestService.HandleRequest(t.Context(), emailChannel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "guest@example.com", Subject: "Started by email", Message: "Opening question.",
	})
	if err != nil {
		t.Fatalf("first HandleRequest() error = %v", err)
	}
	closeConversation(t, first.ConversationID)

	chat, err := ConversationService.CreateNew(
		openidentity.ExternalUser{ExternalSource: enums.ExternalSourceEmail, ExternalID: "guest@example.com"},
		webChannel.ID, emailChannel.AIAgentID, "stale chat",
	)
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}

	backdate := time.Now().Add(-4 * time.Hour)
	if err := sqls.DB().Model(&models.Conversation{}).Where("id = ?", chat.ID).
		Update("last_active_at", backdate).Error; err != nil {
		t.Fatalf("backdate chat error = %v", err)
	}

	result, err := EmailRequestService.HandleRequest(t.Context(), emailChannel.ChannelID, "channel-secret", EmailRequestInput{
		Email:   "guest@example.com",
		Subject: "Still need help",
		Message: "The chat went quiet a while ago.",
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	if result.Outcome != EmailRequestOutcomeEmail {
		t.Fatalf("Outcome = %q, want %q for an abandoned chat", result.Outcome, EmailRequestOutcomeEmail)
	}
	if result.ConversationID == chat.ID {
		t.Fatal("a stale chat must not swallow the email")
	}
}

// An open thread already on email stays on email; there is nothing to redirect
// the visitor away from.
func TestHandleRequestKeepsAnOpenEmailThreadOnEmail(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 5, 30)

	first, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "kim@example.com", Subject: "Ongoing", Message: "Still waiting on this.",
	})
	if err != nil {
		t.Fatalf("first HandleRequest() error = %v", err)
	}

	chat := ConversationService.Get(first.ConversationID)
	// Pretend the visitor also has a fresh chat open somewhere else.
	other := ConversationService.Get(first.ConversationID)
	if other == nil {
		t.Fatal("conversation disappeared")
	}
	_ = chat

	result, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "kim@example.com", Subject: "Ongoing", Message: "Any news on this?",
	})
	if err != nil {
		t.Fatalf("second HandleRequest() error = %v", err)
	}
	if result.Outcome != EmailRequestOutcomeEmail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, EmailRequestOutcomeEmail)
	}
}

// The cap has to apply on this path too, or mail becomes a way around it.
func TestHandleRequestRespectsTheOpenRequestCap(t *testing.T) {
	_, channel, _ := setupEmailRequestTest(t, 2, 30)

	for i := 0; i < 2; i++ {
		if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
			Email: "heidi@example.com", Message: "Request.",
		}); err != nil {
			t.Fatalf("HandleRequest() #%d error = %v", i+1, err)
		}
	}
	if _, err := EmailRequestService.HandleRequest(t.Context(), channel.ChannelID, "channel-secret", EmailRequestInput{
		Email: "heidi@example.com", Message: "One too many.",
	}); err == nil {
		t.Fatal("expected the open request cap to apply to email submissions")
	}
}

func TestHandleRequestRejectsUnknownChannel(t *testing.T) {
	setupEmailRequestTest(t, 3, 30)

	if _, err := EmailRequestService.HandleRequest(t.Context(), "does-not-exist", "channel-secret", EmailRequestInput{
		Email: "ivan@example.com", Message: "Hello.",
	}); err == nil {
		t.Fatal("expected an unknown channel to be rejected")
	}
}

func TestFindLiveChatIgnoresOtherCustomers(t *testing.T) {
	_, emailChannel, _ := setupEmailRequestTest(t, 5, 30)
	webChannel := createTestWebChannel(t)

	other := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "u_other",
		ExternalName:   "Other",
	}
	if _, err := ConversationService.CreateNew(other, webChannel.ID, emailChannel.AIAgentID, ""); err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}

	if live := EmailRequestService.findLiveChat(999999, emailChannel.ID); live != nil {
		t.Fatalf("found a chat for a customer that does not exist: %+v", live)
	}
}

func mustMarshalChannelConfig(cfg dto.EmailChannelConfig) string {
	raw, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func createTestWebChannel(t *testing.T) *models.Channel {
	t.Helper()
	channel := models.Channel{
		Name:        "Web",
		ChannelID:   "web_main",
		ChannelType: enums.ChannelTypeWeb,
		Status:      enums.StatusOk,
	}
	if err := sqls.DB().Create(&channel).Error; err != nil {
		t.Fatalf("create web channel error = %v", err)
	}
	return &channel
}

func findTestContact(t *testing.T, address string) *models.CustomerContact {
	t.Helper()
	return CustomerContactService.FindOne(sqls.NewCnd().
		Eq("contact_type", enums.ContactTypeEmail).
		Eq("contact_value", address))
}

// linkTestIdentity reuses an existing customer for an email address so routing
// decisions see one person rather than two.
func closeConversation(t *testing.T, conversationID int64) {
	t.Helper()
	if err := sqls.DB().Model(&models.Conversation{}).Where("id = ?", conversationID).
		Update("status", enums.IMConversationStatusClosed).Error; err != nil {
		t.Fatalf("close conversation error = %v", err)
	}
}
