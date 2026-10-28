// Fills the balance notice with made-up data for karots-suite-balance-notice-sample.pdf.
// It follows the sample agreement: Sunrise Mini Mart paid Rs 50,000 of Rs 86,500 at
// installation (28 Oct 2026) and the balance was due by 26 Jan 2027.
(() => {
  const { ink, field, tick, signature, stamp } = window.K
  field('Date of this notice', '27 January 2027')
  field('Our reference', 'KS-2026-0007 / N1')
  field('To (shop and owner)', 'Sunrise Mini Mart (sample) · S. Sample, Owner')
  field('Address', '12 Main Street, Kandy')
  const inl = [...document.querySelectorAll('p .line.inline')]
  ink(inl[0], 'KS-2026-0007')
  ink(inl[1], '28 October 2026')
  ink(inl[2], '10 February 2027')
  const cells = document.querySelector('table.fees').rows[1].cells
  ink(cells[0], 'Rs. 86,500'); ink(cells[1], 'Rs. 50,000'); ink(cells[2], 'Rs. 36,500')
  field('Pay to', 'Karots Korp (Karots Dev)')
  field('Bank and account no.', 'Sample Bank, Kandy · 0000 0000 0000 (sample)')
  const sign = document.querySelector('.signs .sign')
  field('Name', 'K. Karots, Sole proprietor', sign)
  sign.querySelector('.sigline').insertAdjacentHTML('beforebegin', signature('K. Karots'))
  document.querySelector('.signs .seal').insertAdjacentHTML('beforeend', stamp('karots', '27.01.2027'))
  const got = document.querySelector('.received')
  field('Name and signature', 'S. Sample', got)
  field('Date', '27 January 2027', got)
  tick('Handed over'); tick('WhatsApp')
  const hand = document.querySelector('[data-k="via-hand"]'); hand.textContent = hand.textContent.replace('☐', '☑'); hand.classList.add('ticked')
})()
