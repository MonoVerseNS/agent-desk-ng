package repositories

import (
	"testing"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Updates passes the map straight to GORM, so every key becomes a column name
// in the generated SQL. A key with no matching column makes the whole UPDATE
// fail, which is how the email channel silently lost every status transition:
// it wrote "send_detail" while the model only had LastError.
func TestChannelMessageOutboxUpdatesAcceptsServiceColumnNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:channel_message_outbox_repository_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	if err := db.AutoMigrate(&models.ChannelMessageOutbox{}); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	if err := db.Exec("DELETE FROM channel_message_outboxes").Error; err != nil {
		t.Fatalf("clean channel message outboxes error = %v", err)
	}

	item := &models.ChannelMessageOutbox{
		ChannelType:    enums.ChannelTypeEmail,
		ConversationID: 1,
		MessageID:      1,
		SendStatus:     string(enums.ChannelMessageOutboxStatusPending),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create outbox error = %v", err)
	}

	// The exact column sets the outbound services write.
	tests := []struct {
		name    string
		columns map[string]interface{}
	}{
		{
			name: "email marks sent",
			columns: map[string]interface{}{
				"send_status":   string(enums.ChannelMessageOutboxStatusSent),
				"send_detail":   "sent to user@example.com via smtp",
				"sent_at":       nil,
				"updated_at":    nil,
				"next_retry_at": nil,
			},
		},
		{
			name: "email marks failed",
			columns: map[string]interface{}{
				"send_status":   string(enums.ChannelMessageOutboxStatusFailed),
				"send_detail":   "smtp: connection refused",
				"updated_at":    nil,
				"next_retry_at": nil,
			},
		},
		{
			name: "telegram records last error",
			columns: map[string]interface{}{
				"send_status": string(enums.ChannelMessageOutboxStatusFailed),
				"last_error":  "telegram: 429 too many requests",
				"updated_at":  nil,
			},
		},
		{
			name: "retry bookkeeping",
			columns: map[string]interface{}{
				"retry_count":   2,
				"next_retry_at": nil,
				"updated_at":    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ChannelMessageOutboxRepository.Updates(db, item.ID, tt.columns); err != nil {
				t.Fatalf("Updates() error = %v", err)
			}
		})
	}

	var stored models.ChannelMessageOutbox
	if err := db.Where("id = ?", item.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload outbox error = %v", err)
	}
	if stored.SendDetail != "smtp: connection refused" {
		t.Fatalf("SendDetail = %q, want %q", stored.SendDetail, "smtp: connection refused")
	}
	if stored.LastError != "telegram: 429 too many requests" {
		t.Fatalf("LastError = %q, want %q", stored.LastError, "telegram: 429 too many requests")
	}
	if stored.RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", stored.RetryCount)
	}
}

// A retried row must come back as eligible for the dispatcher again.
func TestChannelMessageOutboxUpdatesClearsNextRetryAt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:channel_message_outbox_retry_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	if err := db.AutoMigrate(&models.ChannelMessageOutbox{}); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	if err := db.Exec("DELETE FROM channel_message_outboxes").Error; err != nil {
		t.Fatalf("clean channel message outboxes error = %v", err)
	}

	item := &models.ChannelMessageOutbox{
		ChannelType: enums.ChannelTypeEmail,
		MessageID:   2,
		SendStatus:  string(enums.ChannelMessageOutboxStatusPending),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create outbox error = %v", err)
	}
	if err := ChannelMessageOutboxRepository.Updates(db, item.ID, map[string]interface{}{
		"send_status":   string(enums.ChannelMessageOutboxStatusFailed),
		"send_detail":   "temporary failure",
		"retry_count":   1,
		"updated_at":    nil,
		"next_retry_at": nil,
	}); err != nil {
		t.Fatalf("Updates() error = %v", err)
	}

	var stored models.ChannelMessageOutbox
	if err := db.Where("id = ?", item.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload outbox error = %v", err)
	}
	if stored.SendStatus != string(enums.ChannelMessageOutboxStatusFailed) {
		t.Fatalf("SendStatus = %q, want %q", stored.SendStatus, enums.ChannelMessageOutboxStatusFailed)
	}
	if stored.NextRetryAt != nil {
		t.Fatalf("NextRetryAt = %v, want nil so the row is eligible again", stored.NextRetryAt)
	}
}
