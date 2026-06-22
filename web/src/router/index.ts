import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/users'
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue')
  },
  {
    path: '/users',
    name: 'Users',
    component: () => import('@/views/Users.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/nodes',
    name: 'Nodes',
    component: () => import('@/views/Nodes.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/servers',
    name: 'Servers',
    component: () => import('@/views/Servers.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/external-nodes',
    name: 'ExternalNodes',
    component: () => import('@/views/ExternalNodes.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/subscription-config',
    name: 'SubscriptionConfig',
    component: () => import('@/views/SubscriptionConfig.vue'),
    meta: { requiresAuth: true }
  }
]

function isTokenExpired(token: string): boolean {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    if (!payload.exp) return false
    // 提前 30 秒判定过期，避免边界情况
    return Date.now() >= payload.exp * 1000 - 30000
  } catch {
    return true // 无法解析 → 视为过期
  }
}

function clearExpiredToken(): string | null {
  const token = localStorage.getItem('token')
  if (token && isTokenExpired(token)) {
    localStorage.removeItem('token')
    return null
  }
  return token
}

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫 — 验证 token 存在且未过期
router.beforeEach((to, from, next) => {
  const token = clearExpiredToken()

  if (to.meta.requiresAuth && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/users')
  } else {
    next()
  }
})

export default router
