# AI Operator Co-Pilot & Conversation Auto-Summarization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Empower human operators with an AI Co-Pilot that generates contextual draft reply suggestions with 1 click, and provides automated conversation summaries (for quick handoffs and close documentation).

**Architecture:** Create an `AICopilotService` in the Go backend utilizing `ai.LLM.Chat` with conversation history assembly, locale-aware prompt instructions, and graceful fallbacks. Expose endpoints `POST /api/dashboard/conversation/suggest_reply` and `POST /api/dashboard/conversation/summarize`. On the frontend, integrate a Sparkles (✨) AI Suggest action in `SharedMessageEditor` and an AI Summary inspection/close button in the conversation panel.

**Tech Stack:** Go 1.26, Next.js 16 App Router, TypeScript, Base UI / Tailwind CSS, Lucide icons (`Sparkles`, `FileText`), OpenAI/Qwen compatible LLM backend.

**Spec:** Agreed enhancements roadmap (Option 2 - AI operator features).

## Global Constraints

- Follow AGENTS.md layer ownership (`handlers -> services -> repositories/builders`). Handlers must not call LLM or repositories directly.
- Symmetrical 3-locale i18n parity (`zh-CN.json`, `en-US.json`, `ru-RU.json`).
- Resilient against absent/offline LLM config: return clean, localized errors rather than crashes or unhandled exceptions.
- TypeScript compilation must pass with 0 errors (`tsc --noEmit`).
- Go formatting (`gofmt`) and all Go tests must pass cleanly.

## Review Focus

1. Prompt formatting & injection safety: customer input must be clearly delimited so user messages cannot hijack system instructions.
2. Context window limits: only format the most recent 10-15 messages to prevent exceeding LLM context windows or incurring high latency.
3. Locale matching: if the customer spoke Russian or English, the draft reply must match the conversation's language.
4. Concurrent suggestions: disable the AI Suggest button while generation is in flight to prevent duplicate requests.
5. Base UI compatibility: avoid Radix `asChild` assumptions; follow AGENTS.md styling guidelines (`rounded-md`, clean spacing).

---

### Task 1: Backend AI Co-Pilot Service (`SuggestReply` and `SummarizeConversation`)

**Files:**
- Create: `internal/services/ai_copilot_service.go`
- Create: `internal/services/ai_copilot_service_test.go`

**Interfaces:**
- Produces:
  ```go
  var AICopilotService = newAICopilotService()

  type AICopilotService interface {
      SuggestReply(ctx context.Context, conversationID int64, locale string) (string, error)
      SummarizeConversation(ctx context.Context, conversationID int64, locale string) (string, error)
  }
  ```

- [ ] **Step 1: Write unit tests for AICopilotService**

In `internal/services/ai_copilot_service_test.go`:
- Test prompt generation with recent messages.
- Test message normalization and conversation history formatting.
- Test fallback behavior when no messages exist.

- [ ] **Step 2: Run test to verify it fails**

Run: `& 'C:\Program Files\Go\bin\go.exe' test -run TestAICopilotService ./internal/services/...`
Expected: FAIL

- [ ] **Step 3: Implement `AICopilotService`**

In `internal/services/ai_copilot_service.go`:
- Fetch recent messages (up to 12) from `MessageService`.
- Build conversation transcript formatted as `Customer: ...` / `Agent: ...`.
- Assemble system and user prompts respecting `locale`.
- Call `ai.LLM.Chat(ctx, systemPrompt, userPrompt)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `& 'C:\Program Files\Go\bin\go.exe' test -run TestAICopilotService ./internal/services/...`
Expected: PASS

- [ ] **Step 5: Run gofmt and commit**

```bash
git add internal/services/ai_copilot_service.go internal/services/ai_copilot_service_test.go
git commit -m "feat(ai): implement AICopilotService for draft replies and conversation summarization"
```

---

### Task 2: Dashboard API Endpoints for AI Co-Pilot

**Files:**
- Create: `internal/pkg/dto/request/ai_copilot_request.go`
- Create: `internal/pkg/dto/response/ai_copilot_response.go`
- Modify: `internal/handlers/dashboard/conversation_handler.go`
- Modify: `internal/handlers/dashboard/conversation_handler_test.go`
- Modify: `internal/bootstrap/routes.go`

**Interfaces:**
- Produces:
  - `POST /api/dashboard/conversation/suggest_reply`: takes `{ conversationId: int64 }`, returns `{ suggestion: string }`.
  - `POST /api/dashboard/conversation/summarize`: takes `{ conversationId: int64 }`, returns `{ summary: string }`.
  - Permission: requires `constants.PermissionConversationView`.

- [ ] **Step 1: Write handler tests**

In `internal/handlers/dashboard/conversation_handler_test.go`:
Add tests for `ConversationPostSuggest_reply` and `ConversationPostSummarize` (validation, permission check, success).

- [ ] **Step 2: Run test to verify it fails**

Run: `& 'C:\Program Files\Go\bin\go.exe' test -run TestConversationSuggestReply ./internal/handlers/dashboard/...`
Expected: FAIL

- [ ] **Step 3: Implement DTOs, handlers, and routes**

1. Create `internal/pkg/dto/request/ai_copilot_request.go` with `SuggestReplyRequest` and `SummarizeConversationRequest`.
2. Create `internal/pkg/dto/response/ai_copilot_response.go` with `SuggestReplyResponse` and `ConversationSummaryResponse`.
3. In `internal/handlers/dashboard/conversation_handler.go`: implement `ConversationPostSuggest_reply` and `ConversationPostSummarize`.
4. In `internal/bootstrap/routes.go`: register both routes under dashboard conversation group.

- [ ] **Step 4: Run test to verify it passes**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handlers/dashboard/...`
Expected: PASS

- [ ] **Step 5: Run gofmt and commit**

```bash
git add internal/pkg/dto/request/ai_copilot_request.go internal/pkg/dto/response/ai_copilot_response.go internal/handlers/dashboard/conversation_handler.go internal/handlers/dashboard/conversation_handler_test.go internal/bootstrap/routes.go
git commit -m "feat(api): register /suggest_reply and /summarize dashboard endpoints"
```

---

### Task 3: Symmetrical i18n Translations for AI Co-Pilot

**Files:**
- Modify: `web/messages/en-US.json`
- Modify: `web/messages/ru-RU.json`
- Modify: `web/messages/zh-CN.json`
- Test: `web/i18n/message-integrity.test.mjs`

**Interfaces:**
- Produces: Symmetrical keys across all 3 locales:
  - `aiCopilot.suggestReply`: "AI Suggest" / "Подсказать ответ" / "AI 生成回复"
  - `aiCopilot.generating`: "Generating reply..." / "Генерация ответа..." / "正在生成回复..."
  - `aiCopilot.insertReply`: "Use this reply" / "Использовать ответ" / "填入输入框"
  - `aiCopilot.discardReply`: "Dismiss" / "Отклонить" / "放弃"
  - `aiCopilot.summarize`: "AI Summary" / "AI Резюме" / "AI 总结"
  - `aiCopilot.summarizing`: "Summarizing..." / "Составление резюме..." / "正在生成总结..."
  - `aiCopilot.summaryTitle`: "Conversation Summary" / "Краткое резюме диалога" / "会话小结"
  - `aiCopilot.noSummary`: "No summary available yet" / "Резюме пока недоступно" / "暂无会话总结"
  - `aiCopilot.errorGenerating`: "Failed to generate AI suggestion" / "Не удалось сгенерировать ответ" / "生成 AI 建议失败"

- [ ] **Step 1: Check baseline integrity**

Run: `node --test web/i18n/message-integrity.test.mjs`

- [ ] **Step 2: Add keys symmetrically to all 3 JSON files**

Add `aiCopilot.*` keys with exact placeholder parity.

- [ ] **Step 3: Run integrity tests**

Run: `node --test web/i18n/message-integrity.test.mjs`
Expected: PASS (0 missing keys, 0 placeholder mismatches).

- [ ] **Step 4: Commit**

```bash
git add web/messages/en-US.json web/messages/ru-RU.json web/messages/zh-CN.json
git commit -m "feat(i18n): add localized messages for AI copilot draft replies and summarization"
```

---

### Task 4: AI Reply Suggestion UI in Message Editor

**Files:**
- Modify: `web/lib/api/agent.ts`
- Modify: `web/components/chat/shared-message-editor.tsx`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/agent-message-editor.tsx`
- Test: `web/node_modules/.bin/tsc.cmd --noEmit` and node tests

**Interfaces:**
- Consumes: `POST /api/dashboard/conversation/suggest_reply`
- Produces:
  - Sparkles icon button in operator message editor toolbar.
  - When clicked: requests suggestion, renders an interactive floating/docked suggestion card with:
    - Generated text preview
    - "Use this reply" button (inserts text into editor and closes card)
    - "Dismiss" button
  - Disabled / loading state with spinner while generating.

- [ ] **Step 1: Add API client methods in `web/lib/api/agent.ts`**

Export `suggestAgentReply(conversationId: number)` and `summarizeAgentConversation(conversationId: number)`.

- [ ] **Step 2: Add AI Suggest button & suggestion drawer in `SharedMessageEditor`**

In `SharedMessageEditor`:
- Add `aiCopilot?: { onSuggest: () => Promise<string | null>; loading?: boolean }` prop.
- Render `SparklesIcon` button in operator toolbar.
- When suggestion is available, render a clean draft suggestion card above the editor with "Use this reply" and "Dismiss".

- [ ] **Step 3: Wire into `AgentMessageEditor`**

In `agent-message-editor.tsx`:
- Connect `aiCopilot` calling `suggestAgentReply(selectedConversationId)`.
- Handle insertion and toast notification.

- [ ] **Step 4: Run typecheck and tests**

Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Run: `node --test web/lib/*.test.mjs web/i18n/*.test.mjs`
Expected: ALL PASS with 0 errors.

- [ ] **Step 5: Commit**

```bash
git add web/lib/api/agent.ts web/components/chat/shared-message-editor.tsx web/app/\(dashboard\)/dashboard/conversations/_components/agent-message-editor.tsx
git commit -m "feat(ui): add AI reply suggestion action and preview card to operator message editor"
```

---

### Task 5: AI Conversation Summary in Header & Close Dialog

**Files:**
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/chat-panel.tsx`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/conversation-info-panel.tsx`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/conversation-close-dialog.tsx` (if present)
- Test: `web/node_modules/.bin/tsc.cmd --noEmit` and node tests

**Interfaces:**
- Consumes: `summarizeAgentConversation(conversationId)`
- Produces:
  - "AI Summary" popover / dialog button in conversation header or info panel.
  - Generates summary of the conversation turns on demand.
  - Displays structured summary in dialog/card.

- [ ] **Step 1: Add AI Summary dialog/popover component**

Create `web/app/(dashboard)/dashboard/conversations/_components/conversation-summary-dialog.tsx` rendering summary with loading state.

- [ ] **Step 2: Mount summary trigger in header/info panel**

In `chat-panel.tsx` or `conversation-info-panel.tsx`:
Add an action button with `FileTextIcon` / `SparklesIcon` labeled `aiCopilot.summarize` opening the summary dialog.

- [ ] **Step 3: Run typecheck, node tests, and Go backend tests**

Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Run: `node --test web/lib/*.test.mjs web/i18n/*.test.mjs`
Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/...`
Expected: ALL PASS with 0 errors.

- [ ] **Step 4: Commit**

```bash
git add web/app/\(dashboard\)/dashboard/conversations/_components/conversation-summary-dialog.tsx web/app/\(dashboard\)/dashboard/conversations/_components/chat-panel.tsx web/app/\(dashboard\)/dashboard/conversations/_components/conversation-info-panel.tsx
git commit -m "feat(ui): add AI conversation summarization dialog and header action"
```
