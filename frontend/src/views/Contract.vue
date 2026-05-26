<template>
  <div class="contract-page">
    <div class="search-bar">
      <el-input v-model="searchForm.title" placeholder="合同标题" clearable style="width: 220px" />
      <el-select v-model="searchForm.status" placeholder="合同状态" clearable style="width: 160px">
        <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button type="primary" @click="handleSearch">查询</el-button>
      <el-button @click="handleReset">重置</el-button>
      <div class="search-right">
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          合同入库
        </el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="pagedRows" v-loading="loading" style="width: 100%" :cell-style="{ padding: '8px 0' }">
        <el-table-column prop="contract_no" label="合同编号" min-width="160" />
        <el-table-column prop="title" label="合同标题" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="title-cell">
              <span>{{ row.title }}</span>
              <el-tag v-if="row.pending_approval_count > 0" size="small" type="warning" class="reminder-tag">
                待审批 {{ row.pending_approval_count }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="相对方" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.counterparty_name || row.counterparty_id || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="审批进度" min-width="220">
          <template #default="{ row }">
            <div v-if="row.pending_approval_count > 0" class="workflow-cell">
              <el-tag type="warning" size="small">待审批</el-tag>
              <span class="workflow-text">{{ row.pending_approval_summary }}</span>
            </div>
            <span v-else class="text-gray">无待审批事项</span>
          </template>
        </el-table-column>
        <el-table-column label="附件数" width="100">
          <template #default="{ row }">
            {{ Array.isArray(row.document_ids) ? row.document_ids.length : 0 }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" min-width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
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
              <el-tooltip content="编辑" placement="top">
                <el-button type="warning" link @click="handleEdit(row)">
                  <el-icon><Edit /></el-icon>
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

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.size"
        :page-sizes="[10, 20, 50, 100]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        style="margin-top: 12px; justify-content: flex-end"
        @size-change="refreshPagedRows"
        @current-change="refreshPagedRows"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="760px" @close="handleDialogClose">
      <el-alert
        v-if="!isEditMode"
        title="入库流程要求至少上传一个已盖章生效合同附件，系统会先提交附件，再创建合同入库记录。"
        type="info"
        :closable="false"
        class="dialog-alert"
      />
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="110px">
        <el-form-item label="合同标题" prop="title">
          <el-input v-model="formData.title" placeholder="请输入合同标题" />
        </el-form-item>
        <el-form-item label="相对方" prop="counterparty_id">
          <el-select
            v-model="formData.counterparty_id"
            placeholder="请选择相对方"
            filterable
            style="width: 100%"
          >
            <el-option
              v-for="customer in customers"
              :key="customer.id"
              :label="customer.name"
              :value="customer.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="isEditMode ? '已绑定附件' : '入库附件'" prop="document_ids">
          <div class="document-block">
            <div class="document-toolbar">
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                :before-upload="handleSelectDocument"
                accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg"
              >
                <el-button type="primary" :loading="uploadingDocument">
                  <el-icon><UploadFilled /></el-icon>
                  {{ isEditMode ? '补充上传附件' : '上传盖章合同附件' }}
                </el-button>
              </el-upload>
              <span class="field-tip">
                {{ isEditMode ? '编辑时上传的新附件会自动提交并绑定到当前合同。' : '新增时必须至少有一个已提交附件才能完成入库。' }}
              </span>
            </div>

            <div class="document-list">
              <div v-for="item in formDocuments" :key="item.id" class="document-item">
                <div class="document-main">
                  <span class="document-name">{{ item.name || item.id }}</span>
                  <span class="document-meta">{{ item.status || '-' }}</span>
                </div>
                <div class="document-actions">
                  <el-button type="success" link @click="handleDownloadDocument(item)">
                    <el-icon><Download /></el-icon>
                  </el-button>
                  <el-button type="danger" link @click="removeDocumentId(item.id)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </div>
              </div>
              <span v-if="formDocuments.length === 0" class="text-gray">当前未添加附件</span>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">
            {{ isEditMode ? '保存修改' : '确认入库' }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Download, Edit, Plus, UploadFilled, View } from '@element-plus/icons-vue'
import {
  commitTempDocument,
  deleteContract,
  downloadTempDocument,
  getContractDetail,
  getContractList,
  getDocumentTempDetail,
  intakeContract,
  updateContract,
  uploadTempDocument
} from '@/api/contract'
import { getApprovalRequests } from '@/api/approval'
import { getCustomerList } from '@/api/customer'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const dialogVisible = ref(false)
const dialogTitle = ref('')
const formRef = ref(null)
const rawContracts = ref([])
const pagedRows = ref([])
const rawApprovals = ref([])
const customers = ref([])
const customerMap = ref({})
const formDocuments = ref([])
const uploadingDocument = ref(false)
const submitting = ref(false)

const statusOptions = [
  { value: 'registered', label: '已登记' },
  { value: 'active', label: '生效中' },
  { value: 'in_progress', label: '履约中' },
  { value: 'pending_pay', label: '待付款' },
  { value: 'completed', label: '已完成' },
  { value: 'terminated', label: '已终止' },
  { value: 'archived', label: '已归档' }
]

const searchForm = reactive({
  title: '',
  status: ''
})

const pagination = reactive({
  page: 1,
  size: 10,
  total: 0
})

const formData = reactive({
  id: '',
  title: '',
  counterparty_id: '',
  document_ids: []
})

const isEditMode = computed(() => Boolean(formData.id))

const formRules = {
  title: [{ required: true, message: '请输入合同标题', trigger: 'blur' }],
  counterparty_id: [{ required: true, message: '请选择相对方', trigger: 'change' }],
  document_ids: [{
    validator: (_rule, value, callback) => {
      if (!Array.isArray(value) || value.length === 0) {
        callback(new Error('请至少添加一个附件'))
        return
      }
      callback()
    },
    trigger: 'change'
  }]
}

const approvalSummaryByContract = computed(() => {
  const result = {}
  rawApprovals.value.forEach(item => {
    if (item.status !== 'pending' || !item.contract_id) return
    if (!result[item.contract_id]) {
      result[item.contract_id] = []
    }
    result[item.contract_id].push(item)
  })
  return result
})

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

const getStatusType = (status) => {
  const map = {
    registered: 'info',
    active: 'primary',
    in_progress: 'primary',
    pending_pay: 'warning',
    completed: 'success',
    terminated: 'danger',
    archived: 'info'
  }
  return map[status] || 'info'
}

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

const getApprovalRequestTypeText = (type) => {
  const map = {
    status_change: '状态变更',
    plan_adjustment: '履约计划调整',
    archive_borrow: '档案借阅',
    archive_destroy: '档案销毁'
  }
  return map[type] || type || '-'
}

const normalizeContracts = (contracts) => {
  return contracts.map(item => ({
    ...item,
    counterparty_name: customerMap.value[item.counterparty_id] || item.counterparty_name || '',
    document_ids: Array.isArray(item.document_ids) ? item.document_ids : []
  }))
}

const buildRows = () => {
  let rows = normalizeContracts(rawContracts.value).map(item => {
    const pendingApprovals = approvalSummaryByContract.value[item.id] || []
    return {
      ...item,
      pending_approval_count: pendingApprovals.length,
      pending_approval_summary: pendingApprovals.map(req => getApprovalRequestTypeText(req.request_type)).join('、') || '-'
    }
  })

  if (searchForm.title) {
    const keyword = searchForm.title.trim().toLowerCase()
    rows = rows.filter(item => (item.title || '').toLowerCase().includes(keyword))
  }
  if (searchForm.status) {
    rows = rows.filter(item => item.status === searchForm.status)
  }

  pagination.total = rows.length
  const start = (pagination.page - 1) * pagination.size
  const end = start + pagination.size
  pagedRows.value = rows.slice(start, end)
}

const refreshPagedRows = () => {
  buildRows()
}

const loadData = async () => {
  loading.value = true
  try {
    const [contracts, approvals] = await Promise.all([
      getContractList({}),
      getApprovalRequests()
    ])
    rawContracts.value = Array.isArray(contracts) ? contracts : []
    rawApprovals.value = Array.isArray(approvals) ? approvals : []
    buildRows()
  } finally {
    loading.value = false
  }
}

const loadCustomers = async () => {
  try {
    const res = await getCustomerList({ limit: 1000 })
    const list = Array.isArray(res) ? res : (res?.data || [])
    customers.value = list
    customerMap.value = list.reduce((acc, item) => {
      acc[item.id] = item.name
      return acc
    }, {})
  } catch {
    customers.value = []
    customerMap.value = {}
  }
}

const resetFormData = () => {
  Object.assign(formData, {
    id: '',
    title: '',
    counterparty_id: '',
    document_ids: []
  })
  formDocuments.value = []
}

const parseDownloadFileName = (contentDisposition, fallbackName) => {
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }
  const plainMatch = contentDisposition.match(/filename="?([^"]+)"?/i)
  if (plainMatch?.[1]) {
    return plainMatch[1]
  }
  return fallbackName || 'document'
}

const loadFormDocuments = async () => {
  if (!Array.isArray(formData.document_ids) || formData.document_ids.length === 0) {
    formDocuments.value = []
    return
  }
  const rows = await Promise.all(
    formData.document_ids.map(async (id) => {
      try {
        const doc = await getDocumentTempDetail(id)
        return {
          id: doc.id,
          name: doc.file_name || doc.id,
          status: doc.status || '-',
          created_at: doc.created_at
        }
      } catch {
        return {
          id,
          name: id,
          status: 'missing'
        }
      }
    })
  )
  formDocuments.value = rows
}

const handleAdd = () => {
  resetFormData()
  dialogTitle.value = '合同入库'
  dialogVisible.value = true
}

const handleEdit = async (row) => {
  dialogTitle.value = '编辑合同'
  Object.assign(formData, {
    id: row.id,
    title: row.title || '',
    counterparty_id: row.counterparty_id || '',
    document_ids: Array.isArray(row.document_ids) ? [...row.document_ids] : []
  })
  await loadFormDocuments()
  dialogVisible.value = true
}

const handleView = (row) => {
  router.push(`/contracts/${row.id}`)
}

const handleDelete = async (row) => {
  await ElMessageBox.confirm('确定删除该合同吗？已绑定附件会释放回已提交状态。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
  await deleteContract(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

const handleSearch = () => {
  pagination.page = 1
  refreshPagedRows()
}

const handleReset = () => {
  Object.assign(searchForm, { title: '', status: '' })
  handleSearch()
}

const handleSelectDocument = async (file) => {
  uploadingDocument.value = true
  try {
    const uploaded = await uploadTempDocument(file)
    await commitTempDocument(uploaded.id)
    formData.document_ids = [...new Set([...(formData.document_ids || []), uploaded.id])]
    await loadFormDocuments()
    ElMessage.success('附件已上传并提交')
    formRef.value?.validateField('document_ids')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '附件上传失败')
  } finally {
    uploadingDocument.value = false
  }
  return false
}

const removeDocumentId = (documentId) => {
  formData.document_ids = formData.document_ids.filter(item => item !== documentId)
  formDocuments.value = formDocuments.value.filter(item => item.id !== documentId)
  formRef.value?.validateField('document_ids')
}

const handleDownloadDocument = async (row) => {
  try {
    const { blob, contentDisposition } = await downloadTempDocument(row.id)
    const url = window.URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = parseDownloadFileName(contentDisposition, row.name)
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    window.URL.revokeObjectURL(url)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '附件下载失败')
  }
}

const toPayload = () => ({
  title: formData.title.trim(),
  counterparty_id: formData.counterparty_id,
  document_ids: formData.document_ids
})

const handleSubmit = async () => {
  await formRef.value.validate()
  submitting.value = true
  try {
    const payload = toPayload()
    if (isEditMode.value) {
      await updateContract(formData.id, payload)
      ElMessage.success('更新成功')
    } else {
      await intakeContract(payload)
      ElMessage.success('合同入库成功')
    }
    dialogVisible.value = false
    await loadData()
  } finally {
    submitting.value = false
  }
}

const handleDialogClose = () => {
  formRef.value?.resetFields()
  resetFormData()
}

onMounted(async () => {
  if (route.query.status) {
    searchForm.status = String(route.query.status)
  }
  if (route.query.title) {
    searchForm.title = String(route.query.title)
  }

  await loadCustomers()
  await loadData()

  if (route.query.action === 'create') {
    handleAdd()
    window.history.replaceState({}, '', '/contracts')
  } else if (route.query.action === 'edit' && route.query.id) {
    const data = await getContractDetail(String(route.query.id))
    await handleEdit(data)
    window.history.replaceState({}, '', '/contracts')
  }
})
</script>

<style scoped>
.contract-page {
  padding: 16px;
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.search-right {
  margin-left: auto;
}

.table-card {
  border-radius: 8px;
}

.title-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.reminder-tag {
  flex-shrink: 0;
}

.workflow-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.workflow-text {
  font-size: 12px;
  color: #606266;
}

.text-gray {
  color: #909399;
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.action-buttons .el-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.dialog-alert {
  margin-bottom: 16px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.document-block {
  width: 100%;
}

.document-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.document-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.document-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background: #fafafa;
}

.document-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.document-name {
  color: #303133;
}

.document-meta,
.field-tip {
  font-size: 12px;
  color: #909399;
}

.document-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
</style>
