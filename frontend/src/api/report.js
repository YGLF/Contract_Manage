import request from '@/utils/request'

export const getDashboardReport = () => {
  return request({
    url: '/reports/dashboard',
    method: 'get'
  })
}

export const getWorkbenchReport = () => {
  return request({
    url: '/reports/workbench',
    method: 'get'
  })
}
