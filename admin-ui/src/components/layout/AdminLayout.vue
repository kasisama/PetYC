<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import Topbar from './Topbar.vue'
import { adminNavItems } from './navigation'
import { useShellLayout } from '../../composables/useShellLayout'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import UiModal from '../ui/UiModal.vue'
import { UNSAVED_NAVIGATION_EVENT } from '../../composables/useUnsavedChanges'
import OnboardingTour from './OnboardingTour.vue'

const route = useRoute()
const shell = useShellLayout()
const navigationPrompt = ref(false)
const navigationMessage = ref('')
let resolveNavigation: ((allowed: boolean) => void) | null = null

function handleNavigationPrompt(event: Event) {
	const detail = (event as CustomEvent<{ message: string; resolve: (allowed: boolean) => void }>).detail
	if (!detail?.resolve) return
	if (resolveNavigation) resolveNavigation(false)
	resolveNavigation = detail.resolve
	navigationMessage.value = detail.message
	navigationPrompt.value = true
}

function answerNavigation(allowed: boolean) {
	navigationPrompt.value = false
	const resolve = resolveNavigation
	resolveNavigation = null
	if (resolve) resolve(allowed)
}

onMounted(() => window.addEventListener(UNSAVED_NAVIGATION_EVENT, handleNavigationPrompt))
onBeforeUnmount(() => {
	window.removeEventListener(UNSAVED_NAVIGATION_EVENT, handleNavigationPrompt)
	if (resolveNavigation) resolveNavigation(false)
})
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
        :data-tour="`nav-${item.name}`"
        :class="{ active: route.name === item.name }"
      >
        <component :is="item.icon" :size="20" :stroke-width="1.8" aria-hidden="true" />
        <span>{{ item.shortLabel }}</span>
      </RouterLink>
    </nav>
    <UiModal :open="navigationPrompt" title="放弃未保存的修改？" :description="navigationMessage" size="small" @close="answerNavigation(false)">
      <template #footer><button class="btn btn-ghost" @click="answerNavigation(false)">继续编辑</button><button class="btn btn-danger" @click="answerNavigation(true)">放弃并离开</button></template>
    </UiModal>
    <OnboardingTour />
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
	.content { padding: 18px 14px calc(116px + env(safe-area-inset-bottom)); }
  .mobile-nav {
    position: fixed;
    left: 12px;
    right: 12px;
	bottom: max(10px, env(safe-area-inset-bottom));
    z-index: var(--z-sticky);
    display: grid;
	grid-template-columns: none;
	grid-auto-flow: column;
	grid-auto-columns: minmax(62px, 1fr);
	overflow-x: auto;
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
