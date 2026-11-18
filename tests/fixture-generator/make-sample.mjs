// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
//
// Regenerates internal/fixturefile/testdata/sample-2027.jimtennis.json, the
// fixture file both the Go and the JavaScript tests read. Run it after a
// deliberate change to the engine or the file format:
//
//   node tests/fixture-generator/make-sample.mjs

import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import * as E from '../../static/fixture-generator/engine.js';

const root = fileURLToPath(new URL('../../', import.meta.url));
const tables = JSON.parse(fs.readFileSync(`${root}static/fixture-generator/bhplta-2026.json`, 'utf8'));
const movement = E.resolveMovement(tables);
const season = { year: 2027, weeks: 18, week1Monday: E.defaultWeek1Monday(2027) };
const { divisions } = E.generateFixtures(movement.divisions, { ...season, seed: 2027 });
const file = E.buildFile({
  league: tables.league,
  season,
  divisions,
  status: 'accepted',
  acceptedAt: '2026-11-18T12:00:00.000Z',
  generator: { seed: 2027, generatedAt: '2026-11-18T12:00:00.000Z', basedOn: { season: 2026, source: tables.source } },
});
fs.writeFileSync(`${root}internal/fixturefile/testdata/sample-2027.jimtennis.json`, JSON.stringify(file, null, 1) + '\n');
console.log(`wrote ${divisions.reduce((n, d) => n + d.fixtures.length, 0)} fixtures, checksum ${file.checksum}`);
