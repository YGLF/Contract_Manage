<template>
  <el-container class="layout-container">
    <el-aside width="260px">
      <div class="sidebar">
        <div class="logo-area">
          <div class="logo-icon">
            <img src="/log.png" alt="logo" class="logo-img" />
          </div>
          <span class="logo-text">安信合同</span>
        </div>

        <el-menu :default-active="activeMenu" router class="sidebar-menu">
          <el-menu-item v-if="hasPermission('dashboard')" index="/dashboard">
            <div class="menu-item-content">
              <el-icon><Odometer /></el-icon>
              <span>仪表盘</span>
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('contract.read')" index="/contracts">
            <div class="menu-item-content">
              <el-icon><Document /></el-icon>
              <span>合同管理</span>
              <el-badge
                v-if="notificationCounts.expiringContracts > 0"
                :value="notificationCounts.expiringContracts"
                :max="99"
                class="menu-badge-icon"
                type="warning"
              />
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('customer.read')" index="/customers">
            <div class="menu-item-content">
              <el-icon><OfficeBuilding /></el-icon>
              <span>客户管理</span>
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('approval.view')" index="/approvals">
            <div class="menu-item-content">
              <el-icon><Checked /></el-icon>
              <span>审批管理</span>
              <el-badge
                v-if="(notificationCounts.pendingApprovals || 0) + (notificationCounts.pendingStatusChanges || 0) > 0"
                :value="(notificationCounts.pendingApprovals || 0) + (notificationCounts.pendingStatusChanges || 0)"
                :max="99"
                class="menu-badge-icon"
                type="danger"
              />
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('contract.read')" index="/archives">
            <div class="menu-item-content">
              <el-icon><FolderOpened /></el-icon>
              <span>归档管理</span>
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('contract.read')" index="/closures">
            <div class="menu-item-content">
              <el-icon><Finished /></el-icon>
              <span>结案管理</span>
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('dashboard')" index="/reminders">
            <div class="menu-item-content">
              <el-icon><Bell /></el-icon>
              <span>到期提醒</span>
              <el-badge
                v-if="notificationCounts.expiringContracts > 0"
                :value="notificationCounts.expiringContracts"
                :max="99"
                class="menu-badge-icon"
                type="warning"
              />
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('user.manage')" index="/users">
            <div class="menu-item-content">
              <el-icon><UserFilled /></el-icon>
              <span>用户管理</span>
            </div>
          </el-menu-item>
          <el-menu-item v-if="hasPermission('audit.view')" index="/audit">
            <div class="menu-item-content">
              <el-icon><Document /></el-icon>
              <span>审计日志</span>
            </div>
          </el-menu-item>
        </el-menu>

        <div class="sidebar-footer">
          <div class="user-card">
            <el-avatar :size="36" class="user-avatar">
              {{ userStore.userInfo?.username?.charAt(0)?.toUpperCase() || 'U' }}
            </el-avatar>
            <div class="user-info">
              <div class="user-name">{{ userStore.userInfo?.username || '用户' }}</div>
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
            <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentRoute">{{ currentRoute }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="header-right">
          <div class="header-notifications" @click="handleNotificationClick">
            <el-icon :size="20"><Bell /></el-icon>
            <span v-if="unreadNotificationCount > 0" class="notification-red-dot"></span>
          </div>

          <el-dialog v-model="notificationDialogVisible" title="我的通知" width="500px">
            <div v-if="myNotifications.length === 0" class="empty-notice">暂无通知</div>
            <el-scrollbar v-else max-height="400px">
              <div
                v-for="notif in myNotifications"
                :key="notif.id"
                :class="['notification-item', { unread: !notif.is_read }]"
                @click="handleNotificationItemClick(notif)"
              >
                <div class="notification-icon">
                  <el-icon v-if="notif.type === 'rejected'" color="#f56c6c"><Close /></el-icon>
                  <el-icon v-else-if="notif.type === 'approved'" color="#67c23a"><Check /></el-icon>
                  <el-icon v-else color="#409eff"><Bell /></el-icon>
                </div>
                <div class="notification-content">
                  <div class="notification-title">{{ notif.title }}</div>
                  <div class="notification-text">{{ notif.content }}</div>
                  <div class="notification-time">{{ formatNotificationTime(notif.created_at) }}</div>
                </div>
                <el-tag v-if="!notif.is_read" type="danger" size="small">未读</el-tag>
              </div>
            </el-scrollbar>
            <template #footer>
              <div class="dialog-footer spaced">
                <el-button v-if="myNotifications.length > 0" type="danger" plain @click="markAllReadAndClear">全部已阅</el-button>
                <div v-else></div>
                <el-button type="primary" @click="notificationDialogVisible = false">关闭</el-button>
              </div>
            </template>
          </el-dialog>

          <el-dropdown @command="handleCommand" trigger="click">
            <div class="user-dropdown">
              <el-avatar :size="32" class="header-avatar">
                {{ userStore.userInfo?.username?.charAt(0)?.toUpperCase() || 'U' }}
              </el-avatar>
              <span class="username">{{ userStore.userInfo?.username || '用户' }}</span>
              <el-icon><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  <el-icon><User /></el-icon>
                  个人设置
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteAllNotifications, getMyNotifications, getNotificationCounts, getUnreadNotificationCount, markNotificationRead } from '@/api/approval'
import {
  ArrowDown,
  Bell,
  Check,
  Checked,
  Close,
  Document,
  Finished,
  FolderOpened,
  Odometer,
  OfficeBuilding,
  SwitchButton,
  User,
  UserFilled
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const notificationCounts = ref({
  pendingApprovals: 0,
  pendingStatusChanges: 0,
  expiringContracts: 0,
  total: 0
})

const notificationDialogVisible = ref(false)
const myNotifications = ref([])
const unreadNotificationCount = ref(0)

let notificationTimer = null

const hasPermission = (permission) => {
  if (!userStore.userInfo) return false
  return userStore.hasPermission(permission)
}

const activeMenu = computed(() => route.path)

const routeNames = {
  '/dashboard': '仪表盘',
  '/contracts': '合同管理',
  '/customers': '客户管理',
  '/approvals': '审批管理',
  '/archives': '归档管理',
  '/closures': '结案管理',
  '/reminders': '到期提醒',
  '/users': '用户管理',
  '/audit': '审计日志'
}

const currentRoute = computed(() => routeNames[route.path])

const getRoleText = (role) => {
  const textMap = {
    admin: '超级管理员',
    manager: '经理',
    user: '业务人员',
    audit_admin: '审计管理员'
  }
  return textMap[role] || '用户'
}

const loadNotifications = async () => {
  try {
    const counts = await getNotificationCounts()
    if (counts) {
      notificationCounts.value = {
        pendingApprovals: counts.pendingApprovals || 0,
        pendingStatusChanges: counts.pendingStatusChanges || 0,
        expiringContracts: counts.expiringContracts || 0,
        total: counts.total || 0
      }
    }
  } catch (error) {
    console.error('Failed to load notifications:', error)
  }
}

const handleNotificationClick = async () => {
  notificationDialogVisible.value = true
  try {
    const data = await getMyNotifications()
    myNotifications.value = data || []
    const countData = await getUnreadNotificationCount()
    unreadNotificationCount.value = countData?.count || 0
  } catch (error) {
    console.error('Failed to load notifications:', error)
  }
}

const handleNotificationItemClick = async (notif) => {
  if (!notif.is_read) {
    try {
      await markNotificationRead(notif.id)
      unreadNotificationCount.value = Math.max(0, unreadNotificationCount.value - 1)
    } catch (error) {
      console.error('Failed to mark notification read:', error)
    }
  }

  const index = myNotifications.value.findIndex(item => item.id === notif.id)
  if (index > -1) {
    myNotifications.value.splice(index, 1)
  }

  if (notif.contract_id) {
    notificationDialogVisible.value = false
    router.push(`/contracts/${notif.contract_id}`)
  }
}

const markAllReadAndClear = async () => {
  try {
    await deleteAllNotifications()
    myNotifications.value = []
    unreadNotificationCount.value = 0
    ElMessage.success('全部已阅')
  } catch (error) {
    console.error('Failed to delete notifications:', error)
    ElMessage.error('操作失败')
  }
}

const formatNotificationTime = (dateStr) => {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return dateStr
  const now = new Date()
  const diff = now - date
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`
  return date.toLocaleDateString('zh-CN')
}

const handleCommand = (command) => {
  if (command === 'profile') {
    ElMessageBox.alert(
      `<div style="padding: 10px;">
        <p><strong>用户名：</strong>${userStore.userInfo?.username || '-'}</p>
        <p><strong>邮箱：</strong>${userStore.userInfo?.email || '-'}</p>
        <p><strong>角色：</strong>${getRoleText(userStore.userInfo?.role)}</p>
      </div>`,
      '个人设置',
      {
        confirmButtonText: '确定',
        dangerouslyUseHTMLString: true
      }
    )
    return
  }

  if (command === 'logout') {
    ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }).then(() => {
      userStore.logout()
      router.push('/login')
    })
  }
}

onMounted(() => {
  loadNotifications()
  notificationTimer = setInterval(loadNotifications, 8000)
})

onUnmounted(() => {
  if (notificationTimer) {
    clearInterval(notificationTimer)
  }
})
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
  border-bottom: 1px solid #f1f5f9;
}

.logo-icon {
  width: 32px;
  height: 32px;
}

.logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.logo-text {
  font-size: 18px;
  font-weight: 600;
  color: #1e293b;
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
  color: #64748b;
  font-weight: 500;
  transition: all 0.2s ease;
  padding: 0 20px !important;
}

:deep(.el-menu-item:hover) {
  background: #f8fafc;
  color: #1e293b;
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1) 0%, rgba(139, 92, 246, 0.1) 100%);
  color: #6366f1;
}

:deep(.el-menu-item.is-active .el-icon) {
  color: #6366f1;
}

.sidebar-footer {
  padding: 16px;
}

.user-card {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-info {
  min-width: 0;
}

.user-name {
  color: #1e293b;
  font-weight: 600;
}

.user-role {
  color: #64748b;
  font-size: 12px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
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
  position: relative;
}

.header-notifications:hover {
  background: #f8fafc;
}

.header-notifications .el-icon {
  color: #64748b;
}

.header-notifications:hover .el-icon {
  color: #6366f1;
}

.notification-red-dot {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 8px;
  height: 8px;
  background-color: #ef4444;
  border-radius: 50%;
  animation: pulse 2s infinite;
  z-index: 10;
}

@keyframes pulse {
  0% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.2);
    opacity: 0.8;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
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
  background: #f8fafc;
}

.header-avatar {
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  color: white;
  font-weight: 600;
  font-size: 12px;
}

.username {
  color: #1e293b;
  font-weight: 500;
  font-size: 14px;
}

.empty-notice {
  text-align: center;
  color: #909399;
  padding: 20px;
}

.notification-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background 0.2s;
}

.notification-item:hover {
  background: #f5f7fa;
}

.notification-item.unread {
  background: #f0f9ff;
}

.notification-icon {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #f5f7fa;
}

.notification-content {
  flex: 1;
  min-width: 0;
}

.notification-title {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
  margin-bottom: 4px;
}

.notification-text {
  font-size: 13px;
  color: #606266;
  line-height: 1.5;
  margin-bottom: 4px;
}

.notification-time {
  font-size: 12px;
  color: #909399;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.dialog-footer.spaced {
  justify-content: space-between;
}
</style>
