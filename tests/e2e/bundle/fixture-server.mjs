// Static file server for the saved BHPLTA pages (tests/e2e/fixtures/bhplta).
// Usage: node fixture-server.mjs <root> <port>
import http from "node:http";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(process.argv[2]);
const port = parseInt(process.argv[3], 10);

http
  .createServer((req, res) => {
    const urlPath = decodeURIComponent((req.url || "/").split("?")[0]);
    let file = path.join(root, path.normalize(urlPath));
    if (!file.startsWith(root)) return res.writeHead(403).end();
    if (urlPath.endsWith("/") || !path.extname(file)) file = path.join(file, "index.html");
    try {
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" }).end(fs.readFileSync(file));
    } catch {
      res.writeHead(404).end("not found");
    }
  })
  .listen(port, "0.0.0.0");
