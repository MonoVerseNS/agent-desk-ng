import assert from "node:assert/strict"
import test from "node:test"

import { createRealtimeConnectionManager } from "./realtime-connection.ts"

// Minimal WebSocket mock for testing connection manager lifecycle
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  constructor() {
    this.readyState = MockWebSocket.CONNECTING
    this.listeners = new Map()
  }

  addEventListener(type, listener) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, [])
    }
    this.listeners.get(type).push(listener)
  }

  emit(type, event = {}) {
    const list = this.listeners.get(type) ?? []
    for (const listener of list) {
      listener(event)
    }
  }

  open() {
    this.readyState = MockWebSocket.OPEN
    this.emit("open")
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.emit("close", { code: 1000, reason: "Normal" })
  }

  send() {}
}

test("beforeConnect is called and awaited before createSocket", async () => {
  let beforeConnectCalled = false
  let socketCreated = false
  let mockSocket = null

  const manager = createRealtimeConnectionManager({
    beforeConnect: async () => {
      await new Promise((resolve) => setTimeout(resolve, 10))
      beforeConnectCalled = true
    },
    createSocket: () => {
      assert.ok(beforeConnectCalled, "expected beforeConnect to run before createSocket")
      socketCreated = true
      mockSocket = new MockWebSocket()
      return mockSocket
    },
  })

  manager.connect()
  // Wait for beforeConnect promise to resolve
  await new Promise((resolve) => setTimeout(resolve, 30))

  assert.ok(beforeConnectCalled, "beforeConnect should be called")
  assert.ok(socketCreated, "createSocket should be called")
  assert.ok(mockSocket, "socket should be created")
  manager.disconnect()
})

test("beforeConnect failure triggers onConnectError and disconnect status", async () => {
  let connectError = null
  let statusList = []

  const manager = createRealtimeConnectionManager({
    beforeConnect: async () => {
      throw new Error("token expired")
    },
    createSocket: () => {
      assert.fail("createSocket must not be called when beforeConnect fails")
    },
    onConnectError: (err) => {
      connectError = err
    },
    onStatusChange: (status) => {
      statusList.push(status)
    },
  })

  manager.connect()
  await new Promise((resolve) => setTimeout(resolve, 20))

  assert.ok(connectError, "onConnectError should be called")
  assert.equal(connectError.message, "token expired")
  assert.ok(statusList.includes("disconnected"), "status should include disconnected")
  manager.disconnect()
})
