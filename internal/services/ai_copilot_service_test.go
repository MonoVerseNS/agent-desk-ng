package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/i18nx"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupAICopilotTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func TestAICopilotService_BuildConversationTranscript(t *testing.T) {
	recalledTime := time.Now()
	messages := []models.Message{
		{
			ID:          1,
			SenderType:  enums.IMSenderTypeCustomer,
			MessageType: enums.IMMessageTypeText,
			Content:     "Hello, I need help with my account",
		},
		{
			ID:          2,
			SenderType:  enums.IMSenderTypeAgent,
			MessageType: enums.IMMessageTypeText,
			Content:     "Hi there! What issue are you experiencing?",
		},
		{
			ID:          3,
			SenderType:  enums.IMSenderTypeCustomer,
			MessageType: enums.IMMessageTypeHTML,
			Content:     "<p>I see an <b>error code 500</b> on checkout.</p>",
		},
		{
			ID:          4,
			SenderType:  enums.IMSenderTypeCustomer,
			MessageType: enums.IMMessageTypeImage,
			Content:     "screenshot.png",
		},
		{
			ID:          5,
			SenderType:  enums.IMSenderTypeCustomer,
			MessageType: enums.IMMessageTypeAttachment,
			Content:     "log.txt",
		},
		{
			ID:          6,
			SenderType:  enums.IMSenderTypeAgent,
			MessageType: enums.IMMessageTypeText,
			Content:     "This was recalled",
			RecalledAt:  &recalledTime,
			SendStatus:  enums.IMMessageStatusRecalled,
		},
		{
			ID:          7,
			SenderType:  enums.IMSenderTypeCustomer,
			MessageType: enums.IMMessageTypeText,
			Content:     "   ", // empty content should be ignored
		},
		{
			ID:          8,
			SenderType:  enums.IMSenderTypeAI,
			MessageType: enums.IMMessageTypeText,
			Content:     "Our system is checking your account now.",
		},
	}

	svc := newAICopilotService()
	transcript := svc.buildConversationTranscript(messages)

	expectedLines := []string{
		"Customer: Hello, I need help with my account",
		"Agent: Hi there! What issue are you experiencing?",
		"Customer: I see an error code 500 on checkout.",
		"Customer: [Image: screenshot.png]",
		"Customer: [Attachment: log.txt]",
		"Agent: Our system is checking your account now.",
	}

	for _, line := range expectedLines {
		if !strings.Contains(transcript, line) {
			t.Errorf("transcript missing expected line: %q\nFull transcript:\n%s", line, transcript)
		}
	}

	if strings.Contains(transcript, "This was recalled") {
		t.Errorf("recalled message should not appear in transcript: %s", transcript)
	}
}

func TestAICopilotService_BuildSuggestReplyPrompts_Locales(t *testing.T) {
	svc := newAICopilotService()
	transcript := "Customer: How do I change password?"

	locales := []struct {
		locale            string
		wantSystemContent string
		wantUserPrefix    string
	}{
		{
			locale:            i18nx.LocaleEnUS,
			wantSystemContent: "expert customer support co-pilot",
			wantUserPrefix:    "Conversation history:",
		},
		{
			locale:            i18nx.LocaleZhCN,
			wantSystemContent: "智能客服助手",
			wantUserPrefix:    "对话记录：",
		},
		{
			locale:            i18nx.LocaleRuRU,
			wantSystemContent: "ассистент оператора службы поддержки",
			wantUserPrefix:    "История диалога:",
		},
	}

	for _, tc := range locales {
		t.Run(tc.locale, func(t *testing.T) {
			sysPrompt, usrPrompt := svc.buildSuggestReplyPrompts(transcript, tc.locale)
			if !strings.Contains(sysPrompt, tc.wantSystemContent) {
				t.Errorf("sysPrompt for locale %s did not contain %q, got: %s", tc.locale, tc.wantSystemContent, sysPrompt)
			}
			if !strings.Contains(usrPrompt, tc.wantUserPrefix) {
				t.Errorf("usrPrompt for locale %s did not contain %q, got: %s", tc.locale, tc.wantUserPrefix, usrPrompt)
			}
			if !strings.Contains(usrPrompt, "<conversation_history>\n"+transcript+"\n</conversation_history>") {
				t.Errorf("usrPrompt for locale %s did not contain tagged transcript, got: %s", tc.locale, usrPrompt)
			}
		})
	}
}

func TestAICopilotService_BuildSummarizePrompts_Locales(t *testing.T) {
	svc := newAICopilotService()
	transcript := "Customer: My account is locked.\nAgent: Let me unlock it."

	locales := []struct {
		locale            string
		wantSystemContent string
		wantUserPrefix    string
	}{
		{
			locale:            i18nx.LocaleEnUS,
			wantSystemContent: "customer support summarizer",
			wantUserPrefix:    "Conversation history:",
		},
		{
			locale:            i18nx.LocaleZhCN,
			wantSystemContent: "客服对话总结助手",
			wantUserPrefix:    "对话记录：",
		},
		{
			locale:            i18nx.LocaleRuRU,
			wantSystemContent: "ассистент по подведению итогов",
			wantUserPrefix:    "История диалога:",
		},
	}

	for _, tc := range locales {
		t.Run(tc.locale, func(t *testing.T) {
			sysPrompt, usrPrompt := svc.buildSummarizePrompts(transcript, tc.locale)
			if !strings.Contains(sysPrompt, tc.wantSystemContent) {
				t.Errorf("sysPrompt for locale %s did not contain %q, got: %s", tc.locale, tc.wantSystemContent, sysPrompt)
			}
			if !strings.Contains(usrPrompt, tc.wantUserPrefix) {
				t.Errorf("usrPrompt for locale %s did not contain %q, got: %s", tc.locale, tc.wantUserPrefix, usrPrompt)
			}
			if !strings.Contains(usrPrompt, "<conversation_history>\n"+transcript+"\n</conversation_history>") {
				t.Errorf("usrPrompt for locale %s did not contain tagged transcript, got: %s", tc.locale, usrPrompt)
			}
		})
	}
}

func TestAICopilotService_ExtractMessageText_RuneLimit(t *testing.T) {
	svc := newAICopilotService()
	longText := strings.Repeat("a", 2500)
	msg := models.Message{
		SenderType:  enums.IMSenderTypeCustomer,
		MessageType: enums.IMMessageTypeText,
		Content:     longText,
	}
	extracted := svc.extractMessageText(msg)
	if len([]rune(extracted)) != 2000 {
		t.Fatalf("expected extracted length to be 2000 runes, got %d", len([]rune(extracted)))
	}
}

func TestAICopilotService_SuggestReply_ValidationAndErrors(t *testing.T) {
	db := setupAICopilotTestDB(t)
	ctx := context.Background()
	svc := newAICopilotService()

	// 1. Invalid conversation ID <= 0
	_, err := svc.SuggestReply(ctx, 0, "en-US")
	if err == nil {
		t.Fatal("expected error for conversationID <= 0, got nil")
	}

	// 2. Conversation not found
	_, err = svc.SuggestReply(ctx, 99999, "en-US")
	if err == nil {
		t.Fatal("expected error for non-existent conversation, got nil")
	}
	if i18nErr, ok := err.(*errorsx.I18nError); !ok || i18nErr.Key != "error.e0116" {
		t.Fatalf("expected error.e0116 for not found conversation, got: %v", err)
	}

	// 3. Conversation exists but has no messages
	conv := &models.Conversation{
		ID:      101,
		Subject: "Empty conversation",
	}
	if err := db.Create(conv).Error; err != nil {
		t.Fatalf("failed to create conv: %v", err)
	}

	_, err = svc.SuggestReply(ctx, conv.ID, "en-US")
	if err == nil {
		t.Fatal("expected error when conversation has no messages, got nil")
	}

	// 4. Conversation has only recalled messages
	recalledTime := time.Now()
	msg := &models.Message{
		ID:             1,
		ConversationID: conv.ID,
		ClientMsgID:    "msg_1",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "recalled text",
		RecalledAt:     &recalledTime,
		SendStatus:     enums.IMMessageStatusRecalled,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("failed to create msg: %v", err)
	}

	_, err = svc.SuggestReply(ctx, conv.ID, "en-US")
	if err == nil {
		t.Fatal("expected error when conversation has only recalled messages, got nil")
	}

	// 5. LLM chat failure
	validMsg := &models.Message{
		ID:             2,
		ConversationID: conv.ID,
		ClientMsgID:    "msg_2",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "Can you help me reset my password?",
	}
	if err := db.Create(validMsg).Error; err != nil {
		t.Fatalf("failed to create valid msg: %v", err)
	}

	svc.chatFn = func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
		return nil, errors.New("upstream llm timeout")
	}

	_, err = svc.SuggestReply(ctx, conv.ID, "en-US")
	if err == nil || !strings.Contains(err.Error(), "upstream llm timeout") {
		t.Fatalf("expected llm error to propagate, got: %v", err)
	}
}

func TestAICopilotService_SuggestReply_Success(t *testing.T) {
	db := setupAICopilotTestDB(t)
	ctx := context.Background()

	conv := &models.Conversation{
		ID:      201,
		Subject: "Password Reset",
	}
	if err := db.Create(conv).Error; err != nil {
		t.Fatalf("create conversation failed: %v", err)
	}

	msg := &models.Message{
		ID:             10,
		ConversationID: conv.ID,
		ClientMsgID:    "msg_10",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "I forgot my password, can you help?",
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("create message failed: %v", err)
	}

	var capturedSys, capturedUser string
	svc := newAICopilotService()
	svc.chatFn = func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
		capturedSys = systemPrompt
		capturedUser = userPrompt
		return &ai.ChatCompletionResult{
			Content:   "To reset your password, please click the 'Forgot Password' link on the login page.",
			ModelName: "gpt-4o",
		}, nil
	}

	suggestion, err := svc.SuggestReply(ctx, conv.ID, "en-US")
	if err != nil {
		t.Fatalf("SuggestReply failed: %v", err)
	}

	expectedSuggestion := "To reset your password, please click the 'Forgot Password' link on the login page."
	if suggestion != expectedSuggestion {
		t.Fatalf("expected suggestion %q, got %q", expectedSuggestion, suggestion)
	}
	if !strings.Contains(capturedSys, "expert customer support co-pilot") {
		t.Errorf("captured system prompt missing expected instructions: %s", capturedSys)
	}
	if !strings.Contains(capturedUser, "Customer: I forgot my password, can you help?") {
		t.Errorf("captured user prompt missing message transcript: %s", capturedUser)
	}
}

func TestAICopilotService_SummarizeConversation_Success(t *testing.T) {
	db := setupAICopilotTestDB(t)
	ctx := context.Background()

	conv := &models.Conversation{
		ID:      301,
		Subject: "Billing Inquiry",
	}
	if err := db.Create(conv).Error; err != nil {
		t.Fatalf("create conversation failed: %v", err)
	}

	messages := []models.Message{
		{
			ID:             30,
			ConversationID: conv.ID,
			ClientMsgID:    "msg_30",
			SenderType:     enums.IMSenderTypeCustomer,
			MessageType:    enums.IMMessageTypeText,
			Content:        "Why was I charged twice for subscription?",
		},
		{
			ID:             31,
			ConversationID: conv.ID,
			ClientMsgID:    "msg_31",
			SenderType:     enums.IMSenderTypeAgent,
			MessageType:    enums.IMMessageTypeText,
			Content:        "I see the duplicate transaction and have initiated a full refund for the second charge.",
		},
	}
	for _, m := range messages {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("create message failed: %v", err)
		}
	}

	var capturedSys, capturedUser string
	svc := newAICopilotService()
	svc.chatFn = func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
		capturedSys = systemPrompt
		capturedUser = userPrompt
		return &ai.ChatCompletionResult{
			Content:   "1. Main Issue: Duplicate subscription charge.\n2. Actions taken: Initiated full refund for duplicate charge.\n3. Current Status: Resolved.",
			ModelName: "gpt-4o",
		}, nil
	}

	summary, err := svc.SummarizeConversation(ctx, conv.ID, "en-US")
	if err != nil {
		t.Fatalf("SummarizeConversation failed: %v", err)
	}

	if !strings.Contains(summary, "Duplicate subscription charge") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if !strings.Contains(capturedSys, "customer support summarizer") {
		t.Errorf("captured system prompt missing summarizer instructions: %s", capturedSys)
	}
	if !strings.Contains(capturedUser, "Customer: Why was I charged twice for subscription?") {
		t.Errorf("captured user prompt missing transcript: %s", capturedUser)
	}
}
