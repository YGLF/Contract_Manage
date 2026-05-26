import request from '@/utils/request'

export const getClosureRequests = () => {
  return request({
    url: '/closure/requests',
    method: 'get'
  })
}

export const createClosureRequest = (data) => {
  return request({
    url: '/closure/requests',
    method: 'post',
    data
  })
}

export const completeClosureRequest = (id) => {
  return request({
    url: `/closure/requests/${id}/complete`,
    method: 'post'
  })
}
