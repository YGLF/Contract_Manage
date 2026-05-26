<template>
  <div class="performance-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button text @click="$router.back()">
              <el-icon><ArrowLeft /></el-icon>
              返回
            </el-button>
            <div>
              <div class="title">履约计划与执行</div>
              <div class="subtitle">{{ contract.title || contract.contract_no || contractId }}</div>
            </div>
          </div>
          <el-button type="primary" @click="addNode">新增节点</el-button>
        </div>
      </template>

      <div class="summary-bar">
        <el-tag type="info">当前状态：{{ getStatusText(contract.status) }}</el-tag>
        <el-tag type="success">最新版本：V{{ latestVersion.version || 0 }}</el-tag>
        <el-tag type="warning">计划节点：{{ planNodes.length }}</el-tag>
        <el-tag type="primary">执行记录：{{ executionRecords.length }}</el-tag>
      </div>

      <el-divider content-position="left">计划版本</el-divider>
      <el-table :data="planNodes" style="width: 100%" :cell-style="{ padding: '8px 0' }">
        <el-table-column label="节点名称" min-width="220">
          <template #default="{ row }">
            <el-input v-model="row.node_name" placeholder="请输入节点名称" />
          </template>
        </el-table-column>
        <el-table-column label="节点类型" width="180">
          <template #default="{ row }">
            <el-select v-model="row.node_type" placeholder="请选择" style="width: 100%">
              <el-option v-for="item in nodeTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="计划日期" width="220">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.due_date"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ss[Z]"
              placeholder="请选择时间"
              style="width: 100%"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ $index }">
            <el-button type="danger" link @click="removeNode($index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="planNodes.length === 0" description="当前还没有履约计划节点" />

      <div class="footer-actions">
        <el-button @click="$router.back()">取消</el-button>
        <el-button type="primary" :loading="savingPlan" @click="savePlanVersion">保存为新版本</el-button>
      </div>

      <el-divider content-position="left">执行记录</el-divider>
      <el-form :inline="false" :model="executionForm" label-width="100px" class="execution-form">
        <el-form-item label="关联节点">
          <el-select v-model="executionForm.plan_id" placeholder="请选择计划节点" style="width: 100%">
            <el-option
              v-for="item in latestVersion.plans || []"
              :key="item.id"
              :label="`${item.node_name} (${item.node_type})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="执行结果">
          <el-select v-model="executionForm.result" placeholder="请选择执行结果" style="width: 100%">
            <el-option label="已完成" value="completed" />
            <el-option label="进行中" value="in_progress" />
            <el-option label="延期" value="delayed" />
            <el-option label="异常" value="exception" />
          </el-select>
        </el-form-item>
        <el-form-item label="实际时间">
          <el-date-picker
            v-model="executionForm.actual_at"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ss[Z]"
            placeholder="请选择实际执行时间"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="说明备注">
          <el-input v-model="executionForm.remark" type="textarea" :rows="3" placeholder="请输入执行说明" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="savingExecution" @click="saveExecutionRecord">登记执行记录</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="executionRecords" style="width: 100%" :cell-style="{ padding: '8px 0' }">
        <el-table-column prop="plan_name" label="关联节点" min-width="220" />
        <el-table-column prop="result" label="执行结果" width="120">
          <template #default="{ row }">
            <el-tag :type="getExecutionResultType(row.result)">{{ getExecutionResultText(row.result) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="actual_at" label="实际时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.actual_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="operator_id" label="登记人" width="120" />
        <el-table-column prop="remark" label="说明" min-width="220" />
      </el-table>

      <el-empty v-if="executionRecords.length === 0" description="当前还没有执行记录" />
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useUserStore } from '@/store/user'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Delete } from '@element-plus/icons-vue'
import { getContractDetail } from '@/api/contract'
import {
  createExecutionRecord,
  createPlanVersion,
  getExecutionRecords,
  getLatestPlanVersion
} from '@/api/performance'

const route = useRoute()
const userStore = useUserStore()

const contractId = String(route.params.id || '')
const contract = ref({})
const latestVersion = ref({ version: 0, plans: [] })
const planNodes = ref([])
const executionRecords = ref([])
const savingPlan = ref(false)
const savingExecution = ref(false)

const executionForm = reactive({
  plan_id: '',
  result: 'completed',
  actual_at: '',
  remark: ''
})

const nodeTypeOptions = [
  { value: 'payment', label: '付款节点' },
  { value: 'delivery', label: '交付节点' },
  { value: 'acceptance', label: '验收节点' },
  { value: 'milestone', label: '里程碑' }
]

const getStatusText = (status) => {
  const map = {
    registered: '已登记',
    active: '生效中',
    in_progress: '履约中',
    pending_pay: '待付款',
    completed: '已完成',
    terminated: '已终止',
    archived: '已归档'
  }
  return map[status] || status || '-'
}

const getExecutionResultText = (result) => {
  const map = {
    completed: '已完成',
    in_progress: '进行中',
    delayed: '延期',
    exception: '异常'
  }
  return map[result] || result || '-'
}

const getExecutionResultType = (result) => {
  const map = {
    completed: 'success',
    in_progress: 'primary',
    delayed: 'warning',
    exception: 'danger'
  }
  return map[result] || 'info'
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

const addNode = () => {
  planNodes.value.push({
    node_name: '',
    node_type: 'milestone',
    due_date: ''
  })
}

const removeNode = (index) => {
  planNodes.value.splice(index, 1)
}

const loadContract = async () => {
  contract.value = await getContractDetail(contractId)
}

const loadLatestVersion = async () => {
  const data = await getLatestPlanVersion(contractId)
  latestVersion.value = data || { version: 0, plans: [] }
  planNodes.value = Array.isArray(data?.plans) && data.plans.length > 0
    ? data.plans.map((item) => ({
        node_name: item.node_name,
        node_type: item.node_type,
        due_date: item.due_date
      }))
    : []
}

const loadExecutions = async () => {
  const rows = await getExecutionRecords(contractId)
  const planMap = new Map((latestVersion.value.plans || []).map((item) => [item.id, item.node_name]))
  executionRecords.value = (Array.isArray(rows) ? rows : []).map((item) => ({
    ...item,
    plan_name: planMap.get(item.plan_id) || item.plan_id
  }))
}

const savePlanVersion = async () => {
  if (planNodes.value.length === 0) {
    ElMessage.error('请至少添加一个履约节点')
    return
  }

  for (const node of planNodes.value) {
    if (!node.node_name?.trim()) {
      ElMessage.error('请完善节点名称')
      return
    }
    if (!node.node_type) {
      ElMessage.error('请选择节点类型')
      return
    }
    if (!node.due_date) {
      ElMessage.error('请选择计划日期')
      return
    }
  }

  savingPlan.value = true
  try {
    await createPlanVersion(
      contractId,
      planNodes.value.map((item) => ({
        node_name: item.node_name.trim(),
        node_type: item.node_type,
        due_date: item.due_date
      }))
    )
    ElMessage.success('履约计划版本已保存')
    await loadLatestVersion()
    await loadExecutions()
  } finally {
    savingPlan.value = false
  }
}

const saveExecutionRecord = async () => {
  if (!executionForm.plan_id) {
    ElMessage.error('请选择关联节点')
    return
  }
  if (!executionForm.actual_at) {
    ElMessage.error('请选择实际时间')
    return
  }

  savingExecution.value = true
  try {
    await createExecutionRecord(contractId, {
      plan_id: executionForm.plan_id,
      result: executionForm.result,
      actual_at: executionForm.actual_at,
      remark: executionForm.remark,
      operator_id: userStore.userInfo?.username || userStore.userInfo?.id || 'system'
    })
    ElMessage.success('执行记录已登记')
    executionForm.plan_id = ''
    executionForm.result = 'completed'
    executionForm.actual_at = ''
    executionForm.remark = ''
    await loadExecutions()
  } finally {
    savingExecution.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadContract(), loadLatestVersion()])
  await loadExecutions()
})
</script>

<style scoped>
.performance-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title {
  font-size: 18px;
  font-weight: 600;
}

.subtitle {
  font-size: 12px;
  color: #909399;
}

.summary-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
  margin-bottom: 20px;
}

.execution-form {
  margin-bottom: 16px;
}
</style>
