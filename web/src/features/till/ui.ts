/** The three big till surfaces — the only place --glass is allowed (theme.md §2). */
export const PANEL = 'rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)] [backdrop-filter:var(--glass)]'
/** Pill buttons: group chips, quick cash, ⟲ Menu. aria-pressed shows the selected look. */
export const CHIP =
  'inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border border-border bg-card px-3 py-1.5 text-[13px] font-semibold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
/** Text inputs on the till. */
export const FIELD =
  'h-[var(--control-h)] rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[15px] text-foreground outline-none placeholder:text-faint focus-visible:ring-2 focus-visible:ring-ring'
/** Product and group cards: no glass (GPU), lift on hover via motion. */
export const TILE =
  'flex h-full min-h-[98px] w-full flex-col justify-between gap-2 rounded-[var(--radius-sm)] border border-border bg-card p-3 text-left shadow-[var(--shadow)]'
