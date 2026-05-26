export const RoleOptions = [
  { label: '超级管理员', value: 'admin' },
  { label: '销售总监', value: 'sales_director' },
  { label: '技术总监', value: 'tech_director' },
  { label: '财务总监', value: 'finance_director' },
  { label: '合同管理员', value: 'contract_admin' },
  { label: '销售人员', value: 'sales' },
  { label: '审计管理员', value: 'audit_admin' }
]

export const AccountStatusOptions = [
  { label: '长期', value: 'permanent' },
  { label: '临时', value: 'temporary' },
  { label: '禁用', value: 'disabled' },
  { label: '指定时间段', value: 'timed' }
]

export const getRoleText = (role) => {
  const option = RoleOptions.find(opt => opt.value === role)
  return option ? option.label : role
}

export const getRoleType = (role) => {
  const typeMap = {
    admin: 'danger',
    sales_director: 'warning',
    tech_director: 'primary',
    finance_director: 'success',
    contract_admin: 'info',
    sales: 'success',
    audit_admin: 'warning'
  }
  return typeMap[role] || 'info'
}

export const getStatusText = (status) => {
  const option = AccountStatusOptions.find(opt => opt.value === status)
  return option ? option.label : status
}

export const getStatusType = (status) => {
  const typeMap = {
    permanent: 'success',
    temporary: 'warning',
    disabled: 'info',
    timed: ''
  }
  return typeMap[status] || 'info'
}

// 密码强度等级
export const PasswordStrengthLevel = {
  WEAK: 0,
  FAIR: 1,
  MEDIUM: 2,
  STRONG: 3
}

// 计算密码强度
export const calculatePasswordStrength = (password) => {
  if (!password) return { level: PasswordStrengthLevel.WEAK, score: 0, feedback: '' }
  
  let score = 0
  const feedback = []
  
  // 长度检查
  if (password.length >= 6) score += 1
  if (password.length >= 8) score += 1
  if (password.length >= 12) score += 1
  
  if (password.length < 6) {
    feedback.push('至少6个字符')
  }
  
  // 字符类型检查
  if (/[a-z]/.test(password)) score += 1
  else feedback.push('包含小写字母')
  
  if (/[A-Z]/.test(password)) score += 1
  else feedback.push('包含大写字母')
  
  if (/[0-9]/.test(password)) score += 1
  else feedback.push('包含数字')
  
  if (/[!@#$%^&*(),.?":{}|<>]/.test(password)) score += 1
  else feedback.push('包含特殊字符')
  
  // 计算等级
  let level = PasswordStrengthLevel.WEAK
  if (score >= 7) level = PasswordStrengthLevel.STRONG
  else if (score >= 5) level = PasswordStrengthLevel.MEDIUM
  else if (score >= 3) level = PasswordStrengthLevel.FAIR
  
  return {
    level,
    score,
    feedback: feedback.slice(0, 2)
  }
}

// 获取密码强度文本和类型
export const getPasswordStrengthInfo = (level) => {
  const infoMap = {
    [PasswordStrengthLevel.WEAK]: { text: '弱', type: 'danger', color: '#ef4444' },
    [PasswordStrengthLevel.FAIR]: { text: '一般', type: 'warning', color: '#f59e0b' },
    [PasswordStrengthLevel.MEDIUM]: { text: '中等', type: '', color: '#3b82f6' },
    [PasswordStrengthLevel.STRONG]: { text: '强', type: 'success', color: '#22c55e' }
  }
  return infoMap[level] || infoMap[PasswordStrengthLevel.WEAK]
}

// 密码强度验证规则
export const passwordValidator = (rule, value, callback) => {
  if (!value) {
    callback(new Error('请输入密码'))
  } else if (value.length < 6) {
    callback(new Error('密码长度至少6位'))
  } else if (value.length > 50) {
    callback(new Error('密码长度不能超过50位'))
  } else {
    callback()
  }
}
