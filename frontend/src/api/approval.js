import request from '@/utils/request'

export const getApprovalRequests = () => {
  return request({
    url: '/workflows/approval-requests',
    method: 'get'
  })
}

export const createApprovalRequest = (data) => {
  return request({
    url: '/workflows/approval-requests',
    method: 'post',
    data
  })
}

export const approveApprovalRequest = (id, data) => {
  return request({
    url: `/workflows/approval-requests/${id}/approve`,
    method: 'post',
    data
  })
}

export const rejectApprovalRequest = (id, data) => {
  return request({
    url: `/workflows/approval-requests/${id}/reject`,
    method: 'post',
    data
  })
}

export const getPendingApprovals = getApprovalRequests

export const createWorkflow = createApprovalRequest

export const getApprovalRecords = async (contractId) => {
  const rows = await getApprovalRequests()
  const list = Array.isArray(rows) ? rows : []
  return list
    .filter(item => String(item.contract_id) === String(contractId))
    .map(item => ({
      id: item.id,
      status: item.status,
      comment: item.comment || item.payload?.comment || '',
      created_at: item.created_at,
      approved_at: item.approved_at,
      approver: item.approved_by ? { full_name: item.approved_by } : null,
      request_type: item.request_type,
      requested_by: item.requested_by
    }))
}

export const createApproval = (data) => {
  return createApprovalRequest({
    contract_id: data.contract_id,
    request_type: data.request_type || 'status_change',
    requested_by: data.requested_by || 'system',
    resource_id: data.resource_id || '',
    payload: data.payload || {
      status: data.to_status || 'active',
      comment: data.comment || ''
    }
  })
}

export const approveWorkflow = (data) => {
  return approveApprovalRequest(data.workflow_id || data.id, data)
}

export const rejectWorkflow = (data) => {
  return rejectApprovalRequest(data.workflow_id || data.id, data)
}

export const getNotificationCounts = () => {
  return request({
    url: '/notifications/count',
    method: 'get'
  })
}

export const getMyNotifications = () => {
  return request({
    url: '/notifications',
    method: 'get'
  })
}

export const markNotificationRead = (id) => {
  return request({
    url: `/notifications/${id}/read`,
    method: 'put'
  })
}

export const getUnreadNotificationCount = () => {
  return request({
    url: '/notifications/unread-count',
    method: 'get'
  })
}

export const deleteNotification = (id) => {
  return request({
    url: `/notifications/${id}`,
    method: 'delete'
  })
}

export const deleteAllNotifications = () => {
  return request({
    url: '/notifications/all',
    method: 'delete'
  })
}

export const getWorkflowStatus = (contractId) => {
  return request({
    url: `/workflow/${contractId}/status`,
    method: 'get'
  })
}

export const sendApprovalReminder = (contractId) => {
  return request({
    url: `/workflow/${contractId}/remind`,
    method: 'post'
  })
}

export const getStatistics = () => {
  return request({
    url: '/statistics',
    method: 'get'
  })
}

export const createReminder = (data) => {
  return request({
    url: `/contracts/${data.contract_id}/reminders`,
    method: 'post',
    data
  })
}

export const getExpiringContracts = async (days = 30) => {
  const contracts = await request({
    url: '/contracts',
    method: 'get'
  })
  const list = Array.isArray(contracts) ? contracts : []
  const now = new Date()
  const deadline = new Date(now.getTime() + days * 24 * 60 * 60 * 1000)
  const filtered = list.filter(item => {
    if (!item.end_date) return false
    const endDate = new Date(item.end_date)
    if (Number.isNaN(endDate.getTime())) return false
    return endDate >= now && endDate <= deadline
  })
  return { contracts: filtered }
}
