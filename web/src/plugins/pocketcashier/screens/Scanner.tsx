import { BrowserMultiFormatReader } from '@zxing/library'
import { X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

/** The back camera reads one barcode, then closes (works offline; the reader is bundled). */
export function Scanner({ onCode, onClose }: { onCode: (code: string) => void; onClose: () => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    const reader = new BrowserMultiFormatReader()
    let done = false
    reader.decodeFromConstraints({ video: { facingMode: 'environment' } }, video.current!, (res) => {
      if (!res || done) return
      done = true
      reader.reset()
      onCode(res.getText())
    }).catch(() => setError('The camera can’t be opened — allow it in the phone’s settings.'))
    return () => reader.reset()
  }, [onCode])
  return (
    <div role="dialog" aria-label="Scan a barcode" className="fixed inset-0 z-50 flex flex-col bg-scrim">
      <button type="button" aria-label="Close" onClick={onClose} className="absolute top-[calc(env(safe-area-inset-top)+12px)] right-3 z-10 grid size-10 place-items-center rounded-full bg-card text-foreground"><X size={18} /></button>
      <video ref={video} muted playsInline className="h-full w-full object-cover" />
      <div className="pointer-events-none absolute inset-x-10 top-1/2 h-32 -translate-y-1/2 rounded-[var(--radius)] border-2 border-brand" />
      {error && <p role="alert" className="absolute inset-x-4 bottom-10 rounded-[var(--radius-sm)] bg-card p-3 text-center text-sm font-semibold">{error}</p>}
    </div>
  )
}
