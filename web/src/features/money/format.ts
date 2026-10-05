/** "5 Oct, 01:48 PM" — when a money event happened. */
export const stamp = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
