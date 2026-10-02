import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// In production fru-lab's backend serves this app, so the API is on the
// same origin. `npm run dev` serves it from Vite instead: forward /api
// (WebSockets included) to a backend on :8888, or VITE_DEV_API_TARGET.
// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'VITE_')
  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': { target: env.VITE_DEV_API_TARGET || 'http://localhost:8888', changeOrigin: true, ws: true },
      },
    },
  }
})
