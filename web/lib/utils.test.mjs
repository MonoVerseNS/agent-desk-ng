import assert from "node:assert/strict"
import test from "node:test"

import { parseApiTimestamp } from "./utils.ts"

test("parseApiTimestamp parses date without timezone as UTC", () => {
  const result = parseApiTimestamp("2026-10-11 14:00:00")
  assert.equal(result, Date.parse("2026-10-11T14:00:00Z"))
})

test("parseApiTimestamp preserves explicit UTC Z", () => {
  const result = parseApiTimestamp("2026-10-11T14:00:00Z")
  assert.equal(result, Date.parse("2026-10-11T14:00:00Z"))
})

test("parseApiTimestamp preserves explicit offset", () => {
  const result = parseApiTimestamp("2026-10-11T17:00:00+03:00")
  assert.equal(result, Date.parse("2026-10-11T17:00:00+03:00"))
  assert.equal(result, Date.parse("2026-10-11T14:00:00Z"))
})

test("parseApiTimestamp handles Date, number, empty and invalid values", () => {
  assert.equal(parseApiTimestamp(null), 0)
  assert.equal(parseApiTimestamp(""), 0)
  assert.equal(parseApiTimestamp("   "), 0)
  assert.equal(parseApiTimestamp("invalid-date"), 0)
  const now = Date.now()
  assert.equal(parseApiTimestamp(now), now)
  const d = new Date()
  assert.equal(parseApiTimestamp(d), d.getTime())
})
