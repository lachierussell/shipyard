import { resolve } from 'path';
import { defineConfig } from 'vite';

export default defineConfig({
  base: '/',

  resolve: {
    alias: {
      'src': resolve(__dirname, 'src')
    }
  },

  build: {
    outDir: 'dist',
    sourcemap: false,
  },
});
