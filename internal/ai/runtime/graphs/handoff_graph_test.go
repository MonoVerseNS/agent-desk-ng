package graphs

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai/runtime/tooling"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestHandoffGraphOffHoursSendsNoticeWithoutConfirmation(t *testing.T) {
	db := setupHandoffGraphTestDB(t)
	aiAgent := createHandoffGraphAIAgent(t, db, "1")
	conversation := createHandoffGraphConversation(t, db, aiAgent.ID)

	reply, err := NewHandoffGraph(conversation, aiAgent).Run(context.Background(), `{"reason":"用户要求转人工"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var result tooling.ToolResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		t.Fatalf("expected graph tool result JSON, got %q: %v", reply, err)
	}
	if !result.Handled || !result.Terminal || result.ShouldRetry {
		t.Fatalf("unexpected graph result flags: %+v", result)
	}
	if !result.ReplySent {
		t.Fatalf("expected graph result to mark replySent, got %+v", result)
	}
	expectedMsg := services.HandoffOffHoursMessageForLocale(services.ResolveConversationLocale(conversation.ID))
	if result.Action != "off_hours_handoff" || result.ReplyText != expectedMsg {
		t.Fatalf("unexpected off-hours graph result: %+v, want %q", result, expectedMsg)
	}

	message := services.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if message == nil {
		t.Fatalf("expected off-hours notice message")
	}
	if message.Content != expectedMsg {
		t.Fatalf("expected off-hours notice %q, got %q", expectedMsg, message.Content)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current == nil {
		t.Fatalf("expected conversation")
	}
	if current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected conversation to be queued in pending status, got %d", current.Status)
	}
	if current.HandoffAt == nil {
		t.Fatalf("expected handoff_at to be set, got nil")
	}
}

func TestHandoffGraphOffHoursSendsNoticeInRussianForRussianConversation(t *testing.T) {
	db := setupHandoffGraphTestDB(t)
	aiAgent := createHandoffGraphAIAgent(t, db, "1")
	conversation := createHandoffGraphConversation(t, db, aiAgent.ID)

	now := time.Now()
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID,
		SenderType:     enums.IMSenderTypeCustomer,
		SenderID:       1,
		Content:        "Привет хочу возрат денег",
		SentAt:         &now,
	}).Error; err != nil {
		t.Fatalf("create message error = %v", err)
	}

	reply, err := NewHandoffGraph(conversation, aiAgent).Run(context.Background(), `{"reason":"перенаправляю оператору"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var result tooling.ToolResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		t.Fatalf("expected graph tool result JSON, got %q: %v", reply, err)
	}
	expectedRuMsg := services.HandoffOffHoursMessageForLocale("ru-RU")
	if result.Action != "off_hours_handoff" || result.ReplyText != expectedRuMsg {
		t.Fatalf("unexpected off-hours graph result for ru-RU: %+v, want %q", result, expectedRuMsg)
	}

	message := services.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Eq("sender_type", enums.IMSenderTypeAI).Desc("id"))
	if message == nil {
		t.Fatalf("expected off-hours notice message")
	}
	if message.Content != expectedRuMsg {
		t.Fatalf("expected Russian off-hours notice %q, got %q", expectedRuMsg, message.Content)
	}
}

func TestHandoffGraphOffHoursUsesCustomAgentNotice(t *testing.T) {
	db := setupHandoffGraphTestDB(t)
	aiAgent := createHandoffGraphAIAgent(t, db, "1")
	aiAgent.OffHoursMessage = "Операторы сейчас оффлайн. Мы ответим позже!"
	conversation := createHandoffGraphConversation(t, db, aiAgent.ID)

	reply, err := NewHandoffGraph(conversation, aiAgent).Run(context.Background(), `{"reason":"позови человека"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var result tooling.ToolResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		t.Fatalf("expected graph tool result JSON, got %q: %v", reply, err)
	}
	if result.Action != "off_hours_handoff" || result.ReplyText != aiAgent.OffHoursMessage {
		t.Fatalf("expected custom notice %q, got %+v", aiAgent.OffHoursMessage, result)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected conversation to be queued in pending status, got %v", current)
	}
}

func TestHandoffGraphWithActiveScheduleStillRequestsConfirmation(t *testing.T) {
	db := setupHandoffGraphTestDB(t)
	aiAgent := createHandoffGraphAIAgent(t, db, "1")
	createHandoffGraphTeam(t, db, 1)
	createHandoffGraphActiveSchedule(t, db, 1)
	conversation := createHandoffGraphConversation(t, db, aiAgent.ID)

	reply, err := NewHandoffGraph(conversation, aiAgent).Run(context.Background(), `{"reason":"用户要求转人工"}`)
	if err == nil {
		t.Fatalf("expected confirmation interrupt")
	}
	if !strings.Contains(err.Error(), InterruptTypeHandoffConfirmation) {
		t.Fatalf("expected handoff confirmation interrupt, got %v", err)
	}
	if reply != "" {
		t.Fatalf("expected no reply before confirmation, got %q", reply)
	}

	var count int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ?", conversation.ID).Count(&count).Error; err != nil {
		t.Fatalf("count messages error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no notice before confirmation, got %d messages", count)
	}
}

func setupHandoffGraphTestDB(t *testing.T) *gorm.DB {
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
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Channel{},
		&models.CustomerIdentity{},
		&models.AIAgent{},
		&models.AgentTeam{},
		&models.AgentTeamSchedule{},
		&models.Conversation{},
		&models.ConversationAssignment{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.Message{},
		&models.ChannelMessageOutbox{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createHandoffGraphAIAgent(t *testing.T, db *gorm.DB, teamIDs string) models.AIAgent {
	t.Helper()

	item := models.AIAgent{
		Name:        "测试AI",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		TeamIDs:     teamIDs,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}
	return item
}

func createHandoffGraphTeam(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()

	if err := db.Create(&models.AgentTeam{ID: id, Name: "售后支持组", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team error = %v", err)
	}
}

func createHandoffGraphActiveSchedule(t *testing.T, db *gorm.DB, teamID int64) {
	t.Helper()

	now := time.Now()
	if err := db.Create(&models.AgentTeamSchedule{
		TeamID:  teamID,
		StartAt: now.Add(-time.Hour),
		EndAt:   now.Add(time.Hour),
		Status:  enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create schedule error = %v", err)
	}
}

func createHandoffGraphConversation(t *testing.T, db *gorm.DB, aiAgentID int64) models.Conversation {
	t.Helper()

	now := time.Now()
	item := models.Conversation{
		AIAgentID:     aiAgentID,
		ChannelID:     1,
		CustomerID:    1,
		CustomerName:  "测试访客",
		Status:        enums.IMConversationStatusAIServing,
		ServiceMode:   enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now,
		LastActiveAt:  now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create conversation error = %v", err)
	}
	return item
}
