// "Want to try it?" on the lock page: whom to ask for this hour's code (the owner on /__demo/install).

/** A Sri Lankan local number (0769626396) → WhatsApp and call links, and how to show it. */
export function askLinks(phone: string, installId: string) {
  const intl = '94' + phone.replace(/\D/g, '').replace(/^0/, '')
  const local = phone.replace(/\D/g, '')
  const text = `Hello Karots, I would like a code to try the Demo Room (install ${installId}).`
  return {
    whatsapp: `https://wa.me/${intl}?text=${encodeURIComponent(text)}`,
    tel: `tel:+${intl}`,
    shown: `${local.slice(0, 3)} ${local.slice(3, 6)} ${local.slice(6)}`,
  }
}
