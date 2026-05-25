<template>
  <el-container class="layout-container">
    <el-aside width="260px">
      <div class="sidebar">
        <div class="logo-area">
          <div class="logo-icon">
            <svg viewBox="0 0 32 32" fill="none">
              <rect width="32" height="32" rx="8" fill="url(#sidebarLogo)"/>
              <path d="M8 10h16v3H8zM8 15h12v3H8zM8 20h16v3H8z" fill="white" opacity="0.9"/>
              <defs>
                <linearGradient id="sidebarLogo" x1="0" y1="0" x2="32" y2="32">
                  <stop stop-color="#6366F1"/>
                  <stop offset="1" stop-color="#8B5CF6"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
          <span class="logo-text">瀹変俊鍚堝悓</span>
        </div>
        
        <el-menu
          :default-active="activeMenu"
          router
          class="sidebar-menu"
        >
          <el-menu-item
            v-for="item in visibleMenuItems"
            :key="item.path"
            :index="item.path"
          >
            <div class="menu-item-content">
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.title }}</span>
              <el-badge
                v-if="item.badge && item.badge.value > 0"
                :value="item.badge.value"
                :max="99"
                class="menu-badge-icon"
                :type="item.badge.type"
              />
            </div>
          </el-menu-item>
        </el-menu>
        
        <div class="sidebar-footer">
          <div class="user-card">
            <el-avatar :size="36" class="user-avatar">
              {{ userStore.userInfo?.username?.charAt(0)?.toUpperCase() }}
            </el-avatar>
            <div class="user-info">
              <div class="user-name">{{ userStore.userInfo?.username }}</div>
              <div class="user-role">{{ getRoleText(userStore.userInfo?.role) }}</div>
            </div>
          </div>
        </div>
      </div>
    </el-aside>
    
    <el-container>
      <el-header>
        <div class="header-left">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">棣栭〉</el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentRoute">{{ currentRoute }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        
        <div class="header-right">
          <div class="header-notifications" @click="handleNotificationClick">
            <el-badge
              :value="totalNotifications"
              :hidden="totalNotifications === 0"
              :max="99"
              type="danger"
            >
              <el-icon :size="20"><Bell /></el-icon>
            </el-badge>
          </div>
          <el-dropdown @command="handleCommand" trigger="click">
            <div class="user-dropdown">
              <el-avatar :size="32" class="header-avatar">
                {{ userStore.userInfo?.username?.charAt(0)?.toUpperCase() }}
              </el-avatar>
              <span class="username">{{ userStore.userInfo?.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  <el-icon><User /></el-icon>
                  涓汉璁剧疆
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  閫€鍑虹櫥褰?
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      
      <el-main>
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from '@/store/user'
import { routes, hasRouteAccess } from '@/router'
import { ElMessageBox } from 'element-plus'
import { getNotificationCounts } from '@/api/approval'
import {
  Odometer, Document, OfficeBuilding, Checked, Bell,
  UserFilled, User, ArrowDown, SwitchButton
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const iconMap = {
  DataAnalysis: Odometer,
  Document,
  User: OfficeBuilding,
  UserFilled,
  Check: Checked,
  Bell
}

const getRoleText = (role) => {
  const textMap = {
    admin: '绠＄悊鍛?,
    manager: '缁忕悊',
    audit_admin: '瀹¤绠＄悊鍛?,
    user: '鏅€氱敤鎴?
  }
  return textMap[role] || role || '鏈煡'
}

const totalNotifications = computed(() => {
  const { pendingApprovals, pendingStatusChanges, expiringContracts } = notificationCounts.value
  return pendingApprovals + pendingStatusChanges + expiringContracts
})

const notificationCounts = ref({
  pendingApprovals: 0,
  pendingStatusChanges: 0,
  expiringContracts: 0,
  total: 0
})

const getMenuBadge = (path) => {
  if (path === '/contracts' || path === '/reminders') {
    return {
      value: notificationCounts.value.expiringContracts,
      type: 'warning'
    }
  }

  if (path === '/approvals') {
    return {
      value: notificationCounts.value.pendingApprovals + notificationCounts.value.pendingStatusChanges,
      type: 'danger'
    }
  }

  return null
}

const visibleMenuItems = computed(() => {
  const layoutRoute = routes.find((record) => record.path === '/')
  const userRole = userStore.userInfo?.role

  return (layoutRoute?.children || [])
    .filter((record) => !record.meta?.hidden)
    .filter((record) => hasRouteAccess(record, userRole))
    .map((record) => {
      const path = record.path.startsWith('/') ? record.path : `/${record.path}`

      return {
        path,
        title: record.meta?.title,
        icon: iconMap[record.meta?.icon] || Document,
        badge: getMenuBadge(path)
      }
    })
})

let notificationTimer = null

const loadNotifications = async () => {
  try {
    const counts = await getNotificationCounts()
    notificationCounts.value = counts
  } catch (error) {
    console.error('Failed to load notifications:', error)
  }
}

onMounted(() => {
  loadNotifications()
  notificationTimer = setInterval(loadNotifications, 30000)
})

onUnmounted(() => {
  if (notificationTimer) {
    clearInterval(notificationTimer)
  }
})

const activeMenu = computed(() => route.path)

const currentRoute = computed(() => route.meta?.title)

const handleNotificationClick = () => {
  router.push('/approvals')
}

const handleCommand = (command) => {
  if (command === 'profile') {
    ElMessageBox.alert(
      `<div style="padding: 10px;">
        <p><strong>鐢ㄦ埛鍚嶏細</strong>${userStore.userInfo?.username || '-'}</p>
        <p><strong>閭锛?/strong>${userStore.userInfo?.email || '-'}</p>
        <p><strong>瑙掕壊锛?/strong>${getRoleText(userStore.userInfo?.role)}</p>
      </div>`,
      '涓汉璁剧疆',
      {
        confirmButtonText: '纭畾',
        dangerouslyUseHTMLString: true,
      }
    )
  } else if (command === 'logout') {
    ElMessageBox.confirm('纭畾瑕侀€€鍑虹櫥褰曞悧锛?, '鎻愮ず', {
      confirmButtonText: '纭畾',
      cancelButtonText: '鍙栨秷',
      type: 'warning'
    }).then(() => {
      userStore.logout()
      router.push('/login')
    })
  }
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.el-aside {
  background: white;
  box-shadow: 2px 0 12px rgba(0, 0, 0, 0.04);
}

.sidebar {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.logo-area {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  border-bottom: 1px solid #F1F5F9;
}

.logo-icon {
  width: 32px;
  height: 32px;
}

.logo-icon svg {
  width: 100%;
  height: 100%;
}

.logo-text {
  font-size: 18px;
  font-weight: 600;
  color: #1E293B;
  letter-spacing: 1px;
}

.sidebar-menu {
  flex: 1;
  border-right: none;
  padding: 12px 0;
}

.menu-item-content {
  display: flex;
  align-items: center;
  position: relative;
  width: 100%;
}

.menu-item-content .el-icon {
  margin-right: 12px;
  font-size: 18px;
  position: relative;
}

.menu-badge-icon {
  position: absolute;
  right: 10px;
  top: -6px;
  transform: translateY(-50%);
}

.menu-badge-icon :deep(.el-badge__content) {
  border: none;
  font-size: 11px;
  padding: 0 5px;
  height: 18px;
  line-height: 18px;
}

:deep(.el-menu-item) {
  height: 48px;
  margin: 4px 12px;
  border-radius: 12px;
  color: #64748B;
  font-weight: 500;
  transition: all 0.2s ease;
  padding: 0 20px !important;
}

.menu-badge {
  position: absolute;
  right: 60px;
  top: 8px;
}

:deep(.el-menu-item:hover) {
  background: #F8FAFC;
  color: #1E293B;
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1) 0%, rgba(139, 92, 246, 0.1) 100%);
  color: #6366F1;
}

:deep(.el-menu-item.is-active .el-icon) {
  color: #6366F1;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

:deep(.el-dropdown-menu__item) {
  padding: 10px 20px;
  font-size: 14px;
}

:deep(.el-dropdown-menu__item .el-icon) {
  margin-right: 8px;
}

.el-header {
  background: white;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
}

.header-left {
  display: flex;
  align-items: center;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-notifications {
  cursor: pointer;
  padding: 8px;
  border-radius: 8px;
  transition: background 0.2s;
}

.header-notifications:hover {
  background: #F8FAFC;
}

.header-notifications .el-icon {
  color: #64748B;
}

.header-notifications:hover .el-icon {
  color: #6366F1;
}

.user-dropdown {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.2s;
}

.user-dropdown:hover {
  background: #F8FAFC;
}

.header-avatar {
  background: linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%);
  color: white;
  font-weight: 600;
  font-size: 12px;
}

.username {
  color: #1E293B;
  font-weight: 500;
  font-size: 14px;
}
</style>
