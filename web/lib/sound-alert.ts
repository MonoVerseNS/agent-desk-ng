let sharedAudioContext: AudioContext | null = null

/**
 * Resets the cached AudioContext instance (used for testing).
 */
export function resetAudioContextForTesting(): void {
  sharedAudioContext = null
}

function getOrCreateAudioContext(): AudioContext | null {
  if (typeof window === "undefined") {
    return null
  }

  const AudioContextClass =
    window.AudioContext ||
    (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext

  if (!AudioContextClass) {
    return null
  }

  if (!sharedAudioContext || sharedAudioContext.state === "closed") {
    try {
      sharedAudioContext = new AudioContextClass()
    } catch {
      return null
    }
  }

  return sharedAudioContext
}

function scheduleTone(
  ctx: AudioContext,
  freq: number,
  startTime: number,
  duration: number,
  peakGain = 0.15
): void {
  const osc = ctx.createOscillator()
  const gain = ctx.createGain()

  osc.type = "sine"
  osc.frequency.setValueAtTime(freq, startTime)

  gain.gain.setValueAtTime(peakGain, startTime)
  gain.gain.exponentialRampToValueAtTime(0.001, startTime + duration)

  osc.connect(gain)
  gain.connect(ctx.destination)

  osc.start(startTime)
  osc.stop(startTime + duration)
}

/**
 * Plays a gentle dual-tone chime (587.33 Hz D5 -> 880.00 Hz A5) using the Web Audio API.
 * Safely handles SSR/Node environments, unsupported browsers, suspended contexts, and
 * browser autoplay policy rejections without throwing errors.
 *
 * @returns boolean indicating whether the sound playback was successfully initiated.
 */
export function playIncomingMessageSound(): boolean {
  try {
    const ctx = getOrCreateAudioContext()
    if (!ctx) {
      return false
    }

    if (ctx.state === "suspended") {
      ctx.resume().catch(() => {
        // Traps browser autoplay rejection without crashing
      })
    }

    const now = ctx.currentTime
    const note1Duration = 0.08
    const note2Duration = 0.16

    // Note 1: 587.33 Hz (D5) for 0.08s
    scheduleTone(ctx, 587.33, now, note1Duration)

    // Note 2: 880.00 Hz (A5) for 0.16s
    scheduleTone(ctx, 880.00, now + note1Duration, note2Duration)

    return true
  } catch {
    return false
  }
}
