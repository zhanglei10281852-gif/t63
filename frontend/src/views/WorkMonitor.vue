<template>
  <div class="work-monitor">
    <a-card>
      <template #title>
        <div class="card-title">
          <span>今日任务列表</span>
          <a-date-picker v-model:value="selectedDate" style="margin-left: 16px" @change="fetchPlans" />
        </div>
      </template>

      <a-tabs v-model:activeKey="activeTab" @change="handleTabChange">
        <a-tab-pane key="all" tab="全部" />
        <a-tab-pane key="pending" tab="待完成" />
        <a-tab-pane key="completed" tab="已完成" />
        <a-tab-pane key="late" tab="迟到打卡" />
        <a-tab-pane key="missed" tab="未完成" />
      </a-tabs>

      <a-table :columns="columns" :data-source="plans" :loading="loading" row-key="id" :pagination="{ pageSize: 10 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-tag :color="getStatusColor(record.status)">
              {{ getStatusText(record.status) }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'worker'">
            <span v-if="record.worker">{{ record.worker.name }}</span>
            <span v-else style="color: #999">未分配</span>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-button type="link" size="small" @click="showAssignModal(record)" v-if="canAssign">
              调整人员
            </a-button>
            <a-button type="link" size="small" @click="showCheckinModal(record)" v-if="record.status === 'pending'">
              打卡
            </a-button>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card style="margin-top: 16px" title="打卡记录 (Timeline)">
      <a-timeline>
        <a-timeline-item
          v-for="record in checkinRecords"
          :key="record.id"
          :color="record.status === 'late' ? 'orange' : 'green'"
        >
          <div class="timeline-item">
            <div class="timeline-time">{{ formatTime(record.checkin_time) }}</div>
            <div class="timeline-content">
              <strong>{{ record.road_section?.name }}</strong>
              <span class="timeline-worker"> - {{ record.worker?.name || '未分配' }}</span>
              <a-tag :color="record.status === 'late' ? 'orange' : 'green'" style="margin-left: 8px">
                {{ record.status === 'late' ? '迟到' : '准时' }}
              </a-tag>
              <div class="timeline-method">作业方式: {{ getWorkMethodText(record.work_method) }}</div>
            </div>
          </div>
        </a-timeline-item>
      </a-timeline>
      <div v-if="checkinRecords.length === 0" style="text-align: center; color: #999; padding: 40px">
        暂无打卡记录
      </div>
    </a-card>

    <a-modal v-model:open="assignModalVisible" title="调整作业人员" @ok="handleAssign">
      <a-form :model="assignForm" layout="vertical">
        <a-form-item label="当前任务">
          <span>{{ currentPlan?.road_section?.name }} - {{ currentPlan?.plan_time }}</span>
        </a-form-item>
        <a-form-item label="指派工人">
          <a-select v-model:value="assignForm.worker_id" placeholder="请选择工人" style="width: 100%">
            <a-select-option v-for="w in workers" :key="w.id" :value="w.id">
              {{ w.name }} ({{ w.road_section?.name || '未分配路段' }})
            </a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="checkinModalVisible" title="作业打卡" @ok="handleCheckin">
      <a-form :model="checkinForm" layout="vertical">
        <a-form-item label="任务">
          <span>{{ currentPlan?.road_section?.name }} - {{ currentPlan?.plan_time }}</span>
        </a-form-item>
        <a-form-item label="作业方式">
          <a-select v-model:value="checkinForm.work_method" style="width: 100%">
            <a-select-option value="manual">人工清扫</a-select-option>
            <a-select-option value="mechanical">机械清扫</a-select-option>
            <a-select-option value="washing">冲洗</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { getWorkPlans, getCheckinRecords, updatePlanWorker, checkin, getWorkers } from '../api'

const plans = ref([])
const checkinRecords = ref([])
const loading = ref(false)
const activeTab = ref('all')
const selectedDate = ref(dayjs())
const workers = ref([])

const assignModalVisible = ref(false)
const checkinModalVisible = ref(false)
const currentPlan = ref(null)
const assignForm = ref({ worker_id: null })
const checkinForm = ref({ work_method: 'manual' })

const user = JSON.parse(localStorage.getItem('user') || '{}')
const canAssign = computed(() => user.role === 'admin' || user.role === 'area_manager')

const columns = [
  { title: '路段名称', dataIndex: ['road_section', 'name'], key: 'road' },
  { title: '计划时间', dataIndex: 'plan_time', key: 'time', width: 100 },
  { title: '负责工人', key: 'worker', width: 120 },
  { title: '状态', key: 'status', width: 100 },
  { title: '打卡时间', dataIndex: 'checkin_time', key: 'checkin', width: 160 },
  { title: '操作', key: 'action', width: 160 }
]

const getStatusColor = (status) => {
  const colors = {
    pending: 'default',
    completed: 'green',
    late: 'orange',
    missed: 'red'
  }
  return colors[status] || 'default'
}

const getStatusText = (status) => {
  const texts = {
    pending: '待完成',
    completed: '已完成',
    late: '迟到打卡',
    missed: '未完成'
  }
  return texts[status] || status
}

const getWorkMethodText = (method) => {
  const texts = {
    manual: '人工清扫',
    mechanical: '机械清扫',
    washing: '冲洗'
  }
  return texts[method] || method
}

const formatTime = (time) => {
  if (!time) return ''
  return dayjs(time).format('YYYY-MM-DD HH:mm:ss')
}

const fetchPlans = async () => {
  loading.value = true
  try {
    const date = selectedDate.value ? selectedDate.value.format('YYYY-MM-DD') : ''
    const params = { date }
    if (activeTab.value !== 'all') {
      params.status = activeTab.value
    }
    const res = await getWorkPlans(params)
    plans.value = res
    fetchCheckinRecords()
  } catch (e) {
    message.error('获取任务列表失败')
  } finally {
    loading.value = false
  }
}

const fetchCheckinRecords = async () => {
  try {
    const date = selectedDate.value ? selectedDate.value.format('YYYY-MM-DD') : ''
    const res = await getCheckinRecords({ date })
    checkinRecords.value = res
  } catch (e) {
    console.error(e)
  }
}

const fetchWorkers = async () => {
  try {
    const res = await getWorkers()
    workers.value = res
  } catch (e) {
    console.error(e)
  }
}

const handleTabChange = (key) => {
  fetchPlans()
}

const showAssignModal = (record) => {
  currentPlan.value = record
  assignForm.value.worker_id = record.worker_id
  assignModalVisible.value = true
}

const handleAssign = async () => {
  try {
    await updatePlanWorker(currentPlan.value.id, { worker_id: assignForm.value.worker_id })
    message.success('人员调整成功')
    assignModalVisible.value = false
    fetchPlans()
  } catch (e) {
    message.error(e.response?.data?.error || '调整失败')
  }
}

const showCheckinModal = (record) => {
  currentPlan.value = record
  checkinForm.value.work_method = 'manual'
  checkinModalVisible.value = true
}

const handleCheckin = async () => {
  try {
    await checkin({
      plan_id: currentPlan.value.id,
      work_method: checkinForm.value.work_method
    })
    message.success('打卡成功')
    checkinModalVisible.value = false
    fetchPlans()
  } catch (e) {
    message.error(e.response?.data?.error || '打卡失败')
  }
}

onMounted(() => {
  fetchPlans()
  fetchWorkers()
})
</script>

<style scoped>
.card-title {
  display: flex;
  align-items: center;
}

.timeline-item {
  display: flex;
  align-items: flex-start;
}

.timeline-time {
  width: 160px;
  color: #666;
  font-size: 13px;
}

.timeline-content {
  flex: 1;
}

.timeline-worker {
  color: #666;
  font-size: 13px;
}

.timeline-method {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}
</style>
