# Promo videos

Vertical (1080×1920) promo videos for Facebook, WhatsApp status, TikTok and Instagram Reels: one main promo and
eight shorts, each in Sinhala, Tamil and English. Made with [Remotion](https://www.remotion.dev) from data. It's its
own bun project and never goes into the shop's program. Design: `docs/specs/2026-10-28-promo-videos-design.md`.

> **The Sinhala and Tamil lines are drafts.** Have a fluent speaker read `src/lines/si.ts` and `src/lines/ta.ts`
> (captions *and* voice lines) before anything is posted.

## Make them

```sh
cd videos && bun install
bun run voice                 # computer voice for every line (only changed lines are redone)
cd .. && make demo            # if the Demo Room isn't built
./dist/demo/karots-suite-demo-room &   # leave it running
make videos-clips             # screen recordings → videos/public/clips
make videos                   # all 27 → videos/out/<lang>/karots-suite-<video>-<lang>.mp4
```

`bun run render --lang si --video credit` makes just one. `bun run studio` opens Remotion's preview.

## Change something

| To change | Edit | Then |
|---|---|---|
| A caption or a voice line | `src/lines/<lang>.ts` (the scene's id) | `bun run voice`, `make videos` |
| Which scenes a video has, or their screens | `src/videos.ts` | `make videos` |
| A voice | `src/voices.ts` — samples of each in `out/samples/` (`bun run voice --samples`) | `bun run voice --lang <lang> --force` |
| A screen recording | `web/docs-shots/clips.spec.ts` | `DOCS_ONLY=<clip> make videos-clips` |
| The end card (phone number, Pocket Cashier) | `src/EndCard.tsx` | `make videos` |

`bun test` checks every language has every line, no prices or tech words slipped in, and every screen exists.

## Music

`public/music.mp3` plays quietly under the voice (louder on the end card). Without it the videos render without
music. Use only a track whose licence allows ads on social media (e.g. the Pixabay Content License) and note its
title, author, link and licence in `MUSIC.md`.
