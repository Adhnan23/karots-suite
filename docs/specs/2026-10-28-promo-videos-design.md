# Promo videos — design (2026-10-28)

## Intent

Short vertical videos that sell Karots Suite to small Sri Lankan shop owners on **Facebook, WhatsApp status,
TikTok and Instagram Reels**. Many owners don't know software, so the videos **never sell technology** (no "React",
"cloud", "AI", "database"). Every scene names a headache the owner has today and shows it gone, with a real screen
from the suite as proof.

Success: an owner watching with the sound off understands each point from the big caption alone, and the video ends
with one easy action: **WhatsApp for a free demo**.

## Goals

- **Main promo**: 60–90 s. Points 1–5 below get a scene each, then a quick montage of the rest, then the end card.
- **8 shorts**: 15–20 s each, one problem per short, the same end card. Shorts:
  1. You don't have to stand at the counter (watch sales + staff from your phone, from anywhere — remote access is
     set up at install via Tailscale, or VPS hosting; a small caption: "needs internet at the shop").
  2. Stop guessing what to buy (low stock, smart reorder by supplier).
  3. No more credit books (who owes, how much, since when, print a statement).
  4. Staff can't cheat quietly (own PIN each, returns / discounts recorded, till locks itself, drawer short warning at close).
  5. What you owe suppliers (paid vs still to pay).
  6. Your real profit (by day, item, month — not just sales).
  7. Never lose your shop (works without internet, automatic backups to USB + cloud, runs on the PC you have).
  8. Made for your kind of shop (add-ons: pharmacy expiry + batches + sell by card/tablet; phone shop repairs +
     reload float; print/communication shop; clearance of near-expiry; combo deals; customer requests;
     similar-item suggestion when out of stock).
- **Three languages, separate versions**: Sinhala, Tamil, English. Captions and voice in that language.
  → 9 videos × 3 languages = **27 MP4s**.
- **Format**: 9:16 vertical, 1080×1920, 30 fps, H.264 + AAC (fits all four platforms).
- **Sound**: computer voice reading the captions + quiet royalty-free music under it.
- **End card (every video)**: logo `docs/images/logo.jpg`, "Karots Suite", a phone mockup labelled
  **"Coming soon: Pocket Cashier"** — line: *Power cut? PC broken? Keep selling from your phone.* — and
  **"WhatsApp us for a free demo · 076 962 6396"** with the WhatsApp mark.

Main-promo montage points (one caption + one screen each, ~2 s): supplier dues, real profit, customer requests,
out-of-stock suggestions, barcode billing / search in Sinhala, Tamil or English, hold a bill, print your own price
labels and barcode stickers, receipts in your shop's style, no internet needed, automatic backups, the add-ons.

## Non-goals

- No horizontal (16:9) or square versions now; add a 1:1 render later only if Facebook feed posts need it.
- No prices anywhere in the videos.
- No how-to / training videos.
- No real Pocket Cashier screens (the app doesn't exist yet) — the phone mockup is drawn.

## Anti-goals

- Tech words or feature names the owner wouldn't use ("plugin", "JWT", "PostgreSQL", "Demo Room").
- Promising something the suite doesn't do. Every claim must match a real screen; Pocket Cashier is always
  marked "Coming soon".
- Captions too small or too fast to read on a phone with the sound off.
- Fake or unlicensed music; real customer data or real secrets on screen (install ids, PINs, `.env`).

## Constraints

- Lives in **`videos/`** at the repo root with its **own `package.json`** (bun). Never imported by `web/` or the Go
  build; never shipped in the binary. Add `videos/out/` and `node_modules` to `.gitignore`; renders are not committed.
- Built with **Remotion** (React → video). Use Remotion's official Claude Code skills
  (`remotion.dev/docs/ai/claude-code-plugin`) while building.
- **Content is data**: one script file per language (scene id → caption + voice line) and one scene list per video.
  Changing a line or adding a language = edit data + rerun one command that renders all 27.
- Screens come from the **Demo Room's Karots Super Store** (every plugin on, a believable year of data):
  - existing screenshots in `docs/images/**` (the `phone/` set is already 390 px wide — good for vertical);
  - new short screen recordings via Playwright `recordVideo`, reusing the unlock / chrome-hiding flow of
    `web/docs-shots/shots.spec.ts` (hide `.ksd`). Admin clips record at phone width; till clips record at desktop
    width and are zoomed / panned to the part that matters in the vertical frame.
  - The recording spec lives in `videos/` (or beside docs-shots with its own config) and is re-runnable.
- **Voice**: Microsoft neural voices via the free `edge-tts` tool (no account): `si-LK-SameeraNeural` /
  `si-LK-ThiliniNeural`, `ta-LK-KumarNeural` / `ta-LK-SaranyaNeural`, English `en-IN-*` or `en-GB-*`. Generate one
  audio file per scene line; scene length follows the audio length (+ a short pause). Send the user samples of each
  voice to choose from before the full render.
- **Translations**: Claude drafts Sinhala and Tamil; **the user (or someone fluent) checks them before anything is
  posted**. Fonts must render Sinhala and Tamil properly (e.g. Noto Sans Sinhala / Noto Sans Tamil, bundled locally).
- **Screens stay English** in all three languages (user: OK); only captions and voice are translated, so one set of
  recordings serves every language.
- **Paid extras are tagged** "optional add-on" (cloud backup copy, every add-on in short 8 and the montage) — no
  promise that they come with the suite.
- **Length**: a voice a few seconds over target is fine; well over → shorten that language's line.
- **Before recording — prerequisite change (separate commit, core):** product search matches the **local name**
  (`products.name_local`, already on the product form and receipts) everywhere products are searched — the till's
  `searchClause` in `internal/features/products/search.go` and the other product `ILIKE` searches (warranty,
  conversions, productplus, alternatives, pharmacy). The demo shops get Sinhala / Tamil local names on common
  products so "search in Sinhala, Tamil or English" can be recorded; guide page updated.
- Look: the suite's own look (Studio palette colours, logo), bold big captions, simple motion — not a generic template.
- AGENTS.md rules that still apply: no AI attribution in commits; files ≤ 500 lines.

## Decided by the user

- Practical, everyday-problem messaging, not technology.
- Main promo + shorts; the feature list above.
- Advertise Pocket Cashier with a mockup, marked "Coming soon".
- Facebook, WhatsApp, TikTok, Instagram Reels.
- Sinhala, Tamil and English as separate versions.
- Music and computer voice, both.

## Claude's calls (user said "anything you recommend")

- Vertical only; 60–90 s main + 8 shorts of 15–20 s.
- Call to action: WhatsApp for a free demo, 076 962 6396.
- edge-tts voices; royalty-free music from Pixabay / YouTube Audio Library (Claude picks an upbeat track unless the
  user supplies one; keep its licence note in `videos/`).
- The 8-short split above.

## Left to the builder

Scene timing and transitions, caption styling within the suite's palette, which screenshot vs recording backs each
scene, the exact recording steps, the Remotion project layout, and the music ducking level.
