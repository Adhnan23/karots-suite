/** The TypeScript twin of the plugin's Go coerce: checks a typed value the way the
 * server will, so the form and Fill in can mark a bad cell before saving.
 * current (the product's stored value) is accepted as is — a dropdown option
 * removed since stays. */
export function coerce(f: { label: string; type: string; options: string[] }, raw: string, current = ''): { value: string } | { error: string } {
  const v = raw.trim()
  if (f.type === 'bool') {
    if (['1', 'yes', 'y', 'true'].includes(v.toLowerCase())) return { value: '1' }
    if (['', '0', 'no', 'n', 'false'].includes(v.toLowerCase())) return { value: '' }
    return { error: `${f.label}: yes or no` }
  }
  if (v === '' || (current !== '' && v === current)) return { value: v }
  if (f.type === 'number' && !/^[-+]?(\d+\.?\d*|\.\d+)([eE][-+]?\d+)?$/.test(v)) return { error: `${f.label}: not a number` }
  if (f.type === 'date') {
    const d = new Date(`${v}T00:00:00Z`)
    if (!/^\d{4}-\d{2}-\d{2}$/.test(v) || isNaN(d.getTime()) || d.toISOString().slice(0, 10) !== v) return { error: `${f.label}: not a date (YYYY-MM-DD)` }
  }
  if (f.type === 'select') {
    const o = f.options.find((x) => x.toLowerCase() === v.toLowerCase())
    return o === undefined ? { error: `${f.label}: ${v} is not an option` } : { value: o }
  }
  return { value: v }
}
