package services

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/i18nx"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/pkg/utils"

	"github.com/mlogclub/simple/common/strs"
	"github.com/mlogclub/simple/sqls"
)

var EmailRequestService = newEmailRequestService()

func newEmailRequestService() *emailRequestService {
	return &emailRequestService{}
}

type emailRequestService struct{}

// EmailRequestSourceAPI marks a message that entered through the API rather than
// through a real inbound email, so nothing downstream mistakes it for one.
const EmailRequestSourceAPI = "api_request"

// maxEmailRequestSubjectLength mirrors the conversation subject column width.
const maxEmailRequestSubjectLength = 255

// EmailRequestInput is what an external system submits to open a support request
// on a visitor's behalf.
type EmailRequestInput struct {
	Email   string
	Name    string
	Subject string
	Message string
}

// EmailRequestOutcome says where the submission actually landed, which is not
// necessarily an email thread: a live chat takes precedence.
type EmailRequestOutcome string

const (
	// EmailRequestOutcomeEmail means the request was filed as an email thread.
	EmailRequestOutcomeEmail EmailRequestOutcome = "email"
	// EmailRequestOutcomeChat means a chat was still live, so the submission was
	// appended to it and the visitor was nudged back to it.
	EmailRequestOutcomeChat EmailRequestOutcome = "chat"
)

type EmailRequestResult struct {
	ConversationID int64               `json:"conversationId"`
	Outcome        EmailRequestOutcome `json:"outcome"`
	// Verified reports whether the submitter presented the channel secret. When
	// false the contact details are self-reported and agents must be told so.
	Verified bool   `json:"verified"`
	Subject  string `json:"subject"`
}

// HandleRequest files a support request supplied over the API and continues the
// conversation over email from there.
//
// Two modes share one endpoint. Presenting the channel secret means the submitter
// is trusted and the address becomes a verified contact; omitting it files the
// request as self-reported. A secret that is present but wrong is an error rather
// than a silent downgrade, because a typo must not quietly turn a verified
// integration into an unverified one.
func (s *emailRequestService) HandleRequest(ctx context.Context, channelID, secretHeader string, in EmailRequestInput) (*EmailRequestResult, error) {
	channel, err := s.resolveChannel(channelID)
	if err != nil {
		return nil, err
	}
	channelCfg, err := ChannelService.ParseEmailChannelConfig(channel.ConfigJSON)
	if err != nil || channelCfg == nil {
		return nil, errorsx.InvalidParamI18n("error.email.channelConfigInvalid")
	}

	verified, err := verifyEmailRequestSecret(channelCfg, secretHeader)
	if err != nil {
		return nil, err
	}

	fromEmail := normalizeEmailAddress(in.Email)
	if err := validateEmailRequestAddress(fromEmail); err != nil {
		return nil, err
	}
	fromName := strings.TrimSpace(in.Name)
	if fromName == "" {
		fromName = localPartOf(fromEmail)
	}
	subject := normalizeEmailRequestSubject(in.Subject)
	bodyText := strings.TrimSpace(in.Message)
	if bodyText == "" {
		return nil, errorsx.InvalidParamI18n("error.email.requestMessageRequired")
	}

	externalUser := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceEmail,
		ExternalID:     fromEmail,
		ExternalName:   fromName,
	}

	customerID, err := s.ensureCustomer(externalUser)
	if err != nil {
		return nil, err
	}
	if err := s.markContactVerification(customerID, fromEmail, verified); err != nil {
		slog.Warn("failed to record email contact verification",
			"customerId", customerID, "email", fromEmail, "verified", verified, "error", err)
	}

	content := bodyText
	if subject != "" {
		content = fmt.Sprintf("[%s]\n\n%s", subject, bodyText)
	}

	// A chat the visitor is still using wins over their email. Mail that jumps
	// into an open chat gets answered where the visitor is no longer watching.
	if liveChat := s.findLiveChat(customerID, channel.ID); liveChat != nil {
		if err := s.postRequestMessage(liveChat, externalUser, content, subject, fromEmail, fromName); err != nil {
			return nil, err
		}
		if err := s.sendChatNudge(liveChat, channel); err != nil {
			// The message is already in the chat, so a failed notice is not fatal.
			slog.Warn("failed to send chat continuation notice",
				"conversationId", liveChat.ID, "error", err)
		}
		return &EmailRequestResult{
			ConversationID: liveChat.ID,
			Outcome:        EmailRequestOutcomeChat,
			Verified:       verified,
			Subject:        subject,
		}, nil
	}

	conversation, err := ConversationService.CreateNew(externalUser, channel.ID, channel.AIAgentID, subject)
	if err != nil {
		return nil, fmt.Errorf("create email conversation failed: %w", err)
	}
	if err := s.postRequestMessage(conversation, externalUser, content, subject, fromEmail, fromName); err != nil {
		return nil, err
	}
	if err := s.sendAcknowledgement(conversation, channelCfg, channel); err != nil {
		// The request is already filed. Failing here would make the caller retry
		// and file the same request twice, which is worse than a missing receipt.
		slog.Warn("failed to send email acknowledgement",
			"conversationId", conversation.ID, "error", err)
	}

	return &EmailRequestResult{
		ConversationID: conversation.ID,
		Outcome:        EmailRequestOutcomeEmail,
		Verified:       verified,
		Subject:        subject,
	}, nil
}

// resolveChannel prefers an explicit channel id and otherwise falls back to the
// first active email channel, mirroring the webhook path.
func (s *emailRequestService) resolveChannel(channelID string) (*models.Channel, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID != "" {
		channel := ChannelService.Take(
			"channel_id = ? AND channel_type = ? AND status = ?",
			channelID, enums.ChannelTypeEmail, enums.StatusOk,
		)
		if channel == nil {
			return nil, errorsx.InvalidParamI18n("error.email.channelNotFound")
		}
		return channel, nil
	}
	channels := ChannelService.Find(sqls.NewCnd().
		Eq("channel_type", enums.ChannelTypeEmail).
		Eq("status", enums.StatusOk).
		Asc("id"))
	if len(channels) == 0 {
		return nil, errorsx.InvalidParamI18n("error.email.channelNotFound")
	}
	return &channels[0], nil
}

// verifyEmailRequestSecret decides between the trusted and self-reported modes.
func verifyEmailRequestSecret(cfg *dto.EmailChannelConfig, secretHeader string) (bool, error) {
	expected := strings.TrimSpace(cfg.WebhookSecret)
	if expected == "" {
		if current := config.GetCurrent(); current != nil {
			expected = strings.TrimSpace(current.Email.InboundSecret)
		}
	}
	provided := strings.TrimSpace(secretHeader)

	if provided == "" {
		// Nothing was offered to check, so nothing is being trusted. This is the
		// documented guest mode, not a bypass.
		return false, nil
	}
	if expected == "" {
		// A secret was sent but the deployment never configured one to compare
		// against, so we cannot honestly call the submitter verified.
		return false, errorsx.InvalidParamI18n("error.email.requestSecretNotConfigured")
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return false, errorsx.UnauthorizedI18n("error.auth.invalidSignature")
	}
	return true, nil
}

func normalizeEmailAddress(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func localPartOf(address string) string {
	if parts := strings.Split(address, "@"); len(parts) > 0 {
		return parts[0]
	}
	return address
}

// validateEmailRequestAddress stays deliberately permissive. The channel routes
// by recipient address rather than by validating the sender, so a strict check
// would reject legitimate addresses while proving nothing about ownership.
func validateEmailRequestAddress(address string) error {
	if address == "" || !strings.Contains(address, "@") || len(address) > 200 {
		return errorsx.InvalidParamI18n("error.email.requestEmailInvalid")
	}
	return nil
}

func normalizeEmailRequestSubject(subject string) string {
	subject = strings.Join(strings.Fields(subject), " ")
	runes := []rune(subject)
	if len(runes) > maxEmailRequestSubjectLength {
		return strings.TrimSpace(string(runes[:maxEmailRequestSubjectLength]))
	}
	return subject
}

func (s *emailRequestService) ensureCustomer(externalUser openidentity.ExternalUser) (int64, error) {
	var customerID int64
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		id, err := CustomerService.EnsureExternalCustomer(ctx, externalUser)
		if err != nil {
			return err
		}
		customerID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return customerID, nil
}

// findLiveChat returns the customer's most recent open conversation when it is
// not on this email channel and still has recent activity: the case where the
// visitor is watching a chat and an email should not steal the thread.
func (s *emailRequestService) findLiveChat(customerID, emailChannelID int64) *models.Conversation {
	if customerID <= 0 {
		return nil
	}
	conversations := ConversationService.Find(sqls.NewCnd().
		Eq("customer_id", customerID).
		In("status", []enums.IMConversationStatus{
			enums.IMConversationStatusAIServing,
			enums.IMConversationStatusPending,
			enums.IMConversationStatusActive,
		}).
		Desc("last_active_at").
		Desc("id"))
	for i := range conversations {
		conversation := conversations[i]
		if conversation.ChannelID == emailChannelID {
			// Their thread is already on email; keep everything there.
			return nil
		}
		if isConversationLive(&conversation, emailChatLivenessWindow()) {
			return &conversation
		}
	}
	return nil
}

// emailChatLivenessWindow is how long a chat still counts as being watched. A
// chat the visitor abandoned is where an email should not be answered.
func emailChatLivenessWindow() time.Duration {
	if current := config.GetCurrent(); current != nil {
		return current.Conversation.EmailChatLiveWindow()
	}
	return 30 * time.Minute
}

func isConversationLive(conversation *models.Conversation, window time.Duration) bool {
	if conversation == nil || conversation.LastActiveAt.IsZero() || window <= 0 {
		return false
	}
	return time.Since(conversation.LastActiveAt) <= window
}

func (s *emailRequestService) postRequestMessage(conversation *models.Conversation, externalUser openidentity.ExternalUser, content, subject, fromEmail, fromName string) error {
	payloadMap := map[string]any{
		"email_from":      fromEmail,
		"email_from_name": fromName,
		"email_subject":   subject,
		"email_source":    EmailRequestSourceAPI,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	_, err := MessageService.SendCustomerMessage(
		conversation.ID,
		fmt.Sprintf("mail_api_%s", strs.UUID()),
		enums.IMMessageTypeText,
		content,
		string(payloadBytes),
		externalUser,
	)
	if err != nil {
		return fmt.Errorf("send request message failed: %w", err)
	}
	return nil
}

// sendAcknowledgement is the first thing the visitor sees. It goes out as an
// ordinary channel message so it is queued, retried and delivered through
// whichever provider is configured, SMTP included.
func (s *emailRequestService) sendAcknowledgement(conversation *models.Conversation, cfg *dto.EmailChannelConfig, channel *models.Channel) error {
	text := acknowledgementText(cfg)
	return s.sendChannelNotice(conversation, channel, text, "email_acknowledgement")
}

// sendChatNudge tells the visitor their mail arrived while a chat was open.
func (s *emailRequestService) sendChatNudge(conversation *models.Conversation, channel *models.Channel) error {
	text := i18nx.Getf(i18nx.DefaultLocale, "email.request.chatContinuation")
	return s.sendChannelNotice(conversation, channel, text, "email_chat_nudge")
}

// sendChannelNotice records the notice in the conversation and queues it for the
// email channel. It is attributed to the channel's AI agent when there is one,
// because no human has read the request yet and an agent message with no author
// reads as a fault rather than as an acknowledgement.
//
// The queueing is done explicitly rather than left to the message service, which
// only emails messages belonging to an email conversation. A notice about a chat
// has to reach the visitor's inbox even though the chat lives on another channel.
func (s *emailRequestService) sendChannelNotice(conversation *models.Conversation, channel *models.Channel, text, clientMsgPrefix string) error {
	if conversation == nil || channel == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	operator := &dto.AuthPrincipal{UserID: 0, Username: "system", Nickname: "system"}
	clientMsgID := fmt.Sprintf("%s_%s", clientMsgPrefix, strs.UUID())

	var (
		message *models.Message
		err     error
	)
	if channel.AIAgentID > 0 {
		message, err = MessageService.SendAIMessage(
			conversation.ID, channel.AIAgentID, clientMsgID,
			enums.IMMessageTypeText, text, "", operator,
		)
	} else {
		message, err = MessageService.SendAgentMessage(
			conversation.ID, 0, clientMsgID,
			enums.IMMessageTypeText, text, "", operator,
		)
	}
	if err != nil {
		return err
	}
	return ChannelMessageOutboxService.EnqueueEmailMessageVia(channel, conversation, message)
}

// acknowledgementText prefers the operator's wording for the channel and falls
// back to a localized default, so an integrator can brand the first email.
func acknowledgementText(cfg *dto.EmailChannelConfig) string {
	if cfg != nil {
		if configured := strings.TrimSpace(cfg.WelcomeMessage); configured != "" {
			return configured
		}
	}
	return i18nx.Getf(i18nx.DefaultLocale, "email.request.acknowledgement")
}

// markContactVerification records whether we could verify the address.
//
// A verified contact means the submitting system vouched for it with the channel
// secret; an unverified one is whatever the submitter typed. Agents have to be
// able to tell those apart before acting on the address.
func (s *emailRequestService) markContactVerification(customerID int64, address string, verified bool) error {
	if customerID <= 0 || address == "" {
		return nil
	}
	now := time.Now()
	remark := contactVerificationRemark(verified)

	existing := CustomerContactService.FindOne(sqls.NewCnd().
		Eq("customer_id", customerID).
		Eq("contact_type", enums.ContactTypeEmail).
		Eq("contact_value", address))
	if existing != nil {
		updates := map[string]interface{}{
			"is_verified": verified,
			"remark":      remark,
			"updated_at":  now,
		}
		if verified {
			updates["verified_at"] = &now
		}
		return CustomerContactService.Updates(existing.ID, updates)
	}

	contact := &models.CustomerContact{
		CustomerID:   customerID,
		ContactType:  enums.ContactTypeEmail,
		ContactValue: address,
		IsPrimary:    true,
		IsVerified:   verified,
		Source:       EmailRequestSourceAPI,
		Status:       enums.StatusOk,
		Remark:       remark,
		AuditFields:  utils.BuildAuditFields(nil),
	}
	if verified {
		contact.VerifiedAt = &now
	}
	return CustomerContactService.Create(contact)
}

// contactVerificationRemark is the plain-language marker agents read in the
// dashboard, so an unverified address cannot be mistaken for a proven one.
func contactVerificationRemark(verified bool) string {
	if verified {
		return i18nx.Getf(i18nx.DefaultLocale, "email.contact.verifiedRemark")
	}
	return i18nx.Getf(i18nx.DefaultLocale, "email.contact.unverifiedRemark")
}
