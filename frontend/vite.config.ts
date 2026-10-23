import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// The Go server embeds this directory into the binary (internal/web/static).
const outDir = fileURLToPath(new URL('../internal/web/static', import.meta.url))

// emptyOutDir wipes the tracked .gitkeep that keeps go:embed working on a
// fresh clone; put it back after every build.
const keepGitkeep = (): Plugin => ({
  name: 'keep-gitkeep',
  apply: 'build',
  closeBundle() {
    writeFileSync(`${outDir}/.gitkeep`, '')
  },
})

export default defineConfig({
  plugins: [tanstackRouter({ target: 'react', autoCodeSplitting: true }), react(), keepGitkeep()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    outDir,
    emptyOutDir: true,
  },
  server: {
    // `just backend run serve` listens here; the SPA talks to it through the proxy.
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
})
