import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'prompt',
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',
      injectRegister: false,
      includeAssets: ['app-icon.svg', 'app-icon-180.png', 'app-icon-192.png', 'app-icon-512.png'],
      manifest: {
        id: '/',
        name: 'BIKERS.KZ',
        short_name: 'BIKERS.KZ',
        description: 'A motorcycle community for Kazakhstan',
        theme_color: '#181818',
        background_color: '#181818',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        icons: [
          { src: '/app-icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/app-icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable' },
          { src: '/app-icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
        ],
      },
      injectManifest: { globPatterns: ['**/*.{js,css,html,svg,woff2,ttf}'] },
      devOptions: { enabled: false },
    }),
  ],
  server: {
    proxy: { '/api': { target: process.env.BIKERSKZ_DEV_API_TARGET ?? 'http://127.0.0.1:8080', changeOrigin: Boolean(process.env.BIKERSKZ_DEV_API_TARGET), ws: true } },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    restoreMocks: true,
    exclude: ['node_modules/**', 'dist/**'],
  },
})
