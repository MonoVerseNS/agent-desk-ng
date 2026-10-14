package services

import (
	"encoding/json"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
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

func TestWsOperatorPresenceEventType(t *testing.T) {
	event := RealtimeOperatorPresenceEvent{
		Payload: RealtimeOperatorPresencePayload{
			ConversationID: 100,
			UserID:         1,
			Username:       "admin",
			Nickname:       "Admin",
			Action:         "viewing",
			At:             time.Now().Format(time.RFC3339),
		},
	}
	if got := event.EventType(); got != enums.IMRealtimeEventOperatorPresence {
		t.Fatalf("expected %s, got %q", enums.IMRealtimeEventOperatorPresence, got)
	}
	if payload := event.EventPayload(); payload == nil {
		t.Fatalf("expected payload")
	}
}

func TestPublishOperatorPresence(t *testing.T) {
	ws := newWsService()

	subscriber := &ClientSession{
		ID:     "subscriber-conn",
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 10),
	}
	ws.manager.Register(subscriber, []string{"conversation:100"})

	sender := &ClientSession{
		ID:     "sender-conn",
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 10),
	}
	ws.manager.Register(sender, []string{"conversation:100"})

	principal := &dto.AuthPrincipal{
		UserID:   2,
		Username: "operator",
		Nickname: "Agent Smith",
	}

	ws.PublishOperatorPresence(100, principal, "typing", sender.ID)

	select {
	case payload := <-subscriber.Send:
		var event struct {
			Type string                          `json:"type"`
			Data RealtimeOperatorPresencePayload `json:"data"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventOperatorPresence {
			t.Fatalf("expected type %s, got %s", enums.IMRealtimeEventOperatorPresence, event.Type)
		}
		if event.Data.ConversationID != 100 || event.Data.UserID != 2 || event.Data.Username != "operator" || event.Data.Nickname != "Agent Smith" || event.Data.Action != "typing" {
			t.Fatalf("unexpected data: %+v", event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for operator presence event on subscriber")
	}

	select {
	case payload := <-sender.Send:
		t.Fatalf("sender should have been excluded, received: %s", string(payload))
	default:
	}
}

func TestHandleClientMessagePresence(t *testing.T) {
	ws := newWsService()

	subscriber := &ClientSession{
		ID:     "subscriber-conn",
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 10),
	}
	ws.manager.Register(subscriber, []string{"conversation:100"})

	principal := &dto.AuthPrincipal{
		UserID:   2,
		Username: "operator",
		Nickname: "Agent Smith",
	}
	sender := &ClientSession{
		ID:        "sender-conn",
		Principal: principal,
		Topics:    map[string]struct{}{},
		Send:      make(chan []byte, 10),
	}
	ws.manager.Register(sender, []string{"conversation:100"})

	msgJSON := []byte(`{"type":"presence","conversationId":100,"action":"viewing"}`)
	ws.handleClientMessage(sender, msgJSON)

	select {
	case payload := <-subscriber.Send:
		var event struct {
			Type string                          `json:"type"`
			Data RealtimeOperatorPresencePayload `json:"data"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventOperatorPresence {
			t.Fatalf("expected %s, got %s", enums.IMRealtimeEventOperatorPresence, event.Type)
		}
		if event.Data.ConversationID != 100 || event.Data.Action != "viewing" || event.Data.UserID != 2 {
			t.Fatalf("unexpected data: %+v", event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for presence broadcast")
	}

	select {
	case payload := <-sender.Send:
		t.Fatalf("sender should not receive its own presence: %s", string(payload))
	default:
	}
}
