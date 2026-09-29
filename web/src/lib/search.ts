/** A text search param. The router parses params as JSON, so a typed barcode
 * like 4790000000011 arrives as a number — keep it as the text it was. */
export const searchStr = (v: unknown) => (typeof v === 'string' || typeof v === 'number' ? String(v) : '')
