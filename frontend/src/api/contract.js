import request from '@/utils/request'
import axios from 'axios'
import { useUserStore } from '@/store/user'

export const getContractList = (params) => {
  return request({
    url: '/contracts',
    method: 'get',
    params
  })
}

export const getContracts = getContractList

export const getContractDetail = (id) => {
  return request({
    url: `/contracts/${id}`,
    method: 'get'
  })
}

export const createContract = (data) => {
  return request({
    url: '/contracts',
    method: 'post',
    data
  })
}

export const intakeContract = (data) => {
  return request({
    url: '/contracts/intake',
    method: 'post',
    data
  })
}

export const updateContract = (id, data) => {
  return request({
    url: `/contracts/${id}`,
    method: 'put',
    data
  })
}

export const deleteContract = (id) => {
  return request({
    url: `/contracts/${id}`,
    method: 'delete'
  })
}

export const getContractLifecycle = (contractId) => {
  return request({
    url: `/contracts/${contractId}/lifecycle`,
    method: 'get'
  })
}

export const updateContractStatus = (contractId, data) => {
  return request({
    url: `/contracts/${contractId}/status`,
    method: 'post',
    data
  })
}

export const getDocumentTempList = () => {
  return request({
    url: '/documents/temp',
    method: 'get'
  })
}

export const getDocumentTempDetail = (id) => {
  return request({
    url: `/documents/temp/${id}`,
    method: 'get'
  })
}

export const downloadTempDocument = async (id) => {
  let token = ''
  try {
    token = useUserStore().token || ''
  } catch {
    token = ''
  }
  const response = await axios.get(`/api/documents/temp/${id}/download`, {
    responseType: 'blob',
    headers: token ? { Authorization: `Bearer ${token}` } : {}
  })
  return {
    blob: response.data,
    contentDisposition: response.headers['content-disposition'] || ''
  }
}

export const uploadTempDocument = (file) => {
  const formData = new FormData()
  formData.append('file', file)
  return request({
    url: '/documents/temp',
    method: 'post',
    data: formData,
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}

export const commitTempDocument = (tempDocumentId) => {
  return request({
    url: '/documents/commit',
    method: 'post',
    data: {
      temp_document_id: tempDocumentId
    }
  })
}

export const getContractDocuments = async (contractId) => {
  const contract = await getContractDetail(contractId)
  const documentIds = Array.isArray(contract?.document_ids) ? contract.document_ids : []
  if (documentIds.length === 0) {
    return []
  }
  const docs = await Promise.all(
    documentIds.map(async (id) => {
      try {
        return await getDocumentTempDetail(id)
      } catch {
        return {
          id,
          file_name: id,
          status: 'missing'
        }
      }
    })
  )
  return docs.map((item) => ({
    id: item.id,
    name: item.file_name || item.id,
    file_type: item.file_name?.includes('.') ? item.file_name.split('.').pop() : '-',
    file_size: item.size || 0,
    version: item.status || '-',
    created_at: item.created_at,
    bound_contract_id: item.bound_contract_id || ''
  }))
}
