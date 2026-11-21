import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import http from "node:http";
import path from "node:path";

/**
 * Helpers for the fresh-install project (empty database, setup flow).
 */

/**
 * Reads the one-time setup code an app instance printed to its log. The make
 * target saves each instance's log into FRESH_LOG_DIR before the tests run, so
 * the code is read at run time and never stored anywhere in the repository.
 */
export function readSetupCode(logName: string): string {
  const file = path.join(process.env.FRESH_LOG_DIR || "/fresh-logs", logName);
  const matches = [...readFileSync(file, "utf8").matchAll(/SETUP CODE: ([A-Z0-9-]+)/g)];
  if (matches.length === 0) {
    throw new Error(`no "SETUP CODE:" banner in ${file}`);
  }
  // A restart prints a new code and cancels the old one: the last one counts.
  return matches[matches.length - 1][1];
}

/** Runs a read-only SQL query on the fresh instance's database; returns the rows as text. */
export function sqlite(sql: string, db = process.env.FRESH_DB || "/data/tennis.db"): string[] {
  const out = execFileSync("sqlite3", ["-readonly", db, sql], { encoding: "utf8" });
  return out.split("\n").filter((l) => l !== "");
}

const FIXTURE_ROOT = path.resolve(__dirname, "../fixtures/bhplta");

/**
 * A static file server for the saved BHPLTA pages, reachable from the app
 * container as http://fresh-fixtures:<port>. It can be stopped and started
 * again, to prove the club step reports a failure and can be retried.
 */
export class FixtureServer {
  private server: http.Server | null = null;
  readonly requests: string[] = [];
  constructor(
    private readonly port = parseInt(process.env.FRESH_FIXTURE_PORT || "9099", 10),
    private readonly root = FIXTURE_ROOT,
  ) {}

  async start(): Promise<void> {
    const server = http.createServer((req, res) => {
      const urlPath = decodeURIComponent((req.url || "/").split("?")[0]);
      this.requests.push(urlPath);
      let file = path.join(this.root, path.normalize(urlPath));
      if (!file.startsWith(this.root)) {
        res.writeHead(403).end();
        return;
      }
      if (urlPath.endsWith("/") || !path.extname(file)) file = path.join(file, "index.html");
      try {
        const body = readFileSync(file);
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" }).end(body);
      } catch {
        res.writeHead(404).end("not found");
      }
    });
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(this.port, "0.0.0.0", () => resolve());
    });
    this.server = server;
  }

  async stop(): Promise<void> {
    const server = this.server;
    this.server = null;
    if (!server) return;
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
}
