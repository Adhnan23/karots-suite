const $ = (id) => document.getElementById(id)
const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c])
const MASK = '••••'
const OS = [['linux', 'Linux'], ['windows', 'Windows'], ['darwin', 'macOS'], ['freebsd', 'FreeBSD']]
const SHOP_OS = OS.slice(0, 2)
const ARCH = [['amd64', 'Intel / AMD'], ['arm64', 'ARM']]
const MODE = [['file', 'One PC (file mode)'], ['server', 'Server (PostgreSQL)']]
const OPEN = [['kiosk', 'Kiosk — till full-screen'], ['background', 'Background — owner\'s PC']]
const s = { picked: new Set(), os: 'linux', arch: 'amd64', mode: 'server', plugins: [] }
const n = { os: 'linux', arch: 'amd64', mode: 'file' }

// seg draws a segmented control over obj[key].
function seg(el, opts, obj, key, onChange) {
  el.innerHTML = ''
  for (const [v, label] of opts) {
    const b = document.createElement('button')
    b.type = 'button'; b.textContent = label; b.setAttribute('aria-pressed', obj[key] === v)
    b.onclick = () => { if (onChange && onChange(v) === false) return; obj[key] = v; seg(el, opts, obj, key, onChange); summary() }
    el.append(b)
  }
}
// pluginGrid draws the plugin cards over a Set of picked keys.
function pluginGrid(el, picked, onChange) {
  el.innerHTML = ''
  if (!s.plugins.length) { el.innerHTML = '<div class="empty">No plugins under plugins/*/plugin.json yet — this builds the core.</div>'; return }
  for (const p of s.plugins) {
    const b = document.createElement('button')
    b.type = 'button'; b.className = 'plugin'; b.setAttribute('aria-pressed', picked.has(p.key))
    b.innerHTML = '<span class="tick">✓</span><span class="ver"></span><b></b><small></small>'
    b.querySelector('b').textContent = p.name
    b.querySelector('small').textContent = p.description
    b.querySelector('.ver').textContent = p.version ? 'v' + p.version : ''
    b.onclick = () => { picked.has(p.key) ? picked.delete(p.key) : picked.add(p.key); pluginGrid(el, picked, onChange); onChange && onChange() }
    el.append(b)
  }
}
const plugins = () => pluginGrid($('plugins'), s.picked, summary)

function view(name) {
  for (const v of ['vShops', 'vNew', 'vEdit', 'vQuick', 'vDemo']) $(v).hidden = v !== name
  $('tShops').setAttribute('aria-pressed', name !== 'vQuick' && name !== 'vDemo')
  $('tQuick').setAttribute('aria-pressed', name === 'vQuick')
  $('tDemo').setAttribute('aria-pressed', name === 'vDemo')
  $('log').classList.remove('on')
}

// ---- Quick build (no shop, nothing saved) ----
function valid() {
  const name = $('name').value.trim(), iid = $('iid').value.trim().toUpperCase()
  const nameOk = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(name)
  const iidOk = iid === '' || /^[0-9A-Z]{4,32}$/.test(iid)
  $('nameHint').textContent = nameOk ? '' : 'Letters, digits, dot, dash or underscore.'
  $('nameHint').className = 'hint' + (nameOk ? '' : ' bad')
  $('iidHint').className = 'hint' + (iidOk ? '' : ' bad')
  $('iidHint').textContent = iidOk ? (iid ? 'Rebuilding for shop ' + iid + ' — its support PIN stays the same.' : 'Empty mints a new one. Paste a shop\'s id to keep its support PIN.') : '4–32 letters or digits, e.g. 3FA2C91B.'
  return nameOk && iidOk
}
function fileOk(o, a) { return o === 'windows' ? a === 'amd64' : o === 'linux' || o === 'darwin' }
function summary() {
  const k = s.picked.size, osName = OS.find((o) => o[0] === s.os)[1]
  const file = $('name').value.trim() + (s.os === 'windows' ? '.exe' : '')
  const modeOk = s.mode === 'server' || fileOk(s.os, s.arch)
  $('modeHint').className = 'hint' + (modeOk ? '' : ' bad')
  $('modeHint').textContent = !modeOk ? 'File mode has no PostgreSQL for this system — pick Server, or Linux / macOS / Windows on Intel / AMD.'
    : s.mode === 'file' ? 'Runs its own PostgreSQL from a data folder next to the program — nothing to install (+15 MB). Move to a server later by backup → restore.'
    : 'Needs a PostgreSQL server and DATABASE_URL in the .env.'
  $('summary').textContent = `dist/${file} · ${osName} · ${s.mode === 'file' ? 'one PC' : 'server'} · ${k ? k + ' plugin' + (k > 1 ? 's' : '') : 'core only'}`
  $('go').disabled = !valid() || !modeOk || s.building
}
function quickBuild() {
  if (!valid()) return
  stream('/api/build', { plugins: s.plugins.filter((p) => s.picked.has(p.key)).map((p) => p.key), os: s.os, arch: s.arch, mode: s.mode, name: $('name').value.trim(), installId: $('iid').value.trim() }, $('go'))
}

// stream POSTs a build and shows its live log, then the outcome.
async function stream(url, body, btn, after) {
  s.building = true; summary()
  const label = btn.textContent
  btn.disabled = true; btn.innerHTML = '<span class="spin"></span>Building…'
  $('log').classList.add('on'); $('out').textContent = ''; yearsReset(); $('done').className = 'done'
  $('log').scrollIntoView({ behavior: 'smooth' })
  let text = '', r = null, err = null
  try {
    const res = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    if (!res.ok) err = (await res.json()).error
    else {
      const reader = res.body.getReader(), dec = new TextDecoder()
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        text += dec.decode(value, { stream: true })
        years(text)
        $('out').textContent = text.replace(/^[◷▦] .*\n?/gm, '').replace(/\n@@(RESULT|ERROR) .*\n?$/, ''); $('out').scrollTop = 1e9
      }
      const m = text.match(/@@RESULT (.*)/), e = text.match(/@@ERROR (.*)/)
      r = m ? JSON.parse(m[1]) : null
      err = e ? e[1] : r ? null : 'The build stopped without a result — see the log.'
    }
  } catch (x) { err = String(x) }
  s.building = false; btn.disabled = false; btn.textContent = label; summary()
  finish(r, err)
  after && after()
}
// Each demo shop's time machine, fed from the build log's lines as they stream:
// "◷ <shop> · <pct>% · <step>", "▦ <shop>\t<day JSON>", "✓ <shop>: a year built in <t>".
let timeMachine = null
import('/timemachine.js').then((m) => { timeMachine = m.timeMachine })
const tm = { seen: 0, shops: new Map() }
function yearsReset() {
  for (const r of tm.shops.values()) r.tm?.stop()
  tm.seen = 0; tm.shops.clear(); $('years').replaceChildren()
}
function years(text) {
  const end = text.lastIndexOf('\n') + 1
  for (const line of text.slice(tm.seen, end).split('\n')) {
    let m
    if ((m = line.match(/^◷ (.+?) · (\d+)% · (.*)$/))) yearRow(m[1]).step.textContent = `${m[3]} · ${m[2]}%`
    else if ((m = line.match(/^▦ (.+?)\t(.*)$/))) yearRow(m[1]).tm?.add(JSON.parse(m[2]))
    else if ((m = line.match(/^✓ (.+?): a year built in (.*)$/))) {
      const r = yearRow(m[1]); r.step.textContent = 'Built in ' + m[2]; r.tm?.done('✓ a year in ' + m[2])
    }
  }
  tm.seen = end
}
function yearRow(shop) {
  let r = tm.shops.get(shop)
  if (r) return r
  const row = document.createElement('div'); row.className = 'year'
  const b = document.createElement('b'); b.textContent = shop
  const step = document.createElement('span')
  row.append(b, step)
  r = { step, tm: timeMachine ? timeMachine() : null }
  if (r.tm) row.append(r.tm.el)
  $('years').append(row); tm.shops.set(shop, r)
  return r
}
function finish(r, err) {
  const d = $('done'); d.className = 'done on' + (err ? ' fail' : ''); d.innerHTML = ''
  const h = document.createElement('b'); h.textContent = err ? (/^built .*not published/.test(err) ? 'Built — not published' : 'Failed') : '✓ Done'; d.append(h)
  const kv = document.createElement('div'); kv.className = 'kv'; d.append(kv)
  const add = (k, v, cls) => { const a = document.createElement('span'); a.textContent = k; const b = document.createElement('code'); b.textContent = v; if (cls) b.className = cls; kv.append(a, b) }
  if (err) { add('Why', err); return }
  add(r.folder ? 'Folder' : 'Binary', r.folder || r.binary)
  if (r.unlock) { add('Install id', r.installId); add('Unlock code', r.unlock, 'pin'); add('', 'It rotates hourly — your PIN app gives it for this install id.'); return }
  if (r.installId) { add('Install id', r.installId); add('Support login', '0000000001'); add('Support PIN', r.supportPin, 'pin'); add('', 'The PIN rotates hourly — recompute it from the install id.') }
  else add('Support', 'Shared support PIN (no POS_SUPPORT_SECRET)')
}

// ---- Shops ----
async function api(url, body) {
  const res = await fetch(url, body === undefined ? {} : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  const ct = res.headers.get('Content-Type') || ''
  const data = ct.includes('json') ? await res.json() : await res.blob()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data
}
async function loadShops() {
  const st = await api('/api/shops')
  $('shopsPath').textContent = `Saved in ${st.path} — commit it to the private repo (it holds every shop's secrets).`
  const el = $('shops'); el.innerHTML = ''
  if (!st.shops.length) { el.innerHTML = '<div class="empty">No shops yet — + New shop, or bring karots-pos shops over with <code>go run ./cmd/bootstrap -import-pos ../karots-pos/shops.json</code>.</div>'; return }
  for (const sh of st.shops) {
    const row = document.createElement('div'); row.className = 'shop'
    const lb = sh.lastBuilt, pub = sh.published
    const last = lb ? (lb.kind === 'test' ? 'test build ' : 'built ') + new Date(lb.at).toLocaleString() + (lb.commit ? ' · ' + lb.commit : '') : 'never built'
    const unpublished = lb && lb.kind === 'release' && (!pub || pub.version !== lb.version)
    const state = (pub ? `<span class="chip ok">Published ${esc(pub.version)} · ${esc(new Date(pub.at).toLocaleDateString())}</span>` : '<span class="chip bad">Not published</span>')
      + (unpublished && pub ? `<span class="chip bad">${esc(lb.version)} not published</span>` : '')
      + (lb && lb.kind === 'test' && (!pub || lb.version > pub.version)
        // same commit, nothing uncommitted = the published code: not a difference
        ? (pub && lb.commit === pub.commit && !lb.dirty
          ? '<span class="chip">Test build of the published code</span>'
          : `<span class="chip warn">Test build ${esc(lb.version)} — ${lb.dirty ? 'uncommitted changes' : 'newer code'}, not released</span>`)
        : '')
    const warns = (st.warnings[sh.key] || []).map((w) => `<span class="chip warn">⚠ ${esc(w)}</span>`).join('')
    row.innerHTML = `<div><b>${esc(sh.name)}</b> <span class="hint">${esc(sh.key)}</span>
      <div class="chips"><span class="chip">${esc(sh.os)}/${esc(sh.arch)}</span><span class="chip">${sh.mode === 'file' ? 'one PC' : 'server'}</span>
      <span class="chip">:${sh.port}</span><span class="chip">${sh.plugins.length ? sh.plugins.length + ' plugins' : 'core only'}</span>
      <span class="chip">${sh.install.open}</span>${warns}</div><div class="chips">${state}</div><div class="hint">${esc(last)}</div></div>
      <div class="acts"><button class="btn primary" data-a="release">Release…</button>${unpublished ? '<button class="btn" data-a="publish">Publish</button>' : ''}<button class="btn" data-a="build">Test build</button><button class="btn" data-a="edit">Edit</button>${sh.cloudKey ? '<button class="btn" data-a="cloud">Cloud backups</button>' : ''}
      <button class="btn" data-a="env">.env</button><button class="btn danger" data-a="del">Delete</button></div>`
    row.querySelector('[data-a=build]').onclick = (e) => stream(`/api/shops/${sh.key}/build`, { installer: $('installer').checked, postgres: $('postgres').checked }, e.target, loadShops)
    row.querySelector('[data-a=release]').onclick = (e) => release(sh, e.target)
    if (unpublished) row.querySelector('[data-a=publish]').onclick = (e) => stream(`/api/shops/${sh.key}/publish`, {}, e.target, loadShops)
    row.querySelector('[data-a=edit]').onclick = () => edit(sh)
    if (sh.cloudKey) row.querySelector('[data-a=cloud]').onclick = () => cloudBackups(sh)
    row.querySelector('[data-a=env]').onclick = () => downloadEnv(sh.key)
    row.querySelector('[data-a=del]').onclick = () => del(sh)
    el.append(row)
  }
}
// release opens the notes dialog (pre-filled with the fixes since the shop's
// last release); sh = null releases every shop with one note.
async function release(sh, btn) {
  $('relTitle').textContent = sh ? 'Release ' + sh.name : 'Release every shop'
  $('relNotes').value = ''
  if (sh) try { $('relNotes').value = (await api(`/api/shops/${sh.key}/notes`)).notes } catch {}
  $('rel').showModal(); $('relNotes').focus()
  $('relGo').onclick = () => {
    $('rel').close()
    const body = { installer: $('installer').checked, postgres: $('postgres').checked, notes: $('relNotes').value }
    stream(sh ? `/api/shops/${sh.key}/release` : '/api/shops/release-all', body, btn, loadShops)
  }
}
// cloudBackups lists a shop's Karots cloud copies; Download opens one into dist/<key>/.
async function cloudBackups(sh) {
  $('cbTitle').textContent = 'Karots cloud — ' + sh.name
  const el = $('cbList'); el.textContent = 'Looking…'
  $('cb').showModal()
  try {
    const list = await api(`/api/shops/${sh.key}/cloud-backups`, {})
    el.innerHTML = list.length ? '' : '<div class="empty">No cloud backups yet — the shop sends one about once a day.</div>'
    for (const b of list) {
      const row = document.createElement('div'); row.className = 'envrow'
      row.innerHTML = `<div><b>${esc(new Date(b.modifiedAt).toLocaleString())}</b> <span class="hint">${(b.size / 1048576).toFixed(1)} MB</span></div><span></span><button class="btn" type="button">Download</button>`
      row.querySelector('button').onclick = async (e) => {
        e.target.disabled = true
        try {
          const r = await api(`/api/shops/${sh.key}/cloud-backups/fetch`, { name: b.name })
          e.target.textContent = 'Saved'; alert('Saved to ' + r.path)
        } catch (err) { alert(err.message) } finally { e.target.disabled = false }
      }
      el.append(row)
    }
  } catch (e) { el.innerHTML = `<div class="err">${esc(e.message)}</div>` }
}
async function downloadEnv(key) {
  try {
    const blob = await api(`/api/shops/${key}/env`, {})
    const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = key + '.env'; a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) { alert(e.message) }
}
async function del(sh) {
  const typed = prompt(`Delete ${sh.name}? Its install id (${sh.installId}) and update channel go with it — a shop already installed keeps working, but you can't rebuild it with the same support PIN.\n\nType ${sh.key} to delete:`)
  if (typed === null) return
  try { await api(`/api/shops/${sh.key}/delete`, { confirm: typed.trim() }); loadShops() } catch (e) { alert(e.message) }
}
function newShop() {
  Object.assign(n, { os: 'linux', arch: 'amd64', mode: 'file' })
  $('nName').value = ''; $('nKey').value = ''; $('nErr').textContent = ''; delete $('nKey').dataset.touched
  seg($('nOS'), SHOP_OS, n, 'os'); seg($('nArch'), ARCH, n, 'arch'); seg($('nMode'), MODE, n, 'mode')
  view('vNew'); $('nName').focus()
}
async function createShop() {
  try {
    const sh = await api('/api/shops', { key: $('nKey').value.trim(), name: $('nName').value.trim(), os: n.os, arch: n.arch, mode: n.mode })
    edit(sh)
  } catch (e) { $('nErr').textContent = e.message }
}
const keyOf = (name) => name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)

// ---- Edit a shop ----
let cur = null // the shop being edited (masked secrets until Show / Rotate)
function edit(sh) {
  cur = JSON.parse(JSON.stringify(sh))
  cur.picked = new Set(cur.plugins)
  cur.env = cur.env || {}
  renderEdit()
  view('vEdit')
}
function field(id, label, value, attrs = '') {
  return `<div><label class="f" for="${id}">${label}</label><input id="${id}" value="${esc(value)}" autocomplete="off" spellcheck="false" ${attrs}></div>`
}
function secret(id, label, value, what, hint = '') {
  return `<div><label class="f" for="${id}">${label}</label><div class="secret"><input id="${id}" value="${esc(value)}" autocomplete="off" spellcheck="false">
    <button class="btn" type="button" data-rotate="${what}">Rotate</button></div>${hint ? `<div class="hint">${hint}</div>` : ''}</div>`
}
function renderEdit() {
  const c = cur, server = c.mode === 'server'
  const db = c.db || { host: 'localhost', port: 5432, name: 'suite_db', user: 'suite_user', password: '', superPassword: '' }
  const last = c.lastBuilt ? new Date(c.lastBuilt.at).toLocaleString() + ' · ' + (c.lastBuilt.commit || '') : 'never'
  $('vEdit').innerHTML = `
    <div class="row"><h2>Edit shop</h2><span class="hint">${esc(c.key)}</span></div>
    <div class="fields">
      <div><label class="f">Install id</label><div class="ro">${esc(c.installId)}</div></div>
      <div><label class="f">Update channel</label><div class="ro">${esc(c.channel)}</div></div>
      <div><label class="f">Last built</label><div class="ro">${esc(last)}</div></div>
    </div>
    <div class="sec"><h2>Shop</h2>
      <div class="fields">${field('eName', 'Shop name', c.name)}</div>
      <label class="f">Runs on</label><div class="seg" id="eOS"></div>
      <label class="f">Processor</label><div class="seg" id="eArch"></div>
      <label class="f">Plugins</label><div class="plugins" id="ePlugins" style="margin-top:6px"></div>
    </div>
    <div class="sec"><h2>Database</h2>
      <div class="seg" id="eMode"></div>
      ${server ? `<div class="fields">
        ${field('eDbHost', 'Database host', db.host)}${field('eDbPort', 'Database port', db.port, 'inputmode="numeric"')}
        ${field('eDbName', 'Database name', db.name)}${field('eDbUser', 'Database user', db.user)}
        <div><label class="f" for="eDbSsl">SSL</label><select id="eDbSsl" class="sel">${[['', 'Auto — off on this PC, on elsewhere'], ['disable', 'Off'], ['require', 'Required'], ['verify-full', 'Required + check certificate']].map(([v, l]) => `<option value="${v}" ${(db.ssl || '') === v ? 'selected' : ''}>${l}</option>`).join('')}</select></div>
        ${secret('eDbPass', 'Database password', db.password || '(made when you save)', 'db')}
        ${secret('eDbSuper', 'PostgreSQL superuser password', db.superPassword || '(made when you save)', 'dbsuper', 'Windows: Setup installs PostgreSQL with it.')}
        ${secret('eJwt', 'JWT secret', c.jwtSecret || '(made when you save)', 'jwt', 'A new one signs everyone out.')}
      </div><div class="hint">Host localhost = PostgreSQL on the shop PC (Setup installs it); any other host = a LAN server, a VPS or a hosted database (Neon: use the direct host, not "-pooler") that already has it.</div>`
      : `<div class="fields">${field('eFilePort', 'File database port', c.fileDbPort || 5434, 'inputmode="numeric"')}</div>
         <div class="hint">The shop's data stays on its one PC (data/ beside the program); its JWT secret is made there.</div>`}
    </div>
    <div class="sec"><h2>Network</h2>
      <div class="fields">${field('ePort', 'Till port', c.port, 'inputmode="numeric"')}${field('eLan', 'Main PC address on the LAN (optional)', c.install.lanAddress || '', 'placeholder="192.168.1.10"')}</div>
      <div class="hint">The address fills in cashier PCs' Setup. Port 3000 is karots-pos's too — the suite replaces it on a PC, not beside it.</div>
    </div>
    <div class="sec"><h2>Install</h2>
      <div class="fields">${field('eDir', 'Install folder', c.install.dir)}</div>
      <label class="f">When the PC starts</label><div class="seg" id="eOpen"></div>
    </div>
    <div class="sec"><h2>Backups</h2>
      <div class="fields">
        ${field('eBkDir', 'Backup folder', c.backups?.dir || '', `placeholder="${esc(server ? c.install.dir + (c.os === 'windows' ? '\\' : '/') + 'backups' : 'data/backups beside Karots')}"`)}
        <div><label class="f" for="eBkEvery">Back up every</label><select id="eBkEvery" class="sel">${['1h', '3h', '6h', '12h', '24h'].map((v) => `<option value="${v}" ${(c.backups?.every || '6h') === v ? 'selected' : ''}>${v.replace('h', ' hours').replace(/^1 hours$/, '1 hour')}</option>`).join('')}</select></div>
        ${field('eBkKeep', 'Keep the newest', c.backups?.keep || 28, 'inputmode="numeric"')}
      </div>
      <div class="fields">${field('eBkCopy', 'Also copy backups to (optional)', c.backups?.copyDir || '', `placeholder="${c.os === 'windows' ? 'E:\\Karots copies' : '/media/usb/Karots copies'}"`)}</div>
      <label class="check" style="margin-top:8px"><input type="checkbox" id="eBkCloud" ${c.backups?.cloud ? 'checked' : ''}> Karots cloud backup (paid) — a locked copy about once a day, kept 30 days</label>
      <div class="hint">Empty folder = the default shown. The shop can change the folders in Setup and on its Data page; a USB stick or network folder catches up when it is plugged back in. Karots cloud needs CLOUD_BACKUP_* in this .env.</div>
    </div>
    <div class="sec"><h2>More settings</h2>
      <div id="eEnv"></div><button class="btn" type="button" id="eEnvAdd" style="margin-top:8px">+ Add setting</button>
      <div class="hint">Extra .env lines (e.g. JWT_EXPIRES_IN=10h). Port, database, secret and backups come from the fields above.</div>
    </div>
    <div class="err" id="eErr"></div>
    <div class="bar"><button class="btn" id="eShow">Show secrets</button><button class="btn" data-back>Cancel</button><button class="btn primary" id="eSave">Save</button></div>`
  seg($('eOS'), SHOP_OS, c, 'os', (v) => {
    read(); c.os = v
    const dirs = { linux: '/opt/karots-suite', windows: 'C:\\karots-suite' }
    if (Object.values(dirs).includes(c.install.dir)) c.install.dir = dirs[v]
    renderEdit(); return false
  })
  seg($('eArch'), ARCH, c, 'arch')
  seg($('eMode'), MODE, c, 'mode', (v) => {
    if (v === c.mode) return
    if (v === 'file' && !confirm('Switch this shop to one PC (file mode)?\n\nIf it already runs on a server, move the data first: Setup → Data → "Where the shop is kept" has the steps.')) return false
    read(); c.mode = v; renderEdit(); return false
  })
  seg($('eOpen'), OPEN, c.install, 'open')
  pluginGrid($('ePlugins'), c.picked)
  envRows()
  $('eEnvAdd').onclick = () => { read(); c.envList.push(['', '']); envRows(true) }
  $('eShow').onclick = reveal
  $('eSave').onclick = save
  for (const b of $('vEdit').querySelectorAll('[data-rotate]')) b.onclick = () => rotate(b.dataset.rotate)
  for (const b of $('vEdit').querySelectorAll('[data-back]')) b.onclick = backToShops
}
function envRows(keepList) {
  if (!keepList) cur.envList = Object.entries(cur.env)
  const el = $('eEnv'); el.innerHTML = ''
  cur.envList.forEach(([k, v], i) => {
    const row = document.createElement('div'); row.className = 'envrow'
    row.innerHTML = `<input placeholder="KEY" value="${esc(k)}" spellcheck="false"><input placeholder="value" value="${esc(v)}" spellcheck="false"><button class="btn" type="button">✕</button>`
    row.querySelector('button').onclick = () => { read(); cur.envList.splice(i, 1); envRows(true) }
    el.append(row)
  })
}
// read copies the form into cur (before a re-render or a save).
function read() {
  const c = cur, v = (id) => $(id) ? $(id).value.trim() : undefined, num = (id) => parseInt(v(id), 10) || 0
  c.name = v('eName'); c.port = num('ePort'); c.install.dir = v('eDir'); c.install.lanAddress = v('eLan')
  c.plugins = s.plugins.map((p) => p.key).filter((k) => c.picked.has(k))
  if ($('eDbHost')) {
    const keep = (id) => (v(id).startsWith('(') ? '' : v(id))
    c.db = { host: v('eDbHost'), port: num('eDbPort'), name: v('eDbName'), user: v('eDbUser'), password: keep('eDbPass'), superPassword: keep('eDbSuper'), ssl: $('eDbSsl').value }
    c.jwtSecret = keep('eJwt')
  }
  if ($('eFilePort')) c.fileDbPort = num('eFilePort')
  c.backups = { dir: v('eBkDir'), every: $('eBkEvery').value, keep: num('eBkKeep'), copyDir: v('eBkCopy'), cloud: $('eBkCloud').checked }
  c.envList = [...$('eEnv').querySelectorAll('.envrow')].map((r) => [...r.querySelectorAll('input')].map((i) => i.value.trim()))
  c.env = Object.fromEntries(c.envList.filter(([k]) => k))
}
async function save() {
  read()
  const { picked, envList, ...body } = cur
  try {
    await api(`/api/shops/${cur.key}`, body)
    backToShops()
  } catch (e) { $('eErr').textContent = e.message }
}
async function reveal() {
  try {
    const sh = await api(`/api/shops/${cur.key}/reveal`, {})
    read()
    if (sh.db && cur.db) { cur.db.password = sh.db.password; cur.db.superPassword = sh.db.superPassword }
    if (sh.jwtSecret) cur.jwtSecret = sh.jwtSecret
    for (const [k, val] of Object.entries(sh.env || {})) if (cur.env[k] === MASK) cur.env[k] = val
    renderEdit()
  } catch (e) { $('eErr').textContent = e.message }
}
async function rotate(what) {
  const why = { jwt: 'A new JWT secret signs everyone out at the shop after the next Setup → Update.', db: 'The next Setup → Update at the shop sets the new database password.', dbsuper: 'Only for a PostgreSQL Setup hasn\'t installed yet.' }[what]
  if (!confirm(`Rotate this secret?\n\n${why}`)) return
  try {
    const sh = await api(`/api/shops/${cur.key}/rotate`, { what })
    read()
    if (what === 'jwt') cur.jwtSecret = sh.jwtSecret
    if (what === 'db') cur.db.password = sh.db.password
    if (what === 'dbsuper') cur.db.superPassword = sh.db.superPassword
    renderEdit()
  } catch (e) { $('eErr').textContent = e.message }
}
function backToShops() { view('vShops'); loadShops() }

// ---- Start ----
fetch('/api/state').then((r) => r.json()).then((st) => {
  Object.assign(s, { plugins: st.plugins, os: st.os, arch: st.arch })
  $('name').value = st.name; $('secret').hidden = st.hasSecret; $('secret2').hidden = st.hasSecret; $('secret3').hidden = st.hasSecret
  seg($('os'), OS, s, 'os'); seg($('arch'), ARCH, s, 'arch'); seg($('mode'), MODE, s, 'mode'); plugins(); summary()
  loadShops()
})
$('name').oninput = summary; $('iid').oninput = summary; $('go').onclick = quickBuild
$('all').onclick = () => { s.plugins.forEach((p) => s.picked.add(p.key)); plugins(); summary() }
$('none').onclick = () => { s.picked.clear(); plugins(); summary() }
$('tShops').onclick = backToShops
$('tQuick').onclick = () => view('vQuick')
$('tDemo').onclick = () => view('vDemo')
$('demoGo').onclick = (e) => stream('/api/demo', { fresh: $('demoFresh').checked }, e.target)
$('newShop').onclick = newShop
$('releaseAll').onclick = (e) => release(null, e.target)
$('relCancel').onclick = () => $('rel').close()
$('cbClose').onclick = () => $('cb').close()
$('buildAll').onclick = (e) => {
  if (confirm('Test build every shop now, one after another' + ($('installer').checked ? ' (with the installer)' : '') + '? Nothing is uploaded. A shop that fails is listed at the end; the rest still build.'))
    stream('/api/shops/build-all', { installer: $('installer').checked, postgres: $('postgres').checked }, e.target, loadShops)
}
$('nCreate').onclick = createShop
$('nName').oninput = () => { if (!$('nKey').dataset.touched) $('nKey').value = keyOf($('nName').value) }
$('nKey').oninput = () => { $('nKey').dataset.touched = '1' }
for (const b of document.querySelectorAll('#vNew [data-back]')) b.onclick = backToShops
