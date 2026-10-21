/** Motion timings from the palette's tokens (--dur-*, --ease*), for motion/react
 *  transitions. Read at call time, so a palette change applies to the next animation. */
type Bezier = [number, number, number, number]

const DUR = { fast: ['--dur-fast', 0.15], base: ['--dur', 0.22], slow: ['--dur-slow', 0.35] } as const
const EASE = { base: ['--ease', [0.2, 0.8, 0.2, 1]], spring: ['--ease-spring', [0.2, 1.4, 0.4, 1]] } as const

function token(name: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

/** Seconds. */
export function dur(kind: keyof typeof DUR): number {
  const [name, fallback] = DUR[kind]
  const m = /^([\d.]+)(ms|s)$/.exec(token(name))
  return m ? Number(m[1]) / (m[2] === 'ms' ? 1000 : 1) : fallback
}

export function ease(kind: keyof typeof EASE): Bezier {
  const [name, fallback] = EASE[kind]
  const n = /cubic-bezier\(([^)]+)\)/.exec(token(name))?.[1].split(',').map(Number)
  return n?.length === 4 && n.every(Number.isFinite) ? (n as Bezier) : [...fallback]
}

export function tx(kind: keyof typeof DUR = 'base', e: keyof typeof EASE = 'base') {
  return { duration: dur(kind), ease: ease(e) }
}
