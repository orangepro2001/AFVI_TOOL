'use strict';
// Scripted UI self-test, loaded only with -selftest. It acts through the DOM like a user would
// and asks Go to verify the files after each step; the verdict goes back through SelftestReport.
(async () => {
  const A = bridge(), log = [];
  const fail = m => { throw new Error(m); };
  const waitFor = async (fn, what, ms = 8000) => { const end = Date.now() + ms; for (;;) { const v = fn(); if (v) return v; if (Date.now() > end) fail('timeout: ' + what); await sleep(30); } };
  const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); };
  const rows = () => [...document.querySelectorAll('#models tbody tr')];
  const row = n => rows().find(r => r.dataset.name === n);
  const dlg = id => document.getElementById(id);
  try {
    // 1. list of the first machine (the real network machine is hidden by the self-test config)
    await waitFor(() => $('machine').options.length === 2, 'machines');
    set($('machine'), 'T1'); set($('node'), 'FM1');
    await waitFor(() => rows().length === 3, 'three models on T1/FM1');
    log.push('list');

    // 2. search is case-insensitive
    set($('search'), 'M99'); await waitFor(() => rows().length === 1 && rows()[0].dataset.name === '7M99-535', 'search M99');
    set($('search'), ''); await waitFor(() => rows().length === 3, 'search cleared');
    log.push('search');

    // 3. every language: texts switch, no key shows through
    const seen = new Set();
    for (const tag of ['ko', 'zh-Hans', 'zh-Hant', 'en']) {
      const item = [...document.querySelectorAll('#lang-items .w2k-menu-item')].find(e => e.dataset.lang === tag);
      item.click();
      await waitFor(() => $('tab-copy').textContent === packs[tag].tab_copy, 'language ' + tag);
      for (const e of document.querySelectorAll('[data-i18n]')) if (e.textContent === e.dataset.i18n) fail(`key ${e.dataset.i18n} not translated in ${tag}`);
      seen.add($('tab-copy').textContent);
    }
    if (seen.size < 4) fail('languages look the same');
    log.push('languages');
    setLang('en');

    // 4. no BM offered as a target of an FM
    if ([...$('target-single').options].some(o => o.value.endsWith('|BM'))) fail('BM offered as target of FM1');

    // 5. copy m1 FM1 -> FM2 (replaces "M1"), through the plan dialog and the job dialog
    row('m1').click(); await waitFor(() => !$('btn-copy').disabled, 'copy enabled');
    set($('target-single'), 'T1|FM2'); $('btn-copy').click();
    await waitFor(() => dlg('plan-run'), 'plan dialog');
    if (!document.querySelector('#plan-steps .danger')) fail('the deletion of the old M1 is not shown in red');
    dlg('plan-run').click();
    await waitFor(() => dlg('job-result') && dlg('job-result').textContent === packs.en.job_done, 'copy job done', 15000);
    dlg('job-close').click(); await waitFor(() => $('overlay').hidden, 'job closed');
    let v = await A.SelftestVerify('copy'); if (v) fail('copy: ' + v);
    log.push('copy');

    // 6. group rename m2 -> r2
    set($('node'), 'FM1'); await waitFor(() => row('m2'), 'm2 listed');
    row('m2').click(); $('rename-group').checked = true; set($('rename-name'), 'r2');
    await waitFor(() => !$('btn-rename').disabled, 'rename enabled'); $('btn-rename').click();
    await waitFor(() => dlg('plan-run'), 'rename plan'); dlg('plan-run').click();
    await waitFor(() => dlg('job-result') && dlg('job-result').textContent === packs.en.job_done, 'rename job done', 15000);
    dlg('job-close').click(); await waitFor(() => $('overlay').hidden, 'job closed 2');
    v = await A.SelftestVerify('rename'); if (v) fail('rename: ' + v);
    log.push('rename');

    // 7. a refused plan shows its reason and nothing runs
    await waitFor(() => row('r2'), 'r2 listed'); row('r2').click(); $('rename-group').checked = false; set($('rename-name'), 'bad/name');
    await waitFor(() => !$('btn-rename').disabled, 'rename enabled 2'); $('btn-rename').click();
    await waitFor(() => dlg('dlg-ok'), 'refusal dialog');
    if (!/illegal character/.test(document.querySelector('#overlay .w2k-dialog-body').textContent)) fail('refusal text: ' + document.querySelector('#overlay .w2k-dialog-body').textContent);
    dlg('dlg-ok').click(); await waitFor(() => $('overlay').hidden, 'refusal closed');
    log.push('refusal');

    // 8. the Excel entry answers "not implemented"
    $('btn-export').click(); await waitFor(() => dlg('dlg-ok'), 'excel dialog');
    dlg('dlg-ok').click(); log.push('excel');
    await A.SelftestReport(true, log.join(','));
  } catch (e) {
    await A.SelftestReport(false, String(e && e.message || e) + ' [done: ' + log.join(',') + ']');
  }
})();
