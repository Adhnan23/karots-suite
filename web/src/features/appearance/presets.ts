/** One-tap presets the editor offers (spec §4): a material fills the surface tokens,
 *  a motion feel fills the timing tokens. Values follow the built-ins (solid = Studio,
 *  frosted = Night, liquid glass = Lagoon) in neutral tints, so they suit any colours. */
type Tokens = Record<string, string>
type Mode = 'light' | 'dark'
export type Material = 'solid' | 'frosted' | 'liquid'
export type Feel = 'calm' | 'lively'

const MATERIAL: Record<Material, Record<Mode, Tokens>> = {
  solid: {
    light: {
      '--card': 'color-mix(in srgb, #FFFFFF 92%, var(--background))', '--bar': 'var(--card)', '--glass': 'none',
      '--shadow': '0 1px 0 rgb(0 0 0 / .03), 0 6px 18px rgb(0 0 0 / .06)', '--shadow-float': '0 1px 0 rgb(0 0 0 / .04), 0 16px 40px rgb(0 0 0 / .14)',
    },
    dark: {
      '--card': 'color-mix(in srgb, var(--foreground) 6%, var(--background))', '--bar': 'var(--card)', '--glass': 'none',
      '--shadow': 'inset 0 1px 0 rgb(255 255 255 / .03), 0 10px 30px rgb(0 0 0 / .35)', '--shadow-float': 'inset 0 1px 0 rgb(255 255 255 / .05), 0 18px 50px rgb(0 0 0 / .5)',
    },
  },
  frosted: {
    light: {
      '--card': 'rgb(255 255 255 / .62)', '--bar': 'rgb(255 255 255 / .66)', '--glass': 'blur(18px) saturate(160%)',
      '--shadow': 'inset 0 1px 0 rgb(255 255 255 / .7), 0 10px 30px rgb(0 0 0 / .08)', '--shadow-float': 'inset 0 1px 0 rgb(255 255 255 / .8), 0 18px 50px rgb(0 0 0 / .14)',
    },
    dark: {
      '--card': 'rgb(255 255 255 / .055)', '--bar': 'color-mix(in srgb, var(--background) 72%, transparent)', '--glass': 'blur(18px) saturate(140%)',
      '--shadow': 'inset 0 1px 0 rgb(255 255 255 / .06), 0 10px 30px rgb(0 0 0 / .35)', '--shadow-float': 'inset 0 1px 0 rgb(255 255 255 / .08), 0 18px 50px rgb(0 0 0 / .5)',
    },
  },
  liquid: {
    light: {
      '--card': 'rgb(255 255 255 / .42)', '--bar': 'rgb(255 255 255 / .48)', '--glass': 'blur(22px) saturate(180%)',
      '--shadow': 'inset 0 1px 0 rgb(255 255 255 / .9), inset 0 0 0 1px rgb(255 255 255 / .35), 0 8px 30px rgb(0 0 0 / .08)',
      '--shadow-float': 'inset 0 1px 0 rgb(255 255 255 / .95), inset 0 0 0 1px rgb(255 255 255 / .45), 0 18px 50px rgb(0 0 0 / .14)',
    },
    dark: {
      '--card': 'rgb(255 255 255 / .06)', '--bar': 'color-mix(in srgb, var(--background) 62%, transparent)', '--glass': 'blur(22px) saturate(160%)',
      '--shadow': 'inset 0 1px 0 rgb(255 255 255 / .12), inset 0 0 0 1px rgb(255 255 255 / .06), 0 10px 30px rgb(0 0 0 / .4)',
      '--shadow-float': 'inset 0 1px 0 rgb(255 255 255 / .14), inset 0 0 0 1px rgb(255 255 255 / .08), 0 18px 50px rgb(0 0 0 / .55)',
    },
  },
}

export function applyMaterial(t: Tokens, m: Material, mode: Mode): Tokens {
  return { ...t, ...MATERIAL[m][mode] }
}

export function blurOf(t: Tokens): number {
  return Number(/blur\((\d+)px\)/.exec(t['--glass'] ?? '')?.[1] ?? 0)
}

export function materialOf(t: Tokens): Material {
  const b = blurOf(t)
  return b === 0 ? 'solid' : b >= 20 ? 'liquid' : 'frosted'
}

/** Changes the glass blur (0 = no glass). */
export function withBlur(t: Tokens, px: number): Tokens {
  const g = t['--glass'] ?? 'none'
  if (px <= 0) return { ...t, '--glass': 'none' }
  return { ...t, '--glass': /blur\(\d+px\)/.test(g) ? g.replace(/blur\(\d+px\)/, `blur(${px}px)`) : `blur(${px}px) saturate(160%)` }
}

const BASE = { calm: [150, 220, 350], lively: [150, 260, 350] } as const
const EASE = { calm: 'cubic-bezier(.2, .8, .2, 1)', lively: 'cubic-bezier(.2, 1.25, .4, 1)' }

/** speed multiplies every duration (0.5 = snappy, 1.5 = slow); nothing runs past 350 ms. */
export function applyMotion(t: Tokens, feel: Feel, speed: number): Tokens {
  const [fast, base, slow] = BASE[feel].map((ms) => `${Math.min(350, Math.round(ms * speed))}ms`)
  return { ...t, '--dur-fast': fast, '--dur': base, '--dur-slow': slow, '--ease': EASE[feel], '--lift': feel === 'lively' ? '2px' : '0px' }
}

export function motionOf(t: Tokens): { feel: Feel; speed: number } {
  const fast = parseFloat(t['--dur-fast'] ?? '150') || 150
  return { feel: (t['--lift'] ?? '0px') !== '0px' ? 'lively' : 'calm', speed: Math.round((fast / 150) * 20) / 20 }
}
