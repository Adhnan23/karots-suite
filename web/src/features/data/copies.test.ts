import { describe, expect, it } from 'vitest'
import type { BackupCopyRow } from '@/api/types.gen'
import { ago, copyStatus } from './copies'

const now = new Date(2026, 9, 20, 12, 0).getTime()
const at = (min: number) => new Date(now - min * 60_000).toISOString()
const row = (r: Partial<BackupCopyRow>): BackupCopyRow => ({
  kind: 'folder', enabled: true, path: '/media/usb', lastCopiedAt: null, lastTriedAt: null, lastFile: '', lastError: '', waiting: 0, ...r,
})

describe('backup copy status line', () => {
  it('says how long ago', () => {
    expect(ago(at(0), now)).toBe('just now')
    expect(ago(at(10), now)).toBe('10 min ago')
    expect(ago(at(185), now)).toBe('3 h ago')
    expect(ago(at(60 * 24), now)).toBe('1 day ago')
    expect(ago(at(60 * 50), now)).toBe('2 days ago')
  })
  it('off, missing USB, failed, copied, waiting', () => {
    expect(copyStatus(row({ enabled: false, lastError: 'x' }), now).text).toBe('Switched off')
    expect(copyStatus(row({ lastError: 'not there right now: /media/usb' }), now)).toEqual({ text: 'Not there right now — plug the USB in', tone: 'warn' })
    expect(copyStatus(row({ lastError: 'disk full' }), now).text).toBe('Failed: disk full')
    expect(copyStatus(row({ lastCopiedAt: at(10), waiting: 2 }), now)).toEqual({ text: 'Last copied 10 min ago · 2 waiting', tone: 'ok' })
    expect(copyStatus(row({ lastCopiedAt: at(60 * 25) }), now).tone).toBe('warn') // the dashboard warns too
    expect(copyStatus(row({ kind: 'cloud', waiting: 1 }), now).text).toBe('No copy yet · next copy due')
  })
})
