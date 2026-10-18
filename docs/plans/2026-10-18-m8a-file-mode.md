# M8a — File mode (plan)

Spec: `docs/specs/2026-10-18-m8a-file-mode-design.md`. The user said "do as recommended and complete this": port **5434**,
**auto-backup on** in file mode, data folder **next to the binary**.

- [ ] **1. `internal/filedb`.** `Open(Options{Dir, Port})` places the embedded bundle (from the `filemode` build tag,
  or the library's download when there's none, for tests), then:
  - refuses root on Linux;
  - refuses a data dir that holds another Postgres major (the library would **wipe** it) or isn't a Postgres dir;
  - stops a leftover Postgres;
  - starts on 127.0.0.1 with C.UTF-8 on Linux, en_US.UTF-8 on macOS, the default on Windows;
  - fills `data/secret` (JWT secret + DB password) once and loads it.

  `Stop()`. Tests (skipped in `-short`): restart reuses data, leftover stopped, secret stable, Sinhala trigram hit,
  wrong-version dir refused.
- [ ] **2. Server wiring.**
  - When there's no `DATABASE_URL` and the build has the bundle: bind the HTTP port first (a second copy stops
    there), open filedb, set `DATABASE_URL`, `JWT_SECRET` and `BACKUP_DIR` (defaults only), then run as usual.
  - Stop Postgres on shutdown.
  - `-data <dir>` flag.
  - `cfg.FileDataDir`.
- [ ] **3. Shows the mode.** `DataPage.fileDataDir`: Setup → Data says "File mode: data in …" with the move-to-server
  steps and the copy-backups nudge. The Konami line shows file / server.
- [ ] **4. Shop Builder.** Database choice (One PC / Server) and `-mode file|server`. File mode fetches the bundle
  into the builder's cache, drops it in `internal/filedb/bundle/`, builds `-tags filemode`, then removes it
  (gitignored as well). Test: the build args and the tree clean afterwards.
- [ ] **5. Close.** A real file-mode build on Linux: start, sell, close, start again with the data still there, kill
  -9 then start; AGENTS.md, status.md, differences; `make verify`. Windows check listed for the user.
