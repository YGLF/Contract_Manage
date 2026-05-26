<template>
  <div class="archive-page">
    <div class="toolbar">
      <el-select v-model="filters.status" placeholder="归档状态" clearable style="width: 160px">
        <el-option label="已归档" value="archived" />
      </el-select>
      <el-select v-model="filters.borrow_status" placeholder="借阅状态" clearable style="width: 160px">
        <el-option label="空闲" value="idle" />
        <el-option label="已借阅" value="borrowed" />
      </el-select>
      <el-select v-model="filters.destroy_state" placeholder="销毁状态" clearable style="width: 160px">
        <el-option label="保留" value="retained" />
        <el-option label="待销毁" value="requested" />
      </el-select>
      <el-button type="primary" @click="applyFilters">查询</el-button>
      <el-button @click="resetFilters">重置</el-button>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>归档管理</span>
        </div>
      </template>

      <el-table :data="filteredRows" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="归档ID" min-width="130" />
        <el-table-column prop="contract_id" label="合同ID" min-width="130" />
        <el-table-column prop="archive_type" label="归档类型" min-width="120" />
        <el-table-column prop="status" label="归档状态" min-width="110">
          <template #default="{ row }">
            <el-tag type="success">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="borrow_status" label="借阅状态" min-width="110">
          <template #default="{ row }">
            <el-tag :type="row.borrow_status === 'borrowed' ? 'warning' : 'info'">
              {{ getBorrowStatusText(row.borrow_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="destroy_state" label="销毁状态" min-width="110">
          <template #default="{ row }">
            <el-tag :type="row.destroy_state === 'requested' ? 'danger' : 'info'">
              {{ getDestroyStateText(row.destroy_state) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近审批" min-width="220">
          <template #default="{ row }">
            <div v-if="row.latest_approval" class="approval-summary">
              <el-tag :type="getApprovalStatusType(row.latest_approval.status)" size="small">
                {{ getApprovalStatusText(row.latest_approval.status) }}
              </el-tag>
              <span>{{ getApprovalTypeText(row.latest_approval.request_type) }}</span>
              <span class="muted">{{ formatDateTime(row.latest_approval.created_at) }}</span>
            </div>
            <span v-else class="muted">暂无审批记录</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" min-width="160">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <div class="actions">
              <el-button type="primary" link @click="openApprovalDialog(row, 'archive_borrow')">借阅审批</el-button>
              <el-button type="danger" link @click="openApprovalDialog(row, 'archive_destroy')">销毁审批</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="approvalDialogVisible" title="发起档案审批" width="520px">
      <el-form :model="approvalForm" label-width="100px">
        <el-form-item label="归档ID">
          <el-input :model-value="currentCase?.id || '-'" disabled />
        </el-form-item>
        <el-form-item label="审批类型">
          <el-input :model-value="approvalTypeText" disabled />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="approvalForm.comment" type="textarea" :rows="4" placeholder="请输入审批说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="approvalDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitApproval">提交审批</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getArchiveCases } from '@/api/archive'
import { createApproval, getApprovalRequests } from '@/api/approval'
import { useUserStore } from '@/store/user'

const userStore = useUserStore()

const loading = ref(false)
const rows = ref([])
const filteredRows = ref([])
const approvalDialogVisible = ref(false)
const currentCase = ref(null)
const currentApprovalType = ref('archive_borrow')
const approvals = ref([])

const filters = reactive({
  status: '',
  borrow_status: '',
  destroy_state: ''
})

const approvalForm = reactive({
  comment: ''
})

const approvalTypeText = computed(() => {
  return currentApprovalType.value === 'archive_destroy' ? '档案销毁' : '档案借阅'
})

const getOperator = () => userStore.userInfo?.username || userStore.userInfo?.id || 'system'

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

const getStatusText = (status) => status === 'archived' ? '已归档' : (status || '-')
const getBorrowStatusText = (status) => status === 'borrowed' ? '已借阅' : '空闲'
const getDestroyStateText = (status) => status === 'requested' ? '待销毁' : '保留'

const getApprovalTypeText = (type) => {
  const map = {
    archive_borrow: '档案借阅',
    archive_destroy: '档案销毁'
  }
  return map[type] || type || '-'
}

const getApprovalStatusText = (status) => {
  const map = {
    pending: '待审批',
    approved: '已通过',
    rejected: '已驳回'
  }
  return map[status] || status || '-'
}

const getApprovalStatusType = (status) => {
  const map = {
    pending: 'warning',
    approved: 'success',
    rejected: 'danger'
  }
  return map[status] || 'info'
}

const mergeRows = () => {
  const merged = rows.value.map(item => {
    const relatedApprovals = approvals.value
      .filter(req => req.resource_id === item.id && ['archive_borrow', 'archive_destroy'].includes(req.request_type))
      .sort((a, b) => new Date(b.created_at) - new Date(a.created_at))

    return {
      ...item,
      latest_approval: relatedApprovals[0] || null
    }
  })

  filteredRows.value = merged.filter(item => {
    if (filters.status && item.status !== filters.status) return false
    if (filters.borrow_status && item.borrow_status !== filters.borrow_status) return false
    if (filters.destroy_state && item.destroy_state !== filters.destroy_state) return false
    return true
  })
}

const applyFilters = () => {
  mergeRows()
}

const resetFilters = () => {
  filters.status = ''
  filters.borrow_status = ''
  filters.destroy_state = ''
  mergeRows()
}

const loadData = async () => {
  loading.value = true
  try {
    const [archiveCases, approvalRows] = await Promise.all([
      getArchiveCases(),
      getApprovalRequests()
    ])
    rows.value = Array.isArray(archiveCases) ? archiveCases : []
    approvals.value = Array.isArray(approvalRows) ? approvalRows : []
    mergeRows()
  } finally {
    loading.value = false
  }
}

const openApprovalDialog = (row, type) => {
  currentCase.value = row
  currentApprovalType.value = type
  approvalForm.comment = ''
  approvalDialogVisible.value = true
}

const submitApproval = async () => {
  if (!currentCase.value?.id) {
    ElMessage.error('未找到归档记录')
    return
  }
  await createApproval({
    contract_id: currentCase.value.contract_id,
    request_type: currentApprovalType.value,
    resource_id: currentCase.value.id,
    requested_by: getOperator(),
    payload: {
      comment: approvalForm.comment
    },
    comment: approvalForm.comment
  })
  ElMessage.success('档案审批已提交')
  approvalDialogVisible.value = false
  loadData()
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.archive-page {
  padding: 16px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.approval-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.muted {
  color: #909399;
  font-size: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
