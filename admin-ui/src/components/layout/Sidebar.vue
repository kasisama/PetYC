<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router'

// 侧边栏条目与 router 中的命名路由保持一致，新增模块时两处同步添加。
const navItems = [
  { name: 'dashboard', label: '仪表盘' },
  { name: 'config', label: '配置中心' },
  { name: 'pets', label: '宠物管理' },
  { name: 'assets', label: '资产道具' },
  { name: 'system', label: '系统设置' },
] as const

const route = useRoute()
</script>

<template>
  <aside class="sidebar">
    <div class="brand">QQ 宠物</div>
    <nav class="nav">
      <RouterLink
        v-for="item in navItems"
        :key="item.name"
        class="nav-item"
        :class="{ active: route.name === item.name }"
        :to="{ name: item.name }"
      >
        {{ item.label }}
      </RouterLink>
    </nav>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  padding: 20px 12px;
  background-color: var(--bg-base);
}

.brand {
  padding: 0 12px 20px;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav-item {
  padding: 9px 12px;
  font-size: 14px;
  color: var(--text-muted);
  text-decoration: none;
  border-radius: 8px;
}

.nav-item:hover {
  color: var(--text-main);
  background-color: var(--accent-soft);
}

.nav-item.active {
  color: var(--accent-ink);
  background-color: var(--accent-soft);
}
</style>
