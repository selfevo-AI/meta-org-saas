import { defineConfig } from '@playwright/test'
import base from './playwright.config'

export default defineConfig({
  ...base,
  globalSetup: undefined,
  testMatch: 'document-imports-live.spec.ts',
  outputDir: 'test-results/document-imports-live',
  projects: base.projects?.filter((project) => project.name === 'desktop-chromium'),
})
