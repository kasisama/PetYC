<script setup lang="ts">
import { IconChevronLeft, IconChevronRight, IconHeartFilled } from '@tabler/icons-vue'
import { RouterLink, useRoute } from 'vue-router'
import { computed, onMounted, ref } from 'vue'
import { getPlatformStatus } from '../../api/ecosystem'
import { useShellLayout } from '../../composables/useShellLayout'
import { useTheme } from '../../composables/useTheme'
import { adminNavItems } from './navigation'

const route = useRoute()
const shell = useShellLayout()
const { theme, themes, setTheme } = useTheme()
const platformHealth=ref<Record<string,any>>({})
const online=computed(()=>Boolean(platformHealth.value.onebot?.connected||platformHealth.value.qq_official?.connected))
onMounted(async()=>{try{platformHealth.value=await getPlatformStatus()}catch{platformHealth.value={}}})
</script>

<template>
  <aside class="sidebar" :class="{ collapsed: shell.collapsed.value, open: shell.drawerOpen.value }">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true"><IconHeartFilled :size="18" /></span>
      <div class="brand-text">
        <strong>宠物养成</strong>
        <span>运营控制台</span>
      </div>
    </div>
    <nav class="nav" aria-label="后台主导航">
      <RouterLink
        v-for="item in adminNavItems"
        :key="item.name"
        class="nav-item"
        :class="{ active: route.name === item.name }"
        :to="{ name: item.name }"
        :data-tour="`nav-${item.name}`"
        :title="shell.collapsed.value ? item.label : undefined"
        @click="shell.closeDrawer"
      >
        <component :is="item.icon" :size="20" :stroke-width="1.8" aria-hidden="true" />
        <span class="nav-label">{{ item.label }}</span>
      </RouterLink>
    </nav>
    <section class="mobile-theme" aria-label="主题切换"><span>显示主题</span><div><button v-for="item in themes" :key="item.id" :class="{active:theme===item.id}" @click="setTheme(item.id)">{{item.label}}</button></div></section>
    <button class="collapse-button" type="button" @click="shell.toggleCollapsed">
      <IconChevronRight v-if="shell.collapsed.value" :size="18" />
      <IconChevronLeft v-else :size="18" />
      <span>{{ shell.collapsed.value ? '展开导航' : '收起导航' }}</span>
    </button>
    <p class="sidebar-foot"><span class="status-dot" :class="{offline:!online}" />{{online?'机器人在线':'机器人暂未连接'}}</p>
  </aside>
</template>

<style scoped>
.sidebar {
  position: sticky;
  top: 0;
  z-index: var(--z-sidebar);
  width: var(--sidebar-width);
  height: 100dvh;
  display: flex;
  flex-direction: column;
  padding: 18px 12px 14px;
  background: color-mix(in srgb, var(--bg-surface) 94%, var(--accent-soft));
  border-right: 1px solid var(--border-color);
  transition: width 220ms var(--ease-out), transform 220ms var(--ease-out);
}

.sidebar.collapsed { width: var(--sidebar-width-collapsed); }
.sidebar.collapsed .brand-text,
.sidebar.collapsed .nav-label,
.sidebar.collapsed .collapse-button span,
.sidebar.collapsed .sidebar-foot { display: none; }

.sidebar.collapsed .brand,
.sidebar.collapsed .nav-item,
.sidebar.collapsed .collapse-button { justify-content: center; }

.sidebar.collapsed .nav-item { padding-inline: 0; }

.sidebar.collapsed .brand { padding-inline: 0; }

.sidebar.collapsed .collapse-button { width: 40px; margin-inline: auto; }

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 12px 22px;
}

.brand-mark {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 12px;
  background: var(--accent-soft);
  color: var(--accent);
  flex: 0 0 auto;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 18%, transparent);
}

.brand-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.brand-text strong {
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.brand-text span {
  font-size: 11px;
  color: var(--text-muted);
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 11px;
  min-height: var(--control-height);
  padding: 8px 12px;
  font-size: 14px;
  color: var(--text-muted);
  text-decoration: none;
  border-radius: 12px;
  border: 1px solid transparent;
  transition:
    background-color 0.15s ease,
    color 0.15s ease;
}

.nav-item:hover {
  color: var(--text-main);
  background-color: var(--bg-hover);
}

.nav-item.active {
  color: var(--accent);
  background-color: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 18%, transparent);
  font-weight: 600;
}

.collapse-button {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 38px;
  padding: 8px 11px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.collapse-button:hover { color: var(--text-main); background: var(--bg-hover); }
.collapse-button span { white-space: nowrap; }

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success);
  box-shadow: 0 0 0 4px var(--success-soft);
}

.sidebar-foot {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 10px 12px 0;
  font-size: 11px;
  color: var(--text-muted);
}
.mobile-theme{display:none}.status-dot.offline{background:var(--text-muted);box-shadow:0 0 0 4px var(--bg-elevated)}

@media (max-width: 1024px) {
  .sidebar,
  .sidebar.collapsed {
    position: fixed;
    left: 0;
    width: min(286px, calc(100vw - 42px));
    transform: translateX(-105%);
    box-shadow: var(--shadow-dialog);
  }
  .sidebar.open { transform: translateX(0); }
  .sidebar.collapsed .brand-text,
  .sidebar.collapsed .nav-label,
  .sidebar.collapsed .collapse-button span,
  .sidebar.collapsed .sidebar-foot { display: initial; }
  .sidebar.collapsed .brand,
  .sidebar.collapsed .nav-item,
  .sidebar.collapsed .collapse-button { justify-content: flex-start; }
  .sidebar.collapsed .nav-item { padding-inline: 12px; }
  .sidebar.collapsed .brand { padding-inline: 12px; }
  .collapse-button { display: none; }
  .mobile-theme{display:grid;gap:8px;margin:10px 8px;padding:11px;border:1px solid var(--border-color);border-radius:12px}.mobile-theme>span{color:var(--text-muted);font-size:11px}.mobile-theme>div{display:grid;grid-template-columns:repeat(3,1fr);gap:5px}.mobile-theme button{min-height:34px;border:0;border-radius:8px;background:var(--bg-elevated);color:var(--text-muted)}.mobile-theme button.active{background:var(--accent);color:var(--accent-ink)}
}
</style>
