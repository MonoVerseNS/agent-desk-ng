import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"

import {
  findConversationById,
  isOpenImConversation,
  pickConversationToResume,
  resolveConversationTitle,
  sortConversationsByRecency,
  totalUnreadConversations,
  upsertConversation,
  CLOSED_IM_CONVERSATION_STATUS,
} from "./support-conversation-list.ts"

const AISERVING = 1
const PENDING = 2
const ACTIVE = 3
const CLOSED = 4

function makeConversation(overrides = {}) {
  return {
    id: 1,
    channelId: 1,
    customerName: "Alice",
    status: ACTIVE,
    serviceMode: 3,
    priority: 0,
    currentAssigneeId: 0,
    lastMessageId: 0,
    customerUnreadCount: 0,
    agentUnreadCount: 0,
    customerLastReadMessageId: 0,
    agentLastReadMessageId: 0,
    closedBy: 0,
    ...overrides,
  }
}

test("the mirrored closed status still matches the generated backend enum", async () => {
  const source = await readFile(
    new URL("./generated/enums.ts", import.meta.url),
    "utf8"
  )
  const block = source.match(/export enum IMConversationStatus \{[^}]*\}/)
  assert.ok(block, "expected an IMConversationStatus enum in the generated file")

  const values = Object.fromEntries(
    [...block[0].matchAll(/(\w+)\s*=\s*(\d+)/g)].map((match) => [
      match[1],
      Number(match[2]),
    ])
  )
  assert.equal(values.Closed, CLOSED_IM_CONVERSATION_STATUS)
  // The server counts a conversation open while it is any of these three.
  assert.deepEqual(
    { serving: values.AIServing, pending: values.Pending, active: values.Active },
    { serving: AISERVING, pending: PENDING, active: ACTIVE }
  )
})

test("anything not closed is still an open request", () => {
  assert.equal(isOpenImConversation(makeConversation({ status: AISERVING })), true)
  assert.equal(isOpenImConversation(makeConversation({ status: PENDING })), true)
  assert.equal(isOpenImConversation(makeConversation({ status: ACTIVE })), true)
  assert.equal(isOpenImConversation(makeConversation({ status: CLOSED })), false)
  // A status the server adds later defaults to open rather than being mistaken
  // for a finished request.
  assert.equal(isOpenImConversation(makeConversation({ status: 99 })), true)
})

test("a closed request is not open, and neither is nothing", () => {
  assert.equal(isOpenImConversation(null), false)
  assert.equal(isOpenImConversation(undefined), false)
})

test("sorting puts the most recently active request first", () => {
  const list = [
    makeConversation({ id: 1, lastActiveAt: "2026-10-01 10:00:00" }),
    makeConversation({ id: 2, lastActiveAt: "2026-10-03 10:00:00" }),
    makeConversation({ id: 3, lastActiveAt: "2026-10-02 10:00:00" }),
  ]

  assert.deepEqual(
    sortConversationsByRecency(list).map((item) => item.id),
    [2, 3, 1]
  )
})

// The server writes time.DateTime, which has no timezone marker. Parsing it as
// a bare local timestamp keeps it consistent with formatDateTime elsewhere.
test("server timestamps without a zone still order correctly", () => {
  const list = [
    makeConversation({ id: 1, lastActiveAt: "2026-10-01 09:00:00" }),
    makeConversation({ id: 2, lastActiveAt: "2026-10-01 11:00:00" }),
  ]

  assert.deepEqual(
    sortConversationsByRecency(list).map((item) => item.id),
    [2, 1]
  )
})

test("a brand new request with no timestamps sorts ahead by id", () => {
  const list = [
    makeConversation({ id: 9 }),
    makeConversation({ id: 12 }),
    makeConversation({ id: 3, lastActiveAt: "2026-10-01 09:00:00" }),
  ]

  assert.deepEqual(
    sortConversationsByRecency(list).map((item) => item.id),
    [12, 9, 3]
  )
})

test("an unparsable timestamp does not become NaN and scramble the order", () => {
  const list = [
    makeConversation({ id: 1, lastActiveAt: "not-a-date" }),
    makeConversation({ id: 2, lastActiveAt: "2026-10-01 09:00:00" }),
  ]

  assert.deepEqual(
    sortConversationsByRecency(list).map((item) => item.id),
    [2, 1]
  )
})

test("lastMessageAt stands in when lastActiveAt is missing", () => {
  const list = [
    makeConversation({ id: 1, lastMessageAt: "2026-10-01 08:00:00" }),
    makeConversation({ id: 2, lastMessageAt: "2026-10-01 10:00:00" }),
  ]

  assert.deepEqual(
    sortConversationsByRecency(list).map((item) => item.id),
    [2, 1]
  )
})

test("upsert merges an existing request instead of duplicating it", () => {
  const list = [makeConversation({ id: 1, subject: "old" })]

  const next = upsertConversation(list, makeConversation({ id: 1, subject: "new" }))

  assert.equal(next.length, 1)
  assert.equal(next[0].subject, "new")
})

test("upsert keeps fields the patch does not carry", () => {
  const list = [makeConversation({ id: 1, subject: "kept", customerUnreadCount: 4 })]

  const next = upsertConversation(list, { id: 1, status: CLOSED })

  assert.equal(next[0].subject, "kept")
  assert.equal(next[0].customerUnreadCount, 4)
  assert.equal(next[0].status, CLOSED)
})

test("upserting a new request puts it at the top", () => {
  const list = [makeConversation({ id: 1, lastActiveAt: "2026-10-01 09:00:00" })]

  const next = upsertConversation(list, makeConversation({ id: 2 }))

  assert.deepEqual(next.map((item) => item.id), [2, 1])
})

test("upsert does not mutate the input", () => {
  const list = [makeConversation({ id: 1 })]

  upsertConversation(list, makeConversation({ id: 2 }))

  assert.equal(list.length, 1)
})

test("findConversationById returns null rather than throwing", () => {
  const list = [makeConversation({ id: 7 })]

  assert.equal(findConversationById(list, 7).id, 7)
  assert.equal(findConversationById(list, 8), null)
  assert.equal(findConversationById(list, 0), null)
})

test("resume picks the newest open request, skipping closed ones", () => {
  const list = [
    makeConversation({ id: 1, status: CLOSED, lastActiveAt: "2026-10-05 10:00:00" }),
    makeConversation({ id: 2, status: ACTIVE, lastActiveAt: "2026-10-01 10:00:00" }),
  ]

  assert.equal(pickConversationToResume(list).id, 2)
})

test("resume returns null when everything is closed so a new one gets started", () => {
  const list = [
    makeConversation({ id: 1, status: CLOSED }),
    makeConversation({ id: 2, status: CLOSED }),
  ]

  assert.equal(pickConversationToResume(list), null)
})

test("resume returns null for a visitor who never wrote to us", () => {
  assert.equal(pickConversationToResume([]), null)
})

test("title prefers the visitor's subject", () => {
  const conversation = makeConversation({
    subject: "Проблема с оплатой",
    lastMessageSummary: "Здравствуйте",
  })

  assert.equal(resolveConversationTitle(conversation, "Запрос"), "Проблема с оплатой")
})

test("title falls back to the last message, then to the localized label", () => {
  assert.equal(
    resolveConversationTitle(makeConversation({ lastMessageSummary: "Здравствуйте" }), "Запрос"),
    "Здравствуйте"
  )
  assert.equal(resolveConversationTitle(makeConversation(), "Запрос"), "Запрос")
  assert.equal(resolveConversationTitle(null, "Запрос"), "Запрос")
})

test("a whitespace-only subject is treated as absent", () => {
  assert.equal(
    resolveConversationTitle(makeConversation({ subject: "   ", lastMessageSummary: "Привет" }), "Запрос"),
    "Привет"
  )
})

test("unread totals across requests include the ones not being viewed", () => {
  const list = [
    makeConversation({ id: 1, customerUnreadCount: 2 }),
    makeConversation({ id: 2, customerUnreadCount: 3 }),
    makeConversation({ id: 3, customerUnreadCount: 0 }),
  ]

  assert.equal(totalUnreadConversations(list), 5)
})

test("a missing unread count counts as zero", () => {
  assert.equal(
    totalUnreadConversations([
      makeConversation({ id: 1 }),
      { ...makeConversation({ id: 2 }), customerUnreadCount: undefined },
    ]),
    0
  )
})