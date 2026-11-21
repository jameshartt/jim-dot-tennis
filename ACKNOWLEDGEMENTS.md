# Acknowledgements

jim.tennis is built on the work of many open-source projects and contributors. This file lists the third-party software and services that make it possible.

## CourtHive

Tournament and Cup management is developed and maintained by [CourtHive](https://courthive.com), with the primary development effort by **Charles Allen**. CourtHive provides tournament scheduling, draw management, and live scoring for Parks League Cup competitions.

## Brighton & Hove Parks League Tennis Association

The BHPLTA organises the parks league infrastructure. Match card data is imported from the BHPLTA website.

## Go Dependencies

Direct dependencies of the main module (`go.mod`):

| Package | License | Description |
|---------|---------|-------------|
| [github.com/PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery) | BSD-3-Clause | HTML parsing and DOM manipulation |
| [github.com/SherClockHolmes/webpush-go](https://github.com/SherClockHolmes/webpush-go) | MIT | Web push notification library |
| [github.com/golang-migrate/migrate](https://github.com/golang-migrate/migrate) | MIT | Database migration framework |
| [github.com/google/uuid](https://github.com/google/uuid) | BSD-3-Clause | UUID generation |
| [github.com/jmoiron/sqlx](https://github.com/jmoiron/sqlx) | MIT | SQL extensions for Go's database/sql |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | BSD-3-Clause | Cryptographic functions |
| [golang.org/x/net](https://pkg.go.dev/golang.org/x/net) | BSD-3-Clause | Networking libraries |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause | Pure-Go SQLite driver (SQLite itself, translated from C, is public domain) |

### Indirect dependencies

| Package | License | Description |
|---------|---------|-------------|
| [github.com/andybalholm/cascadia](https://github.com/andybalholm/cascadia) | BSD-2-Clause | CSS selector engine (used by goquery) |
| [github.com/golang-jwt/jwt](https://github.com/golang-jwt/jwt) | MIT | JSON Web Token implementation (used by webpush-go) |
| [modernc.org/libc](https://gitlab.com/cznic/libc), [modernc.org/mathutil](https://gitlab.com/cznic/mathutil), [modernc.org/memory](https://gitlab.com/cznic/memory) | BSD-3-Clause | C runtime, maths and memory allocator for the pure-Go SQLite |
| [github.com/remyoudompheng/bigfft](https://github.com/remyoudompheng/bigfft) | BSD-3-Clause | Big-number multiplication (used by modernc.org/mathutil) |
| [github.com/dustin/go-humanize](https://github.com/dustin/go-humanize) | MIT | Number formatting (used by modernc.org/sqlite) |

## Front-end dependencies

Loaded in the browser from `templates/`; nothing is vendored in `static/`.

| Package | License | Description |
|---------|---------|-------------|
| [htmx](https://htmx.org) 1.9.4 (via unpkg / jsDelivr) | BSD-2-Clause | HTML-over-the-wire interactivity |
| [Inter](https://rsms.me/inter/) (Google Fonts) | SIL OFL 1.1 | Typeface |
| [Nunito](https://fonts.google.com/specimen/Nunito) (Google Fonts) | SIL OFL 1.1 | Typeface |

## Test dependencies

End-to-end and accessibility tests (`tests/e2e/package.json`). Not part of any released binary or image.

| Package | License | Description |
|---------|---------|-------------|
| [@playwright/test](https://playwright.dev) | Apache-2.0 | Browser end-to-end test runner |
| [@axe-core/playwright](https://github.com/dequelabs/axe-core-npm) | MPL-2.0 | Accessibility checks for Playwright (pulls in axe-core, also MPL-2.0) |
| [typescript](https://www.typescriptlang.org) | Apache-2.0 | Type checking for the test suite |

## Development-only tools

| Tool | License | Description |
|------|---------|-------------|
| [tools/scraper](tools/scraper/README.md) | AGPL-3.0 (via MuPDF) | One-off 2025 fixture PDF extractor. A separate Go module that links [go-fitz](https://github.com/gen2brain/go-fitz) (MIT bindings) and [MuPDF](https://mupdf.com) (AGPL-3.0). It is not part of the main module, any released binary or any Docker image. |
