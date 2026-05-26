<template>
  <div class="user-page">
    <!-- 页面标题区 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">用户管理</h2>
        <p class="page-desc">管理系统用户账号、角色权限和访问控制</p>
      </div>
      <el-button type="primary" size="large" @click="handleAdd">
        <el-icon><Plus /></el-icon> 新增用户
      </el-button>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-cards">
      <div class="stat-card">
        <div class="stat-icon users">
          <el-icon><User /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ pagination.total }}</span>
          <span class="stat-label">用户总数</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon admins">
          <el-icon><Avatar /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ adminCount }}</span>
          <span class="stat-label">管理员</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon active">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ activeCount }}</span>
          <span class="stat-label">正常账号</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon locked">
          <el-icon><Lock /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ lockedCount }}</span>
          <span class="stat-label">临时/禁用</span>
        </div>
      </div>
    </div>

    <!-- 搜索筛选区 -->
    <div class="search-section">
      <el-card shadow="never" class="search-card">
        <el-form :inline="true" :model="searchForm" class="search-form">
          <el-form-item label="用户名">
            <el-input 
              v-model="searchForm.username" 
              placeholder="请输入用户名" 
              clearable 
              prefix-icon="Search"
              class="search-input"
            />
          </el-form-item>
          <el-form-item label="角色">
            <el-select v-model="searchForm.role" placeholder="请选择角色" clearable class="role-select">
              <el-option v-for="role in RoleOptions" :key="role.value" :label="role.label" :value="role.value" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleSearch">
              <el-icon><Search /></el-icon> 查询
            </el-button>
            <el-button @click="handleReset">
              <el-icon><Refresh /></el-icon> 重置
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </div>

<!-- 数据表格 -->
  <div class="table-section">
    <el-card shadow="never" class="table-card">
      <el-table
        :data="tableData"
        style="width: 100%"
        v-loading="loading"
        stripe
        :cell-style="{ padding: '8px 0' }"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: '600' }"
        :row-class-name="tableRowClassName"
      >
        <el-table-column prop="username" label="用户名" min-width="140">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :size="28" class="user-avatar">
                {{ row.username?.charAt(0)?.toUpperCase() }}
              </el-avatar>
              <span class="username-text">{{ row.username }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="full_name" label="姓名" min-width="100">
          <template #default="{ row }">
            <span class="fullname">{{ row.full_name || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="180">
          <template #default="{ row }">
            <span class="email-text">{{ row.email || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="role" label="角色" min-width="100">
          <template #default="{ row }">
            <el-tag :type="getRoleType(row.role)" effect="light" round size="small">
              {{ getRoleText(row.role) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="department" label="部门" min-width="100">
          <template #default="{ row }">
            <span class="dept-text">{{ row.department || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="account_status" label="账号状态" min-width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.account_status)" size="small" effect="light">
                <el-icon v-if="row.account_status === 'permanent'"><CircleCheck /></el-icon>
                <el-icon v-else-if="row.account_status === 'temporary'"><Clock /></el-icon>
                <el-icon v-else-if="row.account_status === 'disabled'"><Lock /></el-icon>
                <el-icon v-else><Timer /></el-icon>
                {{ getStatusText(row.account_status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="有效期" min-width="150">
            <template #default="{ row }">
              <span v-if="row.account_status === 'temporary'" class="validity-text">
                <el-tooltip :content="'开始时间: ' + (row.valid_from ? formatDate(row.valid_from) : '立即生效')" placement="top">
                  <span>{{ row.valid_hours ? row.valid_hours + '小时' : '-' }}</span>
                </el-tooltip>
              </span>
              <span v-else-if="row.account_status === 'timed'" class="validity-text">
                {{ formatDate(row.valid_from) }} ~ {{ formatDate(row.valid_to) }}
              </span>
              <span v-else class="validity-permanent">长期有效</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right" align="center">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-tooltip content="编辑" placement="top">
                  <el-button type="primary" link @click="handleEdit(row)">
                    <el-icon><Edit /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip :content="row.role === 'admin' ? '管理员不可重置' : '重置密码'" placement="top">
                  <el-button type="warning" link @click="handleResetPassword(row)" :disabled="row.role === 'admin'">
                    <el-icon><Key /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip :content="row.isLocked ? '解锁账户' : '账户未锁定'" placement="top">
                  <el-button 
                    :type="row.isLocked ? 'success' : 'info'" 
                    link 
                    @click="handleUnlock(row)"
                    :disabled="!row.isLocked"
                  >
                    <el-icon><Unlock /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="删除" placement="top">
                  <el-button type="danger" link @click="handleDelete(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </el-tooltip>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrapper">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.size"
            :page-sizes="[10, 20, 50, 100]"
            :total="pagination.total"
            layout="total, sizes, prev, pager, next, jumper"
            background
            @size-change="loadData"
            @current-change="loadData"
          />
        </div>
      </el-card>
    </div>

    <!-- 用户编辑弹窗 -->
    <el-dialog 
      v-model="dialogVisible" 
      :title="dialogTitle" 
      width="800px" 
      @close="handleDialogClose" 
      destroy-on-close
      class="user-dialog"
    >
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="100px">
        <div class="form-grid">
          <div class="form-left">
            <el-form-item label="用户名" prop="username">
              <el-input v-model="formData.username" placeholder="请输入用户名" :disabled="!!formData.id">
                <template #prefix><el-icon><User /></el-icon></template>
              </el-input>
            </el-form-item>
            
            <el-form-item v-if="!formData.id" label="密码" prop="password">
              <el-input v-model="formData.password" type="password" placeholder="请输入密码" show-password @input="onPasswordInput">
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
              <div v-if="formData.password" class="password-strength">
                <div class="strength-bar">
                  <div class="strength-fill" :style="{ width: strengthPercentage + '%', backgroundColor: strengthInfo.color }"></div>
                </div>
                <span class="strength-text" :style="{ color: strengthInfo.color }">{{ strengthInfo.text }}</span>
                <span v-if="strengthFeedback" class="strength-hint">{{ strengthFeedback }}</span>
              </div>
            </el-form-item>
            
            <el-form-item label="姓名" prop="full_name">
              <el-input v-model="formData.full_name" placeholder="请输入姓名">
                <template #prefix><el-icon><UserFilled /></el-icon></template>
              </el-input>
            </el-form-item>
            
            <el-form-item label="邮箱">
              <el-input v-model="formData.email" placeholder="请输入邮箱">
                <template #prefix><el-icon><Message /></el-icon></template>
              </el-input>
            </el-form-item>
            
            <el-form-item label="部门">
              <el-input v-model="formData.department" placeholder="请输入部门">
                <template #prefix><el-icon><OfficeBuilding /></el-icon></template>
              </el-input>
            </el-form-item>
            
            <el-form-item label="电话">
              <el-input v-model="formData.phone" placeholder="请输入电话">
                <template #prefix><el-icon><Phone /></el-icon></template>
              </el-input>
            </el-form-item>
          </div>
          
          <div class="form-right">
            <el-form-item label="角色" prop="role">
              <el-select v-model="formData.role" placeholder="请选择角色" style="width: 100%">
                <el-option v-for="role in RoleOptions" :key="role.value" :label="role.label" :value="role.value" />
              </el-select>
            </el-form-item>
            
            <el-form-item label="账号状态">
              <el-select v-model="formData.account_status" placeholder="请选择账号状态" style="width: 100%">
                <el-option v-for="status in AccountStatusOptions" :key="status.value" :label="status.label" :value="status.value" />
              </el-select>
            </el-form-item>
            
            <el-form-item v-if="formData.account_status === 'temporary'" label="有效时长">
              <div class="hours-picker">
                <el-input-number v-model="formData.valid_hours" :min="1" :max="8760" :step="1" controls-position="right" />
                <span class="hours-suffix">小时</span>
              </div>
              <div class="hours-tip">
                <el-icon><InfoFilled /></el-icon>
                最大可设置1年（8760小时）
              </div>
            </el-form-item>
            
            <el-form-item v-if="formData.account_status === 'timed'" label="有效期限">
              <div class="date-range">
                <el-date-picker v-model="formData.valid_from" type="date" placeholder="开始日期" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width: 100%" />
                <span class="date-sep">至</span>
                <el-date-picker v-model="formData.valid_to" type="date" placeholder="结束日期" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width: 100%" />
              </div>
            </el-form-item>
          </div>
        </div>
        
        <!-- 权限配置 -->
        <div class="permission-section">
          <div class="section-header">
            <el-icon><Lock /></el-icon>
            <span>自定义权限配置</span>
            <el-tag size="small" type="info">追加模式</el-tag>
          </div>
          <div class="permission-grid">
            <div v-for="(perms, category) in groupedPermissions" :key="category" class="perm-category">
              <div class="category-header">
                <span class="category-name">{{ category }}</span>
                <el-badge :value="getSelectedCount(perms)" :hidden="getSelectedCount(perms) === 0" type="primary" />
              </div>
              <div class="perm-list">
                <el-checkbox-group v-model="formData.selectedPermissions">
                  <el-checkbox 
                    v-for="perm in perms" 
                    :key="perm.key" 
                    :label="perm.key"
                    :disabled="isPermissionInherited(perm.key)"
                    class="perm-checkbox"
                  >
                    {{ perm.name }}
                    <el-tag v-if="isPermissionInherited(perm.key)" size="small" type="success" class="inherited-tag">继承</el-tag>
                  </el-checkbox>
                </el-checkbox-group>
              </div>
            </div>
          </div>
          <div class="effective-perms">
            <span class="effective-label">生效权限：</span>
            <el-tag v-for="perm in effectivePermissions" :key="perm.key" size="small" :type="perm.isInherited ? 'success' : 'warning'" class="effective-tag">
              {{ perm.name }}
            </el-tag>
            <span v-if="effectivePermissions.length === 0" class="effective-empty">无</span>
          </div>
        </div>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit">确定</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 密码重置弹窗 -->
    <el-dialog v-model="resetDialogVisible" title="密码重置" width="450px" :close-on-click-modal="false" class="reset-dialog">
      <div class="reset-content">
        <el-alert type="warning" :closable="false" show-icon>
          <template #title>
            <span>即将重置用户 <strong>{{ resetTargetUser.username }}</strong> 的密码</span>
          </template>
        </el-alert>
        
        <div class="new-password-box">
          <div class="password-label">重置后的密码为：</div>
          <div class="password-value">{{ resetNewPassword || '1qazXSW@' }}</div>
        </div>
        
        <el-alert type="info" :closable="false" class="password-notice">
          <template #default>
            <span>请将此密码告知用户，并提醒其登录后尽快修改密码。</span>
          </template>
        </el-alert>
      </div>
      <template #footer>
        <el-button @click="resetDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmResetPassword">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Plus, Edit, Delete, Lock, Key, SuccessFilled, Setting, Document, Bell, Grid, 
  OfficeBuilding, Checked, InfoFilled, User, UserFilled, Message, Phone, 
  Search, Refresh, Avatar, CircleCheck, Clock, Timer, Unlock
} from '@element-plus/icons-vue'
import { getUserList, updateUser, deleteUser, register, resetUserPassword, unlockUser, getUserLockStatus } from '@/api/auth'
import { RoleOptions, AccountStatusOptions, getRoleText, getRoleType, getStatusText, getStatusType, calculatePasswordStrength, getPasswordStrengthInfo, PasswordStrengthLevel } from '@/utils/constants'

const loading = ref(false)
const dialogVisible = ref(false)
const dialogTitle = ref('')
const formRef = ref(null)
const tableData = ref([])

const searchForm = reactive({
  username: '',
  role: ''
})

const pagination = reactive({
  page: 1,
  size: 10,
  total: 0
})

// 统计计算
const adminCount = computed(() => tableData.value.filter(u => u.role === 'admin').length)
const activeCount = computed(() => tableData.value.filter(u => u.account_status === 'permanent' || !u.account_status).length)
const lockedCount = computed(() => tableData.value.filter(u => u.account_status === 'temporary' || u.account_status === 'disabled').length)

// 密码强度
const passwordStrength = ref({ level: PasswordStrengthLevel.WEAK, score: 0, feedback: [] })
const strengthPercentage = computed(() => Math.min(100, (passwordStrength.value.score / 8) * 100))
const strengthInfo = computed(() => getPasswordStrengthInfo(passwordStrength.value.level))
const strengthFeedback = computed(() => {
  if (passwordStrength.value.feedback.length > 0) {
    return '建议：' + passwordStrength.value.feedback.join('、')
  }
  return ''
})
const onPasswordInput = () => {
  passwordStrength.value = calculatePasswordStrength(formData.password)
}

// 权限定义
const allPermissions = [
  { key: 'dashboard', name: '仪表盘', category: '系统' },
  { key: 'user.manage', name: '用户管理', category: '系统' },
  { key: 'audit.view', name: '查看审计', category: '系统' },
  { key: 'contract.read', name: '查看合同', category: '合同' },
  { key: 'contract.create', name: '创建合同', category: '合同' },
  { key: 'contract.edit', name: '编辑合同', category: '合同' },
  { key: 'contract.delete', name: '删除合同', category: '合同' },
  { key: 'customer.read', name: '查看客户', category: '客户' },
  { key: 'customer.create', name: '创建客户', category: '客户' },
  { key: 'customer.edit', name: '编辑客户', category: '客户' },
  { key: 'customer.delete', name: '删除客户', category: '客户' },
  { key: 'approval.process', name: '审批处理', category: '审批' },
  { key: 'approval.view', name: '查看审批', category: '审批' }
]

const roleDefaultPermissions = {
  admin: ['all'],
  sales_director: ['dashboard', 'contract.read', 'contract.create', 'customer.read', 'customer.create', 'approval.process', 'approval.view'],
  tech_director: ['dashboard', 'contract.read', 'contract.create', 'customer.read', 'customer.create', 'approval.process', 'approval.view'],
  finance_director: ['dashboard', 'contract.read', 'contract.create', 'customer.read', 'customer.create', 'approval.process', 'approval.view'],
  contract_admin: ['dashboard', 'contract.read', 'contract.create', 'contract.edit', 'customer.read', 'customer.create', 'customer.edit', 'approval.process', 'approval.view'],
  sales: ['dashboard', 'contract.read', 'contract.create', 'customer.read', 'customer.create', 'approval.view'],
  audit_admin: ['dashboard', 'audit.view', 'contract.read', 'customer.read', 'approval.view']
}

const groupedPermissions = {
  '系统': [
    { key: 'dashboard', name: '仪表盘' },
    { key: 'user.manage', name: '用户管理' },
    { key: 'audit.view', name: '查看审计' }
  ],
  '合同': [
    { key: 'contract.read', name: '查看合同' },
    { key: 'contract.create', name: '创建合同' },
    { key: 'contract.edit', name: '编辑合同' },
    { key: 'contract.delete', name: '删除合同' }
  ],
  '客户': [
    { key: 'customer.read', name: '查看客户' },
    { key: 'customer.create', name: '创建客户' },
    { key: 'customer.edit', name: '编辑客户' },
    { key: 'customer.delete', name: '删除客户' }
  ],
  '审批': [
    { key: 'approval.process', name: '审批处理' },
    { key: 'approval.view', name: '查看审批' }
  ]
}

const isPermissionInherited = (permissionKey) => {
  const rolePerms = roleDefaultPermissions[formData.role] || []
  return rolePerms.includes(permissionKey) || rolePerms.includes('all')
}

const getSelectedCount = (perms) => {
  return perms.filter(p => formData.selectedPermissions.includes(p.key)).length
}

const effectivePermissions = computed(() => {
  const result = []
  const rolePerms = roleDefaultPermissions[formData.role] || []
  
  if (rolePerms.includes('all')) {
    return [{ key: 'all', name: '全部权限', isInherited: true }]
  }
  
  for (const key of rolePerms) {
    const perm = allPermissions.find(p => p.key === key)
    if (perm) result.push({ ...perm, isInherited: true })
  }
  
  const customPerms = formData.selectedPermissions.filter(p => !isPermissionInherited(p))
  for (const key of customPerms) {
    const perm = allPermissions.find(p => p.key === key)
    if (perm) result.push({ ...perm, isInherited: false })
  }
  
  return result
})

const formData = reactive({
  username: '',
  password: '',
  full_name: '',
  email: '',
  role: 'user',
  department: '',
  phone: '',
  is_active: true,
  account_status: 'permanent',
  valid_from: '',
  valid_to: '',
  valid_hours: 24,
  selectedPermissions: []
})

const formRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  full_name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  role: [{ required: true, message: '请选择角色', trigger: 'change' }]
}

const formatDate = (dateStr) => {
  if (!dateStr) return '-'
  if (typeof dateStr === 'string' && dateStr.includes('T')) {
    return dateStr.split('T')[0]
  }
  return dateStr
}

const tableRowClassName = ({ row }) => {
  if (row.role === 'admin') return 'admin-row'
  if (row.account_status === 'disabled') return 'disabled-row'
  return ''
}

const loadData = async () => {
  loading.value = true
  try {
    const params = {
      skip: (pagination.page - 1) * pagination.size,
      limit: pagination.size
    }
    if (searchForm.username) {
      params.username = searchForm.username
    }
    if (searchForm.role) {
      params.role = searchForm.role
    }
    const res = await getUserList(params)
    const data = res.data || res
    for (const user of data) {
      try {
        const lockStatus = await getUserLockStatus(user.id)
        user.isLocked = lockStatus.is_locked
        user.failCount = lockStatus.fail_count
      } catch (e) {
        user.isLocked = false
        user.failCount = 0
      }
    }
    tableData.value = data
    pagination.total = res.total || data.length
  } finally {
    loading.value = false
  }
}

const handleUnlock = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要解锁用户 "${row.username}" 吗？解锁后将清除登录失败记录。`, 
      '解锁确认', 
      { confirmButtonText: '确定解锁', cancelButtonText: '取消', type: 'warning' }
    )
    await unlockUser(row.id)
    row.isLocked = false
    row.failCount = 0
    ElMessage.success('解锁成功')
  } catch (error) {
    if (error !== 'cancel') {
      console.error('解锁失败:', error)
      ElMessage.error(error.response?.data?.error || '解锁失败')
    }
  }
}

const handleAdd = () => {
  dialogTitle.value = '新增用户'
  dialogVisible.value = true
}

const handleEdit = (row) => {
  dialogTitle.value = '编辑用户'
  let selectedPermissions = []
  if (row.custom_permissions) {
    try { selectedPermissions = JSON.parse(row.custom_permissions) } catch (e) { selectedPermissions = [] }
  }
  let validFrom = '', validTo = ''
  if (row.valid_from) validFrom = row.valid_from.split('T')[0]
  if (row.valid_to) validTo = row.valid_to.split('T')[0]
  Object.assign(formData, {
    ...row,
    password: '',
    account_status: row.account_status || 'permanent',
    valid_from: validFrom,
    valid_to: validTo,
    valid_hours: row.valid_hours || 24,
    selectedPermissions
  })
  dialogVisible.value = true
}

const handleDelete = async (row) => {
  await ElMessageBox.confirm('确定要删除该用户吗？此操作不可恢复。', '删除确认', {
    confirmButtonText: '确定删除',
    cancelButtonText: '取消',
    type: 'warning'
  })
  await deleteUser(row.id)
  ElMessage.success('删除成功')
  loadData()
}

const handleSearch = () => {
  pagination.page = 1
  loadData()
}

const handleReset = () => {
  Object.assign(searchForm, { username: '', role: '' })
  handleSearch()
}

// 密码重置
const resetDialogVisible = ref(false)
const resetTargetUser = ref({ username: '' })
const resetNewPassword = ref('')

const handleResetPassword = (row) => {
  resetTargetUser.value = { username: row.username, id: row.id }
  resetNewPassword.value = ''
  resetDialogVisible.value = true
}

const confirmResetPassword = async () => {
  try {
    await resetUserPassword(resetTargetUser.value.id)
    resetNewPassword.value = '1qazXSW@'
    ElMessage.success('密码重置成功，请将新密码告知用户')
  } catch (error) {
    console.error('重置密码失败:', error)
    ElMessage.error(error.response?.data?.error || '密码重置失败')
  }
}

const handleSubmit = async () => {
  await formRef.value.validate(async (valid) => {
    if (valid) {
      const customPermissions = formData.selectedPermissions.filter(p => !isPermissionInherited(p))
      const submitData = {
        ...formData,
        custom_permissions: customPermissions.length > 0 ? JSON.stringify(customPermissions) : '[]'
      }
      delete submitData.selectedPermissions
      
      if (formData.id) {
        await updateUser(formData.id, submitData)
        ElMessage.success('更新成功')
      } else {
        await register(submitData)
        ElMessage.success('注册成功')
      }
      dialogVisible.value = false
      loadData()
    }
  })
}

const handleDialogClose = () => {
  formRef.value?.resetFields()
  Object.assign(formData, {
    username: '', password: '', full_name: '', email: '', role: 'user',
    department: '', phone: '', is_active: true, account_status: 'permanent',
    valid_from: '', valid_to: '', valid_hours: 24, selectedPermissions: []
  })
  passwordStrength.value = { level: PasswordStrengthLevel.WEAK, score: 0, feedback: [] }
}

onMounted(() => { loadData() })
</script>

<style scoped>
.user-page {
  padding: 24px;
  background: #f5f7fa;
  min-height: 100vh;
}

/* 页面标题区 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 4px 0;
}

.page-desc {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}

/* 统计卡片 */
.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  background: white;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.08);
  transition: transform 0.2s, box-shadow 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
}

.stat-icon.users { background: linear-gradient(135deg, #6366f1, #8b5cf6); color: white; }
.stat-icon.admins { background: linear-gradient(135deg, #ef4444, #f97316); color: white; }
.stat-icon.active { background: linear-gradient(135deg, #22c55e, #10b981); color: white; }
.stat-icon.locked { background: linear-gradient(135deg, #f59e0b, #d97706); color: white; }

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: #64748b;
  margin-top: 4px;
}

/* 搜索区 */
.search-section { margin-bottom: 16px; }
.search-card { border-radius: 12px; }
.search-form :deep(.el-form-item) { margin-bottom: 0; }
.search-input { width: 200px; }
.role-select { width: 160px; }

/* 表格区 */
.table-section { margin-bottom: 20px; }
.table-card { border-radius: 12px; overflow: visible; }

.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-avatar {
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  color: white;
  font-weight: 600;
}

.username-text {
  font-weight: 500;
  color: #1e293b;
}

.fullname, .email-text, .dept-text {
  color: #475569;
}

.validity-text { color: #64748b; font-size: 13px; }
.validity-permanent { color: #22c55e; font-size: 13px; }

.action-buttons {
  display: flex;
  justify-content: center;
  gap: 4px;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 12px 0 0;
}

/* 表格行样式 */
:deep(.el-table .admin-row) {
  background: #fef2f2;
}

:deep(.el-table .disabled-row) {
  background: #f9fafb;
  color: #9ca3af;
}

/* 弹窗表单 */
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}

.form-left, .form-right {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* 密码强度 */
.password-strength {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}

.strength-bar {
  flex: 1;
  height: 4px;
  background: #e2e8f0;
  border-radius: 2px;
  overflow: hidden;
}

.strength-fill {
  height: 100%;
  transition: all 0.3s ease;
  border-radius: 2px;
}

.strength-text {
  font-size: 12px;
  font-weight: 600;
  min-width: 30px;
}

.strength-hint {
  font-size: 11px;
  color: #64748b;
}

/* 权限配置 */
.permission-section {
  margin-top: 20px;
  padding: 16px;
  background: #f8fafc;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  font-weight: 600;
  color: #475569;
}

.permission-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.perm-category {
  background: white;
  border-radius: 8px;
  padding: 12px;
  border: 1px solid #e2e8f0;
}

.category-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  font-weight: 600;
  font-size: 13px;
  color: #1e293b;
}

.perm-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.perm-checkbox {
  margin: 0;
  font-size: 13px;
}

.inherited-tag {
  margin-left: 4px;
}

.effective-perms {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid #e2e8f0;
}

.effective-label {
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
}

.effective-tag { margin: 0; }
.effective-empty { color: #9ca3af; font-size: 13px; }

/* 临时账号时长 */
.hours-picker {
  display: flex;
  align-items: center;
  gap: 10px;
}

.hours-suffix { color: #64748b; }
.hours-tip {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #92400e;
  font-size: 12px;
  margin-top: 4px;
}

/* 日期范围 */
.date-range {
  display: flex;
  align-items: center;
  gap: 10px;
}

.date-sep { color: #64748b; }

/* 弹窗底部 */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* 密码重置 */
.reset-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.new-password-box {
  padding: 24px;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  border-radius: 12px;
  border: 1px solid #93c5fd;
  text-align: center;
}

.password-label {
  font-size: 13px;
  color: #64748b;
  margin-bottom: 12px;
}

.password-value {
  font-size: 32px;
  font-weight: 700;
  color: #1d4ed8;
  font-family: 'Courier New', monospace;
  letter-spacing: 4px;
}

.password-notice { margin-top: 8px; }

/* 响应式 */
@media (max-width: 1400px) {
  .stats-cards { grid-template-columns: repeat(2, 1fr); }
  .permission-grid { grid-template-columns: repeat(2, 1fr); }
  .table-card { overflow-x: auto; }
}

@media (max-width: 1200px) {
  .stats-cards { grid-template-columns: repeat(2, 1fr); }
  .permission-grid { grid-template-columns: repeat(2, 1fr); }
}

@media (max-width: 992px) {
  .user-page { padding: 16px; }
  .page-header { flex-direction: column; align-items: flex-start; gap: 12px; }
  .stats-cards { grid-template-columns: repeat(2, 1fr); gap: 12px; }
  .form-grid { grid-template-columns: 1fr; }
  .permission-grid { grid-template-columns: 1fr; }
  .search-form :deep(.el-form-item) { display: block; margin-bottom: 12px; }
  .search-input { width: 100%; }
  .role-select { width: 100%; }
}

@media (max-width: 768px) {
  .stats-cards { grid-template-columns: 1fr; }
  .form-grid { grid-template-columns: 1fr; }
  .permission-grid { grid-template-columns: 1fr; }
  .search-form :deep(.el-form-item) { display: block; }
  .table-section :deep(.el-table) { font-size: 12px; }
  .table-section :deep(.el-table__header) { font-size: 12px; }
  .dialog-footer { flex-direction: column; }
  .dialog-footer .el-button { width: 100%; }
}

/* 表格自适应 */
.table-section :deep(.el-table) {
  width: 100%;
  table-layout: fixed;
}

.table-section :deep(.el-table__body) {
  width: 100%;
}

.table-section :deep(.el-table .cell) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.table-section :deep(.el-table__header) {
  width: 100%;
}

/* 弹窗自适应 */
.user-dialog :deep(.el-dialog) {
  max-width: 95vw;
  margin: 0 2.5vw;
}

.user-dialog :deep(.el-dialog__body) {
  max-height: 70vh;
  overflow-y: auto;
}

/* 密码重置弹窗 */
.reset-dialog :deep(.el-dialog) {
  max-width: 95vw;
  margin: 0 2.5vw;
}
</style>
