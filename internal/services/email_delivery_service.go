package services

import (
	"context"
	"strings"

	"agent-desk/internal/email"
	"agent-desk/internal/pkg/errorsx"
)

var EmailDeliveryService = newEmailDeliveryService()

func newEmailDeliveryService() *emailDeliveryService {
	return &emailDeliveryService{}
}

type emailDeliveryService struct{}

// TestChannelSMTP verifies that a channel's bound mail server is reachable and
// will accept the channel's credentials.
//
// It exists so binding a channel to an SMTP server is a checked action rather
// than a guess. Without it the only way to learn that a host, port or password
// is wrong is to file a real request and watch the outbox fail five times.
//
// The probe goes through the same settings resolution and the same transport
// rules as the sender, so a green probe means the real send would get the same
// treatment.
func (s *emailDeliveryService) TestChannelSMTP(ctx context.Context, channelID int64) (*email.SMTPProbeResult, error) {
	channel := ChannelService.Get(channelID)
	if channel == nil {
		return nil, errorsx.InvalidParamI18n("error.email.channelNotFound")
	}
	channelCfg, err := emailChannelSettings(channel)
	if err != nil {
		return nil, err
	}

	settings := resolveEmailDeliverySettings(channelCfg)
	if settings.Provider != email.ProviderSMTP {
		return nil, errorsx.InvalidParamI18n("error.email.probeRequiresSmtp")
	}
	if strings.TrimSpace(settings.SMTPHost) == "" {
		return nil, errorsx.InvalidParamI18n("error.email.smtpHostMissing")
	}

	result, err := email.ProbeSMTP(ctx, settings.clientConfig())
	if err != nil {
		// The transport detail is the actionable part for whoever bound the
		// server, so it is surfaced rather than flattened into a generic failure.
		return result, errorsx.BusinessErrorI18n(0, "error.email.probeFailed", err.Error())
	}
	return result, nil
}

// TestSystemSMTP probes the deployment-wide mail server, which is what a channel
// with no host of its own will actually use.
func (s *emailDeliveryService) TestSystemSMTP(ctx context.Context) (*email.SMTPProbeResult, error) {
	settings := resolveEmailDeliverySettings(nil)
	if settings.Provider != email.ProviderSMTP {
		return nil, errorsx.InvalidParamI18n("error.email.probeRequiresSmtp")
	}
	if strings.TrimSpace(settings.SMTPHost) == "" {
		return nil, errorsx.InvalidParamI18n("error.email.smtpHostMissing")
	}
	result, err := email.ProbeSMTP(ctx, settings.clientConfig())
	if err != nil {
		return result, errorsx.BusinessErrorI18n(0, "error.email.probeFailed", err.Error())
	}
	return result, nil
}
