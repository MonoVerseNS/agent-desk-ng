package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/repositories"
	"testing"

	"github.com/mlogclub/simple/sqls"
)

func setupSatisfactionTest(t *testing.T) (*conversationService, *conversationSatisfactionService, openidentity.ExternalUser, int64) {
	t.Helper()
	convSvc, external, aiAgentID := setupCustomerConversationTest(t, 5)
	if err := sqls.DB().AutoMigrate(&models.ConversationSatisfaction{}); err != nil {
		t.Fatalf("auto migrate conversation satisfaction error = %v", err)
	}
	return convSvc, ConversationSatisfactionService, external, aiAgentID
}

func TestSubmitSatisfaction(t *testing.T) {
	convSvc, csatSvc, external, aiAgentID := setupSatisfactionTest(t)

	// Create conversation and close it
	conv, err := convSvc.CreateNew(external, 1, aiAgentID, "Need help with refund")
	if err != nil {
		t.Fatalf("CreateNew() error = %v", err)
	}
	if err := convSvc.CloseCustomerConversation(conv.ID, external); err != nil {
		t.Fatalf("CloseCustomerConversation() error = %v", err)
	}

	t.Run("rejects invalid score", func(t *testing.T) {
		invalidScores := []int{0, -1, 6, 10}
		for _, score := range invalidScores {
			_, err := csatSvc.SubmitSatisfaction(conv.ID, external, score, "Bad score")
			if err == nil {
				t.Fatalf("SubmitSatisfaction() with score %d expected error, got nil", score)
			}
		}
	})

	t.Run("rejects if external user is not conversation owner", func(t *testing.T) {
		unauthorizedUser := openidentity.ExternalUser{
			ExternalSource: "user",
			ExternalID:     "other_user",
			ExternalName:   "Attacker",
		}
		_, err := csatSvc.SubmitSatisfaction(conv.ID, unauthorizedUser, 5, "Nice")
		if err == nil {
			t.Fatal("SubmitSatisfaction() with unauthorized user expected forbidden error, got nil")
		}
	})

	t.Run("creates record when valid and closed", func(t *testing.T) {
		res, err := csatSvc.SubmitSatisfaction(conv.ID, external, 5, "Very helpful!")
		if err != nil {
			t.Fatalf("SubmitSatisfaction() error = %v", err)
		}
		if res == nil {
			t.Fatal("expected satisfaction record, got nil")
		}
		if res.ConversationID != conv.ID {
			t.Fatalf("ConversationID = %d, want %d", res.ConversationID, conv.ID)
		}
		if res.Score != 5 {
			t.Fatalf("Score = %d, want 5", res.Score)
		}
		if res.Feedback != "Very helpful!" {
			t.Fatalf("Feedback = %q, want %q", res.Feedback, "Very helpful!")
		}
		if res.CustomerID != conv.CustomerID {
			t.Fatalf("CustomerID = %d, want %d", res.CustomerID, conv.CustomerID)
		}
		if res.ChannelID != conv.ChannelID {
			t.Fatalf("ChannelID = %d, want %d", res.ChannelID, conv.ChannelID)
		}

		// Verify retrieval
		fetched := csatSvc.GetByConversationID(conv.ID)
		if fetched == nil {
			t.Fatal("GetByConversationID() returned nil")
		}
		if fetched.ID != res.ID || fetched.Score != 5 {
			t.Fatalf("fetched record mismatch: %+v", fetched)
		}
	})

	t.Run("returns existing record if already rated (idempotent)", func(t *testing.T) {
		initial := csatSvc.GetByConversationID(conv.ID)
		if initial == nil {
			t.Fatal("initial satisfaction record not found")
		}

		updated, err := csatSvc.SubmitSatisfaction(conv.ID, external, 4, "Updated: Good overall")
		if err != nil {
			t.Fatalf("SubmitSatisfaction() update error = %v", err)
		}
		if updated.ID != initial.ID {
			t.Fatalf("expected updated record ID to be %d, got %d", initial.ID, updated.ID)
		}
		if updated.Score != 4 {
			t.Fatalf("Score = %d, want 4", updated.Score)
		}
		if updated.Feedback != "Updated: Good overall" {
			t.Fatalf("Feedback = %q, want %q", updated.Feedback, "Updated: Good overall")
		}

		// Count should still be 1 for this conversation
		count := repositories.ConversationSatisfactionRepository.Count(sqls.DB(), "conversation_id = ?", conv.ID)
		if count != 1 {
			t.Fatalf("expected count = 1, got %d", count)
		}

		// Average score should be 4.0
		avg := repositories.ConversationSatisfactionRepository.AverageScore(sqls.DB(), "conversation_id = ?", conv.ID)
		if avg != 4.0 {
			t.Fatalf("expected avg = 4.0, got %f", avg)
		}
	})

	t.Run("returns error when conversation does not exist", func(t *testing.T) {
		_, err := csatSvc.SubmitSatisfaction(999999, external, 5, "Non-existent")
		if err == nil {
			t.Fatal("expected error for non-existent conversation, got nil")
		}
	})

	t.Run("returns nil for non-positive conversation ID in GetByConversationID", func(t *testing.T) {
		if got := csatSvc.GetByConversationID(0); got != nil {
			t.Fatalf("expected nil for conversationID 0, got %+v", got)
		}
		if got := csatSvc.GetByConversationID(-1); got != nil {
			t.Fatalf("expected nil for conversationID -1, got %+v", got)
		}
		if got := repositories.ConversationSatisfactionRepository.Get(sqls.DB(), 0); got != nil {
			t.Fatalf("expected nil for ID 0, got %+v", got)
		}
		if got := repositories.ConversationSatisfactionRepository.GetByConversationID(sqls.DB(), 0); got != nil {
			t.Fatalf("expected nil for conversationID 0, got %+v", got)
		}
	})

	t.Run("returns zero average when no records match", func(t *testing.T) {
		avg := repositories.ConversationSatisfactionRepository.AverageScore(sqls.DB(), "conversation_id = ?", 888888)
		if avg != 0 {
			t.Fatalf("expected 0 average for unmatched condition, got %f", avg)
		}
	})
}
