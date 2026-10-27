import { api, ApiError } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { configQuery } from '@/features/till/api'
import { paperOf } from './kit'
import { labelPictureQuery } from './picture'

/** Prints a label run. In Image mode it's the picture of `data` in the shop's
 * style for `kind`, posted to Classic's own endpoint (`classic.path`) with
 * ?count&size; if the picture can't be made (or the printer refuses it) the run
 * goes as the Classic request instead — a label never fails for its looks.
 * Classic mode posts `classic.body` as always. Printer errors are the caller's. */
export async function printLabel(o: { kind: string; data: unknown; size: string; count: number; classic: { path: string; body: unknown } }): Promise<void> {
  const cfg = queryClient.getQueryData(configQuery.queryKey) ?? (await queryClient.fetchQuery(configQuery))
  if (cfg.labelPrintAs === 'image') {
    const paper = paperOf(o.size, cfg.labelSize)
    const pic = await queryClient.fetchQuery(labelPictureQuery(o.kind, o.data, paper)).catch((e) => console.warn('label picture failed, printing Classic', e))
    if (pic) {
      const q = new URLSearchParams({ count: String(o.count), size: `${paper.wMm}x${paper.hMm}` })
      try {
        return await api(`${o.classic.path}?${q}`, { blob: pic.blob })
      } catch (e) {
        // Only a refused picture is safe to resend as Classic: any other error may
        // come after the printer already got it (a lost connection), so no second run.
        if (!(e instanceof ApiError && e.code === 'BAD_IMAGE')) throw e
        console.warn('label picture refused, printing Classic', e)
      }
    }
  }
  await api(o.classic.path, { json: o.classic.body })
}
