export type Lang = 'en' | 'si' | 'ta'
export const LANGS: Lang[] = ['si', 'ta', 'en']

/** Where the camera settles: the centre (0..1 of the picture) and how close (1 = whole picture). */
export type Focus = { x: number; y: number; zoom: number }

export type Visual =
  | { kind: 'clip'; src: string; focus?: Focus; from?: number } // public/clips/<src>.webm, from = seconds in
  | { kind: 'shot'; src: string; focus?: Focus } // docs/images/<src>.jpg (copied to public/shots)
  | { kind: 'problem' } // the owner's headache: a big line on the palette, no screen

export type Scene = { id: string; visual: Visual; addon?: boolean; note?: 'internet' }

export type VideoId = 'promo' | 'away' | 'buying' | 'credit' | 'staff' | 'suppliers' | 'profit' | 'safe' | 'addons'
/** A video's scenes; the end card is added by the composition. */
export type Video = { id: VideoId; scenes: Scene[] }

/** caption = what's on screen (short); voice = what the computer voice reads. */
export type Line = { caption: string; voice: string }
/** Scene id → its line, plus the shared ones in SHARED_LINES. */
export type Lines = Record<string, Line>

/** Lines every language has besides the scenes'. */
export const SHARED_LINES = ['end', 'end-soon', 'soon-title', 'addon-tag', 'internet-note'] as const
/** Shared lines that are only shown, never read out. */
export const SILENT_LINES = ['soon-title', 'addon-tag', 'internet-note']
