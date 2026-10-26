import type { CSSProperties, ReactNode } from 'react'

/** A text that wraps to at most `lines` lines and is cut after that. Marked
 * `data-fit`, so the picture can tell when something didn't fit. */
export function Fit({ lines, style, children, ...rest }: { lines: number; style?: CSSProperties; children: ReactNode; [k: `data-${string}`]: string | boolean | undefined }) {
  return (
    <div data-fit="" {...rest} style={{ display: '-webkit-box', WebkitBoxOrient: 'vertical', WebkitLineClamp: Math.max(1, lines), overflow: 'hidden', overflowWrap: 'anywhere', ...style }}>
      {children}
    </div>
  )
}

/** True when any `data-fit` text in `root` was cut (more lines than it may show). */
export const anyCut = (root: HTMLElement) =>
  Array.from(root.querySelectorAll<HTMLElement>('[data-fit]')).some((el) => el.scrollHeight > el.clientHeight + 1)

/** The shop's size choice ('1'–'4' = XS, S, M, L) as a scale on a style's own size. */
export const sizeScale = (size: string) => ({ '1': 0.75, '2': 0.9, '3': 1, '4': 1.2 })[size] ?? 1
