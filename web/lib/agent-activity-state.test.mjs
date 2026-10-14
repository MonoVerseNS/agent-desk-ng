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

test("clears activity state when status is failed", () => {
  const current = { conversationId: 42, stepCode: "thinking" }
  const event = {
    conversationId: 42,
    stepCode: "failed",
    status: "failed",
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

test("preserves existing activity when event is for another conversation", () => {
  const current = { conversationId: 42, stepCode: "thinking" }
  const event = {
    conversationId: 99,
    stepCode: "calling_tool",
    status: "running",
  }
  const next = resolveAgentActivityState(current, 42, event)
  assert.deepEqual(next, current)
})

test("preserves toolName when provided", () => {
  const current = null
  const event = {
    conversationId: 42,
    stepCode: "calling_tool",
    toolName: "web_search",
    status: "running",
  }
  const next = resolveAgentActivityState(current, 42, event)
  assert.deepEqual(next, {
    conversationId: 42,
    stepCode: "calling_tool",
    toolName: "web_search",
  })
})
