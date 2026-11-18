// Stryker's test run (make mutate): only the tests of the till's money maths, so a
// mutant is judged in seconds and the sandbox needs nothing outside web/ but proofs/.
import { defineConfig } from 'vitest/config'
import path from 'node:path'

export default defineConfig({
  resolve: { alias: { '@': path.resolve(import.meta.dirname, 'src') } },
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], include: ['src/features/till/cart/*.test.ts', 'src/features/counter/returns/*.test.ts'] },
})
