package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupConversationAPITestDB(t *testing.T) (*gorm.DB, *models.Channel, *models.AIAgent, openidentity.ExternalUser) {
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
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.Channel{},
		&models.AIAgent{},
		&models.Conversation{},
		&models.ConversationAssignment{},
		&models.ConversationParticipant{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.ConversationSatisfaction{},
		&models.Message{},
		&models.User{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)

	previousConfig := config.GetCurrent()
	config.SetCurrent(&config.Config{
		Conversation: config.ConversationConfig{CustomerMaxOpen: 5},
	})
	t.Cleanup(func() {
		config.SetCurrent(previousConfig)
	})

	aiAgent := models.AIAgent{
		Name:        "CSAT Bot",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&aiAgent).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}

	channel := models.Channel{
		ChannelID:   "web-channel-csat",
		Name:        "Web Channel",
		ChannelType: enums.ChannelTypeWeb,
		AIAgentID:   aiAgent.ID,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel error = %v", err)
	}

	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "visitor_123",
		ExternalName:   "Visitor 123",
	}

	return db, &channel, &aiAgent, external
}

func TestConversationSatisfactionAPI(t *testing.T) {
	_, channel, aiAgent, external := setupConversationAPITestDB(t)

	// Create conversation and close it
	conv, err := services.ConversationService.CreateNew(external, channel.ID, aiAgent.ID, "Need help with service")
	if err != nil {
		t.Fatalf("CreateNew error = %v", err)
	}
	if err := services.ConversationService.CloseCustomerConversation(conv.ID, external); err != nil {
		t.Fatalf("CloseCustomerConversation error = %v", err)
	}

	router := gin.New()
	router.POST("/api/conversation/satisfaction", func(c *gin.Context) {
		ConversationPostSatisfaction(c)
	})
	router.GET("/api/conversation/:id", func(c *gin.Context) {
		ConversationGetBy(c)
	})

	t.Run("missing channel returns error", func(t *testing.T) {
		body, _ := json.Marshal(request.SubmitSatisfactionRequest{
			ConversationID: conv.ID,
			Score:          5,
			Feedback:       "Great",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/conversation/satisfaction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		// no channel header
		httpx.SetExternalUser(&gin.Context{}, &external)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected failure without channel, got success: true")
		}
	})

	t.Run("missing external user returns error", func(t *testing.T) {
		body, _ := json.Marshal(request.SubmitSatisfactionRequest{
			ConversationID: conv.ID,
			Score:          5,
			Feedback:       "Great",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/conversation/satisfaction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Channel-ID", channel.ChannelID)
		// no external user set in context

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected failure without external user, got success: true")
		}
	})

	// Helper for authorized requests
	authRouter := gin.New()
	authRouter.Use(func(c *gin.Context) {
		httpx.SetExternalUser(c, &external)
		c.Next()
	})
	authRouter.POST("/api/conversation/satisfaction", ConversationPostSatisfaction)
	authRouter.GET("/api/conversation/:id", ConversationGetBy)

	t.Run("rejects invalid score", func(t *testing.T) {
		invalidScores := []int{0, -1, 6, 10}
		for _, score := range invalidScores {
			body, _ := json.Marshal(request.SubmitSatisfactionRequest{
				ConversationID: conv.ID,
				Score:          score,
				Feedback:       "Invalid score test",
			})
			req, _ := http.NewRequest(http.MethodPost, "/api/conversation/satisfaction", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Channel-ID", channel.ChannelID)

			rec := httptest.NewRecorder()
			authRouter.ServeHTTP(rec, req)

			var resp map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp["success"] == true {
				t.Fatalf("expected failure for score %d, got success: true", score)
			}
		}
	})

	t.Run("rejects unauthorized external user", func(t *testing.T) {
		attacker := openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     "attacker_999",
			ExternalName:   "Attacker",
		}
		attackerRouter := gin.New()
		attackerRouter.Use(func(c *gin.Context) {
			httpx.SetExternalUser(c, &attacker)
			c.Next()
		})
		attackerRouter.POST("/api/conversation/satisfaction", ConversationPostSatisfaction)

		body, _ := json.Marshal(request.SubmitSatisfactionRequest{
			ConversationID: conv.ID,
			Score:          5,
			Feedback:       "Hacked rating",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/conversation/satisfaction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Channel-ID", channel.ChannelID)

		rec := httptest.NewRecorder()
		attackerRouter.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] == true {
			t.Fatalf("expected forbidden failure for non-owner, got success: true")
		}
	})

	t.Run("successful submission and response mapping", func(t *testing.T) {
		body, _ := json.Marshal(request.SubmitSatisfactionRequest{
			ConversationID: conv.ID,
			Score:          5,
			Feedback:       "Outstanding support!",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/conversation/satisfaction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Channel-ID", channel.ChannelID)

		rec := httptest.NewRecorder()
		authRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rec.Code)
		}

		type JSONResult struct {
			Code int                                        `json:"code"`
			Msg  string                                     `json:"msg"`
			Ok   bool                                       `json:"success"`
			Data *response.ConversationSatisfactionResponse `json:"data"`
		}
		var result JSONResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if !result.Ok {
			t.Fatalf("expected success: true, got code=%d msg=%s", result.Code, result.Msg)
		}
		if result.Data == nil {
			t.Fatalf("expected non-nil data")
		}
		if result.Data.ConversationID != conv.ID {
			t.Fatalf("ConversationID = %d, want %d", result.Data.ConversationID, conv.ID)
		}
		if result.Data.Score != 5 {
			t.Fatalf("Score = %d, want 5", result.Data.Score)
		}
		if result.Data.Feedback != "Outstanding support!" {
			t.Fatalf("Feedback = %q, want %q", result.Data.Feedback, "Outstanding support!")
		}
		if result.Data.ID == 0 {
			t.Fatalf("expected non-zero satisfaction ID")
		}
		if result.Data.CreatedAt == "" {
			t.Fatalf("expected non-empty CreatedAt")
		}

		// Verify ConversationGetBy now returns satisfaction
		getReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/conversation/%d", conv.ID), nil)
		getReq.Header.Set("X-Channel-ID", channel.ChannelID)
		getRec := httptest.NewRecorder()
		authRouter.ServeHTTP(getRec, getReq)

		type DetailResult struct {
			Ok   bool                                 `json:"success"`
			Data *response.ConversationDetailResponse `json:"data"`
		}
		var detailResult DetailResult
		if err := json.Unmarshal(getRec.Body.Bytes(), &detailResult); err != nil {
			t.Fatalf("failed to unmarshal detail response: %v", err)
		}
		if !detailResult.Ok || detailResult.Data == nil {
			t.Fatalf("expected success detail result")
		}
		if detailResult.Data.Satisfaction == nil {
			t.Fatalf("expected ConversationResponse to include satisfaction, got nil")
		}
		if detailResult.Data.Satisfaction.Score != 5 {
			t.Fatalf("satisfaction score = %d, want 5", detailResult.Data.Satisfaction.Score)
		}
		if detailResult.Data.Satisfaction.Feedback != "Outstanding support!" {
			t.Fatalf("satisfaction feedback = %q, want %q", detailResult.Data.Satisfaction.Feedback, "Outstanding support!")
		}
	})

	t.Run("idempotent update submission", func(t *testing.T) {
		body, _ := json.Marshal(request.SubmitSatisfactionRequest{
			ConversationID: conv.ID,
			Score:          4,
			Feedback:       "Updated to 4 stars",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/conversation/satisfaction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Channel-ID", channel.ChannelID)

		rec := httptest.NewRecorder()
		authRouter.ServeHTTP(rec, req)

		var result struct {
			Ok   bool                                       `json:"success"`
			Data *response.ConversationSatisfactionResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if !result.Ok || result.Data == nil {
			t.Fatalf("expected success result")
		}
		if result.Data.Score != 4 {
			t.Fatalf("updated score = %d, want 4", result.Data.Score)
		}
		if result.Data.Feedback != "Updated to 4 stars" {
			t.Fatalf("updated feedback = %q, want %q", result.Data.Feedback, "Updated to 4 stars")
		}
	})
}
