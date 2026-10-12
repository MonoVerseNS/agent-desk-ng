# Fix Agent Tool Naming, Prompt Catalog, Timezone, Channel Build Arg, and WebSocket Reconnect Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve five production issues across AI runtime tool execution, prompt capability catalogs, timestamp timezone handling, Docker image channel building, and frontend WebSocket reconnection.

**Architecture:** 
Sanitize wire function names in the AI agent loop so providers accepting only `^[a-zA-Z0-9_-]+$` succeed while mapping back to canonical codes on execution.
Align prompt catalog strings with direct function names and explicit instructions.
Normalize timestamps to UTC when timezone offsets are missing and emit RFC3339 from the backend.
Support `NEXT_PUBLIC_OPEN_IM_CHANNEL_ID` build argument in Dockerfile and ensure `beforeConnect` token validation precedes WebSocket connections.

**Tech Stack:** Go 1.26, Next.js 16 (React 19, TypeScript), WebSocket, Docker

**Spec:** User prompt requirements (5 targeted fixes)

## Global Constraints

- Preserve all existing staged and user-owned worktree changes.
- Follow layer boundaries: models -> repositories -> services -> handlers.
- Portable SQL / SQLite & MySQL compatibility.
- Format all Go files with `gofmt`.
- TypeScript checks must pass with `cd web && pnpm typecheck`.
- Maintain test coverage across changed backend and frontend surfaces.

## Review Focus

- Function name sanitization must only alter the wire definition, preserving registry, whitelist, and audit codes.
- Direct invocation and tool_search calls must both resolve to the correct capability code.
- Timestamp parsing must treat strings lacking timezone offsets as UTC while keeping explicit offsets untouched.
- Reconnection lifecycle must not create duplicate sockets or run stale connection loops when disconnected mid-flight.
- Docker ARG must properly inject into the Next.js builder stage.

---

### Task 1: Sanitize wire tool function names and reverse mapping on call

**Files:**
- Modify: `internal/pkg/toolx/builtin_tools.go`
- Modify: `internal/pkg/toolx/builtin_tools_test.go`
- Modify: `internal/ai/application/runtime/agent_loop_engine.go`
- Modify: `internal/ai/application/runtime/agent_loop_engine_test.go`

**Interfaces:**
- Consumes: `RegisteredToolSpecs`, `ToolSpec`, `ai.ToolDefinition`, `ai.ToolCall`
- Produces: `toolx.SanitizeToolFunctionName(toolCode string) string`, `toolx.ResolveToolFunctionName(name string) string`

- [ ] **Step 1: Write the failing tests for tool function name sanitization and resolution**

In `internal/pkg/toolx/builtin_tools_test.go`, add unit tests:
```go
func TestSanitizeToolFunctionName(t *testing.T) {
	cases := map[string]string{
		"builtin/knowledge_retrieve":   "builtin_knowledge_retrieve",
		"graph/triage_service_request": "graph_triage_service_request",
		"skill/12":                     "skill_12",
		"workflow/99":                  "workflow_99",
		"tool_search":                  "tool_search",
	}
	for input, expected := range cases {
		if got := SanitizeToolFunctionName(input); got != expected {
			t.Errorf("SanitizeToolFunctionName(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestResolveToolFunctionName(t *testing.T) {
	cases := map[string]string{
		"builtin_knowledge_retrieve":   "builtin/knowledge_retrieve",
		"graph_triage_service_request": "graph/triage_service_request",
		"knowledge_retrieve":           "builtin/knowledge_retrieve",
		"skill_12":                     "skill/12",
		"workflow/99":                  "workflow/99",
		"workflow_99":                  "workflow/99",
		"tool_search":                  "tool_search",
	}
	for input, expected := range cases {
		if got := ResolveToolFunctionName(input); got != expected {
			t.Errorf("ResolveToolFunctionName(%q) = %q, want %q", input, got, expected)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `& "C:\Program Files\Go\bin\go.exe" test ./internal/pkg/toolx -run TestSanitizeToolFunctionName`
Expected: Compilation failure or FAIL (functions undefined).

- [ ] **Step 3: Implement `SanitizeToolFunctionName` and `ResolveToolFunctionName` in `internal/pkg/toolx/builtin_tools.go`**

Implement character sanitization (`/` and any non `[a-zA-Z0-9_-]` runes mapped to `_`), spec code mapping, and prefix resolution for `skill_`, `workflow_`, `builtin_`, `graph_`.

- [ ] **Step 4: Update `internal/ai/application/runtime/agent_loop_engine.go`**

In `agentLoopToolDefinitions`:
Register capability definitions using `toolx.SanitizeToolFunctionName(code)` as `definition.Name`.
In `resolveAgentLoopToolCall`:
Resolve `call.Name` using `toolx.ResolveToolFunctionName(toolCode)`.
In `agent_loop_engine_test.go`:
Update `TestAgentLoopRegistersDirectCapabilityAliases` and add test verifying direct sanitized tool call execution.

- [ ] **Step 5: Run tests to verify they pass**

Run: `& "C:\Program Files\Go\bin\go.exe" test ./internal/pkg/toolx`
Run: `& "C:\Program Files\Go\bin\go.exe" test ./internal/ai/application/runtime`
Expected: PASS

---

### Task 2: Update prompt catalog with `| direct function: <sanitized>` and instruction prohibition

**Files:**
- Modify: `internal/ai/application/runtime/agent_turn.go`
- Modify: `internal/ai/application/runtime/agent_loop_engine_test.go`

**Interfaces:**
- Consumes: `toolx.SanitizeToolFunctionName`
- Produces: Prompt catalog lines including `| direct function: <sanitized>` and system prompt instructions forbidding capability codes with slashes as direct function names.

- [ ] **Step 1: Write test checking catalog format and instructions**

In `internal/ai/application/runtime/agent_loop_engine_test.go`, add test asserting catalog items in `prepareTurn` contain `| direct function:` and prompt specifies prohibition.

- [ ] **Step 2: Run test to verify it fails**

Run: `& "C:\Program Files\Go\bin\go.exe" test ./internal/ai/application/runtime -run TestAgentTurnCatalogFormat`
Expected: FAIL

- [ ] **Step 3: Implement catalog line updates and system prompt instruction in `internal/ai/application/runtime/agent_turn.go`**

For Builtin, Skill, Workflow, and MCP entries in `catalog`, append ` | direct function: <sanitized>`.
In `systemPrompt`, add instruction: `Never pass a capability code containing slashes as a tool function name; use the exact direct function name provided in the catalog or call tool_search with toolCode.`

- [ ] **Step 4: Run test to verify it passes**

Run: `& "C:\Program Files\Go\bin\go.exe" test ./internal/ai/application/runtime`
Expected: PASS

---

### Task 3: Timestamp parsing with UTC fallback & backend RFC3339 formatting

**Files:**
- Modify: `web/lib/utils.ts`
- Modify: `web/lib/api/im.ts`
- Modify: `web/lib/support-conversation-list.ts`
- Modify: `internal/services/customer_session_service.go`
- Modify: `internal/services/ws_service.go`
- Modify: `internal/services/auth_service.go`
- Modify: `web/lib/support-conversation-list.test.mjs`

**Interfaces:**
- Consumes: Raw date/time strings
- Produces: `parseApiTimestamp(value?: string | number | Date | null): number` in `web/lib/utils.ts`, RFC3339 timestamps in backend responses.

- [ ] **Step 1: Write tests for `parseApiTimestamp` in `web/lib/support-conversation-list.test.mjs` (or a dedicated test)**

Assert:
- `parseApiTimestamp("2026-10-11 12:00:00")` parses as UTC (`Date.parse("2026-10-11T12:00:00Z")`).
- `parseApiTimestamp("2026-10-11T12:00:00+03:00")` preserves `+03:00` offset.
- `parseApiTimestamp("2026-10-11T12:00:00Z")` preserves `Z`.
- Invalid strings return `0`.

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test web/lib/support-conversation-list.test.mjs`
Expected: FAIL (`parseApiTimestamp` is not defined or fails).

- [ ] **Step 3: Implement `parseApiTimestamp` in `web/lib/utils.ts`**

Export `parseApiTimestamp`: if missing timezone offset (`/(Z|[+-]\d{2}(?::?\d{2})?)$/i`), append `Z` to treat as UTC.
Use in `web/lib/api/im.ts` for `parseExpiresAt`.
Use in `web/lib/support-conversation-list.ts` for `recencyTimestamp`.

- [ ] **Step 4: Update backend date formatting to `time.RFC3339`**

In:
- `internal/services/customer_session_service.go` lines 80 and 164
- `internal/services/ws_service.go` line 163
- `internal/services/auth_service.go` line 309
Use `.UTC().Format(time.RFC3339)` (or `.Format(time.RFC3339)`).

- [ ] **Step 5: Run frontend and backend tests**

Run: `node --test web/lib/support-conversation-list.test.mjs`
Run: `& "C:\Program Files\Go\bin\go.exe" test ./internal/services`
Expected: PASS

---

### Task 4: Add `ARG NEXT_PUBLIC_OPEN_IM_CHANNEL_ID` in `Dockerfile`

**Files:**
- Modify: `Dockerfile`

**Interfaces:**
- Docker build argument `NEXT_PUBLIC_OPEN_IM_CHANNEL_ID` available to `web-builder` stage.

- [ ] **Step 1: Update `Dockerfile`**

In `web-builder` stage before `RUN pnpm build:sdk && pnpm build`, add:
```dockerfile
ARG NEXT_PUBLIC_OPEN_IM_CHANNEL_ID=""
ENV NEXT_PUBLIC_OPEN_IM_CHANNEL_ID=${NEXT_PUBLIC_OPEN_IM_CHANNEL_ID}
```

- [ ] **Step 2: Verify `Dockerfile` syntax**

Check syntax and confirm no regressions.

---

### Task 5: WebSocket connection lifecycle with `beforeConnect: () => ensureCustomerSession()`

**Files:**
- Modify: `web/lib/realtime-connection.ts`
- Modify: `web/lib/stores/support-chat.ts`
- Create / Update: `web/lib/realtime-connection.test.mjs`

**Interfaces:**
- Consumes: `options.beforeConnect`
- Produces: Guaranteed customer session exchange/refresh before WebSocket handshake.

- [ ] **Step 1: Write test in `web/lib/realtime-connection.test.mjs`**

Verify that `createRealtimeConnectionManager`:
- Awaits `beforeConnect` before calling `createSocket`.
- If `beforeConnect` fails, reports connect error, enters disconnected status, and schedules reconnect.
- Re-runs `beforeConnect` on reconnect.

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test web/lib/realtime-connection.test.mjs`
Expected: FAIL

- [ ] **Step 3: Implement `beforeConnect` in `web/lib/realtime-connection.ts` and wire into `web/lib/stores/support-chat.ts`**

Update `RealtimeConnectionManagerOptions` and `connect()` implementation.
Pass `beforeConnect: () => ensureCustomerSession()` in `useSupportChatStore`.

- [ ] **Step 4: Run tests and typecheck**

Run: `node --test web/lib/realtime-connection.test.mjs`
Run: `cd web && pnpm typecheck`
Expected: PASS
