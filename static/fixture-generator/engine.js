// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
//
// Fixture generator engine. Pure logic, no DOM: it runs in the browser and
// under `node --test` (tests/fixture-generator).
//
//   final tables ──resolveMovement──▶ next season's divisions
//                ──generateFixtures──▶ an 18-week double round robin
//                ──buildFile/parseFile──▶ the shareable fixture file

export const FILE_FORMAT = 'jim.tennis/fixtures';
export const FILE_VERSION = 1;
export const ENGINE_VERSION = 1;

export const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

export const DEFAULT_RULES = {
  promote: 2, // teams promoted from each division except the top
  relegate: 2, // teams relegated from each division except the bottom
  maxPerClub: 2, // BHPLTA rule 2(b): no more than two teams from one club per division
  // How many places below the automatic spots a promotion offer may pass.
  // 1 is rule 2(b) as written ("the third team shall be offered"); 'any' has no limit.
  offerDepth: 2,
};

/** All k-element subsets of items, best-first: earlier items are preferred. */
function* combinations(items, k, start = 0, picked = []) {
  if (picked.length === k) {
    yield picked.slice();
    return;
  }
  for (let i = start; i <= items.length - (k - picked.length); i++) {
    picked.push(items[i]);
    yield* combinations(items, k, i + 1, picked);
    picked.pop();
  }
}

// ---------------------------------------------------------------------------
// Names
// ---------------------------------------------------------------------------

/** Normalise curly apostrophes and whitespace so "St Ann’s" and "St Ann's" match. */
export function cleanName(name) {
  return String(name ?? '')
    .replace(/[‘’ʼ`´]/g, "'")
    .replace(/\s+/g, ' ')
    .trim();
}

/** "Dyke A" → "Dyke", "King Alfred B" → "King Alfred", "Park Avenue" → "Park Avenue". */
export function clubOf(teamName) {
  const name = cleanName(teamName);
  const m = name.match(/^(.*\S)\s+[A-Z]$/);
  return m ? m[1] : name;
}

// ---------------------------------------------------------------------------
// Deterministic randomness
// ---------------------------------------------------------------------------

export function mulberry32(seed) {
  let a = seed >>> 0;
  return function next() {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// ---------------------------------------------------------------------------
// Tables
// ---------------------------------------------------------------------------

function num(v) {
  const n = typeof v === 'number' ? v : parseFloat(v);
  return Number.isFinite(n) ? n : 0;
}

/**
 * Rank a division by points, then sets won minus sets lost (BHPLTA league
 * regulation 10). Teams still level after that keep their incoming order and
 * are flagged, because the rules call for a deciding match.
 */
export function rankDivision(teams) {
  const rows = teams.map((t, i) => ({
    ...t,
    name: cleanName(t.name),
    club: t.club ? cleanName(t.club) : clubOf(t.name),
    points: num(t.points),
    setsWon: num(t.setsWon),
    setsLost: num(t.setsLost),
    _order: i,
  }));
  rows.sort(
    (a, b) =>
      b.points - a.points || b.setsWon - b.setsLost - (a.setsWon - a.setsLost) || a._order - b._order
  );
  rows.forEach((row, i) => {
    const prev = rows[i - 1];
    row.position = i + 1;
    row.tied =
      !!prev && prev.points === row.points && prev.setsWon - prev.setsLost === row.setsWon - row.setsLost;
    if (row.tied) prev.tied = true;
    delete row._order;
  });
  return rows;
}

// ---------------------------------------------------------------------------
// Promotion and relegation
// ---------------------------------------------------------------------------

/**
 * Work out next season's divisions from the final tables.
 *
 * tables: { season, divisions: [{ name, level, playDay, teams: [...] }] } top division first.
 * options:
 *   rules      – overrides for DEFAULT_RULES
 *   declined   – team names that turn promotion down
 *   manual     – { teamName: divisionIndex } committee overrides, applied last
 *   withdrawn  – team names leaving the league
 *   added      – [{ name, club?, division? }] teams joining (bottom division by default)
 *
 * Returns { divisions, decisions, violations }.
 */
export function resolveMovement(tables, options = {}) {
  const rules = { ...DEFAULT_RULES, ...(options.rules || {}) };
  const declined = new Set([...(options.declined || [])].map(cleanName));
  const withdrawn = new Set([...(options.withdrawn || [])].map(cleanName));
  const manual = options.manual || {};
  const decisions = [];

  const src = tables.divisions.map((d, i) => ({
    name: d.name,
    level: d.level ?? i + 1,
    playDay: d.playDay || '',
    ranked: rankDivision(d.teams).filter((t) => {
      if (!withdrawn.has(t.name)) return true;
      decisions.push({ kind: 'withdrawn', team: t.name, from: i, position: t.position });
      return false;
    }),
  }));
  const count = src.length;
  // `position` is where a team finished and is what people are told; `place`
  // closes the gaps left by withdrawn teams and is what the rules work from.
  src.forEach((d) => d.ranked.forEach((t, i) => (t.place = i + 1)));

  src.forEach((d, i) => {
    const tied = d.ranked.filter((t) => t.tied).map((t) => t.name);
    if (tied.length) decisions.push({ kind: 'tie', division: i, teams: tied });
  });

  const entry = (t, from, movement, extra = {}) => ({
    name: t.name,
    club: t.club,
    from,
    previousPosition: t.position,
    movement,
    ...extra,
  });

  // Relegation zone of each division (never the bottom division).
  const zone = src.map((d, i) => {
    if (i === count - 1) return [];
    const size = Math.max(0, Math.min(rules.relegate, d.ranked.length - rules.promote));
    return size ? d.ranked.slice(-size) : [];
  });

  const promotedOut = src.map(() => new Set()); // names leaving upward
  const relegatedOut = src.map(() => new Set()); // names leaving downward
  const arrivals = src.map(() => []); // entries arriving in each division
  const flags = new Map(); // name → extra info for teams that stay

  const tally = (teams) => {
    const byClub = new Map();
    teams.forEach((t) => byClub.set(t.club, [...(byClub.get(t.club) || []), t.name]));
    return byClub;
  };
  const fits = (teams, movers) => {
    const byClub = tally(teams);
    return movers.every((t) => byClub.get(t.club).length <= rules.maxPerClub);
  };

  // Each boundary is settled top-down as an exchange of k teams: the k
  // best-placed teams able to go up swap with the k lowest-placed teams able
  // to go down, for the largest k (at most rules.promote) that keeps every
  // club within the cap in both divisions.
  for (let upper = 0; upper < count - 1; upper++) {
    const lower = upper + 1;
    const zoneNames = new Set(zone[upper].map((t) => t.name));
    const upperSettled = [
      ...src[upper].ranked.filter((t) => !promotedOut[upper].has(t.name) && !zoneNames.has(t.name)),
      ...arrivals[upper],
    ];
    const lowerZone = new Set(zone[lower].map((t) => t.name));
    const lowerSettled = src[lower].ranked.filter((t) => !lowerZone.has(t.name));

    const depth = rules.offerDepth === 'any' ? Infinity : Math.max(0, rules.offerDepth);
    const inReach = lowerSettled.filter((t) => t.place <= rules.promote + depth);
    const willing = inReach.filter((t) => !declined.has(t.name));
    const downOrder = zone[upper].slice().reverse(); // bottom team first

    let chosenUp = [];
    let chosenDown = [];
    search: for (let k = Math.min(rules.promote, willing.length, downOrder.length); k > 0; k--) {
      for (const downs of combinations(downOrder, k)) {
        const downNames = new Set(downs.map((t) => t.name));
        // Take the best-placed teams that fit; a team is passed over only
        // when its own club already fills the cap above.
        const upperNext = [...upperSettled, ...zone[upper].filter((t) => !downNames.has(t.name))];
        const ups = [];
        for (const t of willing) {
          if (ups.length === k) break;
          if (upperNext.filter((x) => x.club === t.club).length < rules.maxPerClub) {
            ups.push(t);
            upperNext.push(t);
          }
        }
        if (ups.length < k) continue;
        const upNames = new Set(ups.map((t) => t.name));
        const lowerNext = [...lowerSettled.filter((t) => !upNames.has(t.name)), ...downs];
        if (fits(lowerNext, downs)) {
          chosenUp = ups;
          chosenDown = downs;
          break search;
        }
      }
    }

    const upNames = new Set(chosenUp.map((t) => t.name));
    const downNames = new Set(chosenDown.map((t) => t.name));
    const upperClubs = tally([...upperSettled, ...zone[upper].filter((t) => !downNames.has(t.name))]);
    const lowerClubs = tally(lowerSettled.filter((t) => !upNames.has(t.name)));
    const lastUp = chosenUp.length ? chosenUp[chosenUp.length - 1].place : 0;

    // Explain every team in the promotion picture, in table order.
    let passedOver = null;
    inReach.forEach((t) => {
      const automatic = t.place <= rules.promote;
      const base = { team: t.name, from: lower, to: upper, position: t.position, automatic };
      if (upNames.has(t.name)) {
        promotedOut[lower].add(t.name);
        arrivals[upper].push(entry(t, lower, 'promoted', { offered: !automatic }));
        decisions.push({ ...base, kind: automatic ? 'promoted' : 'offered', insteadOf: automatic ? null : passedOver });
        return;
      }
      if (!automatic && t.place > lastUp) return; // never came into the picture
      if (declined.has(t.name)) {
        flags.set(t.name, { declined: true });
        decisions.push({ ...base, kind: 'declined' });
      } else if ((upperClubs.get(t.club) || []).length >= rules.maxPerClub) {
        flags.set(t.name, { blocked: true });
        decisions.push({ ...base, kind: 'blocked', club: t.club, already: upperClubs.get(t.club).slice() });
      } else if (automatic) {
        flags.set(t.name, { held: true });
        decisions.push({ ...base, kind: 'held' });
      } else {
        return;
      }
      passedOver = passedOver || t.name;
    });

    // ...and every team in the relegation zone.
    zone[upper].forEach((t) => {
      const base = { team: t.name, from: upper, to: lower, position: t.position };
      if (downNames.has(t.name)) {
        relegatedOut[upper].add(t.name);
        arrivals[lower].push(entry(t, upper, 'relegated'));
        decisions.push({ ...base, kind: 'relegated' });
      } else if ((lowerClubs.get(t.club) || []).length >= rules.maxPerClub) {
        flags.set(t.name, { spared: true });
        decisions.push({ ...base, kind: 'spared', club: t.club, already: lowerClubs.get(t.club).slice() });
      } else {
        flags.set(t.name, { reprieved: true });
        decisions.push({ ...base, kind: 'reprieved' });
      }
    });
  }

  // Assemble: relegated arrivals first (strongest), then those who stay, then promoted.
  const divisions = src.map((d, i) => {
    const stay = d.ranked
      .filter((t) => !promotedOut[i].has(t.name) && !relegatedOut[i].has(t.name))
      .map((t) => entry(t, i, 'stay', flags.get(t.name) || {}));
    const down = arrivals[i].filter((t) => t.movement === 'relegated');
    const up = arrivals[i].filter((t) => t.movement === 'promoted');
    return { name: d.name, level: d.level, playDay: d.playDay, teams: [...down, ...stay, ...up] };
  });

  // Teams joining the league.
  (options.added || []).forEach((t) => {
    const name = cleanName(t.name);
    if (!name) return;
    const target = Math.min(count - 1, Math.max(0, t.division ?? count - 1));
    divisions[target].teams.push({
      name,
      club: t.club ? cleanName(t.club) : clubOf(name),
      from: null,
      previousPosition: null,
      movement: 'new',
    });
    decisions.push({ kind: 'new', team: name, to: target });
  });

  // Committee overrides.
  Object.entries(manual).forEach(([rawName, target]) => {
    const name = cleanName(rawName);
    if (!(target >= 0 && target < count)) return;
    for (let i = 0; i < count; i++) {
      const idx = divisions[i].teams.findIndex((t) => t.name === name);
      if (idx === -1) continue;
      if (i === target) return;
      const [team] = divisions[i].teams.splice(idx, 1);
      divisions[target].teams.push({ ...team, manual: true, autoDivision: i });
      decisions.push({ kind: 'manual', team: name, from: i, to: target });
      return;
    }
  });

  return { divisions, decisions, violations: findViolations(divisions, rules) };
}

/** Problems that must be settled before fixtures can be generated. */
export function findViolations(divisions, rules = DEFAULT_RULES) {
  const out = [];
  const seen = new Map();
  divisions.forEach((d, i) => {
    const clubs = new Map();
    d.teams.forEach((t) => {
      clubs.set(t.club, [...(clubs.get(t.club) || []), t.name]);
      if (seen.has(t.name)) out.push({ kind: 'duplicate', team: t.name, division: i });
      seen.set(t.name, i);
    });
    clubs.forEach((teams, club) => {
      if (teams.length > rules.maxPerClub) out.push({ kind: 'club-cap', division: i, club, teams });
    });
    if (d.teams.length < 2) out.push({ kind: 'too-small', division: i, size: d.teams.length });
    if (d.teams.length > 19) out.push({ kind: 'too-big', division: i, size: d.teams.length });
  });
  return out;
}

// ---------------------------------------------------------------------------
// Dates
// ---------------------------------------------------------------------------

export function parseISO(iso) {
  const [y, m, d] = String(iso).split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d));
}

export function toISO(date) {
  return date.toISOString().slice(0, 10);
}

export function addDays(iso, days) {
  const d = parseISO(iso);
  d.setUTCDate(d.getUTCDate() + days);
  return toISO(d);
}

/** The league traditionally starts in the week of the second Monday of April. */
export function defaultWeek1Monday(year) {
  const d = new Date(Date.UTC(year, 3, 1));
  while (d.getUTCDay() !== 1) d.setUTCDate(d.getUTCDate() + 1);
  d.setUTCDate(d.getUTCDate() + 7);
  return toISO(d);
}

export function defaultPlayDay(level) {
  // BHPLTA league regulation 1: Div 1 Thursdays, Div 2 Wednesdays, Div 3 and 4 Tuesdays.
  return level === 1 ? 'Thursday' : level === 2 ? 'Wednesday' : 'Tuesday';
}

/**
 * Date of a slot. `extra` marks the second fixture of a double-header week.
 * Following established practice: an extra round in the opening week is played
 * a week early, one in the closing week a week late, and any other extra round
 * on the Saturday of that week.
 */
export function slotDate(config, division, week, extra) {
  const key = `${division.name}|${week}|${extra ? 1 : 0}`;
  if (config.dateOverrides && config.dateOverrides[key]) return config.dateOverrides[key];
  const day = Math.max(0, DAYS.indexOf(division.playDay || defaultPlayDay(division.level)));
  const regular = addDays(config.week1Monday, (week - 1) * 7 + day);
  if (!extra) return regular;
  if (week === 1) return addDays(regular, -7);
  if (week === config.weeks) return addDays(regular, 7);
  const saturday = addDays(config.week1Monday, (week - 1) * 7 + 5);
  return saturday === regular ? addDays(saturday, 1) : saturday;
}

// ---------------------------------------------------------------------------
// Fixture generation
// ---------------------------------------------------------------------------

// Which weeks of a half take a second round when a division has more rounds
// than weeks: the opening and closing weeks first, then working inwards.
function doubleWeekOrder(half) {
  const order = [];
  let lo = 1;
  let hi = half;
  while (lo <= hi) {
    order.push(lo);
    if (hi !== lo) order.push(hi);
    lo++;
    hi--;
  }
  return order;
}

/** Slots of the first half for a division needing `rounds` rounds. */
export function buildSlots(rounds, half) {
  if (rounds > half * 2) throw new Error(`${rounds} rounds cannot fit into ${half} weeks`);
  const slots = [];
  for (let w = 1; w <= Math.min(rounds, half); w++) slots.push({ week: w, extra: false });
  doubleWeekOrder(half)
    .slice(0, Math.max(0, rounds - half))
    .forEach((w) => slots.push({ week: w, extra: true }));
  return slots;
}

// Circle method with the classic minimum-break home/away pattern. n is even:
// seats 0..n-2 sit on the circle and seat n-1 stays put. In round r, seats i
// and j meet when i + j = r (mod n-1); the seat with 2i = r meets the fixed
// seat. Each pair is listed home first. Played in this order, every seat
// alternates home and away with at most one "break" (two the same in a row),
// which is the fewest a round robin allows.
function canonicalRounds(n) {
  const m = n - 1;
  const rounds = [];
  for (let r = 0; r < m; r++) {
    const pairs = [];
    for (let i = 0; i < m; i++) {
      const j = (((r - i) % m) + m) % m;
      if (i === j) pairs.push(r % 2 === 0 ? [i, m] : [m, i]);
      else if (i < j) pairs.push((((i - j) % m) + m) % m % 2 === 1 ? [i, j] : [j, i]);
    }
    rounds.push(pairs);
  }
  return rounds;
}

// What the search minimises. Runs are handled first by construction (see
// attemptSchedule): the weight here only matters where a bye makes them
// depend on who sits where. Court sharing comes next: the 2026 card, drawn up
// by hand, never asks a club to host more than half the teams it has out on a
// night.
const WEIGHTS = {
  run: 80, // three or more consecutive home (or away) matches
  clash: 60, // a club hosting more than its fair share on one day
  balance: 40, // home/away split within a half off by more than one
  derby: 2, // same-club matches drifting later in the half
  lateDerby: 25, // ...and especially past week 5
};

/**
 * Generate the season.
 *
 * divisions: [{ name, level, playDay, teams: [{ name, club }] }]
 * config: { year, week1Monday, weeks (18), seed, derbiesEarly (true), dateOverrides, effort }
 *
 * Returns { divisions: [{ ...division, fixtures: [{ week, extra, date, home, away }] }], stats, config }.
 *
 * Each division is a single round robin played in the first half and
 * mirrored, venues reversed, in the second. The round robin is the classic
 * circle-method draw with its minimum-break home/away pattern, the same shape
 * as the league's hand-made cards: teams alternate home and away, and where
 * that is impossible play two in a row, never three. That is fixed by
 * construction, by choosing only the ways of laying the rounds over the
 * weeks that keep it true.
 *
 * What is left to decide is who sits where in that draw. A search across all
 * divisions at once shuffles teams between seats (starting from the seed, so
 * every draw differs) until clubs sharing courts on a night are not asked to
 * host more than half their teams and same-club derbies fall in the opening
 * weeks. It makes several short attempts and keeps the best.
 */
export function generateFixtures(divisions, config = {}) {
  const cfg = { weeks: 18, derbiesEarly: true, effort: 1, seed: 1, iterations: 3000, startTemp: 14, attempts: 24, ...config };
  if (cfg.weeks % 2) throw new Error('The season needs an even number of weeks');
  if (!cfg.week1Monday) cfg.week1Monday = defaultWeek1Monday(cfg.year || new Date().getUTCFullYear() + 1);
  const half = cfg.weeks / 2;

  let best = null;
  const attempts = Math.max(1, Math.round(cfg.attempts * cfg.effort));
  for (let attempt = 0; attempt < attempts; attempt++) {
    const result = attemptSchedule(divisions, cfg, half, (cfg.seed >>> 0) + attempt * 0x9e3779b1);
    if (!best || result.cost < best.cost) best = result;
    if (best.settled) break;
  }

  const out = best.model.map((m, i) => {
    const fixtures = [];
    m.matches.forEach((match) => {
      const slot = m.slots[match.slot];
      const home = m.teams[match.flip ? match.b : match.a].name;
      const away = m.teams[match.flip ? match.a : match.b].name;
      fixtures.push({ week: slot.week, extra: slot.extra, date: slot.first, home, away });
      fixtures.push({ week: slot.week + half, extra: slot.extra, date: slot.second, home: away, away: home });
    });
    fixtures.sort(compareFixtures);
    return {
      name: m.division.name,
      level: m.division.level,
      playDay: m.division.playDay,
      teams: divisions[i].teams,
      fixtures,
    };
  });

  return { divisions: out, stats: analyse(out, cfg.weeks), config: cfg };
}

function attemptSchedule(divisions, cfg, half, seed) {
  const rand = mulberry32(seed);
  const pick = (n) => Math.floor(rand() * n);
  const shuffle = (list) => {
    for (let i = list.length - 1; i > 0; i--) {
      const j = pick(i + 1);
      [list[i], list[j]] = [list[j], list[i]];
    }
    return list;
  };

  const clubIds = new Map();
  const clubId = (club) => {
    if (!clubIds.has(club)) clubIds.set(club, clubIds.size);
    return clubIds.get(club);
  };
  const dateIds = new Map();
  const dateId = (iso) => {
    if (!dateIds.has(iso)) dateIds.set(iso, dateIds.size);
    return dateIds.get(iso);
  };

  // --- the mutable model ------------------------------------------------------
  // rounds:   the minimum-break pairings by seat, each pair listed home first
  // layouts:  the ways of laying those rounds over the weeks that keep their
  //           order (start anywhere, run forwards or backwards)
  // layout:   which of those is in use
  // swapped:  whether every venue in the division is reversed
  // seats:    which team sits in which seat (-1 is the bye)
  const model = divisions.map((d) => {
    const division = { ...d, playDay: d.playDay || defaultPlayDay(d.level) };
    const teams = d.teams.map((t) => ({
      name: cleanName(t.name),
      club: clubId(t.club ? cleanName(t.club) : clubOf(t.name)),
    }));
    const n = teams.length + (teams.length % 2);
    const rounds = n >= 2 ? canonicalRounds(n) : [];
    const slots = buildSlots(rounds.length, half).map((s) => {
      const first = slotDate(cfg, division, s.week, s.extra);
      const second = slotDate(cfg, division, s.week + half, s.extra);
      return { ...s, first, second, firstId: dateId(first), secondId: dateId(second) };
    });
    // Every slot of the season in date order, for run lengths.
    const timeline = [
      ...slots.map((s, i) => ({ slot: i, second: false, date: s.first })),
      ...slots.map((s, i) => ({ slot: i, second: true, date: s.second })),
    ].sort((x, y) => (x.date < y.date ? -1 : x.date > y.date ? 1 : 0));
    const inDateOrder = slots.map((_, i) => i).sort((x, y) => (slots[x].first < slots[y].first ? -1 : slots[x].first > slots[y].first ? 1 : 0));
    const count = rounds.length;
    const layouts = [];
    for (let start = 0; start < count; start++) {
      for (const backwards of [false, true]) {
        const order = new Int16Array(count); // round → slot
        for (let i = 0; i < count; i++) {
          const round = (((backwards ? -i : i) + start) % count + count) % count;
          order[round] = inDateOrder[i];
        }
        layouts.push(order);
      }
    }
    const seats = teams.map((_, i) => i);
    while (seats.length < n) seats.push(-1);
    shuffle(seats);
    return {
      division,
      teams,
      n,
      rounds,
      slots,
      seats,
      layouts,
      layout: layouts.length ? pick(layouts.length) : 0,
      swapped: rand() < 0.5,
      timelineSlot: Int16Array.from(timeline, (e) => e.slot),
      timelineSign: Int8Array.from(timeline, (e) => (e.second ? -1 : 1)),
      grid: new Int8Array(slots.length * Math.max(1, teams.length)),
    };
  });

  const dateTotal = Math.max(1, dateIds.size);
  const homeLoad = new Int16Array(clubIds.size * dateTotal);
  const playLoad = new Int16Array(clubIds.size * dateTotal);

  // --- cost -------------------------------------------------------------------
  // Balance, runs and derby timing belong to one division and are cached per
  // division; the club-day loads couple divisions that share a night.
  function divisionCost(m) {
    let cost = 0;
    const { teams, rounds, slots, seats, grid, timelineSlot, timelineSign } = m;
    if (!rounds.length) return 0;
    const order = m.layouts[m.layout];
    const hi = m.swapped ? 1 : 0;
    const count = teams.length;
    const slotCount = slots.length;
    grid.fill(0); // grid[slot * count + team]: +1 home, -1 away, 0 bye (first half)
    for (let r = 0; r < rounds.length; r++) {
      const slot = order[r];
      const pairs = rounds[r];
      for (let p = 0; p < pairs.length; p++) {
        const h = seats[pairs[p][hi]];
        const a = seats[pairs[p][1 - hi]];
        if (h < 0 || a < 0) continue;
        grid[slot * count + h] = 1;
        grid[slot * count + a] = -1;
        if (cfg.derbiesEarly && teams[h].club === teams[a].club) {
          const week = slots[slot].week;
          cost += WEIGHTS.derby * (week - 1) + (week > 5 ? WEIGHTS.lateDerby * (week - 5) : 0);
        }
      }
    }
    const length = timelineSlot.length;
    for (let t = 0; t < count; t++) {
      let home = 0;
      let away = 0;
      for (let s = 0; s < slotCount; s++) {
        const v = grid[s * count + t];
        if (v > 0) home++;
        else if (v < 0) away++;
      }
      const diff = home > away ? home - away : away - home;
      if (diff > 1) cost += WEIGHTS.balance * (diff - 1);

      let run = 0;
      let last = 0;
      for (let i = 0; i < length; i++) {
        const v = grid[timelineSlot[i] * count + t] * timelineSign[i];
        if (v === 0) continue;
        if (v === last) {
          run++;
          if (run >= 3) cost += WEIGHTS.run * (2 * run - 5); // 3 in a row: 1x, 4: 4x, 5: 9x
        } else {
          last = v;
          run = 1;
        }
      }
    }
    return cost;
  }

  // A club with several teams out on the same day shares its courts: at most
  // half of them (rounded up) should be at home. A derby is one home match.
  function applyLoad(m, sign) {
    const { teams, rounds, slots, seats } = m;
    if (!rounds.length) return;
    const order = m.layouts[m.layout];
    const hi = m.swapped ? 1 : 0;
    for (let r = 0; r < rounds.length; r++) {
      const slot = slots[order[r]];
      const pairs = rounds[r];
      for (let p = 0; p < pairs.length; p++) {
        const h = seats[pairs[p][hi]];
        const a = seats[pairs[p][1 - hi]];
        if (h < 0 || a < 0) continue;
        const hc = teams[h].club * dateTotal;
        const ac = teams[a].club * dateTotal;
        homeLoad[hc + slot.firstId] += sign;
        homeLoad[ac + slot.secondId] += sign;
        playLoad[hc + slot.firstId] += sign;
        playLoad[hc + slot.secondId] += sign;
        playLoad[ac + slot.firstId] += sign;
        playLoad[ac + slot.secondId] += sign;
      }
    }
  }

  function clashCost() {
    let cost = 0;
    for (let i = 0; i < homeLoad.length; i++) {
      const over = homeLoad[i] - ((playLoad[i] + 1) >> 1);
      if (over > 0) cost += WEIGHTS.clash * over * over;
    }
    return cost;
  }

  // With an even number of teams, how long anyone's runs are depends only on
  // the layout, not on who sits where. Keep just the layouts that do best on
  // that score, so the search below cannot trade short runs away. (With a bye
  // the runs depend on which seat is empty, so every layout stays in play.)
  model.forEach((m) => {
    if (!m.rounds.length || m.teams.length % 2) return;
    const seats = m.seats;
    m.seats = m.teams.map((_, i) => i);
    const derbies = cfg.derbiesEarly;
    cfg.derbiesEarly = false;
    const scored = m.layouts.map((order, i) => {
      m.layout = i;
      return [divisionCost(m), order];
    });
    cfg.derbiesEarly = derbies;
    const best = Math.min(...scored.map(([cost]) => cost));
    m.layouts = scored.filter(([cost]) => cost === best).map(([, order]) => order);
    m.layout = pick(m.layouts.length);
    m.seats = seats;
  });

  // --- simulated annealing ------------------------------------------------------
  model.forEach((m) => applyLoad(m, 1));
  const partial = model.map(divisionCost);
  let clash = clashCost();
  let current = partial.reduce((x, y) => x + y, 0) + clash;
  const usable = model.map((m, i) => (m.teams.length >= 2 ? i : -1)).filter((i) => i >= 0);
  const iterations = Math.round(cfg.iterations * Math.max(1, usable.length));
  const startTemp = cfg.startTemp;

  for (let it = 0; it < iterations && current > 0 && usable.length; it++) {
    const di = usable[pick(usable.length)];
    const m = model[di];
    const kind = rand();
    let change;
    let undo;
    if (kind < 0.72) {
      // Swap two teams' seats in the draw.
      const a = pick(m.n);
      let b = pick(m.n - 1);
      if (b >= a) b++;
      change = undo = () => {
        [m.seats[a], m.seats[b]] = [m.seats[b], m.seats[a]];
      };
    } else if (kind < 0.92) {
      // Lay the rounds over the weeks differently.
      const before = m.layout;
      if (m.layouts.length < 2) continue;
      const after = pick(m.layouts.length);
      if (after === before) continue;
      change = () => (m.layout = after);
      undo = () => (m.layout = before);
    } else {
      // Reverse every venue in the division.
      change = undo = () => (m.swapped = !m.swapped);
    }
    applyLoad(m, -1);
    change();
    applyLoad(m, 1);
    const nextPartial = divisionCost(m);
    const nextClash = clashCost();
    const next = current - partial[di] - clash + nextPartial + nextClash;
    const temp = startTemp * (1 - it / iterations);
    if (next <= current || (temp > 0.01 && rand() < Math.exp((current - next) / temp))) {
      partial[di] = nextPartial;
      clash = nextClash;
      current = next;
    } else {
      applyLoad(m, -1);
      undo();
      applyLoad(m, 1);
    }
  }

  // Flatten to matches for the caller.
  model.forEach((m) => {
    m.matches = [];
    const order = m.layouts[m.layout];
    const hi = m.swapped ? 1 : 0;
    m.rounds.forEach((round, r) => {
      round.forEach((pair) => {
        const a = m.seats[pair[hi]];
        const b = m.seats[pair[1 - hi]];
        if (a >= 0 && b >= 0) m.matches.push({ slot: order[r], a, b, flip: false });
      });
    });
  });
  // Settled: nobody over-hosted and no derby later than week 5.
  const settled =
    clash === 0 &&
    model.every((m) => m.matches.every((x) => m.teams[x.a].club !== m.teams[x.b].club || m.slots[x.slot].week <= 5 || !cfg.derbiesEarly));
  return { model, cost: Math.round(current), settled };
}

export function compareFixtures(a, b) {
  return (
    a.week - b.week ||
    (a.date < b.date ? -1 : a.date > b.date ? 1 : 0) ||
    (a.home < b.home ? -1 : a.home > b.home ? 1 : 0)
  );
}

// ---------------------------------------------------------------------------
// Analysis and validation (also used on loaded files)
// ---------------------------------------------------------------------------

/**
 * Quality figures and structural errors for a set of divisions with fixtures.
 * errors is empty for a sound fixture list.
 */
export function analyse(divisions, weeks = 18) {
  const errors = [];
  const half = weeks / 2;
  const clubDay = new Map(); // "club|date" → home fixtures that day
  const clubPlaying = new Map(); // "club|date" → teams of that club in action
  const teams = [];
  const derbies = [];
  let fixtureCount = 0;
  let doubleHeaders = 0;

  divisions.forEach((d) => {
    const names = d.teams.map((t) => cleanName(t.name));
    const clubs = new Map(d.teams.map((t) => [cleanName(t.name), t.club ? cleanName(t.club) : clubOf(t.name)]));
    const known = new Set(names);
    const meetings = new Map();
    const perTeam = new Map(names.map((n) => [n, []]));
    const perDate = new Map();
    const extras = new Set();

    d.fixtures.forEach((f) => {
      fixtureCount++;
      const home = cleanName(f.home);
      const away = cleanName(f.away);
      if (!known.has(home) || !known.has(away)) {
        errors.push(`${d.name}: ${home} v ${away} involves a team that is not in the division`);
        return;
      }
      if (home === away) {
        errors.push(`${d.name}: ${home} is drawn against itself`);
        return;
      }
      if (!(f.week >= 1 && f.week <= weeks)) errors.push(`${d.name}: ${home} v ${away} has week ${f.week}`);
      if (!/^\d{4}-\d{2}-\d{2}$/.test(f.date || '')) errors.push(`${d.name}: ${home} v ${away} has no valid date`);
      const key = `${home}|${away}`;
      meetings.set(key, [...(meetings.get(key) || []), f]);
      perTeam.get(home).push({ date: f.date, week: f.week, venue: 'H', opponent: away });
      perTeam.get(away).push({ date: f.date, week: f.week, venue: 'A', opponent: home });
      for (const team of [home, away]) {
        const k = `${team}|${f.date}`;
        if (perDate.has(k)) errors.push(`${d.name}: ${team} has two matches on ${f.date}`);
        perDate.set(k, true);
      }
      if (f.extra) extras.add(`${f.week}|${f.date}`);
      const homeClub = clubs.get(home);
      const cd = `${homeClub}|${f.date}`;
      clubDay.set(cd, [...(clubDay.get(cd) || []), `${home} v ${away}`]);
      for (const team of [home, away]) {
        const k = `${clubs.get(team)}|${f.date}`;
        clubPlaying.set(k, (clubPlaying.get(k) || 0) + 1);
      }
      if (homeClub === clubs.get(away)) derbies.push({ division: d.name, week: f.week, date: f.date, home, away });
    });
    doubleHeaders += extras.size;

    for (let i = 0; i < names.length; i++) {
      for (let j = 0; j < names.length; j++) {
        if (i === j) continue;
        const games = meetings.get(`${names[i]}|${names[j]}`) || [];
        if (games.length !== 1) {
          errors.push(`${d.name}: ${names[i]} should host ${names[j]} exactly once (found ${games.length})`);
        }
      }
    }
    // Second half mirrors the first.
    meetings.forEach((games, key) => {
      const [home, away] = key.split('|');
      const back = (meetings.get(`${away}|${home}`) || [])[0];
      const f = games[0];
      if (back && f.week <= half && back.week !== f.week + half) {
        errors.push(`${d.name}: ${home} v ${away} (week ${f.week}) is not mirrored in week ${f.week + half}`);
      }
    });

    names.forEach((name) => {
      const games = perTeam.get(name).sort((a, b) => (a.date < b.date ? -1 : a.date > b.date ? 1 : a.week - b.week));
      let longest = 0;
      let run = 0;
      let last = '';
      games.forEach((g) => {
        run = g.venue === last ? run + 1 : 1;
        last = g.venue;
        longest = Math.max(longest, run);
      });
      const firstHalf = games.filter((g) => g.week <= half);
      teams.push({
        division: d.name,
        name,
        club: clubs.get(name),
        played: games.length,
        home: games.filter((g) => g.venue === 'H').length,
        away: games.filter((g) => g.venue === 'A').length,
        firstHalfHome: firstHalf.filter((g) => g.venue === 'H').length,
        firstHalfAway: firstHalf.filter((g) => g.venue === 'A').length,
        longestRun: longest,
      });
    });
  });

  // Days on which a club hosts more than half (rounded up) of its teams in action.
  const clashes = [];
  clubDay.forEach((matches, key) => {
    const [club, date] = key.split('|');
    const playing = clubPlaying.get(key) || 0;
    const limit = Math.ceil(playing / 2);
    if (matches.length > limit) clashes.push({ club, date, matches, limit });
  });
  clashes.sort((a, b) => (a.date < b.date ? -1 : a.date > b.date ? 1 : a.club < b.club ? -1 : 1));

  return {
    errors,
    fixtures: fixtureCount,
    teams,
    clashes,
    derbies: derbies.filter((x) => x.week <= half),
    doubleHeaders,
    longestRun: teams.reduce((m, t) => Math.max(m, t.longestRun), 0),
    unevenTeams: teams.filter((t) => Math.abs(t.firstHalfHome - t.firstHalfAway) > 1).length,
  };
}

// ---------------------------------------------------------------------------
// Fixture file
// ---------------------------------------------------------------------------

/**
 * FNV-1a (32 bit) over the canonical fixture list. It lets a recipient see
 * that a file still says what the generator produced; it is not a signature.
 * The Go importer computes the same value (internal/fixturefile).
 */
export function checksum(divisions) {
  const lines = [];
  divisions.forEach((d) => {
    d.fixtures.forEach((f) => {
      lines.push(`${cleanName(d.name)}|${f.week}|${f.date}|${cleanName(f.home)}|${cleanName(f.away)}`);
    });
  });
  lines.sort();
  const bytes = new TextEncoder().encode(lines.join('\n'));
  let h = 0x811c9dc5;
  for (let i = 0; i < bytes.length; i++) {
    h ^= bytes[i];
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return h.toString(16).padStart(8, '0');
}

/** Assemble the shareable fixture file. */
export function buildFile({ league, season, divisions, generator, status, acceptedAt, decisions, memorial }) {
  const file = {
    format: FILE_FORMAT,
    version: FILE_VERSION,
    league: { name: league?.name || 'Parks League', shortName: league?.shortName || '' },
    season: {
      year: season.year,
      weeks: season.weeks || 18,
      week1Commencing: season.week1Monday,
    },
    status: status === 'accepted' ? 'accepted' : 'draft',
    generatedAt: generator?.generatedAt || new Date().toISOString(),
    generator: {
      name: 'jim.tennis fixture generator',
      engine: ENGINE_VERSION,
      seed: generator?.seed ?? null,
      basedOn: generator?.basedOn || null,
    },
    divisions: divisions.map((d, i) => ({
      name: d.name,
      level: d.level ?? i + 1,
      playDay: d.playDay,
      teams: d.teams.map((t) => {
        const team = { name: cleanName(t.name), club: t.club ? cleanName(t.club) : clubOf(t.name) };
        if (t.movement) team.movement = t.movement;
        if (t.previousDivision) team.previousDivision = t.previousDivision;
        if (t.previousPosition) team.previousPosition = t.previousPosition;
        return team;
      }),
      fixtures: d.fixtures
        .slice()
        .sort(compareFixtures)
        .map((f) => {
          const fixture = { week: f.week, date: f.date, home: cleanName(f.home), away: cleanName(f.away) };
          if (f.extra) fixture.extra = true;
          return fixture;
        }),
    })),
  };
  if (file.status === 'accepted') file.acceptedAt = acceptedAt || new Date().toISOString();
  if (decisions?.length) file.decisions = decisions;
  if (memorial) file.dedication = memorial;
  file.checksum = checksum(file.divisions);
  return file;
}

/**
 * Read a fixture file. Throws an Error with a message fit to show a user when
 * the file is not usable; otherwise returns { file, stats, intact }.
 */
export function parseFile(text) {
  let file;
  try {
    file = typeof text === 'string' ? JSON.parse(text) : text;
  } catch {
    throw new Error("That file isn't a jim.tennis fixture file (it is not valid JSON).");
  }
  if (!file || file.format !== FILE_FORMAT) {
    throw new Error("That file isn't a jim.tennis fixture file.");
  }
  if (!(file.version >= 1) || file.version > FILE_VERSION) {
    throw new Error(
      `This fixture file is version ${file.version}; this copy of jim.tennis reads up to version ${FILE_VERSION}.`
    );
  }
  if (!Array.isArray(file.divisions) || !file.divisions.length) {
    throw new Error('The fixture file has no divisions.');
  }
  if (!file.season || !(file.season.year > 1900)) throw new Error('The fixture file has no season year.');
  file.divisions.forEach((d, i) => {
    if (!d || !d.name || !Array.isArray(d.teams) || !Array.isArray(d.fixtures)) {
      throw new Error(`Division ${i + 1} in the fixture file is incomplete.`);
    }
    d.name = cleanName(d.name);
    d.level = d.level ?? i + 1;
    d.teams = d.teams.map((t) => ({ ...t, name: cleanName(t.name), club: t.club ? cleanName(t.club) : clubOf(t.name) }));
    d.fixtures = d.fixtures.map((f) => ({ ...f, home: cleanName(f.home), away: cleanName(f.away), extra: !!f.extra }));
  });
  const stats = analyse(file.divisions, file.season.weeks || 18);
  if (stats.errors.length) {
    const more = stats.errors.length > 1 ? ` (and ${stats.errors.length - 1} more)` : '';
    throw new Error(`The fixture file is inconsistent: ${stats.errors[0]}${more}.`);
  }
  return { file, stats, intact: !file.checksum || file.checksum === checksum(file.divisions) };
}

function csvCell(v) {
  const s = String(v ?? '');
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** Flat CSV of every fixture, for spreadsheets. */
export function toCSV(divisions) {
  const rows = [['Division', 'Week', 'Date', 'Day', 'Home', 'Away']];
  divisions.forEach((d) => {
    d.fixtures
      .slice()
      .sort(compareFixtures)
      .forEach((f) => {
        const day = DAYS[(parseISO(f.date).getUTCDay() + 6) % 7];
        rows.push([d.name, f.week, f.date, day, f.home, f.away]);
      });
  });
  return rows.map((r) => r.map(csvCell).join(',')).join('\r\n') + '\r\n';
}
