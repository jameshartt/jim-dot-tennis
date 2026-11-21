import { defineConfig } from "@playwright/test";

// Specs that need passkeys (WebAuthn needs a secure context), so they only run
// in the "auth" project against http://localhost:8080.
const AUTH_SPECS = [
  "**/passkeys.spec.ts",
  "**/account-links.spec.ts",
  "**/admin-passkeys.spec.ts",
  "**/cli-reset.spec.ts",
  // Creates users and changes roles: those forms POST from a page whose
  // Referrer-Policy is no-referrer, and over plain http to a non-localhost host
  // the browser then sends Origin: null with no Sec-Fetch-Site, which the
  // app's cross-origin guard (correctly) refuses. http://localhost is a secure
  // context, so the browser sends Sec-Fetch-Site there.
  "**/admin/users.spec.ts",
];

export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 1, // Retry once to handle transient SQLite locking
  workers: process.env.FRESH_INSTALL
    ? 1
    : process.env.WORKERS
      ? parseInt(process.env.WORKERS, 10)
      : 2,
  timeout: 30_000,

  globalSetup: "./global-setup.ts",

  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || "http://app-e2e:8080",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },

  reporter: [
    ["json", { outputFile: "test-results/results.json" }],
    ["html", { open: "never", outputFolder: "playwright-report" }],
    ["list"],
  ],

  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
      testIgnore: ["**/multi-club.spec.ts", "**/fresh-install*.spec.ts", ...AUTH_SPECS],
    },
    // Passkey-enabled run: the browser reaches the app as http://localhost:8080
    // (a secure context), see docker-compose.yml `e2e-auth` and
    // scripts/test-e2e-auth.sh. Run with --workers=1 (the script does): these specs share the app's
    // sign-in throttling and the "require passkeys" setting.
    {
      name: "auth",
      use: { browserName: "chromium" },
      testMatch: AUTH_SPECS,
      fullyParallel: false,
    },
    {
      name: "multi-club",
      use: { browserName: "chromium" },
      testMatch: "**/multi-club.spec.ts",
    },
    // Starts from EMPTY databases (no seed, no admin account), so it only
    // exists when `make test-e2e-fresh` sets FRESH_INSTALL. One serial flow
    // against one fresh instance: a single worker and no retries.
    ...(process.env.FRESH_INSTALL
      ? [
          {
            name: "fresh-install",
            use: { browserName: "chromium" as const },
            testMatch: "**/fresh-install*.spec.ts",
            fullyParallel: false,
            retries: 0,
            timeout: 120_000,
          },
        ]
      : []),
  ],
});
