package response

type SetupIssueItem struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	EntityType     string `json:"entity_type"`
	EntityID       int64  `json:"entity_id"`
	EntityName     string `json:"entity_name"`
	QuickFixAction string `json:"quick_fix_action,omitempty"`
}

type SetupStepItem struct {
	Key             string           `json:"key"`
	StepNumber      int              `json:"step_number"`
	Status          string           `json:"status"`
	SummaryCode     string           `json:"summary_code"`
	TotalCount      int              `json:"total_count"`
	ReadyCount      int              `json:"ready_count"`
	Issues          []SetupIssueItem `json:"issues"`
	QuickFixActions []string         `json:"quick_fix_actions"`
	ManagePath      string           `json:"manage_path"`
}

type CurrentOperatorDiagnostic struct {
	UserID                  int64    `json:"user_id"`
	Username                string   `json:"username"`
	Nickname                string   `json:"nickname"`
	HasProfile              bool     `json:"has_profile"`
	ProfileStatusOk         bool     `json:"profile_status_ok"`
	TeamID                  int64    `json:"team_id"`
	TeamName                string   `json:"team_name"`
	TeamActive              bool     `json:"team_active"`
	TeamHasScheduleNow      bool     `json:"team_has_schedule_now"`
	ServiceStatus           int      `json:"service_status"`
	AutoAssignEnabled       bool     `json:"auto_assign_enabled"`
	ActiveConversationCount int64    `json:"active_conversation_count"`
	MaxConcurrentCount      int      `json:"max_concurrent_count"`
	CanReceiveNow           bool     `json:"can_receive_now"`
	BlockerCodes            []string `json:"blocker_codes"`
}

type SetupChannelBrief struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	ChannelType    string `json:"channel_type"`
	ChannelID      string `json:"channel_id"`
	AIAgentID      int64  `json:"ai_agent_id"`
	RolloutPercent int    `json:"rollout_percent"`
	Status         int    `json:"status"`
}

type SetupReadinessResponse struct {
	OverallStatus         string                    `json:"overall_status"`
	ReadySteps            int                       `json:"ready_steps"`
	TotalSteps            int                       `json:"total_steps"`
	CanReceiveRequestsNow bool                      `json:"can_receive_requests_now"`
	Steps                 []SetupStepItem           `json:"steps"`
	CurrentOperator       CurrentOperatorDiagnostic `json:"current_operator"`
	Channels              []SetupChannelBrief       `json:"channels"`
}

type SimulatedRouteHop struct {
	HopKey         string `json:"hop_key"`
	Status         string `json:"status"`
	ReasonCode     string `json:"reason_code"`
	EntityID       int64  `json:"entity_id,omitempty"`
	EntityName     string `json:"entity_name,omitempty"`
	QuickFixAction string `json:"quick_fix_action,omitempty"`
}

type SimulatedOperatorCandidate struct {
	UserID             int64  `json:"user_id"`
	DisplayName        string `json:"display_name"`
	TeamID             int64  `json:"team_id"`
	TeamName           string `json:"team_name"`
	Eligible           bool   `json:"eligible"`
	SkipReasonCode     string `json:"skip_reason_code"`
	ActiveCount        int64  `json:"active_count"`
	MaxConcurrentCount int    `json:"max_concurrent_count"`
	ServiceStatus      int    `json:"service_status"`
	AutoAssignEnabled  bool   `json:"auto_assign_enabled"`
}

type SetupSimulateRouteResponse struct {
	ChannelID          int64                        `json:"channel_id"`
	ChannelName        string                       `json:"channel_name"`
	ChannelType        string                       `json:"channel_type"`
	WillReachOperator  bool                         `json:"will_reach_operator"`
	FinalOutcomeCode   string                       `json:"final_outcome_code"`
	MatchedOperator    *SimulatedOperatorCandidate  `json:"matched_operator,omitempty"`
	Hops               []SimulatedRouteHop          `json:"hops"`
	CandidateOperators []SimulatedOperatorCandidate `json:"candidate_operators"`
}

type SetupQuickFixResponse struct {
	AppliedActions []string               `json:"applied_actions"`
	Readiness      SetupReadinessResponse `json:"readiness"`
}
