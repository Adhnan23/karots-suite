import { bold } from './bold/kit'
import { classic } from './classic/kit'
import type { StyleDef } from './kit'
import { minimal } from './minimal/kit'
import { ticket } from './ticket/kit'

/** Every receipt style, in the order the Appearance page shows them. */
export const STYLES: StyleDef[] = [
  { key: 'classic', label: 'Classic', kit: classic },
  { key: 'bold', label: 'Bold', kit: bold },
  { key: 'minimal', label: 'Minimal', kit: minimal },
  { key: 'ticket', label: 'Ticket', kit: ticket },
]

/** The style for a saved key; unknown or old keys draw Classic. */
export const styleOf = (key: string): StyleDef => STYLES.find((s) => s.key === key) ?? STYLES[0]
