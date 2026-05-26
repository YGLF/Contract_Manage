import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'

const request = axios.create({
  baseURL: '/api',
  timeout: 15000
})

let isRedirecting = false

request.interceptors.request.use(
  config => {
    try {
      const userStore = useUserStore()
      if (userStore.token) {
        config.headers.Authorization = `Bearer ${userStore.token}`
      }
    } catch (e) {
      console.error('Failed to get user store:', e)
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

request.interceptors.response.use(
  response => {
    return response.data
  },
  error => {
    if (isRedirecting) {
      return Promise.reject(error)
    }
    
    if (error.response) {
      const { status, data } = error.response
      if (status === 401) {
        isRedirecting = true
        try {
          const userStore = useUserStore()
          userStore.logout()
        } catch (e) {
          localStorage.removeItem('token')
          localStorage.removeItem('userInfo')
        }
        ElMessage.error('登录已过期，请重新登录')
        window.location.href = '/login'
      } else if (status === 403) {
        ElMessage.error(data.error || '没有权限访问此资源')
      } else if (status === 404) {
        ElMessage.error('请求的资源不存在')
      } else if (status === 500) {
        ElMessage.error('服务器错误')
      } else {
        ElMessage.error(data.error || data.detail || '请求失败')
      }
    } else if (error.request) {
      ElMessage.error('网络连接失败，请检查网络')
    } else {
      ElMessage.error('请求配置错误')
    }
    return Promise.reject(error)
  }
)

export default request