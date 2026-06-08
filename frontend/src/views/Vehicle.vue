<template>
  <div class="vehicle">
    <a-row :gutter="16">
      <a-col :span="16">
        <a-card title="车辆列表">
          <template #extra>
            <a-space>
              <a-button type="primary" @click="showStartModal" v-if="isAdmin">出车登记</a-button>
              <a-button @click="fetchVehicles">
                <template #icon><ReloadOutlined /></template>
                刷新
              </a-button>
            </a-space>
          </template>

          <a-table :columns="columns" :data-source="vehicles" :loading="loading" row-key="id" :pagination="{ pageSize: 10 }">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'type'">
              <span>{{ getVehicleTypeText(record.type) }}</span>
              </template>
              <template v-else-if="column.key === 'status'">
                <a-tag :color="getStatusColor(record.status)">
                  {{ getStatusText(record.status) }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'action'">
                <a-space>
                  <a-button type="link" size="small" @click="viewRecords(record)" v-if="isAdmin">
                    出车记录
                  </a-button>
                  <a-button type="link" size="small" @click="showReturnModal(record)" v-if="record.status === 'on_duty' && isAdmin">
                    收车
                  </a-button>
                  <a-dropdown v-if="isAdmin">
                    <a-button type="link" size="small">状态管理</a-button>
                    <template #overlay>
                      <a-menu>
                        <a-menu-item key="available" @click="updateStatus(record, 'available')">
                          设为可用
                        </a-menu-item>
                        <a-menu-item key="maintenance" @click="updateStatus(record, 'maintenance')">
                          维修中
                        </a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>

      <a-col :span="8">
        <a-card title="保养提醒" :bordered="false">
          <a-alert
            v-for="reminder in maintenanceReminders"
            :key="reminder.vehicle_id"
            type="warning"
            show-icon
            style="margin-bottom: 12px"
          >
            <template #message>
              <strong>{{ reminder.plate_number }}</strong>
            </template>
            <template #description>
              {{ getVehicleTypeText(reminder.type) }} - {{ reminder.reason }}
            </template>
          </a-alert>
          <div v-if="maintenanceReminders.length === 0" style="text-align: center; color: #999; padding: 20px">
            暂无保养提醒
          </div>
        </a-card>
      </a-col>
    </a-row>

    <a-modal v-model:open="recordModalVisible" title="出车记录" :width="800">
      <a-table :columns="recordColumns" :data-source="vehicleRecords" :loading="recordLoading" row-key="id" size="small" :pagination="{ pageSize: 5 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'depart_time'">
            {{ formatTime(record.depart_time) }}
          </template>
          <template v-else-if="column.key === 'return_time'">
            {{ formatTime(record.return_time) || '进行中' }}
          </template>
        </template>
      </a-table>
    </a-modal>

    <a-modal v-model:open="startModalVisible" title="出车登记" @ok="handleStart">
      <a-form :model="startForm" layout="vertical">
        <a-form-item label="选择车辆" name="vehicle_id" :rules="[{ required: true, message: '请选择车辆' }]">
          <a-select v-model:value="startForm.vehicle_id" placeholder="请选择可用车辆" style="width: 100%">
            <a-select-option v-for="v in availableVehicles" :key="v.id" :value="v.id">
              {{ v.plate_number }} ({{ getVehicleTypeText(v.type) }})
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="司机">
          <a-select v-model:value="startForm.driver_id" placeholder="请选择司机" allow-clear style="width: 100%">
            <a-select-option v-for="w in workers" :key="w.id" :value="w.id">
              {{ w.name }}
            </a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="returnModalVisible" title="收车登记" @ok="handleReturn">
      <a-form :model="returnForm" layout="vertical">
        <a-form-item label="车辆">
          <span>{{ currentVehicle?.plate_number }} ({{ getVehicleTypeText(currentVehicle?.type) }})</span>
        </a-form-item>
        <a-form-item label="行驶里程(公里)">
          <a-input-number v-model:value="returnForm.mileage" :min="0" :step="1" style="width: 100%" placeholder="请输入行驶里程" />
        </a-form-item>
        <a-form-item label="油耗(升)">
          <a-input-number v-model:value="returnForm.fuel_consumption" :min="0" :step="0.1" style="width: 100%" placeholder="请输入油耗" />
        </a-form-item>
        <a-form-item label="作业路段">
          <a-select v-model:value="returnForm.road_section_ids" mode="multiple" placeholder="选择作业路段" style="width: 100%">
            <a-select-option v-for="r in roads" :key="r.id" :value="r.id">
              {{ r.name }}
            </a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { ReloadOutlined } from '@ant-design/icons-vue'
import {
  getVehicles,
  getVehicleRecords,
  getMaintenanceReminders,
  startVehicle,
  returnVehicle,
  updateVehicleStatus,
  getWorkers,
  getRoadSections
} from '../api'

const vehicles = ref([])
const loading = ref(false)
const maintenanceReminders = ref([])
const vehicleRecords = ref([])
const recordLoading = ref(false)
const recordModalVisible = ref(false)
const startModalVisible = ref(false)
const returnModalVisible = ref(false)
const currentVehicle = ref(null)
const currentRecord = ref(null)
const workers = ref([])
const roads = ref([])

const user = JSON.parse(localStorage.getItem('user') || '{}')
const isAdmin = computed(() => user.role === 'admin')

const startForm = reactive({
  vehicle_id: null,
  driver_id: null
})

const returnForm = reactive({
  mileage: 0,
  fuel_consumption: 0,
  road_section_ids: []
})

const columns = [
  { title: '车牌号', dataIndex: 'plate_number', key: 'plate_number', width: 120 },
  { title: '车辆类型', key: 'type', width: 100 },
  { title: '状态', key: 'status', width: 100 },
  { title: '当前里程', dataIndex: 'mileage', key: 'mileage', width: 100, customRender: ({ text }) => text + ' km' },
  { title: '上次保养里程', dataIndex: 'last_maintenance_mileage', key: 'last_maint', width: 120, customRender: ({ text }) => text + ' km' },
  { title: '操作', key: 'action', width: 180 }
]

const recordColumns = [
  { title: '车牌号', dataIndex: ['vehicle', 'plate_number'], key: 'plate', width: 100 },
  { title: '司机', dataIndex: ['driver', 'name'], key: 'driver', width: 80 },
  { title: '出车时间', key: 'depart_time', width: 150 },
  { title: '收车时间', key: 'return_time', width: 150 },
  { title: '里程(km)', dataIndex: 'mileage', key: 'mileage', width: 80 },
  { title: '油耗(L)', dataIndex: 'fuel_consumption', key: 'fuel', width: 80 }
]

const availableVehicles = computed(() => {
  return vehicles.value.filter(v => v.status === 'available')
})

const getVehicleTypeText = (type) => {
  const texts = {
    sprinkler: '洒水车',
    sweeper: '清扫车',
    garbage_truck: '垃圾转运车'
  }
  return texts[type] || type
}

const getStatusColor = (status) => {
  const colors = {
    available: 'green',
    on_duty: 'blue',
    maintenance: 'orange'
  }
  return colors[status] || 'default'
}

const getStatusText = (status) => {
  const texts = {
    available: '可用',
    on_duty: '出车中',
    maintenance: '维修中'
  }
  return texts[status] || status
}

const formatTime = (time) => {
  if (!time) return ''
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

const fetchVehicles = async () => {
  loading.value = true
  try {
    const res = await getVehicles()
    vehicles.value = res
    fetchMaintenanceReminders()
  } catch (e) {
    message.error('获取车辆列表失败')
  } finally {
    loading.value = false
  }
}

const fetchMaintenanceReminders = async () => {
  try {
    const res = await getMaintenanceReminders()
    maintenanceReminders.value = res
  } catch (e) {
    console.error(e)
  }
}

const viewRecords = async (vehicle) => {
  currentVehicle.value = vehicle
  recordLoading.value = true
  recordModalVisible.value = true
  try {
    const res = await getVehicleRecords({ vehicle_id: vehicle.id })
    vehicleRecords.value = res
  } catch (e) {
    message.error('获取出车记录失败')
  } finally {
    recordLoading.value = false
  }
}

const showStartModal = () => {
  startForm.vehicle_id = null
  startForm.driver_id = null
  startModalVisible.value = true
}

const handleStart = async () => {
  if (!startForm.vehicle_id) {
    message.error('请选择车辆')
    return
  }
  try {
    await startVehicle({
      vehicle_id: startForm.vehicle_id,
      driver_id: startForm.driver_id
    })
    message.success('出车登记成功')
    startModalVisible.value = false
    fetchVehicles()
  } catch (e) {
    message.error(e.response?.data?.error || '出车登记失败')
  }
}

const showReturnModal = (vehicle) => {
  currentVehicle.value = vehicle
  returnForm.mileage = 0
  returnForm.fuel_consumption = 0
  returnForm.road_section_ids = []
  
  getVehicleRecords({ vehicle_id: vehicle.id }).then(res => {
    const ongoing = res.find(r => !r.return_time)
    if (ongoing) {
      currentRecord.value = ongoing
    }
  })
  
  returnModalVisible.value = true
}

const handleReturn = async () => {
  if (!currentRecord.value) {
    message.error('未找到进行中的出车记录')
    return
  }
  try {
    await returnVehicle({
      record_id: currentRecord.value.id,
      mileage: returnForm.mileage,
      fuel_consumption: returnForm.fuel_consumption,
      road_section_ids: returnForm.road_section_ids.join(',')
    })
    message.success('收车登记成功')
    returnModalVisible.value = false
    fetchVehicles()
  } catch (e) {
    message.error(e.response?.data?.error || '收车登记失败')
  }
}

const updateStatus = async (vehicle, status) => {
  try {
    await updateVehicleStatus(vehicle.id, { status })
    message.success('状态更新成功')
    fetchVehicles()
  } catch (e) {
    message.error(e.response?.data?.error || '状态更新失败')
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

const fetchRoads = async () => {
  try {
    const res = await getRoadSections()
    roads.value = res
  } catch (e) {
    console.error(e)
  }
}

onMounted(() => {
  fetchVehicles()
  fetchWorkers()
  fetchRoads()
})
</script>

<style scoped>
</style>
