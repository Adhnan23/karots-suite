// Fills the Order Form with made-up data for karots-suite-agreement-sample.pdf:
// SAMPLE on every page (the drawing helpers are in sample-kit.js).
// Everything here is fictional (the Demo Room's Sunrise Mini Mart).
(() => {
  const { ink, field, row, tick, inline, signature, stamp } = window.K
  field('Order no.', 'KS-2026-0007')
  field('Date', '28 October 2026')
  const parties = document.querySelector('table.parties')
  row(parties, 'Name', [null, 'Sunrise Mini Mart (sample)'])
  row(parties, 'Business registration no.', ['W/00000 (sample)', 'W/11111 (sample)'])
  row(parties, 'Address', ['No. 1, Sample Lane, Colombo', '12 Main Street, Kandy'])
  row(parties, 'Email', [null, 'owner@example.com'])
  row(parties, 'Phone', [null, '077 000 0000'])
  row(parties, 'Represented by (name, title)', ['K. Karots, Owner', 'S. Sample, Owner'])
  field('Site (premises where it is used)', '12 Main Street, Kandy: shop floor and back office')
  field('Install ID', 'SAMPLE01')
  ;['Repairs', 'Reload', 'Combos', 'One PC', 'Phone', 'WhatsApp', 'Yes, copies', 'Cash', 'Bank transfer'].forEach(tick)
  inline('☑ Yes, copies kept for', '30')
  for (const s of document.querySelectorAll('.boxes span')) if (s.textContent.trim().startsWith('Hours')) ink(s.querySelector('.line'), 'Mon–Sat 9–6')
  field('Updates included until', '28 October 2027')
  field('Support until', '28 October 2027')

  const fees = document.querySelector('table.fees')
  row(fees, 'Licence', ['72,000', 'Core 59,000 (launch) + Repairs, Reload, Combos'])
  row(fees, 'Installation and training', ['7,000', 'On installation'])
  row(fees, 'Support and updates (per month / year)', ['Included', 'Year 1'])
  row(fees, 'Karots cloud backup (per month / year)', ['7,500', 'Yearly, in advance'])
  row(fees, 'Less discount', ['—', ''])
  row(fees, 'Total', ['86,500', ''])
  field('Total in words', 'Eighty-six thousand five hundred rupees only')
  field('Advance paid (Rs.)', '50,000')
  field('Balance due (Rs.) and date', '36,500 by 26 January 2027')
  field('Liability amount for clause 10.3(b) of the terms (Rs.)', '72,000')
  field('Anything else agreed', 'Karots types in the first 300 products at no charge.')

  // Acceptance: every row ticked and initialled, the guide's version written in.
  for (const tr of [...document.querySelectorAll('table.agree tr')].slice(1)) {
    tick('*')
    ink(tr.cells[2], 'S.S.')
    const v = tr.querySelector('.line'); if (v) ink(v, '1.1')
  }
  const signs = [...document.querySelectorAll('.sign')]
  const who = (t) => signs.find((s) => s.querySelector('.who')?.textContent === t)
  const people = {
    'For Karots': ['K. Karots', '000000000V (sample)', '28 October 2026', 'karots'],
    'For the Shop': ['S. Sample', '111111111V (sample)', '28 October 2026', 'shop'],
    'Witness 1': ['W. One', '222222222V (sample)'],
    'Witness 2': ['W. Two', '333333333V (sample)'],
  }
  for (const [k, [name, nic, date, seal]] of Object.entries(people)) {
    const s = who(k)
    field('Name', name, s); field('NIC no.', nic, s); if (date) field('Date', date, s)
    s.querySelector('.sigline').insertAdjacentHTML('beforebegin', signature(name))
    if (seal) s.querySelector('.seal').insertAdjacentHTML('beforeend', stamp(seal))
  }

})()
