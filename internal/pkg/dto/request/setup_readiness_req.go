package request

type SetupSimulateRouteRequest struct {
	ChannelID int64 `json:"channel_id" form:"channel_id"`
	BypassAI  bool  `json:"bypass_ai" form:"bypass_ai"`
}

type SetupQuickFixRequest struct {
	Action   string `json:"action" form:"action"`
	EntityID int64  `json:"entity_id" form:"entity_id"`
}
