import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ImagePlus, Printer, Trash2 } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, apiUpload } from '@/api/client'
import type { SettingsPage as Page, ShopSettings } from '@/api/types.gen'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Paper } from '@/features/counter/receipts/Paper'
import { errMsg } from '@/features/till/api'
import { slots } from '@/plugins/slots'
import { cn } from '@/lib/utils'
import { Section, Segmented, settingsQuery, useSettingsSaved } from './kit'

const FONT = [{ value: '1', label: 'XS' }, { value: '2', label: 'S' }, { value: '3', label: 'M' }, { value: '4', label: 'L' }]
const int = (s: string) => Math.max(0, Math.trunc(Number(s) || 0))

/** Setup → Settings (karots-pos /admin/settings): the shop's name and receipt,
 * tax, the till's cash rules, stock tools, staff sign-in and printers. */
export function SettingsPage() {
  const page = useQuery(settingsQuery).data
  if (!page) return <PageHead title="Settings" sub="Loading…" />
  const s = page.settings
  return (
    <>
      <PageHead title="Settings" sub="How the till, receipts, printers and staff sign-in work. Each card saves on its own." />
      <div className="grid items-start gap-3 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="grid min-w-0 gap-3">
          <Section title="Shop & receipt" sub="Your name, contact and currency — they head every receipt." saved={s}
            keys={['shopName', 'shopNameLocal', 'address', 'phone', 'currencyCode', 'currencySymbol', 'receiptFooter', 'receiptWidth', 'defaultSaleType', 'askToPrint', 'logoUrl']}>
            {({ v, set }) => (
              <>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="Shop name"><input className={INPUT} value={v.shopName} onChange={(e) => set('shopName', e.target.value)} /></Field>
                  <Field label="Name in your language"><input className={INPUT} value={v.shopNameLocal} placeholder="e.g. කඩේ / கடை" onChange={(e) => set('shopNameLocal', e.target.value)} /></Field>
                  <Field label="Address" className="sm:col-span-2"><input className={INPUT} value={v.address} onChange={(e) => set('address', e.target.value)} /></Field>
                  <Field label="Phone"><input className={INPUT} value={v.phone} inputMode="tel" onChange={(e) => set('phone', e.target.value)} /></Field>
                  <div className="grid grid-cols-2 gap-3">
                    <Field label="Currency"><input className={INPUT} value={v.currencyCode} onChange={(e) => set('currencyCode', e.target.value)} /></Field>
                    <Field label="Symbol"><input className={INPUT} value={v.currencySymbol} onChange={(e) => set('currencySymbol', e.target.value)} /></Field>
                  </div>
                  <Field label="Receipt footer" className="sm:col-span-2"><input className={INPUT} value={v.receiptFooter} placeholder="Thank you, come again!" onChange={(e) => set('receiptFooter', e.target.value)} /></Field>
                </div>
                <div className="flex flex-wrap gap-x-6 gap-y-3">
                  <Segmented label="Receipt paper" value={v.receiptWidth} onChange={(x) => set('receiptWidth', x)} options={[{ value: '80', label: '80 mm' }, { value: '58', label: '58 mm' }]} />
                  <Segmented label="Default sale type" value={v.defaultSaleType} onChange={(x) => set('defaultSaleType', x)} options={[{ value: 'retail', label: 'Retail' }, { value: 'wholesale', label: 'Wholesale' }]} />
                </div>
                <Toggle label="Ask before printing" hint="Show Print / New sale after each sale. Off prints the receipt straight away." checked={v.askToPrint} onChange={(x) => set('askToPrint', x)} />
                <Field label="Logo address (online only)" hint="For a logo that prints without the internet, upload one below.">
                  <input className={INPUT} value={v.logoUrl} placeholder="https://…" onChange={(e) => set('logoUrl', e.target.value)} />
                </Field>
              </>
            )}
          </Section>
          <Logo page={page} />
          <Section title="Tax" sub="Only a shop registered for VAT may add tax to a bill. Until this is on with its number, no tax is charged — even on products that have a tax rate." saved={s} keys={['taxRegistered', 'taxRegNo']}>
            {({ v, set }) => (
              <>
                <Toggle label="Registered for VAT" checked={v.taxRegistered} onChange={(x) => set('taxRegistered', x)} />
                {v.taxRegistered && (
                  <Field label="VAT registration number" hint="Printed on receipts. Products' tax rates apply from the next sale.">
                    <input className={INPUT} value={v.taxRegNo} onChange={(e) => set('taxRegNo', e.target.value)} />
                  </Field>
                )}
              </>
            )}
          </Section>
          <Section title="Till & cash" sub="The drawer, and how cash going in and out of it is tracked." saved={s}
            keys={['openCashDrawer', 'drawerKickPin', 'allowUntrackedCash', 'defaultLockerId', 'allowContinueLastCount']}>
            {({ v, set }) => (
              <>
                <Toggle label="Open the drawer on cash events" hint="Needs a drawer wired to the receipt printer. Cash sales, deposits, withdrawals, credit collected and no-sale — never card-only sales."
                  checked={v.openCashDrawer} onChange={(x) => set('openCashDrawer', x)} />
                {v.openCashDrawer && <Segmented label="Drawer pin (if it never opens, try pin 5)" value={v.drawerKickPin} onChange={(x) => set('drawerKickPin', x)} options={[{ value: 0, label: 'Pin 2 — most drawers' }, { value: 1, label: 'Pin 5' }]} />}
                <Field label="Default locker" hint={page.lockers.length ? 'Picked for you in every till money move (open, deposit, withdraw, close).' : <>Add a locker on <Link to="/admin/lockers" className="text-brand underline">Lockers</Link> first.</>}>
                  <select className={INPUT} value={v.defaultLockerId} disabled={!page.lockers.length} onChange={(e) => set('defaultLockerId', Number(e.target.value))}>
                    <option value={0}>None — the cashier chooses</option>
                    {page.lockers.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}
                  </select>
                </Field>
                <Toggle label="Allow untracked cash" hint="Off: withdrawing or closing the till must put the cash in a locker. Money in is never blocked."
                  checked={v.allowUntrackedCash} onChange={(x) => set('allowUntrackedCash', x)} />
                <Toggle label="Offer “Continue last count” when opening" hint="Turn off for one drawer shared by several people — a carried-over count goes stale."
                  checked={v.allowContinueLastCount} onChange={(x) => set('allowContinueLastCount', x)} />
              </>
            )}
          </Section>
          <Section title="Stock" sub="Alerts, and the tools that rewrite quantities and prices." saved={s} keys={['lowStockAlerts', 'stockTakeEnabled']}>
            {({ v, set }) => (
              <>
                <Toggle label="Low-stock alerts" hint="Flag products at or under their reorder level on the dashboard." checked={v.lowStockAlerts} onChange={(x) => set('lowStockAlerts', x)} />
                <Toggle label="Stock-take & intake" hint="Off hides the Stock-take and Intake pages." checked={v.stockTakeEnabled} onChange={(x) => set('stockTakeEnabled', x)} />
              </>
            )}
          </Section>
          <Section title="Staff sign-in" sub="PIN rules and the screen lock." saved={s} keys={['forcePinChange', 'allowCashierPinChange', 'lockTimeoutMinutes']}>
            {({ v, set }) => (
              <>
                <Toggle label="New PINs must be changed at first sign-in" hint="New and reset accounts choose their own PIN before they can work." checked={v.forcePinChange} onChange={(x) => set('forcePinChange', x)} />
                <Toggle label="Cashiers may change their own PIN" checked={v.allowCashierPinChange} onChange={(x) => set('allowCashierPinChange', x)} />
                <Field label="Auto-lock after (minutes idle)" hint="0 = never" className="max-w-xs">
                  <input className={INPUT} inputMode="numeric" value={v.lockTimeoutMinutes} onChange={(e) => set('lockTimeoutMinutes', Math.min(1440, int(e.target.value)))} />
                </Field>
              </>
            )}
          </Section>
          <Printers page={page} />
          {slots.settingsSections.map(({ key, title, Section: Plug }) => (
            <section key={key} aria-label={title} className={cn(SURFACE, 'p-4 md:p-5')}><h2 className="mb-3 font-display text-lg font-bold">{title}</h2><Plug /></section>
          ))}
        </div>
        <Preview s={s} logo={page.logo} />
      </div>
    </>
  )
}

/** The receipt as it will look, from the saved settings. */
function Preview({ s, logo }: { s: ShopSettings; logo: string }) {
  const narrow = s.receiptWidth === '58'
  return (
    <aside aria-label="Receipt preview" className="min-w-0 lg:sticky lg:top-4">
      <div className={cn(SURFACE, 'p-4')}>
        <h2 className="mb-3 text-sm font-bold text-muted-foreground">Receipt preview · {s.receiptWidth} mm</h2>
        <Paper shop={{ name: s.shopName, address: s.address, phone: s.phone, footer: s.receiptFooter }} logo={logo} narrow={narrow}>
          <div className="flex justify-between"><span>Basmati 5kg</span><span>{s.currencySymbol} 2,450.00</span></div>
          <div className="flex justify-between"><span>Milk 1L ×2</span><span>{s.currencySymbol} 760.00</span></div>
          <hr className="my-2 border-dashed border-ink" />
          <div className="flex justify-between font-bold"><span>Total</span><span>{s.currencySymbol} 3,210.00</span></div>
          {s.taxRegistered && s.taxRegNo && <div className="mt-1 text-center">Tax reg. {s.taxRegNo}</div>}
        </Paper>
      </div>
    </aside>
  )
}

function Logo({ page }: { page: Page }) {
  const input = useRef<HTMLInputElement>(null)
  const done = useSettingsSaved()
  const [busy, setBusy] = useState(false)
  const run = async (p: Promise<Page>, msg: string) => {
    setBusy(true)
    try { done(await p); toast.success(msg) } catch (e) { toast.error(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <section aria-label="Logo" className={cn(SURFACE, 'p-4 md:p-5')}>
      <h2 className="font-display text-lg font-bold">Logo</h2>
      <p className="mb-3 text-[13px] text-muted-foreground">Printed at the top of receipts. Any picture works — it's shrunk to 400 px and kept here, so it prints offline.</p>
      <div className="flex flex-wrap items-center gap-3">
        <div className="grid size-20 place-items-center overflow-hidden rounded-[var(--radius-sm)] border border-border bg-paper">
          {page.logo ? <img src={page.logo} alt="Shop logo" className="max-h-full max-w-full object-contain" /> : <ImagePlus size={22} className="text-ink/40" />}
        </div>
        <Button variant="outline" className="h-10 gap-1.5 px-4" disabled={busy} onClick={() => input.current?.click()}><ImagePlus size={16} /> {page.logoUploaded ? 'Replace' : 'Upload'}</Button>
        {page.logoUploaded && <Button variant="ghost" className="h-10 gap-1.5 px-3 text-danger" disabled={busy}
          onClick={() => void run(api<Page>('/admin/settings/logo/clear', { method: 'POST' }), 'Logo removed')}><Trash2 size={16} /> Remove</Button>}
        <input ref={input} type="file" hidden accept="image/*" onChange={(e) => {
          const f = e.target.files?.[0]
          e.target.value = ''
          if (f) void run(apiUpload<Page>('/admin/settings/logo', 'logo', f), 'Logo uploaded')
        }} />
      </div>
    </section>
  )
}

function Printers({ page }: { page: Page }) {
  const s = page.settings
  const [testing, setTesting] = useState(false)
  const test = async () => {
    setTesting(true)
    try { await api('/admin/settings/printer/test', { method: 'POST' }); toast.success(`Test slip sent to ${s.receiptPrinter}`) } catch (e) { toast.error(errMsg(e)) } finally { setTesting(false) }
  }
  return (
    <Section title="Printers & labels" sub="Pick a printer this computer found, or type a network printer as tcp://192.168.1.50:9100. Label size is the default — change it per print." saved={s}
      keys={['receiptPrinter', 'labelPrinter', 'labelWidthMm', 'labelHeightMm', 'labelGapMm', 'labelNameLines', 'labelBottomLines', 'labelNameFont', 'labelBottomFont']}>
      {({ v, set }) => (
        <>
          <datalist id="ks-printers">{page.printers.map((p) => <option key={p} value={p} />)}</datalist>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Receipt printer" hint={page.printers.length ? `${page.printers.length} found on this computer` : 'None found on this computer'}>
              <input className={INPUT} list="ks-printers" value={v.receiptPrinter} placeholder="System default" autoComplete="off" onChange={(e) => set('receiptPrinter', e.target.value)} />
            </Field>
            <Field label="Label printer">
              <input className={INPUT} list="ks-printers" value={v.labelPrinter} placeholder="System default" autoComplete="off" onChange={(e) => set('labelPrinter', e.target.value)} />
            </Field>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" className="h-10 gap-1.5 px-4" disabled={testing || !s.receiptPrinter} onClick={() => void test()}><Printer size={16} /> Print a test slip</Button>
            <small className="text-[12px] text-faint">Uses the saved receipt printer and opens the drawer.</small>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <Field label="Width mm"><input className={INPUT} inputMode="numeric" value={v.labelWidthMm} onChange={(e) => set('labelWidthMm', int(e.target.value))} /></Field>
            <Field label="Height mm"><input className={INPUT} inputMode="numeric" value={v.labelHeightMm} onChange={(e) => set('labelHeightMm', int(e.target.value))} /></Field>
            <Field label="Gap mm"><input className={INPUT} inputMode="numeric" value={v.labelGapMm} onChange={(e) => set('labelGapMm', int(e.target.value))} /></Field>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Segmented label="Top text lines" value={v.labelNameLines} onChange={(x) => set('labelNameLines', x)} options={[1, 2, 3, 4].map((n) => ({ value: n, label: String(n) }))} />
            <Segmented label="Bottom text lines" value={v.labelBottomLines} onChange={(x) => set('labelBottomLines', x)} options={[1, 2, 3, 4].map((n) => ({ value: n, label: String(n) }))} />
            <Segmented label="Top text size" value={v.labelNameFont} onChange={(x) => set('labelNameFont', x)} options={FONT} />
            <Segmented label="Bottom text size" value={v.labelBottomFont} onChange={(x) => set('labelBottomFont', x)} options={FONT} />
          </div>
        </>
      )}
    </Section>
  )
}
