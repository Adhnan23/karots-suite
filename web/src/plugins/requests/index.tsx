import type { slots as Slots } from '../slots'
import { NotePanel } from './NotePanel'

/** Customer requests' React half: the till's Note request (also offered when a
 * search finds nothing). */
export default function requests(s: typeof Slots) {
  s.tillActions.push({ key: 'requests', label: 'Note request', noMatch: 'Not stocked? Note a request', Panel: NotePanel })
}
