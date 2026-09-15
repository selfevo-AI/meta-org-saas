import { defineConfig, devices } from '@playwright/test'
import base from './playwright.config'

// UI interaction regressions use intercepted APIs and never provision or modify a database.
export default defineConfig({
  ...base,
  globalSetup: undefined,
  testMatch: 'ui-workspace.spec.ts',
  projects: [
    ...(base.projects ?? []),
    { name: 'tablet-chromium', use: { ...devices['Desktop Chrome'], viewport: { width: 1024, height: 768 } } },
  ],
})
