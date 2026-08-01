import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  // 后台挂载在 /admin 路径下，产物需要用该前缀引用静态资源。
  base: '/admin/',
  build: {
    // 直接构建到 Go 侧的 embed 目录，省去额外的拷贝步骤。
    outDir: '../admin/dist',
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
