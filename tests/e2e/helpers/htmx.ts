import { Page } from "@playwright/test";

/**
 * Wait for HTMX to finish settling after a content swap.
 * Listens for the htmx:afterSettle DOM event.
 */
export async function waitForHtmxSettle(
  page: Page,
  timeout = 1000,
): Promise<void> {
  await page.evaluate((ms) => {
    return new Promise<void>((resolve) => {
      const timer = setTimeout(resolve, ms);
      document.addEventListener(
        "htmx:afterSettle",
        () => {
          clearTimeout(timer);
          resolve();
        },
        { once: true },
      );
    });
  }, timeout);
}

/**
 * Wait for an HTMX request to complete (htmx:afterRequest event).
 */
export async function waitForHtmxRequest(
  page: Page,
  timeout = 1000,
): Promise<void> {
  await page.evaluate((ms) => {
    return new Promise<void>((resolve) => {
      const timer = setTimeout(resolve, ms);
      document.addEventListener(
        "htmx:afterRequest",
        () => {
          clearTimeout(timer);
          resolve();
        },
        { once: true },
      );
    });
  }, timeout);
}
