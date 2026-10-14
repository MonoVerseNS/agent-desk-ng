# Realtime AI Agent Activity Indicator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide immediate visual feedback in support chat and the operator dashboard by broadcasting real-time agent execution states ("thinking", "searching knowledge base...", "calling tool...", "running workflow...") over WebSocket during AI agent processing turns.

**Architecture:** The Go backend emits lightweight `agent.activity` domain events via `WsService` to the conversation's WebSocket topic at each phase of the Agent Loop (start, knowledge search, tool invocation, completion/failure). The Next.js frontend catches these events in `support-chat` and `agent-conversations` stores, rendering an animated, localized activity indicator bubble at the bottom of the message scroller until the final response message arrives.

**Tech Stack:** Go 1.26 (Gin, Gorilla WebSocket, Eino agent loop), React 19 / Next.js 16, TypeScript, Zustand, Lucide icons, Tailwind CSS, i18n (`zh-CN`, `en-US`, `ru-RU`).

**Spec:** Variant 2: Real-time Execution Step Statuses via WebSocket (non-token-streaming step indicators).

## Global Constraints

- Backend must maintain full 3-locale i18n support (`zh-CN`, `en-US`, `ru-RU`).
- All new translation keys must be added symmetrically to `web/messages/zh-CN.json`, `web/messages/en-US.json`, and `web/messages/ru-RU.json`.
- Event contracts must preserve existing WebSocket protocol conventions in `internal/services/ws_realtime_types.go`.
- Code changes must adhere to layer ownership defined in `AGENTS.md` (no direct DB access from handlers, pure builders, structured `slog` logging).
- No mock data or unhandled error drops; activity indicator must automatically dismiss on message arrival or conversation change.

## Review Focus

- Visitor switches conversation while AI agent is thinking in previous conversation (must not leak indicator across conversations).
- AI agent encounters error or times out during execution (must clean up indicator with "failed" status and not get stuck indefinitely).
- Multi-step tool loop where several tools run sequentially (must update step text dynamically without layout flicker).
- Rapid message send right after another (activity state must reset gracefully for the new turn).
- Offline evaluation or background test runs where `req.Conversation.ID <= 0` (must safely no-op without attempting WebSocket broadcast).

---

### Task 1: Backend WebSocket Event Contract & WsService Method

**Files:**
- Modify: `internal/pkg/enums/im.go:240-265`
- Modify: `internal/services/ws_realtime_types.go:210-250`
- Modify: `internal/services/ws_service.go:460-490`
- Test: `internal/services/ws_service_test.go`

**Interfaces:**
- Consumes: `models.Conversation`, `enums.IMRealtimeEventAgentActivity`
- Produces:
  - `RealtimeAgentActivityPayload{ConversationID int64, StepCode string, ToolName string, Status string, At string}`
  - `RealtimeAgentActivityEvent` implementing `RealtimeDomainEvent`
  - `WsService.PublishAgentActivity(conversation *models.Conversation, stepCode, toolName, status string)`

- [ ] **Step 1: Write the failing test**

Add `TestPublishAgentActivity` in `internal/services/ws_service_test.go`:
```go
func TestPublishAgentActivity(t *testing.T) {
	ws := newWsService()
	session := &ClientSession{
		ID:     "test-sess",
		Topics: map[string]struct{}{"conversation:100": {}},
		Send:   make(chan []byte, 10),
	}
	ws.manager.Register(session)

	conv := &models.Conversation{ID: 100}
	ws.PublishAgentActivity(conv, "searching_knowledge", "", "running")

	select {
	case payload := <-session.Send:
		var event RealtimeEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventAgentActivity {
			t.Fatalf("expected type %s, got %s", enums.IMRealtimeEventAgentActivity, event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for agent activity event")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/services -run TestPublishAgentActivity`
Expected: FAIL (missing `IMRealtimeEventAgentActivity` or `PublishAgentActivity`)

- [ ] **Step 3: Implement event types and `PublishAgentActivity`**

1. In `internal/pkg/enums/im.go`, define:
   ```go
   IMRealtimeEventAgentActivity = "agent.activity"
   ```
2. In `internal/services/ws_realtime_types.go`, define:
   ```go
   type RealtimeAgentActivityPayload struct {
       ConversationID int64  `json:"conversationId"`
       StepCode       string `json:"stepCode"`
       ToolName       string `json:"toolName,omitempty"`
       Status         string `json:"status"`
       At             string `json:"at"`
   }

   func (RealtimeAgentActivityPayload) realtimeEventPayload() {}

   type RealtimeAgentActivityEvent struct {
       Payload RealtimeAgentActivityPayload
   }

   func (e RealtimeAgentActivityEvent) EventType() string {
       return enums.IMRealtimeEventAgentActivity
   }

   func (e RealtimeAgentActivityEvent) EventPayload() RealtimeEventPayload {
       return e.Payload
   }
   ```
3. In `internal/services/ws_service.go`, add:
   ```go
   func (s *wsService) PublishAgentActivity(conversation *models.Conversation, stepCode, toolName, status string) {
       if conversation == nil || conversation.ID <= 0 {
           return
       }
       now := time.Now()
       event := s.newEvent(s.conversationTopic(conversation.ID), RealtimeAgentActivityEvent{
           Payload: RealtimeAgentActivityPayload{
               ConversationID: conversation.ID,
               StepCode:       stepCode,
               ToolName:       toolName,
               Status:         status,
               At:             formatWsTime(&now),
           },
       })
       s.PublishToTopics(s.routeConversationTopics(conversation), event)
   }
   ```

- [ ] **Step 4: Run test to verify it passes**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/services -run TestPublishAgentActivity`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/enums/im.go internal/services/ws_realtime_types.go internal/services/ws_service.go internal/services/ws_service_test.go
git commit -m "feat(realtime): add agent.activity event and PublishAgentActivity"
```

---

### Task 2: Emit Agent Activity During Loop Execution

**Files:**
- Modify: `internal/ai/application/runtime/agent_loop_engine.go:55-120,615-680`
- Test: `internal/ai/application/runtime/agent_loop_engine_test.go`

**Interfaces:**
- Consumes: `RunInput.Conversation`, `svc.WsService.PublishAgentActivity`
- Produces: Realtime step broadcasts during:
  - Turn preparation / start (`stepCode: "thinking"`, `status: "started"`)
  - Knowledge retrieval (`stepCode: "searching_knowledge"`, `status: "running"`)
  - Tool execution (`stepCode: "calling_tool"`, `toolName: <name>`, `status: "running"`)
  - Workflow execution (`stepCode: "running_workflow"`, `toolName: <name>`, `status: "running"`)
  - Turn completed (`stepCode: "completed"`, `status: "completed"`)
  - Turn failed (`stepCode: "failed"`, `status: "failed"`)

- [ ] **Step 1: Write the failing test**

In `internal/ai/application/runtime/agent_loop_engine_test.go`, add test asserting that `toolSearchExecutor` and `Run` safely execute without errors when `req.Conversation.ID > 0`.

- [ ] **Step 2: Run test to verify behavior**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/ai/application/runtime -run TestAgentLoopEngine`
Expected: PASS existing tests.

- [ ] **Step 3: Implement activity emissions in `AgentLoopEngine`**

1. In `Run(ctx context.Context, req RunInput) (*RunResult, error)`:
   - Emit start:
     ```go
     if req.Conversation.ID > 0 {
         svc.WsService.PublishAgentActivity(&req.Conversation, "thinking", "", "started")
     }
     ```
   - Before knowledge retrieval in `retrieveKnowledge`:
     ```go
     if req.Conversation.ID > 0 {
         svc.WsService.PublishAgentActivity(&req.Conversation, "searching_knowledge", "", "running")
     }
     ```
   - On exit (after `resolveAgentLoopReply` / error):
     ```go
     if req.Conversation.ID > 0 {
         if loopErr != nil || err != nil {
             svc.WsService.PublishAgentActivity(&req.Conversation, "failed", "", "failed")
         } else {
             svc.WsService.PublishAgentActivity(&req.Conversation, "completed", "", "completed")
         }
     }
     ```
2. In `toolSearchExecutor`:
   - When dispatching tool call:
     ```go
     if runInput.Conversation.ID > 0 {
         svc.WsService.PublishAgentActivity(&runInput.Conversation, "calling_tool", call.Name, "running")
     }
     ```

- [ ] **Step 4: Run Go package tests**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/ai/application/runtime/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/application/runtime/agent_loop_engine.go internal/ai/application/runtime/agent_loop_engine_test.go
git commit -m "feat(runtime): publish realtime agent activity steps during loop execution"
```

---

### Task 3: Frontend i18n Translations for Agent Activity

**Files:**
- Modify: `web/messages/en-US.json`
- Modify: `web/messages/ru-RU.json`
- Modify: `web/messages/zh-CN.json`
- Test: `web/i18n/message-integrity.test.mjs`

**Interfaces:**
- Consumes: i18n keys under `agentActivity.*`
- Produces: Identical keys across all 3 locales:
  - `agentActivity.thinking`
  - `agentActivity.searchingKnowledge`
  - `agentActivity.callingTool`
  - `agentActivity.runningWorkflow`
  - `agentActivity.default`

- [ ] **Step 1: Write the failing test / check**

Run message integrity test: `node --test web/i18n/message-integrity.test.mjs`
Expected: PASS (current baseline).

- [ ] **Step 2: Add matching translation keys**

1. In `web/messages/en-US.json`:
   ```json
   "agentActivity": {
     "thinking": "AI Agent is thinking...",
     "searchingKnowledge": "Searching knowledge base...",
     "callingTool": "Executing action: {name}...",
     "runningWorkflow": "Running workflow: {name}...",
     "default": "AI Agent is working on your request..."
   }
   ```
2. In `web/messages/ru-RU.json`:
   ```json
   "agentActivity": {
     "thinking": "AI-ассистент думает...",
     "searchingKnowledge": "Поиск в базе знаний...",
     "callingTool": "Выполнение действия: {name}...",
     "runningWorkflow": "Выполнение сценария: {name}...",
     "default": "AI-ассистент обрабатывает запрос..."
   }
   ```
3. In `web/messages/zh-CN.json`:
   ```json
   "agentActivity": {
     "thinking": "AI 思考中...",
     "searchingKnowledge": "检索知识库中...",
     "callingTool": "执行工具：{name}...",
     "runningWorkflow": "执行流程：{name}...",
     "default": "AI 正在处理您的请求..."
   }
   ```

- [ ] **Step 3: Run message integrity test**

Run: `node --test web/i18n/message-integrity.test.mjs`
Expected: PASS (all keys match in all supported locales).

- [ ] **Step 4: Commit**

```bash
git add web/messages/en-US.json web/messages/ru-RU.json web/messages/zh-CN.json
git commit -m "feat(i18n): add localized strings for agent activity states"
```

---

### Task 4: Frontend State & Realtime Event Integration

**Files:**
- Modify: `web/lib/im-realtime.ts:1-25`
- Modify: `web/lib/stores/support-chat.ts:60-100,230-310,480-520`
- Modify: `web/lib/stores/agent-conversations.ts:80-120,400-440`
- Test: `web/lib/agent-activity-state.test.mjs`

**Interfaces:**
- Consumes: `ImRealtimeEnvelope` with `type: "agent.activity"`, payload `RealtimeAgentActivityPayload`
- Produces:
  - Store slice `activeAgentActivity: { conversationId: number; stepCode: string; toolName?: string } | null`
  - Reducer/event-handler updating `activeAgentActivity` on `agent.activity`, and clearing on `message.created`, `conversation.closed`, or conversation switch.

- [ ] **Step 1: Write the failing test**

Create `web/lib/agent-activity-state.test.mjs`:
```javascript
import test from "node:test"
import assert from "node:assert/strict"
import { resolveAgentActivityState } from "./agent-activity-state.ts"

test("sets activity state when event matches active conversation and status is running", () => {
  const current = null
  const event = {
    conversationId: 42,
    stepCode: "searching_knowledge",
    status: "running",
  }
  const next = resolveAgentActivityState(current, 42, event)
  assert.deepEqual(next, { conversationId: 42, stepCode: "searching_knowledge" })
})

test("clears activity state when status is completed or failed", () => {
  const current = { conversationId: 42, stepCode: "thinking" }
  const event = {
    conversationId: 42,
    stepCode: "completed",
    status: "completed",
  }
  const next = resolveAgentActivityState(current, 42, event)
  assert.equal(next, null)
})

test("ignores events belonging to a different conversation", () => {
  const current = null
  const event = {
    conversationId: 99,
    stepCode: "thinking",
    status: "started",
  }
  const next = resolveAgentActivityState(current, 42, event)
  assert.equal(next, null)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test web/lib/agent-activity-state.test.mjs`
Expected: FAIL (module not found).

- [ ] **Step 3: Implement `agent-activity-state.ts` and wire into stores**

1. Create `web/lib/agent-activity-state.ts` with pure transition helper:
   ```typescript
   export type AgentActivityInfo = {
     conversationId: number
     stepCode: string
     toolName?: string
   }

   export type AgentActivityEventPayload = {
     conversationId: number
     stepCode: string
     toolName?: string
     status: string
   }

   export function resolveAgentActivityState(
     current: AgentActivityInfo | null,
     activeConversationId: number | undefined,
     payload: AgentActivityEventPayload | null | undefined
   ): AgentActivityInfo | null {
     if (!payload || !activeConversationId || payload.conversationId !== activeConversationId) {
       return current
     }
     if (payload.status === "completed" || payload.status === "failed") {
       return null
     }
     return {
       conversationId: payload.conversationId,
       stepCode: payload.stepCode,
       toolName: payload.toolName,
     }
   }
   ```
2. Update `web/lib/im-realtime.ts` to include `RealtimeAgentActivityPayload` in `ImRealtimeEnvelope`.
3. In `web/lib/stores/support-chat.ts`:
   - Add `activeAgentActivity: AgentActivityInfo | null` to `SupportChatStore`.
   - In WebSocket `onMessage`:
     - If `event.type === "agent.activity"`, update `activeAgentActivity` via `resolveAgentActivityState`.
     - In `message.created`, clear `activeAgentActivity: null` when message belongs to active conversation.
   - In `selectConversation`, reset `activeAgentActivity: null`.
4. In `web/lib/stores/agent-conversations.ts`:
   - Track `activeAgentActivity` similarly for the operator's active conversation.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test web/lib/agent-activity-state.test.mjs`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/lib/agent-activity-state.ts web/lib/agent-activity-state.test.mjs web/lib/im-realtime.ts web/lib/stores/support-chat.ts web/lib/stores/agent-conversations.ts
git commit -m "feat(store): handle agent.activity realtime events in support chat and agent stores"
```

---

### Task 5: UI Agent Activity Bubble Components

**Files:**
- Create: `web/components/chat/agent-activity-bubble.tsx`
- Modify: `web/components/support-chat/message-list.tsx`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/chat-panel.tsx`
- Test: `web/node_modules/.bin/tsc.cmd --noEmit` and `node --test web/lib/*.test.mjs`

**Interfaces:**
- Consumes: `activeAgentActivity`, `useI18n()`
- Produces:
  - `<AgentActivityBubble stepCode={...} toolName={...} />` component rendering:
    - Subtle AI avatar (`BotIcon` or `SparklesIcon`)
    - Animated pulse dots (`span className="animate-pulse"`)
    - Localized step label
  - Seamless insertion at bottom of message list in both customer support chat and operator chat panel.

- [ ] **Step 1: Create `AgentActivityBubble` component**

In `web/components/chat/agent-activity-bubble.tsx`:
```tsx
"use client"

import { memo } from "react"
import { BotIcon, Loader2Icon } from "lucide-react"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { ConversationMessageRow } from "@/components/chat/conversation-message-row"
import { ConversationMessageBubble } from "@/components/chat/conversation-message-bubble"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

export type AgentActivityBubbleProps = {
  stepCode: string
  toolName?: string
  className?: string
}

export const AgentActivityBubble = memo(function AgentActivityBubble({
  stepCode,
  toolName,
  className,
}: AgentActivityBubbleProps) {
  const t = useI18n()

  let label = t("agentActivity.default")
  if (stepCode === "thinking") {
    label = t("agentActivity.thinking")
  } else if (stepCode === "searching_knowledge") {
    label = t("agentActivity.searchingKnowledge")
  } else if (stepCode === "calling_tool") {
    label = t("agentActivity.callingTool", { name: toolName || "..." })
  } else if (stepCode === "running_workflow") {
    label = t("agentActivity.runningWorkflow", { name: toolName || "..." })
  }

  return (
    <ConversationMessageRow
      align="start"
      avatar={
        <Avatar className="size-8">
          <AvatarFallback className="bg-primary/10 text-primary">
            <BotIcon className="size-4" />
          </AvatarFallback>
        </Avatar>
      }
      contentClassName="max-w-[86%] gap-1.5"
      className={cn("my-1 animate-in fade-in-50 duration-200", className)}
    >
      <ConversationMessageBubble
        variant="system"
        className="flex items-center gap-2 rounded-lg border border-border/60 bg-muted/50 px-3 py-2 text-xs text-muted-foreground shadow-sm"
      >
        <Loader2Icon className="size-3.5 animate-spin text-primary shrink-0" />
        <span className="font-medium text-foreground/80">{label}</span>
      </ConversationMessageBubble>
    </ConversationMessageRow>
  )
})
```

- [ ] **Step 2: Mount bubble in `SupportChatMessageList`**

In `web/components/support-chat/message-list.tsx`:
- Receive `agentActivity?: AgentActivityInfo | null` prop (or read from `useSupportChatStore`).
- Render `<AgentActivityBubble ... />` right after the last message item inside `ConversationMessageScroller`.
- Trigger `scrollerRef.current?.scrollToBottom()` when activity state transitions from null to non-null if user was near bottom.

- [ ] **Step 3: Mount bubble in operator `chat-panel.tsx`**

In `web/app/(dashboard)/dashboard/conversations/_components/chat-panel.tsx`:
- Read `activeAgentActivity` from `useAgentConversationsStore`.
- Render `<AgentActivityBubble ... />` in `messagesScroller` when `activeAgentActivity` matches current conversation.

- [ ] **Step 4: Run TypeScript check and test suites**

Run:
1. `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
2. `node --test web/lib/*.test.mjs web/i18n/*.test.mjs`
3. `& 'C:\Program Files\Go\bin\go.exe' test ./...`
Expected: ALL PASS with zero errors.

- [ ] **Step 5: Commit**

```bash
git add web/components/chat/agent-activity-bubble.tsx web/components/support-chat/message-list.tsx web/app/\(dashboard\)/dashboard/conversations/_components/chat-panel.tsx
git commit -m "feat(ui): display real-time agent activity indicators in customer and operator chat"
```
