package services

import (
	"strings"

	"agent-desk/internal/email"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
)

// emailDeliverySettings is the fully resolved delivery configuration for one
// channel: the channel's own values with the system defaults filled in behind
// anything the channel left unset.
type emailDeliverySettings struct {
	Provider     email.DeliveryProvider
	APIKey       string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPUseTLS   bool
	// AllowInsecure is true when the channel or the deployment is bound to a
	// self-signed test server.
	AllowInsecure bool
	FromEmail     string
	FromName      string
	ReplyTo       string
}

// resolveEmailDeliverySettings is the single place channel and system email
// settings are merged. Sending and the connection probe both go through it, so a
// binding can never verify as reachable while delivering through something else.
func resolveEmailDeliverySettings(channelCfg *dto.EmailChannelConfig) emailDeliverySettings {
	var sysEmail config.EmailConfig
	if current := config.GetCurrent(); current != nil {
		sysEmail = current.Email
	}
	if channelCfg == nil {
		channelCfg = &dto.EmailChannelConfig{}
	}

	provider := email.DeliveryProvider(strings.ToLower(strings.TrimSpace(channelCfg.Provider)))
	if provider == "" || provider == "default" {
		provider = email.DeliveryProvider(strings.ToLower(strings.TrimSpace(sysEmail.Provider)))
	}
	if provider == "" {
		provider = email.ProviderSMTP
	}

	settings := emailDeliverySettings{
		Provider: provider,
		APIKey:   firstNonBlank(channelCfg.APIKey, sysEmail.APIKey),

		SMTPHost:     firstNonBlank(channelCfg.SMTPHost, sysEmail.SMTPHost),
		SMTPPort:     channelCfg.SMTPPort,
		SMTPUser:     firstNonBlank(channelCfg.SMTPUser, sysEmail.SMTPUser),
		SMTPPassword: firstNonBlank(channelCfg.SMTPPassword, sysEmail.SMTPPassword),
		// An explicit per-channel value wins; an absent one inherits the system
		// setting. The pointer is what makes "explicitly off" expressible, which a
		// channel bound to a plaintext test server needs.
		SMTPUseTLS:    sysEmail.SMTPUseTLS,
		AllowInsecure: channelCfg.SMTPAllowInsecure || sysEmail.SMTPAllowInsecure,

		FromEmail: firstNonBlank(channelCfg.EmailAddress, sysEmail.FromAddress),
		FromName:  firstNonBlank(channelCfg.SenderName, sysEmail.FromName, "Customer Support"),
		// Point replies at the channel's own inbound address rather than the
		// mailbox the sending agent happens to authenticate with. Without this a
		// reply lands in that person's inbox instead of back in AgentDesk, and the
		// thread stops being a support channel after the first answer.
		ReplyTo: firstNonBlank(channelCfg.EmailAddress, channelCfg.ForwardingAddress),
	}
	if channelCfg.SMTPUseTLS != nil {
		settings.SMTPUseTLS = *channelCfg.SMTPUseTLS
	}
	if settings.SMTPPort <= 0 {
		settings.SMTPPort = sysEmail.SMTPPort
	}
	if settings.SMTPPort <= 0 {
		settings.SMTPPort = 587
	}
	if settings.FromEmail == "" {
		settings.FromEmail = "support@example.com"
	}
	return settings
}

func (s emailDeliverySettings) clientConfig() email.ClientConfig {
	return email.ClientConfig{
		Provider:          s.Provider,
		APIKey:            s.APIKey,
		SMTPHost:          s.SMTPHost,
		SMTPPort:          s.SMTPPort,
		SMTPUser:          s.SMTPUser,
		SMTPPassword:      s.SMTPPassword,
		SMTPUseTLS:        s.SMTPUseTLS,
		SMTPAllowInsecure: s.AllowInsecure,
	}
}

// emailChannelSettings loads a channel's parsed email config.
func emailChannelSettings(channel *models.Channel) (*dto.EmailChannelConfig, error) {
	if channel == nil {
		return nil, errorsx.InvalidParamI18n("error.email.channelNotFound")
	}
	if channel.ChannelType != enums.ChannelTypeEmail || channel.Status != enums.StatusOk {
		return nil, errorsx.InvalidParamI18n("error.email.channelNotFound")
	}
	cfg, err := ChannelService.ParseEmailChannelConfig(channel.ConfigJSON)
	if err != nil || cfg == nil {
		return nil, errorsx.InvalidParamI18n("error.email.channelConfigInvalid")
	}
	return cfg, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
