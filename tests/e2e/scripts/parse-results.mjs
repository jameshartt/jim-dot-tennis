#!/usr/bin/env node

/**
 * Parse Playwright JSON test results into a Claude-friendly summary.
 *
 * Usage:
 *   node parse-results.mjs [path/to/results.json]
 *
 * Default path: test-results/results.json
 *
 * Exit codes:
 *   0 - all tests passed (or flaky but passed on retry)
 *   1 - one or more tests failed
 */

import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const inputPath = process.argv[2] || "test-results/results.json";
const absPath = resolve(inputPath);

let report;
try {
  const raw = readFileSync(absPath, "utf-8");
  report = JSON.parse(raw);
} catch (err) {
  console.error(`Error reading results file: ${absPath}`);
  console.error(err.message);
  process.exit(1);
}

// Strip ANSI escape codes
function stripAnsi(str) {
  return str.replace(
    // eslint-disable-next-line no-control-regex
    /[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g,
    "",
  );
}

// Recursively collect specs from nested suites
function collectSpecs(suite, parentTitle = "") {
  const specs = [];
  const title = parentTitle
    ? `${parentTitle} > ${suite.title}`
    : suite.title || "";

  for (const spec of suite.specs || []) {
    for (const test of spec.tests || []) {
      const results = test.results || [];
      const lastResult = results[results.length - 1];
      const status = test.status || lastResult?.status || "unknown";

      specs.push({
        title: title ? `${title} > ${spec.title}` : spec.title,
        file: spec.file || "",
        line: spec.line || 0,
        status,
        duration: results.reduce((sum, r) => sum + (r.duration || 0), 0),
        retries: results.length - 1,
        error: lastResult?.error?.message
          ? stripAnsi(lastResult.error.message)
          : "",
        screenshot: lastResult?.attachments?.find((a) =>
          a.name?.includes("screenshot"),
        )?.path,
      });
    }
  }

  for (const child of suite.suites || []) {
    specs.push(...collectSpecs(child, title));
  }

  return specs;
}

// Collect all specs
const allSpecs = [];
for (const suite of report.suites || []) {
  allSpecs.push(...collectSpecs(suite));
}

// Categorise
const passed = allSpecs.filter((s) => s.status === "expected");
const failed = allSpecs.filter((s) => s.status === "unexpected");
const flaky = allSpecs.filter((s) => s.status === "flaky");
const skipped = allSpecs.filter(
  (s) => s.status === "skipped" || s.status === "disabled",
);

const totalDuration = allSpecs.reduce((sum, s) => sum + s.duration, 0);
const durationSec = (totalDuration / 1000).toFixed(1);

// Output
const sep = "=".repeat(60);
console.log(sep);
console.log("E2E TEST RESULTS");
console.log(sep);
console.log(
  `Total: ${allSpecs.length}  Passed: ${passed.length}  Failed: ${failed.length}  Flaky: ${flaky.length}  Skipped: ${skipped.length}  Duration: ${durationSec}s`,
);

if (failed.length > 0) {
  console.log(`\nFAILED (${failed.length}):`);
  failed.forEach((spec, i) => {
    console.log(`  ${i + 1}. ${spec.title}`);
    if (spec.file) {
      console.log(`     File: ${spec.file}${spec.line ? `:${spec.line}` : ""}`);
    }
    if (spec.error) {
      // Truncate long error messages
      const errLines = spec.error.split("\n").slice(0, 3).join("\n     ");
      console.log(`     Error: ${errLines}`);
    }
    if (spec.screenshot) {
      console.log(`     Screenshot: ${spec.screenshot}`);
    }
  });
}

if (flaky.length > 0) {
  console.log(`\nFLAKY (${flaky.length}):`);
  flaky.forEach((spec, i) => {
    console.log(`  ${i + 1}. ${spec.title}`);
    if (spec.file) {
      console.log(
        `     File: ${spec.file}${spec.line ? `:${spec.line}` : ""} (passed on retry)`,
      );
    }
  });
}

if (failed.length === 0 && flaky.length === 0) {
  console.log("\nAll tests passed.");
}

process.exit(failed.length > 0 ? 1 : 0);
