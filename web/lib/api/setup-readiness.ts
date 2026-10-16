import { request } from "@/lib/api/client"

export type SetupStepStatus = "ok" | "warning" | "error"

export type SetupQuickFixAction =
  | "quick_setup_all"
  | "ensure_team_and_schedule"
  | "enable_my_operator_profile"
  | "fix_ai_agent_handoff"
  | "ensure_default_channel"

export type SetupIssueItem = {
  code: string
  severity: "warning" | "error"
  entity_type: "team" | "schedule" | "operator" | "ai_agent" | "channel" | "system"
  entity_id: number
  entity_name: string
  quick_fix_action?: SetupQuickFixAction
}

export type SetupStepItem = {
  key: "teams" | "schedules" | "operators" | "ai_agents" | "channels" | "end_to_end"
  step_number: number
  status: SetupStepStatus
  summary_code: string
  total_count: number
  ready_count: number
  issues: SetupIssueItem[]
  quick_fix_actions: SetupQuickFixAction[]
  manage_path: string
}

export type CurrentOperatorDiagnostic = {
  user_id: number
  username: string
  nickname: string
  has_profile: boolean
  profile_status_ok: boolean
  team_id: number
  team_name: string
  team_active: boolean
  team_has_schedule_now: boolean
  service_status: number
  auto_assign_enabled: boolean
  active_conversation_count: number
  max_concurrent_count: number
  can_receive_now: boolean
  blocker_codes: string[]
}

export type SetupChannelBrief = {
  id: number
  name: string
  channel_type: string
  channel_id: string
  ai_agent_id: number
  rollout_percent: number
  status: number
}

export type SetupReadinessResponse = {
  overall_status: SetupStepStatus
  ready_steps: number
  total_steps: number
  can_receive_requests_now: boolean
  steps: SetupStepItem[]
  current_operator: CurrentOperatorDiagnostic
  channels: SetupChannelBrief[]
}

export type SimulatedRouteHop = {
  hop_key: "channel" | "ai_agent" | "teams" | "schedules" | "operators"
  status: SetupStepStatus
  reason_code: string
  entity_id?: number
  entity_name?: string
  quick_fix_action?: SetupQuickFixAction
}

export type SimulatedOperatorCandidate = {
  user_id: number
  display_name: string
  team_id: number
  team_name: string
  eligible: boolean
  skip_reason_code: string
  active_count: number
  max_concurrent_count: number
  service_status: number
  auto_assign_enabled: boolean
}

export type SetupSimulateRouteResponse = {
  channel_id: number
  channel_name: string
  channel_type: string
  will_reach_operator: boolean
  final_outcome_code: string
  matched_operator?: SimulatedOperatorCandidate
  hops: SimulatedRouteHop[]
  candidate_operators: SimulatedOperatorCandidate[]
}

export type SetupQuickFixResponse = {
  applied_actions: string[]
  readiness: SetupReadinessResponse
}

export function fetchSetupReadiness() {
  return request<SetupReadinessResponse>("/api/dashboard/setup/readiness")
}

export function simulateSetupRoute(payload: {
  channel_id?: number
  bypass_ai?: boolean
}) {
  return request<SetupSimulateRouteResponse>(
    "/api/dashboard/setup/simulate_route",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
  )
}

export function applySetupQuickFix(payload: {
  action: SetupQuickFixAction
  entity_id?: number
}) {
  return request<SetupQuickFixResponse>("/api/dashboard/setup/quick_fix", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}
