<template>
  <div class="contract-detail">
    <el-card>
      <template #header>
        <div class="card-header">
          <el-button text @click="$router.back()">
            <el-icon><ArrowLeft /></el-icon>
            返回
          </el-button>
          <span class="title">合同详情</span>
          <div class="header-actions">
            <el-button @click="handlePerformance">履约计划</el-button>
            <el-button type="primary" @click="handleEdit">编辑合同</el-button>
          </div>
        </div>
      </template>

      <el-tabs v-model="activeTab" type="border-card" @tab-change="tabChange">
        <el-tab-pane label="基本信息" name="info">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="合同编号">{{ contract.contract_no || '-' }}</el-descriptions-item>
            <el-descriptions-item label="合同标题">{{ contract.title || '-' }}</el-descriptions-item>
            <el-descriptions-item label="相对方">{{ contract.counterparty_name || contract.counterparty_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="合同状态">
              <el-tag :type="getStatusType(contract.status)">{{ getStatusText(contract.status) }}</el-tag>
              <div class="status-actions">
                <el-button type="primary" link size="small" @click="openStatusDialog">
                  <el-icon><RefreshRight /></el-icon>
                  状态变更
                </el-button>
                <el-button v-if="contract.status !== 'archived'" type="warning" link size="small" @click="handleArchive">
                  <el-icon><FolderOpened /></el-icon>
                  申请归档
                </el-button>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="附件数量">{{ Array.isArray(contract.document_ids) ? contract.document_ids.length : 0 }}</el-descriptions-item>
            <el-descriptions-item label="创建人">{{ contract.created_by || 'system' }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatDateTime(contract.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="最新补充协议" :span="2">
              {{ contract.latest_amendment_title || contract.latest_amendment_id || '-' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-tab-pane>

        <el-tab-pane label="生命周期" name="lifecycle">
          <div class="tab-header">
            <span>合同生命周期</span>
          </div>
          <el-timeline>
            <el-timeline-item
              v-for="event in lifecycleEvents"
              :key="event.id || `${event.event_type}-${event.occurred_at || event.created_at}`"
              :timestamp="formatDateTime(event.occurred_at || event.created_at)"
              :type="getLifecycleItemType(event.event_type)"
            >
              <div class="lifecycle-content">
                <div class="lifecycle-title">{{ getLifecycleTitle(event.event_type) }}</div>
                <div class="lifecycle-desc">
                  <span v-if="event.from_status || event.to_status">
                    {{ getStatusText(event.from_status) }} -> {{ getStatusText(event.to_status) }}
                  </span>
                  <span v-if="event.description">{{ event.description }}</span>
                </div>
              </div>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-if="lifecycleEvents.length === 0" description="暂无生命周期记录" />
        </el-tab-pane>

        <el-tab-pane label="文档管理" name="documents">
          <div class="tab-header">
            <span>合同附件</span>
            <div class="document-actions">
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                :before-upload="handleSelectDocument"
                accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg"
              >
                <el-button type="primary" :loading="uploadingDocument">
                  <el-icon><UploadFilled /></el-icon>
                  上传附件
                </el-button>
              </el-upload>
              <span class="hint-text">上传后自动提交并绑定到当前合同</span>
            </div>
          </div>
          <el-table :data="documents" v-loading="documentsLoading" :cell-style="{ padding: '8px 0' }">
            <el-table-column prop="name" label="文件名称" min-width="220" />
            <el-table-column prop="file_type" label="类型" width="120" />
            <el-table-column prop="file_size" label="大小" width="120">
              <template #default="{ row }">
                {{ formatFileSize(row.file_size) }}
              </template>
            </el-table-column>
            <el-table-column prop="version" label="状态" width="120" />
            <el-table-column prop="created_at" label="上传时间" width="180">
              <template #default="{ row }">
                {{ formatDateTime(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <div class="action-buttons">
                  <el-button type="success" link @click="handleDownload(row)">
                    <el-icon><Download /></el-icon>
                  </el-button>
                  <el-button type="danger" link :disabled="removingDocumentId === row.id" @click="handleRemoveDocument(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="审批记录" name="approvals">
          <div class="tab-header">
            <span>审批历史</span>
            <el-button v-if="canCreateApproval" type="primary" size="small" @click="showApprovalDialog = true">
              <el-icon><Plus /></el-icon>
              发起审批
            </el-button>
          </div>
          <el-table :data="approvals" v-loading="approvalsLoading" :cell-style="{ padding: '8px 0' }">
            <el-table-column label="审批类型" width="140">
              <template #default="{ row }">
                <el-tag :type="getApprovalRequestTypeTag(row.request_type)">{{ getApprovalRequestTypeText(row.request_type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="requested_by" label="申请人" width="120" />
            <el-table-column prop="approver.full_name" label="审批人" width="120" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="getApprovalStatusType(row.status)">{{ getApprovalStatusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="comment" label="说明" min-width="220" />
            <el-table-column prop="approved_at" label="审批时间" width="180">
              <template #default="{ row }">
                {{ formatDateTime(row.approved_at) }}
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="提交时间" width="180">
              <template #default="{ row }">
                {{ formatDateTime(row.created_at) }}
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog v-model="showApprovalDialog" title="发起审批" width="520px">
      <el-form ref="approvalFormRef" :model="approvalForm" :rules="approvalRules" label-width="100px">
        <el-form-item label="审批类型" prop="request_type">
          <el-select v-model="approvalForm.request_type" style="width: 100%">
            <el-option label="状态变更" value="status_change" />
            <el-option label="履约计划调整" value="plan_adjustment" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="approvalForm.request_type === 'status_change'" label="目标状态" prop="to_status">
          <el-select v-model="approvalForm.to_status" style="width: 100%" placeholder="请选择目标状态">
            <el-option v-for="opt in getAvailableStatusOptions()" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="审批说明" prop="comment">
          <el-input v-model="approvalForm.comment" type="textarea" :rows="4" placeholder="请输入审批说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showApprovalDialog = false">取消</el-button>
          <el-button type="primary" @click="handleSubmitApproval">发起审批</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="showStatusDialog" title="发起状态变更审批" width="500px">
      <el-form label-width="100px">
        <el-form-item label="当前状态">
          <el-tag :type="getStatusType(contract.status)">{{ getStatusText(contract.status) }}</el-tag>
        </el-form-item>
        <el-form-item label="目标状态">
          <el-select v-model="newStatus" style="width: 100%" placeholder="请选择目标状态">
            <el-option v-for="opt in getAvailableStatusOptions()" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="变更说明">
          <el-input v-model="statusDescription" type="textarea" :rows="3" placeholder="请输入状态变更原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showStatusDialog = false">取消</el-button>
          <el-button type="primary" @click="handleUpdateStatus">提交审批</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Delete, Download, FolderOpened, Plus, RefreshRight, UploadFilled } from '@element-plus/icons-vue'
import {
  commitTempDocument,
  downloadTempDocument,
  getContractDetail,
  getContractDocuments,
  getContractLifecycle,
  updateContract,
  uploadTempDocument
} from '@/api/contract'
import { createApproval, getApprovalRecords } from '@/api/approval'
import { getCustomerDetail } from '@/api/customer'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const activeTab = ref('info')
const contract = ref({})
const documents = ref([])
const approvals = ref([])
const lifecycleEvents = ref([])
const documentsLoading = ref(false)
const approvalsLoading = ref(false)
const showApprovalDialog = ref(false)
const showStatusDialog = ref(false)
const newStatus = ref('')
const statusDescription = ref('')
const approvalFormRef = ref(null)
const uploadingDocument = ref(false)
const removingDocumentId = ref('')

const contractId = computed(() => String(route.params.id || ''))
const canCreateApproval = computed(() => contract.value?.status !== 'archived')

const approvalForm = reactive({
  request_type: 'status_change',
  to_status: 'active',
  comment: ''
})

const approvalRules = {
  request_type: [{ required: true, message: '请选择审批类型', trigger: 'change' }],
  to_status: [{ required: true, message: '请选择目标状态', trigger: 'change' }],
  comment: [{ required: true, message: '请输入审批说明', trigger: 'blur' }]
}

const statusOptions = computed(() => {
  const currentStatus = contract.value?.status
  const allOptions = [
    { value: 'registered', label: '已登记', from: [''] },
    { value: 'active', label: '生效中', from: ['registered', 'approved'] },
    { value: 'in_progress', label: '履约中', from: ['active'] },
    { value: 'pending_pay', label: '待付款', from: ['active', 'in_progress'] },
    { value: 'completed', label: '已完成', from: ['active', 'in_progress', 'pending_pay'] },
    { value: 'terminated', label: '已终止', from: ['registered', 'active', 'in_progress', 'pending_pay', 'completed'] },
    { value: 'archived', label: '已归档', from: ['completed', 'terminated', 'active'] }
  ]
  return allOptions.filter((item) => !currentStatus || item.from.includes(currentStatus))
})

const getAvailableStatusOptions = () => statusOptions.value

const getStatusType = (status) => {
  const map = {
    registered: 'info',
    approved: 'success',
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
    approved: '已审批',
    active: '生效中',
    in_progress: '履约中',
    pending_pay: '待付款',
    completed: '已完成',
    terminated: '已终止',
    archived: '已归档'
  }
  return map[status] || status || '-'
}

const getLifecycleItemType = (eventType) => {
  const map = {
    'contract.created': 'primary',
    'contract.intake.created': 'primary',
    'contract.status_changed': 'warning',
    'contract.amendment_applied': 'success',
    'contract.updated': 'warning'
  }
  return map[eventType] || 'info'
}

const getLifecycleTitle = (eventType) => {
  const map = {
    'contract.created': '合同创建',
    'contract.intake.created': '合同入库',
    'contract.status_changed': '状态变更',
    'contract.amendment_applied': '补充协议生效',
    'contract.updated': '合同信息更新'
  }
  return map[eventType] || eventType || '-'
}

const getApprovalStatusType = (status) => {
  const map = { pending: 'warning', approved: 'success', rejected: 'danger' }
  return map[status] || 'info'
}

const getApprovalStatusText = (status) => {
  const map = { pending: '待审批', approved: '已通过', rejected: '已驳回' }
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

const getApprovalRequestTypeTag = (type) => {
  const map = {
    status_change: 'warning',
    plan_adjustment: 'primary',
    archive_borrow: 'success',
    archive_destroy: 'danger'
  }
  return map[type] || 'info'
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

const formatFileSize = (bytes) => {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
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

const mergeCounterpartyName = async (data) => {
  if (!data?.counterparty_id) return data
  try {
    const counterparty = await getCustomerDetail(data.counterparty_id)
    return {
      ...data,
      counterparty_name: counterparty?.name || data.counterparty_name
    }
  } catch {
    return {
      ...data,
      counterparty_name: data.counterparty_name || ''
    }
  }
}

const reloadContract = async () => {
  contract.value = await mergeCounterpartyName(await getContractDetail(contractId.value))
}

const loadDocuments = async () => {
  documentsLoading.value = true
  try {
    const data = await getContractDocuments(contractId.value)
    documents.value = Array.isArray(data) ? data : []
  } finally {
    documentsLoading.value = false
  }
}

const loadApprovals = async () => {
  approvalsLoading.value = true
  try {
    approvals.value = await getApprovalRecords(contractId.value)
  } finally {
    approvalsLoading.value = false
  }
}

const loadLifecycle = async () => {
  try {
    const data = await getContractLifecycle(contractId.value)
    lifecycleEvents.value = Array.isArray(data) ? data : []
  } catch (error) {
    console.error('Failed to load lifecycle:', error)
  }
}

const reloadDocumentSection = async () => {
  await reloadContract()
  await loadDocuments()
}

const updateDocumentBindings = async (documentIds) => {
  await updateContract(contractId.value, {
    title: contract.value.title,
    counterparty_id: contract.value.counterparty_id,
    document_ids: documentIds
  })
}

const handleSelectDocument = async (file) => {
  uploadingDocument.value = true
  try {
    const uploaded = await uploadTempDocument(file)
    await commitTempDocument(uploaded.id)
    const nextDocumentIDs = [...new Set([...(contract.value.document_ids || []), uploaded.id])]
    await updateDocumentBindings(nextDocumentIDs)
    ElMessage.success('附件上传并绑定成功')
    await reloadDocumentSection()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '附件上传失败')
  } finally {
    uploadingDocument.value = false
  }
  return false
}

const handleRemoveDocument = async (row) => {
  await ElMessageBox.confirm('确定移除该附件吗？移除后会释放附件绑定状态。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
  removingDocumentId.value = row.id
  try {
    const nextDocumentIDs = (contract.value.document_ids || []).filter((id) => id !== row.id)
    await updateDocumentBindings(nextDocumentIDs)
    ElMessage.success('附件已移除')
    await reloadDocumentSection()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '移除附件失败')
  } finally {
    removingDocumentId.value = ''
  }
}

const openStatusDialog = () => {
  newStatus.value = ''
  statusDescription.value = ''
  showStatusDialog.value = true
}

const getOperator = () => userStore.userInfo?.username || userStore.userInfo?.id || 'system'

const handleUpdateStatus = async () => {
  if (!newStatus.value) {
    ElMessage.error('请选择目标状态')
    return
  }
  try {
    await createApproval({
      contract_id: contractId.value,
      request_type: 'status_change',
      requested_by: getOperator(),
      to_status: newStatus.value,
      comment: statusDescription.value
    })
    ElMessage.success('状态变更审批已提交')
    showStatusDialog.value = false
    newStatus.value = ''
    statusDescription.value = ''
    loadApprovals()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '提交失败')
  }
}

const handleArchive = async () => {
  try {
    await ElMessageBox.confirm(
      '归档通过审批子流程提交，审批通过后再进入归档链路，是否继续？',
      '申请归档',
      {
        confirmButtonText: '确认提交',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await createApproval({
      contract_id: contractId.value,
      request_type: 'status_change',
      requested_by: getOperator(),
      to_status: 'archived',
      comment: '申请归档'
    })
    ElMessage.success('归档审批已提交')
    loadApprovals()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '提交失败')
    }
  }
}

const handleEdit = () => {
  router.push(`/contracts?action=edit&id=${contractId.value}`)
}

const handlePerformance = () => {
  router.push(`/contracts/${contractId.value}/performance`)
}

const handleDownload = async (row) => {
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

const handleSubmitApproval = async () => {
  await approvalFormRef.value.validate()
  const payload = approvalForm.request_type === 'plan_adjustment'
    ? {
        nodes: [
          {
            node_name: '待补充计划节点',
            node_type: 'manual_adjustment',
            due_date: new Date().toISOString()
          }
        ],
        comment: approvalForm.comment
      }
    : {
        status: approvalForm.to_status || 'active',
        comment: approvalForm.comment
      }

  await createApproval({
    contract_id: contractId.value,
    request_type: approvalForm.request_type,
    requested_by: getOperator(),
    to_status: approvalForm.to_status,
    payload,
    comment: approvalForm.comment
  })

  ElMessage.success('审批已发起')
  showApprovalDialog.value = false
  approvalForm.request_type = 'status_change'
  approvalForm.to_status = 'active'
  approvalForm.comment = ''
  loadApprovals()
}

const tabChange = (tab) => {
  if (tab === 'documents') loadDocuments()
  if (tab === 'approvals') loadApprovals()
  if (tab === 'lifecycle') loadLifecycle()
}

watch(() => route.params.id, () => {
  if (route.params.id) {
    reloadContract()
    loadDocuments()
    loadApprovals()
    loadLifecycle()
  }
})

onMounted(async () => {
  await reloadContract()
  loadApprovals()
  loadLifecycle()
})
</script>

<style scoped>
.contract-detail {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title {
  font-size: 18px;
  font-weight: 600;
}

.tab-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-weight: 600;
  color: #1e293b;
}

.document-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.hint-text {
  font-size: 12px;
  color: #909399;
}

.status-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 12px;
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 4px;
}

.action-buttons .el-button,
.status-actions .el-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.lifecycle-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.lifecycle-title {
  font-weight: 600;
  color: #303133;
}

.lifecycle-desc {
  color: #606266;
  line-height: 1.6;
}
</style>
