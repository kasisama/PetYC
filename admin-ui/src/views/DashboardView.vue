<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, ApiError } from '../api/client'

// 后端 /api/admin/stats 的字段以后可能扩展，这里只声明当前用到的部分。
interface AdminStats {
  [key: string]: unknown
}

const stats = ref<AdminStats | null>(null)
const error = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    stats.value = await api.get<AdminStats>('/api/admin/stats')
  } catch (err) {
    // 401 已由拦截器处理跳转，这里只呈现其余错误。
    error.value = err instanceof ApiError ? err.message : '加载统计数据失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section>
    <h1 class="page-title">仪表盘</h1>
    <p v-if="loading" class="page-hint">正在加载…</p>
    <p v-else-if="error" class="page-hint">{{ error }}</p>
    <div v-else class="stat-grid">
      <div v-for="(value, key) in stats" :key="key" class="stat-card">
        <span class="stat-key">{{ key }}</span>
        <strong class="stat-value">{{ value }}</strong>
      </div>
    </div>
  </section>
</template>

<style scoped>
.page-title {
  margin: 0 0 16px;
  font-size: 20px;
  font-weight: 600;
}

.page-hint {
  color: var(--text-muted);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px;
  background-color: var(--bg-base);
  border: 1px solid var(--border-color);
  border-radius: 12px;
}

.stat-key {
  font-size: 12px;
  color: var(--text-muted);
}

.stat-value {
  font-size: 20px;
  font-weight: 600;
}
</style>
