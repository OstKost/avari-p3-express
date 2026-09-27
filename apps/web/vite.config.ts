import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 4810,
    proxy: {
      '/api': {
        target: process.env.AVARI_PROXY_TARGET || 'http://localhost:4820',
        changeOrigin: true,
      },
      '^/s/': {
        target: process.env.AVARI_PROXY_TARGET || 'http://localhost:4820',
        changeOrigin: true,
      },
      '/swagger': {
        target: process.env.AVARI_PROXY_TARGET || 'http://localhost:4820',
        changeOrigin: true,
      },
    },
  },
});
