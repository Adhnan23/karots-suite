import type { ReactNode } from 'react'
import type { JobInfo } from './jobs'

/** A job's own controls that its prompt needs (a tick list of fields…), by job key. */
export const jobControls: Record<string, (p: { job: JobInfo; value: unknown; onChange: (v: unknown) => void }) => ReactNode> = {}
