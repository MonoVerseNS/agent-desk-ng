package services

import (
	"strings"
	"testing"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/openidentity"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// The whole point of CreateNew is that it does not behave like Create: it must
// start a second thread instead of handing back the unfinished one.
func TestCreateNewDoesNotResumeUnfinishedConversation(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 3)

	first, err := svc.CreateNew(external, 1, aiAgentID, "Проблема с оплатой")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	second, err := svc.CreateNew(external, 1, aiAgentID, "Не приходит письмо")
	if err != nil {
		t.Fatalf("second CreateNew() error = %v", err)
	}

	if first.ID == second.ID {
		t.Fatalf("CreateNew reused conversation %d instead of starting a new one", first.ID)
	}
	if first.Subject != "Проблема с оплатой" || second.Subject != "Не приходит письмо" {
		t.Fatalf("subjects = %q / %q", first.Subject, second.Subject)
	}

	// The resume path must still rejoin the newest one, so a page reload lands
	// the visitor back in their live thread.
	resumed, err := svc.Create(external, 1, aiAgentID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resumed.ID != second.ID {
		t.Fatalf("Create() resumed conversation %d, want the newest %d", resumed.ID, second.ID)
	}
}

func TestCreateNewStoresANormalizedSubject(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 3)

	item, err := svc.CreateNew(external, 1, aiAgentID, "  Проблема   с\nоплатой  ")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	if item.Subject != "Проблема с оплатой" {
		t.Fatalf("Subject = %q, want collapsed single spaces", item.Subject)
	}
}

// An empty subject stays empty rather than becoming a blank-looking row, so the
// frontend can substitute its own fallback text.
func TestCreateNewKeepsAnEmptySubjectEmpty(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 3)

	item, err := svc.CreateNew(external, 1, aiAgentID, "   \n\t ")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	if item.Subject != "" {
		t.Fatalf("Subject = %q, want empty", item.Subject)
	}
}

func TestCreateNewEnforcesTheOpenConversationCap(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 2)

	for i := 0; i < 2; i++ {
		if _, err := svc.CreateNew(external, 1, aiAgentID, ""); err != nil {
			t.Fatalf("CreateNew() #%d error = %v", i+1, err)
		}
	}

	_, err := svc.CreateNew(external, 1, aiAgentID, "")
	if err == nil {
		t.Fatal("expected the third open conversation to be refused")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error %q should state the cap", err)
	}

	// Closing one frees a slot: the cap protects the queue, it is not a lifetime
	// limit on how many requests a visitor may ever file.
	if err := svc.CloseCustomerConversation(firstCustomerConversationID(t, svc, external), external); err != nil {
		t.Fatalf("CloseCustomerConversation() error = %v", err)
	}
	if _, err := svc.CreateNew(external, 1, aiAgentID, ""); err != nil {
		t.Fatalf("CreateNew() after closing one error = %v", err)
	}
}

func TestCreateNewHonoursADisabledCap(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, -1)

	for i := 0; i < 6; i++ {
		if _, err := svc.CreateNew(external, 1, aiAgentID, ""); err != nil {
			t.Fatalf("CreateNew() #%d error = %v with the cap disabled", i+1, err)
		}
	}
}

// A second visitor must never see the first visitor's requests.
func TestListCustomerConversationsIsScopedToTheVisitor(t *testing.T) {
	svc, alice, aiAgentID := setupCustomerConversationTest(t, 5)
	bob := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "u_bob",
		ExternalName:   "Bob",
	}

	if _, err := svc.CreateNew(alice, 1, aiAgentID, "Alice request"); err != nil {
		t.Fatalf("CreateNew(alice) error = %v", err)
	}
	if _, err := svc.CreateNew(alice, 1, aiAgentID, "Alice second"); err != nil {
		t.Fatalf("CreateNew(alice) error = %v", err)
	}
	bobItem, err := svc.CreateNew(bob, 1, aiAgentID, "Bob request")
	if err != nil {
		t.Fatalf("CreateNew(bob) error = %v", err)
	}

	aliceList, paging, err := svc.ListCustomerConversations(alice, sqls.NewCnd().Page(1, 10))
	if err != nil {
		t.Fatalf("ListCustomerConversations(alice) error = %v", err)
	}
	if len(aliceList) != 2 {
		t.Fatalf("alice sees %d conversations, want 2", len(aliceList))
	}
	if paging == nil || paging.Total != 2 {
		t.Fatalf("unexpected paging for alice: %+v", paging)
	}

	bobList, _, err := svc.ListCustomerConversations(bob, sqls.NewCnd().Page(1, 10))
	if err != nil {
		t.Fatalf("ListCustomerConversations(bob) error = %v", err)
	}
	if len(bobList) != 1 || bobList[0].ID != bobItem.ID {
		t.Fatalf("bob sees %+v, want only his own", bobList)
	}
}

// A visitor who has never talked to us has no identity row. That is an empty
// list, not an error and not somebody else's data.
func TestListCustomerConversationsIsEmptyForAnUnknownVisitor(t *testing.T) {
	svc, _, _ := setupCustomerConversationTest(t, 3)

	unknown := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     "never_seen",
	}
	list, paging, err := svc.ListCustomerConversations(unknown, sqls.NewCnd().Page(1, 10))
	if err != nil {
		t.Fatalf("ListCustomerConversations() error = %v", err)
	}
	if list == nil {
		t.Fatal("list must be an empty slice, not nil, so the client gets [] instead of null")
	}
	if len(list) != 0 {
		t.Fatalf("list = %+v, want empty", list)
	}
	if paging == nil {
		t.Fatal("paging must be present so the client can render pagination")
	}
}

// Newest activity first: the request the visitor is looking at should be on top.
func TestListCustomerConversationsOrdersByRecentActivity(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 5)

	older, err := svc.CreateNew(external, 1, aiAgentID, "older")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	newer, err := svc.CreateNew(external, 1, aiAgentID, "newer")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	if err := sqls.DB().Model(&models.Conversation{}).Where("id = ?", older.ID).
		Update("last_active_at", older.LastActiveAt.Add(-24*60*60*1e9)).Error; err != nil {
		t.Fatalf("backdate conversation error = %v", err)
	}

	list, _, err := svc.ListCustomerConversations(external, sqls.NewCnd().Page(1, 10))
	if err != nil {
		t.Fatalf("ListCustomerConversations() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
	if list[0].ID != newer.ID {
		t.Fatalf("first row = %d, want the most recently active %d", list[0].ID, newer.ID)
	}
}

func TestListCustomerConversationsRespectsPagination(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 5)

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateNew(external, 1, aiAgentID, ""); err != nil {
			t.Fatalf("CreateNew() #%d error = %v", i+1, err)
		}
	}

	firstPage, paging, err := svc.ListCustomerConversations(external, sqls.NewCnd().Page(1, 2))
	if err != nil {
		t.Fatalf("ListCustomerConversations() error = %v", err)
	}
	if len(firstPage) != 2 {
		t.Fatalf("first page length = %d, want 2", len(firstPage))
	}
	if paging.Total != 3 {
		t.Fatalf("paging.Total = %d, want 3", paging.Total)
	}

	secondPage, _, err := svc.ListCustomerConversations(external, sqls.NewCnd().Page(2, 2))
	if err != nil {
		t.Fatalf("ListCustomerConversations() page 2 error = %v", err)
	}
	if len(secondPage) != 1 {
		t.Fatalf("second page length = %d, want 1", len(secondPage))
	}
	for _, item := range firstPage {
		for _, other := range secondPage {
			if item.ID == other.ID {
				t.Fatalf("conversation %d appeared on both pages", item.ID)
			}
		}
	}
}

// The cap must not block the resume path: reloading a page with three open
// conversations has to rejoin one, not fail.
func TestCreateStillResumesWhenTheCapIsReached(t *testing.T) {
	svc, external, aiAgentID := setupCustomerConversationTest(t, 1)

	only, err := svc.CreateNew(external, 1, aiAgentID, "only")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	resumed, err := svc.Create(external, 1, aiAgentID)
	if err != nil {
		t.Fatalf("Create() at the cap error = %v", err)
	}
	if resumed.ID != only.ID {
		t.Fatalf("Create() returned %d, want %d", resumed.ID, only.ID)
	}
}

func TestNormalizeConversationSubject(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		want    string
	}{
		{name: "empty", subject: "", want: ""},
		{name: "whitespace only", subject: "  \t\n ", want: ""},
		{name: "inner runs collapse", subject: "a   b\t\tc", want: "a b c"},
		{name: "edges trimmed", subject: "  hello  ", want: "hello"},
		{name: "newlines become spaces", subject: "line one\nline two", want: "line one line two"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeConversationSubject(tt.subject); got != tt.want {
				t.Fatalf("normalizeConversationSubject(%q) = %q, want %q", tt.subject, got, tt.want)
			}
		})
	}
}

// Truncation must count runes, not bytes. A byte-wise cut can split a multi-byte
// character and produce invalid UTF-8, and it would also cut far too early:
// MySQL counts a VARCHAR(255) in characters, so 255 Cyrillic runes is 510 bytes
// yet still fits the column.
func TestNormalizeConversationSubjectTruncatesOnRuneBoundaries(t *testing.T) {
	long := strings.Repeat("я", conversationSubjectMaxLength+50)

	got := normalizeConversationSubject(long)

	if len([]rune(got)) != conversationSubjectMaxLength {
		t.Fatalf("rune length = %d, want %d", len([]rune(got)), conversationSubjectMaxLength)
	}
	if !strings.HasPrefix(long, got) {
		t.Fatal("truncation must keep a prefix of the input")
	}
}

func firstCustomerConversationID(t *testing.T, svc *conversationService, external openidentity.ExternalUser) int64 {
	t.Helper()
	list, _, err := svc.ListCustomerConversations(external, sqls.NewCnd().Page(1, 10))
	if err != nil {
		t.Fatalf("ListCustomerConversations() error = %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one conversation")
	}
	return list[0].ID
}

// setupCustomerConversationTest gives each test its own in-memory database and
// restores the process-wide singletons it touches, so tests stay independent
// even though config and the DB handle are globals.
func setupCustomerConversationTest(t *testing.T, maxOpen int) (*conversationService, openidentity.ExternalUser, int64) {
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
		&models.Channel{},
		&models.AIAgent{},
		&models.Conversation{},
		&models.ConversationAssignment{},
		&models.ConversationParticipant{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.Message{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)

	previousConfig := config.GetCurrent()
	config.SetCurrent(&config.Config{
		Conversation: config.ConversationConfig{CustomerMaxOpen: maxOpen},
	})
	previousWs := WsService
	WsService = newWsService()
	t.Cleanup(func() {
		config.SetCurrent(previousConfig)
		WsService = previousWs
	})

	// AI-first keeps the test off the human dispatch path, which needs teams,
	// schedules and agent profiles that are irrelevant here.
	aiAgent := models.AIAgent{
		Name:        "测试AI",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&aiAgent).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}

	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "u_alice",
		ExternalName:   "Alice",
	}
	return ConversationService, external, aiAgent.ID
}
