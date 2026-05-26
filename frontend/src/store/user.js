import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const userInfo = ref(JSON.parse(localStorage.getItem('userInfo') || 'null'))
  const notifications = ref([])
  const unreadCount = ref(0)

  const setToken = (newToken) => {
    token.value = newToken
    if (newToken) {
      localStorage.setItem('token', newToken)
    } else {
      localStorage.removeItem('token')
    }
  }

  const setUserInfo = (info) => {
    userInfo.value = info
    if (info) {
      localStorage.setItem('userInfo', JSON.stringify(info))
    } else {
      localStorage.removeItem('userInfo')
    }
  }

  const setNotifications = (notifs, count) => {
    notifications.value = notifs || []
    unreadCount.value = count || 0
  }

  const clearNotifications = () => {
    notifications.value = []
    unreadCount.value = 0
  }

  const logout = () => {
    token.value = ''
    userInfo.value = null
    notifications.value = []
    unreadCount.value = 0
    localStorage.removeItem('token')
    localStorage.removeItem('userInfo')
  }

  const permissions = computed(() => {
    if (!userInfo.value) return []
    return userInfo.value.permissions || []
  })

  const hasPermission = (permission) => {
    if (!permission) return false
    const userPermissions = permissions.value
    if (!userPermissions || userPermissions.length === 0) return false
    return userPermissions.includes('all') || userPermissions.includes(permission)
  }

  const hasAnyPermission = (permissionList) => {
    if (!permissionList || permissionList.length === 0) return false
    const userPermissions = permissions.value
    if (!userPermissions || userPermissions.length === 0) return false
    if (userPermissions.includes('all')) return true
    return permissionList.some(p => userPermissions.includes(p))
  }

  return {
    token,
    userInfo,
    notifications,
    unreadCount,
    permissions,
    setToken,
    setUserInfo,
    setNotifications,
    clearNotifications,
    logout,
    hasPermission,
    hasAnyPermission
  }
})