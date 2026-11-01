export type PhoneKind = 'android' | 'iphone' | 'other'

/** A setup step; `file` is the certificate download it starts with (under the plugin's API). */
export type Step = { text: string; file?: 'ca.crt' | 'ca.mobileconfig' | 'ca.pem' }

export function phoneKind(ua: string): PhoneKind {
  if (/iPhone|iPad|iPod/.test(ua)) return 'iphone'
  if (/Android/.test(ua)) return 'android'
  return 'other'
}

const caName = (shop: string) => (shop ? `Karots Shop CA — ${shop}` : 'Karots Shop CA')

/** The exact steps for this phone to trust the shop's certificate. Android
 * installs it as a CA certificate (never through Wi-Fi settings); an iPhone
 * needs Safari, the profile, and Certificate Trust Settings. */
export function steps(kind: PhoneKind, shop: string): Step[] {
  if (kind === 'iphone') {
    return [
      { text: 'In Safari (not Chrome), tap Download below and Allow.', file: 'ca.mobileconfig' },
      { text: 'Open Settings — tap “Profile Downloaded” at the top — Install, enter the phone’s passcode, Install again.' },
      { text: `Settings → General → About → Certificate Trust Settings → turn on “${caName(shop)}”, then Continue.` },
      { text: 'Come back to this page: the app link shows here once the phone trusts the shop.' },
    ]
  }
  if (kind === 'android') {
    return [
      { text: 'Tap Download below. If Chrome asks, keep the file.', file: 'ca.crt' },
      { text: 'Settings → Security (or Security & privacy) → Encryption & credentials (under More security settings on some phones) → Install a certificate → CA certificate → Install anyway, then pick the downloaded “karots-shop-ca” file.' },
      { text: 'Don’t use Wi-Fi certificate settings — it must be a CA certificate.' },
      { text: 'Come back to this page: the app link shows here once the phone trusts the shop.' },
    ]
  }
  return [
    { text: 'Download the shop certificate below.', file: 'ca.pem' },
    { text: `Add it to this device’s trusted root certificates (“${caName(shop)}”).` },
    { text: 'Come back to this page: the app link shows here once the device trusts the shop.' },
  ]
}

/** Asks the HTTPS port every 3 s; calls ok once a ping gets through (the phone
 * trusts the certificate, so the app opens with no warning), then stops. */
export function watchTrust(pingUrl: string, ok: () => void, every = 3_000): () => void {
  let stopped = false
  const t = setInterval(() => {
    fetch(pingUrl, { cache: 'no-store' }).then((r) => {
      if (!r.ok || stopped) return
      stopped = true
      clearInterval(t)
      ok()
    }, () => {})
  }, every)
  return () => { stopped = true; clearInterval(t) }
}
