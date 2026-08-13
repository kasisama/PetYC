<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import Topbar from './Topbar.vue'
import { adminNavItems } from './navigation'
import { useShellLayout } from '../../composables/useShellLayout'

const route = useRoute()
const shell = useShellLayout()
</script>

<template>
  <div class="admin-layout" :class="{ 'is-collapsed': shell.collapsed.value, 'is-drawer-open': shell.drawerOpen.value }">
    <Sidebar />
    <button
      v-if="shell.drawerOpen.value"
      class="drawer-backdrop"
      type="button"
      aria-label="关闭导航"
      @click="shell.closeDrawer"
    />
    <main id="main-content" class="main" tabindex="-1">
      <Topbar />
      <div class="content">
        <router-view />
      </div>
    </main>
    <nav class="mobile-nav" aria-label="移动端主导航">
      <RouterLink
        v-for="item in adminNavItems"
        :key="item.name"
        :to="{ name: item.name }"
        :class="{ active: route.name === item.name }"
      >
        <component :is="item.icon" :size="20" :stroke-width="1.8" aria-hidden="true" />
        <span>{{ item.shortLabel }}</span>
      </RouterLink>
    </nav>
  </div>
</template>

<style scoped>
.admin-layout {
  min-height: 100dvh;
  display: grid;
  grid-template-columns: var(--sidebar-width) minmax(0, 1fr);
  background: var(--bg-base);
  transition: grid-template-columns 220ms var(--ease-out);
}

.admin-layout.is-collapsed { grid-template-columns: var(--sidebar-width-collapsed) minmax(0, 1fr); }

.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 100dvh;
}

.content {
  flex: 1;
  overflow-x: hidden;
  overflow-y: auto;
  padding: var(--page-padding);
  min-width: 0;
}

.drawer-backdrop,
.mobile-nav { display: none; }

@media (max-width: 1024px) {
  .admin-layout,
  .admin-layout.is-collapsed {
    grid-template-columns: 1fr;
  }

  .drawer-backdrop {
    position: fixed;
    inset: 0;
    z-index: calc(var(--z-sidebar) - 1);
    display: block;
    border: 0;
    background: color-mix(in srgb, #140f18 42%, transparent);
    backdrop-filter: blur(3px);
  }
}

@media (max-width: 700px) {
  .content { padding: 18px 14px 92px; }
  .mobile-nav {
    position: fixed;
    left: 12px;
    right: 12px;
    bottom: 10px;
    z-index: var(--z-sticky);
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    padding: 6px;
    border: 1px solid var(--border-strong);
    border-radius: 18px;
    background: color-mix(in srgb, var(--bg-surface) 92%, transparent);
    box-shadow: var(--shadow-popover);
    backdrop-filter: blur(18px);
  }

  .mobile-nav a {
    display: grid;
    justify-items: center;
    gap: 2px;
    padding: 6px 2px;
    color: var(--text-muted);
    font-size: 10px;
    border-radius: 12px;
  }

  .mobile-nav a.active { color: var(--accent); background: var(--accent-soft); }
}
</style>
