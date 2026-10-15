import { ArrowDown, ChevronLeft, ChevronRight, ChevronsDown, RotateCw, X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { readBest, saveBest, token } from './games'

const BEST = 'ks-blocks-best'
const COLS = 10
const ROWS = 20
// The seven pieces (rotation 0), as filled cells in a 4×4 box; colours are theme tokens.
const SHAPES: Record<string, number[][]> = {
  I: [[0, 1], [1, 1], [2, 1], [3, 1]],
  O: [[1, 0], [2, 0], [1, 1], [2, 1]],
  T: [[1, 0], [0, 1], [1, 1], [2, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
}
const KINDS = Object.keys(SHAPES)
const TINT: Record<string, string> = { I: '--chart-3', O: '--chart-1', T: '--foreground', S: '--chart-2', Z: '--chart-5', J: '--chart-4', L: '--chart-6' }
const LINE_SCORE = [0, 100, 300, 500, 800]

type Piece = { k: string; cells: number[][]; x: number; y: number }
export type Act = 'left' | 'right' | 'rotate' | 'down' | 'drop'

const rotate = (cells: number[][], k: string) =>
  k === 'O' ? cells : cells.map(([x, y]) => [k === 'I' ? 3 - y : 2 - y, x])

/** The second login / lock easter egg: falling blocks. ← → move, ↑ rotates, ↓ drops
 * a row, Space drops it all the way; on a touch screen, the buttons below. Clear
 * lines to score; it speeds up every 10 lines. Esc (or ✕) closes. */
export default function BlockGame({ onClose }: { onClose: () => void }) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const act = useRef<(a: Act) => void>(() => {})

  useEffect(() => {
    const cv = canvas.current!
    const ctx = cv.getContext('2d')!
    const col = { bg: token('--background'), fg: token('--foreground'), muted: token('--muted-foreground'), brand: token('--brand'), paper: token('--paper') }
    const tint = Object.fromEntries(KINDS.map((k) => [k, token(TINT[k])]))
    let W = 0, H = 0
    const size = () => {
      const dpr = window.devicePixelRatio || 1
      W = window.innerWidth; H = window.innerHeight
      cv.width = W * dpr; cv.height = H * dpr
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }
    size()

    let state: 'ready' | 'play' | 'over' = 'ready'
    let board: (string | null)[][] = [], bag: string[] = [], cur: Piece, next = ''
    let score = 0, lines = 0, top = readBest(BEST), fallMs = 0, last = performance.now(), raf = 0
    const level = () => Math.floor(lines / 10)
    const pick = () => {
      if (!bag.length) bag = [...KINDS].sort(() => Math.random() - 0.5) // 7-bag: no long droughts
      return bag.pop()!
    }
    const spawn = (k: string): Piece => ({ k, cells: SHAPES[k].map((c) => [...c]), x: 3, y: k === 'I' ? -1 : 0 })
    const fits = (cells: number[][], px: number, py: number) =>
      cells.every(([cx, cy]) => {
        const x = px + cx, y = py + cy
        return x >= 0 && x < COLS && y < ROWS && (y < 0 || !board[y][x])
      })
    const start = () => {
      board = Array.from({ length: ROWS }, () => Array<string | null>(COLS).fill(null))
      bag = []; score = 0; lines = 0; fallMs = 0
      cur = spawn(pick()); next = pick(); state = 'play'
    }
    const lock = () => {
      for (const [cx, cy] of cur.cells) {
        if (cur.y + cy < 0) { over(); return }
        board[cur.y + cy][cur.x + cx] = cur.k
      }
      const kept = board.filter((row) => row.some((c) => !c))
      const cleared = ROWS - kept.length
      board = [...Array.from({ length: cleared }, () => Array<string | null>(COLS).fill(null)), ...kept]
      lines += cleared
      score += LINE_SCORE[cleared] * (level() + 1)
      cur = spawn(next); next = pick()
      if (!fits(cur.cells, cur.x, cur.y)) over()
    }
    const over = () => {
      state = 'over'
      if (score > top) { top = score; saveBest(BEST, top) }
    }
    act.current = (a: Act) => {
      if (state !== 'play') { if (a === 'drop' || a === 'rotate') start(); return }
      if (a === 'left' && fits(cur.cells, cur.x - 1, cur.y)) cur.x--
      if (a === 'right' && fits(cur.cells, cur.x + 1, cur.y)) cur.x++
      if (a === 'rotate') {
        const r = rotate(cur.cells, cur.k)
        const kick = [0, -1, 1, -2, 2].find((d) => fits(r, cur.x + d, cur.y))
        if (kick !== undefined) { cur.cells = r; cur.x += kick }
      }
      if (a === 'down') { if (fits(cur.cells, cur.x, cur.y + 1)) { cur.y++; score += 1; fallMs = 0 } else lock() }
      if (a === 'drop') {
        while (fits(cur.cells, cur.x, cur.y + 1)) { cur.y++; score += 2 }
        lock()
      }
    }

    function step(now: number) {
      const dt = Math.min(100, now - last)
      last = now
      if (state === 'play') {
        fallMs += dt
        if (fallMs >= Math.max(90, 800 - level() * 70)) {
          fallMs = 0
          if (fits(cur.cells, cur.x, cur.y + 1)) cur.y++
          else lock()
        }
      }
      draw()
      raf = requestAnimationFrame(step)
    }

    function block(x: number, y: number, s: number, c: string, ghost = false) {
      ctx.fillStyle = c
      if (ghost) { ctx.globalAlpha = 0.25; ctx.fillRect(x + 1, y + 1, s - 2, s - 2); ctx.globalAlpha = 1; return }
      ctx.beginPath(); ctx.roundRect(x + 1, y + 1, s - 2, s - 2, 4); ctx.fill()
      ctx.globalAlpha = 0.25; ctx.fillStyle = col.paper; ctx.fillRect(x + 4, y + 4, s - 8, 3); ctx.globalAlpha = 1
    }
    function label(s: string, x: number, y: number, px: number, c: string, weight = 800) {
      ctx.font = `${weight} ${px}px ui-sans-serif, system-ui, sans-serif`; ctx.fillStyle = c; ctx.textAlign = 'left'; ctx.fillText(s, x, y)
    }

    function draw() {
      ctx.fillStyle = col.bg
      ctx.fillRect(0, 0, W, H)
      const s = Math.floor(Math.min((H - 190) / ROWS, (W - 40) / (COLS + 7)))
      const bw = s * COLS, bh = s * ROWS
      const bx = Math.floor((W - bw - s * 6) / 2), by = 40
      ctx.globalAlpha = 0.05; ctx.fillStyle = col.fg; ctx.fillRect(bx, by, bw, bh)
      ctx.globalAlpha = 0.08; ctx.strokeStyle = col.fg; ctx.lineWidth = 1
      for (let i = 1; i < COLS; i++) { ctx.beginPath(); ctx.moveTo(bx + i * s, by); ctx.lineTo(bx + i * s, by + bh); ctx.stroke() }
      for (let j = 1; j < ROWS; j++) { ctx.beginPath(); ctx.moveTo(bx, by + j * s); ctx.lineTo(bx + bw, by + j * s); ctx.stroke() }
      ctx.globalAlpha = 1
      board.forEach((row, y) => row.forEach((k, x) => k && block(bx + x * s, by + y * s, s, tint[k])))
      if (state === 'play') {
        let gy = cur.y
        while (fits(cur.cells, cur.x, gy + 1)) gy++
        for (const [cx, cy] of cur.cells) if (gy + cy >= 0) block(bx + (cur.x + cx) * s, by + (gy + cy) * s, s, tint[cur.k], true)
        for (const [cx, cy] of cur.cells) if (cur.y + cy >= 0) block(bx + (cur.x + cx) * s, by + (cur.y + cy) * s, s, tint[cur.k])
      }
      // The side panel: score, best, lines, level and the next piece.
      const px = bx + bw + s, ns = Math.floor(s * 0.8)
      label('Score', px, by + 18, 14, col.muted, 600); label(String(score), px, by + 46, 28, col.fg)
      label('Best', px, by + 82, 14, col.muted, 600); label(String(top), px, by + 106, 20, col.fg)
      label(`Lines ${lines} · Level ${level() + 1}`, px, by + 140, 14, col.muted, 600)
      if (state === 'play') {
        label('Next', px, by + 180, 14, col.muted, 600)
        for (const [cx, cy] of SHAPES[next]) block(px + cx * ns, by + 194 + cy * ns, ns, tint[next])
      }
      if (state !== 'play') {
        ctx.globalAlpha = 0.85; ctx.fillStyle = col.bg; ctx.fillRect(bx, by, bw, bh); ctx.globalAlpha = 1
        ctx.textAlign = 'center'
        ctx.font = '800 30px ui-sans-serif, system-ui, sans-serif'; ctx.fillStyle = state === 'over' ? col.brand : col.fg
        ctx.fillText(state === 'over' ? `${score}` : 'Blocks', bx + bw / 2, by + bh / 2 - 20)
        ctx.font = '600 14px ui-sans-serif, system-ui, sans-serif'; ctx.fillStyle = col.muted
        ctx.fillText(state === 'over' ? (score >= top && score > 0 ? 'New best!' : 'Game over') : '← → move · ↑ rotate · Space drop', bx + bw / 2, by + bh / 2 + 10)
        ctx.fillText('Space or ⟳ to play · Esc to close', bx + bw / 2, by + bh / 2 + 34)
      }
    }

    const keys: Record<string, Act> = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'rotate', x: 'rotate', ArrowDown: 'down', ' ': 'drop', Enter: 'drop' }
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.preventDefault(); onClose(); return }
      const a = keys[e.key]
      if (a) { e.preventDefault(); act.current(a) }
      e.stopPropagation()
    }
    window.addEventListener('keydown', key, true)
    window.addEventListener('resize', size)
    ;(document.activeElement as HTMLElement | null)?.blur()
    raf = requestAnimationFrame(step)
    return () => {
      cancelAnimationFrame(raf)
      window.removeEventListener('keydown', key, true)
      window.removeEventListener('resize', size)
    }
  }, [onClose])

  const btn = 'grid size-16 place-items-center rounded-full border border-border bg-card text-foreground shadow-[var(--shadow)] active:scale-95 active:bg-muted'
  const pad: [Act, typeof ChevronLeft, string][] = [
    ['left', ChevronLeft, 'Move left'], ['rotate', RotateCw, 'Rotate'], ['down', ArrowDown, 'Down one row'],
    ['drop', ChevronsDown, 'Drop'], ['right', ChevronRight, 'Move right'],
  ]
  return (
    <div role="dialog" aria-label="Blocks game" className="fixed inset-0 z-[100] touch-none select-none">
      <canvas ref={canvas} className="block h-full w-full" />
      <button type="button" aria-label="Close the game" onClick={onClose}
        className="absolute top-5 right-5 grid size-11 place-items-center rounded-full bg-muted text-foreground hover:bg-border">
        <X size={20} />
      </button>
      <div className="absolute inset-x-0 bottom-6 flex justify-center gap-3" onPointerDown={(e) => e.preventDefault()}>
        {pad.map(([a, Icon, name]) => (
          <button key={a} type="button" aria-label={name} onClick={() => act.current(a)} className={btn}><Icon size={26} /></button>
        ))}
      </div>
    </div>
  )
}
