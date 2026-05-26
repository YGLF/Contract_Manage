import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/store/user'

const directorRoles = ['sales_director', 'tech_director', 'finance_director']
const directorAllowedRoutes = [
  'dashboard',
  'contracts',
  'contracts/:id',
  'contracts/:id/performance',
  'customers',
  'approvals',
  'archives',
  'closures',
  'reminders'
]

const routes = [
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
        meta: { title: '仪表盘', icon: 'DataAnalysis', permission: 'dashboard' }
      },
      {
        path: 'contracts',
        name: 'Contracts',
        component: () => import('@/views/Contract.vue'),
        meta: { title: '合同管理', icon: 'Document', permission: 'contract.read' }
      },
      {
        path: 'contracts/:id',
        name: 'ContractDetail',
        component: () => import('@/views/ContractDetail.vue'),
        meta: { title: '合同详情', hidden: true, permission: 'contract.read' }
      },
      {
        path: 'contracts/:id/performance',
        name: 'ContractPerformance',
        component: () => import('@/views/ContractPerformance.vue'),
        meta: { title: '履约计划', hidden: true, permission: 'contract.read' }
      },
      {
        path: 'customers',
        name: 'Customers',
        component: () => import('@/views/Customer.vue'),
        meta: { title: '客户管理', icon: 'User', permission: 'customer.read' }
      },
      {
        path: 'approvals',
        name: 'Approvals',
        component: () => import('@/views/Approval.vue'),
        meta: { title: '审批管理', icon: 'Check', permission: 'approval.view' }
      },
      {
        path: 'archives',
        name: 'Archives',
        component: () => import('@/views/Archive.vue'),
        meta: { title: '归档管理', icon: 'FolderOpened', permission: 'contract.read' }
      },
      {
        path: 'closures',
        name: 'Closures',
        component: () => import('@/views/Closure.vue'),
        meta: { title: '结案管理', icon: 'Finished', permission: 'contract.read' }
      },
      {
        path: 'reminders',
        name: 'Reminders',
        component: () => import('@/views/Reminder.vue'),
        meta: { title: '到期提醒', icon: 'Bell', permission: 'dashboard' }
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('@/views/User.vue'),
        meta: { title: '用户管理', icon: 'UserFilled', permission: 'user.manage' }
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('@/views/Audit.vue'),
        meta: { title: '审计日志', icon: 'Document', permission: 'audit.view' }
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/'
  }
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes
})

router.beforeEach((to, from, next) => {
  try {
    const userStore = useUserStore()

    if (to.meta.requiresAuth && !userStore.token) {
      next('/login')
      return
    }

    if ((to.path === '/login' || to.path === '/register') && userStore.token) {
      next('/dashboard')
      return
    }

    const userRole = userStore.userInfo?.role || ''
    const isDirector = directorRoles.includes(userRole)

    if (isDirector) {
      const path = to.path.replace(/^\//, '')
      const pathMatches = directorAllowedRoutes.some((allowed) => {
        if (allowed.includes(':')) {
          const pattern = allowed.replace(/:[^/]+/g, '[^/]+')
          const regex = new RegExp(`^${pattern}$`)
          return regex.test(path)
        }
        return path === allowed
      })

      if (!pathMatches && path !== '') {
        next('/dashboard')
        return
      }
    }

    if (to.meta.permission) {
      const hasPermission = userStore.hasPermission(to.meta.permission)
      if (!hasPermission) {
        next('/dashboard')
        return
      }
    }
  } catch (error) {
    console.error('Router error:', error)
  }

  next()
})

export default router
