<template>
  <div class="dashboard">
    <div class="page-header">
      <div class="welcome-section">
        <h1 class="page-title">仪表盘</h1>
        <p class="page-desc">当前展示合同、审批、风险、归档、结案聚合视图</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" :icon="Plus" @click="createContract">新建合同</el-button>
      </div>
    </div>

    <el-row :gutter="24" class="stats-row">
      <el-col :span="6" v-for="(stat, index) in statsCards" :key="index">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" :style="{ background: stat.gradient }">
              <el-icon :size="24"><component :is="stat.icon" /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ stat.value }}</div>
              <div class="stat-label">{{ stat.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="24" class="charts-row">
      <el-col :span="14">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>
                <el-icon><TrendCharts /></el-icon>
                合同状态统计
              </span>
              <el-radio-group v-model="chartType" size="small">
                <el-radio-button label="pie">饼图</el-radio-button>
                <el-radio-button label="bar">柱状图</el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <div ref="chartRef" style="height: 320px"></div>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card class="overview-card">
          <template #header>
            <div class="card-header">
              <span>
                <el-icon><DataAnalysis /></el-icon>
                业务概览
              </span>
            </div>
          </template>
          <div class="overview-grid">
            <div class="overview-item">
              <div class="overview-icon indigo">
                <el-icon :size="22"><Document /></el-icon>
              </div>
              <div class="overview-content">
                <div class="overview-value">{{ statistics.contract_total }}</div>
                <div class="overview-label">合同总数</div>
              </div>
            </div>
            <div class="overview-item">
              <div class="overview-icon green">
                <el-icon :size="22"><FolderOpened /></el-icon>
              </div>
              <div class="overview-content">
                <div class="overview-value">{{ statistics.archived_contracts }}</div>
                <div class="overview-label">已归档</div>
              </div>
            </div>
            <div class="overview-item">
              <div class="overview-icon amber">
                <el-icon :size="22"><Finished /></el-icon>
              </div>
              <div class="overview-content">
                <div class="overview-value">{{ statistics.pending_closures }}</div>
                <div class="overview-label">待结案</div>
              </div>
            </div>
            <div class="overview-item">
              <div class="overview-icon red">
                <el-icon :size="22"><Warning /></el-icon>
              </div>
              <div class="overview-content">
                <div class="overview-value">{{ statistics.high_risks }}</div>
                <div class="overview-label">高风险</div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="24">
      <el-col :span="12">
        <el-card class="table-card">
          <template #header>
            <div class="card-header">
              <span>
                <el-icon><Finished /></el-icon>
                待结案事项
              </span>
              <el-button type="primary" size="small" @click="$router.push('/closures')">查看全部</el-button>
            </div>
          </template>
          <el-table :data="pendingClosures" style="width: 100%">
            <el-table-column prop="contract_id" label="合同ID" min-width="120" />
            <el-table-column prop="request_type" label="类型" min-width="100">
              <template #default="{ row }">
                {{ getClosureTypeText(row.request_type) }}
              </template>
            </el-table-column>
            <el-table-column prop="requested_by" label="申请人" min-width="100" />
            <el-table-column prop="created_at" label="提交时间" min-width="150">
              <template #default="{ row }">
                {{ formatDateTime(row.created_at) }}
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="table-card">
          <template #header>
            <div class="card-header">
              <span>
                <el-icon><FolderOpened /></el-icon>
                归档关注事项
              </span>
              <el-button type="primary" size="small" @click="$router.push('/archives')">查看全部</el-button>
            </div>
          </template>
          <el-table :data="attentionArchives" style="width: 100%">
            <el-table-column prop="id" label="归档ID" min-width="120" />
            <el-table-column prop="contract_id" label="合同ID" min-width="120" />
            <el-table-column prop="borrow_status" label="借阅状态" min-width="100">
              <template #default="{ row }">
                {{ row.borrow_status === 'borrowed' ? '已借阅' : '空闲' }}
              </template>
            </el-table-column>
            <el-table-column prop="destroy_state" label="销毁状态" min-width="100">
              <template #default="{ row }">
                {{ row.destroy_state === 'requested' ? '待销毁' : '保留' }}
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" min-width="150">
              <template #default="{ row }">
                {{ formatDateTime(row.created_at) }}
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { getDashboardReport, getWorkbenchReport } from '@/api/report'
import {
  DataAnalysis,
  Document,
  Finished,
  FolderOpened,
  Plus,
  TrendCharts,
  Warning
} from '@element-plus/icons-vue'

const router = useRouter()
const chartRef = ref(null)
const chartType = ref('bar')
const chartInstance = ref(null)
const pendingClosures = ref([])
const attentionArchives = ref([])

const statistics = ref({
  contract_total: 0,
  pending_approvals: 0,
  open_risks: 0,
  high_risks: 0,
  archived_contracts: 0,
  borrowed_archives: 0,
  pending_destroy_archives: 0,
  pending_closures: 0,
  completed_closures: 0
})

const statusBreakdown = ref({})

const statsCards = computed(() => [
  {
    icon: 'Document',
    label: '合同总数',
    value: statistics.value.contract_total,
    gradient: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)'
  },
  {
    icon: 'Warning',
    label: '待审批',
    value: statistics.value.pending_approvals,
    gradient: 'linear-gradient(135deg, #F59E0B 0%, #FBBF24 100%)'
  },
  {
    icon: 'FolderOpened',
    label: '已归档',
    value: statistics.value.archived_contracts,
    gradient: 'linear-gradient(135deg, #10B981 0%, #34D399 100%)'
  },
  {
    icon: 'Finished',
    label: '待结案',
    value: statistics.value.pending_closures,
    gradient: 'linear-gradient(135deg, #EF4444 0%, #F87171 100%)'
  }
])

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

const getClosureTypeText = (type) => {
  const map = {
    close: '结案',
    terminate: '终止',
    cancel: '解除'
  }
  return map[type] || type || '-'
}

const loadStatistics = async () => {
  const data = await getDashboardReport()
  const overview = data?.overview || {}
  statistics.value = {
    contract_total: overview.contract_total ?? 0,
    pending_approvals: overview.pending_approvals ?? 0,
    open_risks: overview.open_risks ?? 0,
    high_risks: overview.high_risks ?? 0,
    archived_contracts: overview.archived_contracts ?? 0,
    borrowed_archives: overview.borrowed_archives ?? 0,
    pending_destroy_archives: overview.pending_destroy_archives ?? 0,
    pending_closures: overview.pending_closures ?? 0,
    completed_closures: overview.completed_closures ?? 0
  }
  statusBreakdown.value = data?.contract_status_breakdown || {}
  nextTick(() => initChart())
}

const loadWorkbench = async () => {
  const data = await getWorkbenchReport()
  pendingClosures.value = data?.pending_closures || []
  attentionArchives.value = data?.attention_archives || []
}

const getPieOption = () => ({
  tooltip: {
    trigger: 'item',
    formatter: '{b}: {c}'
  },
  legend: {
    orient: 'vertical',
    right: 20,
    top: 'center'
  },
  color: ['#6366F1', '#10B981', '#F59E0B', '#EF4444', '#94A3B8'],
  series: [{
    type: 'pie',
    radius: ['45%', '70%'],
    center: ['45%', '50%'],
    data: Object.entries(statusBreakdown.value).map(([name, value]) => ({
      name,
      value
    }))
  }]
})

const getBarOption = () => ({
  tooltip: {
    trigger: 'axis'
  },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    data: Object.keys(statusBreakdown.value),
    axisLine: { lineStyle: { color: '#E2E8F0' } },
    axisLabel: { color: '#64748B', fontSize: 12 }
  },
  yAxis: {
    type: 'value',
    axisLine: { show: false },
    axisLabel: { color: '#64748B' },
    splitLine: { lineStyle: { color: '#F1F5F9' } }
  },
  series: [{
    data: Object.values(statusBreakdown.value),
    type: 'bar',
    barWidth: '50%',
    itemStyle: {
      borderRadius: [6, 6, 0, 0],
      color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: '#6366F1' },
        { offset: 1, color: '#8B5CF6' }
      ])
    }
  }]
})

const initChart = () => {
  if (!chartRef.value) return
  if (chartInstance.value) {
    chartInstance.value.dispose()
  }
  chartInstance.value = echarts.init(chartRef.value)
  chartInstance.value.setOption(chartType.value === 'pie' ? getPieOption() : getBarOption())
}

watch(chartType, () => {
  if (chartInstance.value) {
    initChart()
  }
})

const createContract = () => {
  router.push('/contracts?action=create')
}

onMounted(async () => {
  await Promise.all([loadStatistics(), loadWorkbench()])
})

onUnmounted(() => {
  if (chartInstance.value) {
    chartInstance.value.dispose()
    chartInstance.value = null
  }
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #1E293B;
  margin: 0 0 4px;
}

.page-desc {
  color: #64748B;
  margin: 0;
  font-size: 14px;
}

.stats-row {
  margin-bottom: 24px;
}

.stat-card,
.chart-card,
.table-card,
.overview-card {
  border: none;
  border-radius: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #1E293B;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: #64748B;
  margin-top: 2px;
}

.charts-row {
  margin-bottom: 24px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  font-size: 15px;
  width: 100%;
}

.card-header span {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #1E293B;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.overview-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  background: #F8FAFC;
  border-radius: 12px;
}

.overview-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
}

.overview-icon.indigo {
  background: linear-gradient(135deg, #6366F1, #8B5CF6);
}

.overview-icon.green {
  background: linear-gradient(135deg, #10B981, #34D399);
}

.overview-icon.amber {
  background: linear-gradient(135deg, #F59E0B, #FBBF24);
}

.overview-icon.red {
  background: linear-gradient(135deg, #EF4444, #F87171);
}

.overview-value {
  font-size: 18px;
  font-weight: 700;
  color: #1E293B;
}

.overview-label {
  font-size: 12px;
  color: #64748B;
  margin-top: 2px;
}

:deep(.el-card__header) {
  padding: 16px 20px;
  border-bottom: 1px solid #F1F5F9;
}

:deep(.el-card__body) {
  padding: 20px;
}

:deep(.el-button--primary) {
  background: linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%);
  border: none;
}
</style>
