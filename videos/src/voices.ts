import type { Lang } from './types'

// Microsoft's neural voices (free, through edge-tts). Swap a name here and run
// `bun run voice --lang <lang> --force` — samples of both voices per language
// are in out/samples/.
export const VOICE: Record<Lang, string> = {
  si: 'si-LK-ThiliniNeural',
  ta: 'ta-LK-SaranyaNeural',
  en: 'en-IN-NeerjaNeural',
}

export const SAMPLE_VOICES: Record<Lang, string[]> = {
  si: ['si-LK-ThiliniNeural', 'si-LK-SameeraNeural'],
  ta: ['ta-LK-SaranyaNeural', 'ta-LK-KumarNeural'],
  en: ['en-IN-NeerjaNeural', 'en-IN-PrabhatNeural', 'en-GB-SoniaNeural', 'en-GB-RyanNeural'],
}
