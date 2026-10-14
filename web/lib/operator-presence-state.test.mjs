import test from "node:test"
import assert from "node:assert/strict"
import {
  updateOperatorPresenceList,
  purgeExpiredOperatorPresences,
  resolveOperatorPresence,
} from "./operator-presence-state.ts"

test("adds a new peer operator record with current timestamp", () => {
  const currentList = []
  const incoming = {
    conversationId: 101,
    userId: 2,
    username: "alice",
    nickname: "Alice Admin",
    action: "viewing",
  }
  const now = 100_000
  const result = updateOperatorPresenceList(currentList, incoming, 1, now)

  assert.equal(result.length, 1)
  assert.deepEqual(result[0], {
    conversationId: 101,
    userId: 2,
    username: "alice",
    nickname: "Alice Admin",
    action: "viewing",
    at: now,
  })
})

test("ignores incoming events from the current user (self)", () => {
  const currentList = [
    {
      conversationId: 101,
      userId: 2,
      username: "alice",
      nickname: "Alice Admin",
      action: "viewing",
      at: 100_000,
    },
  ]
  const incoming = {
    conversationId: 101,
    userId: 1, // current user ID
    username: "myself",
    nickname: "Me",
    action: "typing",
  }
  const result = updateOperatorPresenceList(currentList, incoming, 1, 101_000)

  assert.equal(result, currentList)
})

test("updates action from viewing to typing and updates timestamp", () => {
  const currentList = [
    {
      conversationId: 101,
      userId: 2,
      username: "alice",
      nickname: "Alice Admin",
      action: "viewing",
      at: 100_000,
    },
  ]
  const incoming = {
    conversationId: 101,
    userId: 2,
    username: "alice",
    nickname: "Alice Admin",
    action: "typing",
  }
  const now = 103_000
  const result = updateOperatorPresenceList(currentList, incoming, 1, now)

  assert.equal(result.length, 1)
  assert.equal(result[0].action, "typing")
  assert.equal(result[0].at, now)
})

test("removes operator record on action left", () => {
  const currentList = [
    {
      conversationId: 101,
      userId: 2,
      username: "alice",
      nickname: "Alice Admin",
      action: "viewing",
      at: 100_000,
    },
    {
      conversationId: 101,
      userId: 3,
      username: "bob",
      nickname: "Bob Operator",
      action: "viewing",
      at: 101_000,
    },
  ]
  const incoming = {
    conversationId: 101,
    userId: 2,
    username: "alice",
    nickname: "Alice Admin",
    action: "left",
  }
  const now = 102_000
  const result = updateOperatorPresenceList(currentList, incoming, 1, now)

  assert.equal(result.length, 1)
  assert.equal(result[0].userId, 3)
})

test("automatically filters out records older than 15 seconds during update", () => {
  const currentList = [
    {
      conversationId: 101,
      userId: 2,
      username: "alice",
      nickname: "Alice",
      action: "viewing",
      at: 80_000, // 25s old at now = 105_000 (> 15s)
    },
    {
      conversationId: 101,
      userId: 3,
      username: "bob",
      nickname: "Bob",
      action: "viewing",
      at: 95_000, // 10s old at now = 105_000 (valid)
    },
  ]
  const incoming = {
    conversationId: 101,
    userId: 4,
    username: "charlie",
    nickname: "Charlie",
    action: "typing",
  }
  const now = 105_000
  const result = updateOperatorPresenceList(currentList, incoming, 1, now)

  assert.equal(result.length, 2)
  const userIds = result.map((r) => r.userId)
  assert.ok(!userIds.includes(2), "User 2 should be expired")
  assert.ok(userIds.includes(3), "User 3 should be kept")
  assert.ok(userIds.includes(4), "User 4 should be added")
})

test("purgeExpiredOperatorPresences filters out expired entries based on maxAgeMs", () => {
  const list = [
    {
      conversationId: 101,
      userId: 2,
      username: "alice",
      nickname: "Alice",
      action: "viewing",
      at: 84_000, // 16s ago at 100_000 -> expired
    },
    {
      conversationId: 101,
      userId: 3,
      username: "bob",
      nickname: "Bob",
      action: "viewing",
      at: 86_000, // 14s ago at 100_000 -> kept
    },
  ]
  const purged = purgeExpiredOperatorPresences(list, 100_000, 15_000)
  assert.equal(purged.length, 1)
  assert.equal(purged[0].userId, 3)
})

test("resolveOperatorPresence alias works identically to updateOperatorPresenceList", () => {
  const currentList = []
  const incoming = {
    conversationId: 101,
    userId: 2,
    username: "alice",
    nickname: "Alice",
    action: "viewing",
  }
  const now = 50_000
  const result = resolveOperatorPresence(currentList, incoming, 1, now)
  assert.equal(result.length, 1)
  assert.equal(result[0].userId, 2)
})
