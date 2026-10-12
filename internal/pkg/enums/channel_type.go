package enums

type ChannelMessageOutboxStatus string

const (
	ChannelMessageOutboxStatusPending ChannelMessageOutboxStatus = "pending"
	ChannelMessageOutboxStatusSending ChannelMessageOutboxStatus = "sending"
	ChannelMessageOutboxStatusSent    ChannelMessageOutboxStatus = "sent"
	ChannelMessageOutboxStatusFailed  ChannelMessageOutboxStatus = "failed"
	ChannelMessageOutboxStatusIgnored ChannelMessageOutboxStatus = "ignored"
)

const (
	ChannelTypeWeb      = "web"
	ChannelTypeTelegram = "telegram"
	ChannelTypeZaloOA   = "zalo_oa"
	ChannelTypeSlack    = "slack"
	ChannelTypeDiscord  = "discord"
	ChannelTypeEmail    = "email"
)
