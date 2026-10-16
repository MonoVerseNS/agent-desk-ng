# Step-by-Step Setup & Operator Routing Readiness Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete step-by-step setup wizard, operator routing readiness diagnostic, live route simulator, and one-click quick-fix system across backend and frontend in all three locales (`en-US`, `ru-RU`, `zh-CN`).

**Architecture:** A dedicated `SetupReadinessService` in Go evaluates the 6-step customer-to-operator routing chain (`AgentTeam` -> `AgentTeamSchedule` -> `AgentProfile` -> `AIAgent` -> `Channel` -> End-to-End path), simulates channel dispatch dry-runs, and executes atomic one-click fixes. The Next.js frontend exposes this through a dedicated `/dashboard/setup` page, a setup banner on `/dashboard`, a sidebar navigation entry, and an interactive operator readiness indicator in `WorkbenchHeader` and `SiteHeader`.

**Tech Stack:** Go 1.26, Gin, GORM (`github.com/mlogclub/simple/sqls`), Next.js 16 App Router, React 19, TypeScript, Tailwind CSS, Base UI / shadcn primitives.

**Spec:** `docs/superpowers/specs/2026-10-16-onboarding-readiness-wizard-design.md`

## Global Constraints

- Follow `models -> repositories -> services -> handlers -> builders/response DTOs` layer ownership (`AGENTS.md` 4.1).
- Use `any` instead of `interface{}` in new Go code (`AGENTS.md` 3).
- Support all 3 locales on both backend (`internal/pkg/i18nx/locales/{en-US,ru-RU,zh-CN}.yml`) and frontend (`web/messages/{en-US,ru-RU,zh-CN}.json`).
- Route all frontend API calls through `web/lib/api/client.ts` (`AGENTS.md` 5.1).
- Use `rounded-md` and no `shadow` on support/dashboard cards, and use `OptionCombobox` for dropdowns (`AGENTS.md` 5.1, 5.4).

## Review Focus

1. **Off-hours schedule false-positive:** `AgentTeamSchedule` exists for a team, but its `StartAt`/`EndAt` window does not cover `time.Now()` -> `schedules` step must return `"warning"` with `no_schedule_now` and `can_receive_requests_now = false`.
2. **AI-Only mode blocking human handoff:** All active `AIAgent` records have `ServiceMode == IMConversationServiceModeAIOnly` -> `ai_agents` step must flag `"agent_ai_only_mode"` and simulator must report `"blocked_ai_only_mode"`.
3. **Empty `AIAgent.TeamIDs` or deleted teams:** `AIAgent.TeamIDs` is `""` or references only disabled/non-existent team IDs -> `ai_agents` step must flag `"agent_no_teams"` and `fix_ai_agent_handoff` must bind valid active team IDs.
4. **Operator profile busy or `AutoAssignEnabled == false`:** Operator is in an active team with an active schedule, but `ServiceStatus == ServiceStatusBusy` or `AutoAssignEnabled == false` -> `current_operator.can_receive_now` must be `false` with exact `blocker_codes` (`status_busy` / `auto_assign_disabled`), and `enable_my_operator_profile` must restore `ServiceStatusIdle` and `AutoAssignEnabled = true`.
5. **Idempotent `quick_setup_all` on partially configured database:** Running `quick_setup_all` twice in a row must not duplicate default teams, channels, or schedules when valid active ones already exist.

---

### Task 1: Backend DTOs, Locales, and SetupReadinessService (with TDD)

**Files:**
- Create: `internal/pkg/dto/request/setup_readiness_req.go`
- Create: `internal/pkg/dto/response/setup_readiness_resp.go`
- Create: `internal/services/setup_readiness_service.go`
- Create: `internal/services/setup_readiness_service_test.go`
- Modify: `internal/pkg/i18nx/locales/en-US.yml`
- Modify: `internal/pkg/i18nx/locales/ru-RU.yml`
- Modify: `internal/pkg/i18nx/locales/zh-CN.yml`

**Interfaces:**
- Produces:
  - `request.SetupSimulateRouteRequest { ChannelID int64; BypassAI bool }`
  - `request.SetupQuickFixRequest { Action string; EntityID int64 }`
  - `response.SetupReadinessResponse`, `response.SetupSimulateRouteResponse`, `response.SetupQuickFixResponse`
  - `services.SetupReadinessService.GetReadiness(operator *dto.AuthPrincipal) (*response.SetupReadinessResponse, error)`
  - `services.SetupReadinessService.SimulateRoute(req request.SetupSimulateRouteRequest, operator *dto.AuthPrincipal) (*response.SetupSimulateRouteResponse, error)`
  - `services.SetupReadinessService.ApplyQuickFix(req request.SetupQuickFixRequest, operator *dto.AuthPrincipal) (*response.SetupQuickFixResponse, error)`

- [ ] **Step 1: Write failing tests in `internal/services/setup_readiness_service_test.go`**
  Cover:
  1. `TestSetupReadiness_EmptyDBAndQuickSetupAll` (verifies all 6 steps fail on empty DB, then `ApplyQuickFix("quick_setup_all")` makes all 6 steps `"ok"`, `CanReceiveRequestsNow == true`, `CurrentOperator.CanReceiveNow == true`, and a second call is idempotent).
  2. `TestSetupReadiness_OffHoursAndBusyOperator` (verifies schedule outside `now` triggers `no_schedule_now` and `team_off_hours`, and busy/auto-assign-off operator triggers `status_busy`/`auto_assign_disabled`, then `enable_my_operator_profile` and `ensure_team_and_schedule` resolve them).
  3. `TestSetupReadiness_SimulateRoute_AIOnlyAndMissingTeams` (verifies `SimulateRoute` reports `blocked_ai_only_mode` when `ServiceMode == IMConversationServiceModeAIOnly`, `blocked_agent_no_teams` when `TeamIDs == ""`, and `assigned_to_operator` with matched operator after `fix_ai_agent_handoff`).

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/services -run TestSetupReadiness`
  Expected: FAIL (missing types and `SetupReadinessService`)

- [ ] **Step 3: Implement DTOs, backend locale keys, and `SetupReadinessService`**
  Create `internal/pkg/dto/request/setup_readiness_req.go`, `internal/pkg/dto/response/setup_readiness_resp.go`, and `internal/services/setup_readiness_service.go`. Add `error.setup.*` keys to `en-US.yml`, `ru-RU.yml`, and `zh-CN.yml`.

- [ ] **Step 4: Run tests to verify they pass**
  Run: `go test -v ./internal/services -run TestSetupReadiness`
  Expected: PASS

---

### Task 2: Backend HTTP Handlers and Route Registration

**Files:**
- Create: `internal/handlers/dashboard/setup_readiness_handler.go`
- Modify: `internal/bootstrap/routes.go`
- Modify: `internal/bootstrap/server.go`

**Interfaces:**
- Consumes: `services.SetupReadinessService` from Task 1.
- Produces:
  - `GET /api/dashboard/setup/readiness` (`dashboard.SetupGetReadiness`)
  - `POST /api/dashboard/setup/simulate_route` (`dashboard.SetupPostSimulate_route`)
  - `POST /api/dashboard/setup/quick_fix` (`dashboard.SetupPostQuick_fix`)

- [ ] **Step 1: Create `internal/handlers/dashboard/setup_readiness_handler.go`**
  Implement `SetupGetReadiness`, `SetupPostSimulate_route`, and `SetupPostQuick_fix` using `services.AuthService.GetCurrentPrincipal(ctx)` and `httpx.WriteJSON`.

- [ ] **Step 2: Register routes in `internal/bootstrap/routes.go` and `internal/bootstrap/server.go`**
  Add `registerDashboardSetupRoutes(dashboardGroup.Group("/setup"))` in `server.go` and define `registerDashboardSetupRoutes` in `routes.go`.

- [ ] **Step 3: Run `gofmt` and Go build/tests**
  Run: `gofmt -w internal/pkg/dto/request/setup_readiness_req.go internal/pkg/dto/response/setup_readiness_resp.go internal/services/setup_readiness_service.go internal/services/setup_readiness_service_test.go internal/handlers/dashboard/setup_readiness_handler.go internal/bootstrap/routes.go internal/bootstrap/server.go && go test ./internal/services -run TestSetupReadiness`
  Expected: PASS with zero compile/test errors.

---

### Task 3: Frontend API Client & 3-Locale Translations (`en-US`, `ru-RU`, `zh-CN`)

**Files:**
- Create: `web/lib/api/setup-readiness.ts`
- Modify: `web/messages/en-US.json`
- Modify: `web/messages/ru-RU.json`
- Modify: `web/messages/zh-CN.json`
- Modify: `web/lib/navigation.tsx`

**Interfaces:**
- Produces:
  - `fetchSetupReadiness(): Promise<SetupReadinessResponse>`
  - `simulateSetupRoute(payload: { channel_id?: number; bypass_ai?: boolean }): Promise<SetupSimulateRouteResponse>`
  - `applySetupQuickFix(payload: { action: SetupQuickFixAction; entity_id?: number }): Promise<SetupQuickFixResponse>`
  - `nav.setupWizard` and `setupWizard.*` translation keys in `en-US.json`, `ru-RU.json`, and `zh-CN.json`
  - Navigation entry `/dashboard/setup` in `web/lib/navigation.tsx`

- [ ] **Step 1: Create `web/lib/api/setup-readiness.ts`**
  Define exact TypeScript interfaces matching `SetupReadinessResponse`, `SetupSimulateRouteResponse`, and `SetupQuickFixResponse`, using `requestData` from `@/lib/api/client`.

- [ ] **Step 2: Add `/dashboard/setup` to `web/lib/navigation.tsx`**
  Add `{ titleKey: "nav.setupWizard", url: "/dashboard/setup", icon: <ListChecksIcon /> }` to `dashboardNavSections` under Reception Center (`nav.receptionCenter`) right after Overview, and add page title mapping in `getPageTitleKey`.

- [ ] **Step 3: Add complete `setupWizard` and `nav.setupWizard` translations in `en-US.json`, `ru-RU.json`, and `zh-CN.json`**
  Delegate translation generation/verification to a **Gemini Flash** subagent (`model: "flash"`) and apply matching keys across all three JSON files.

---

### Task 4: Frontend Setup Wizard Page, Dashboard Banner, and Operator Readiness Popover

**Files:**
- Create: `web/app/(dashboard)/dashboard/setup/page.tsx`
- Create: `web/app/(dashboard)/dashboard/setup/_components/setup-wizard-view.tsx`
- Create: `web/app/(dashboard)/dashboard/_components/setup-readiness-banner.tsx`
- Create: `web/components/operator-readiness-indicator.tsx`
- Modify: `web/app/(dashboard)/dashboard/_components/dashboard-home.tsx`
- Modify: `web/components/workbench-header.tsx`
- Modify: `web/components/site-header.tsx`

**Interfaces:**
- Consumes: `web/lib/api/setup-readiness.ts` and `setupWizard.*` i18n keys from Task 3.
- Produces:
  - `/dashboard/setup` page with 6-step interactive checklist, current operator readiness card, 1-click quick fixes, and live route simulator with candidate operator table.
  - `SetupReadinessBanner` displayed at the top of `DashboardHome` whenever any step is not `"ok"` or current operator cannot receive chats.
  - `OperatorReadinessIndicator` displayed in `WorkbenchHeader` and `SiteHeader` showing live operator readiness status and 1-click fix popover.

- [ ] **Step 1: Build `OperatorReadinessIndicator` (`web/components/operator-readiness-indicator.tsx`)**
  Compact header button + popover showing whether customer requests can reach the current operator right now, listing specific blockers (`missing_profile`, `no_team`, `team_off_hours`, `status_busy`, `auto_assign_disabled`, `at_capacity`, or global chain issues), with a 1-click **"Enable Chat Reception"** (`enable_my_operator_profile` / `quick_setup_all`) button and link to `/dashboard/setup`.

- [ ] **Step 2: Build `SetupReadinessBanner` (`web/app/(dashboard)/dashboard/_components/setup-readiness-banner.tsx`)**
  Alert card on `/dashboard` home page when setup is incomplete or current operator is blocked, showing `ready_steps / total_steps`, broken step badges, 1-click **"Fix Automatically"** button, and **"Open Step-by-Step Setup"** button.

- [ ] **Step 3: Build `/dashboard/setup` page and `SetupWizardView` (`web/app/(dashboard)/dashboard/setup/_components/setup-wizard-view.tsx`)**
  Implement the 6-step readiness checklist with per-step issues, 1-click quick-fix actions, management links (`/dashboard/agents`, `/dashboard/agent-team-schedules`, `/dashboard/ai-agents`, `/dashboard/channels`), current operator status card, and the interactive Route Simulator (`OptionCombobox` for channel selection, 5-hop visual pipeline, and candidate operator table).

- [ ] **Step 4: Wire into `DashboardHome`, `WorkbenchHeader`, and `SiteHeader`**
  Mount `<SetupReadinessBanner />` in `web/app/(dashboard)/dashboard/_components/dashboard-home.tsx`, and mount `<OperatorReadinessIndicator />` in `web/components/workbench-header.tsx` and `web/components/site-header.tsx`.

- [ ] **Step 5: Run TypeScript check, ESLint, Go tests, and Snyk security scan**
  Run:
  - `cd web && pnpm typecheck`
  - `cd web && pnpm lint`
  - `go test ./internal/services/...`
  - Snyk code scan (`snyk_code_scan`) on changed files.
