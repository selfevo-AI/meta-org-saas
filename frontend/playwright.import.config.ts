import { defineConfig } from '@playwright/test'
import base from './playwright.config'

export default defineConfig({
  ...base,
  globalSetup: undefined,
  testMatch: 'document-imports.spec.ts',
  outputDir: 'test-results/document-imports',
})
