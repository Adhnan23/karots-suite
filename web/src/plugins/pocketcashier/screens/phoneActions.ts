import { slots, type TillAction } from '@/plugins/slots'

export type PhoneAction = TillAction

/** The till actions on the phone: Customer requests stays on the PC (user: it
 * doesn't sell anything, and a cut is no time to take notes). */
export const phoneActions = (): PhoneAction[] => slots.tillActions.filter((a) => a.key !== 'requests')
