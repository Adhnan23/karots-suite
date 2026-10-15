import { X } from 'lucide-react'
import { useEffect, useRef } from 'react'

const BEST = 'ks-coin-best'
const VALUES = [1, 2, 5, 10]
const LIVES = 3

/** v 0 = a heart: catch it for a life back (missing one costs nothing). */
type Coin = { x: number; y: number; r: number; v: number; vy: number }

/** A theme token resolved to a colour the canvas can paint with. */
function token(name: string) {
  const el = document.createElement('span')
  el.style.color = `var(${name})`
  document.body.appendChild(el)
  const c = getComputedStyle(el).color
  el.remove()
  return c
}

function best() {
  try { return Number(localStorage.getItem(BEST)) || 0 } catch { return 0 }
}

/** The login / lock easter egg: catch the falling coins in the till. Drag, move the
 * mouse or use ← →; three misses and it's over. Esc (or ✕) closes. Best score is
 * this device's only. It only paints over the page — signing in is untouched. */
export default function CoinGame({ onClose }: { onClose: () => void }) {
  const canvas = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const cv = canvas.current!
    const ctx = cv.getContext('2d')!
    const col = { bg: token('--hero'), fg: token('--hero-foreground'), muted: token('--hero-muted'), coin: token('--brand'), coinFg: token('--brand-foreground'), bad: token('--danger') }
    let W = 0, H = 0
    const size = () => {
      const dpr = window.devicePixelRatio || 1
      W = window.innerWidth; H = window.innerHeight
      cv.width = W * dpr; cv.height = H * dpr
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }
    size()

    let state: 'ready' | 'play' | 'over' = 'ready'
    let coins: Coin[] = [], score = 0, lives = LIVES, top = best(), spawn = 0, flash = 0
    let x = W / 2, keyDir = 0, last = performance.now(), raf = 0
    // Fair drops: coins land one after another (clock in frames), each within reach
    // of the last at the drawer's keyboard speed — never two you can't both catch.
    const SPEED = 12, GAP = 16
    let clock = 0, lastLand = 0, lastX = W / 2
    const tray = () => ({ w: Math.min(170, W * 0.3), h: 26, y: H - 70 })

    const start = () => { coins = []; score = 0; lives = LIVES; spawn = 0; state = 'play'; lastLand = clock; lastX = x }
    const over = () => {
      state = 'over'
      if (score > top) { top = score; try { localStorage.setItem(BEST, String(top)) } catch { /* private mode */ } }
    }

    function step(now: number) {
      const dt = Math.min(50, now - last) / 16.67
      last = now
      clock += dt
      const t = tray()
      x = Math.max(t.w / 2, Math.min(W - t.w / 2, x + keyDir * 12 * dt))
      if (state === 'play') {
        // Faster and busier as the till fills.
        spawn -= dt * 16.67
        if (spawn <= 0) {
          const v = VALUES[Math.floor(Math.random() * VALUES.length)]
          const r = 16 + VALUES.indexOf(v) * 5
          const fall = t.y + r // from just above the screen to the drawer
          let vy = 3 + Math.random() * 1.5 + Math.min(6, score / 60)
          let land = clock + fall / vy
          if (land < lastLand + GAP) { land = lastLand + GAP; vy = fall / (land - clock) }
          const reach = SPEED * (land - lastLand) * 0.75 + t.w * 0.4
          const cx = Math.max(r, Math.min(W - r, lastX + (Math.random() * 2 - 1) * reach))
          // Now and then, when a life is down, a heart falls instead.
          const heart = lives < LIVES && !coins.some((c) => c.v === 0) && Math.random() < 0.12
          coins.push({ x: cx, y: -r, r: heart ? 20 : r, v: heart ? 0 : v, vy })
          lastLand = land
          lastX = cx
          spawn = Math.max(330, 900 - score * 3)
        }
        for (const c of coins) c.y += c.vy * dt
        coins = coins.filter((c) => {
          if (c.y + c.r >= t.y && c.y - c.r <= t.y + t.h && Math.abs(c.x - x) <= t.w / 2 + c.r * 0.5) { if (c.v) score += c.v; else lives = Math.min(LIVES, lives + 1); flash = 8; return false }
          if (c.y - c.r > H) { if (c.v) { lives--; if (lives <= 0) over() } return false }
          return true
        })
      }
      draw(t)
      raf = requestAnimationFrame(step)
    }

    function text(s: string, px: number, y: number, color: string, align: CanvasTextAlign = 'center', weight = 800) {
      ctx.font = `${weight} ${px}px ui-sans-serif, system-ui, sans-serif`
      ctx.fillStyle = color
      ctx.textAlign = align
      ctx.fillText(s, align === 'center' ? W / 2 : align === 'left' ? 28 : W - 80, y)
    }

    function draw(t: ReturnType<typeof tray>) {
      ctx.fillStyle = col.bg
      ctx.fillRect(0, 0, W, H)
      for (const c of coins) {
        if (!c.v) {
          ctx.font = `${c.r * 2}px ui-sans-serif, system-ui, sans-serif`; ctx.fillStyle = col.bad; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'
          ctx.fillText('♥', c.x, c.y + 2); ctx.textBaseline = 'alphabetic'
          continue
        }
        ctx.beginPath(); ctx.arc(c.x, c.y, c.r, 0, Math.PI * 2); ctx.fillStyle = col.coin; ctx.fill()
        ctx.lineWidth = 2; ctx.strokeStyle = col.coinFg; ctx.globalAlpha = 0.35; ctx.beginPath(); ctx.arc(c.x, c.y, c.r - 4, 0, Math.PI * 2); ctx.stroke(); ctx.globalAlpha = 1
        ctx.font = `800 ${c.r * 0.9}px ui-sans-serif, system-ui, sans-serif`; ctx.fillStyle = col.coinFg; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'
        ctx.fillText(String(c.v), c.x, c.y + 1); ctx.textBaseline = 'alphabetic'
      }
      // The till drawer.
      ctx.fillStyle = flash > 0 ? col.coin : col.fg
      flash = Math.max(0, flash - 1)
      ctx.beginPath(); ctx.roundRect(x - t.w / 2, t.y, t.w, t.h, 8); ctx.fill()
      text(`Rs ${score}`, 34, 56, col.fg, 'left')
      text(`Best Rs ${top}`, 15, 82, col.muted, 'left', 600)
      text('●'.repeat(lives) + '○'.repeat(LIVES - lives), 22, 56, lives === 1 ? col.bad : col.fg, 'right')
      if (state === 'ready') {
        text('Catch the coins', 54, H / 2 - 30, col.fg)
        text('Catch coins, and ♥ for a life back  ·  drag, mouse or ← →  ·  tap or Space to start  ·  Esc to close', 18, H / 2 + 14, col.muted, 'center', 600)
      } else if (state === 'over') {
        text(`Rs ${score}`, 72, H / 2 - 30, col.coin)
        text(score >= top && score > 0 ? 'New best!' : `Best Rs ${top}`, 22, H / 2 + 12, col.fg, 'center', 700)
        text('Tap or Space to play again  ·  Esc to close', 18, H / 2 + 50, col.muted, 'center', 600)
      }
    }

    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.preventDefault(); onClose(); return }
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') { e.preventDefault(); keyDir = e.key === 'ArrowLeft' ? -1 : 1 }
      if (e.key === ' ' || e.key === 'Enter') { e.preventDefault(); if (state !== 'play') start() }
      e.stopPropagation()
    }
    const keyUp = (e: KeyboardEvent) => { if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') keyDir = 0 }
    const move = (e: PointerEvent) => { x = e.clientX }
    const down = (e: PointerEvent) => { x = e.clientX; if (state !== 'play') start() }
    window.addEventListener('keydown', key, true)
    window.addEventListener('keyup', keyUp, true)
    window.addEventListener('resize', size)
    cv.addEventListener('pointermove', move)
    cv.addEventListener('pointerdown', down)
    ;(document.activeElement as HTMLElement | null)?.blur()
    raf = requestAnimationFrame(step)
    return () => {
      cancelAnimationFrame(raf)
      window.removeEventListener('keydown', key, true)
      window.removeEventListener('keyup', keyUp, true)
      window.removeEventListener('resize', size)
    }
  }, [onClose])

  return (
    <div role="dialog" aria-label="Coin game" className="fixed inset-0 z-[100] touch-none">
      <canvas ref={canvas} className="block h-full w-full" />
      <button type="button" aria-label="Close the game" onClick={onClose}
        className="absolute top-5 right-5 grid size-11 place-items-center rounded-full bg-white/10 text-hero-foreground hover:bg-white/20">
        <X size={20} />
      </button>
    </div>
  )
}
