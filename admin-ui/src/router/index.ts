import { createRouter, createWebHistory } from 'vue-router'
import { onUnauthorized } from '../api/client'
import { fetchSession } from '../api/auth'
import { useSession } from '../composables/useSession'
import AdminLayout from '../components/layout/AdminLayout.vue'
import LoginView from '../views/LoginView.vue'
import DashboardView from '../views/DashboardView.vue'
import PlayersView from '../views/PlayersView.vue'
import GameplayView from '../views/GameplayView.vue'
import CommunitiesView from '../views/CommunitiesView.vue'
import ContentView from '../views/ContentView.vue'
import PlatformsView from '../views/PlatformsView.vue'
import SystemView from '../views/SystemView.vue'
import NotFoundView from '../views/NotFoundView.vue'
import SetupView from '../views/SetupView.vue'
import ConfigProfilesView from '../views/ConfigProfilesView.vue'
import AdventureView from '../views/AdventureView.vue'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { requiresAuth: false } },
    { path: '/setup', name: 'setup', component: SetupView, meta: { title: '首次配置' } },
    {
      path: '/', component: AdminLayout, children: [
        { path: '', name: 'dashboard', component: DashboardView, meta: { title: '运营总览' } },
        { path: 'players', name: 'players', component: PlayersView, meta: { title: '玩家管理' } },
        { path: 'gameplay', name: 'gameplay', component: GameplayView, meta: { title: '玩法运营' } },
        { path: 'adventure/:section?', name: 'adventure', component: AdventureView, meta: { title: '冒险世界' } },
        { path: 'communities', name: 'communities', component: CommunitiesView, meta: { title: '社群运营' } },
        { path: 'content', name: 'content', component: ContentView, meta: { title: '内容配置' } },
        { path: 'profiles', name: 'profiles', component: ConfigProfilesView, meta: { title: '配置方案' } },
        { path: 'platforms', name: 'platforms', component: PlatformsView, meta: { title: '平台状态' } },
        { path: 'system', name: 'system', component: SystemView, meta: { title: '系统设置' } },
        { path: 'pets', redirect: { name: 'players' } },
        { path: 'assets', redirect: { name: 'players', query: { panel: 'operations' } } },
        { path: 'config', redirect: { name: 'content' } },
      ],
    },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView, meta: { title: '页面未找到' } },
  ],
})

const session = useSession()
onUnauthorized(() => {
  session.clearSession()
  const current = router.currentRoute.value
  if (current.name !== 'login') router.replace({ name: 'login', query: { redirect: current.fullPath, reason: 'session-expired' } })
})
router.beforeEach(async (to) => {
  if (to.meta.requiresAuth === false) return true
  if (session.authenticated.value === null) {
    const info = await fetchSession()
    if (info.authenticated) session.setSession(info.username ?? '')
    else session.clearSession()
  }
  if (!session.authenticated.value) return { name: 'login', query: { redirect: to.fullPath } }
  return true
})

export default router
