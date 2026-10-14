package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupConversationDashboardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

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
		&models.Conversation{},
		&models.Message{},
		&models.User{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func TestConversationSuggestReply(t *testing.T) {
	db := setupConversationDashboardTestDB(t)

	conv := &models.Conversation{
		ID:      101,
		Subject: "Suggest Reply Test",
	}
	if err := db.Create(conv).Error; err != nil {
		t.Fatalf("create conv error = %v", err)
	}
	msg := &models.Message{
		ID:             1,
		ConversationID: conv.ID,
		ClientMsgID:    "msg_1",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "Can you help me reset my password?",
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("create msg error = %v", err)
	}

	t.Run("unauthorized when no auth principal", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/dashboard/conversation/suggest_reply", ConversationPostSuggest_reply)

		body, _ := json.Marshal(request.SuggestReplyRequest{ConversationID: conv.ID})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/suggest_reply", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected failure without auth, got success: true")
		}
	})

	t.Run("forbidden when principal lacks permission", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("authPrincipal", &dto.AuthPrincipal{
				UserID:      1,
				Username:    "agent",
				Permissions: []string{"unrelated.permission"},
			})
			c.Next()
		})
		router.POST("/api/dashboard/conversation/suggest_reply", ConversationPostSuggest_reply)

		body, _ := json.Marshal(request.SuggestReplyRequest{ConversationID: conv.ID})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/suggest_reply", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected forbidden error, got success: true")
		}
	})

	t.Run("invalid params when conversationId <= 0", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("authPrincipal", &dto.AuthPrincipal{
				UserID:      1,
				Username:    "agent",
				Permissions: []string{constants.PermissionConversationView.Code},
			})
			c.Next()
		})
		router.POST("/api/dashboard/conversation/suggest_reply", ConversationPostSuggest_reply)

		body, _ := json.Marshal(request.SuggestReplyRequest{ConversationID: 0})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/suggest_reply", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected error for conversationId <= 0, got success: true")
		}
	})

	t.Run("successful suggestion", func(t *testing.T) {
		restore := services.AICopilotService.SetChatFnForTest(func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
			return &ai.ChatCompletionResult{
				Content:   "Click 'Forgot Password' to reset your credentials.",
				ModelName: "mock-model",
			}, nil
		})
		defer restore()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("authPrincipal", &dto.AuthPrincipal{
				UserID:      1,
				Username:    "agent",
				Permissions: []string{constants.PermissionConversationView.Code},
			})
			c.Next()
		})
		router.POST("/api/dashboard/conversation/suggest_reply", ConversationPostSuggest_reply)

		body, _ := json.Marshal(request.SuggestReplyRequest{ConversationID: conv.ID})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/suggest_reply", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Code    int                           `json:"code"`
			Success bool                          `json:"success"`
			Data    response.SuggestReplyResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response error = %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success: true, got false. Body: %s", rec.Body.String())
		}
		expectedSuggestion := "Click 'Forgot Password' to reset your credentials."
		if resp.Data.Suggestion != expectedSuggestion {
			t.Fatalf("expected suggestion %q, got %q", expectedSuggestion, resp.Data.Suggestion)
		}
	})
}

func TestConversationSummarize(t *testing.T) {
	db := setupConversationDashboardTestDB(t)

	conv := &models.Conversation{
		ID:      201,
		Subject: "Summarize Test",
	}
	if err := db.Create(conv).Error; err != nil {
		t.Fatalf("create conv error = %v", err)
	}
	msg := &models.Message{
		ID:             20,
		ConversationID: conv.ID,
		ClientMsgID:    "msg_20",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "Billing dispute for invoice #1234",
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("create msg error = %v", err)
	}

	t.Run("unauthorized when no auth principal", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/dashboard/conversation/summarize", ConversationPostSummarize)

		body, _ := json.Marshal(request.SummarizeConversationRequest{ConversationID: conv.ID})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/summarize", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected failure without auth, got success: true")
		}
	})

	t.Run("invalid params when conversationId <= 0", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("authPrincipal", &dto.AuthPrincipal{
				UserID:      1,
				Username:    "agent",
				Permissions: []string{constants.PermissionConversationView.Code},
			})
			c.Next()
		})
		router.POST("/api/dashboard/conversation/summarize", ConversationPostSummarize)

		body, _ := json.Marshal(request.SummarizeConversationRequest{ConversationID: -5})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/summarize", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected error for conversationId <= 0, got success: true")
		}
	})

	t.Run("successful summary", func(t *testing.T) {
		restore := services.AICopilotService.SetChatFnForTest(func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
			return &ai.ChatCompletionResult{
				Content:   "Summary: Customer inquired about invoice #1234.",
				ModelName: "mock-model",
			}, nil
		})
		defer restore()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("authPrincipal", &dto.AuthPrincipal{
				UserID:      1,
				Username:    "agent",
				Permissions: []string{constants.PermissionConversationView.Code},
			})
			c.Next()
		})
		router.POST("/api/dashboard/conversation/summarize", ConversationPostSummarize)

		body, _ := json.Marshal(request.SummarizeConversationRequest{ConversationID: conv.ID})
		req, _ := http.NewRequest(http.MethodPost, "/api/dashboard/conversation/summarize", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Code    int                                  `json:"code"`
			Success bool                                 `json:"success"`
			Data    response.ConversationSummaryResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response error = %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success: true, got false. Body: %s", rec.Body.String())
		}
		expectedSummary := "Summary: Customer inquired about invoice #1234."
		if resp.Data.Summary != expectedSummary {
			t.Fatalf("expected summary %q, got %q", expectedSummary, resp.Data.Summary)
		}
	})
}
