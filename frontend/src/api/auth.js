import request from '@/utils/request'

export const login = (data) => {
  return request({
    url: '/auth/login',
    method: 'post',
    data
  })
}

export const register = (data) => {
  return request({
    url: '/auth/register',
    method: 'post',
    data
  })
}

export const getUserList = (params) => {
  return request({
    url: '/auth/users',
    method: 'get',
    params
  })
}

export const getUserDetail = (id) => {
  return request({
    url: `/auth/users/${id}`,
    method: 'get'
  })
}

export const updateUser = (id, data) => {
  return request({
    url: `/auth/users/${id}`,
    method: 'put',
    data
  })
}

export const deleteUser = (id) => {
  return request({
    url: `/auth/users/${id}`,
    method: 'delete'
  })
}

export const resetUserPassword = (id) => {
  return request({
    url: `/auth/users/${id}/reset-password`,
    method: 'post'
  })
}

export const unlockUser = (id) => {
  return request({
    url: `/auth/users/${id}/unlock`,
    method: 'post'
  })
}

export const getUserLockStatus = (id) => {
  return request({
    url: `/auth/users/${id}/lock-status`,
    method: 'get'
  })
}