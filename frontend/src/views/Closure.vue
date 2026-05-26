<template>
  <div class="closure-page">
    <div class="toolbar">
      <el-button type="primary" @click="openCreateDialog">发起结案申请</el-button>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>结案管理</span>
        </div>
      </template>

      <el-table :data="rows" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="申请ID" min-width="130" />
        <el-table-column prop="contract_id" label="合同ID" min-width="130" />
        <el-table-column prop="request_type" label="类型" min-width="120">
          <template #default="{ row }">
            {{ getRequestTypeText(row.request_type) }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" min-width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'completed' ? 'success' : 'warning'">
              {{ row.status === 'completed' ? '已完成' : '待处理' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="requested_by" label="申请人" min-width="120" />
        <el-table-column label="校验项" min-width="260">
          <template #default="{ row }">
            <div class="checks">
              <el-tag :type="row.risk_checked ? 'success' : 'danger'" size="small">风险闭环</el-tag>
              <el-tag :type="row.performance_ok ? 'success' : 'danger'" size="small">履约完成</el-tag>
              <el-tag :type="row.evidence_ready ? 'success' : 'danger'" size="small">材料齐全</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="归档联动" min-width="220">
          <template #default="{ row }">
            <div v-if="row.archive_case" class="archive-summary">
              <el-tag type="success" size="small">已生成归档</el-tag>
              <span>{{ row.archive_case.id }}</span>
              <span class="muted">{{ row.archive_case.archive_type }}</span>
            </div>
            <span v-else class="muted">尚未联动归档</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" min-width="160">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status !== 'completed'" type="success" link @click="completeRequest(row)">
              完成结案
            </el-button>
            <span v-else class="done-text">已完成</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="发起结案申请" width="560px">
      <el-form :model="formData" label-width="110px">
        <el-form-item label="合同ID">
          <el-input v-model="formData.contract_id" placeholder="请输入合同ID" @blur="loadPerformanceAutoCheck" />
        </el-form-item>
        <el-form-item label="申请类型">
          <el-select v-model="formData.request_type" style="width: 100%">
            <el-option label="结案" value="close" />
            <el-option label="终止" value="terminate" />
            <el-option label="解除" value="cancel" />
          </el-select>
        </el-form-item>
        <el-form-item label="申请原因">
          <el-input v-model="formData.reason" type="textarea" :rows="3" placeholder="请输入申请原因" />
        </el-form-item>
        <el-form-item label="风险闭环">
          <el-switch v-model="formData.risk_checked" />
        </el-form-item>
        <el-form-item label="履约完成">
          <div class="auto-check-block">
            <el-switch :model-value="formData.performance_ok" disabled />
            <span class="muted">
              {{ performanceSummaryText }}
            </span>
          </div>
        </el-form-item>
        <el-form-item label="材料齐全">
          <el-switch v-model="formData.evidence_ready" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="submitRequest">提交</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { completeClosureRequest, createClosureRequest, getClosureRequests } from '@/api/closure'
import { getArchiveCases } from '@/api/archive'
import { getPerformanceSummary } from '@/api/performance'
import { useUserStore } from '@/store/user'

const userStore = useUserStore()

const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const rows = ref([])
const performanceSummary = ref(null)

const formData = reactive({
  contract_id: '',
  request_type: 'close',
  reason: '',
  risk_checked: false,
  performance_ok: false,
  evidence_ready: false
})

const performanceSummaryText = computed(() => {
  if (!formData.contract_id) return '输入合同ID后自动判断履约完成情况'
  if (!performanceSummary.value) return '当前未获取履约摘要'
  return `计划 ${performanceSummary.value.plan_count} 项，完成 ${performanceSummary.value.completed_count} 项，延期 ${performanceSummary.value.delayed_count} 项，异常 ${performanceSummary.value.exception_count} 项`
})

const getOperator = () => userStore.userInfo?.username || userStore.userInfo?.id || 'system'

const getRequestTypeText = (type) => {
  const map = {
    close: '结案',
    terminate: '终止',
    cancel: '解除'
  }
  return map[type] || type || '-'
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

const loadData = async () => {
  loading.value = true
  try {
    const [closureRows, archiveCases] = await Promise.all([
      getClosureRequests(),
      getArchiveCases()
    ])
    const closureList = Array.isArray(closureRows) ? closureRows : []
    const archiveList = Array.isArray(archiveCases) ? archiveCases : []

    rows.value = closureList.map(item => ({
      ...item,
      archive_case: archiveList.find(archive => archive.contract_id === item.contract_id) || null
    }))
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  formData.contract_id = ''
  formData.request_type = 'close'
  formData.reason = ''
  formData.risk_checked = false
  formData.performance_ok = false
  formData.evidence_ready = false
  performanceSummary.value = null
}

const loadPerformanceAutoCheck = async () => {
  if (!formData.contract_id.trim()) {
    formData.performance_ok = false
    performanceSummary.value = null
    return
  }
  try {
    const summary = await getPerformanceSummary(formData.contract_id.trim())
    performanceSummary.value = summary
    formData.performance_ok = Boolean(summary?.performance_ok)
  } catch {
    performanceSummary.value = null
    formData.performance_ok = false
  }
}

const openCreateDialog = () => {
  resetForm()
  dialogVisible.value = true
}

const submitRequest = async () => {
  if (!formData.contract_id.trim()) {
    ElMessage.error('请输入合同ID')
    return
  }
  await loadPerformanceAutoCheck()
  submitting.value = true
  try {
    await createClosureRequest({
      ...formData,
      contract_id: formData.contract_id.trim(),
      requested_by: getOperator()
    })
    ElMessage.success('结案申请已提交')
    dialogVisible.value = false
    await loadData()
  } finally {
    submitting.value = false
  }
}

const completeRequest = async (row) => {
  await completeClosureRequest(row.id)
  ElMessage.success('结案已完成，并已联动归档。')
  await loadData()
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.closure-page {
  padding: 16px;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.checks,
.archive-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.auto-check-block {
  display: flex;
  align-items: center;
  gap: 12px;
}

.muted {
  color: #909399;
  font-size: 12px;
}

.done-text {
  color: #67c23a;
  font-size: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
