import { createRouter, createWebHistory } from 'vue-router'
import { onUnauthorized } from '../api/client'
import { fetchSession } from '../api/auth'
import { useSession } from '../composables/useSession'
import AdminLayout from '../components/layout/AdminLayout.vue'
import LoginView from '../views/LoginView.vue'
import DashboardView from '../views/DashboardView.vue'
import ConfigView from '../views/ConfigView.vue'
import PetsView from '../views/PetsView.vue'
import AssetsView from '../views/AssetsView.vue'
import SystemView from '../views/SystemView.vue'

const router = createRouter({
  // 后台挂载在 /admin 路径下，路由基础路径必须与之对齐。
  history: createWebHistory('/admin/'),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      component: AdminLayout,
      children: [
        { path: '', name: 'dashboard', component: DashboardView },
        { path: 'config', name: 'config', component: ConfigView },
        { path: 'pets', name: 'pets', component: PetsView },
        { path: 'assets', name: 'assets', component: AssetsView },
        { path: 'system', name: 'system', component: SystemView },
      ],
    },
    // 未知路径统一回到仪表盘，避免 SPA 深链接落到空白页。
    { path: '/:pathMatch(.*)*', redirect: { name: 'dashboard' } },
  ],
})

const session = useSession()

// 任何接口返回 401 都说明服务端会话已失效：清掉本地会话状态并回登录页，
// 同时记下当前路径，登录成功后可以直接回到原来的位置。
onUnauthorized(() => {
  session.clearSession()
  const current = router.currentRoute.value
  if (current.name === 'login') {
    return
  }
  router.replace({ name: 'login', query: { redirect: current.fullPath } })
})

// 路由守卫：未登录用户不允许访问需要验证的页面。
router.beforeEach(async (to) => {
  if (to.meta.requiresAuth === false) {
    return true
  }

  // authenticated 为 null 表示本次会话尚未探测过，只在首次进入时请求一次接口，
  // 之后的跳转直接读缓存，避免每次导航都打一遍 /auth/session。
  if (session.authenticated.value === null) {
    const info = await fetchSession()
    if (info.authenticated) {
      session.setSession(info.username ?? '')
    } else {
      session.clearSession()
    }
  }

  if (!session.authenticated.value) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
