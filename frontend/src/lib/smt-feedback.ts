// Shop-floor feedback for the SMT workstation: audible beeps and voice prompts.
// Browsers require a user gesture before audio can start, so every call is
// wrapped in try/catch and silently degrades when the environment blocks it.

export type SMTFeedbackKind = 'OK' | 'ERROR' | 'CONFIRM'

const toneByKind: Record<SMTFeedbackKind, { frequency: number; duration: number; repeats: number }> = {
  OK: { frequency: 880, duration: 0.12, repeats: 1 },
  CONFIRM: { frequency: 520, duration: 0.18, repeats: 2 },
  ERROR: { frequency: 220, duration: 0.28, repeats: 2 },
}

type AudioWindow = Window & { webkitAudioContext?: typeof AudioContext }

export function smtBeep(kind: SMTFeedbackKind): void {
  if (typeof window === 'undefined') return
  const scope = window as AudioWindow
  const Ctor = window.AudioContext ?? scope.webkitAudioContext
  if (!Ctor) return
  try {
    const context = new Ctor()
    const tone = toneByKind[kind]
    for (let index = 0; index < tone.repeats; index += 1) {
      const start = context.currentTime + index * (tone.duration + 0.06)
      const oscillator = context.createOscillator()
      const gain = context.createGain()
      oscillator.type = 'sine'
      oscillator.frequency.value = tone.frequency
      gain.gain.setValueAtTime(0.0001, start)
      gain.gain.linearRampToValueAtTime(0.09, start + 0.01)
      gain.gain.linearRampToValueAtTime(0.0001, start + tone.duration)
      oscillator.connect(gain)
      gain.connect(context.destination)
      oscillator.start(start)
      oscillator.stop(start + tone.duration + 0.02)
    }
    window.setTimeout(() => void context.close().catch(() => undefined), tone.repeats * 600 + 400)
  } catch {
    // Audio is best effort; the visual banner always carries the result.
  }
}

export function smtSpeak(message: string): void {
  if (!message || typeof window === 'undefined' || !('speechSynthesis' in window)) return
  try {
    const utterance = new SpeechSynthesisUtterance(message)
    utterance.lang = /[一-鿿]/.test(message) ? 'zh-CN' : 'en-US'
    utterance.rate = 1.05
    window.speechSynthesis.speak(utterance)
  } catch {
    // Ignore speech failures (unsupported voices, autoplay policies).
  }
}
