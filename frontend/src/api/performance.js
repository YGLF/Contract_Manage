import request from '@/utils/request'

export const getContractPlans = (contractId) => {
  return request({
    url: `/performance/contracts/${contractId}/plans`,
    method: 'get'
  })
}

export const getLatestPlanVersion = (contractId) => {
  return request({
    url: `/performance/contracts/${contractId}/plan-versions/latest`,
    method: 'get'
  })
}

export const createPlanVersion = (contractId, nodes) => {
  return request({
    url: `/performance/contracts/${contractId}/plan-versions`,
    method: 'post',
    data: { nodes }
  })
}

export const getExecutionRecords = (contractId) => {
  return request({
    url: `/performance/contracts/${contractId}/executions`,
    method: 'get'
  })
}

export const createExecutionRecord = (contractId, data) => {
  return request({
    url: `/performance/contracts/${contractId}/executions`,
    method: 'post',
    data
  })
}

export const getPerformanceSummary = (contractId) => {
  return request({
    url: `/performance/contracts/${contractId}/performance-summary`,
    method: 'get'
  })
}
