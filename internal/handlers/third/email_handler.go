package third

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

// EmailPostWebhook receives incoming inbound email webhook events from Cloudflare, Brevo, SendGrid, Postmark, Mailgun, or SMTP forwarders.
func EmailPostWebhook(ctx *gin.Context) {
	channelID := strings.TrimSpace(ctx.Param("channel_id"))
	if channelID == "" {
		channelID = strings.TrimSpace(ctx.Query("channel_id"))
	}

	secretHeader := ctx.GetHeader("X-Webhook-Secret")
	if secretHeader == "" {
		secretHeader = ctx.GetHeader("X-Brevo-Webhook-Secret")
	}
	if secretHeader == "" {
		secretHeader = ctx.GetHeader("X-Postmark-Webhook-Secret")
	}
	if secretHeader == "" {
		secretHeader = ctx.Query("secret")
	}

	contentType := ctx.GetHeader("Content-Type")

	var formValues url.Values
	var bodyBytes []byte

	if strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		if err := ctx.Request.ParseMultipartForm(32 << 20); err == nil && ctx.Request.MultipartForm != nil {
			formValues = ctx.Request.MultipartForm.Value
		}
	} else if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		if err := ctx.Request.ParseForm(); err == nil {
			formValues = ctx.Request.PostForm
		}
	}

	if ctx.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(ctx.Request.Body)
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	if err := services.EmailInboundService.HandleWebhook(ctx.Request.Context(), channelID, secretHeader, contentType, bodyBytes, formValues); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"ok": true, "message": "email processed"})
}

// EmailPostRequest opens a support request from an external system and continues
// the conversation over email.
//
// It is the API counterpart to EmailPostWebhook: same channel, same identity, but
// the caller supplies the fields instead of a parsed email, so an integration
// does not have to run a mail forwarder to reach support.
//
// Presenting X-Webhook-Secret marks the submitted contact details as verified.
// Omitting it files the request as self-reported, which agents can see. A wrong
// secret is rejected rather than treated as a guest submission.
func EmailPostRequest(ctx *gin.Context) {
	channelID := strings.TrimSpace(ctx.Param("channel_id"))
	if channelID == "" {
		channelID = strings.TrimSpace(ctx.Query("channel_id"))
	}

	req := request.EmailRequestRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	result, err := services.EmailRequestService.HandleRequest(
		ctx.Request.Context(),
		channelID,
		webhookSecretFromHeader(ctx),
		services.EmailRequestInput{
			Email:   req.Email,
			Name:    req.Name,
			Subject: req.Subject,
			Message: req.Message,
		},
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// webhookSecretFromHeader reads the channel secret. The query-string form the
// webhook accepts is deliberately not honoured here: this endpoint is meant to be
// called by programs, and a secret in a URL leaks into access logs and proxy
// history for no benefit.
func webhookSecretFromHeader(ctx *gin.Context) string {
	for _, header := range []string{"X-Webhook-Secret", "X-Breveo-Webhook-Secret", "X-Postmark-Webhook-Secret"} {
		if value := strings.TrimSpace(ctx.GetHeader(header)); value != "" {
			return value
		}
	}
	return ""
}