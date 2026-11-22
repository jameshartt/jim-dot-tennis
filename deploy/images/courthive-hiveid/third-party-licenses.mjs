#!/usr/bin/env node
// Write the licences of third-party npm packages that ship in the bundle image.
//
//   node bundle/third-party-licenses.mjs --maps <dist dir> <out file>
//       Front ends. The packages are the ones whose source files appear in the
//       build's source maps, i.e. the code that was actually bundled. Run it
//       after `pnpm build` and before the maps are deleted.
//
//   node bundle/third-party-licenses.mjs --node-modules <project dir> <out file>
//       Server. Every package installed under node_modules/.pnpm.
//
// For each package: name, version, the `license` field of its package.json and
// the text of every LICENSE / LICENCE / COPYING / NOTICE file at its root.
// Packages that ship no licence file are listed with their `license` field only,
// and counted at the end, rather than given an invented text.
// No dependencies.

import { existsSync, readdirSync, readFileSync, statSync, writeFileSync, lstatSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';

const [mode, input, out] = process.argv.slice(2);
if (!['--maps', '--node-modules'].includes(mode) || !input || !out) {
  console.error('usage: third-party-licenses.mjs --maps <dist> <out> | --node-modules <dir> <out>');
  process.exit(2);
}

const walkFiles = (dir, pred, acc = []) => {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) walkFiles(p, pred, acc);
    else if (pred(p)) acc.push(p);
  }
  return acc;
};

// Given any path inside node_modules, return the package root directory.
const packageRoot = (p) => {
  const parts = p.split(sep);
  const i = parts.lastIndexOf('node_modules');
  if (i < 0 || i + 1 >= parts.length) return null;
  const n = parts[i + 1].startsWith('@') ? 2 : 1;
  return parts.slice(0, i + 1 + n).join(sep);
};

const roots = new Set();
if (mode === '--maps') {
  const maps = walkFiles(resolve(input), (p) => p.endsWith('.map'));
  if (!maps.length) {
    console.error(`no source maps under ${input}: cannot tell which packages were bundled`);
    process.exit(1);
  }
  for (const m of maps) {
    const map = JSON.parse(readFileSync(m, 'utf8'));
    const base = resolve(dirname(m), map.sourceRoot || '');
    for (const s of map.sources || []) {
      if (!s.includes('node_modules/')) continue;
      const r = packageRoot(resolve(base, s.replace(/^[a-z-]+:\/\/\/?/i, '')));
      if (r) roots.add(r);
    }
  }
} else {
  const store = join(resolve(input), 'node_modules', '.pnpm');
  for (const entry of readdirSync(store)) {
    const nm = join(store, entry, 'node_modules');
    if (!existsSync(nm) || !statSync(nm).isDirectory()) continue;
    for (const name of readdirSync(nm)) {
      const dirs = name.startsWith('@') ? readdirSync(join(nm, name)).map((s) => join(nm, name, s)) : [join(nm, name)];
      // Real directories only: symlinks point at a package listed elsewhere.
      for (const d of dirs) if (!lstatSync(d).isSymbolicLink()) roots.add(d);
    }
  }
}

const LICENCE_FILE = /^(licen[cs]e|copying|notice)([.-].*)?$/i;
const pkgs = new Map();
let missingPkgJson = 0;
for (const r of roots) {
  const pj = join(r, 'package.json');
  if (!existsSync(pj)) {
    missingPkgJson++;
    continue;
  }
  const meta = JSON.parse(readFileSync(pj, 'utf8'));
  if (!meta.name) continue;
  const key = `${meta.name}@${meta.version}`;
  if (pkgs.has(key)) continue;
  let licence = meta.license ?? meta.licenses ?? 'UNKNOWN';
  if (typeof licence === 'object') licence = JSON.stringify(licence);
  const files = readdirSync(r)
    .filter((f) => LICENCE_FILE.test(f) && statSync(join(r, f)).isFile())
    .sort()
    .map((f) => ({ f, text: readFileSync(join(r, f), 'utf8').trim() }));
  pkgs.set(key, { name: meta.name, version: meta.version, licence, files });
}

const sorted = [...pkgs.values()].sort((a, b) => (a.name + a.version).localeCompare(b.name + b.version));
const noFile = sorted.filter((p) => !p.files.length);
const rule = '-'.repeat(78);
let text = `Third-party software in this image\n\nGenerated at build time (${mode === '--maps' ? 'packages found in the build source maps' : 'packages installed in node_modules'}).\n`;
text += `${sorted.length} packages; ${noFile.length} ship no licence file and are listed with their declared licence only.\n\n`;
for (const p of sorted) {
  text += `${rule}\n${p.name} ${p.version}\nLicense: ${p.licence}\n`;
  for (const { f, text: t } of p.files) text += `\n--- ${f} ---\n${t}\n`;
  text += '\n';
}
writeFileSync(out, text);
console.log(`third-party licences: ${sorted.length} packages, ${noFile.length} without a licence file -> ${out}`);
if (missingPkgJson) console.log(`(${missingPkgJson} paths without package.json skipped)`);
