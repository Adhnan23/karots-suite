# Karots cloud backup — bucket setup (M8c2)

Paying shops' automatic backups also go, sealed (AES-256-GCM with the shop's own `cloudKey` from `shops.json`), to
the vendor's R2 bucket — about one a day per shop, kept 30 days. Spec: `docs/specs/2026-10-20-m8c2-backup-copies-design.md`.

## Once

1. Cloudflare → R2 → **Create bucket** `karots-suite-backups` (not public; separate from the `karots-suite` update bucket).
2. R2 → **Manage API tokens** → Create: **Object Read & Write**, scoped to `karots-suite-backups` **only**.
   Put it in the Builder's root `.env`:
   ```
   CLOUD_BACKUP_URL=https://<account id>.r2.cloudflarestorage.com/karots-suite-backups
   CLOUD_BACKUP_KEY_ID=<access key id>
   CLOUD_BACKUP_SECRET=<secret access key>
   ```
3. Bucket → Settings → **Object lifecycle rules**: delete objects after **30 days**.
4. **Bucket lock** (no shop — or a leaked token — can delete or overwrite a copy for 30 days):
   ```
   wrangler r2 bucket lock add karots-suite-backups --name keep-30 --retention-days 30
   ```

Free plan: 10 GB stored, 1 M Class A (writes / lists) and 10 M Class B (reads) a month. A shop writes ~30 objects a
month and lists only when someone opens "Backups here…" or Setup's Bring back a shop — far inside it.

## Per shop

- Shop Builder → Edit → Backups → tick **Karots cloud backup** → Save (the shop's `cloudKey` is made then and never
  changes — older copies open only with it; `shops.json` is the only copy, so it stays committed in the private repo).
- **Release** the shop and have it run Setup → Data → Updates → Update. On the Data page the cloud row appears; only
  Karots (the system user) can switch it on there, any admin can switch it off.
- **Look at / fetch a shop's copies:** Builder → the shop's **Cloud backups** → Download saves one, opened, into
  `dist/<key>/` (carry it to a PC with Setup). The shop's own Data page and Setup's Bring back a shop list them too.

## When things change

- **A token leaks:** make a new token (step 2), put it in `.env`, delete the old token, **Release all**, shops Update.
  The bucket lock keeps the old token from deleting anything meanwhile; the copies stay sealed with each shop's key.
- **A shop stops paying:** untick Karots cloud → Release → the shop runs Update (its `.env` loses `CLOUD_BACKUP_*`;
  the Data page's cloud row goes). What is in the bucket ages out in 30 days.
