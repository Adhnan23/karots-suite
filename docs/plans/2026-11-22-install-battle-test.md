# Battle test: installing and running a shop

Rule book: `docs/rules/install.md` (BAK, CPY, UPD, SET, BLD). Same layers as the core areas, but the auditor is the
**round trip** (data → backup → restore → same rows, `ledgercheck` passes). Test-first: a failing test naming the rule,
then the fix, one commit each. Delete this plan when done.

## Desk work
- [x] Rule book + grid, questions Q1–Q3 answered (2026-11-22)

## Round trip and backups
- [ ] `ltest.RoundTrip(t, db)`: dump → restore into a fresh template copy → every table compared row for row (secret
      columns, `backup_copies`, goose excluded) → `ledgercheck` on the copy. Used by every case below.
- [ ] BAK-1 failing test: backups taken while sales run on two goroutines restore and audit clean → fix: `Dump` reads
      in one `REPEATABLE READ READ ONLY` transaction.
- [ ] BAK-6 failing test: a sale at once with a Data page restore lands in the restored shop or is refused, never
      lost → fix: the restore locks the tables, writes its safety copy inside the same transaction, then replaces.
- [ ] BAK-4: rotation counts only automatic backups; safety copies and `cloud-*` fetched ones kept 30 days.
- [ ] Cases: BAK-2 (receipt numbers and ids continue), BAK-5 (bad files, dangling reference), BAK-7 (map
      restore_compat tests), BAK-8, BAK-9 (another person's session signed out), BAK-10 (setup-only restores into an
      empty shop), BAK-11 refusals, BAK-12.
- [ ] Machine: the sales / money / stock machines take a backup now and then, round-trip it into a side database and
      audit it (every Nth step).
- [ ] Faults: the real binary killed during `-backup`, an automatic backup and `-restore` → no bad `.json.gz`, data
      as before (power-cut harness).

## Copies
- [ ] Map `copies*_test.go` to CPY-1–6; add what's missing (CPY-2 at once, CPY-5 refusals, CPY-6 preset).

## Updates
- [ ] Map `internal/update` tests to UPD-1–5, 7; add what's missing.
- [ ] UPD-4 faults: an install stopped after each step (download, backup, swap, mark) still leaves a program that
      starts.
- [ ] UPD-6: a guard test over `migrations/` and plugin migrations newer than 00042 refusing DROP / RENAME / SET NOT
      NULL / NOT NULL without DEFAULT; and a battle test that builds the previous release's server (git worktree at
      the last Release commit), migrates with HEAD, starts the old binary and sells.
- [ ] UPD-8: a Pay sent across a server restart is one sale (SALE-50 over a real restart).

## Setup and the Shop Builder
- [ ] SET-3 failing test: Setup's Update backs up to the keep dir before the program step; a failed backup changes
      nothing → add the step.
- [ ] Map `cmd/setup` tests to SET-1–9 and `cmd/bootstrap` tests to BLD-1–5; add what's missing.

## Close
- [ ] Mutation: gremlins on `internal/backup`, `internal/update`, `cmd/setup`, replayed against own + battle tests;
      read survivors, add tests.
- [ ] `make battle`, full run with coverage, coverage map (add rows), `docs/status.md`, delete this plan.
- [ ] Then: real-printer and real-phone tests, before the Release.
