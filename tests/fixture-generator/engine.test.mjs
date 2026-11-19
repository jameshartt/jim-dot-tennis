// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
//
// Tests for the fixture generator engine. Run with:
//   node --test tests/fixture-generator/engine.test.mjs

import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import * as E from '../../static/fixture-generator/engine.js';

const root = fileURLToPath(new URL('../../', import.meta.url));
const tables2026 = () => JSON.parse(fs.readFileSync(`${root}static/fixture-generator/bhplta-2026.json`, 'utf8'));
const sampleText = () => fs.readFileSync(`${root}internal/fixturefile/testdata/sample-2027.jimtennis.json`, 'utf8');

/** A division of n teams called "<club> A", one team per club unless clubs are given. */
function division(name, level, teamNames) {
  return {
    name,
    level,
    teams: teamNames.map((n, i) => ({ name: n, points: 100 - i, setsWon: 50 - i, setsLost: 10 })),
  };
}
const names = (d) => d.teams.map((t) => t.name);
const kinds = (mv, kind) => mv.decisions.filter((d) => d.kind === kind).map((d) => d.team);

test('clubOf strips a team letter but keeps two-word club names', () => {
  assert.equal(E.clubOf('Dyke A'), 'Dyke');
  assert.equal(E.clubOf('King Alfred B'), 'King Alfred');
  assert.equal(E.clubOf('Park Avenue'), 'Park Avenue');
  assert.equal(E.clubOf('St Ann’s'), "St Ann's");
  assert.equal(E.clubOf('Queens'), 'Queens');
});

test('rankDivision orders by points then set difference and flags true ties', () => {
  const ranked = E.rankDivision([
    { name: 'Low', points: 10, setsWon: 5, setsLost: 5 },
    { name: 'Level worse sets', points: 20, setsWon: 30, setsLost: 20 },
    { name: 'Level better sets', points: 20, setsWon: 40, setsLost: 20 },
    { name: 'Tied one', points: 5, setsWon: 1, setsLost: 1 },
    { name: 'Tied two', points: 5, setsWon: 3, setsLost: 3 },
  ]);
  assert.deepEqual(
    ranked.map((t) => t.name),
    ['Level better sets', 'Level worse sets', 'Low', 'Tied one', 'Tied two']
  );
  assert.deepEqual(
    ranked.filter((t) => t.tied).map((t) => t.name),
    ['Tied one', 'Tied two']
  );
});

test('standard movement: top two up, bottom two down', () => {
  const mv = E.resolveMovement({
    divisions: [
      division('Division 1', 1, ['A1', 'A2', 'A3', 'A4', 'A5', 'A6']),
      division('Division 2', 2, ['B1', 'B2', 'B3', 'B4', 'B5', 'B6']),
    ],
  });
  assert.deepEqual(names(mv.divisions[0]).sort(), ['A1', 'A2', 'A3', 'A4', 'B1', 'B2']);
  assert.deepEqual(names(mv.divisions[1]).sort(), ['A5', 'A6', 'B3', 'B4', 'B5', 'B6']);
  assert.deepEqual(mv.violations, []);
});

test('a club with two teams above blocks promotion and the place passes down', () => {
  const tables = {
    divisions: [
      division('Division 1', 1, ['Dyke', 'Dyke A', 'P', 'Q', 'R', 'S']),
      division('Division 2', 2, ['T', 'Dyke B', 'U', 'V', 'W', 'X']),
    ],
  };
  const mv = E.resolveMovement(tables);
  assert.deepEqual(kinds(mv, 'blocked'), ['Dyke B']);
  assert.deepEqual(kinds(mv, 'offered'), ['U']);
  assert.ok(names(mv.divisions[0]).includes('U'));
  assert.ok(names(mv.divisions[1]).includes('Dyke B'));

  // The offered team declines and the offer may go no further: the
  // next-to-bottom team above is reprieved instead.
  const declined = E.resolveMovement(tables, { declined: ['U'], rules: { offerDepth: 1 } });
  assert.deepEqual(kinds(declined, 'reprieved'), ['R']);
  assert.deepEqual(kinds(declined, 'relegated'), ['S']);
  assert.equal(declined.divisions[0].teams.length, 6);
  assert.equal(declined.divisions[1].teams.length, 6);
});

test('a club with two teams below blocks relegation and one fewer team goes up', () => {
  const mv = E.resolveMovement({
    divisions: [
      division('Division 1', 1, ['P', 'Q', 'R', 'S', 'T', 'Salt']),
      division('Division 2', 2, ['U', 'V', 'Salt A', 'Salt B', 'W', 'X']),
    ],
  });
  assert.deepEqual(kinds(mv, 'spared'), ['Salt']);
  assert.deepEqual(kinds(mv, 'relegated'), ['T']);
  assert.deepEqual(kinds(mv, 'held'), ['V']);
  assert.deepEqual(mv.violations, []);
});

test('the 2026 tables resolve to legal 2027 divisions', () => {
  const mv = E.resolveMovement(tables2026());
  assert.deepEqual(mv.violations, []);
  assert.deepEqual(
    mv.divisions.map((d) => d.teams.length),
    [10, 10, 10, 12]
  );
  assert.deepEqual(kinds(mv, 'blocked'), ['Dyke C', 'Hove E']);
  assert.deepEqual(kinds(mv, 'offered'), ['Preston Park B']);
  assert.deepEqual(kinds(mv, 'spared'), ['Saltdean B']);
  for (const d of mv.divisions) {
    const perClub = new Map();
    d.teams.forEach((t) => perClub.set(t.club, (perClub.get(t.club) || 0) + 1));
    assert.ok(Math.max(...perClub.values()) <= 2, `${d.name} respects the club cap`);
  }
  // Nobody is lost or duplicated.
  const before = tables2026().divisions.flatMap((d) => d.teams.map((t) => t.name)).sort();
  assert.deepEqual(mv.divisions.flatMap(names).sort(), before);
});

test('committee overrides: manual moves, withdrawals and new teams', () => {
  const mv = E.resolveMovement(tables2026(), {
    manual: { 'Hove E': 2 },
    withdrawn: ['Hove F'],
    added: [{ name: 'Rookery' }],
  });
  assert.ok(names(mv.divisions[2]).includes('Hove E'));
  assert.ok(!mv.divisions.flatMap(names).includes('Hove F'));
  assert.ok(names(mv.divisions[3]).includes('Rookery'));
  // Moving Hove E up breaks the cap, and the tool says so rather than hiding it.
  assert.deepEqual(
    mv.violations.map((v) => [v.kind, v.club]),
    [['club-cap', 'Hove']]
  );
});

test('buildSlots doubles up the opening and closing weeks first', () => {
  assert.deepEqual(
    E.buildSlots(11, 9).filter((s) => s.extra).map((s) => s.week),
    [1, 9]
  );
  assert.equal(E.buildSlots(9, 9).filter((s) => s.extra).length, 0);
  assert.equal(E.buildSlots(7, 9).length, 7);
  assert.throws(() => E.buildSlots(19, 9));
});

test('defaultWeek1Monday is the second Monday of April', () => {
  assert.equal(E.defaultWeek1Monday(2026), '2026-04-13');
  assert.equal(E.defaultWeek1Monday(2027), '2027-04-12');
});

test('generated 2027 fixtures are a sound double round robin', () => {
  const mv = E.resolveMovement(tables2026());
  const { divisions, stats } = E.generateFixtures(mv.divisions, { year: 2027, seed: 7 });
  assert.deepEqual(stats.errors, []);
  assert.equal(stats.fixtures, 3 * 90 + 132);

  for (const d of divisions) {
    const n = d.teams.length;
    assert.equal(d.fixtures.length, n * (n - 1));
    for (const f of d.fixtures) {
      assert.ok(f.week >= 1 && f.week <= 18);
      const back = d.fixtures.find((g) => g.home === f.away && g.away === f.home);
      assert.equal(Math.abs(back.week - f.week), 9, 'second half mirrors the first');
    }
  }
  for (const t of stats.teams) {
    assert.equal(t.home, t.away, `${t.name} plays as many at home as away`);
    assert.ok(Math.abs(t.firstHalfHome - t.firstHalfAway) <= 1, `${t.name} splits the first half evenly`);
  }

  // Division 4 has 12 teams: 11 rounds per half, so weeks 1 and 9 (and their
  // mirrors 10 and 18) carry an extra round on its own date.
  const d4 = divisions[3];
  assert.deepEqual([...new Set(d4.fixtures.filter((f) => f.extra).map((f) => f.week))].sort((a, b) => a - b), [1, 9, 10, 18]);
  assert.equal(stats.doubleHeaders, 4);
  // Thursdays, Wednesdays, Tuesdays, Tuesdays.
  const day = (iso) => E.parseISO(iso).getUTCDay();
  assert.ok(divisions[0].fixtures.every((f) => day(f.date) === 4));
  assert.ok(divisions[1].fixtures.every((f) => day(f.date) === 3));
  assert.ok(divisions[2].fixtures.every((f) => day(f.date) === 2));
  assert.equal(divisions[0].fixtures[0].date, '2027-04-15');
});

test('nobody plays more than two in a row, home or away, and courts are shared', () => {
  // The league's hand-made cards alternate home and away with the odd pair of
  // matches at the same venue. Every draw must do at least as well, in every
  // division, while never asking a club to host more than half its teams.
  for (const options of [{}, { withdrawn: ['Saltdean D'] }]) {
    const mv = E.resolveMovement(tables2026(), options);
    for (const seed of [1, 2, 3, 4, 5, 6]) {
      const { stats } = E.generateFixtures(mv.divisions, { year: 2027, seed });
      assert.deepEqual(stats.errors, []);
      assert.equal(stats.longestRun, 2, `seed ${seed}: longest run`);
      assert.equal(stats.clashes.length, 0, `seed ${seed}: crowded nights`);
      assert.ok(stats.derbies.every((d) => d.week <= 5), `seed ${seed}: derbies fall early`);
    }
  }
});

test('even-sized divisions of any size keep runs to two', () => {
  const teams = (n) => Array.from({ length: n }, (_, i) => ({ name: `Club${i}`, club: `Club${i}` }));
  for (const n of [6, 8, 10, 12, 14, 16, 18]) {
    const { stats } = E.generateFixtures([{ name: 'Division 1', level: 1, teams: teams(n) }], { year: 2027, seed: n });
    assert.deepEqual(stats.errors, [], `${n} teams`);
    assert.equal(stats.longestRun, 2, `${n} teams`);
  }
});

test('the same seed gives the same fixtures; a new seed gives new ones', () => {
  const mv = E.resolveMovement(tables2026());
  const a = E.generateFixtures(mv.divisions, { year: 2027, seed: 11 });
  const b = E.generateFixtures(mv.divisions, { year: 2027, seed: 11 });
  const c = E.generateFixtures(mv.divisions, { year: 2027, seed: 12 });
  assert.equal(E.checksum(a.divisions), E.checksum(b.divisions));
  assert.notEqual(E.checksum(a.divisions), E.checksum(c.divisions));
});

test('odd and small divisions still schedule', () => {
  const teams = (n) => Array.from({ length: n }, (_, i) => ({ name: `Club${i}`, club: `Club${i}` }));
  for (const n of [2, 3, 5, 7, 8, 9, 11, 13]) {
    const { stats, divisions } = E.generateFixtures([{ name: 'Division 1', level: 1, teams: teams(n) }], { year: 2027, seed: n });
    assert.deepEqual(stats.errors, [], `${n} teams`);
    assert.equal(divisions[0].fixtures.length, n * (n - 1), `${n} teams`);
  }
});

test('date overrides move a round', () => {
  const teams = Array.from({ length: 4 }, (_, i) => ({ name: `Club${i}` }));
  const { divisions } = E.generateFixtures([{ name: 'Division 1', level: 1, teams }], {
    year: 2027,
    seed: 1,
    dateOverrides: { 'Division 1|2|0': '2027-04-24' },
  });
  assert.ok(divisions[0].fixtures.filter((f) => f.week === 2).every((f) => f.date === '2027-04-24'));
});

test('fixture file round-trips and detects edits', () => {
  const { file, stats, intact } = E.parseFile(sampleText());
  assert.equal(intact, true);
  assert.equal(file.season.year, 2027);
  assert.equal(file.status, 'accepted');
  assert.deepEqual(stats.errors, []);

  // Swapping the dates of two whole rounds keeps the file sound but no longer as generated.
  const edited = JSON.parse(sampleText());
  const d1 = edited.divisions[0];
  const [w1, w2] = [d1.fixtures.find((f) => f.week === 1).date, d1.fixtures.find((f) => f.week === 2).date];
  d1.fixtures.forEach((f) => {
    if (f.week === 1) f.date = w2;
    else if (f.week === 2) f.date = w1;
  });
  assert.equal(E.parseFile(JSON.stringify(edited)).intact, false);
});

test('parseFile rejects files that are not sound', () => {
  assert.throws(() => E.parseFile('not json'), /fixture file/);
  assert.throws(() => E.parseFile('{"format":"something-else"}'), /fixture file/);
  const future = JSON.parse(sampleText());
  future.version = 99;
  assert.throws(() => E.parseFile(JSON.stringify(future)), /version 99/);
  const missing = JSON.parse(sampleText());
  missing.divisions[0].fixtures.pop();
  assert.throws(() => E.parseFile(JSON.stringify(missing)), /inconsistent/);
});

test('toCSV lists every fixture', () => {
  const { file } = E.parseFile(sampleText());
  const lines = E.toCSV(file.divisions).trim().split('\r\n');
  assert.equal(lines[0], 'Division,Week,Date,Day,Home,Away');
  assert.equal(lines.length, 1 + 402);
  assert.match(lines[1], /^Division 1,1,2027-04-15,Thursday,/);
});

test('a club dropping a team is worked out before anything else', () => {
  // With Saltdean D gone, Saltdean B's relegation is no longer blocked.
  const mv = E.resolveMovement(tables2026(), { withdrawn: ['Saltdean D'] });
  assert.deepEqual(mv.violations, []);
  assert.deepEqual(kinds(mv, 'spared'), []);
  assert.ok(names(mv.divisions[3]).includes('Saltdean B'));
  assert.ok(!mv.divisions.flatMap(names).includes('Saltdean D'));
  assert.deepEqual(
    mv.divisions.map((d) => d.teams.length),
    [10, 10, 10, 11]
  );
  // An 11-team division still schedules: each team sits out one round per half.
  const { stats, divisions } = E.generateFixtures(mv.divisions, { year: 2027, seed: 3 });
  assert.deepEqual(stats.errors, []);
  assert.equal(divisions[3].fixtures.length, 11 * 10);
});

test('when a champion withdraws the next two teams take the promotion places', () => {
  const tables = {
    divisions: [
      division('Division 1', 1, ['A1', 'A2', 'A3', 'A4', 'A5', 'A6']),
      division('Division 2', 2, ['B1', 'B2', 'B3', 'B4', 'B5', 'B6']),
    ],
  };
  const mv = E.resolveMovement(tables, { withdrawn: ['B1'] });
  assert.deepEqual(kinds(mv, 'promoted'), ['B2', 'B3']);
  assert.deepEqual(kinds(mv, 'offered'), []);
});
