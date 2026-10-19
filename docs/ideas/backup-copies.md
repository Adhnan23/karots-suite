# Idea: Backup copies (M8c2, after M8c)

Captured 2026-10-19 from the user: periodic backups should not only land in the backup folder — also on a USB flash
drive, a network drive, an S3 bucket or a VPS. Spec it after M8c (it reuses M8c's S3 client and the shop profiles).

- **After each automatic backup**, copy the file to every place the shop added. Places: a **folder** (USB stick,
  `\\server\share`, a mounted path) or an **S3 bucket** (Cloudflare R2, Backblaze B2, AWS; a VPS runs MinIO / Garage —
  no SFTP, it would add two dependencies for what S3 covers).
- **Never stop a sale:** a missing USB / unreachable share is skipped, the missed backups are copied when it is back,
  and the Data page + admin home say "USB last copied 3 days ago". A failed copy only warns.
- **Keep N per place** (as `BACKUP_KEEP`).
- **Off-PC copies are encrypted** (customers' names, phones, money). The key lives in the shop profile (`shops.json`)
  so the vendor can always open them; in the shop it is a `DataGroup.Secret` (never inside a backup).
- **Restore from a copy:** the Data page lists a place's backups and restores one.
- **Setup → Data → Backup copies:** Add (USB / folder / S3), Test. The Shop Builder can preset an S3 place per shop
  (each shop in its own folder of the vendor's bucket) → an off-site copy of every shop for support.
- karots-pos `cmd/release/r2.go` is a stdlib-only S3 client (SigV4) — fork it once for M8c publishing and reuse here.
