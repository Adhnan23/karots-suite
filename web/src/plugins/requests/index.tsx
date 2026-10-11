import { NotebookPen } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { NotePanel } from './NotePanel'
import { RequestsPage } from './RequestsPage'

/** Customer requests' React half: the Apps tile and its page, and the till's
 * Note request (also offered when a search finds nothing). */
export default function requests(s: typeof Slots) {
  s.apps.push({ key: 'requests', label: 'Customer requests', description: 'What customers asked for that you don’t stock', icon: NotebookPen, color: 'var(--brand)', to: '/admin/x/requests' })
  s.adminPages.push({ key: 'requests', Page: RequestsPage })
  s.tillActions.push({ key: 'requests', label: 'Note request', noMatch: 'Not stocked? Note a request', Panel: NotePanel })
}
