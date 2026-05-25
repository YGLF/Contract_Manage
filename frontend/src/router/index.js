import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/store/user'

export const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { title: '登录' }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register.vue'),
    meta: { title: '注册' }
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: '仪表盘', icon: 'DataAnalysis' }
      },
      {
        path: 'contracts',
        name: 'Contracts',
        component: () => import('@/views/Contract.vue'),
        meta: { title: '合同管理', icon: 'Document' }
      },
      {
        path: 'contracts/:id',
        name: 'ContractDetail',
        component: () => import('@/views/ContractDetail.vue'),
        meta: { title: '合同详情', hidden: true }
      },
      {
        path: 'customers',
        name: 'Customers',
        component: () => import('@/views/Customer.vue'),
        meta: { title: '客户管理', icon: 'User' }
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('@/views/User.vue'),
        meta: { title: '用户管理', icon: 'UserFilled', roles: ['admin'] }
      },
      {
        path: 'approvals',
        name: 'Approvals',
        component: () => import('@/views/Approval.vue'),
        meta: { title: '审批管理', icon: 'Check' }
      },
      {
        path: 'reminders',
        name: 'Reminders',
        component: () => import('@/views/Reminder.vue'),
        meta: { title: '到期提醒', icon: 'Bell' }
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('@/views/Audit.vue'),
        meta: { title: '审计日志', icon: 'Document', roles: ['admin', 'audit_admin'] }
      }
    ]
  }
]

export const hasRouteAccess = (route, role) => {
  const records = route.matched?.length ? route.matched : [route]

  return records.every((record) => {
    const allowedRoles = record.meta?.roles
    return !allowedRoles?.length || (role && allowedRoles.includes(role))
  })
}

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const userStore = useUserStore()
  const userRole = userStore.userInfo?.role
  const requiresAuth = to.matched.some((record) => record.meta?.requiresAuth)
  
  if (requiresAuth && !userStore.token) {
    next('/login')
  } else if ((to.path === '/login' || to.path === '/register') && userStore.token) {
    next('/')
  } else if (!hasRouteAccess(to, userRole)) {
    next('/dashboard')
  } else {
    next()
  }
})

export default router
