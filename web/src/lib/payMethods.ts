/** The ways money moves (core). A plugin's own tenders come from the till config. */
export type PayMethod = 'cash' | 'card' | 'online' | 'cheque'

/** Every method a money form offers, in order (forms drop the ones with no account to use). */
export const PAY_METHODS: [PayMethod, string][] = [['cash', 'Cash'], ['card', 'Card'], ['online', 'Online'], ['cheque', 'Cheque']]

export const METHOD_LABEL: Record<string, string> = { cash: 'Cash', card: 'Card', online: 'Online', cheque: 'Cheque', credit: 'On account' }

export type ChequeStatus = 'pending' | 'cleared' | 'bounced' | 'cancelled'
