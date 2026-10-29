import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { MethodBadge } from './MethodBadge'

describe('MethodBadge', () => {
  it('marks a pending cheque in the cheque colour', () => {
    const html = renderToStaticMarkup(<MethodBadge method="cheque" status="pending" />)
    expect(html).toContain('Cheque')
    expect(html).toContain('· pending')
    expect(html).toContain('text-cheque')
    expect(html).toContain('var(--cheque)')
  })
  it('shows a bounced cheque as a warning, a cleared one plainly', () => {
    expect(renderToStaticMarkup(<MethodBadge method="cheque" status="bounced" />)).toContain('text-warning')
    const cleared = renderToStaticMarkup(<MethodBadge method="cheque" status="cleared" />)
    expect(cleared).toContain('text-cheque')
    expect(cleared).not.toContain('·')
  })
  it('leaves other methods in the muted look', () => {
    const html = renderToStaticMarkup(<MethodBadge method="cash" />)
    expect(html).toContain('Cash')
    expect(html).not.toContain('cheque')
  })
})
