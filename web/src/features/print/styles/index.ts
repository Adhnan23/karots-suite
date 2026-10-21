import { classic } from './classic/kit'
import type { StyleDef } from './kit'

/** Every receipt style, in the order the Appearance page shows them. */
export const STYLES: StyleDef[] = [{ key: 'classic', label: 'Classic', kit: classic }]

/** The style for a saved key; unknown or old keys draw Classic. */
export const styleOf = (key: string): StyleDef => STYLES.find((s) => s.key === key) ?? STYLES[0]
