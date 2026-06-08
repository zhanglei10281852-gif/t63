<template>
  <div class="complaint">
    <a-card>
      <template #title>
        <div class="card-title">
          <span>投诉工单</span>
          <a-badge :count="pendingCount" style="margin-left: 12px">
            <span style="font-size: 12px; color: #999">待处理</span>
          </a-badge>
        </div>
      </template>
      <template #extra>
        <a-button type="primary" @click="showCreateModal">
          新增投诉
        </a-button>
      </template>

      <a-tabs v-model:activeKey="activeTab" @change="fetchComplaints">
        <a-tab-pane key="all" tab="全部" />
        <a-tab-pane key="pending" tab="待处理" />
        <a-tab-pane key="processing" tab="处理中" />
        <a-tab-pane key="resolved" tab="已解决" />
        <a-tab-pane key="invalid" tab="无效投诉" />
      </a-tabs>

      <a-table :columns="columns" :data-source="complaints" :loading="loading" row-key="id" :pagination="{ pageSize: 10 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <a-tag :color="getStatusColor(record.status)">
              {{ getStatusText(record.status) }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'road'">
            {{ record.road_section?.name }}
          </template>
          <template v-else-if="column.key === 'assigned'">
            {{ record.assigned_user?.real_name || '未指派' }}
          </template>
          <template v-else-if="column.key === 'complaint_time'">
            {{ formatTime(record.complaint_time) }}
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button type="link" size="small" @click="viewDetail(record)">
                详情
              </a-button>
              <a-button
                type="link"
                size="small"
                @click="showHandleModal(record)"
                v-if="canHandle && (record.status === 'pending' || record.status === 'processing')"
              >
                处理
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal v-model:open="detailModalVisible" title="投诉详情" :width="600">
      <div v-if="currentComplaint" class="detail-content">
        <a-descriptions :column="1" bordered size="small">
          <a-descriptions-item label="投诉内容">
            {{ currentComplaint.content }}
          </a-descriptions-item>
          <a-descriptions-item label="投诉路段">
            {{ currentComplaint.road_section?.name }}
          </a-descriptions-item>
          <a-descriptions-item label="投诉时间">
            {{ formatTime(currentComplaint.complaint_time) }}
          </a-descriptions-item>
          <a-descriptions-item label="投诉人">
            {{ currentComplaint.complainant || '匿名' }}
          </a-descriptions-item>
          <a-descriptions-item label="联系电话">
            {{ currentComplaint.complainant_phone || '无' }}
          </a-descriptions-item>
          <a-descriptions-item label="状态">
            <a-tag :color="getStatusColor(currentComplaint.status)">
              {{ getStatusText(currentComplaint.status) }}
            </a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="指派处理人">
            {{ currentComplaint.assigned_user?.real_name || '未指派' }}
          </a-descriptions-item>
          <a-descriptions-item v-if="currentComplaint.handle_result" label="处理结果">
            {{ currentComplaint.handle_result }}
          </a-descriptions-item>
          <a-descriptions-item v-if="currentComplaint.handle_time" label="处理时间">
            {{ formatTime(currentComplaint.handle_time) }}
          </a-descriptions-item>
          <a-descriptions-item v-if="currentComplaint.is_valid !== null" label="是否有效">
            {{ currentComplaint.is_valid ? '有效投诉' : '无效投诉' }}
          </a-descriptions-item>
        </a-descriptions>
      </div>
    </a-modal>

    <a-modal v-model:open="createModalVisible" title="新增投诉" @ok="handleCreate">
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="投诉路段" name="road_section_id" :rules="[{ required: true, message: '请选择路段' }]">
          <a-select v-model:value="createForm.road_section_id" placeholder="请选择路段" style="width: 100%">
            <a-select-option v-for="r in roads" :key="r.id" :value="r.id">
              {{ r.name }} ({{ r.area?.name }})
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="投诉内容" name="content" :rules="[{ required: true, message: '请输入投诉内容' }]">
          <a-textarea v-model:value="createForm.content" :rows="4" placeholder="请描述投诉内容" />
        </a-form-item>
        <a-form-item label="投诉人">
          <a-input v-model:value="createForm.complainant" placeholder="请输入投诉人姓名" />
        </a-form-item>
        <a-form-item label="联系电话">
          <a-input v-model:value="createForm.complainant_phone" placeholder="请输入联系电话" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="handleModalVisible" title="处理投诉" @ok="handleSubmit">
      <a-form :model="handleForm" layout="vertical">
        <a-form-item label="处理结果" name="handle_result" :rules="[{ required: true, message: '请输入处理结果' }]">
          <a-textarea v-model:value="handleForm.handle_result" :rows="4" placeholder="请输入处理结果" />
        </a-form-item>
        <a-form-item label="是否有效投诉">
          <a-radio-group v-model:value="handleForm.is_valid">
            <a-radio :value="true">有效投诉</a-radio>
            <a-radio :value="false">无效投诉</a-radio>
          </a-radio-group>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import {
  getComplaints,
  createComplaint,
  handleComplaint,
  getRoadSections
} from '../api'

const complaints = ref([])
const loading = ref(false)
const activeTab = ref('all')
const pendingCount = ref(0)

const detailModalVisible = ref(false)
const createModalVisible = ref(false)
const handleModalVisible = ref(false)
const currentComplaint = ref(null)
const roads = ref([])

const user = JSON.parse(localStorage.getItem('user') || '{}')
const canHandle = computed(() => user.role === 'admin' || user.role === 'area_manager')

const createForm = reactive({
  road_section_id: null,
  content: '',
  complainant: '',
  complainant_phone: ''
})

const handleForm = reactive({
  handle_result: '',
  is_valid: true
})

const columns = [
  { title: '投诉内容', dataIndex: 'content', key: 'content', ellipsis: true },
  { title: '投诉路段', key: 'road', width: 120 },
  { title: '投诉时间', key: 'complaint_time', width: 160 },
  { title: '状态', key: 'status', width: 100 },
  { title: '处理人', key: 'assigned', width: 100 },
  { title: '操作', key: 'action', width: 150 }
]

const getStatusColor = (status) => {
  const colors = {
    pending: 'red',
    processing: 'orange',
    resolved: 'green',
    invalid: 'default'
  }
  return colors[status] || 'default'
}

const getStatusText = (status) => {
  const texts = {
    pending: '待处理',
    processing: '处理中',
    resolved: '已解决',
    invalid: '无效投诉'
  }
  return texts[status] || status
}

const formatTime = (time) => {
  if (!time) return ''
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

const fetchComplaints = async () => {
  loading.value = true
  try {
    const params = {}
    if (activeTab.value !== 'all') {
      params.status = activeTab.value
    }
    const res = await getComplaints(params)
    complaints.value = res
    
    const pending = await getComplaints({ status: 'pending' })
    pendingCount.value = pending.length || 0
  } catch (e) {
    message.error('获取投诉列表失败')
  } finally {
    loading.value = false
  }
}

const fetchRoads = async () => {
  try {
    const res = await getRoadSections()
    roads.value = res
  } catch (e) {
    console.error(e)
  }
}

const viewDetail = (record) => {
  currentComplaint.value = record
  detailModalVisible.value = true
}

const showCreateModal = () => {
  createForm.road_section_id = null
  createForm.content = ''
  createForm.complainant = ''
  createForm.complainant_phone = ''
  createModalVisible.value = true
}

const handleCreate = async () => {
  if (!createForm.road_section_id || !createForm.content) {
    message.error('请填写必填项')
    return
  }
  try {
    await createComplaint(createForm)
    message.success('投诉提交成功')
    createModalVisible.value = false
    fetchComplaints()
  } catch (e) {
    message.error(e.response?.data?.error || '提交失败')
  }
}

const showHandleModal = (record) => {
  currentComplaint.value = record
  handleForm.handle_result = ''
  handleForm.is_valid = true
  handleModalVisible.value = true
}

const handleSubmit = async () => {
  if (!handleForm.handle_result) {
    message.error('请输入处理结果')
    return
  }
  try {
    await handleComplaint(currentComplaint.value.id, handleForm)
    message.success('处理完成')
    handleModalVisible.value = false
    fetchComplaints()
  } catch (e) {
    message.error(e.response?.data?.error || '处理失败')
  }
}

onMounted(() => {
  fetchComplaints()
  fetchRoads()
})
</script>

<style scoped>
.card-title {
  display: flex;
  align-items: center;
}

.detail-content {
  padding: 10px 0;
}
</style>
