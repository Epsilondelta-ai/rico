import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import fs from 'fs'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  server: {
    https: {
      key: fs.readFileSync(path.resolve(__dirname, '../server/desktop-i5hhv32.tail804969.ts.net.key')),
      cert: fs.readFileSync(path.resolve(__dirname, '../server/desktop-i5hhv32.tail804969.ts.net.crt')),
    },
    host: '0.0.0.0',
    port: 5173,
  },
})
