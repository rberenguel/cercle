'use strict';

// ── Helpers ───────────────────────────────────────────────────────────────────

const $ = id => document.getElementById(id);

function source() { return $('source-filter').value.trim(); }

async function apiFetch(path, opts = {}) {
  const r = await fetch(path, opts);
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  return r.json();
}

// Shorten a path to last 3 segments for display.
function shortPath(path) {
  const parts = path.split('/');
  return parts.length > 3 ? '…/' + parts.slice(-3).join('/') : path;
}

// Convert **bold** markers from the API into <mark> spans.
function renderSnippet(text) {
  return text.replace(/\*\*(.+?)\*\*/g, '<mark>$1</mark>');
}

// ── Markdown renderer (summary body) ──────────────────────────────────────────

function escHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function inlineMd(text) {
  return escHtml(text)
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g,     '<em>$1</em>')
    .replace(/`([^`]+)`/g,     '<code>$1</code>');
}

function renderMarkdown(text) {
  // Extract fenced code blocks so their contents are not processed.
  const blocks = [];
  const src = text.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
    const i = blocks.length;
    const cls = lang ? ` class="lang-${lang}"` : '';
    blocks.push(`<pre><code${cls}>${escHtml(code.trimEnd())}</code></pre>`);
    return `\x00${i}\x00`;
  });

  const out  = [];
  let inList = false;

  for (const raw of src.split('\n')) {
    // Restore code block placeholder (appears on its own line).
    const ph = raw.trim().match(/^\x00(\d+)\x00$/);
    if (ph) {
      if (inList) { out.push('</ul>'); inList = false; }
      out.push(blocks[+ph[1]]);
      continue;
    }

    const h3 = raw.match(/^### (.+)/);
    if (h3) { if (inList) { out.push('</ul>'); inList = false; } out.push(`<h3>${inlineMd(h3[1])}</h3>`); continue; }
    const h2 = raw.match(/^## (.+)/);
    if (h2) { if (inList) { out.push('</ul>'); inList = false; } out.push(`<h2>${inlineMd(h2[1])}</h2>`); continue; }
    const h1 = raw.match(/^# (.+)/);
    if (h1) { if (inList) { out.push('</ul>'); inList = false; } out.push(`<h1>${inlineMd(h1[1])}</h1>`); continue; }

    const li = raw.match(/^[-*] (.+)/);
    if (li) { if (!inList) { out.push('<ul>'); inList = true; } out.push(`<li>${inlineMd(li[1])}</li>`); continue; }

    if (raw.trim() === '') {
      if (inList) { out.push('</ul>'); inList = false; }
      continue; // blank line — gap handled by CSS margin
    }

    if (inList) { out.push('</ul>'); inList = false; }
    out.push(`<p>${inlineMd(raw)}</p>`);
  }

  if (inList) out.push('</ul>');
  return out.join('');
}

// ── API ───────────────────────────────────────────────────────────────────────

const api = {
  health() {
    return apiFetch('/health');
  },

  files(glob) {
    const p = new URLSearchParams();
    if (source()) p.set('source', source());
    if (glob)     p.set('glob', glob);
    return apiFetch(`/files?${p}`);
  },

  lexical(q, limit = 10) {
    const p = new URLSearchParams({ q, limit });
    if (source()) p.set('source', source());
    return apiFetch(`/search/lexical?${p}`);
  },

  structural(q, limit = 20) {
    const p = new URLSearchParams({ q, limit });
    if (source()) p.set('source', source());
    return apiFetch(`/search/structural?${p}`);
  },

  semantic(q, limit = 10) {
    const p = new URLSearchParams({ q, limit });
    if (source()) p.set('source', source());
    return apiFetch(`/search/semantic?${p}`);
  },

  summaries(limit = 50) {
    const p = new URLSearchParams({ limit });
    if (source()) p.set('source', source());
    return apiFetch(`/summaries?${p}`);
  },

  addSummary(tags, text) {
    return apiFetch('/summary', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tags, text, source: source() }),
    });
  },

  deleteSummary(id) {
    return apiFetch(`/summary?id=${id}`, { method: 'DELETE' });
  },

  sources() {
    return apiFetch('/sources');
  },

  indexDir(path, src) {
    return apiFetch('/index', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, source: src || undefined }),
    });
  },
};

// ── Health ────────────────────────────────────────────────────────────────────

async function pollHealth() {
  const dot   = $('health-dot');
  const label = $('health-label');
  try {
    const h = await api.health();
    dot.className   = 'health-dot ok';
    dot.title       = `v${h.version} · semantic: ${h.semantic}`;
    label.textContent = h.semantic ? 'ok · semantic' : 'ok';
    label.style.color = '';
  } catch {
    dot.className     = 'health-dot error';
    dot.title         = 'unreachable';
    label.textContent = 'unreachable';
    label.style.color = 'var(--red)';
  }
}

// ── Tabs ──────────────────────────────────────────────────────────────────────

let currentTab = 'search';

function switchTab(tab) {
  currentTab = tab;
  document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
  $(`tab-${tab}`).classList.add('active');
  document.querySelector(`[data-tab="${tab}"]`).classList.add('active');
  if (tab === 'files')     loadFiles();
  if (tab === 'summaries') loadSummaries();
  if (tab === 'index')     loadIndexedSources();
}

// ── Search ────────────────────────────────────────────────────────────────────

let currentMode = 'all';

function switchMode(mode) {
  currentMode = mode;
  document.querySelectorAll('.mode-btn').forEach(el => el.classList.remove('active'));
  document.querySelector(`[data-mode="${mode}"]`).classList.add('active');
}

function lexicalCard(r) {
  const el = document.createElement('div');
  el.className = 'result-card';
  el.title = r.path;
  el.innerHTML = `
    <div class="result-header">
      <span class="result-path">${shortPath(r.path)}</span>
      <span class="result-score">${r.rank.toFixed(2)}</span>
    </div>
    <div class="result-snippet">${renderSnippet(r.snippet)}</div>
  `;
  return el;
}

function structuralCard(r) {
  const el = document.createElement('div');
  el.className = 'result-card';
  el.title = r.path;
  el.innerHTML = `
    <div class="result-header">
      <span class="result-kind kind-${r.kind}">${r.kind}</span>
      <span class="result-name">${r.name}</span>
    </div>
    <div class="result-path">${shortPath(r.path)}:${r.start_line}</div>
    ${r.signature ? `<div class="result-sig">${r.signature}</div>` : ''}
  `;
  return el;
}

function semanticCard(r) {
  const pct = (r.similarity * 100).toFixed(1);
  const el  = document.createElement('div');
  el.className = 'result-card';
  el.title = r.path;
  el.innerHTML = `
    <div class="result-header">
      <span class="result-path">${shortPath(r.path)}</span>
      <span class="result-score similarity">${pct}%</span>
    </div>
    <div class="result-kind kind-${r.kind}">${r.kind}</div>
  `;
  return el;
}

function renderColumn(container, label, colorClass, results, cardFn) {
  const col = document.createElement('div');
  col.className = 'results-col';

  const header = document.createElement('div');
  header.className = `col-header col-${colorClass}`;
  header.textContent = `${label} (${results.length})`;
  col.appendChild(header);

  if (results.length === 0) {
    const empty = document.createElement('div');
    empty.className   = 'empty-state';
    empty.textContent = 'no results';
    col.appendChild(empty);
  } else {
    results.forEach(r => col.appendChild(cardFn(r)));
  }

  container.appendChild(col);
}

async function runSearch() {
  const q = $('search-query').value.trim();
  if (!q) return;

  const container = $('search-results');
  container.className  = 'search-results';
  container.innerHTML  = '<div class="loading">searching…</div>';

  try {
    if (currentMode === 'all') {
      const [lex, str, sem] = await Promise.allSettled([
        api.lexical(q),
        api.structural(q),
        api.semantic(q),
      ]);

      container.innerHTML = '';
      container.className = 'search-results three-col';

      renderColumn(container, 'Lexical',    'blue',   settled(lex), lexicalCard);
      renderColumn(container, 'Structural', 'cyan',   settled(str), structuralCard);
      renderColumn(container, 'Semantic',   'violet', settled(sem), semanticCard);
    } else {
      const fetchers = { lexical: api.lexical, structural: api.structural, semantic: api.semantic };
      const cardFns  = { lexical: lexicalCard, structural: structuralCard, semantic: semanticCard };
      const colors   = { lexical: 'blue', structural: 'cyan', semantic: 'violet' };
      const label    = currentMode.charAt(0).toUpperCase() + currentMode.slice(1);

      const raw = await fetchers[currentMode].call(api, q);
      const results = Array.isArray(raw) ? raw : (raw.results ?? []);
      container.innerHTML = '';
      container.className = 'search-results one-col';
      renderColumn(container, label, colors[currentMode], results, cardFns[currentMode]);
    }
  } catch (err) {
    container.innerHTML = `<div class="error-state">error: ${err.message}</div>`;
  }
}

// Unwrap a Promise.allSettled result; return [] on rejection.
// Lexical returns {results, query}; all others return a plain array.
function settled(r) {
  if (r.status !== 'fulfilled') return [];
  const v = r.value ?? [];
  return Array.isArray(v) ? v : (v.results ?? []);
}

// ── Files ─────────────────────────────────────────────────────────────────────

function fileEntry(path) {
  const el = document.createElement('div');
  el.className   = 'file-entry';
  el.textContent = path;
  el.title       = 'click to copy';
  el.addEventListener('click', () => {
    navigator.clipboard.writeText(path);
    el.classList.add('copied');
    setTimeout(() => el.classList.remove('copied'), 900);
  });
  return el;
}

async function loadFiles() {
  const glob      = $('files-glob').value.trim();
  const srcFilter = source();
  const container = $('files-list');
  const countEl   = $('file-count');
  container.innerHTML = '<div class="loading">loading…</div>';
  countEl.textContent = '';

  try {
    // When a source filter is active, show a flat list as before.
    if (srcFilter) {
      const data  = await api.files(glob);
      const files = data.files ?? [];
      countEl.textContent = `${files.length} file${files.length !== 1 ? 's' : ''}`;
      container.innerHTML = '';
      if (files.length === 0) {
        container.innerHTML = '<div class="empty-state">no files indexed</div>';
        return;
      }
      files.forEach(path => container.appendChild(fileEntry(path)));
      return;
    }

    // No source filter: group by source namespace.
    const sources = await api.sources();
    container.innerHTML = '';

    if (sources.length === 0) {
      countEl.textContent = '0 files';
      container.innerHTML = '<div class="empty-state">no files indexed</div>';
      return;
    }

    let total = 0;
    for (const s of sources) {
      const p    = new URLSearchParams({ source: s.source });
      if (glob) p.set('glob', glob);
      const data  = await apiFetch(`/files?${p}`);
      const files = data.files ?? [];
      total += files.length;

      const pendingClass = s.pending > 0 ? 'stat-pending' : 'stat-ok';
      const header = document.createElement('div');
      header.className = 'source-group-header';
      header.innerHTML = `
        <span class="source-group-name">${s.source}</span>
        <span class="source-group-meta">
          ${files.length} files &middot;
          ${s.chunks} chunks &middot;
          ${s.embedded} embedded &middot;
          <span class="${pendingClass}">${s.pending} pending</span>
        </span>
      `;
      container.appendChild(header);

      if (files.length === 0) {
        const empty = document.createElement('div');
        empty.className   = 'empty-state';
        empty.textContent = 'no matching files';
        container.appendChild(empty);
      } else {
        files.forEach(path => container.appendChild(fileEntry(path)));
      }
    }
    countEl.textContent = `${total} file${total !== 1 ? 's' : ''} across ${sources.length} source${sources.length !== 1 ? 's' : ''}`;
  } catch (err) {
    container.innerHTML = `<div class="error-state">error: ${err.message}</div>`;
  }
}

// ── Summaries ─────────────────────────────────────────────────────────────────

async function loadSummaries() {
  const container = $('summaries-list');
  container.innerHTML = '<div class="loading">loading…</div>';

  try {
    const data = await api.summaries();
    container.innerHTML = '';

    const items = Array.isArray(data) ? data : [];
    if (items.length === 0) {
      container.innerHTML = '<div class="empty-state">no summaries yet</div>';
      return;
    }

    items.forEach(s => {
      const date    = new Date(s.created_at * 1000).toLocaleString();
      const bodyHtml = renderMarkdown(s.text || '');
      const lineCount = (s.text || '').split('\n').length;
      const collapsible = lineCount > 12;

      const el = document.createElement('div');
      el.className = 'summary-card';
      el.innerHTML = `
        <div class="summary-header">
          <span class="summary-tags">${escHtml(s.tags || '(no tags)')}</span>
          <span class="summary-date">${date}</span>
          <button class="delete-btn" data-id="${s.id}" title="delete">×</button>
        </div>
        <div class="summary-body${collapsible ? ' collapsed' : ''}">${bodyHtml}</div>
        ${collapsible ? '<button class="summary-toggle">▾ show all</button>' : ''}
      `;
      el.querySelector('.delete-btn').addEventListener('click', async () => {
        await api.deleteSummary(s.id);
        loadSummaries();
      });
      if (collapsible) {
        const body   = el.querySelector('.summary-body');
        const toggle = el.querySelector('.summary-toggle');
        toggle.addEventListener('click', () => {
          const open = body.classList.toggle('collapsed');
          toggle.textContent = open ? '▾ show all' : '▴ collapse';
        });
      }
      container.appendChild(el);
    });
  } catch (err) {
    container.innerHTML = `<div class="error-state">error: ${err.message}</div>`;
  }
}

async function submitSummary() {
  const tags = $('summary-tags').value.trim();
  const text = $('summary-text').value.trim();
  if (!text) return;

  const btn = $('summary-submit');
  btn.textContent = 'adding…';
  btn.disabled    = true;

  try {
    await api.addSummary(tags, text);
    $('summary-tags').value = '';
    $('summary-text').value = '';
    loadSummaries();
  } catch (err) {
    alert(`error: ${err.message}`);
  } finally {
    btn.textContent = 'add summary';
    btn.disabled    = false;
  }
}

// ── Index ─────────────────────────────────────────────────────────────────────

async function loadIndexedSources() {
  const list = $('indexed-sources');
  list.innerHTML = '<div class="loading">loading…</div>';
  try {
    const sources = await api.sources();
    list.innerHTML = '';
    if (sources.length === 0) {
      list.innerHTML = '<div class="empty-state">nothing indexed yet</div>';
      return;
    }
    sources.forEach(s => {
      const pendingClass = s.pending > 0 ? 'stat-pending' : 'stat-ok';
      const el = document.createElement('div');
      el.className = 'indexed-source';
      el.innerHTML = `
        <div class="indexed-source-path">${s.source}</div>
        <div class="indexed-source-stats">
          <span class="istat">${s.files} <em>files</em></span>
          <span class="istat">${s.chunks} <em>chunks</em></span>
          <span class="istat">${s.embedded} <em>embedded</em></span>
          <span class="istat ${pendingClass}">${s.pending} <em>pending</em></span>
        </div>
      `;
      el.title = 'click to prefill path';
      el.addEventListener('click', () => { $('index-path').value = s.source; });
      list.appendChild(el);
    });
  } catch (err) {
    list.innerHTML = `<div class="error-state">error: ${err.message}</div>`;
  }
}

async function runIndex() {
  const path = $('index-path').value.trim();
  if (!path) return;

  const src = $('index-source').value.trim();
  const btn = $('index-btn');
  const out = $('index-result');

  btn.textContent = 'indexing…';
  btn.disabled    = true;
  out.innerHTML   = '<div class="loading">indexing…</div>';

  try {
    const r = await api.indexDir(path, src);
    const errors = (r.errors ?? []).map(e => `<div class="index-error">⚠ ${e}</div>`).join('');
    out.innerHTML = `
      <div class="index-stats">
        <span class="stat"><span class="stat-n">${r.files}</span> files</span>
        <span class="stat"><span class="stat-n">${r.symbols}</span> symbols</span>
        <span class="stat"><span class="stat-n">${r.skipped}</span> skipped</span>
        <span class="stat"><span class="stat-n">${r.pending_embed}</span> pending embed</span>
      </div>
      ${errors}
    `;
    loadIndexedSources();
  } catch (err) {
    out.innerHTML = `<div class="error-state">error: ${err.message}</div>`;
  } finally {
    btn.textContent = 'index';
    btn.disabled    = false;
  }
}

// ── Init ──────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  // Tabs
  document.querySelectorAll('.tab').forEach(btn =>
    btn.addEventListener('click', () => switchTab(btn.dataset.tab))
  );

  // Search mode
  document.querySelectorAll('.mode-btn').forEach(btn =>
    btn.addEventListener('click', () => switchMode(btn.dataset.mode))
  );

  // Search
  $('search-query').addEventListener('keydown', e => { if (e.key === 'Enter') runSearch(); });
  $('search-btn').addEventListener('click', runSearch);

  // Files
  $('files-glob').addEventListener('keydown', e => { if (e.key === 'Enter') loadFiles(); });
  $('files-refresh').addEventListener('click', loadFiles);

  // Summaries
  $('summary-submit').addEventListener('click', submitSummary);
  $('summary-text').addEventListener('keydown', e => {
    if (e.key === 'Enter' && e.metaKey) submitSummary();
  });

  // Index
  $('index-path').addEventListener('keydown', e => { if (e.key === 'Enter') runIndex(); });
  $('index-btn').addEventListener('click', runIndex);

  // Health
  pollHealth();
  setInterval(pollHealth, 5000);
});
