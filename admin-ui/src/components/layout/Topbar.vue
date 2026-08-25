<script setup lang="ts">
import {
  IconBolt,
  IconChevronDown,
  IconLayoutSidebarLeftExpand,
  IconLogout,
  IconHelpCircle,
  IconUserCircle,
  IconSpacingHorizontal,
} from '@tabler/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { logout } from '../../api/auth'
import { reloadConfigs } from '../../api/config'
import { useDensity } from '../../composables/useDensity'
import { useSession } from '../../composables/useSession'
import { useShellLayout } from '../../composables/useShellLayout'
import { useTheme } from '../../composables/useTheme'
import { useToast } from '../../composables/useToast'

const router = useRouter()
const route = useRoute()
const { theme, themes, setTheme } = useTheme()
const { density, densities, setDensity } = useDensity()
const { username, clearSession } = useSession()
const shell = useShellLayout()
const toast = useToast()
const title = computed(() => String(route.meta.title || '后台管理'))

const reloading = ref(false)
const accountMenuOpen = ref(false)
const accountRoot = ref<HTMLElement | null>(null)
const accountTrigger = ref<HTMLButtonElement | null>(null)
const logoutItem = ref<HTMLButtonElement | null>(null)
const densityOpen = ref(false)
const densityRoot = ref<HTMLElement | null>(null)

function closeAccountMenu({ restoreFocus = false } = {}) {
  accountMenuOpen.value = false
  if (restoreFocus) {
    nextTick(() => accountTrigger.value?.focus())
  }
}

function toggleAccountMenu() {
  accountMenuOpen.value = !accountMenuOpen.value
}

async function openAccountMenuFromKeyboard() {
  accountMenuOpen.value = true
  await nextTick()
  logoutItem.value?.focus()
}

function handlePointerDown(event: PointerEvent) {
  if (accountMenuOpen.value && !accountRoot.value?.contains(event.target as Node)) {
    closeAccountMenu()
  }
	if (densityOpen.value && !densityRoot.value?.contains(event.target as Node)) densityOpen.value = false
}

onMounted(() => document.addEventListener('pointerdown', handlePointerDown))
onBeforeUnmount(() => document.removeEventListener('pointerdown', handlePointerDown))

async function handleLogout() {
  closeAccountMenu()
  try {
    await logout()
  } finally {
    clearSession()
    router.replace({ name: 'login' })
  }
}

async function handleReload() {
  if (reloading.value) return
  reloading.value = true
  try {
    toast.success(await reloadConfigs())
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '热重载失败')
  } finally {
    reloading.value = false
  }
}

function replayTour() {
  closeAccountMenu()
  window.dispatchEvent(new CustomEvent('qqpet:replay-tour'))
}
</script>

<template>
  <header class="topbar">
    <div class="left">
      <button class="ui-icon-button topbar-action menu-button" type="button" aria-label="打开导航" @click="shell.openDrawer">
        <IconLayoutSidebarLeftExpand :size="20" />
      </button>
      <div class="page-context">
        <span>运营控制台</span>
        <strong class="page-title">{{ title }}</strong>
      </div>
      <div class="theme-switch" role="group" aria-label="主题切换">
        <button
          v-for="item in themes"
          :key="item.id"
          class="btn-ghost topbar-action theme-option"
          :class="{ active: theme === item.id }"
          type="button"
          @click="setTheme(item.id)"
        >
          {{ item.label }}
        </button>
      </div>
    </div>

    <div class="account-actions">
      <div ref="densityRoot" class="density-control">
        <button class="ui-icon-button topbar-action density-button" type="button" aria-label="页面密度" aria-haspopup="menu" :aria-expanded="densityOpen" @click="densityOpen = !densityOpen">
          <IconSpacingHorizontal :size="19" />
        </button>
        <div v-if="densityOpen" class="density-popover" role="menu" aria-label="页面密度">
          <span>页面密度</span>
          <button v-for="item in densities" :key="item.id" type="button" role="menuitemradio" :aria-checked="density === item.id" :class="{ active: density === item.id }" @click="setDensity(item.id); densityOpen = false">
            <i aria-hidden="true" />
            <b>{{ item.label }}</b>
            <small>{{ item.id === 'comfortable' ? '留白更多，阅读轻松' : '同屏展示更多内容' }}</small>
          </button>
        </div>
      </div>
      <button
        data-tour="hot-reload"
        class="btn btn-ghost topbar-action reload-btn"
        type="button"
        :disabled="reloading"
        title="将数据库中的配置同步到机器人运行内存"
        @click="handleReload"
      >
        <IconBolt :size="17" />
        <span class="action-label">{{ reloading ? '重载中…' : '热重载' }}</span>
      </button>
      <div ref="accountRoot" class="account">
        <button
          ref="accountTrigger"
          class="btn btn-ghost topbar-action account-trigger"
          type="button"
          aria-label="账号菜单"
          aria-haspopup="menu"
          :aria-expanded="accountMenuOpen"
          aria-controls="account-menu"
          @click="toggleAccountMenu"
          @keydown.arrow-down.prevent="openAccountMenuFromKeyboard"
          @keydown.escape.prevent="closeAccountMenu({ restoreFocus: true })"
        >
          <IconUserCircle :size="19" aria-hidden="true" />
          <span class="account-name">{{ username || 'admin' }}</span>
          <IconChevronDown class="account-chevron" :size="15" aria-hidden="true" />
        </button>
        <div v-if="accountMenuOpen" id="account-menu" class="account-menu" role="menu" aria-label="账号菜单">
          <button class="account-menu-item help-item" type="button" role="menuitem" @click="replayTour">
            <IconHelpCircle :size="18" aria-hidden="true" />
            <span>重新播放功能导览</span>
          </button>
          <button
            ref="logoutItem"
            class="logout-item"
            type="button"
            role="menuitem"
            @click="handleLogout"
            @keydown.escape.prevent="closeAccountMenu({ restoreFocus: true })"
          >
            <IconLogout :size="18" aria-hidden="true" />
            <span>退出登录</span>
          </button>
        </div>
      </div>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 60px;
  padding: 10px var(--page-padding);
  border-bottom: 1px solid var(--border-color);
  background: color-mix(in srgb, var(--bg-base) 86%, transparent);
  backdrop-filter: blur(16px);
}

.left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.menu-button { display: none; }
.page-context { display: grid; min-width: 0; }
.page-context span { color: var(--text-muted); font-size: 10px; letter-spacing: .08em; text-transform: uppercase; }
.page-context strong { overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }

.topbar-action {
  height: 40px;
  white-space: nowrap;
}

.theme-switch {
  display: flex;
  gap: 4px;
  padding: 3px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  background: var(--bg-base);
}

.theme-option {
  padding-inline: 12px;
  border: none;
  border-radius: 9px;
  font-size: 12px;
}

.theme-option.active {
  color: var(--accent-ink);
  background-color: var(--accent);
}

.account-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.reload-btn {
  font-size: 13px;
  padding: 0.45rem 0.9rem;
}

.density-control { position: relative; }
.density-button { width: 40px; padding: 0; color: var(--text-muted); }
.density-button[aria-expanded='true'] { border-color: var(--accent); color: var(--accent); background: var(--accent-soft); }
.density-popover { position:absolute;top:calc(100% + 8px);right:0;z-index:calc(var(--z-sticky) + 1);display:grid;gap:5px;width:246px;padding:8px;border:1px solid var(--border-color);border-radius:14px;background:var(--bg-base);box-shadow:0 16px 40px color-mix(in srgb,var(--text-primary) 14%,transparent) }
.density-popover>span { padding:5px 8px 3px;color:var(--text-muted);font-size:11px;font-weight:700;letter-spacing:.06em }
.density-popover button { display:grid;grid-template-columns:14px 1fr;gap:1px 9px;padding:9px;border:0;border-radius:10px;background:transparent;color:var(--text-main);font:inherit;text-align:left;cursor:pointer }
.density-popover button:hover,.density-popover button.active { background:var(--bg-hover) }
.density-popover i { grid-row:1/3;align-self:center;width:10px;height:10px;border:2px solid var(--border-strong);border-radius:50% }
.density-popover button.active i { border-color:var(--accent);box-shadow:inset 0 0 0 2px var(--bg-hover);background:var(--accent) }
.density-popover b { font-size:13px }.density-popover small { color:var(--text-muted);font-size:11px }

.account {
  position: relative;
}

.account-trigger {
  gap: 7px;
  padding-inline: 11px;
}

.account-name {
  font-size: 13px;
  color: inherit;
}

.account-chevron {
  transition: transform 160ms ease;
}

.account-trigger[aria-expanded='true'] .account-chevron {
  transform: rotate(180deg);
}

.account-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: calc(var(--z-sticky) + 1);
  min-width: 164px;
  padding: 6px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-base);
  box-shadow: 0 12px 32px color-mix(in srgb, var(--text-primary) 12%, transparent);
}

.logout-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 40px;
  padding: 0 10px;
  border: 0;
  border-radius: 8px;
  color: var(--danger);
  background: transparent;
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.account-menu-item { display:flex;align-items:center;gap:8px;width:100%;min-height:40px;padding:0 10px;border:0;border-radius:8px;background:transparent;cursor:pointer;font:inherit;text-align:left; }

.logout-item:hover,
.logout-item:focus-visible {
  color: var(--danger);
  background: var(--danger-soft);
  outline: none;
}
.help-item { color: var(--text-main); }
.help-item:hover, .help-item:focus-visible { color: var(--text-main); background: var(--bg-hover); }

@media (max-width: 1024px) {
  .theme-switch { display: none; }
  .menu-button { display: grid; }
  .action-label,
  .account-name,
  .account-chevron { display: none; }
  .density-button,
  .reload-btn,
  .account-trigger { width: 40px; padding: 0; }
}

@media (max-width: 700px) {
  .topbar { min-height: 58px; padding-inline: 14px; }
  .density-button,
  .reload-btn { display: none; }
  .left { flex: 1; overflow: hidden; }
  .page-context span { display: none; }
}
</style>
