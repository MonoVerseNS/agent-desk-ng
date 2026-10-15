# AI Agent Context & Cost Optimization + Auto-Handoff on Fallback/Failure Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable granular UI control over AI agent token spending (context window size, ReAct reasoning steps limit, tool output truncation), and ensure that if the AI agent fails, has no answer, or the fallback does not work, the conversation automatically transfers to a human operator while notifying the customer with a clear localized message so they never feel ignored.

**Architecture:**
1. Extend `AIAgent` in `internal/models/models.go` and DTOs with `AutoHandoffOnFailure bool` and `HandoffMessage string` (while surfacing existing `MaxSteps` and `ContextWindow`).
2. In `internal/ai/runtime/reply_trigger_service.go`, when `s.executor.Run` errors or returns an empty reply without an answer, trigger automatic human handoff (`svc.ConversationHumanDispatchService.HandoffByAIWithRequestID`).
3. In `internal/services/conversation_human_dispatch_service.go`, ensure that on auto-handoff, the customer receives the agent's custom `HandoffMessage` or localized `conversation.handoff.waiting` ("Мы соединяем вас с оператором поддержки. Пожалуйста, подождите.").
4. In `internal/ai/application/runtime/agent_loop_engine.go` and `tool_result_reducer.go`, enforce `ContextWindow` message limits and truncate oversized tool outputs (2500 runes cap) to prevent context window blowouts.
5. In `web/app/(dashboard)/dashboard/ai-agents/_components/config-workbench.tsx`, provide an intuitive "Context & Cost Optimization" card and "Auto-Handoff on Failure" toggle with customizable handoff message.
6. Symmetrical i18n parity across `zh-CN`, `en-US`, and `ru-RU`.

**Tech Stack:** Go 1.26, GORM, Next.js 16 App Router, TypeScript, Base UI / Tailwind CSS, sonner.

---

## Global Constraints

- Follow AGENTS.md layer ownership (`handlers -> services -> repositories/builders`).
- Symmetrical 3-locale i18n parity (`zh-CN.json`, `en-US.json`, `ru-RU.json`).
- Non-destructive DB migrations (portable SQLite & MySQL compatibility).
- TypeScript compilation must pass cleanly (`tsc --noEmit`).
- Go formatting (`gofmt`) and all tests must pass cleanly.

---

### Task 1: Backend Model & Auto-Handoff Dispatch on Failure

**Files:**
- Modify: `internal/models/models.go`
- Modify: `internal/pkg/dto/request/ai_request.go`
- Modify: `internal/pkg/dto/response/ai_response.go`
- Modify: `internal/services/ai_agent_service.go`
- Modify: `internal/services/conversation_human_dispatch_service.go`
- Modify: `internal/ai/runtime/reply_trigger_service.go`
- Modify: `internal/services/conversation_human_dispatch_service_test.go`
- Modify: `internal/ai/runtime/reply_trigger_service_test.go`

- [ ] **Step 1: Add unit tests for Auto-Handoff on AI failure & empty reply**
In `internal/ai/runtime/reply_trigger_service_test.go`:
- Test that when `executor.Run` returns an error, if `AutoHandoffOnFailure` is true, `HandoffByAIWithRequestID` is invoked and sends a waiting message.
- Test that when `executor.Run` returns empty `ReplyText`, auto-handoff is invoked.
In `internal/services/conversation_human_dispatch_service_test.go`:
- Test that custom `HandoffMessage` on `AIAgent` is used when present, otherwise localized default is sent.

- [ ] **Step 2: Run tests to verify failure**
Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/ai/runtime/... ./internal/services/...`

- [ ] **Step 3: Update `models.AIAgent` and DTOs**
In `internal/models/models.go`:
```go
AutoHandoffOnFailure bool   `gorm:"type:tinyint(1);not null;default:1"` // AutoHandoffOnFailure: whether to transfer to human when AI fails or has no answer.
HandoffMessage       string `gorm:"type:text"`                          // HandoffMessage: custom notification sent to customer upon handoff.
```
Update `internal/pkg/dto/request/ai_request.go` and `internal/pkg/dto/response/ai_response.go`.
Update `internal/services/ai_agent_service.go` to persist and return these fields.

- [ ] **Step 4: Implement auto-handoff on failure in `reply_trigger_service.go`**
In `executeReply`:
When `err != nil`:
```go
if replyCtx.AIAgent.AutoHandoffOnFailure {
    _, handoffErr := svc.ConversationHumanDispatchService.HandoffByAIWithRequestID(
        replyCtx.Conversation.ID,
        replyCtx.AIAgent,
        "AI execution failure: "+err.Error(),
        replyCtx.Message.RequestID,
    )
    if handoffErr != nil {
        slog.Error("failed to auto-handoff on ai failure", "error", handoffErr)
    }
}
return err
```
When `summary != nil && strings.TrimSpace(summary.ReplyText) == "" && !summary.Interrupted && !summary.HandoffRequested`:
```go
if replyCtx.AIAgent.AutoHandoffOnFailure {
    _, handoffErr := svc.ConversationHumanDispatchService.HandoffByAIWithRequestID(
        replyCtx.Conversation.ID,
        replyCtx.AIAgent,
        "AI generated empty reply or fallback unavailable",
        replyCtx.Message.RequestID,
    )
    if handoffErr != nil {
        slog.Error("failed to auto-handoff on empty ai reply", "error", handoffErr)
    }
}
```

- [ ] **Step 5: Send customer notification message in `dispatchAfterHandoffWithRequestID`**
In `internal/services/conversation_human_dispatch_service.go`:
```go
waitingMessage := strings.TrimSpace(aiAgent.HandoffMessage)
if waitingMessage == "" {
    waitingMessage = HandoffWaitingMessageForLocale(conversation.Locale)
}
if err := s.sendAITextWithRequestID(conversationID, aiAgentID, waitingMessage, requestID); err != nil {
    return nil, err
}
```

- [ ] **Step 6: Run tests and verify they pass**
Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/ai/runtime/... ./internal/services/...`

---

### Task 2: Context Window & Tool Output Truncation

**Files:**
- Modify: `internal/ai/application/runtime/agent_turn.go`
- Modify: `internal/ai/application/runtime/agent_loop_engine.go`
- Modify: `internal/ai/runtime/tooling/tool_result_reducer.go`
- Modify: `internal/ai/runtime/tooling/tool_result_reducer_test.go`

- [ ] **Step 1: Write unit tests for tool output truncation**
In `internal/ai/runtime/tooling/tool_result_reducer_test.go`:
- Test that tool outputs exceeding 2500 runes are truncated with a clear ellipsis notice, preventing massive MCP responses from bloating the context window.

- [ ] **Step 2: Implement tool output truncation in `tool_result_reducer.go`**
Ensure outputs are capped to 2500 runes safely.

- [ ] **Step 3: Enforce `ContextWindow` and `MaxSteps` in `agent_loop_engine.go` and `agent_turn.go`**
In `agent_loop_engine.go`:
- If `req.AIAgent.ContextWindow > 0`, use it as `limit` (bounded between 2 and 30).
- If `req.AIAgent.MaxSteps > 0`, pass to `einoAgentLoop` (bounded between 1 and 8, default 4).

- [ ] **Step 4: Run Go tests**
Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/ai/...`

---

### Task 3: Symmetrical i18n Translations

**Files:**
- Modify: `web/messages/en-US.json`
- Modify: `web/messages/ru-RU.json`
- Modify: `web/messages/zh-CN.json`

- [ ] **Step 1: Add symmetrical keys for context optimization and auto-handoff**
Add:
- `aiAgent.sectionContextCosts`: "Context & Cost Optimization" / "Контекст и оптимизация расходов" / "上下文与成本优化"
- `aiAgent.contextCostsDescription`: "Manage context window size, reasoning step limits, and tool payload sizes."
- `aiAgent.contextWindow`: "Context Window (messages)" / "Контекстное окно (сообщений)" / "上下文历史消息数"
- `aiAgent.contextWindowHelp`: "Number of recent conversation messages included in context. Smaller window reduces token usage."
- `aiAgent.maxSteps`: "Max Reasoning Steps" / "Максимум шагов ReAct (Max Steps)" / "单次最大推理步骤"
- `aiAgent.maxStepsHelp`: "Max tool call steps allowed per turn. Setting 3–4 prevents runaways and token spikes."
- `aiAgent.autoHandoffOnFailure`: "Auto-handoff on AI failure or missing answer" / "Автоперевод на оператора при сбое или отсутствии ответа" / "AI异常或无答案时自动转人工"
- `aiAgent.autoHandoffOnFailureHelp`: "When the AI model errors, exhausts steps, or has no knowledge, immediately transfer the customer to a human agent."
- `aiAgent.handoffMessage`: "Customer Notice upon Handoff" / "Сообщение клиенту при переводе к оператору" / "转人工提示文案"
- `aiAgent.handoffMessageHelp`: "Sent to the customer so they know they have been transferred and are not being ignored."
- `aiAgent.handoffMessagePlaceholder`: "We are connecting you to a human support agent. Please wait." / "Мы соединяем вас с оператором поддержки. Пожалуйста, подождите." / "正在为你接入人工客服，请稍候。"

- [ ] **Step 2: Run message integrity test**
Run: `node --test web/i18n/message-integrity.test.mjs`

---

### Task 4: Dashboard UI Workbench (`config-workbench.tsx`)

**Files:**
- Modify: `web/lib/api/admin.ts`
- Modify: `web/app/(dashboard)/dashboard/ai-agents/_components/config-workbench.tsx`
- Create: `web/app/(dashboard)/dashboard/ai-agents/_components/config-workbench-cost.test.mjs`

- [ ] **Step 1: Write component unit test for workbench form controls**
In `config-workbench-cost.test.mjs`:
- Verify `contextWindow`, `maxSteps`, `autoHandoffOnFailure`, and `handoffMessage` are rendered and included in the save payload.

- [ ] **Step 2: Run test to verify failure**
Run: `node --test web/app/\(dashboard\)/dashboard/ai-agents/_components/config-workbench-cost.test.mjs`

- [ ] **Step 3: Update `admin.ts` types**
Add `maxSteps`, `contextWindow`, `autoHandoffOnFailure`, `handoffMessage` to `AIAgent` and `CreateAIAgentPayload`.

- [ ] **Step 4: Update `config-workbench.tsx` UI**
- Add state:
  - `contextWindow: string` (default "10")
  - `maxSteps: string` (default "4")
  - `autoHandoffOnFailure: boolean` (default true)
  - `handoffMessage: string` (default "")
- In Section `setup` or new sub-section `sectionContextCosts`:
  - Render `contextWindow` input with helper text.
  - Render `maxSteps` input with helper text.
- In Section `service` under `sectionFallback`:
  - Render Switch / Toggle for `autoHandoffOnFailure`.
  - Render `handoffMessage` Textarea with placeholder and helper text.
- Update `buildPayload()` to include all 4 fields.
- Initialize all 4 fields in `fetchAIAgent` loading logic.

- [ ] **Step 5: Run tests and typecheck**
Run: `node --test web/app/\(dashboard\)/dashboard/ai-agents/_components/config-workbench-cost.test.mjs`
Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`

---

### Task 5: Whole-Branch Integration Verification & Security Scan

- [ ] **Step 1: Run all backend Go tests**
`& 'C:\Program Files\Go\bin\go.exe' test ./internal/...`
- [ ] **Step 2: Run all frontend unit tests**
`node --test web/lib/*.test.mjs web/i18n/*.test.mjs web/components/*.test.mjs web/components/support-chat/*.test.mjs`
- [ ] **Step 3: Run TypeScript typecheck**
`& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
- [ ] **Step 4: Run Snyk code scan**
Call `snyk_code_scan` on changed directories.
- [ ] **Step 5: Git commit and push**
Commit changes and push to `origin main`.
