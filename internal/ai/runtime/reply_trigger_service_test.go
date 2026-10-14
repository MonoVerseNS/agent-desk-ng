package runtime

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	applicationruntime "agent-desk/internal/ai/application/runtime"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	svc "agent-desk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type mockReplyExecutor struct {
	runFunc    func(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error)
	resumeFunc func(ctx context.Context, input runtimeReplyResumeInput) (*applicationruntime.RunResult, error)
}

func (m *mockReplyExecutor) Run(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, input)
	}
	return nil, nil
}

func (m *mockReplyExecutor) ResumePendingInterrupt(ctx context.Context, input runtimeReplyResumeInput) (*applicationruntime.RunResult, error) {
	if m.resumeFunc != nil {
		return m.resumeFunc(ctx, input)
	}
	return nil, nil
}

func setupReplyTriggerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "reply_trigger_test_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.AIAgent{},
		&models.AgentTeam{},
		&models.AgentTeamSchedule{},
		&models.AgentProfile{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.ConversationAssignment{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.ConversationInterrupt{},
		&models.Message{},
		&models.Channel{},
		&models.ChannelMessageOutbox{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createReplyTriggerTestAIAgent(t *testing.T, db *gorm.DB, autoHandoff bool, handoffMsg string) models.AIAgent {
	t.Helper()
	now := time.Now()
	item := models.AIAgent{
		Name:                 "TriggerAI",
		Status:               enums.StatusOk,
		ServiceMode:          enums.IMConversationServiceModeAIFirst,
		AutoHandoffOnFailure: autoHandoff,
		HandoffMessage:       handoffMsg,
		TeamIDs:              "1",
		RolloutPercent:       100,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ai agent: %v", err)
	}
	if !autoHandoff {
		if err := db.Model(&item).Update("auto_handoff_on_failure", false).Error; err != nil {
			t.Fatalf("update auto_handoff_on_failure: %v", err)
		}
		item.AutoHandoffOnFailure = false
	}
	return item
}

func createReplyTriggerTestTeam(t *testing.T, db *gorm.DB, teamID int64) {
	t.Helper()
	if err := db.Create(&models.AgentTeam{ID: teamID, Name: "支持组", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.AgentTeamSchedule{
		TeamID:  teamID,
		StartAt: now.Add(-time.Hour),
		EndAt:   now.Add(time.Hour),
		Status:  enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create schedule: %v", err)
	}
}

func createReplyTriggerTestConversation(t *testing.T, db *gorm.DB, aiAgentID int64) models.Conversation {
	t.Helper()
	now := time.Now()
	item := models.Conversation{
		CustomerID:   1,
		ChannelID:    1,
		AIAgentID:    aiAgentID,
		Status:       enums.IMConversationStatusAIServing,
		ServiceMode:  enums.IMConversationServiceModeAIFirst,
		LastActiveAt: now,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return item
}

func createCustomerTestMessage(t *testing.T, db *gorm.DB, conversationID int64, content string) models.Message {
	t.Helper()
	now := time.Now()
	item := models.Message{
		ConversationID: conversationID,
		SenderType:     enums.IMSenderTypeCustomer,
		SenderID:       1,
		Content:        content,
		RequestID:      "req-trigger-test",
		SendStatus:     enums.IMMessageStatusSent,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	return item
}

func TestAIReplyService_AutoHandoffOnExecutionFailure(t *testing.T) {
	db := setupReplyTriggerTestDB(t)
	createReplyTriggerTestTeam(t, db, 1)
	aiAgent := createReplyTriggerTestAIAgent(t, db, true, "AI故障，正在为您转接人工客服...")
	conversation := createReplyTriggerTestConversation(t, db, aiAgent.ID)
	message := createCustomerTestMessage(t, db, conversation.ID, "帮助我")

	mock := &mockReplyExecutor{
		runFunc: func(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error) {
			return nil, fmt.Errorf("llm connection timeout")
		},
	}

	service := newAIReplyServiceWithExecutor(mock)
	err := service.TriggerReply(context.Background(), conversation, message, aiAgent)
	if err == nil || !strings.Contains(err.Error(), "llm connection timeout") {
		t.Fatalf("expected llm timeout error, got %v", err)
	}

	currentConv := svc.ConversationService.Get(conversation.ID)
	if currentConv.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected conversation status pending after handoff, got %v", currentConv.Status)
	}
	if currentConv.HandoffAt == nil {
		t.Fatalf("expected handoffAt to be set")
	}
	if !strings.Contains(currentConv.HandoffReason, "AI execution failure: llm connection timeout") {
		t.Fatalf("unexpected handoff reason: %q", currentConv.HandoffReason)
	}

	sentMsg := svc.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if sentMsg == nil {
		t.Fatalf("expected customer notice message to be sent")
	}
	if sentMsg.Content != "AI故障，正在为您转接人工客服..." {
		t.Fatalf("unexpected notice message content: %q", sentMsg.Content)
	}
}

func TestAIReplyService_AutoHandoffDisabledOnExecutionFailure(t *testing.T) {
	db := setupReplyTriggerTestDB(t)
	createReplyTriggerTestTeam(t, db, 1)
	aiAgent := createReplyTriggerTestAIAgent(t, db, false, "")
	conversation := createReplyTriggerTestConversation(t, db, aiAgent.ID)
	message := createCustomerTestMessage(t, db, conversation.ID, "帮助我")

	mock := &mockReplyExecutor{
		runFunc: func(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error) {
			return nil, fmt.Errorf("llm connection timeout")
		},
	}

	service := newAIReplyServiceWithExecutor(mock)
	err := service.TriggerReply(context.Background(), conversation, message, aiAgent)
	if err == nil || !strings.Contains(err.Error(), "llm connection timeout") {
		t.Fatalf("expected llm timeout error, got %v", err)
	}

	currentConv := svc.ConversationService.Get(conversation.ID)
	if currentConv.Status != enums.IMConversationStatusAIServing {
		t.Fatalf("expected conversation to stay AI serving, got %v", currentConv.Status)
	}
	if currentConv.HandoffAt != nil {
		t.Fatalf("expected handoffAt to stay nil")
	}
}

func TestAIReplyService_AutoHandoffOnEmptyReply(t *testing.T) {
	db := setupReplyTriggerTestDB(t)
	createReplyTriggerTestTeam(t, db, 1)
	aiAgent := createReplyTriggerTestAIAgent(t, db, true, "AI无法解答，已为您转接人工客服")
	conversation := createReplyTriggerTestConversation(t, db, aiAgent.ID)
	message := createCustomerTestMessage(t, db, conversation.ID, "复杂未知问题")

	mock := &mockReplyExecutor{
		runFunc: func(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error) {
			return &applicationruntime.RunResult{
				ReplyText: "   ",
			}, nil
		},
	}

	service := newAIReplyServiceWithExecutor(mock)
	err := service.TriggerReply(context.Background(), conversation, message, aiAgent)
	if err != nil {
		t.Fatalf("expected nil error on successful empty reply auto-handoff, got %v", err)
	}

	currentConv := svc.ConversationService.Get(conversation.ID)
	if currentConv.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected conversation status pending after handoff, got %v", currentConv.Status)
	}
	if currentConv.HandoffAt == nil {
		t.Fatalf("expected handoffAt to be set")
	}
	if !strings.Contains(currentConv.HandoffReason, "AI generated empty reply or fallback unavailable") {
		t.Fatalf("unexpected handoff reason: %q", currentConv.HandoffReason)
	}

	sentMsg := svc.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if sentMsg == nil {
		t.Fatalf("expected customer notice message to be sent")
	}
	if sentMsg.Content != "AI无法解答，已为您转接人工客服" {
		t.Fatalf("unexpected notice message content: %q", sentMsg.Content)
	}
}

func TestAIReplyService_AutoHandoffDisabledOnEmptyReply(t *testing.T) {
	db := setupReplyTriggerTestDB(t)
	createReplyTriggerTestTeam(t, db, 1)
	aiAgent := createReplyTriggerTestAIAgent(t, db, false, "")
	conversation := createReplyTriggerTestConversation(t, db, aiAgent.ID)
	message := createCustomerTestMessage(t, db, conversation.ID, "复杂未知问题")

	mock := &mockReplyExecutor{
		runFunc: func(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error) {
			return &applicationruntime.RunResult{
				ReplyText: "",
			}, nil
		},
	}

	service := newAIReplyServiceWithExecutor(mock)
	err := service.TriggerReply(context.Background(), conversation, message, aiAgent)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	currentConv := svc.ConversationService.Get(conversation.ID)
	if currentConv.Status != enums.IMConversationStatusAIServing {
		t.Fatalf("expected conversation to stay AI serving, got %v", currentConv.Status)
	}
	if currentConv.HandoffAt != nil {
		t.Fatalf("expected handoffAt to stay nil")
	}
}

func TestAIReplyService_NormalReplyCommitted(t *testing.T) {
	db := setupReplyTriggerTestDB(t)
	aiAgent := createReplyTriggerTestAIAgent(t, db, true, "")
	conversation := createReplyTriggerTestConversation(t, db, aiAgent.ID)
	message := createCustomerTestMessage(t, db, conversation.ID, "你好")

	mock := &mockReplyExecutor{
		runFunc: func(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error) {
			return &applicationruntime.RunResult{
				ReplyText: "您好，有什么可以帮您？",
			}, nil
		},
	}

	service := newAIReplyServiceWithExecutor(mock)
	err := service.TriggerReply(context.Background(), conversation, message, aiAgent)
	if err != nil {
		t.Fatalf("expected nil error on normal reply, got %v", err)
	}

	currentConv := svc.ConversationService.Get(conversation.ID)
	if currentConv.Status != enums.IMConversationStatusAIServing {
		t.Fatalf("expected conversation to stay AI serving, got %v", currentConv.Status)
	}
	if currentConv.HandoffAt != nil {
		t.Fatalf("expected handoffAt to stay nil")
	}

	sentMsg := svc.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if sentMsg == nil {
		t.Fatalf("expected reply message to be sent")
	}
	if sentMsg.Content != "您好，有什么可以帮您？" {
		t.Fatalf("unexpected reply content: %q", sentMsg.Content)
	}
}
