# Operator Workspace: Slash Commands, Collision Detection & Alerts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance the human operator workspace with inline slash command (`/`) quick replies, real-time operator collision detection (preventing two operators from answering the same customer at once), and audio-visual alerts for incoming messages.

**Architecture:** Extend the existing WebSocket service with a lightweight `operator.presence` topic broadcast (`viewing`, `typing`, `left`). Integrate slash command trigger into `SharedMessageEditor` for instant macro insertion. Add client-side presence state tracking with automatic TTL decay, rendering an operator collision banner in the chat panel. Provide synthesized Web Audio chime alerts for incoming customer messages.

**Tech Stack:** Go 1.26, Gorilla WebSocket, Next.js 16 App Router, TypeScript, Tiptap, Base UI / Tailwind CSS, Web Audio API.

**Spec:** Agreed enhancements roadmap (Option 2 - Operator Workspace improvements).

## Global Constraints

- Follow AGENTS.md layer ownership (no direct SQL in handlers, clean WebSocket envelope structures).
- Symmetrical 3-locale i18n parity (`zh-CN.json`, `en-US.json`, `ru-RU.json`).
- Zero external binary/audio assets — synthesize audio chime using native Web Audio API.
- TypeScript compilation must pass with 0 errors (`tsc --noEmit`).
- Go formatting (`gofmt`) and all Go tests must pass cleanly.

## Review Focus

1. Operator presence event storms: typing events must be throttled (max once per 3s).
2. Presence cleanup: presence records must auto-expire after TTL (15s) so disconnected or crashed tabs don't show ghost operators.
3. Slash command replacement: selecting a quick reply via `/` must replace the `/` text cleanly without leaving orphan slashes.
4. Audio context autoplay policies: Web Audio API must initialize lazily upon user interaction and safely no-op when blocked.
5. Multi-operator collision UI: banner must be informative, non-disruptive, and properly localized across all 3 languages.

---

### Task 1: Backend Operator Presence Event & WebSocket Protocol

**Files:**
- Modify: `internal/pkg/enums/im.go`
- Modify: `internal/services/ws_realtime_types.go`
- Modify: `internal/services/ws_service.go`
- Test: `internal/services/ws_service_test.go`

**Interfaces:**
- Produces:
  - `enums.IMRealtimeEventOperatorPresence = "operator.presence"`
  - `RealtimeOperatorPresencePayload`:
    ```go
    type RealtimeOperatorPresencePayload struct {
        ConversationID int64  `json:"conversationId"`
        UserID         int64  `json:"userId"`
        Username       string `json:"username"`
        Nickname       string `json:"nickname"`
        Action         string `json:"action"` // "viewing", "typing", "left"
        At             string `json:"at"`
    }
    ```
  - `RealtimeOperatorPresenceEvent` implementing `RealtimeDomainEvent`.
  - Client message parsing in `ws_service.go` for `type: "presence"`.
  - `WsService.PublishOperatorPresence(conversationID int64, principal *dto.AuthPrincipal, action string, excludeConnID string)`.

- [ ] **Step 1: Write test for Operator Presence publishing**

In `internal/services/ws_service_test.go`:
Add `TestPublishOperatorPresence` verifying that publishing presence sends the event to subscribers on `conversation:{id}` while skipping `excludeConnID`.

- [ ] **Step 2: Run test to verify it fails**

Run: `& 'C:\Program Files\Go\bin\go.exe' test -run TestPublishOperatorPresence ./internal/services/...`
Expected: FAIL

- [ ] **Step 3: Implement enum, payload, event, and WsService methods**

1. In `internal/pkg/enums/im.go`: define `IMRealtimeEventOperatorPresence = "operator.presence"`.
2. In `internal/services/ws_realtime_types.go`: define `RealtimeOperatorPresencePayload` and `RealtimeOperatorPresenceEvent`.
3. In `internal/services/ws_service.go`:
   - Handle client message `type == "presence"`: parse `conversationId` and `action`, call `PublishOperatorPresence`.
   - Implement `PublishOperatorPresence(conversationID int64, principal *dto.AuthPrincipal, action string, excludeConnID string)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `& 'C:\Program Files\Go\bin\go.exe' test -run TestPublishOperatorPresence ./internal/services/...`
Expected: PASS

- [ ] **Step 5: Run gofmt and commit**

```bash
git add internal/pkg/enums/im.go internal/services/ws_realtime_types.go internal/services/ws_service.go internal/services/ws_service_test.go
git commit -m "feat(ws): support operator.presence realtime event and client presence messages"
```

---

### Task 2: Symmetrical i18n Translations for Collision Alerts and Audio Notifications

**Files:**
- Modify: `web/messages/en-US.json`
- Modify: `web/messages/ru-RU.json`
- Modify: `web/messages/zh-CN.json`
- Test: `web/i18n/message-integrity.test.mjs`

**Interfaces:**
- Produces: Symmetrical keys across all 3 locales:
  - `operatorPresence.viewing`: "{name} is viewing this conversation" / "{name} просматривает этот диалог" / "{name} 正在查看此会话"
  - `operatorPresence.typing`: "{name} is typing a reply..." / "{name} печатает ответ..." / "{name} 正在输入回复..."
  - `operatorPresence.multipleViewing`: "{count} operators are viewing this conversation" / "{count} оператора просматривают этот диалог" / "{count} 位客服正在查看此会话"
  - `operatorPresence.collisionWarning`: "Warning: Another operator is active here to avoid duplicate replies." / "Внимание: другой оператор активен в этом диалоге во избежание дублирования ответов." / "注意：其他客服正在处理此会话，请避免重复回复。"
  - `conversation.quickReplySlashHint`: "Type / for quick replies" / "Введите / для быстрого ответа" / "输入 / 快捷回复"

- [ ] **Step 1: Check baseline integrity**

Run: `node --test web/i18n/message-integrity.test.mjs`

- [ ] **Step 2: Add keys symmetrically to all 3 JSON files**

Add `operatorPresence.*` and `conversation.quickReplySlashHint` with exact placeholder parity `{name}` and `{count}`.

- [ ] **Step 3: Run integrity tests**

Run: `node --test web/i18n/message-integrity.test.mjs`
Expected: PASS (0 missing keys, 0 placeholder mismatches).

- [ ] **Step 4: Commit**

```bash
git add web/messages/en-US.json web/messages/ru-RU.json web/messages/zh-CN.json
git commit -m "feat(i18n): add localized messages for operator collision and slash commands"
```

---

### Task 3: Slash Commands (`/`) Quick Reply Autocomplete in SharedMessageEditor

**Files:**
- Modify: `web/components/chat/shared-message-editor.tsx`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/agent-message-editor.tsx`
- Test: `web/node_modules/.bin/tsc.cmd --noEmit` and manual verification

**Interfaces:**
- Consumes: `quickReplies` prop in `SharedMessageEditor`
- Produces:
  - When typing `/` as first character or when editor text matches `/query`, opens the Quick Replies popover and filters items by `query`.
  - Selecting a quick reply removes the `/query` prefix and inserts the chosen quick reply content.
  - Shows subtle hint `conversation.quickReplySlashHint` in editor toolbar for operator variant.

- [ ] **Step 1: Implement slash trigger and query tracking**

In `web/components/chat/shared-message-editor.tsx`:
1. In `editor.on("update")`:
   - Inspect plain text of current block.
   - If text matches `^/([^\s]*)$`:
     - Open quick replies popover.
     - Set quick reply search query to match the typed characters.
2. In `handleInsertQuickReply(item)`:
   - If current text starts with `/`, delete the slash prefix before inserting `item.content`.
3. In toolbar: show subtle hint for operators (`conversation.quickReplySlashHint`).

- [ ] **Step 2: Run typecheck**

Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/components/chat/shared-message-editor.tsx web/app/\(dashboard\)/dashboard/conversations/_components/agent-message-editor.tsx
git commit -m "feat(editor): support slash command / for quick reply autocomplete"
```

---

### Task 4: Operator Presence Tracking & Collision Warning UI

**Files:**
- Create: `web/lib/operator-presence-state.ts`
- Create: `web/lib/operator-presence-state.test.mjs`
- Modify: `web/hooks/use-agent-conversation-realtime.ts`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/chat-panel.tsx`
- Test: `node --test web/lib/operator-presence-state.test.mjs` and `web/node_modules/.bin/tsc.cmd --noEmit`

**Interfaces:**
- Produces:
  - `resolveOperatorPresence(currentList, incomingEvent, currentUserId)` -> pure function returning active peer presences, purging expired entries (>15s TTL).
  - Outgoing presence messages:
    - `"viewing"` sent when selecting conversation.
    - `"typing"` sent when operator types (throttled to 3s).
    - `"left"` sent when unselecting conversation.
  - In `chat-panel.tsx`:
    - Renders an alert banner above the message editor or below conversation header if peer operators are active.
    - Displays avatar badge and text e.g. "Jane is viewing this conversation" or "Jane is typing a reply...".

- [ ] **Step 1: Write unit tests for pure presence state transitions**

Create `web/lib/operator-presence-state.test.mjs` verifying:
- Adds new peer operator with timestamp.
- Ignores self (currentUserId).
- Updates action from "viewing" to "typing".
- Removes operator on action "left".
- Filters out records older than 15 seconds.

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test web/lib/operator-presence-state.test.mjs`
Expected: FAIL

- [ ] **Step 3: Implement `web/lib/operator-presence-state.ts`**

Export:
- `OperatorPresenceRecord`
- `updateOperatorPresenceList`
- `purgeExpiredOperatorPresences`

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test web/lib/operator-presence-state.test.mjs`
Expected: PASS

- [ ] **Step 5: Integrate into `use-agent-conversation-realtime.ts` and `chat-panel.tsx`**

1. In `use-agent-conversation-realtime.ts`:
   - Send `{"type": "presence", "conversationId": id, "action": "viewing"}` on subscribe.
   - Send `{"type": "presence", "conversationId": id, "action": "left"}` on unsubscribe.
   - Listen for `operator.presence` events and update store presence state.
2. In `chat-panel.tsx`:
   - When active peer operators exist on `currentConversation.id`:
     - Render collision warning banner with `UsersIcon` and localized description.

- [ ] **Step 6: Run typecheck and tests**

Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Run: `node --test web/lib/operator-presence-state.test.mjs`
Expected: ALL PASS with 0 errors.

- [ ] **Step 7: Commit**

```bash
git add web/lib/operator-presence-state.ts web/lib/operator-presence-state.test.mjs web/hooks/use-agent-conversation-realtime.ts web/app/\(dashboard\)/dashboard/conversations/_components/chat-panel.tsx
git commit -m "feat(presence): detect operator collision and render warning banner"
```

---

### Task 5: Incoming Message Audio & Expanded Browser Alerting

**Files:**
- Create: `web/lib/sound-alert.ts`
- Create: `web/lib/sound-alert.test.mjs`
- Modify: `web/hooks/use-agent-conversation-realtime.ts`
- Test: `node --test web/lib/sound-alert.test.mjs` and `web/node_modules/.bin/tsc.cmd --noEmit`

**Interfaces:**
- Produces:
  - `playIncomingMessageSound()`: synthesizes a clean 587Hz -> 880Hz (D5 -> A5) chime via Web Audio API. Safe when audio is suspended or unsupported.
  - In `use-agent-conversation-realtime.ts`:
    - When `message.created` arrives from a customer:
      - If message is from customer and either:
        - assigned to current operator, OR
        - conversation is unassigned / pending
      - Play sound alert via `playIncomingMessageSound()`.
      - Show browser notification if tab is in background.

- [ ] **Step 1: Write unit test for sound alert helper**

In `web/lib/sound-alert.test.mjs`:
Test safe fallback when `AudioContext` is undefined (Node.js/SSR environment).

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test web/lib/sound-alert.test.mjs`
Expected: FAIL

- [ ] **Step 3: Implement `web/lib/sound-alert.ts`**

Synthesizes gentle dual-tone sine oscillator with exponential decay.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test web/lib/sound-alert.test.mjs`
Expected: PASS

- [ ] **Step 5: Wire into `use-agent-conversation-realtime.ts`**

Trigger `playIncomingMessageSound()` on new customer messages for assigned/pending conversations.

- [ ] **Step 6: Run typecheck and all tests**

Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Run: `node --test web/lib/*.test.mjs web/i18n/*.test.mjs`
Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/...`
Expected: ALL PASS with 0 errors.

- [ ] **Step 7: Commit**

```bash
git add web/lib/sound-alert.ts web/lib/sound-alert.test.mjs web/hooks/use-agent-conversation-realtime.ts
git commit -m "feat(notification): add audio chime and expanded alerts for incoming customer messages"
```
