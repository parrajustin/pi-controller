import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  expect: {
    toHaveScreenshot: {
      maxDiffPixels: 5,
      stylePath: './e2e/hide-dynamic.css'
    }
  },
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // To avoid websocket port collisions if multiple tests
  reporter: 'html',
  // Store the golden images in __golden_images__
  snapshotPathTemplate: '{testDir}/../__golden_images__/{testFilePath}/{arg}{ext}',
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: 'http://localhost:8081',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
    
    /* 800x480 Display */
    viewport: { width: 800, height: 480 },
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], viewport: { width: 800, height: 480 } },
    },
  ],

  /* Run your local dev server before starting the tests */
  webServer: {
    command: 'npm run build && npx http-server . -p 8081 -c-1',
    url: 'http://localhost:8081',
    reuseExistingServer: !process.env.CI,
  },
});
