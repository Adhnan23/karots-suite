import { ChevronDown } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { slots } from '@/plugins/slots'
import { TOKENS } from '@/theme/palettes/tokens'
import { drawnColour, formatColour, parseCssColour } from './colourValue'
import type { PaletteDoc } from './paletteCss'
import { applyMaterial, applyMotion, blurOf, materialOf, motionOf, withBlur, type Feel, type Material } from './presets'

type Mode = 'light' | 'dark'
type Tokens = Record<string, string>

const label = (name: string) => name.replace(/^--/, '').replace(/-/g, ' ').replace(/^./, (c) => c.toUpperCase())

// Generated art follows the palette's own brand colours, so it suits any palette.
const ART = 'radial-gradient(900px 500px at 12% -10%, color-mix(in srgb, var(--brand) 24%, transparent), transparent 60%), radial-gradient(700px 500px at 110% 110%, color-mix(in srgb, var(--brand-2) 20%, transparent), transparent 60%)'
const DEPTH = {
  Flat: { light: ['0 1px 2px rgb(0 0 0 / .05)', '0 6px 16px rgb(0 0 0 / .10)'], dark: ['0 1px 2px rgb(0 0 0 / .3)', '0 8px 20px rgb(0 0 0 / .4)'] },
  Soft: { light: ['0 8px 24px rgb(0 0 0 / .08)', '0 18px 50px rgb(0 0 0 / .16)'], dark: ['0 10px 30px rgb(0 0 0 / .35)', '0 18px 50px rgb(0 0 0 / .5)'] },
  Deep: { light: ['0 14px 40px rgb(0 0 0 / .16)', '0 26px 70px rgb(0 0 0 / .26)'], dark: ['0 16px 44px rgb(0 0 0 / .55)', '0 28px 70px rgb(0 0 0 / .7)'] },
} as const
const GLOW = '0 0 14px color-mix(in srgb, var(--brand) 45%, transparent)'

/** Keeps a shadow's inset layers (the glass rim) and swaps its drop layers. */
export function withDrop(shadow: string, drop: string) {
  const layers: string[] = []
  let depth = 0
  let start = 0
  for (let i = 0; i <= shadow.length; i++) {
    const c = shadow[i]
    if (c === '(') depth++
    else if (c === ')') depth--
    else if ((c === ',' && depth === 0) || i === shadow.length) {
      layers.push(shadow.slice(start, i).trim())
      start = i + 1
    }
  }
  return [...layers.filter((l) => l.startsWith('inset')), drop].join(', ')
}

/** The editor's groups (spec §4). Colours and background colour edit the mode on screen;
 *  material, depth, shape and motion edit both modes. */
export function EditorControls({ doc, mode, onChange }: { doc: PaletteDoc; mode: Mode; onChange: (d: PaletteDoc) => void }) {
  const cur = doc[mode]
  const setMode = (t: Tokens) => onChange({ ...doc, [mode]: t })
  const setBoth = (f: (t: Tokens, m: Mode) => Tokens) => onChange({ ...doc, light: f(doc.light, 'light'), dark: f(doc.dark, 'dark') })
  const material = materialOf(cur)
  const motion = motionOf(cur)
  const radius = parseFloat(cur['--radius'] ?? '16') || 0
  const [allColours, setAllColours] = useState(false)
  // Derived colours (card foreground, ring…) follow another token; shown on request or once changed.
  const shown = (name: string) => allColours || !(cur[name] ?? '').startsWith('var(')
  const subs = [...new Set(TOKENS.filter((t) => t.kind === 'color' && t.group === 'colour').map((t) => t.sub!))]

  return (
    <div className="flex flex-col gap-3">
      <Group title="Colours" hint={`Editing ${mode} mode`} open>
        <label className="flex items-center gap-2 self-start text-sm font-semibold">
          <input type="checkbox" checked={allColours} onChange={(e) => setAllColours(e.target.checked)} className="size-4 accent-(--brand)" />
          Show all colours
          <span className="font-normal text-faint">(the ones that follow another colour)</span>
        </label>
        {subs.filter((sub) => TOKENS.some((t) => t.sub === sub && shown(t.name))).map((sub) => (
          <div key={sub} className="flex flex-col gap-1.5">
            <p className="text-[11px] font-bold tracking-wide text-faint uppercase">{sub}</p>
            {TOKENS.filter((t) => t.sub === sub && shown(t.name)).map((t) => (
              <ColourRow key={t.name} name={t.name} value={cur[t.name] ?? ''} onChange={(v) => setMode({ ...cur, [t.name]: v })} />
            ))}
          </div>
        ))}
      </Group>

      <Group title="Material" hint={material === 'liquid' ? 'Liquid glass' : material === 'frosted' ? 'Frosted' : 'Solid'}>
        <Seg label="Material" value={material} options={[['solid', 'Solid'], ['frosted', 'Frosted'], ['liquid', 'Liquid glass']]}
          onChange={(m: Material) => setBoth((t, md) => applyMaterial(t, m, md))} />
        <Slider label="Blur" unit="px" min={0} max={32} value={blurOf(cur)} onChange={(px) => setBoth((t) => withBlur(t, px))} />
        <ColourRow name="--bar" value={cur['--bar'] ?? ''} onChange={(v) => setMode({ ...cur, '--bar': v })} />
      </Group>

      <Group title="Background">
        <Seg label="Background art" value={(cur['--bg-art'] ?? 'none') === 'none' ? 'plain' : 'art'} options={[['plain', 'Plain'], ['art', 'Glow art']]}
          onChange={(v) => setBoth((t) => ({ ...t, '--bg-art': v === 'art' ? ART : 'none' }))} />
        <ColourRow name="--background" value={cur['--background'] ?? ''} onChange={(v) => setMode({ ...cur, '--background': v })} />
      </Group>

      <Group title="Depth">
        <Seg label="Shadows" value="" options={[['Flat', 'Flat'], ['Soft', 'Soft'], ['Deep', 'Deep']]}
          onChange={(d: keyof typeof DEPTH) => setBoth((t, md) => ({ ...t, '--shadow': withDrop(t['--shadow'] ?? '', DEPTH[d][md][0]), '--shadow-float': withDrop(t['--shadow-float'] ?? '', DEPTH[d][md][1]) }))} />
        <Seg label="Glow" value={(cur['--glow'] ?? 'none') === 'none' ? 'off' : 'on'} options={[['off', 'No glow'], ['on', 'Glow']]}
          onChange={(v) => setBoth((t) => ({ ...t, '--glow': v === 'on' ? GLOW : 'none' }))} />
      </Group>

      <Group title="Shape">
        <Slider label="Corners" unit="px" min={0} max={28} value={radius}
          onChange={(r) => setBoth((t) => ({ ...t, '--radius': `${r}px`, '--radius-sm': `${Math.round(r * 0.7)}px` }))} />
        <Seg label="Pills" value={cur['--pill'] === '9999px' ? 'round' : 'soft'} options={[['round', 'Round'], ['soft', 'Soft']]}
          onChange={(v) => setBoth((t) => ({ ...t, '--pill': v === 'round' ? '9999px' : 'var(--radius-sm)' }))} />
      </Group>

      <Group title="Motion" hint={motion.feel === 'lively' ? 'Lively' : 'Calm'}>
        <Seg label="Feel" value={motion.feel} options={[['calm', 'Calm'], ['lively', 'Lively']]}
          onChange={(f: Feel) => setBoth((t) => applyMotion(t, f, motion.speed))} />
        <Slider label="Length" unit="×" min={0.5} max={1.5} step={0.05} value={motion.speed}
          onChange={(s) => setBoth((t) => applyMotion(t, motion.feel, s))} />
      </Group>

      {slots.paletteTokens.length > 0 && (
        <Group title="Plugins" hint="Blank = the plugin's default">
          {slots.paletteTokens.map((p) => (
            <div key={p.plugin} className="flex flex-col gap-1.5">
              <p className="text-[11px] font-bold tracking-wide text-faint uppercase">{p.label}</p>
              {p.tokens.map((t) => (
                <ColourRow key={t.name} name={t.name} value={cur[t.name] ?? ''} onChange={(v) => {
                  const next = { ...cur, [t.name]: v }
                  if (!v.trim()) delete next[t.name]
                  setMode(next)
                }} />
              ))}
            </div>
          ))}
        </Group>
      )}

      <Group title="Custom CSS" hint={doc.extra.trim() ? 'In use' : 'Empty'}>
        <textarea aria-label="Custom CSS" spellCheck={false} value={doc.extra} onChange={(e) => onChange({ ...doc, extra: e.target.value })}
          placeholder={'.till-total { letter-spacing: .02em; }'}
          className="min-h-40 w-full rounded-[var(--radius-sm)] border border-input bg-background/60 p-3 font-mono text-xs outline-none focus:border-brand" />
      </Group>
    </div>
  )
}

function Group({ title, hint, open, children }: { title: string; hint?: string; open?: boolean; children: ReactNode }) {
  return (
    <details open={open} className="group rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)] [backdrop-filter:var(--glass)]">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 [&::-webkit-details-marker]:hidden">
        <span className="font-display font-bold">{title}</span>
        {hint && <span className="text-xs font-semibold text-faint">{hint}</span>}
        <ChevronDown className="ml-auto size-4 text-muted-foreground transition-transform duration-(--dur-fast) group-open:rotate-180" />
      </summary>
      <div className="flex flex-col gap-3 border-t border-border px-4 py-3">{children}</div>
    </details>
  )
}

/** A colour token: picker (any colour; a see-through one keeps its opacity, and a
 *  reference like var(…) starts from what's drawn), the CSS text, and an opacity
 *  slider for see-through values. */
function ColourRow({ name, value, onChange }: { name: string; value: string; onChange: (v: string) => void }) {
  const own = parseCssColour(value)
  const picked = own ?? (value.trim() ? drawnColour(value) : null)
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2 text-sm">
        <span className="w-28 shrink-0 truncate text-muted-foreground sm:w-36">{label(name)}</span>
        <span className="relative size-8 shrink-0 overflow-hidden rounded-[var(--radius-sm)] border border-border" style={{ background: value }}>
          {picked && <input type="color" aria-label={`${label(name)} picker`} value={picked.hex.toLowerCase()}
            onChange={(e) => onChange(formatColour(e.target.value, own ? own.alpha : picked.alpha))}
            className="absolute inset-0 size-full cursor-pointer opacity-0" />}
        </span>
        <input aria-label={label(name)} value={value} spellCheck={false} onChange={(e) => onChange(e.target.value)}
          className="h-8 min-w-0 flex-1 rounded-[var(--radius-sm)] border border-input bg-background/60 px-2 font-mono text-xs outline-none focus:border-brand" />
      </div>
      {own && own.alpha < 1 && (
        <label className="flex items-center gap-2 pl-[calc(7rem+0.5rem)] text-xs text-muted-foreground sm:pl-[calc(9rem+0.5rem)]">
          Opacity
          <input type="range" min={0} max={100} value={Math.round(own.alpha * 100)} aria-label={`${label(name)} opacity`}
            onChange={(e) => onChange(formatColour(own.hex, Math.min(0.99, Number(e.target.value) / 100)))} className="min-w-0 flex-1 accent-(--brand)" />
          <span className="w-9 text-right font-mono tabular-nums">{Math.round(own.alpha * 100)}%</span>
        </label>
      )}
    </div>
  )
}

function Seg<T extends string>({ label, value, options, onChange }: { label: string; value: string; options: [T, string][]; onChange: (v: T) => void }) {
  return (
    <div className="flex items-center gap-2 text-sm">
      <span className="w-20 shrink-0 text-muted-foreground">{label}</span>
      <div role="group" aria-label={label} className="flex flex-wrap gap-0.5 rounded-pill bg-muted p-1">
        {options.map(([v, text]) => (
          <button key={v} type="button" aria-pressed={value === v} onClick={() => onChange(v)}
            className="rounded-pill px-3 py-1 text-xs font-semibold text-muted-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground">{text}</button>
        ))}
      </div>
    </div>
  )
}

function Slider({ label, unit, min, max, step = 1, value, onChange }: { label: string; unit: string; min: number; max: number; step?: number; value: number; onChange: (v: number) => void }) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <span className="w-20 shrink-0 text-muted-foreground">{label}</span>
      <input type="range" min={min} max={max} step={step} value={value} onChange={(e) => onChange(Number(e.target.value))} className="min-w-0 flex-1 accent-(--brand)" />
      <span className="w-12 text-right font-mono text-xs tabular-nums">{value}{unit}</span>
    </label>
  )
}
