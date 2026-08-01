<script setup lang="ts">
import Sidebar from './Sidebar.vue'
import Topbar from './Topbar.vue'
</script>

<template>
  <div class="admin-layout">
    <Sidebar />
    <main class="main">
      <Topbar />
      <div class="content">
        <router-view />
      </div>
    </main>
  </div>
</template>

<style scoped>
/* 计划中的布局原本用 Tailwind 原子类描述，本项目未引入 Tailwind，
   这里用等价的 scoped CSS + theme.css 变量实现同样的结构。 */
.admin-layout {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 200px 1fr;
}

.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.content {
  flex: 1;
  overflow: auto;
  padding: 24px;
  margin: 8px 0 0 8px;
  background-color: var(--bg-surface);
  border-top: 1px solid var(--border-color);
  border-left: 1px solid var(--border-color);
  border-top-left-radius: 24px;
}
</style>
