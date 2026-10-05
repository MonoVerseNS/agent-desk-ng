package services

import (
	"testing"
	"time"

	"agent-desk/internal/email"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto"
)

func withEmailConfig(t *testing.T, system config.EmailConfig) {
	t.Helper()
	previous := config.GetCurrent()
	config.SetCurrent(&config.Config{Email: system})
	t.Cleanup(func() { config.SetCurrent(previous) })
}

func boolRef(value bool) *bool { return &value }

func TestResolveEmailDeliverySettingsInheritsSystemDefaults(t *testing.T) {
	withEmailConfig(t, config.EmailConfig{
		SMTPHost:     "smtp.system.example",
		SMTPPort:     2525,
		SMTPUser:     "system-user",
		SMTPPassword: "system-pass",
		SMTPUseTLS:   true,
		FromAddress:  "system@example.com",
		FromName:     "System Support",
	})

	settings := resolveEmailDeliverySettings(&dto.EmailChannelConfig{})

	if settings.SMTPHost != "smtp.system.example" || settings.SMTPPort != 2525 {
		t.Fatalf("smtp endpoint = %s:%d, want the system values", settings.SMTPHost, settings.SMTPPort)
	}
	if settings.SMTPUser != "system-user" || settings.SMTPPassword != "system-pass" {
		t.Fatal("channel with no credentials should inherit the system ones")
	}
	if !settings.SMTPUseTLS {
		t.Fatal("an unset channel TLS flag should inherit the system value")
	}
	if settings.FromEmail != "system@example.com" || settings.FromName != "System Support" {
		t.Fatalf("sender = %s / %s, want the system values", settings.FromEmail, settings.FromName)
	}
}

// The whole reason the flag is a pointer: an explicitly unchecked box must be
// able to turn TLS off for one channel without changing it for everyone else.
func TestResolveEmailDeliverySettingsHonoursExplicitTLSOff(t *testing.T) {
	withEmailConfig(t, config.EmailConfig{SMTPHost: "smtp.system.example", SMTPUseTLS: true})

	settings := resolveEmailDeliverySettings(&dto.EmailChannelConfig{SMTPUseTLS: boolRef(false)})

	if settings.SMTPUseTLS {
		t.Fatal("an explicit smtpUseTls=false must win over the system value")
	}
}

func TestResolveEmailDeliverySettingsPrefersChannelValues(t *testing.T) {
	withEmailConfig(t, config.EmailConfig{
		SMTPHost:    "smtp.system.example",
		SMTPPort:    587,
		FromAddress: "system@example.com",
	})

	settings := resolveEmailDeliverySettings(&dto.EmailChannelConfig{
		SMTPHost:          "smtp.channel.example",
		SMTPPort:          1025,
		EmailAddress:      "help@channel.example",
		SenderName:        "Channel Support",
		SMTPUseTLS:        boolRef(true),
		SMTPAllowInsecure: true,
	})

	if settings.SMTPHost != "smtp.channel.example" || settings.SMTPPort != 1025 {
		t.Fatalf("smtp endpoint = %s:%d, want the channel values", settings.SMTPHost, settings.SMTPPort)
	}
	if !settings.SMTPUseTLS || !settings.AllowInsecure {
		t.Fatal("channel TLS and insecure flags should both carry through")
	}
	if settings.FromEmail != "help@channel.example" || settings.FromName != "Channel Support" {
		t.Fatalf("sender = %s / %s, want the channel values", settings.FromEmail, settings.FromName)
	}
	// Reply-To has to be the channel's own inbound address, or a reply lands in
	// the sending agent's mailbox and the thread ends.
	if settings.ReplyTo != "help@channel.example" {
		t.Fatalf("ReplyTo = %q, want the channel inbound address", settings.ReplyTo)
	}
}

func TestResolveEmailDeliverySettingsReplyToFallsBackToForwardingAddress(t *testing.T) {
	withEmailConfig(t, config.EmailConfig{FromAddress: "system@example.com"})

	settings := resolveEmailDeliverySettings(&dto.EmailChannelConfig{
		ForwardingAddress: "forward@channel.example",
	})

	if settings.ReplyTo != "forward@channel.example" {
		t.Fatalf("ReplyTo = %q, want the forwarding address", settings.ReplyTo)
	}
}

func TestResolveEmailDeliverySettingsDefaultsPortAndSender(t *testing.T) {
	withEmailConfig(t, config.EmailConfig{})

	settings := resolveEmailDeliverySettings(&dto.EmailChannelConfig{})

	if settings.SMTPPort != 587 {
		t.Fatalf("SMTPPort = %d, want the conventional submission default", settings.SMTPPort)
	}
	if settings.Provider != email.ProviderSMTP {
		t.Fatalf("Provider = %q, want smtp", settings.Provider)
	}
	if settings.FromEmail != "support@example.com" || settings.FromName != "Customer Support" {
		t.Fatalf("sender = %s / %s, want usable fallbacks", settings.FromEmail, settings.FromName)
	}
}

func TestResolveEmailDeliverySettingsAllowInsecureIsEitherSide(t *testing.T) {
	withEmailConfig(t, config.EmailConfig{SMTPAllowInsecure: true})
	if !resolveEmailDeliverySettings(&dto.EmailChannelConfig{}).AllowInsecure {
		t.Fatal("a system-wide insecure setting should apply to a channel that sets nothing")
	}

	withEmailConfig(t, config.EmailConfig{})
	if !resolveEmailDeliverySettings(&dto.EmailChannelConfig{SMTPAllowInsecure: true}).AllowInsecure {
		t.Fatal("a channel should be able to opt in on its own")
	}
}

func TestEmailChatLiveWindowDefaultsToThirtyMinutes(t *testing.T) {
	previous := config.GetCurrent()
	t.Cleanup(func() { config.SetCurrent(previous) })

	config.SetCurrent(&config.Config{})
	if got := (config.ConversationConfig{}).EmailChatLiveWindow(); got != 30*time.Minute {
		t.Fatalf("default window = %s, want 30m", got)
	}
	config.SetCurrent(&config.Config{Conversation: config.ConversationConfig{EmailChatLiveMinutes: 5}})
	if got := (config.ConversationConfig{EmailChatLiveMinutes: 5}).EmailChatLiveWindow(); got != 5*time.Minute {
		t.Fatalf("configured window = %s, want 5m", got)
	}
}
