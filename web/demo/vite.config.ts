import { defineConfig } from 'vitest/config'
import path from 'node:path'

// The Demo Room's own pages and the chrome it lays over the real app. Built into
// internal/demo/ui/dist and served by cmd/demo at /__demo/ui/ — never part of the
// SPA a shop gets.
const dirname = import.meta.dirname

export default defineConfig({
  root: dirname,
  base: '/__demo/ui/',
  resolve: { alias: { '@': path.resolve(dirname, '../src') } },
  build: {
    outDir: path.resolve(dirname, '../../internal/demo/ui/dist'),
    emptyOutDir: true,
    rollupOptions: {
      input: { index: path.resolve(dirname, 'index.html'), lock: path.resolve(dirname, 'lock.html'), overlay: path.resolve(dirname, 'overlay.ts') },
      output: { entryFileNames: '[name].js', chunkFileNames: '[name]-[hash].js', assetFileNames: '[name][extname]' },
    },
  },
  test: { environment: 'jsdom', include: ['**/*.test.ts'], css: { include: /palettes\/|overlay\.css/ } },
})
