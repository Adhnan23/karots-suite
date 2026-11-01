import { afterEach, describe, expect, it, vi } from 'vitest'
import { phoneKind, steps, watchTrust } from './setupSteps'

const IPHONE = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1'
const ANDROID = 'Mozilla/5.0 (Linux; Android 15; CPH2653) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36'

describe('the setup page', () => {
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('tells an iPhone from an Android', () => {
    expect(phoneKind(IPHONE)).toBe('iphone')
    expect(phoneKind(ANDROID)).toBe('android')
    expect(phoneKind('Mozilla/5.0 (X11; Linux x86_64) Firefox/131.0')).toBe('other')
  })

  it('gives each phone its exact steps', () => {
    const ios = steps('iphone', 'Lanka Mart').map((s) => s.text).join(' ')
    expect(ios).toContain('Certificate Trust Settings')
    expect(ios).toContain('Karots Shop CA — Lanka Mart')
    expect(steps('iphone', '')[0].file).toBe('ca.mobileconfig')
    const android = steps('android', 'x').map((s) => s.text).join(' ')
    expect(android).toContain('CA certificate')
    expect(steps('android', 'x')[0].file).toBe('ca.crt')
    expect(steps('other', 'x')[0].file).toBe('ca.pem')
  })

  it('shows the app link only once the phone trusts the certificate', async () => {
    vi.useFakeTimers()
    let trusted = false
    vi.stubGlobal('fetch', vi.fn(() => (trusted ? Promise.resolve(new Response(null, { status: 204 })) : Promise.reject(new TypeError('cert')))))
    const ok = vi.fn()
    const stop = watchTrust('https://192.168.1.5:8443/api/v1/x/pocketcashier/ping', ok)
    await vi.advanceTimersByTimeAsync(3_100)
    expect(ok).not.toHaveBeenCalled()
    trusted = true
    await vi.advanceTimersByTimeAsync(3_100)
    expect(ok).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(9_000)
    expect(ok).toHaveBeenCalledTimes(1) // stops asking once trusted
    stop()
  })
})
