import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 1, // Retry once to handle transient SQLite locking
  workers: 2, // Low worker count avoids SQLite locking and login rate limits
  timeout: 30_000,

  globalSetup: "./global-setup.ts",

  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || "http://webapp:8080",
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
    },
  ],
});
