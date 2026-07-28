import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'

const cacheBustEmbeddedAssets = () => ({
  name: 'cache-bust-embedded-assets',
  closeBundle() {
    const indexURL = new URL('./dist/index.html', import.meta.url)
    const appURL = new URL('./dist/app.js', import.meta.url)
    const stylesURL = new URL('./dist/styles.css', import.meta.url)
    const digest = (url) => createHash('sha256').update(readFileSync(url)).digest('hex').slice(0, 12)
    const html = readFileSync(indexURL, 'utf8')
      .replace(/\/ui\/app\.js(?:\?v=[a-f0-9]+)?/g, `/ui/app.js?v=${digest(appURL)}`)
      .replace(/\/ui\/styles\.css(?:\?v=[a-f0-9]+)?/g, `/ui/styles.css?v=${digest(stylesURL)}`)
    writeFileSync(indexURL, html)
  },
})

export default defineConfig({
  plugins: [vue(), cacheBustEmbeddedAssets()],
  base: '/ui/',
  build: {
    outDir: 'dist',
    emptyOutDir: false,
    rollupOptions: {
      output: {
        entryFileNames: 'app.js',
        assetFileNames: (assetInfo) => {
          if (assetInfo.name && assetInfo.name.endsWith('.css')) return 'styles.css';
          return 'assets/[name].[ext]';
        },
      },
    },
  },
})
