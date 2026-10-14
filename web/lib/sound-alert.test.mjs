import test from "node:test"
import assert from "node:assert/strict"
import {
  playIncomingMessageSound,
  resetAudioContextForTesting,
} from "./sound-alert.ts"

class MockAudioParam {
  constructor(defaultValue = 0) {
    this.value = defaultValue
    this.calls = []
  }
  setValueAtTime(val, time) {
    this.calls.push({ method: "setValueAtTime", val, time })
  }
  exponentialRampToValueAtTime(val, time) {
    this.calls.push({ method: "exponentialRampToValueAtTime", val, time })
  }
}

class MockAudioNode {
  constructor() {
    this.connectedTo = null
  }
  connect(target) {
    this.connectedTo = target
  }
}

class MockOscillatorNode extends MockAudioNode {
  constructor() {
    super()
    this.type = "sine"
    this.frequency = new MockAudioParam(440)
    this.startCalls = []
    this.stopCalls = []
  }
  start(time) {
    this.startCalls.push(time)
  }
  stop(time) {
    this.stopCalls.push(time)
  }
}

class MockGainNode extends MockAudioNode {
  constructor() {
    super()
    this.gain = new MockAudioParam(1)
  }
}

class MockAudioContext {
  constructor() {
    this.state = "running"
    this.currentTime = 5.0
    this.destination = new MockAudioNode()
    this.oscillators = []
    this.gains = []
    this.resumeCalled = false
    this.resumePromise = Promise.resolve()
  }
  createOscillator() {
    const osc = new MockOscillatorNode()
    this.oscillators.push(osc)
    return osc
  }
  createGain() {
    const gain = new MockGainNode()
    this.gains.push(gain)
    return gain
  }
  resume() {
    this.resumeCalled = true
    return this.resumePromise
  }
}

test("returns false safely in Node.js / SSR environment without window", () => {
  const originalWindow = globalThis.window
  try {
    delete globalThis.window
    resetAudioContextForTesting()
    const result = playIncomingMessageSound()
    assert.equal(result, false)
  } finally {
    if (originalWindow !== undefined) {
      globalThis.window = originalWindow
    }
  }
})

test("returns false safely when window exists but AudioContext is unsupported", () => {
  const originalWindow = globalThis.window
  try {
    globalThis.window = {}
    resetAudioContextForTesting()
    const result = playIncomingMessageSound()
    assert.equal(result, false)
  } finally {
    if (originalWindow !== undefined) {
      globalThis.window = originalWindow
    } else {
      delete globalThis.window
    }
  }
})

test("synthesizes dual-tone chime (587.33Hz and 880.00Hz) with gain ramps", () => {
  const originalWindow = globalThis.window
  try {
    let mockCtx = null
    globalThis.window = {
      AudioContext: class {
        constructor() {
          mockCtx = new MockAudioContext()
          return mockCtx
        }
      },
    }
    resetAudioContextForTesting()

    const result = playIncomingMessageSound()
    assert.equal(result, true)
    assert.ok(mockCtx, "AudioContext should be created")

    // Verify 2 oscillators created
    assert.equal(mockCtx.oscillators.length, 2)
    const [osc1, osc2] = mockCtx.oscillators

    // Note 1: 587.33 Hz (D5) for 0.08s
    assert.equal(osc1.type, "sine")
    assert.equal(osc1.frequency.calls[0].val, 587.33)
    assert.equal(osc1.frequency.calls[0].time, 5.0)
    assert.equal(osc1.startCalls[0], 5.0)
    assert.equal(osc1.stopCalls[0], 5.08)

    // Note 2: 880.00 Hz (A5) for 0.16s
    assert.equal(osc2.type, "sine")
    assert.equal(osc2.frequency.calls[0].val, 880.00)
    assert.equal(osc2.frequency.calls[0].time, 5.08)
    assert.equal(osc2.startCalls[0], 5.08)
    assert.equal(osc2.stopCalls[0], 5.08 + 0.16)

    // Verify 2 gains with exponentialRampToValueAtTime
    assert.equal(mockCtx.gains.length, 2)
    const [gain1, gain2] = mockCtx.gains
    const ramp1 = gain1.gain.calls.find(c => c.method === "exponentialRampToValueAtTime")
    const ramp2 = gain2.gain.calls.find(c => c.method === "exponentialRampToValueAtTime")
    assert.ok(ramp1, "gain1 should have exponential ramp")
    assert.ok(ramp2, "gain2 should have exponential ramp")
    assert.equal(ramp1.time, 5.08)
    assert.equal(ramp2.time, 5.24)
  } finally {
    if (originalWindow !== undefined) {
      globalThis.window = originalWindow
    } else {
      delete globalThis.window
    }
  }
})

test("handles suspended AudioContext and catches rejected resume() safely", async () => {
  const originalWindow = globalThis.window
  try {
    let mockCtx = null
    globalThis.window = {
      AudioContext: class {
        constructor() {
          mockCtx = new MockAudioContext()
          mockCtx.state = "suspended"
          mockCtx.resumePromise = Promise.reject(new Error("Autoplay not allowed"))
          return mockCtx
        }
      },
    }
    resetAudioContextForTesting()

    // Should not throw or crash on autoplay rejection
    const result = playIncomingMessageSound()
    assert.equal(result, true)
    assert.equal(mockCtx.resumeCalled, true)

    // Wait a tick for promise rejection to settle
    await new Promise((resolve) => setTimeout(resolve, 10))
  } finally {
    if (originalWindow !== undefined) {
      globalThis.window = originalWindow
    } else {
      delete globalThis.window
    }
  }
})
