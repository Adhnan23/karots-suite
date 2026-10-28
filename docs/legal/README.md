# Legal pack — what Karots needs, and why

*Prepared 2026-10-28 from the Sri Lankan statutes below. This is research for a lawyer to check,
not legal advice. Have a Sri Lankan attorney-at-law review it before selling under it.*

## Do we need separate agreements, or is one licence enough?

**Mostly one document.** The [`LICENSE`](../../LICENSE) is written as one set of terms with two
schedules:

| Piece | Where | Why it's there |
|---|---|---|
| Licence (what the shop may and may not do) | `LICENSE` §1–12 | Copyright in the program; the Intellectual Property Act protects computer programs |
| Services (updates, support, cloud backup) | `LICENSE` Schedule A | So paid services sit under the same terms, with no second contract |
| Personal data handling | `LICENSE` Schedule B | The PDPA requires a written contract between a controller (the shop) and a processor (Karots, for cloud backups and support) |
| **Per-shop details: price, plugins, install ID, support period, liability amount** | **[Order Form](karots-suite-agreement.pdf), signed once per shop** | The only part that changes per shop. It pulls in the LICENSE by reference |
| What's free, what's charged, visits, cloud backup, refunds, shop dos and don'ts, price list | **[Shop Rules & Service Guide](karots-suite-shop-rules.pdf)** (`agreement/shop-rules.html`) | Brought in by `LICENSE` §8.2 and the Order Form's acceptance line, so signing accepts it too. Change its version date on every edit |

You need **one more agreement only if you take on resellers**. A reseller is a different party
with different rights (to sell, install and support, with commission and territory), so that
belongs in its own **Reseller Agreement**. Have the lawyer draft it when the first reseller
signs up.

**Print [`karots-suite-agreement.pdf`](karots-suite-agreement.pdf)** for each shop. It holds the Order Form with blanks
(shop details, plugins, the agreed price, advance and balance, signatures, seals and two witnesses) and the full terms, with
initials and page numbers on every page. Rebuild it with `make agreement` after any change to `LICENSE` or
`agreement/` (the form's source).

So for each shop: **the shop signs the Order Form** (paper, or a PDF signed electronically), and
the LICENSE comes with it. Recommended: **karots-setup also shows the terms with an "I agree"
tick** before Install, so every install has an electronic acceptance on record.

## What the law says, and how the licence follows it

### Copyright: Intellectual Property Act, No. 36 of 2003

- A **computer program** is a protected work. The owner has the exclusive right to reproduce,
  adapt, distribute and **rent** it (s.9(1)).
- The **lawful owner of a copy** may make a single copy or adaptation **when it's necessary to use
  the program**, or **for archival / replacement** if the copy is lost or damaged (s.12(7)). The
  licence must not forbid this, so `LICENSE` §3.2 allows it in those words.
- No decompilation or reverse-engineering exception was found in the Act, so §4(c) forbids it
  "except as the law expressly allows".
- Wilful infringement is an offence: a fine of up to Rs. 500,000 or 6 months, doubled on a repeat
  (s.178). `LICENSE` §4 points to it.

### Unfair terms: Unfair Contract Terms Act, No. 26 of 1997

This Act matters most for the "no warranty / limit of liability" clauses:

- **Liability for death or personal injury from negligence cannot be excluded** (s.3(1)), so
  `LICENSE` §10.1 doesn't try.
- Other exclusions for negligence must meet the **reasonableness** test (s.3(2)).
- When one side contracts on the **other's written standard terms** (our LICENSE is exactly
  that), the supplier can't exclude liability for its own breach, or excuse doing something
  substantially different, unless the term is reasonable (s.4).
- On reasonableness, **it's for Karots to prove** the term is reasonable (s.10(5)). For a cap at
  a fixed sum, the court looks at **Karots' resources and the insurance available** (s.10(4)).
  That's why `LICENSE` §10.3 caps at fees paid (or the Order Form amount) and says it considered
  both. §9.2 also gives a real remedy (fix, work around or refund) rather than "as is" alone.
- A written **arbitration** agreement doesn't count as excluding liability (s.11 proviso), so
  §12.2's arbitration clause is safe under this Act.
- "Deals as consumer" means someone not acting in a business (s.13(2)). Shops buy in the course
  of business, and §2.2 has them confirm it. That also keeps the Consumer Affairs Authority
  Act's consumer warranties out of the picture, which the courts apply to consumer sales.
- **For the lawyer:** the Act's **First Schedule** lists contracts excluded from ss.3–5. The UK
  model excludes contracts about the **creation or transfer of intellectual property**. Check
  whether Sri Lanka's does too: if so, the licence part may fall outside ss.3–5.

### Electronic acceptance: Electronic Transactions Act, No. 19 of 2006 (amended by No. 25 of 2017)

A contract isn't unenforceable just because it was made electronically. The 2017 amendment
aligns the Act with the UN Electronic Communications Convention, which Sri Lanka ratified in
2015. So "I agree" in karots-setup and an emailed or e-signed Order Form are both fine.
`LICENSE` §2.1.

### Personal data: Personal Data Protection Act, No. 9 of 2022 (amended by No. 22 of 2025)

- **In force now:** only the Data Protection Authority and the administrative parts (from July and
  December 2023).
- **Not yet in force:** data subjects' rights, controller and processor duties, processing rules
  and penalties (Parts I, II, III, VII). The 2025 amendment removed the fixed deadlines, and the
  Minister will set the dates by Gazette. As of mid-2026 none had been announced. **Check the
  Gazette and dpa.gov.lk before signing the first shop.**
- When they commence: a **controller** (each shop) needs a **written contract with its
  processors** (Karots), and **health data is a special category** (the Pharmacy plugin keeps
  patients' allergies and prescriptions). There will also be **cross-border transfer rules**
  (Karots cloud backups can be stored outside Sri Lanka), breach notification, and penalties of
  up to Rs. 10 million.
- `LICENSE` Schedule B is written to that shape now (instructions only, confidentiality,
  encryption, breach notice, help with requests, deletion at the end, transfers agreed), so
  little should need to change when the Act commences.

## Checklist for the lawyer

1. Read `LICENSE` and the Order Form. Is the liability cap (§10.3) reasonable under UCTA s.10?
   What insurance would make a higher figure reasonable?
2. UCTA First Schedule: are IP licences excluded from ss.3–5?
3. Is "Karots Korp (Karots Dev)" the right legal name for the contracting party? If it's a sole
   proprietorship, use the owner's name "trading as Karots Korp", plus the business
   registration number.
4. Stamp duty or any other formality for signing the Order Form?
5. Arbitration in Colombo, one arbitrator: suitable for small claims, or do the Magistrate /
   District Courts suit better?
6. PDPA: has a commencement order been made yet? If so, are there extra processor terms, and is
   a Data Protection Officer needed?
7. Cloud backups are stored with a foreign provider: are any conditions needed for the transfer?
8. Should the licence also cover the **Demo Room** (given to prospects, not shops)? Today its
   lock code and watermark protect it. A short evaluation notice may help.
9. Draft a **Reseller Agreement** when the first reseller signs.
10. Shop Rules §7 refund scale: after day 7, Karots keeps the set-up fee plus 25 / 50 / 75 % of the licence by week,
    with nothing refunded after day 30. Is this reasonable under UCTA s.4 and s.10? Is it fair when the shop paid only an advance?
11. Shop Rules §5: freezing cloud backup after 7 days unpaid, and §1: no free updates or fixes after year 1. Does that sit
    with `LICENSE` §9 (fit for purpose)?

## Sources

- [Intellectual Property Act, No. 36 of 2003 (WIPO Lex)](https://www.wipo.int/wipolex/es/text/597541)
- [Unfair Contract Terms Act, No. 26 of 1997 (lankalaw.net)](https://lankalaw.net/wp-content/uploads/2025/02/1997Y0V0C26A.html) ·
  [srilankalaw.lk](https://www.srilankalaw.lk/revised-statutes/volume-viii/1319-unfair-contract-terms-act.html)
- [Electronic Transactions (Amendment) Act, No. 25 of 2017](https://www.documents.gov.lk/view/acts/2017/11/25-2017_E.pdf)
- [Personal Data Protection Act — Wikipedia (amendment No. 22 of 2025)](https://en.wikipedia.org/wiki/Personal_Data_Protection_Act_(Sri_Lanka)) ·
  [Data Protection Authority of Sri Lanka](https://www.dpa.gov.lk/) ·
  [PDPA 2026 status guide (Hashtag Coders)](https://hashtagcoders.lk/blogs/data-privacy-laws-sri-lanka-2026)
- [Consumer Affairs Authority implied warranty case law (Kia Motors, v2 Sri LR 299)](https://lankalaw.net/wp-content/uploads/2020/v2-sri-lr-299.pdf)
