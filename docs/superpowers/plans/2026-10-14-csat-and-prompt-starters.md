# CSAT (Customer Satisfaction Survey) & Channel Prompt Starters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide actionable customer feedback collection (CSAT 1-5 star ratings + optional comments) upon conversation closure, and display quick starter question pills (Prompt Starters) for new visitor chats, with complete multi-locale support and operator visibility.

**Architecture:** 
1. **Prompt Starters:** Channel web config (`WebChannelConfig.PromptStarters []string`) is exposed via `GET /api/channel/config`. When a visitor has zero messages, `PromptStarters` renders clickable question pills that automatically dispatch the question.
2. **CSAT Storage & Service:** A persistent `ConversationSatisfaction` model (`conversation_id`, `score`, `feedback`, `customer_id`, `channel_id`, `assignee_id`, `ai_agent_id`) records customer ratings.
3. **API & Realtime:** `POST /api/conversation/satisfaction` securely accepts ratings from the conversation owner. The rating is included in `ConversationDetailResponse` and displayed to operators.
4. **UI:** A modern, accessible `CSATCard` component appears in `/support/chat` when the conversation is closed, allowing star selection (1-5) and feedback submission.

**Tech Stack:** Go 1.26 (GORM, Gin), SQLite/MySQL, Next.js 16, React 19, TypeScript, Tailwind CSS, Lucide icons, i18n (`zh-CN`, `en-US`, `ru-RU`).

**Spec:** Complete end-to-end customer satisfaction and prompt starters system.

## Global Constraints

- Full 3-locale i18n support (`zh-CN`, `en-US`, `ru-RU`).
- All keys must be added symmetrically to `web/messages/zh-CN.json`, `web/messages/en-US.json`, and `web/messages/ru-RU.json`.
- Strict AGENTS.md layer ownership (`models -> repositories -> services -> handlers -> builders/DTOs`).
- Every transactional DB operation must use `ctx.Tx`.
- No raw SQL errors exposed to clients; return localized errors via `errorsx.*I18n`.
- Frontend code must typecheck cleanly (`tsc --noEmit`) and pass all node test suites.

## Review Focus

- Visitor attempting to rate a conversation they do not own (must return forbidden error).
- Submitting duplicate satisfaction ratings for the same conversation (must reject or update idempotently).
- Submitting invalid rating score (e.g. 0, 6, -1 — must enforce 1-5 constraint).
- Channel with empty or absent `PromptStarters` (must gracefully render standard empty state without layout shifts).
- Switching between closed and open conversations (must update CSAT display without stale state leaks).

---

### Task 1: Prompt Starters in Channel Configuration and Widget API

**Files:**
- Modify: `internal/pkg/dto/dto.go:16-24`
- Modify: `internal/pkg/dto/response/widget_response.go:3-12`
- Modify: `internal/handlers/api/channel_handler.go:25-58`
- Test: `internal/handlers/api/channel_handler_test.go`
- Create: `web/components/support-chat/prompt-starters.tsx`
- Modify: `web/components/support-chat/message-list.tsx`
- Modify: `web/lib/api/im.ts`

**Interfaces:**
- Consumes: `Channel.ConfigJSON`, `services.ChannelService.ParseWebChannelConfig`
- Produces:
  - `WebChannelConfig.PromptStarters []string`
  - `WidgetConfigResponse.PromptStarters []string`
  - `<PromptStarters items={...} onSelect={...} />` component

- [ ] **Step 1: Write failing test for channel config PromptStarters**

In `internal/handlers/api/channel_handler_test.go`, test that `resolveWidgetConfig` parses `promptStarters` from channel JSON and maps it to `WidgetConfigResponse`.

- [ ] **Step 2: Run test to verify failure**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handlers/api -run TestChannelConfigPromptStarters`
Expected: FAIL

- [ ] **Step 3: Implement PromptStarters backend and frontend**

1. In `internal/pkg/dto/dto.go`, add `PromptStarters []string` to `WebChannelConfig`:
   ```go
   PromptStarters []string `json:"promptStarters,omitempty"`
   ```
2. In `internal/pkg/dto/response/widget_response.go`:
   ```go
   PromptStarters []string `json:"promptStarters,omitempty"`
   ```
3. In `internal/handlers/api/channel_handler.go`:
   - Map `cfg.PromptStarters` in `webLikeWidgetConfig` and `WidgetConfigResponse`.
4. In `web/lib/api/im.ts`:
   - Add `promptStarters?: string[]` to `WidgetConfigResponse`.
5. Create `web/components/support-chat/prompt-starters.tsx`:
   - Renders a clean grid/flex list of buttons with question icon and hover effects.
   - Calls `onSelect(text)` on click.
6. In `web/components/support-chat/message-list.tsx`:
   - When `messages.length === 0` and `promptStarters?.length > 0`, render `<PromptStarters ... />`.

- [ ] **Step 4: Run test to verify pass**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handlers/api -run TestChannelConfigPromptStarters`
Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/dto/dto.go internal/pkg/dto/response/widget_response.go internal/handlers/api/channel_handler.go internal/handlers/api/channel_handler_test.go web/components/support-chat/prompt-starters.tsx web/components/support-chat/message-list.tsx web/lib/api/im.ts
git commit -m "feat(channel): support prompt starters for visitor onboarding"
```

---

### Task 2: Conversation Satisfaction (CSAT) Model, Repository & Service

**Files:**
- Modify: `internal/models/models.go:70-80,410-430`
- Create: `internal/repositories/conversation_satisfaction_repository.go`
- Create: `internal/services/conversation_satisfaction_service.go`
- Create: `internal/services/conversation_satisfaction_service_test.go`

**Interfaces:**
- Consumes: `models.Conversation`, `openidentity.ExternalUser`
- Produces:
  - `models.ConversationSatisfaction`
  - `repositories.ConversationSatisfactionRepository`
  - `services.ConversationSatisfactionService.SubmitSatisfaction(conversationID int64, externalUser openidentity.ExternalUser, score int, feedback string) (*models.ConversationSatisfaction, error)`
  - `services.ConversationSatisfactionService.GetByConversationID(conversationID int64) *models.ConversationSatisfaction`

- [ ] **Step 1: Write failing test**

In `internal/services/conversation_satisfaction_service_test.go`:
```go
func TestSubmitSatisfaction(t *testing.T) {
    // Tests:
    // 1. Rejects score < 1 or score > 5
    // 2. Rejects if external user is not conversation owner
    // 3. Creates record when valid and closed
    // 4. Returns existing record if already rated (idempotent)
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/services -run TestSubmitSatisfaction`
Expected: FAIL

- [ ] **Step 3: Implement model, repository, and service**

1. In `internal/models/models.go`:
   ```go
   type ConversationSatisfaction struct {
       ID             int64  `gorm:"primaryKey;autoIncrement"`
       ConversationID int64  `gorm:"type:bigint;not null;uniqueIndex"`
       Score          int    `gorm:"type:int;not null;default:5;index"`
       Feedback       string `gorm:"type:text"`
       ChannelID      int64  `gorm:"type:bigint;not null;default:0;index"`
       CustomerID     int64  `gorm:"type:bigint;not null;default:0;index"`
       AssigneeID     int64  `gorm:"type:bigint;not null;default:0;index"`
       AIAgentID      int64  `gorm:"type:bigint;not null;default:0;index"`
       AuditFields
   }
   ```
   Add `&ConversationSatisfaction{}` to `models.Models`.
2. Create `internal/repositories/conversation_satisfaction_repository.go`:
   - `GetByConversationID(db *gorm.DB, conversationID int64) *models.ConversationSatisfaction`
   - `Create(db *gorm.DB, item *models.ConversationSatisfaction) error`
   - `Update(db *gorm.DB, item *models.ConversationSatisfaction) error`
3. Create `internal/services/conversation_satisfaction_service.go`:
   - Validate `score >= 1 && score <= 5`.
   - Validate ownership via `ConversationService.IsCustomerConversationOwner`.
   - Save or update existing satisfaction record idempotently inside `sqls.WithTransaction`.

- [ ] **Step 4: Run test to verify pass**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/services -run TestSubmitSatisfaction`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/models.go internal/repositories/conversation_satisfaction_repository.go internal/services/conversation_satisfaction_service.go internal/services/conversation_satisfaction_service_test.go
git commit -m "feat(csat): add ConversationSatisfaction model, repository and service"
```

---

### Task 3: Customer CSAT API Route & Response Builder

**Files:**
- Create: `internal/pkg/dto/request/satisfaction_request.go`
- Create: `internal/pkg/dto/response/satisfaction_response.go`
- Modify: `internal/pkg/dto/response/conversation_response.go`
- Modify: `internal/builders/conversation_builder.go`
- Modify: `internal/handlers/api/conversation_handler.go`
- Modify: `internal/bootstrap/routes.go:90-100`
- Test: `internal/handlers/api/conversation_handler_test.go`

**Interfaces:**
- Consumes: `services.ConversationSatisfactionService`
- Produces:
  - Route `POST /api/conversation/satisfaction`
  - `ConversationResponse.Satisfaction *ConversationSatisfactionResponse`

- [ ] **Step 1: Write failing test**

In `internal/handlers/api/conversation_handler_test.go`:
Test `POST /api/conversation/satisfaction` with valid payload, score validation, and response mapping.

- [ ] **Step 2: Run test to verify failure**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handlers/api -run TestConversationSatisfactionAPI`
Expected: FAIL

- [ ] **Step 3: Implement route and handler**

1. Create `internal/pkg/dto/request/satisfaction_request.go`:
   ```go
   type SubmitSatisfactionRequest struct {
       ConversationID int64  `json:"conversationId"`
       Score          int    `json:"score"`
       Feedback       string `json:"feedback"`
   }
   ```
2. Create `internal/pkg/dto/response/satisfaction_response.go`:
   ```go
   type ConversationSatisfactionResponse struct {
       ID             int64  `json:"id"`
       ConversationID int64  `json:"conversationId"`
       Score          int    `json:"score"`
       Feedback       string `json:"feedback"`
       CreatedAt      string `json:"createdAt"`
   }
   ```
3. Update `ConversationResponse` and `builders.BuildConversationWithLocale` to embed satisfaction if present.
4. Add `ConversationPostSatisfaction` in `internal/handlers/api/conversation_handler.go`.
5. Register in `internal/bootstrap/routes.go`:
   ```go
   group.POST("/satisfaction", api.ConversationPostSatisfaction)
   ```

- [ ] **Step 4: Run test to verify pass**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handlers/api/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/dto/request/satisfaction_request.go internal/pkg/dto/response/satisfaction_response.go internal/pkg/dto/response/conversation_response.go internal/builders/conversation_builder.go internal/handlers/api/conversation_handler.go internal/bootstrap/routes.go internal/handlers/api/conversation_handler_test.go
git commit -m "feat(api): register /api/conversation/satisfaction endpoint"
```

---

### Task 4: Symmetrical i18n Translations for CSAT and Prompt Starters

**Files:**
- Modify: `web/messages/en-US.json`
- Modify: `web/messages/ru-RU.json`
- Modify: `web/messages/zh-CN.json`
- Test: `web/i18n/message-integrity.test.mjs`

**Interfaces:**
- Consumes: i18n keys for CSAT and prompt starters
- Produces: Identical keys in all 3 locales:
  - `csat.title`
  - `csat.score1`, `csat.score2`, `csat.score3`, `csat.score4`, `csat.score5`
  - `csat.commentPlaceholder`
  - `csat.submit`
  - `csat.submitting`
  - `csat.thankYou`
  - `csat.alreadyRated`
  - `promptStarters.title`

- [ ] **Step 1: Write test / integrity check**

Run `node --test web/i18n/message-integrity.test.mjs` (must pass baseline).

- [ ] **Step 2: Add keys symmetrically to all 3 locales**

Add:
```json
"csat": {
  "title": "How would you rate our support?",
  "score1": "Very dissatisfied",
  "score2": "Dissatisfied",
  "score3": "Neutral",
  "score4": "Satisfied",
  "score5": "Very satisfied",
  "commentPlaceholder": "Tell us what could be improved... (optional)",
  "submit": "Submit Rating",
  "submitting": "Submitting...",
  "thankYou": "Thank you for your feedback!",
  "alreadyRated": "You rated this conversation {score}/5"
},
"promptStarters": {
  "title": "Frequently Asked Questions"
}
```
And exact Russian and Chinese equivalents.

- [ ] **Step 3: Run integrity test**

Run: `node --test web/i18n/message-integrity.test.mjs`
Expected: PASS (0 missing keys, 0 placeholder mismatches).

- [ ] **Step 4: Commit**

```bash
git add web/messages/en-US.json web/messages/ru-RU.json web/messages/zh-CN.json
git commit -m "feat(i18n): add localized messages for CSAT and prompt starters"
```

---

### Task 5: Customer Chat CSAT UI & Operator View

**Files:**
- Modify: `web/lib/api/im.ts` (Add `submitConversationSatisfaction`)
- Create: `web/components/support-chat/csat-card.tsx`
- Modify: `web/components/support-chat/chat-shell.tsx`
- Modify: `web/app/(dashboard)/dashboard/conversations/_components/chat-panel.tsx`
- Test: `web/node_modules/.bin/tsc.cmd --noEmit` and `node --test web/lib/*.test.mjs`

**Interfaces:**
- Consumes: `submitConversationSatisfaction`, `ConversationResponse.satisfaction`
- Produces:
  - Interactive CSAT 5-star rating widget in `/support/chat` when conversation is closed.
  - CSAT rating badge displayed in operator conversation header.

- [ ] **Step 1: Implement `submitConversationSatisfaction` in `web/lib/api/im.ts`**

Add POST request to `/api/conversation/satisfaction` with `customerSessionToken`.

- [ ] **Step 2: Create `CSATCard` component in `web/components/support-chat/csat-card.tsx`**

- Renders 5 star icons with hover and active selection.
- Displays score label (`csat.score1`..`csat.score5`).
- Shows optional comment textarea.
- Submit button with pending state.
- Upon success or if already rated: renders clean thank-you confirmation with score.

- [ ] **Step 3: Mount in `chat-shell.tsx` and `chat-panel.tsx`**

- In `chat-shell.tsx`: when `conversation.status === 4` (closed), render `CSATCard` above the bottom container.
- In `chat-panel.tsx`: if `conversation.satisfaction` is present, display badge ⭐ `{satisfaction.score}/5` with tooltip showing feedback.

- [ ] **Step 4: Run typecheck and tests**

Run: `& "web/node_modules/.bin/tsc.cmd" --noEmit --project web/tsconfig.json`
Run: `node --test web/lib/*.test.mjs web/i18n/*.test.mjs`
Run: `& 'C:\Program Files\Go\bin\go.exe' test ./...`
Expected: ALL PASS with 0 errors.

- [ ] **Step 5: Commit**

```bash
git add web/lib/api/im.ts web/components/support-chat/csat-card.tsx web/components/support-chat/chat-shell.tsx web/app/\(dashboard\)/dashboard/conversations/_components/chat-panel.tsx
git commit -m "feat(ui): render CSAT rating card on closed conversations and display in operator dashboard"
```
