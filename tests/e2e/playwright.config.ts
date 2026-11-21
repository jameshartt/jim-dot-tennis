import { defineConfig } from "@playwright/test";

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
      testIgnore: ["**/multi-club.spec.ts", "**/fresh-install*.spec.ts"],
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
