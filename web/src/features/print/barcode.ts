// Code 128 (set B) as module widths — bar, space, bar… — for slip numbers
// (S-, CR-, DP-, R- + digits). Start B, the data, the checksum, the stop.

// Symbol 0–106, six widths each (the stop has seven).
const PATTERNS = (
  '212222 222122 222221 121223 121322 131222 122213 122312 132212 221213 221312 231212 112232 122132 122231 113222 ' +
  '123122 123221 223211 221132 221231 213212 223112 312131 311222 321122 321221 312212 322112 322211 212123 212321 ' +
  '232121 111323 131123 131321 112313 132113 132311 211313 231113 231311 112133 112331 132131 113123 113321 133121 ' +
  '313121 211331 231131 213113 213311 213131 311123 311321 331121 312113 312311 332111 314111 221411 431111 111224 ' +
  '111422 121124 121421 141122 141221 112214 112412 122114 122411 142112 142211 241211 221114 413111 241112 134111 ' +
  '111242 121142 121241 114212 124112 124211 411212 421112 421211 212141 214121 412121 111143 111341 131141 114113 ' +
  '114311 411113 411311 113141 114131 311141 411131 211412 211214 211232 2331112'
).split(' ')

const START_B = 104
const STOP = 106

/** The module widths of `value` as Code 128 set B (printable ASCII only). */
export function code128(value: string): number[] {
  const codes = [START_B]
  for (const ch of value) {
    const c = ch.charCodeAt(0) - 32
    if (c < 0 || c > 95) throw new Error(`code128: ${JSON.stringify(ch)} is not in set B`)
    codes.push(c)
  }
  codes.push(codes.reduce((sum, c, i) => sum + c * Math.max(i, 1), 0) % 103, STOP)
  return codes.flatMap((c) => [...PATTERNS[c]].map(Number))
}
