/** Zoom that makes `needed` px fit `avail` px (never above 1). Ported from karots-pos kioskAutoFit. */
export function autofitZoom(needed: number, avail: number, pad = 16) {
  return needed + pad > avail ? Number((avail / (needed + pad)).toFixed(4)) : 1
}

/** Keeps the till frame fitted on sub-1080p counter screens. Returns a cleanup fn. */
export function startAutofit(el: HTMLElement = document.documentElement) {
  const fit = () => {
    el.style.setProperty('zoom', '1')
    el.style.setProperty('zoom', String(autofitZoom(Math.max(el.scrollWidth, document.body.scrollWidth), el.clientWidth)))
  }
  const ro = new ResizeObserver(fit)
  ro.observe(document.body)
  window.addEventListener('resize', fit)
  document.fonts?.ready.then(fit)
  fit()
  return () => { ro.disconnect(); window.removeEventListener('resize', fit); el.style.removeProperty('zoom') }
}
