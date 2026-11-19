// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
//
// Fixture generator interface. All the league logic lives in engine.js; this
// file draws the four steps and animates between them.

import * as E from './engine.js';

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

const $ = (sel, root = document) => root.querySelector(sel);
const reduceMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/** Build a DOM node. Text always goes in as text, never as HTML. */
function h(tag, props, ...children) {
  const el = document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (value === null || value === undefined || value === false) continue;
    if (key === 'class') el.className = value;
    else if (key === 'dataset') Object.assign(el.dataset, value);
    else if (key === 'style') el.style.cssText = value;
    else if (key.startsWith('on')) el.addEventListener(key.slice(2), value);
    else if (value === true) el.setAttribute(key, '');
    else el.setAttribute(key, value);
  }
  for (const child of children.flat(Infinity)) {
    if (child === null || child === undefined || child === false) continue;
    el.append(child.nodeType ? child : document.createTextNode(String(child)));
  }
  return el;
}

const ordinal = (n) => {
  const s = ['th', 'st', 'nd', 'rd'];
  const v = n % 100;
  return n + (s[(v - 20) % 10] || s[v] || s[0]);
};

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const SHORT_DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
function shortDate(iso) {
  const d = E.parseISO(iso);
  return `${SHORT_DAYS[d.getUTCDay()]} ${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}`;
}

function clubHue(club) {
  let hash = 0;
  for (const ch of club) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0;
  return (hash * 47) % 360;
}
const clubDot = (club) => h('span', { class: 'dot', style: `--hue:${clubHue(club)}`, 'aria-hidden': 'true' });

function toast(message, kind = '') {
  const el = $('#toast');
  el.textContent = message;
  el.className = `toast show ${kind}`;
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => (el.className = 'toast'), 4200);
}

function download(filename, text, type) {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = h('a', { href: url, download: filename });
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

const slug = (s) =>
  String(s || 'league')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

const config = JSON.parse($('#fg-config').textContent || '{}');
const STORE_KEY = 'jim-tennis-fixture-generator-v1';
const STEPS = ['Final tables', 'Promotion & relegation', 'Fixtures', 'Share'];

const state = {
  step: 0,
  tables: null, // { league, season, source, fetchedAt, snapshot, divisions }
  tablesNote: '',
  rules: {}, // overrides of E.DEFAULT_RULES
  declined: [],
  withdrawn: [],
  added: [],
  manual: {},
  season: { year: null, week1Monday: null, playDays: {}, derbiesEarly: true, dateOverrides: {} },
  seed: 0,
  draw: 0, // how many draws have been made, shown so a new draw is plainly a new draw
  result: null, // { divisions, stats } from the engine or a loaded file
  resultKey: '',
  accepted: null, // ISO timestamp once accepted
  loaded: null, // { name, file, intact } when showing a loaded file
  division: 0,
  club: null,
  team: null,
};

function persist() {
  const { tables, rules, declined, withdrawn, added, manual, season, seed, draw, accepted, step } = state;
  try {
    localStorage.setItem(
      STORE_KEY,
      JSON.stringify({ tables, rules, declined, withdrawn, added, manual, season, seed, draw, accepted, step: Math.min(step, 2) })
    );
  } catch {
    /* private mode: carry on without saving */
  }
}

function restore() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORE_KEY) || 'null');
    if (!saved || !saved.tables) return false;
    Object.assign(state, saved);
    state.season = { playDays: {}, dateOverrides: {}, derbiesEarly: true, ...saved.season };
    return true;
  } catch {
    return false;
  }
}

const rules = () => ({ ...E.DEFAULT_RULES, ...state.rules });

function movement() {
  return E.resolveMovement(state.tables, {
    rules: state.rules,
    declined: state.declined,
    withdrawn: state.withdrawn,
    added: state.added,
    manual: state.manual,
  });
}

/** Next season's divisions with play days and where each team came from. */
function nextDivisions(mv = movement()) {
  return mv.divisions.map((d, i) => ({
    name: d.name,
    level: d.level,
    playDay: state.season.playDays[d.name] || d.playDay || E.defaultPlayDay(d.level),
    teams: d.teams.map((t) => ({
      ...t,
      previousDivision: t.from === null || t.from === undefined ? undefined : state.tables.divisions[t.from].name,
    })),
    index: i,
  }));
}

function setTables(tables, note) {
  state.tables = tables;
  state.tablesNote = note || '';
  const year = (Number(tables.season) || new Date().getFullYear()) + 1;
  if (state.season.year !== year) {
    state.season = { year, week1Monday: E.defaultWeek1Monday(year), playDays: {}, derbiesEarly: true, dateOverrides: {} };
  }
  if (!state.seed) state.seed = Math.floor(Math.random() * 2 ** 31) + 1;
}

async function loadTables(force) {
  const attempt = async (url) => {
    const res = await fetch(url, { headers: { Accept: 'application/json' } });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    if (!data || !Array.isArray(data.divisions) || !data.divisions.length) throw new Error('empty');
    return data;
  };
  try {
    const live = await attempt((config.tablesURL || '/fixture-generator/api/tables') + (force ? '?refresh=1' : ''));
    return { tables: live, note: '' };
  } catch {
    const snap = await attempt(config.snapshotURL || '/static/fixture-generator/bhplta-2026.json');
    return { tables: snap, note: "Couldn't reach the league website, so this is the saved copy of the final tables." };
  }
}

// ---------------------------------------------------------------------------
// Animation
// ---------------------------------------------------------------------------

/** Remember where every team row is, so the next render can fly them into place. */
function snapshotRows() {
  const rects = new Map();
  document.querySelectorAll('#stage [data-team]').forEach((el) => {
    const r = el.getBoundingClientRect();
    if (r.width) rects.set(el.dataset.team, r);
  });
  return rects;
}

function flyRows(before) {
  if (reduceMotion() || !before.size) return;
  const rows = [...document.querySelectorAll('#stage [data-team]')];
  let moved = 0;
  rows.forEach((el) => {
    const from = before.get(el.dataset.team);
    const to = el.getBoundingClientRect();
    if (!from) {
      el.animate([{ opacity: 0, transform: 'scale(.9)' }, { opacity: 1, transform: 'none' }], { duration: 400, easing: 'ease-out' });
      return;
    }
    const dx = from.left - to.left;
    const dy = from.top - to.top;
    if (Math.abs(dx) < 1 && Math.abs(dy) < 1) return;
    const far = Math.abs(dx) > 40 || Math.abs(dy) > 120;
    const delay = far ? 120 + Math.min(moved++, 14) * 70 : 0;
    el.style.zIndex = far ? 3 : 1;
    const anim = el.animate(
      [
        { transform: `translate(${dx}px, ${dy}px)`, offset: 0 },
        ...(far ? [{ transform: `translate(${dx * 0.5}px, ${dy * 0.5}px) scale(1.06)`, offset: 0.5 }] : []),
        { transform: 'none', offset: 1 },
      ],
      { duration: far ? 900 : 450, delay, easing: 'cubic-bezier(.2,.8,.2,1)', fill: 'backwards' }
    );
    if (far) el.classList.add('flying');
    anim.finished.finally(() => {
      el.style.zIndex = '';
      el.classList.remove('flying');
    });
  });
}

function countUp(el, to) {
  if (reduceMotion() || !Number.isFinite(to) || to === 0) {
    el.textContent = to;
    return;
  }
  const start = performance.now();
  const tick = (now) => {
    const p = Math.min(1, (now - start) / 700);
    el.textContent = Math.round(to * (1 - Math.pow(1 - p, 3)));
    if (p < 1) requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
}

function celebrate() {
  if (reduceMotion()) return;
  const layer = h('div', { class: 'confetti', 'aria-hidden': 'true' });
  document.body.append(layer);
  for (let i = 0; i < 28; i++) {
    const ball = h('span', {});
    layer.append(ball);
    const x = Math.random() * 100;
    const drift = (Math.random() - 0.5) * 30;
    ball.style.left = `${x}vw`;
    ball
      .animate(
        [
          { transform: 'translate(0,-10vh) rotate(0)', opacity: 1 },
          { transform: `translate(${drift}vw,70vh) rotate(${360 + Math.random() * 360}deg)`, opacity: 1, offset: 0.7 },
          { transform: `translate(${drift * 1.3}vw,110vh) rotate(900deg)`, opacity: 0 },
        ],
        { duration: 1500 + Math.random() * 1200, delay: Math.random() * 350, easing: 'cubic-bezier(.3,.1,.6,1)', fill: 'both' }
      );
  }
  setTimeout(() => layer.remove(), 3400);
}

// ---------------------------------------------------------------------------
// Shell: steps, action bar, season line
// ---------------------------------------------------------------------------

function canVisit(step) {
  if (state.loaded) return step >= 2;
  if (!state.tables) return step === 0;
  if (step <= 1) return true;
  return movement().violations.length === 0 && (step === 2 || !!state.result);
}

function go(step, opts = {}) {
  if (!canVisit(step)) return;
  const before = opts.fly ? snapshotRows() : null;
  state.step = step;
  persist();
  render();
  if (before) flyRows(before);
  if (!opts.keepScroll) window.scrollTo({ top: 0, behavior: reduceMotion() ? 'auto' : 'smooth' });
  $('#stage').focus({ preventScroll: true });
}

function renderShell() {
  const list = $('#steps');
  list.replaceChildren(
    ...STEPS.map((label, i) =>
      h(
        'li',
        { class: i === state.step ? 'current' : i < state.step ? 'done' : '' },
        h(
          'button',
          { type: 'button', disabled: !canVisit(i), 'aria-current': i === state.step ? 'step' : null, onclick: () => go(i, { fly: i <= 1 && state.step <= 1 }) },
          h('span', { class: 'n' }, i + 1),
          h('span', { class: 'l' }, label)
        )
      )
    )
  );
  const line = $('#season-line');
  if (state.loaded) {
    const f = state.loaded.file;
    line.textContent = `${f.league?.name || 'Fixture file'} · ${f.season.year} season`;
  } else if (state.tables) {
    line.textContent = `${state.tables.league?.name || 'League'} · ${state.tables.season} results → ${state.season.year} fixtures`;
  } else {
    line.textContent = '';
  }
}

function setActions(...nodes) {
  $('#action-bar').replaceChildren(...nodes.filter(Boolean));
  document.body.classList.toggle('has-actions', nodes.filter(Boolean).length > 0);
}

// ---------------------------------------------------------------------------
// Step 1: final tables
// ---------------------------------------------------------------------------

function renderTables(stage) {
  if (!state.tables) {
    stage.append(h('div', { class: 'loading' }, h('span', { class: 'ball big' }), h('p', {}, 'Fetching the final tables…')));
    setActions();
    return;
  }
  const r = rules();
  const count = state.tables.divisions.length;
  const fetched = state.tables.fetchedAt ? new Date(state.tables.fetchedAt) : null;

  stage.append(
    h(
      'section',
      { class: 'intro' },
      h('p', { class: 'eyebrow' }, `${state.tables.season} season · final tables`),
      h('h2', {}, 'Start from how last season finished.'),
      h(
        'p',
        { class: 'lede' },
        `Top ${r.promote} go up, bottom ${r.relegate} go down, and no club may have more than ${r.maxPerClub} teams in a division. `,
        'The next step applies those rules for you.'
      ),
      h(
        'p',
        { class: 'source' },
        h('span', { class: `pill ${state.tables.snapshot ? 'warn' : 'ok'}` }, state.tables.snapshot ? 'Saved copy' : 'Live'),
        ' ',
        state.tables.snapshot
          ? state.tablesNote || 'Saved copy of the final tables.'
          : `From the league website${fetched && !isNaN(fetched) ? `, ${fetched.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })}` : ''}.`,
        ' ',
        h('button', { type: 'button', class: 'link', onclick: refreshTables }, 'Refresh')
      )
    )
  );

  const grid = h('div', { class: 'divisions' });
  state.tables.divisions.forEach((d, di) => {
    const ranked = E.rankDivision(d.teams);
    const size = ranked.length;
    const downFrom = di < count - 1 ? size - Math.min(r.relegate, Math.max(0, size - r.promote)) : Infinity;
    grid.append(
      h(
        'article',
        { class: 'division', style: `--i:${di}` },
        h('header', {}, h('h3', {}, d.name), h('span', { class: 'meta' }, `${size} teams`)),
        h(
          'ol',
          { class: 'table' },
          h('li', { class: 'thead', 'aria-hidden': 'true' }, h('span', {}), h('span', {}, 'Team'), h('span', {}, 'Pld'), h('span', {}, 'Sets'), h('span', {}, 'Pts')),
          ranked.map((t, i) => {
            const zone = di > 0 && i < r.promote ? 'up' : i >= downFrom ? 'down' : '';
            const diff = t.setsWon - t.setsLost;
            return h(
              'li',
              { class: `row ${zone}`, dataset: { team: t.name }, style: `--r:${i}` },
              h('span', { class: 'pos' }, t.position),
              h('span', { class: 'name' }, clubDot(t.club), t.name, t.tied ? h('abbr', { class: 'tie', title: 'Level on points and sets with another team' }, '=') : null),
              h('span', { class: 'num' }, t.played ?? ''),
              h('span', { class: 'num' }, (diff > 0 ? '+' : '') + diff),
              h('span', { class: 'num pts' }, t.points, t.penalty ? h('abbr', { title: 'Includes penalty points deducted' }, '*') : null)
            );
          })
        )
      )
    );
  });
  stage.append(grid);
  stage.append(
    h(
      'p',
      { class: 'legend' },
      h('span', { class: 'key up' }),
      'promotion places',
      h('span', { class: 'key down' }),
      'relegation places',
      h('span', { class: 'sep' }),
      'Ties are split on sets won minus sets lost, as the league rules say.'
    )
  );

  setActions(h('button', { type: 'button', class: 'btn primary', onclick: () => go(1, { fly: true }) }, 'Apply promotion & relegation', h('span', { class: 'arrow' }, '→')));
}

async function refreshTables() {
  toast('Fetching the latest tables…');
  const { tables, note } = await loadTables(true);
  setTables(tables, note);
  state.result = null;
  persist();
  render();
  toast(note || 'Tables are up to date.');
}

// ---------------------------------------------------------------------------
// Step 2: promotion and relegation
// ---------------------------------------------------------------------------

const BADGES = {
  promoted: ['▲', 'Promoted', 'up'],
  relegated: ['▼', 'Relegated', 'down'],
  new: ['＋', 'New team', 'new'],
};

function teamBadge(t) {
  if (t.manual) return ['✎', 'Moved by committee', 'manual'];
  if (t.movement !== 'stay') return BADGES[t.movement] || null;
  if (t.blocked) return ['■', 'Promotion blocked', 'blocked'];
  if (t.declined) return ['✕', 'Declined promotion', 'blocked'];
  if (t.held) return ['■', 'No place above', 'blocked'];
  if (t.spared) return ['◆', 'Relegation blocked', 'saved'];
  if (t.reprieved) return ['◆', 'Reprieved', 'saved'];
  return null;
}

function listNames(names) {
  if (names.length <= 1) return names.join('');
  return `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
}

/** A ruling in plain words. Returns null for the routine ones. */
function describe(d, names) {
  const r = rules();
  switch (d.kind) {
    case 'blocked':
      return {
        tone: 'blocked',
        title: `${d.team} can't go up`,
        body: `${d.team} finished ${ordinal(d.position)} in ${names[d.from]}, but ${listNames(d.already)} will already be in ${names[d.to]} and a club may have at most ${r.maxPerClub} teams in a division.`,
        rule: 'Rule 2(b)',
      };
    case 'offered':
      return {
        tone: 'up',
        title: `${d.team} offered promotion`,
        body: `${d.team} finished ${ordinal(d.position)} in ${names[d.from]} and takes the place${d.insteadOf ? ` ${d.insteadOf} could not` : ''} in ${names[d.to]}.`,
        rule: 'Rule 2(b)',
        team: d.team,
        canDecline: true,
      };
    case 'declined':
      return {
        tone: 'blocked',
        title: `${d.team} declined promotion`,
        body: `${d.team} (${ordinal(d.position)} in ${names[d.from]}) stay where they are.`,
        rule: 'Committee',
        team: d.team,
        canAccept: true,
      };
    case 'held':
      return {
        tone: 'blocked',
        title: `${d.team} stay in ${names[d.from]}`,
        body: `${d.team} finished ${ordinal(d.position)}, but no place opened in ${names[d.to]}: a team there could not be relegated.`,
        rule: 'Rule 2(b)',
      };
    case 'spared':
      return {
        tone: 'saved',
        title: `${d.team} can't go down`,
        body: `${d.team} finished ${ordinal(d.position)} in ${names[d.from]}, but ${listNames(d.already)} will already be in ${names[d.to]}. They stay up, and one fewer team is promoted in exchange.`,
        rule: 'Rule 2(b)',
      };
    case 'reprieved':
      return {
        tone: 'saved',
        title: `${d.team} reprieved`,
        body: `A promotion place into ${names[d.from]} went unfilled, so ${d.team} (${ordinal(d.position)}) are not relegated.`,
        rule: 'Rule 2(b)',
      };
    case 'tie':
      return {
        tone: 'warn',
        title: `Tie in ${names[d.division]}`,
        body: `${listNames(d.teams)} finished level on points and on sets. The rules call for a deciding match; until then they are in the order the league website lists them.`,
        rule: 'Regulation 10',
      };
    case 'manual':
      return {
        tone: 'manual',
        title: `${d.team} moved to ${names[d.to]}`,
        body: `Committee decision (the rules placed them in ${names[d.from]}).`,
        rule: 'Committee',
        team: d.team,
        canReset: true,
      };
    case 'withdrawn':
      return { tone: 'manual', title: `${d.team} withdrawn`, body: `${d.team} leave the league.`, rule: 'Committee', team: d.team, canRestore: true };
    case 'new':
      return { tone: 'up', title: `${d.team} join ${names[d.to]}`, body: 'New team.', rule: 'Committee', team: d.team, canRemove: true };
    default:
      return null;
  }
}

function describeViolation(v, names) {
  if (v.kind === 'club-cap') {
    return `${names[v.division]} has ${v.teams.length} ${v.club} teams (${listNames(v.teams)}). Move one of them to settle it.`;
  }
  if (v.kind === 'too-small') return `${names[v.division]} needs at least two teams.`;
  if (v.kind === 'too-big') return `${names[v.division]} has ${v.size} teams, more than an 18-week season can hold.`;
  return `${v.team} appears twice.`;
}

function changeMovement(mutate) {
  const before = snapshotRows();
  mutate();
  state.result = null;
  state.accepted = null;
  persist();
  render();
  flyRows(before);
}

function renderMovement(stage) {
  const mv = movement();
  const names = state.tables.divisions.map((d) => d.name);
  const rulings = mv.decisions.map((d) => describe(d, names)).filter(Boolean);
  const ups = mv.decisions.filter((d) => d.kind === 'promoted' || d.kind === 'offered').length;
  const downs = mv.decisions.filter((d) => d.kind === 'relegated').length;

  stage.append(
    h(
      'section',
      { class: 'intro' },
      h('p', { class: 'eyebrow' }, `${state.season.year} season · divisions`),
      h('h2', {}, `${ups} up, ${downs} down.`),
      h(
        'p',
        { class: 'lede' },
        rulings.length
          ? `${rulings.length} ${rulings.length === 1 ? 'place needed a ruling' : 'places needed a ruling'}. Each one is explained below, and the committee can overrule any of them: tap a team to move it.`
          : 'Every move followed the standard rule. Tap a team if the committee needs to move it.'
      )
    )
  );

  stage.append(renderEntries(mv, names));

  if (mv.violations.length) {
    stage.append(
      h(
        'div',
        { class: 'banner bad', role: 'alert' },
        h('strong', {}, 'Needs a committee decision'),
        h('ul', {}, mv.violations.map((v) => h('li', {}, describeViolation(v, names))))
      )
    );
  }

  const grid = h('div', { class: 'divisions next' });
  mv.divisions.forEach((d, di) => {
    const clubs = new Map();
    d.teams.forEach((t) => clubs.set(t.club, (clubs.get(t.club) || 0) + 1));
    grid.append(
      h(
        'article',
        { class: 'division' },
        h('header', {}, h('h3', {}, d.name), h('span', { class: 'meta' }, `${d.teams.length} teams`)),
        h(
          'ol',
          { class: 'table movement' },
          d.teams.map((t, i) => {
            const badge = teamBadge(t);
            const over = clubs.get(t.club) > rules().maxPerClub;
            const was = t.from === null || t.from === undefined ? 'new this season' : `${ordinal(t.previousPosition)} in ${names[t.from]}`;
            return h(
              'li',
              { class: `row ${badge ? badge[2] : ''} ${over ? 'over' : ''}`, dataset: { team: t.name }, style: `--r:${i}` },
              h(
                'button',
                { type: 'button', class: 'team-btn', onclick: () => openTeamSheet(t, di, mv), 'aria-label': `${t.name}, ${badge ? badge[1] + ', ' : ''}${was}. Change` },
                h('span', { class: 'name' }, clubDot(t.club), t.name),
                h('span', { class: 'was' }, was),
                badge ? h('span', { class: `badge ${badge[2]}`, title: badge[1] }, h('span', { 'aria-hidden': 'true' }, badge[0]), h('span', { class: 'badge-text' }, badge[1])) : null
              )
            );
          })
        ),
        h('button', { type: 'button', class: 'add-row', onclick: () => openAddSheet(di, names) }, h('span', { 'aria-hidden': 'true' }, '＋'), `Add a team to ${d.name}`)
      )
    );
  });
  stage.append(grid);

  if (rulings.length) {
    stage.append(
      h(
        'section',
        { class: 'rulings' },
        h('h3', {}, 'Rulings'),
        h(
          'ol',
          {},
          rulings.map((x, i) =>
            h(
              'li',
              { class: `ruling ${x.tone}`, style: `--i:${i}` },
              h('span', { class: 'rule' }, x.rule),
              h('strong', {}, x.title),
              h('p', {}, x.body),
              x.canDecline
                ? h('button', { type: 'button', class: 'btn small', onclick: () => changeMovement(() => state.declined.push(x.team)) }, `${x.team} decline`)
                : null,
              x.canAccept
                ? h('button', { type: 'button', class: 'btn small', onclick: () => changeMovement(() => (state.declined = state.declined.filter((n) => n !== x.team))) }, 'Undo')
                : null,
              x.canReset ? h('button', { type: 'button', class: 'btn small', onclick: () => changeMovement(() => delete state.manual[x.team]) }, 'Undo') : null,
              x.canRestore
                ? h('button', { type: 'button', class: 'btn small', onclick: () => changeMovement(() => (state.withdrawn = state.withdrawn.filter((n) => n !== x.team))) }, 'Undo')
                : null,
              x.canRemove
                ? h('button', { type: 'button', class: 'btn small', onclick: () => changeMovement(() => (state.added = state.added.filter((a) => E.cleanName(a.name) !== x.team))) }, 'Remove')
                : null
            )
          )
        )
      )
    );
  }

  stage.append(renderRulesPanel(names));

  setActions(
    h('button', { type: 'button', class: 'btn ghost', onclick: () => go(0, { fly: true }) }, 'Back'),
    h(
      'button',
      {
        type: 'button',
        class: 'btn primary',
        disabled: mv.violations.length > 0,
        onclick: () => {
          if (!state.accepted) newDraw();
          go(2);
        },
      },
      state.accepted ? 'View accepted fixtures' : 'Generate fixtures',
      h('span', { class: 'arrow' }, '→')
    )
  );
}

function renderRulesPanel(names) {
  const r = rules();
  const setRule = (key, value) => changeMovement(() => (state.rules = { ...state.rules, [key]: value }));
  const select = (key, options) =>
    h(
      'select',
      { onchange: (e) => setRule(key, e.target.value === 'any' ? 'any' : Number(e.target.value)) },
      options.map(([value, label]) => h('option', { value, selected: String(r[key]) === String(value) }, label))
    );
  const dirty = Object.keys(state.rules).length || state.declined.length || state.withdrawn.length || state.added.length || Object.keys(state.manual).length;

  return h(
    'details',
    { class: 'panel' },
    h('summary', {}, 'League rules'),
    h(
      'div',
      { class: 'fields' },
      h('label', {}, h('span', {}, 'Promoted per division'), select('promote', [[1, '1'], [2, '2'], [3, '3']])),
      h('label', {}, h('span', {}, 'Relegated per division'), select('relegate', [[1, '1'], [2, '2'], [3, '3']])),
      h('label', {}, h('span', {}, 'Most teams per club in a division'), select('maxPerClub', [[1, '1'], [2, '2'], [3, '3'], [99, 'No limit']])),
      h(
        'label',
        {},
        h('span', {}, 'When promotion is blocked, offer the place'),
        select('offerDepth', [
          [1, 'to the next team only (rule as written)'],
          [2, 'down two places'],
          [3, 'down three places'],
          ['any', 'as far down as needed'],
        ])
      )
    ),
    dirty
      ? h(
          'button',
          {
            type: 'button',
            class: 'link',
            onclick: () =>
              changeMovement(() => {
                state.rules = {};
                state.declined = [];
                state.withdrawn = [];
                state.added = [];
                state.manual = {};
              }),
          },
          'Undo every committee change'
        )
      : null
  );
}

/** Teams entering and leaving: the committee's call, made before the draw. */
function renderEntries(mv, names) {
  const leaving = state.withdrawn;
  const joining = state.added;
  return h(
    'section',
    { class: 'entries' },
    h(
      'div',
      { class: 'entries-text' },
      h('h3', {}, 'Who is entering next season?'),
      h('p', {}, 'If a club is dropping a team, or entering a new one, say so here first. Promotion and relegation are worked out again straight away, because one team leaving can unblock another.')
    ),
    h(
      'div',
      { class: 'entries-actions' },
      h('button', { type: 'button', class: 'btn', onclick: () => openRemoveSheet(mv, names) }, h('span', { 'aria-hidden': 'true' }, '−'), 'Remove a team'),
      h('button', { type: 'button', class: 'btn', onclick: () => openAddSheet(names.length - 1, names) }, h('span', { 'aria-hidden': 'true' }, '＋'), 'Add a team')
    ),
    leaving.length || joining.length
      ? h(
          'ul',
          { class: 'entries-list' },
          leaving.map((name) =>
            h(
              'li',
              { class: 'out' },
              h('span', {}, h('strong', {}, name), ' leave the league'),
              h('button', { type: 'button', class: 'link', onclick: () => changeMovement(() => (state.withdrawn = state.withdrawn.filter((n) => n !== name))) }, 'Put back')
            )
          ),
          joining.map((a) =>
            h(
              'li',
              { class: 'in' },
              h('span', {}, h('strong', {}, E.cleanName(a.name)), ` join ${names[Math.min(names.length - 1, a.division ?? names.length - 1)]}`),
              h('button', { type: 'button', class: 'link', onclick: () => changeMovement(() => (state.added = state.added.filter((x) => x !== a))) }, 'Take out')
            )
          )
        )
      : null
  );
}

function openAddSheet(divisionIndex, names) {
  const sheet = $('#sheet');
  sheet.replaceChildren(
    h(
      'form',
      {
        method: 'dialog',
        onsubmit: (e) => {
          const data = new FormData(e.target);
          const name = E.cleanName(data.get('name'));
          if (!name) return e.preventDefault();
          const exists = movement().divisions.some((d) => d.teams.some((t) => t.name.toLowerCase() === name.toLowerCase()));
          if (exists) {
            e.preventDefault();
            return toast(`${name} are already in the league.`, 'bad');
          }
          changeMovement(() => {
            state.withdrawn = state.withdrawn.filter((n) => n.toLowerCase() !== name.toLowerCase());
            state.added.push({ name, division: Number(data.get('division')) });
          });
        },
      },
      h('header', {}, h('h3', {}, 'Add a team'), h('button', { type: 'button', class: 'x', 'aria-label': 'Close', onclick: () => sheet.close() }, '×')),
      h('p', { class: 'muted' }, 'Use the club name and a letter, the way the league lists teams: “Saltdean E”, “Rookery”. The club cap is checked as soon as the team is in.'),
      h('label', { class: 'field' }, h('span', {}, 'Team name'), h('input', { name: 'name', placeholder: 'e.g. Rookery', maxlength: 40, autocomplete: 'off', required: true })),
      h('label', { class: 'field' }, h('span', {}, 'Division'), h('select', { name: 'division' }, names.map((n, i) => h('option', { value: i, selected: i === divisionIndex }, n)))),
      h('div', { class: 'sheet-actions' }, h('button', { type: 'submit', class: 'btn primary small' }, 'Add team'))
    )
  );
  sheet.showModal();
  sheet.querySelector('input[name="name"]').focus();
}

function openRemoveSheet(mv, names) {
  const sheet = $('#sheet');
  const remove = (team) => () => {
    sheet.close();
    changeMovement(() => {
      if (team.movement === 'new') state.added = state.added.filter((a) => E.cleanName(a.name) !== team.name);
      else {
        state.withdrawn.push(team.name);
        delete state.manual[team.name];
      }
    });
  };
  // Bottom division first: that is where teams usually come and go.
  const order = mv.divisions.map((d, i) => [d, i]).reverse();
  sheet.replaceChildren(
    h(
      'form',
      { method: 'dialog' },
      h('header', {}, h('h3', {}, 'Remove a team'), h('button', { type: 'submit', class: 'x', 'aria-label': 'Close' }, '×')),
      h('p', { class: 'muted' }, 'Pick the team that will not enter next season. You can put it back afterwards.'),
      h(
        'div',
        { class: 'remove-list' },
        order.map(([d, i]) =>
          h(
            'div',
            {},
            h('p', { class: 'label' }, names[i]),
            h('div', { class: 'remove-teams' }, d.teams.map((t) => h('button', { type: 'button', class: 'chip', onclick: remove(t) }, clubDot(t.club), t.name)))
          )
        )
      )
    )
  );
  sheet.showModal();
}

function openTeamSheet(team, divisionIndex, mv) {
  const sheet = $('#sheet');
  const names = state.tables.divisions.map((d) => d.name);
  const close = () => sheet.close();
  const act = (fn) => () => {
    close();
    changeMovement(fn);
  };
  const cap = rules().maxPerClub;
  const isNew = team.movement === 'new';

  sheet.replaceChildren(
    h(
      'form',
      { method: 'dialog' },
      h('header', {}, h('h3', {}, clubDot(team.club), team.name), h('button', { type: 'submit', class: 'x', 'aria-label': 'Close' }, '×')),
      h('p', { class: 'muted' }, isNew ? 'New team this season.' : `Finished ${ordinal(team.previousPosition)} in ${names[team.from]}. Currently placed in ${names[divisionIndex]}.`),
      h('p', { class: 'label' }, 'Committee decision: place in'),
      h(
        'div',
        { class: 'choices' },
        names.map((n, i) => {
          const there = mv.divisions[i].teams.filter((t) => t.club === team.club && t.name !== team.name).length;
          return h(
            'button',
            {
              type: 'button',
              class: `choice ${i === divisionIndex ? 'on' : ''}`,
              disabled: i === divisionIndex,
              onclick: act(() => {
                if (isNew) state.added = state.added.map((a) => (E.cleanName(a.name) === team.name ? { ...a, division: i } : a));
                else state.manual = { ...state.manual, [team.name]: i };
              }),
            },
            h('strong', {}, n),
            h('span', {}, i === divisionIndex ? 'current' : there >= cap ? `would make ${there + 1} ${team.club} teams` : '')
          );
        })
      ),
      h(
        'div',
        { class: 'sheet-actions' },
        team.manual ? h('button', { type: 'button', class: 'btn small', onclick: act(() => delete state.manual[team.name]) }, 'Back to where the rules put them') : null,
        team.movement === 'promoted' && !team.manual
          ? h('button', { type: 'button', class: 'btn small', onclick: act(() => state.declined.push(team.name)) }, 'They decline promotion')
          : null,
        team.declined ? h('button', { type: 'button', class: 'btn small', onclick: act(() => (state.declined = state.declined.filter((n) => n !== team.name))) }, 'They accept promotion after all') : null,
        isNew
          ? h('button', { type: 'button', class: 'btn small danger', onclick: act(() => (state.added = state.added.filter((a) => E.cleanName(a.name) !== team.name))) }, 'Remove this team')
          : h(
              'button',
              {
                type: 'button',
                class: 'btn small danger',
                onclick: act(() => {
                  state.withdrawn.push(team.name);
                  delete state.manual[team.name];
                }),
              },
              'Remove from the league'
            )
      )
    )
  );
  sheet.showModal();
}

// ---------------------------------------------------------------------------
// Step 3: fixtures
// ---------------------------------------------------------------------------

function inputsKey(divisions) {
  return JSON.stringify([divisions.map((d) => [d.name, d.playDay, d.teams.map((t) => t.name)]), state.season, state.seed]);
}

function generate() {
  const divisions = nextDivisions();
  const out = E.generateFixtures(divisions, {
    year: state.season.year,
    week1Monday: state.season.week1Monday,
    derbiesEarly: state.season.derbiesEarly,
    dateOverrides: state.season.dateOverrides,
    seed: state.seed,
  });
  state.result = { divisions: out.divisions, stats: out.stats };
  state.resultKey = inputsKey(divisions);
}

function ensureResult() {
  if (state.loaded) return true;
  const key = inputsKey(nextDivisions());
  if (state.result && state.resultKey === key) return true;
  return false;
}

function drawFixtures(stage) {
  stage.append(
    h('div', { class: 'loading draw' }, h('span', { class: 'ball big' }), h('p', {}, 'Drawing the fixtures…'), h('p', { class: 'muted' }, 'Balancing home and away across every club'))
  );
  setActions();
  const started = performance.now();
  // Let the loader paint before the (synchronous) search runs.
  setTimeout(() => {
    try {
      generate();
    } catch (err) {
      toast(err.message || 'Could not generate fixtures.', 'bad');
      state.step = 1;
      render();
      return;
    }
    const wait = reduceMotion() ? 0 : Math.max(0, 650 - (performance.now() - started));
    setTimeout(() => {
      persist();
      render();
      document.body.classList.add('dealing');
      setTimeout(() => document.body.classList.remove('dealing'), 2600);
    }, wait);
  }, 40);
}

/** Group a division's fixtures into the cards of a fixture card: week w beside week w + half. */
function fixtureCards(division, weeks) {
  const half = weeks / 2;
  const cards = new Map();
  division.fixtures.forEach((f) => {
    const first = f.week <= half;
    const key = `${first ? f.week : f.week - half}|${f.extra ? 1 : 0}`;
    if (!cards.has(key)) cards.set(key, { week: first ? f.week : f.week - half, extra: !!f.extra, rows: [], firstDate: '', secondDate: '' });
    const card = cards.get(key);
    if (first) {
      card.firstDate = f.date;
      card.rows.push({ home: f.home, away: f.away });
    } else {
      card.secondDate = f.date;
    }
  });
  // With an odd number of teams, one team has no match in each round.
  const everyone = division.teams.map((t) => t.name);
  cards.forEach((card) => {
    const playing = new Set(card.rows.flatMap((r) => [r.home, r.away]));
    card.resting = everyone.filter((name) => !playing.has(name));
  });
  return [...cards.values()].sort((a, b) => (a.firstDate < b.firstDate ? -1 : a.firstDate > b.firstDate ? 1 : 0));
}

function editDate(division, week, extra, current) {
  if (state.loaded || state.accepted) return;
  const sheet = $('#sheet');
  const key = `${division.name}|${week}|${extra ? 1 : 0}`;
  sheet.replaceChildren(
    h(
      'form',
      {
        method: 'dialog',
        onsubmit: (e) => {
          const value = new FormData(e.target).get('date');
          if (e.submitter?.value === 'reset') delete state.season.dateOverrides[key];
          else if (value) state.season.dateOverrides[key] = value;
          else return;
          state.result = null;
          persist();
          render();
        },
      },
      h('header', {}, h('h3', {}, `${division.name} · Week ${week}${extra ? ' (extra round)' : ''}`), h('button', { type: 'submit', class: 'x', value: 'cancel', formnovalidate: true, 'aria-label': 'Close', onclick: (e) => { e.preventDefault(); sheet.close(); } }, '×')),
      h('p', { class: 'muted' }, 'Move this round to another date, for a bank holiday or a courts booking. The fixtures are redrawn around it.'),
      h('label', { class: 'field' }, h('span', {}, 'Date'), h('input', { type: 'date', name: 'date', value: current, required: true })),
      h(
        'div',
        { class: 'sheet-actions' },
        h('button', { type: 'submit', class: 'btn primary small', value: 'save' }, 'Change date'),
        state.season.dateOverrides[key] ? h('button', { type: 'submit', class: 'btn small', value: 'reset', formnovalidate: true }, 'Back to the usual day') : null
      )
    )
  );
  sheet.showModal();
}

function renderCards(division, weeks, opts = {}) {
  const half = weeks / 2;
  const cards = fixtureCards(division, weeks);
  const clubs = new Map(division.teams.map((t) => [t.name, t.club || E.clubOf(t.name)]));
  const lit = (name) => (opts.team ? name === opts.team : opts.club ? clubs.get(name) === opts.club : false);
  const dim = !!(opts.team || opts.club);
  let n = 0;
  return h(
    'div',
    { class: `cards ${dim ? 'filtered' : ''}` },
    cards.map((card, ci) =>
      h(
        'article',
        { class: `card ${card.extra ? 'extra' : ''}`, style: `--c:${ci}` },
        h(
          'header',
          {},
          h(
            opts.editable ? 'button' : 'div',
            { class: 'wk', type: opts.editable ? 'button' : null, title: opts.editable ? 'Change this date' : null, onclick: opts.editable ? () => editDate(division, card.week, card.extra, card.firstDate) : null },
            h('span', { class: 'wk-n' }, `Wk ${card.week}`),
            h('span', { class: 'wk-d' }, shortDate(card.firstDate))
          ),
          card.extra ? h('span', { class: 'extra-tag' }, 'Extra round') : h('span', { class: 'mirror', 'aria-hidden': 'true' }, '⇄'),
          h(
            opts.editable ? 'button' : 'div',
            { class: 'wk right', type: opts.editable ? 'button' : null, title: opts.editable ? 'Change this date' : null, onclick: opts.editable ? () => editDate(division, card.week + half, card.extra, card.secondDate) : null },
            h('span', { class: 'wk-n' }, `Wk ${card.week + half}`),
            h('span', { class: 'wk-d' }, shortDate(card.secondDate))
          )
        ),
        h(
          'ul',
          {},
          card.rows.map((row) =>
            h(
              'li',
              { class: `fx ${lit(row.home) || lit(row.away) ? 'lit' : ''} ${clubs.get(row.home) === clubs.get(row.away) ? 'derby' : ''}`, style: `--n:${n++}` },
              h('span', { class: `t home ${lit(row.home) ? 'me' : ''}` }, row.home),
              h('span', { class: 'v' }, 'v'),
              h('span', { class: `t away ${lit(row.away) ? 'me' : ''}` }, row.away)
            )
          ),
          card.resting.length
            ? h('li', { class: `fx rest ${card.resting.some(lit) ? 'lit' : ''}`, style: `--n:${n++}` }, h('span', { class: 'rest-l' }, 'No match'), h('span', { class: `rest-t ${card.resting.some(lit) ? 'me' : ''}` }, card.resting.join(', ')))
            : null
        )
      )
    )
  );
}

function teamSchedule(result, teamName) {
  const games = [];
  result.divisions.forEach((d) => {
    d.fixtures.forEach((f) => {
      if (f.home === teamName) games.push({ ...f, venue: 'H', opponent: f.away });
      else if (f.away === teamName) games.push({ ...f, venue: 'A', opponent: f.home });
    });
  });
  games.sort((a, b) => (a.date < b.date ? -1 : 1));
  return h(
    'section',
    { class: 'team-schedule' },
    h('h3', {}, `${teamName}: the season`),
    h(
      'ol',
      {},
      games.map((g, i) =>
        h(
          'li',
          { style: `--n:${i}` },
          h('span', { class: 'wk-n' }, `Wk ${g.week}`),
          h('span', { class: 'wk-d' }, shortDate(g.date)),
          h('span', { class: `ha ${g.venue}` }, g.venue === 'H' ? 'Home' : 'Away'),
          h('span', { class: 'opp' }, g.opponent)
        )
      )
    )
  );
}

function qualityTiles(result, weeks) {
  const s = result.stats;
  const perTeam = s.teams.length ? s.teams.every((t) => t.home === t.away) : false;
  const lastDerby = s.derbies.reduce((m, d) => Math.max(m, d.week), 0);
  const doubles = result.divisions
    .map((d) => {
      const extra = [...new Set(d.fixtures.filter((f) => f.extra).map((f) => f.week))].sort((a, b) => a - b);
      const odd = d.teams.length % 2 === 1;
      const parts = [];
      if (extra.length) parts.push(`weeks ${listNames(extra.map(String))} carry an extra round`);
      if (odd) parts.push('each team has one round off in each half');
      return parts.length ? `${d.name}: ${d.teams.length} teams, so ${parts.join(' and ')}` : null;
    })
    .filter(Boolean);
  const tile = (value, label, detail, tone) => {
    const v = h('strong', {}, typeof value === 'number' ? '0' : value);
    if (typeof value === 'number') countUp(v, value);
    return h('div', { class: `tile ${tone || ''}` }, v, h('span', { class: 'tile-l' }, label), detail ? h('span', { class: 'tile-d' }, detail) : null);
  };
  return h(
    'section',
    { class: 'quality', 'aria-label': 'Quality checks' },
    tile(s.fixtures, 'fixtures', `${result.divisions.length} divisions, ${weeks} weeks`),
    tile(s.clashes.length, s.clashes.length === 1 ? 'crowded night' : 'crowded nights', 'Nights a club hosts more than half the teams it has out', s.clashes.length ? 'warn' : 'good'),
    tile(perTeam ? '✓' : '!', 'home and away', perTeam ? 'Every team plays each opponent home and away' : 'Some teams are uneven', perTeam ? 'good' : 'warn'),
    tile(s.longestRun, 'in a row at most', 'Longest run of home, or of away, matches', s.longestRun > 3 ? 'warn' : 'good'),
    s.derbies.length ? tile(`Wk ${lastDerby}`, 'derbies done by', `${s.derbies.length} same-club matches played early in each half`, lastDerby > 5 ? 'warn' : 'good') : null,
    doubles.length ? h('p', { class: 'doubles' }, doubles.join('. ') + '.') : null
  );
}

function renderFixtures(stage) {
  if (!ensureResult()) return drawFixtures(stage);
  const result = state.result;
  const loaded = state.loaded;
  const weeks = loaded ? loaded.file.season.weeks || 18 : 18;
  const year = loaded ? loaded.file.season.year : state.season.year;
  const locked = !!loaded || !!state.accepted;
  state.division = Math.min(state.division, result.divisions.length - 1);
  const division = result.divisions[state.division];

  stage.append(
    h(
      'section',
      { class: 'intro' },
      h('p', { class: 'eyebrow' }, loaded ? `Loaded from ${loaded.name}` : `${year} season · fixtures · draw ${state.draw || 1}`),
      h('h2', {}, loaded ? `${year} fixtures${loaded.file.status === 'accepted' ? ', accepted' : ' (draft)'}` : `${result.stats.fixtures} fixtures, drawn.`),
      loaded
        ? h(
            'p',
            { class: 'lede' },
            h('span', { class: `pill ${loaded.intact ? 'ok' : 'warn'}` }, loaded.intact ? 'Unaltered' : 'Edited by hand'),
            ' ',
            loaded.intact ? 'This file is exactly as the generator wrote it, and every check below passes.' : 'This file has been changed since it was generated. It still passes every check below.'
          )
        : h('p', { class: 'lede' }, 'Each pair meets in the first nine weeks and again, venues swapped, in the second nine. Every draw is different: press Draw again for another that passes the same checks.')
    )
  );

  stage.append(qualityTiles(result, weeks));

  if (!locked) stage.append(renderSeasonPanel());

  // Division tabs
  stage.append(
    h(
      'div',
      { class: 'tabs', role: 'tablist', 'aria-label': 'Division' },
      result.divisions.map((d, i) =>
        h(
          'button',
          {
            type: 'button',
            role: 'tab',
            'aria-selected': i === state.division ? 'true' : 'false',
            class: i === state.division ? 'on' : '',
            onclick: () => {
              state.division = i;
              state.team = null;
              go(2, { keepScroll: true });
            },
          },
          d.name,
          h('span', { class: 'tab-day' }, (d.playDay || '').slice(0, 3))
        )
      )
    )
  );

  // Club and team filter
  const clubs = [...new Set(result.divisions.flatMap((d) => d.teams.map((t) => t.club || E.clubOf(t.name))))].sort();
  const teamsOfClub = state.club ? result.divisions.flatMap((d) => d.teams.filter((t) => (t.club || E.clubOf(t.name)) === state.club).map((t) => ({ name: t.name, division: d.name }))) : [];
  stage.append(
    h(
      'div',
      { class: 'filter' },
      h('span', { class: 'filter-l' }, 'Follow a club'),
      h(
        'div',
        { class: 'chips' },
        clubs.map((c) =>
          h(
            'button',
            {
              type: 'button',
              class: `chip ${state.club === c ? 'on' : ''}`,
              'aria-pressed': state.club === c ? 'true' : 'false',
              onclick: () => {
                state.club = state.club === c ? null : c;
                state.team = null;
                go(2, { keepScroll: true });
              },
            },
            clubDot(c),
            c
          )
        )
      ),
      teamsOfClub.length
        ? h(
            'div',
            { class: 'chips teams' },
            teamsOfClub.map((t) =>
              h(
                'button',
                {
                  type: 'button',
                  class: `chip ${state.team === t.name ? 'on' : ''}`,
                  'aria-pressed': state.team === t.name ? 'true' : 'false',
                  onclick: () => {
                    state.team = state.team === t.name ? null : t.name;
                    const di = result.divisions.findIndex((d) => d.name === t.division);
                    if (state.team && di >= 0) state.division = di;
                    go(2, { keepScroll: true });
                  },
                },
                t.name,
                h('span', { class: 'chip-d' }, t.division.replace('Division ', 'Div '))
              )
            )
          )
        : null
    )
  );

  if (state.team) stage.append(teamSchedule(result, state.team));

  stage.append(
    h('p', { class: 'card-key' }, 'Home team on the left for the first date, on the right for the second.', locked ? '' : ' Tap a date to move it.'),
    renderCards(division, weeks, { club: state.club, team: state.team, editable: !locked })
  );

  if (result.stats.clashes.length) {
    stage.append(
      h(
        'details',
        { class: 'panel' },
        h('summary', {}, `${result.stats.clashes.length} crowded ${result.stats.clashes.length === 1 ? 'night' : 'nights'}`),
        h('ul', { class: 'plain' }, result.stats.clashes.map((c) => h('li', {}, `${c.club}, ${shortDate(c.date)}: ${c.matches.join(' and ')}`)))
      )
    );
  }

  if (loaded) {
    setActions(
      h('button', { type: 'button', class: 'btn ghost', onclick: closeLoaded }, 'Close file'),
      h('button', { type: 'button', class: 'btn primary', onclick: () => go(3) }, 'Share or print', h('span', { class: 'arrow' }, '→'))
    );
  } else if (state.accepted) {
    setActions(
      h(
        'button',
        {
          type: 'button',
          class: 'btn ghost',
          onclick: () => {
            state.accepted = null;
            persist();
            render();
          },
        },
        'Reopen the draw'
      ),
      h('button', { type: 'button', class: 'btn primary', onclick: () => go(3) }, 'Share', h('span', { class: 'arrow' }, '→'))
    );
  } else {
    setActions(
      h('button', { type: 'button', class: 'btn ghost', onclick: shuffle }, h('span', { class: 'dice', 'aria-hidden': 'true' }, '↻'), 'Draw again'),
      h('button', { type: 'button', class: 'btn primary', onclick: accept }, 'Accept these fixtures', h('span', { class: 'arrow' }, '✓'))
    );
  }
}

function renderSeasonPanel() {
  const divisions = nextDivisions();
  const change = (fn) => (e) => {
    fn(e.target);
    state.result = null;
    persist();
    render();
  };
  const overrides = Object.keys(state.season.dateOverrides).length;
  return h(
    'details',
    { class: 'panel' },
    h('summary', {}, `Season dates: week 1 starts ${shortDate(state.season.week1Monday)} ${state.season.year}`),
    h(
      'div',
      { class: 'fields' },
      h(
        'label',
        {},
        h('span', {}, 'Week 1 begins (Monday)'),
        h('input', {
          type: 'date',
          value: state.season.week1Monday,
          onchange: change((el) => {
            if (!el.value) return;
            // Snap to the Monday of the chosen week.
            const d = E.parseISO(el.value);
            const back = (d.getUTCDay() + 6) % 7;
            state.season.week1Monday = E.addDays(el.value, -back);
            state.season.dateOverrides = {};
          }),
        })
      ),
      divisions.map((d) =>
        h(
          'label',
          {},
          h('span', {}, `${d.name} plays on`),
          h(
            'select',
            { onchange: change((el) => (state.season.playDays = { ...state.season.playDays, [d.name]: el.value })) },
            E.DAYS.map((day) => h('option', { value: day, selected: day === d.playDay }, day))
          )
        )
      ),
      h(
        'label',
        { class: 'check' },
        h('input', { type: 'checkbox', checked: state.season.derbiesEarly, onchange: change((el) => (state.season.derbiesEarly = el.checked)) }),
        h('span', {}, 'Play same-club derbies in the first weeks of each half')
      )
    ),
    overrides
      ? h(
          'button',
          {
            type: 'button',
            class: 'link',
            onclick: change(() => (state.season.dateOverrides = {})),
          },
          `Undo ${overrides} moved ${overrides === 1 ? 'date' : 'dates'}`
        )
      : null
  );
}

/** A fresh seed: every draw is a different, equally valid, set of fixtures. */
function newDraw() {
  let seed = state.seed;
  while (seed === state.seed) seed = Math.floor(Math.random() * 2 ** 31) + 1;
  state.seed = seed;
  state.draw = (state.draw || 0) + 1;
  state.result = null;
}

function shuffle() {
  newDraw();
  persist();
  render();
  window.scrollTo({ top: 0, behavior: reduceMotion() ? 'auto' : 'smooth' });
}

function accept() {
  state.accepted = new Date().toISOString();
  persist();
  celebrate();
  go(3);
}

// ---------------------------------------------------------------------------
// Step 4: share
// ---------------------------------------------------------------------------

function currentFile() {
  if (state.loaded) return state.loaded.file;
  const mv = movement();
  const names = state.tables.divisions.map((d) => d.name);
  return E.buildFile({
    league: state.tables.league,
    season: { year: state.season.year, weeks: 18, week1Monday: state.season.week1Monday },
    divisions: state.result.divisions,
    generator: { seed: state.seed, basedOn: { season: state.tables.season, source: state.tables.source || null } },
    status: state.accepted ? 'accepted' : 'draft',
    acceptedAt: state.accepted,
    decisions: mv.decisions.map((d) => describe(d, names)).filter(Boolean).map((x) => `${x.title}. ${x.body}`),
    memorial: 'In memory of Mick Cox, who made the Parks League fixtures.',
  });
}

function fileBase(file) {
  return `${slug(file.league?.shortName || file.league?.name)}-${file.season.year}-fixtures`;
}

function renderShare(stage) {
  const file = currentFile();
  const accepted = file.status === 'accepted';
  const fixtures = file.divisions.reduce((n, d) => n + d.fixtures.length, 0);
  const teams = file.divisions.reduce((n, d) => n + d.teams.length, 0);

  stage.append(
    h(
      'section',
      { class: 'intro share-intro' },
      accepted ? h('div', { class: 'stamp', 'aria-hidden': 'true' }, 'Accepted') : null,
      h('p', { class: 'eyebrow' }, `${file.season.year} season`),
      h('h2', {}, accepted ? 'The fixtures are ready to send out.' : 'Share this draft.'),
      h('p', { class: 'lede' }, `${fixtures} fixtures for ${teams} teams in ${file.divisions.length} divisions, in one file that any club's jim.tennis can load.`)
    ),
    h(
      'div',
      { class: 'share-grid' },
      h(
        'article',
        { class: 'share-card main' },
        h('h3', {}, 'Fixture file'),
        h('p', {}, 'The whole season in one small file. Email it to club secretaries; each club loads it into their own jim.tennis, or opens it here to browse and print.'),
        h('button', { type: 'button', class: 'btn primary', onclick: () => download(`${fileBase(file)}.jimtennis.json`, JSON.stringify(file, null, 2) + '\n', 'application/json') }, 'Download fixture file'),
        h('p', { class: 'muted small' }, `Check code ${file.checksum}. A loaded file shows “Unaltered” when it matches.`)
      ),
      h(
        'article',
        { class: 'share-card' },
        h('h3', {}, 'Fixture cards'),
        h('p', {}, 'One A4 page per division, laid out like the cards clubs are used to.'),
        h('button', { type: 'button', class: 'btn', onclick: printCards }, 'Print or save as PDF')
      ),
      h(
        'article',
        { class: 'share-card' },
        h('h3', {}, 'All divisions on one sheet'),
        h('p', {}, 'Every division side by side on a single large sheet. It is bigger than A4: save it as a PDF, then print it large or scaled to fit.'),
        h('button', { type: 'button', class: 'btn', onclick: printPoster }, 'Print the big sheet')
      ),
      h(
        'article',
        { class: 'share-card' },
        h('h3', {}, 'Spreadsheet'),
        h('p', {}, 'Every fixture as a row: division, week, date, home, away.'),
        h('button', { type: 'button', class: 'btn', onclick: () => download(`${fileBase(file)}.csv`, E.toCSV(file.divisions), 'text/csv') }, 'Download CSV')
      ),
      h(
        'article',
        { class: 'share-card' },
        h('h3', {}, 'Load into a club'),
        h('p', {}, 'A club admin creates the new season, then chooses the fixture file under Seasons → Setup → Import. Teams, divisions and all 18 weeks arrive in one go.'),
        h('a', { class: 'btn', href: '/admin/league/seasons' }, 'Open Seasons (admin login)')
      )
    )
  );

  setActions(
    h('button', { type: 'button', class: 'btn ghost', onclick: () => go(2) }, 'Back to fixtures'),
    state.loaded ? h('button', { type: 'button', class: 'btn', onclick: closeLoaded }, 'Close file') : null
  );
}

/** The @page rule for the next print: A4 for the cards, a made-to-measure sheet for the poster. */
function setPageRule(css) {
  let el = $('#print-page-style');
  if (!el) {
    el = h('style', { id: 'print-page-style' });
    document.head.append(el);
  }
  el.textContent = css;
}

const A4 = { width: 210, height: 297, margin: 13 };
const PRINT_HEADER_MM = 25; // league, title and division lines, with space beneath
const PRINT_FOOTER_MM = 9;

/**
 * Row height that lets a division's cards fill one A4 page, two cards across.
 * A card is a one-line heading plus one line per match (and one for the team
 * with no match). Below about 6pt the page stops being readable, so very
 * large divisions run onto a second page instead, breaking between cards.
 */
function printDensity(division, weeks) {
  const cards = fixtureCards(division, weeks);
  const rows = Math.ceil(cards.length / 2);
  const lines = Math.max(1, ...cards.map((c) => c.rows.length + (c.resting.length ? 1 : 0)));
  const gap = 3.2;
  const usable = A4.height - 2 * A4.margin - PRINT_HEADER_MM - PRINT_FOOTER_MM - 1.5; // a little slack for rounding
  const perCard = (usable - (rows - 1) * gap) / Math.max(1, rows);
  const chrome = 6.8; // heading padding, list padding and borders
  const line = Math.min(5.8, Math.max(2.9, (perCard - chrome) / (lines + 1)));
  return `--pl:${line.toFixed(2)}mm;--pf:${(line * 0.72).toFixed(2)}mm;--pg:${gap}mm`;
}

function printHeader(file, title) {
  return h('header', {}, h('p', {}, file.league?.name || ''), h('h2', {}, `Fixtures ${file.season.year}`), h('h3', {}, title));
}

function printFooter(file, weeks) {
  return h(
    'footer',
    {},
    h('span', {}, `Wks 1–${weeks / 2}: home team on left. Wks ${weeks / 2 + 1}–${weeks}: home team on right.`),
    h('span', { class: 'draft' }, file.status === 'accepted' ? '' : 'DRAFT'),
    h('span', { class: 'memorial' }, 'In memory of Mick Cox, who made the Parks League fixtures')
  );
}

/** One A4 page per division. */
function printCards() {
  const file = currentFile();
  const weeks = file.season.weeks || 18;
  setPageRule(`@page { size: A4 portrait; margin: ${A4.margin}mm; }`);
  const root = $('#print-root');
  root.className = 'print-root';
  root.replaceChildren(
    ...file.divisions.map((d) =>
      h('section', { class: 'print-page', style: printDensity(d, weeks) }, printHeader(file, `${d.name} · ${d.playDay || ''}s`), renderCards(d, weeks), printFooter(file, weeks))
    )
  );
  window.print();
}

/**
 * Every division side by side on one sheet, as large as it needs to be. The
 * sheet is laid out in millimetres, measured, and the page is made that size,
 * so it saves as a single-page PDF to print big or scaled to fit.
 */
function printPoster() {
  const file = currentFile();
  const weeks = file.season.weeks || 18;
  const column = 100;
  const gap = 11;
  const margin = 16;
  const width = file.divisions.length * column + (file.divisions.length - 1) * gap;

  const root = $('#print-root');
  root.replaceChildren(
    h(
      'section',
      { class: 'poster', style: `width:${width}mm;--col:${column}mm;--gap:${gap}mm` },
      printHeader(file, 'All divisions'),
      h(
        'div',
        { class: 'poster-grid' },
        file.divisions.map((d) => h('section', { class: 'poster-division' }, h('h3', {}, d.name, h('span', {}, `${d.playDay || ''}s · ${d.teams.length} teams`)), renderCards(d, weeks)))
      ),
      printFooter(file, weeks)
    )
  );
  // Measure it laid out exactly as it will print.
  root.className = 'print-root measuring';
  const px = root.firstElementChild.getBoundingClientRect().height;
  root.className = 'print-root';
  const height = Math.ceil((px * 25.4) / 96) + 2;
  setPageRule(`@page { size: ${width + 2 * margin}mm ${height + 2 * margin}mm; margin: ${margin}mm; }`);
  window.print();
}

// ---------------------------------------------------------------------------
// Loading a fixture file
// ---------------------------------------------------------------------------

async function openFile(blob) {
  try {
    if (blob.size > 5 * 1024 * 1024) throw new Error('That file is too large to be a fixture file.');
    const { file, stats, intact } = E.parseFile(await blob.text());
    state.loaded = { name: blob.name || 'fixture file', file, intact };
    state.result = { divisions: file.divisions, stats };
    state.division = 0;
    state.club = null;
    state.team = null;
    state.step = 2;
    render();
    document.body.classList.add('dealing');
    setTimeout(() => document.body.classList.remove('dealing'), 2600);
    window.scrollTo({ top: 0 });
    toast(`Loaded ${file.season.year} fixtures.`);
  } catch (err) {
    toast(err.message || "That file couldn't be read.", 'bad');
  }
}

function closeLoaded() {
  state.loaded = null;
  state.result = null;
  state.club = null;
  state.team = null;
  state.step = state.tables ? 0 : 0;
  render();
  window.scrollTo({ top: 0 });
}

function wireFileLoading() {
  const input = $('#load-file-input');
  $('#load-file-btn').addEventListener('click', () => input.click());
  input.addEventListener('change', () => {
    if (input.files[0]) openFile(input.files[0]);
    input.value = '';
  });
  const veil = $('#drop-veil');
  let depth = 0;
  const hasFiles = (e) => [...(e.dataTransfer?.types || [])].includes('Files');
  window.addEventListener('dragenter', (e) => {
    if (!hasFiles(e)) return;
    depth++;
    veil.classList.add('show');
  });
  window.addEventListener('dragleave', () => {
    depth = Math.max(0, depth - 1);
    if (!depth) veil.classList.remove('show');
  });
  window.addEventListener('dragover', (e) => hasFiles(e) && e.preventDefault());
  window.addEventListener('drop', (e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    depth = 0;
    veil.classList.remove('show');
    if (e.dataTransfer.files[0]) openFile(e.dataTransfer.files[0]);
  });
}

// ---------------------------------------------------------------------------
// Render and boot
// ---------------------------------------------------------------------------

function render() {
  const stage = $('#stage');
  stage.replaceChildren();
  stage.dataset.step = state.step;
  renderShell();
  if (state.step === 0) renderTables(stage);
  else if (state.step === 1) renderMovement(stage);
  else if (state.step === 2) renderFixtures(stage);
  else renderShare(stage);
}

async function boot() {
  wireFileLoading();
  const restored = restore();
  if (restored && state.step === 2) state.step = 1; // fixtures are redrawn on demand
  render();
  if (!restored) {
    try {
      const { tables, note } = await loadTables(false);
      setTables(tables, note);
      persist();
    } catch {
      $('#stage').replaceChildren(
        h('div', { class: 'banner bad', role: 'alert' }, h('strong', {}, "The final tables couldn't be loaded."), h('p', {}, 'Check your connection and reload, or load a fixture file instead.'))
      );
      return;
    }
    render();
  }
}

boot();
