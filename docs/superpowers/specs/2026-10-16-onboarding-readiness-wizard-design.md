# Step-by-Step Setup & Operator Routing Readiness Wizard Design

**Date:** 2026-10-16  
**Topic:** Step-by-Step Onboarding Setup, Readiness Diagnostics, Route Simulator, and One-Click Quick Fixes (`en-US`, `ru-RU`, `zh-CN`)

---

## 1. Problem Statement & User Intent

When deploying or configuring `agent-desk-ng`, administrators and operators must configure six interdependent entities for customer requests to reach a human operator:
1. **Operator Teams (`AgentTeam`)**: At least one active team (`Status == StatusOk`).
2. **Working Schedules (`AgentTeamSchedule`)**: At least one active schedule (`Status == StatusOk`) covering the current timestamp (`start_at <= now < end_at`) for the target team. Without an active shift, conversations stay in `Pending` and return an off-hours message.
3. **Operator Profiles (`AgentProfile` & `User`)**: An active user (`User.Status == StatusOk`) with an active `AgentProfile` (`Status == StatusOk`), assigned to the active team, with `AutoAssignEnabled == true`, `ServiceStatus == ServiceStatusIdle (0)`, `MaxConcurrentCount > 0`, and available capacity (`activeCount < MaxConcurrentCount`).
4. **AI Agent & Handoff Mode (`AIAgent`)**: An active `AIAgent` (`Status == StatusOk`) whose `ServiceMode` allows reaching a human (`HumanOnly` or `AIFirst`), and whose `TeamIDs` field references at least one valid, active `AgentTeam`.
5. **Inbound Channels (`Channel`)**: At least one active `Channel` (`Status == StatusOk`) linked to a valid `AIAgentID` with `AIAgentRolloutPercent > 0`.
6. **End-to-End Routing Path**: A complete, unbroken path from an active `Channel` -> active `AIAgent` (with handoff enabled) -> active `AgentTeam` -> currently active `AgentTeamSchedule` -> at least one eligible, idle `AgentProfile` with free capacity.

Currently, if any single link in this chain is missing or misconfigured (e.g., missing shift schedule for the current time, empty `TeamIDs` on the AI agent, operator `AutoAssignEnabled == false` or status `Busy`, or `AIOnly` service mode), requests silently fail to reach the operator without explaining why.

### Goals
1. **Step-by-Step Readiness Check**: Inspect all 6 steps of the operator routing chain and report exact status (`ok`, `warning`, `error`), specific blocking reasons, and actionable fixes.
2. **Current Operator Diagnostic**: Tell the currently logged-in operator whether they personally can receive customer conversations right now, and if not, why (missing profile, no team, team off-hours, busy status, auto-assign disabled, at capacity).
3. **Interactive Route Simulator ("Why isn't my request reaching an operator?")**: Dry-run the actual dispatch evaluation for a selected channel (or all channels) and visualize every hop from Channel -> AI Agent -> Team -> Shift Schedule -> Candidate Operators -> Winning Operator (or exact failure point).
4. **One-Click Quick Fixes**: Provide safe, atomic backend actions to fix common blockers in one click (create/enable default team & 24/7 shift schedule, activate current user's operator profile and set idle, bind teams & handoff mode on AI agents, create/link default web channel, or run a complete "Quick Setup All" action).
5. **Multi-Surface UI**:
   - Dedicated `/dashboard/setup` page with Step-by-Step Wizard and Route Simulator.
   - Setup Readiness Banner on `/dashboard` Overview page whenever any step is incomplete or degraded.
   - Sidebar navigation item under Dashboard.
   - Operator Readiness status indicator & popover with 1-click fix inside the `/support` operator workbench header.
6. **Full Localization**: Complete support across all three repository locales: `en-US`, `ru-RU`, and `zh-CN` (both backend and frontend).

---

## 2. Backend Architecture

Following `AGENTS.md` layer ownership (`models -> repositories -> services -> handlers -> builders/response DTOs`):

### 2.1 Endpoints (`internal/bootstrap/routes.go`)
Mounted under `/api/dashboard/setup`:
- `GET /api/dashboard/setup/readiness` -> `SetupReadinessHandler.GetReadiness`
- `POST /api/dashboard/setup/simulate_route` -> `SetupReadinessHandler.PostSimulateRoute`
- `POST /api/dashboard/setup/quick_fix` -> `SetupReadinessHandler.PostQuickFix`

Permissions:
- `GET /api/dashboard/setup/readiness` and `POST /api/dashboard/setup/simulate_route` and `POST /api/dashboard/setup/quick_fix` are available to authenticated dashboard/support users so both administrators on `/dashboard` and operators on `/support` can diagnose and fix their readiness. Write actions that modify global resources check appropriate permissions or allow self-profile/bootstrap setup safely.

### 2.2 Request & Response DTOs (`internal/pkg/dto/request/setup_readiness_req.go`, `internal/pkg/dto/response/setup_readiness_resp.go`)

#### `SetupReadinessResponse`
- `overall_status`: `"ok" | "warning" | "error"`
- `ready_steps`: `int`
- `total_steps`: `int` (6)
- `can_receive_requests_now`: `bool`
- `steps`: `[]SetupStepItem`
  - `key`: `"teams" | "schedules" | "operators" | "ai_agents" | "channels" | "end_to_end"`
  - `step_number`: `1..6`
  - `status`: `"ok" | "warning" | "error"`
  - `summary_code`: machine-readable reason code (e.g. `"ok"`, `"no_active_team"`, `"no_schedule_now"`, `"no_idle_operator"`, `"agent_ai_only"`, `"agent_missing_teams"`, `"no_active_channel"`, `"no_end_to_end_path"`)
  - `total_count`: `int`
  - `ready_count`: `int`
  - `issues`: `[]SetupIssueItem`
    - `code`: string (e.g. `"team_no_active_schedule"`, `"agent_ai_only_mode"`, `"agent_no_teams"`, `"operator_busy"`, `"operator_auto_assign_off"`, `"operator_no_team"`, `"operator_at_capacity"`, `"channel_no_agent"`, `"channel_zero_rollout"`)
    - `severity`: `"warning" | "error"`
    - `entity_type`: `"team" | "schedule" | "operator" | "ai_agent" | "channel" | "system"`
    - `entity_id`: `int64`
    - `entity_name`: `string`
    - `quick_fix_action`: `string` (optional action key)
  - `quick_fix_actions`: `[] string` (e.g. `["ensure_team_and_schedule"]`)
  - `manage_path`: `string` (e.g. `"/dashboard/agent-team"`)
- `current_operator`: `CurrentOperatorDiagnostic`
  - `user_id`: `int64`
  - `username`: `string`
  - `nickname`: `string`
  - `has_profile`: `bool`
  - `profile_status_ok`: `bool`
  - `team_id`: `int64`
  - `team_name`: `string`
  - `team_active`: `bool`
  - `team_has_schedule_now`: `bool`
  - `service_status`: `int` (`0` = Idle, `1` = Busy)
  - `auto_assign_enabled`: `bool`
  - `active_conversation_count`: `int64`
  - `max_concurrent_count`: `int`
  - `can_receive_now`: `bool`
  - `blocker_codes`: `[]string` (e.g. `["missing_profile", "no_team", "team_off_hours", "status_busy", "auto_assign_disabled", "at_capacity"]`)

#### `SetupSimulateRouteRequest` & `SetupSimulateRouteResponse`
- Request:
  - `channel_id`: `int64` (optional; if `0`, picks the first active channel or simulates all active channels)
  - `bypass_ai`: `bool` (optional; simulates what happens when customer requests a human / handoff triggers)
- Response:
  - `channel_id`: `int64`
  - `channel_name`: `string`
  - `channel_type`: `string`
  - `will_reach_operator`: `bool`
  - `final_outcome_code`: `"assigned_to_operator" | "blocked_no_channel" | "blocked_channel_disabled" | "blocked_zero_rollout" | "blocked_missing_ai_agent" | "blocked_ai_only_mode" | "queued_ai_first_requires_handoff" | "blocked_agent_no_teams" | "queued_off_hours_no_schedule" | "queued_no_eligible_operator"`
  - `matched_operator`: `*SimulatedOperatorCandidate`
  - `hops`: `[]SimulatedRouteHop`
    - `hop_key`: `"channel" | "ai_agent" | "teams" | "schedules" | "operators"`
    - `status`: `"ok" | "warning" | "error"`
    - `reason_code`: string
    - `entity_name`: string
    - `quick_fix_action`: string
  - `candidate_operators`: `[]SimulatedOperatorCandidate`
    - `user_id`: `int64`
    - `display_name`: `string`
    - `team_id`: `int64`
    - `team_name`: `string`
    - `eligible`: `bool`
    - `skip_reason_code`: `"" | "profile_disabled" | "user_disabled" | "team_not_in_agent" | "team_off_hours" | "auto_assign_off" | "status_busy" | "at_capacity"`
    - `active_count`: `int64`
    - `max_concurrent_count`: `int`
    - `service_status`: `int`
    - `auto_assign_enabled`: `bool`

#### `SetupQuickFixRequest` & `SetupQuickFixResponse`
- Request:
  - `action`:
    - `"quick_setup_all"` — ensures an active team exists, ensures a 24/7 active shift schedule exists for all active teams without a current schedule, ensures the current user has an active idle `AgentProfile` in an active team with `AutoAssignEnabled = true`, ensures at least one active `AIAgent` is linked to the active teams and allows human handoff (`AIFirst` with `AutoHandoffOnFailure = true` or `HumanOnly`), and ensures at least one active Web `Channel` is linked to that `AIAgent` with `RolloutPercent = 100`.
    - `"ensure_team_and_schedule"` — creates a default team if none exists, and creates an active shift schedule (`now - 1h` to `now + 365d`) for any active team lacking a currently active schedule.
    - `"enable_my_operator_profile"` — creates or updates the current user's `AgentProfile`: assigns to first active team (creating one + schedule if needed), sets `Status = StatusOk`, `AutoAssignEnabled = true`, `ServiceStatus = ServiceStatusIdle`, `MaxConcurrentCount = max(existing, 10)`. Also triggers pending conversation dispatch.
    - `"fix_ai_agent_handoff"` — for `entity_id` (or all active AI agents): binds all active `TeamIDs` if empty, switches `AIOnly` to `AIFirst` (or `HumanOnly` if no model is configured), sets `AutoHandoffOnFailure = true`, and ensures `Status = StatusOk`. If no `AIAgent` exists at all, creates a default `HumanOnly` reception agent bound to the active team.
    - `"ensure_default_channel"` — if no active channel exists, creates a default Web channel linked to an active `AIAgent` with `AIAgentRolloutPercent = 100`; if channels exist with missing `AIAgentID` or `0%` rollout, fixes them.
  - `entity_id`: `int64` (optional target ID)
- Response:
  - `applied_actions`: `[]string`
  - `readiness`: `SetupReadinessResponse` (fresh snapshot after fix)

---

## 3. Frontend Architecture

### 3.1 API Module (`web/lib/api/setup-readiness.ts`)
- Typed client functions calling `/api/dashboard/setup/readiness`, `/api/dashboard/setup/simulate_route`, and `/api/dashboard/setup/quick_fix` via `web/lib/api/client.ts`.

### 3.2 Dedicated Setup & Diagnostics Page (`web/app/(dashboard)/dashboard/setup/page.tsx`)
- Uses `DashboardPage` layout and standard `rounded-md` border styling without shadows (`AGENTS.md` 5.4).
- **Top Readiness Header Card**:
  - Shows overall status badge (`Ready` / `Needs Attention`), progress bar (`X / 6 steps ready`), current operator status summary, and primary CTA **"Quick Setup All in 1 Click"** (`quick_setup_all`) + **"Refresh Check"**.
- **Two Main Sections**:
  1. **Step-by-Step Setup Checklist (Steps 1 to 6)**:
     - Renders each of the 6 steps in logical order:
       1. Operator Teams
       2. Working Hours & Active Shift Schedules
       3. Operator Profiles & Online Availability
       4. AI Agent & Handoff Configuration
       5. Inbound Channels
       6. End-to-End Dispatch Verification
     - Each step displays its status badge, counts (`ready / total`), specific detected issues (with entity names and localized explanations), a **1-Click Quick Fix** button when applicable, and a secondary **"Open Settings"** link to the corresponding management CRUD page.
  2. **Interactive Route Simulator ("Why isn't a request reaching an operator?")**:
     - Allows selecting any channel via `OptionCombobox` (or auto-selects the first channel) and running the live route trace.
     - Displays the 5-hop visual pipeline (`Channel` -> `AI Agent` -> `Target Teams` -> `Active Schedule` -> `Operator Selection`) with status indicators and inline 1-click fix buttons on any failing hop.
     - Displays the **Candidate Operators Table** showing every operator profile, their team, load (`active / max`), status (`Idle` / `Busy`), auto-assign state, and exact reason why they were selected or skipped.

### 3.3 Dashboard Home Readiness Banner (`web/app/(dashboard)/dashboard/_components/setup-readiness-banner.tsx`)
- Rendered at the top of `DashboardHome` (`web/app/(dashboard)/dashboard/_components/dashboard-home.tsx`).
- When `can_receive_requests_now == false` or `overall_status != "ok"` (or when the current operator cannot receive requests), shows a clear alert banner listing the broken steps with:
  - **"Fix Automatically in 1 Click"** button
  - **"Open Step-by-Step Setup"** button linking to `/dashboard/setup`

### 3.4 Operator Workbench Readiness Indicator (`web/app/(support)/support/_components/operator-readiness-Button.tsx` / Header integration)
- Integrated into the `/support` top bar (`web/app/(support)/support/page.tsx` or header component).
- Shows a compact badge indicating whether the operator is currently able to receive incoming customer chats:
  - Green (`Ready to Receive Chats`) when `current_operator.can_receive_now && readiness.can_receive_requests_now`.
  - Amber/Red (`Routing Blocked: N issues`) when blocked.
- Clicking opens a popover showing:
  - Why requests are not reaching this operator right now (e.g., *"Your team has no active shift schedule right now (Off-Hours)"*, *"Your operator profile is Busy"*, *"Auto-assign is disabled"*, *"AI Agent has no team assigned"*).
  - **"Enable Chat Reception in 1 Click"** button (`enable_my_operator_profile` / `quick_setup_all`).
  - Link to `/dashboard/setup` for full diagnostics.

### 3.5 Navigation (`web/components/app-sidebar.tsx`)
- Adds a navigation item for `/dashboard/setup` under the Overview section so users can always find the step-by-step setup wizard.

---

## 4. Localization (`en-US`, `ru-RU`, `zh-CN`)

- All backend error messages in `internal/pkg/i18nx/locales/{en-US,ru-RU,zh-CN}.yml`.
- All frontend UI copy, step titles, issue descriptions, simulator hop reasons, candidate skip reasons, and action labels in `web/messages/{en-US,ru-RU,zh-CN}.json`.
- Subagents running **Gemini Flash** (`model: "flash"`) will be used to prepare and verify translations and assist with parallel inspections.

---

## 5. Testing & Verification

1. **Backend Unit Tests (`internal/services/setup_readiness_service_test.go`)**:
   - Empty database -> `readiness` reports missing steps (`teams`, `schedules`, `operators`, `ai_agents`, `channels`, `end_to_end`).
   - Running `quick_Fix("quick_setup_all")` -> creates/fixes the chain atomically so `can_receive_requests_now` becomes `true` and all 6 steps become `ok`.
   - Route simulator (`SimulateRoute`) accurately identifies off-hours schedule blocks, `AIOnly` blocks, `Busy` operator blocks, and selects the winning operator when ready.
2. **Code Formatting & Static Checks**:
   - `gofmt` and `go test ./internal/services/...`
   - `cd web && pnpm typecheck` and `cd web && pnpm lint`
   - Locale parity check across `en-US.json`, `ru-RU.json`, and `zh-CN.json`.
   - Security scan via Snyk MCP (`snyk_code_scan`) per global rules.
