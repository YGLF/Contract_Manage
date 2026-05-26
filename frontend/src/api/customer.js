import request from '@/utils/request'

export const getCustomerList = (params) => {
  const query = {}
  if (params?.name) query.name = params.name
  if (params?.type) query.status = params.type === 'supplier' ? 'inactive' : 'active'
  return request({
    url: '/parties',
    method: 'get',
    params: query
  })
}

export const getCustomerDetail = (id) => {
  return request({
    url: `/parties/${id}`,
    method: 'get'
  })
}

export const createCustomer = (data) => {
  return request({
    url: '/parties',
    method: 'post',
    data: {
      name: data.name,
      unified_social_code: data.code,
      contact_name: data.contact_person,
      contact_phone: data.contact_phone,
      credit_rating: data.credit_rating,
      credit_source: 'frontend_manual',
      status: data.type === 'supplier' ? 'inactive' : 'active'
    }
  })
}

export const updateCustomer = (id, data) => {
  return request({
    url: `/parties/${id}`,
    method: 'put',
    data: {
      name: data.name,
      contact_name: data.contact_person,
      contact_phone: data.contact_phone,
      credit_rating: data.credit_rating,
      credit_source: 'frontend_manual',
      status: data.type === 'supplier' ? 'inactive' : 'active'
    }
  })
}

export const deleteCustomer = (id) => {
  return request({
    url: `/parties/${id}`,
    method: 'delete'
  })
}

export const getContractTypeList = (params) => {
  return request({
    url: '/contract-types',
    method: 'get',
    params
  })
}

export const createContractType = (data) => {
  return request({
    url: '/contract-types',
    method: 'post',
    data
  })
}

export const updateContractType = (id, data) => {
  return request({
    url: `/contract-types/${id}`,
    method: 'put',
    data
  })
}

export const deleteContractType = (id) => {
  return request({
    url: `/contract-types/${id}`,
    method: 'delete'
  })
}
