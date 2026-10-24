'use strict';
// Front end of the Wails build: plain JavaScript, no build step. It talks to the Go service
// through the bound methods (window.go.<package>.App.*), i.e. in-process RPC.

// ---- bridge -------------------------------------------------------------------
function bridge() {
  const go = window.go || {};
  for (const pkg of Object.keys(go)) if (go[pkg].App) return go[pkg].App;
  return null;
}
const sleep = ms => new Promise(r => setTimeout(r, ms));
async function ready() { for (let i = 0; i < 200 && !bridge(); i++) await sleep(25); return bridge(); }
const $ = id => document.getElementById(id);

// ---- i18n -----------------------------------------------------------------------
const LANGS = [['ko', '한국어'], ['zh-Hans', '简体中文'], ['zh-Hant', '繁體中文'], ['en', 'English']];
const packs = {};
let lang = 'ko';
function t(key, ...args) {
  let s = (packs[lang] && packs[lang][key]) ?? (packs.en && packs.en[key]) ?? key;
  s = s.replace(/\{(\d+)\}/g, (_, i) => args[+i] ?? '');
  return /^(menu|btn|chk)_/.test(key) ? s.replace(/_(?=\w)/g, '') : s; // drop the access-key underscore of the shared packs
}
async function loadPacks() {
  await Promise.all(LANGS.map(async ([tag]) => { packs[tag] = await (await fetch(`lang/${tag}.json`)).json(); }));
}
function applyI18n() {
  document.documentElement.lang = lang;
  document.querySelectorAll('[data-i18n]').forEach(e => { e.textContent = t(e.dataset.i18n); });
  document.querySelectorAll('[data-i18n-ph]').forEach(e => { e.placeholder = t(e.dataset.i18nPh); });
  buildLangMenu(); buildFontMenu(); renderTable(); renderStatus(); renderSource(); renderTargets();
}

// ---- state -------------------------------------------------------------------------
const S = { machines: [], rows: [], selected: null, targets: { single: [], group: [] }, readOnly: true,
  repoPath: '', status: ['st_ready', []], lockOpen: false, busy: false, fontSize: 14 };
function setStatus(key, ...args) { S.status = [key, args]; renderStatus(); }
function renderStatus() {
  $('st-text').textContent = t(S.status[0], ...S.status[1]);
  $('st-path').textContent = S.repoPath;
  const q = $('search').value.trim();
  const n = S.rows.length, shown = filtered().length;
  $('st-count').textContent = q ? t('count_filtered', shown, n) : t('count_models', n);
  $('st-lock').textContent = S.readOnly ? t(S.lockOpen ? 'lock_open' : 'lock_locked') : t('lock_writable');
}
const src = () => ({ machine: $('machine').value, node: $('node').value });
function renderSource() { $('source-label').textContent = t('lbl_source_is', `${src().machine} / ${src().node}`); }

// ---- menus, caption, tabs -------------------------------------------------------------
function closeMenus() { document.querySelectorAll('.w2k-menu.is-open').forEach(m => m.classList.remove('is-open')); }
document.querySelectorAll('[data-menu]').forEach(item => item.addEventListener('click', e => {
  e.stopPropagation();
  const m = $('menu-' + item.dataset.menu), open = m.classList.contains('is-open');
  closeMenus(); if (!open) m.classList.add('is-open');
}));
document.addEventListener('click', closeMenus);
document.addEventListener('keydown', e => { if (e.key === 'F5') { e.preventDefault(); reload(); } if (e.key === 'Escape') { closeMenus(); } });
function buildLangMenu() {
  const box = $('lang-items'); box.textContent = '';
  for (const [tag, name] of LANGS) {
    const d = document.createElement('div'); d.className = 'w2k-menu-item' + (tag === lang ? ' is-on' : '');
    d.textContent = name; d.dataset.lang = tag; d.addEventListener('click', () => setLang(tag)); box.appendChild(d);
  }
}
function buildFontMenu() {
  const box = $('font-items'); box.textContent = '';
  for (const size of [12, 13, 14, 16, 18, 20]) {
    const d = document.createElement('div'); d.className = 'w2k-menu-item' + (size === S.fontSize ? ' is-on' : '');
    d.textContent = size === 14 ? t('font_default', size) : String(size); d.addEventListener('click', () => setFont(size)); box.appendChild(d);
  }
}
function setLang(tag) { lang = tag; try { localStorage.setItem('lang', tag); } catch (e) {} applyI18n(); }
function setFont(px) {
  S.fontSize = px; document.documentElement.style.setProperty('--w2k-font-size', px + 'px');
  try { localStorage.setItem('font', String(px)); } catch (e) {} buildFontMenu();
}
function showTab(name) {
  $('page-copy').hidden = name !== 'copy'; $('page-export').hidden = name !== 'export';
  $('tab-copy').setAttribute('aria-selected', name === 'copy'); $('tab-export').setAttribute('aria-selected', name === 'export');
}
$('tab-copy').onclick = () => showTab('copy'); $('tab-export').onclick = () => showTab('export');
$('cap-min').onclick = () => window.runtime && window.runtime.WindowMinimise();
$('cap-max').onclick = () => window.runtime && window.runtime.WindowToggleMaximise();
$('cap-close').onclick = () => bridge().CloseWindow();
$('caption').addEventListener('dblclick', () => window.runtime && window.runtime.WindowToggleMaximise());
$('mi-exit').onclick = () => bridge().CloseWindow();
$('mi-refresh').onclick = () => reload();
$('mi-about').onclick = () => message(t('about_title'), t('about_text', S.version || 'dev'));

// ---- models -----------------------------------------------------------------------------------
function filtered() {
  const q = $('search').value.trim().toLowerCase();
  return S.rows.filter(r => !q || r.name.toLowerCase().includes(q));
}
function fmtTime(s) {
  if (!s) return '';
  const d = new Date(s), p = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function renderTable() {
  const body = $('models').tBodies[0]; body.textContent = '';
  const yes = t('yes_mark'), no = t('no_mark');
  for (const r of filtered()) {
    const tr = document.createElement('tr'); tr.dataset.name = r.name;
    if (S.selected === r.name) tr.className = 'is-selected';
    for (const v of [r.name, r.class, r.repo ? yes : no, r.history ? yes : no, r.inspect ? String(r.inspect) : no, r.light ? yes : no, fmtTime(r.modTime)]) {
      const td = document.createElement('td'); td.textContent = v; tr.appendChild(td);
    }
    tr.addEventListener('click', () => { S.selected = r.name; renderTable(); updateButtons(); });
    body.appendChild(tr);
  }
  renderStatus();
}
$('search').addEventListener('input', () => { if (S.selected && !filtered().some(r => r.name === S.selected)) S.selected = null; renderTable(); updateButtons(); });
function renderTargets() {
  const keepS = $('target-single').value, keepG = $('target-group').value;
  const ts = $('target-single'); ts.textContent = '';
  for (const r of S.targets.single) { const o = document.createElement('option'); o.value = `${r.machine}|${r.node}`; o.textContent = `${r.machine} / ${r.node}`; ts.appendChild(o); }
  const tg = $('target-group'); tg.textContent = '';
  for (const g of S.targets.group) { const o = document.createElement('option'); o.value = g; o.textContent = g; tg.appendChild(o); }
  if ([...ts.options].some(o => o.value === keepS)) ts.value = keepS; else {
    const pref = [...ts.options].find(o => o.value.startsWith(src().machine + '|') && !o.value.endsWith('|' + src().node)); if (pref) ts.value = pref.value;
  }
  if ([...tg.options].some(o => o.value === keepG)) tg.value = keepG;
  $('mode-single').disabled = !S.targets.single.length; $('mode-group').disabled = !S.targets.group.length;
  if (!S.targets.group.length) $('mode-single').checked = true;
  updateButtons();
}
function updateButtons() {
  const have = !!S.selected && !S.busy, group = $('mode-group').checked;
  $('btn-copy').disabled = !have || (group ? !$('target-group').value : !$('target-single').value);
  $('btn-rename').disabled = !have;
  $('target-single').disabled = group; $('target-group').disabled = !group;
}
document.querySelectorAll('input[name=copymode]').forEach(e => e.addEventListener('change', updateButtons));
async function reload(keepStatus) {
  const A = bridge(), { machine, node } = src(); if (!machine) return;
  const seq = reload.seq = (reload.seq || 0) + 1;
  if (!keepStatus) setStatus('st_reading', machine, node);
  try {
    const [list, targets] = await Promise.all([A.ListModels(lang, machine, node), A.Targets(lang, machine, node)]);
    if (seq !== reload.seq) return;
    S.rows = list.models || []; S.readOnly = list.readOnly; S.repoPath = list.repository; S.targets = { single: targets.single || [], group: targets.group || [] };
    if (S.selected && !S.rows.some(r => r.name === S.selected)) S.selected = null;
    if (!keepStatus) setStatus('st_read_ok', machine, node, S.rows.length);
    renderTable(); renderTargets(); renderSource();
  } catch (e) {
    if (seq !== reload.seq) return;
    S.rows = []; renderTable(); setStatus('st_read_fail', machine, node, String(e));
  }
}
$('machine').onchange = () => { S.selected = null; renderSource(); reload(); };
$('node').onchange = () => { S.selected = null; renderSource(); reload(); };
$('btn-refresh').onclick = () => reload();

// ---- dialogs ----------------------------------------------------------------------------------------
function dialog(title, bodyNodes, buttons) {
  return new Promise(resolve => {
    const ov = $('overlay'); ov.textContent = ''; ov.hidden = false;
    const w = document.createElement('div'); w.className = 'w2k-window dialog';
    const cap = document.createElement('div'); cap.className = 'w2k-caption'; cap.textContent = title; w.appendChild(cap);
    const body = document.createElement('div'); body.className = 'w2k-dialog-body'; bodyNodes.forEach(n => body.appendChild(n)); w.appendChild(body);
    const bar = document.createElement('div'); bar.className = 'w2k-dialog-buttons';
    const close = v => { ov.hidden = true; ov.textContent = ''; resolve(v); };
    for (const b of buttons) {
      if (b.sum) { const s = document.createElement('span'); s.className = 'sum'; s.textContent = b.sum; bar.appendChild(s); continue; }
      const el = document.createElement('button'); el.className = 'w2k-btn'; el.type = 'button'; el.textContent = b.text; el.disabled = !!b.disabled; el.id = b.id || '';
      el.onclick = () => close(b.value); bar.appendChild(el);
    }
    w.appendChild(bar); ov.appendChild(w);
    dialog.close = close;
  });
}
function div(text, cls) { const d = document.createElement('div'); if (cls) d.className = cls; d.textContent = text; return d; }
const message = (title, text) => dialog(title, text.split('\n').map(l => div(l)), [{ text: 'OK', value: true, id: 'dlg-ok' }]);
const confirmBox = (title, text) => dialog(title, text.split('\n').map(l => div(l)), [{ text: t('btn_run').replace('Run', 'Yes'), value: true }, { text: t('btn_cancel'), value: false }]);
function human(n) { const u = ['B', 'KB', 'MB', 'GB', 'TB']; let f = n, i = 0; while (f >= 1024 && i < 4) { f /= 1024; i++; } return i ? f.toFixed(1) + ' ' + u[i] : n + ' B'; }

async function confirmPlan(plan) {
  const nodes = [];
  if (plan.blockers.length || plan.warnings.length) {
    const g = document.createElement('fieldset'); g.className = 'w2k-groupbox'; const lg = document.createElement('legend'); lg.textContent = t('gb_notes'); g.appendChild(lg);
    plan.blockers.forEach(b => g.appendChild(div(t('plan_blocker', b), 'blocker')));
    plan.warnings.forEach(w => g.appendChild(div(t('plan_warning', w), 'warn')));
    nodes.push(g);
  }
  const g2 = document.createElement('fieldset'); g2.className = 'w2k-groupbox'; const lg2 = document.createElement('legend'); lg2.textContent = t('gb_steps'); g2.appendChild(lg2);
  const steps = document.createElement('div'); steps.className = 'steps'; steps.id = 'plan-steps';
  for (const u of plan.units) {
    steps.appendChild(div(`[${u.target.machine} / ${u.target.node}]  ${u.model}` + (u.newName !== u.model ? ` → ${u.newName}` : ''), 'h'));
    for (const s of u.steps) steps.appendChild(div(s.text, 'indent' + (s.danger ? ' danger' : '')));
  }
  g2.appendChild(steps); nodes.push(g2);
  const sum = plan.kind === 'copy' ? t('plan_summary_copy', plan.files, human(plan.bytes)) : t('plan_summary_rename', plan.units.length);
  return dialog(plan.title, nodes, [{ sum }, { text: t('btn_run'), value: true, id: 'plan-run', disabled: plan.blockers.length > 0 }, { text: t('btn_cancel'), value: false, id: 'plan-cancel' }]);
}

async function showJob(job) {
  const phase = div('', ''), bar = document.createElement('div'), fill = document.createElement('i'), counts = div('', 'w2k-note'), cur = div('', 'w2k-note'), log = div('', 'log'), res = div('', '');
  bar.className = 'bar'; bar.appendChild(fill); phase.id = 'job-phase'; res.id = 'job-result'; res.style.fontWeight = 'bold';
  const p = dialog(job.title, [phase, bar, counts, cur, log, res], [{ text: t('btn_close'), value: true, id: 'job-close', disabled: true }]);
  const closeBtn = () => document.getElementById('job-close');
  let j = job;
  for (;;) {
    const pr = j.progress, pct = pr.bytesTotal ? 100 * pr.bytesDone / pr.bytesTotal : pr.filesTotal ? 100 * pr.filesDone / pr.filesTotal : 0;
    phase.textContent = pr.phase; fill.style.right = (100 - Math.min(100, pct)) + '%'; fill.style.width = 'auto';
    counts.textContent = t('job_counts', pr.filesDone, pr.filesTotal, human(pr.bytesDone), human(pr.bytesTotal), pr.seconds.toFixed(1));
    cur.textContent = pr.current; log.textContent = (pr.log || []).join('\n'); log.style.whiteSpace = 'pre'; log.scrollTop = log.scrollHeight;
    if (j.state !== 'running') break;
    await sleep(150);
    try { j = await bridge().GetJob(job.id); } catch (e) { j = { ...j, state: 'failed', error: String(e) }; }
  }
  res.textContent = j.state === 'done' ? t('job_done') : (j.error || j.state);
  if (j.state !== 'done') res.className = 'danger';
  closeBtn().disabled = false;
  await p;
  return j;
}

async function runPlan(planFn) {
  S.busy = true; updateButtons();
  try {
    setStatus('st_planning');
    let plan;
    try { plan = await planFn(); } catch (e) { setStatus('st_cannot', String(e)); await message(t('dlg_cannot'), String(e)); return; }
    if (!await confirmPlan(plan)) { setStatus('st_canceled'); return; }
    let job;
    try { job = await bridge().StartJob(lang, plan.id); } catch (e) { await message(t('dlg_cannot_start'), String(e)); return; }
    setStatus('st_planning'); $('st-text').textContent = plan.title + ' ...';
    const done = await showJob(job);
    if (done.state === 'done') setStatus('st_done', plan.title); else if (done.state === 'canceled') setStatus('st_job_canceled', plan.title); else setStatus('st_failed', done.error || plan.title);
    await reload(true);
  } catch (e) { setStatus('st_error', String(e)); await message(t('dlg_error'), String(e)); }
  finally { S.busy = false; updateButtons(); }
}
$('btn-copy').onclick = () => {
  const s = src(), group = $('mode-group').checked, newName = $('copy-name').value.trim();
  runPlan(() => bridge().PlanCopy(lang, group
    ? { mode: 'group', model: S.selected, newName, source: { machine: s.machine, node: s.node }, target: { machine: $('target-group').value, node: s.node } }
    : { mode: 'single', model: S.selected, newName, source: { machine: s.machine, node: s.node },
        target: { machine: $('target-single').value.split('|')[0], node: $('target-single').value.split('|')[1] } }));
};
$('btn-rename').onclick = () => {
  const s = src();
  runPlan(() => bridge().PlanRename(lang, { mode: $('rename-group').checked ? 'group' : 'single', model: S.selected, newName: $('rename-name').value.trim(), ref: { machine: s.machine, node: s.node } }));
};
async function exportExcel() {
  try { await bridge().ExportExcel(lang); } catch (e) { await message(t('tab_export'), String(e) + '\n' + t('export_note')); }
}
$('mi-export').onclick = exportExcel; $('btn-export').onclick = exportExcel;

// ---- start ---------------------------------------------------------------------------------------------
async function init() {
  const A = await ready(); if (!A) { document.body.textContent = 'Wails bridge not found'; return; }
  await loadPacks();
  try { lang = localStorage.getItem('lang') || await A.DefaultLanguage(); } catch (e) { lang = 'ko'; }
  try { const f = +localStorage.getItem('font'); if (f) S.fontSize = f; } catch (e) {}
  setFont(S.fontSize); applyI18n();
  try {
    const [h, cfg] = await Promise.all([A.Health(), A.GetConfig()]);
    S.version = h.version; S.lockOpen = h.allowNetworkWrite;
    $('caption-title').textContent = 'AFVI TOOLS ' + h.version;
    S.machines = cfg.machines || [];
    const sel = $('machine'); sel.textContent = '';
    for (const m of S.machines) { const o = document.createElement('option'); o.value = m.name; o.textContent = m.name; sel.appendChild(o); }
    renderSource(); await reload();
  } catch (e) { setStatus('st_init_fail', String(e)); }
  if (await A.SelftestEnabled()) { const s = document.createElement('script'); s.src = 'selftest.js'; document.body.appendChild(s); }
}
init();
