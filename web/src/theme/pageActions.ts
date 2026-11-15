import type { PaletteEntry } from './nav'

/** What the open page adds to Ctrl K (the Sell page: all its buttons), above the
 * page jumps. The page sets it while it's shown and empties it when it goes. */
export const pageActions: { current: PaletteEntry[] } = { current: [] }
