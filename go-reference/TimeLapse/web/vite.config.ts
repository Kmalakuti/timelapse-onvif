import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  // Load env file based on `mode` in the current working directory.
  // Also includes process.env variables (from Docker environment)
  const env = loadEnv(mode, process.cwd(), '')

  // Use VITE_API_URL from environment, or fall back to localhost
  const apiTarget = env.VITE_API_URL || process.env.VITE_API_URL || 'http://localhost:8000'

  console.log(`[Vite] API proxy target: ${apiTarget}`)

  return {
    plugins: [react()],
    server: {
      port: 5173,
      host: '0.0.0.0',
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
        '/health': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
