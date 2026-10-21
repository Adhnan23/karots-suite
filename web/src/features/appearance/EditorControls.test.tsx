import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { slots } from '@/plugins/slots'
import { EditorControls } from './EditorControls'
import { builtinDoc } from './paletteCss'

describe('EditorControls plugin tokens', () => {
  afterEach(() => { slots.paletteTokens.length = 0; cleanup() })

  it('lists a plugin token under Plugins and sets or clears it', () => {
    slots.paletteTokens.push({ plugin: 'demo', label: 'Demo', tokens: [{ name: '--demo-due', group: 'colour', kind: 'color' }] })
    const doc = builtinDoc('studio')
    const onChange = vi.fn()
    render(<EditorControls doc={doc} mode="light" onChange={onChange} />)
    expect(screen.getByText('Plugins')).toBeTruthy()
    fireEvent.change(screen.getByLabelText('Demo due'), { target: { value: '#ff0000' } })
    expect(onChange.mock.calls[0][0].light['--demo-due']).toBe('#ff0000')
    render(<EditorControls doc={{ ...doc, light: { ...doc.light, '--demo-due': 'red' } }} mode="light" onChange={onChange} />)
    fireEvent.change(screen.getAllByLabelText('Demo due')[1], { target: { value: '' } })
    expect('--demo-due' in onChange.mock.calls[1][0].light).toBe(false)
  })
  it('has no Plugins group without plugin tokens', () => {
    render(<EditorControls doc={builtinDoc('studio')} mode="light" onChange={() => {}} />)
    expect(screen.queryByText('Plugins')).toBeNull()
  })

  it('hides derived colours until Show all colours is ticked', () => {
    const doc = builtinDoc('studio')
    render(<EditorControls doc={{ ...doc, light: { ...doc.light, '--ring': '#ff0000' } }} mode="light" onChange={() => {}} />)
    expect(screen.getAllByLabelText('Foreground').length).toBeGreaterThan(0)
    expect(screen.queryAllByLabelText('Card foreground')).toHaveLength(0)
    expect(screen.getAllByLabelText('Ring').length).toBeGreaterThan(0) // changed by hand, so still shown
    fireEvent.click(screen.getByLabelText(/Show all colours/))
    expect(screen.getAllByLabelText('Card foreground').length).toBeGreaterThan(0)
  })
})
