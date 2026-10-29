/** The sales filters' options, shared by the Sales list and the Sales report. */
export const STATUS: [string, string][] = [['', 'Any status'], ['completed', 'Completed'], ['credit', 'On credit'], ['partially_returned', 'Part returned'], ['returned', 'Returned']]
// ponytail: plugin tenders are left out — they're stored under a core payment
// method, so filtering by their own key would match nothing; add when one isn't.
export const METHODS: [string, string][] = [['', 'Any payment'], ['cash', 'Cash'], ['card', 'Card'], ['online', 'Online'], ['cheque', 'Cheque'], ['credit', 'Credit']]
export const STATUS_TAG: Record<string, [string, 'warn' | 'muted' | 'danger']> = { credit: ['credit', 'warn'], partially_returned: ['part returned', 'muted'], returned: ['returned', 'muted'], void: ['void', 'danger'] }
