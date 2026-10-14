package services

import (
	"encoding/json"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
)

func TestWsNotificationTopic(t *testing.T) {
	svc := newWsService()
	if got := svc.notificationTopic(123); got != "notification:123" {
		t.Fatalf("expected notification:123, got %q", got)
	}
}

func TestWsNotificationCreatedEventType(t *testing.T) {
	event := RealtimeNotificationCreatedEvent{
		Payload: RealtimeNotificationCreatedPayload{
			Notification: response.NotificationResponse{ID: 1},
		},
	}
	if got := event.EventType(); got != "notification.created" {
		t.Fatalf("expected notification.created, got %q", got)
	}
	if payload := event.EventPayload(); payload == nil {
		t.Fatalf("expected payload")
	}
}

func TestWsAgentActivityEventType(t *testing.T) {
	event := RealtimeAgentActivityEvent{
		Payload: RealtimeAgentActivityPayload{
			ConversationID: 100,
			StepCode:       "searching_knowledge",
			Status:         "running",
		},
	}
	if got := event.EventType(); got != enums.IMRealtimeEventAgentActivity {
		t.Fatalf("expected %s, got %q", enums.IMRealtimeEventAgentActivity, got)
	}
	if payload := event.EventPayload(); payload == nil {
		t.Fatalf("expected payload")
	}
}

func TestPublishAgentActivity(t *testing.T) {
	ws := newWsService()
	session := &ClientSession{
		ID:     "test-sess",
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 10),
	}
	ws.manager.Register(session, []string{"conversation:100"})

	conv := &models.Conversation{ID: 100}
	ws.PublishAgentActivity(conv, "searching_knowledge", "", "running")

	select {
	case payload := <-session.Send:
		var event struct {
			Type string                       `json:"type"`
			Data RealtimeAgentActivityPayload `json:"data"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventAgentActivity {
			t.Fatalf("expected type %s, got %s", enums.IMRealtimeEventAgentActivity, event.Type)
		}
		if event.Data.ConversationID != 100 || event.Data.StepCode != "searching_knowledge" || event.Data.Status != "running" {
			t.Fatalf("unexpected data: %+v", event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for agent activity event")
	}
}
