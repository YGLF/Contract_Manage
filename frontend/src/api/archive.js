import request from '@/utils/request'

export const getArchiveCases = () => {
  return request({
    url: '/archive/cases',
    method: 'get'
  })
}

export const createArchiveCase = (data) => {
  return request({
    url: '/archive/cases',
    method: 'post',
    data
  })
}
