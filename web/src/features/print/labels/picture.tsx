import { queryOptions } from '@tanstack/react-query'
import { queryClient } from '@/api/query-client'
import { configQuery } from '@/features/till/api'
import { PAPER_FONTS, SANS_FONTS } from '../paper'
import { shoot } from '../picture'
import { anyCut } from './fit'
import type { LabelPaper } from './kit'
import { kindOf, styleFor } from './registry'

/** Every font a label style may use (monospace for Classic, Inter for the rest). */
const LABEL_FONTS = [...PAPER_FONTS, ...SANS_FONTS.slice(0, 1)] as const

export type LabelPicture = { blob: Blob; url: string; cut: boolean; style: string; fellBack: boolean }

/** The shop's saved style for a label kind (the one that prints). */
export const savedLabelStyle = (kind: string) => queryClient.getQueryData(configQuery.queryKey)?.labelStyles?.[kind] ?? 'classic'

/** The label drawn at paper size, in style `key`, as its template sees it. */
export function LabelPage({ kind, data, paper, style }: { kind: string; data: unknown; paper: LabelPaper; style: string }) {
  const k = kindOf(kind)
  if (!k) return null
  const { style: s } = styleFor(k, style, paper, data)
  return (
    <div data-label-style={s.key} style={{ width: paper.w, height: paper.h, overflow: 'hidden', position: 'relative', background: '#fff', color: '#000', boxSizing: 'border-box' }}>
      <s.Template d={data} paper={paper} />
    </div>
  )
}

async function make(kind: string, data: unknown, paper: LabelPaper, style: string): Promise<LabelPicture> {
  const k = kindOf(kind)
  if (!k) throw new Error(`no label kind ${kind}`)
  const { style: s, fellBack } = styleFor(k, style, paper, data)
  let cut = false
  const blob = await shoot(<LabelPage kind={kind} data={data} paper={paper} style={style} />, paper.w, LABEL_FONTS, paper.h, (node) => { cut = anyCut(node) })
  // ponytail: object URLs live as long as the page, like slip pictures.
  return { blob, url: URL.createObjectURL(blob), cut, style: s.key, fellBack }
}

/** A label as the exact picture the printer gets, in the shop's style for its
 * kind (`style` only for previews of an unsaved choice). Cached briefly, so the
 * preview and the print that follows send the same pixels. */
export const labelPictureQuery = (kind: string, data: unknown, paper: LabelPaper, style = savedLabelStyle(kind)) =>
  queryOptions({
    queryKey: ['label-picture', kind, JSON.stringify(data), paper.wMm, paper.hMm, style],
    queryFn: () => make(kind, data, paper, style),
    staleTime: 60_000,
    retry: false,
  })
