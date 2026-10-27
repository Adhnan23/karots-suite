/** Whether a compiled-in plugin registers on this page. A shop build's page has no
 *  list, so every plugin is on; the Demo Room injects `<meta name="ks-plugins">`
 *  with the current shop's plugins (the demo build's enabled-plugins.ts asks). */
export function pluginOn(key: string): boolean {
  const list = document.querySelector<HTMLMetaElement>('meta[name="ks-plugins"]')
  if (!list) return true
  return list.content.split(',').map((k) => k.trim()).includes(key)
}
