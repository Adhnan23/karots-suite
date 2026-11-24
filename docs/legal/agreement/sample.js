// Fills the agreement with made-up data for karots-suite-agreement-sample.pdf: SAMPLE on
// every page (the drawing helpers are in sample-kit.js). Everything here is fictional
// (the Demo Room's Sunrise Mini Mart).
(() => {
  const { ink, field, row, tick, inline, signature, stamp } = window.K
  const k = (key, value) => document.querySelectorAll(`[data-k="${key}"]`).forEach((el) => ink(el, value))
  const opt = (key) => document.querySelectorAll(`[data-k="${key}"]`).forEach((el) => {
    el.textContent = el.textContent.replace('☐', '☑'); el.classList.add('ticked')
  })

  // The opening: place, date and the parties.
  k('agno', 'KS-2026-0007')
  k('place', 'Kandy'); k('day', '28th'); k('month', 'October'); k('year', 'Twenty Six')
  k('k-name', 'K. Karots'); k('k-nic', '000000000V (sample)'); k('k-addr', 'No. 1, Sample Lane, Colombo')
  k('k-pc', 'Western'); k('k-reg', 'W/00000 (sample)')
  opt('s-person')
  k('s-name', 'S. Sample'); k('s-nic', '111111111V (sample)'); k('s-coreg', '—'); k('s-addr', '12 Main Street, Kandy')
  k('s-trade', 'Sunrise Mini Mart (sample)'); k('s-reg', 'C/11111 (sample)')

  // The attestation: every row ticked and initialled, the guide's version written in.
  for (const tr of [...document.querySelectorAll('table.agree tr')].slice(1)) {
    tick('*')
    ink(tr.cells[2], 'S.S.')
    const v = tr.querySelector('.line'); if (v) ink(v, '1.4')
  }
  k('w-place', 'Kandy')
  const signs = [...document.querySelectorAll('.sign')]
  const who = (t) => signs.find((s) => s.querySelector('.who')?.textContent.includes(t))
  const people = {
    'within-named Karots': ['K. Karots', '000000000V (sample)', 'Sole proprietor', 'karots'],
    'within-named Shop': ['S. Sample', '111111111V (sample)', 'Sole proprietor', 'shop'],
    '1. Witness': ['W. One', '222222222V (sample)', null, null, '3 Sample Road, Kandy'],
    '2. Witness': ['W. Two', '333333333V (sample)', null, null, '4 Sample Road, Kandy'],
  }
  for (const [key, [name, nic, cap, seal, addr]] of Object.entries(people)) {
    const s = who(key)
    field('Name', name, s); field('NIC no.', nic, s)
    if (cap) field('Capacity', cap, s)
    if (addr) field('Address', addr, s)
    s.querySelector('.sigline').insertAdjacentHTML('beforebegin', signature(name))
    if (seal) s.querySelector('.seal').insertAdjacentHTML('beforeend', stamp(seal))
  }

  // The First Schedule: what the Shop gets, contacts, the price.
  field('Site (premises where it is used)', '12 Main Street, Kandy: shop floor and back office')
  field('Install ID', 'SAMPLE01')
  ;['Repairs', 'Reload', 'Combos', 'Built in', 'Phone', 'WhatsApp', 'Yearly', 'Cash', 'Bank transfer'].forEach(tick)
  for (const s of document.querySelectorAll('.boxes span')) if (s.textContent.trim().startsWith('Hours')) ink(s.querySelector('.line'), 'Mon–Sat 9–6')
  field('Installed on', '28 October 2026')
  field('Free months end on (3; 6 with Complete Shop)', '28 January 2027')
  k('s-contact', 'owner@example.com · 077 000 0000')
  const fees = document.querySelector('table.fees')
  row(fees, 'Licence', ['100,000', 'Karots Suite with Repairs, Reload, Combos; installation and training included'])
  row(fees, 'Putting in products (number × Rs. 10)', ['3,000', '300 products, on installation'])
  row(fees, 'Service plan (per month / year, from the end of the first 3 months)', ['60,000 / year', 'Yearly in advance, from 28 Jan 2027 (not in the total)'])
  row(fees, 'Less discount', ['—', ''])
  row(fees, 'Total', ['103,000', ''])
  field('Total in words', 'One hundred and three thousand rupees only')
  field('Advance paid (Rs.)', '50,000')
  field('Balance due (Rs.) and date', '53,000 by 26 Jan 2027')
  field('Liability amount for clause 10.3(b) of the terms (Rs.)', '100,000')
  field('Anything else agreed', 'Pharmacy can be added later at no charge (Guide section 6).')
})()
