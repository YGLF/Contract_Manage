<template>
  <div class="approval-page">
    <div class="approval-stats">
      <el-row :gutter="12">
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon pending"><el-icon><Clock /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ stats.pendingCount }}</div>
                <div class="stat-label">待审批</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon status"><el-icon><Switch /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ stats.statusChangeCount }}</div>
                <div class="stat-label">状态变更</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon plan"><el-icon><Calendar /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ stats.planAdjustmentCount }}</div>
                <div class="stat-label">计划调整</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon archive"><el-icon><FolderOpened /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ stats.archiveRequestCount }}</div>
                <div class="stat-label">借阅与销毁</div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <el-card class="approval-card">
      <template #header>
        <div class="header">
          <div class="header-left">
            <el-icon class="header-icon"><Check /></el-icon>
            <span class="header-title">审批中心</span>
            <el-tag type="warning" effect="dark">{{ pendingRows.length }} 项待处理</el-tag>
          </div>
        </div>
      </template>

      <el-table
        :data="tableData"
        style="width: 100%"
        v-loading="loading"
        class="approval-table"
        :default-sort="{ prop: 'created_at', order: 'descending' }"
      >
        <el-table-column prop="display_contract_no" label="合同编号" min-width="140" />
        <el-table-column prop="display_title" label="事项标题" min-width="220" />
        <el-table-column prop="request_type" label="审批类型" min-width="130">
          <template #default="{ row }">
            <el-tag :type="getRequestTypeTag(row.request_type)" effect="dark" round size="small">
              {{ getRequestTypeText(row.request_type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="requested_by" label="申请人" min-width="110">
          <template #default="{ row }">
            <span>{{ row.requested_by || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" min-width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" effect="dark" round size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="申请时间" min-width="170">
          <template #default="{ row }">
            <span class="time">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="说明" min-width="220">
          <template #default="{ row }">
            <span class="summary">{{ buildSummary(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-tooltip content="查看详情" placement="top">
                <el-button type="primary" link @click="handleView(row)">
                  <el-icon><View /></el-icon>
                </el-button>
              </el-tooltip>
              <template v-if="row.status === 'pending'">
                <el-tooltip content="审批通过" placement="top">
                  <el-button type="success" link @click="handleApprove(row)">
                    <el-icon><Check /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="审批驳回" placement="top">
                  <el-button type="danger" link @click="handleReject(row)">
                    <el-icon><Close /></el-icon>
                  </el-button>
                </el-tooltip>
              </template>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="640px">
      <el-form :model="formData" label-width="110px">
        <el-form-item label="合同编号">
          <el-input :model-value="currentRow.display_contract_no || '-'" disabled />
        </el-form-item>
        <el-form-item label="事项标题">
          <el-input :model-value="currentRow.display_title || '-'" disabled />
        </el-form-item>
        <el-form-item label="审批类型">
          <el-tag :type="getRequestTypeTag(currentRow.request_type)">
            {{ getRequestTypeText(currentRow.request_type) }}
          </el-tag>
        </el-form-item>
        <el-form-item label="审批结论">
          <el-radio-group v-model="formData.action">
            <el-radio label="approved">通过</el-radio>
            <el-radio label="rejected">驳回</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="审批意见">
          <el-input
            v-model="formData.comment"
            type="textarea"
            :rows="4"
            placeholder="请输入审批意见"
          />
        </el-form-item>
        <el-form-item label="业务摘要">
          <div class="dialog-summary">{{ buildSummary(currentRow) }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmitApproval">确定</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Calendar,
  Check,
  Clock,
  Close,
  FolderOpened,
  Switch,
  View
} from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import {
  approveApprovalRequest,
  getApprovalRequests,
  rejectApprovalRequest
} from '@/api/approval'
import { getContractDetail, getContracts } from '@/api/contract'

const router = useRouter()
const userStore = useUserStore()

const loading = ref(false)
const dialogVisible = ref(false)
const tableData = ref([])
const currentRow = ref({})
const contractMap = ref({})

const stats = reactive({
  pendingCount: 0,
  statusChangeCount: 0,
  planAdjustmentCount: 0,
  archiveRequestCount: 0,
  approvedCount: 0,
  rejectedCount: 0,
  activeCount: 0,
  completedCount: 0
})

const formData = reactive({
  action: 'approved',
  comment: ''
})

const dialogTitle = computed(() => {
  return formData.action === 'rejected' ? '驳回审批' : '处理审批'
})

const pendingRows = computed(() => tableData.value.filter(item => item.status === 'pending'))

const requestTypeTextMap = {
  status_change: '状态变更',
  plan_adjustment: '履约计划调整',
  archive_borrow: '档案借阅',
  archive_destroy: '档案销毁'
}

const statusTextMap = {
  pending: '待审批',
  approved: '已通过',
  rejected: '已驳回'
}

const getRequestTypeText = (type) => requestTypeTextMap[type] || type || '-'

const getRequestTypeTag = (type) => {
  const tagMap = {
    status_change: 'warning',
    plan_adjustment: 'primary',
    archive_borrow: 'success',
    archive_destroy: 'danger'
  }
  return tagMap[type] || 'info'
}

const getStatusText = (status) => statusTextMap[status] || status || '-'

const getStatusType = (status) => {
  const tagMap = {
    pending: 'warning',
    approved: 'success',
    rejected: 'danger'
  }
  return tagMap[status] || 'info'
}

const formatDateTime = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return dateStr
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}`
}

const buildSummary = (row) => {
  const payload = row.payload || {}
  if (row.request_type === 'status_change') {
    return `目标状态：${payload.status || '-'}`
  }
  if (row.request_type === 'plan_adjustment') {
    const count = Array.isArray(payload.nodes) ? payload.nodes.length : 0
    return `计划节点数：${count}`
  }
  if (row.request_type === 'archive_borrow') {
    return `档案借阅申请，档案ID：${row.resource_id || '-'}`
  }
  if (row.request_type === 'archive_destroy') {
    return `档案销毁申请，档案ID：${row.resource_id || '-'}`
  }
  return '待处理审批事项'
}

const normalizeApprovalRows = (rows) => {
  return rows.map(row => {
    const contract = contractMap.value[row.contract_id] || {}
    return {
      ...row,
      display_contract_no: contract.contract_no || row.contract_id || row.resource_id || '-',
      display_title: contract.title || getRequestTypeText(row.request_type),
      payload: row.payload || {}
    }
  })
}

const loadContracts = async () => {
  try {
    const contracts = await getContracts({})
    if (!Array.isArray(contracts)) {
      contractMap.value = {}
      return
    }
    const nextMap = {}
    contracts.forEach(item => {
      nextMap[item.id] = item
    })
    contractMap.value = nextMap
    stats.activeCount = contracts.filter(item => item.status === 'active').length
    stats.completedCount = contracts.filter(item => item.status === 'completed').length
    stats.approvedCount = contracts.filter(item => item.status === 'approved').length
    stats.rejectedCount = contracts.filter(item => item.status === 'terminated').length
  } catch (error) {
    console.error('Failed to load contracts:', error)
    contractMap.value = {}
  }
}

const calculateStats = (rows) => {
  const pending = rows.filter(item => item.status === 'pending')
  stats.pendingCount = pending.length
  stats.statusChangeCount = pending.filter(item => item.request_type === 'status_change').length
  stats.planAdjustmentCount = pending.filter(item => item.request_type === 'plan_adjustment').length
  stats.archiveRequestCount = pending.filter(item => ['archive_borrow', 'archive_destroy'].includes(item.request_type)).length
}

const loadData = async () => {
  loading.value = true
  try {
    await loadContracts()
    const rows = await getApprovalRequests()
    const normalizedRows = normalizeApprovalRows(Array.isArray(rows) ? rows : [])
    tableData.value = normalizedRows
    calculateStats(normalizedRows)
  } catch (error) {
    console.error('Failed to load approval requests:', error)
    tableData.value = []
    calculateStats([])
  } finally {
    loading.value = false
  }
}

const handleView = async (row) => {
  if (!row.contract_id) {
    ElMessage.info('该审批事项没有关联合同详情页')
    return
  }
  try {
    if (!contractMap.value[row.contract_id]) {
      const detail = await getContractDetail(row.contract_id)
      contractMap.value = {
        ...contractMap.value,
        [row.contract_id]: detail
      }
    }
    router.push(`/contracts/${row.contract_id}`)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '未找到关联合同')
  }
}

const openDialog = (row, action) => {
  currentRow.value = row
  formData.action = action
  formData.comment = ''
  dialogVisible.value = true
}

const handleApprove = (row) => {
  openDialog(row, 'approved')
}

const handleReject = (row) => {
  openDialog(row, 'rejected')
}

const buildApprovePayload = () => ({
  approved_by: userStore.userInfo?.username || userStore.userInfo?.id || 'system',
  comment: formData.comment
})

const handleSubmitApproval = async () => {
  if (!currentRow.value?.id) {
    ElMessage.error('未找到审批记录')
    return
  }

  if (formData.action === 'rejected' && !formData.comment.trim()) {
    ElMessage.error('请填写驳回原因')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确认${formData.action === 'approved' ? '通过' : '驳回'}该审批事项吗？`,
      '审批确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: formData.action === 'approved' ? 'success' : 'warning'
      }
    )

    if (formData.action === 'approved') {
      await approveApprovalRequest(currentRow.value.id, buildApprovePayload())
      ElMessage.success('审批已通过')
    } else {
      await rejectApprovalRequest(currentRow.value.id, buildApprovePayload())
      ElMessage.success('审批已驳回')
    }

    dialogVisible.value = false
    await loadData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '审批处理失败')
    }
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.approval-page {
  padding: 16px;
}

.approval-stats {
  margin-bottom: 16px;
}

.stat-card {
  border-radius: 8px;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.stat-icon {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.stat-icon.pending {
  background: #fdf6ec;
  color: #e6a23c;
}

.stat-icon.status {
  background: #fef0f0;
  color: #f56c6c;
}

.stat-icon.plan {
  background: #ecf5ff;
  color: #409eff;
}

.stat-icon.archive {
  background: #f0f9eb;
  color: #67c23a;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.approval-card {
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-icon {
  color: #409eff;
  font-size: 18px;
}

.header-title {
  color: #303133;
  font-size: 16px;
  font-weight: 600;
}

.approval-table {
  border-radius: 8px;
  overflow: hidden;
}

.time,
.summary {
  color: #606266;
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.dialog-summary {
  color: #606266;
  line-height: 1.6;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
