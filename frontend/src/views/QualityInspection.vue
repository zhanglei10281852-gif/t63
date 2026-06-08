<template>
  <div class="quality-inspection">
    <a-row :gutter="16">
      <a-col :span="8">
        <a-card title="新增抽查记录">
          <a-form :model="form" layout="vertical" @finish="handleSubmit">
            <a-form-item label="抽查路段" name="road_section_id" :rules="[{ required: true, message: '请选择路段' }]">
              <a-select v-model:value="form.road_section_id" placeholder="请选择路段">
                <a-select-option v-for="r in roads" :key="r.id" :value="r.id">
                  {{ r.name }} ({{ r.area?.name }})
                </a-select-option>
              </a-select>
            </a-form-item>

            <a-form-item label="扣分项">
              <div class="deduction-list">
                <a-checkbox
                  v-for="item in deductionItems"
                  :key="item.name"
                  :checked="isDeductionSelected(item.name)"
                  @change="(e) => toggleDeduction(item, e.target.checked)"
                >
                  {{ item.name }} (扣{{ item.score }}分)
                </a-checkbox>
              </div>
            </a-form-item>

            <a-form-item label="当前得分">
              <span class="score-display" :class="scoreClass">{{ totalScore }}分</span>
            </a-form-item>

            <a-form-item label="备注">
              <a-textarea v-model:value="form.remark" :rows="3" placeholder="请输入备注" />
            </a-form-item>

            <a-form-item>
              <a-button type="primary" html-type="submit" block :loading="submitting">
                提交抽查记录
              </a-button>
            </a-form-item>
          </a-form>
        </a-card>
      </a-col>

      <a-col :span="16">
        <a-card title="历史抽查记录">
          <div class="filter-bar">
            <a-select v-model:value="filterRoad" placeholder="选择路段" allow-clear style="width: 200px; margin-right: 12px" @change="fetchInspections">
              <a-select-option v-for="r in roads" :key="r.id" :value="r.id">
                {{ r.name }}
              </a-select-option>
            </a-select>
            <a-date-picker v-model:value="filterMonth" picker="month" placeholder="选择月份" style="width: 200px" @change="fetchInspections" />
          </div>

          <a-table :columns="columns" :data-source="inspections" :loading="loading" row-key="id" style="margin-top: 16px" :pagination="{ pageSize: 10 }">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'score'">
                <span :class="getScoreClass(record.score)">{{ record.score }}分</span>
              </template>
              <template v-else-if="column.key === 'deductions'">
                <span v-if="record.deductions && record.deductions.length">
                  <a-tag v-for="d in parseDeductions(record.deductions)" :key="d.name" color="red">
                    {{ d.name }}(-{{ d.score }})
                  </a-tag>
                </span>
                <span v-else style="color: #999">无扣分</span>
              </template>
              <template v-else-if="column.key === 'inspect_time'">
                {{ formatTime(record.inspect_time) }}
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { getRoadSections, getInspections, createInspection } from '../api'

const roads = ref([])
const inspections = ref([])
const loading = ref(false)
const submitting = ref(false)
const filterRoad = ref(null)
const filterMonth = ref(null)

const deductionItems = [
  { name: '路面有明显垃圾', score: 5 },
  { name: '果皮箱满溢', score: 3 },
  { name: '绿化带有杂物', score: 3 },
  { name: '积水未清理', score: 4 },
  { name: '有小广告未清除', score: 2 }
]

const form = reactive({
  road_section_id: null,
  selectedDeductions: [],
  remark: ''
})

const totalScore = computed(() => {
  let score = 100
  form.selectedDeductions.forEach(d => {
    score -= d.score
  })
  return Math.max(0, score)
})

const scoreClass = computed(() => {
  if (totalScore.value >= 90) return 'score-high'
  if (totalScore.value >= 75) return 'score-medium'
  return 'score-low'
})

const columns = [
  { title: '路段名称', dataIndex: ['road_section', 'name'], key: 'road' },
  { title: '检查时间', key: 'inspect_time', width: 160 },
  { title: '检查人', dataIndex: ['inspector', 'real_name'], key: 'inspector', width: 100 },
  { title: '得分', key: 'score', width: 80 },
  { title: '扣分项', key: 'deductions' },
  { title: '备注', dataIndex: 'remark', key: 'remark', width: 150 }
]

const isDeductionSelected = (name) => {
  return form.selectedDeductions.some(d => d.name === name)
}

const toggleDeduction = (item, checked) => {
  if (checked) {
    form.selectedDeductions.push(item)
  } else {
    form.selectedDeductions = form.selectedDeductions.filter(d => d.name !== item.name)
  }
}

const parseDeductions = (deductionsStr) => {
  if (!deductionsStr) return []
  try {
    return typeof deductionsStr === 'string' ? JSON.parse(deductionsStr) : deductionsStr
  } catch (e) {
    return []
  }
}

const getScoreClass = (score) => {
  if (score >= 90) return 'score-high'
  if (score >= 75) return 'score-medium'
  return 'score-low'
}

const formatTime = (time) => {
  if (!time) return ''
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

const fetchRoads = async () => {
  try {
    const res = await getRoadSections()
    roads.value = res
  } catch (e) {
    console.error(e)
  }
}

const fetchInspections = async () => {
  loading.value = true
  try {
    const params = {}
    if (filterRoad.value) {
      params.road_section_id = filterRoad.value
    }
    if (filterMonth.value) {
      params.month = filterMonth.value.format('YYYY-MM')
    }
    const res = await getInspections(params)
    inspections.value = res
  } catch (e) {
    message.error('获取抽查记录失败')
  } finally {
    loading.value = false
  }
}

const handleSubmit = async () => {
  if (!form.road_section_id) {
    message.error('请选择路段')
    return
  }
  submitting.value = true
  try {
    await createInspection({
      road_section_id: form.road_section_id,
      deductions: form.selectedDeductions,
      remark: form.remark
    })
    message.success('抽查记录提交成功')
    form.road_section_id = null
    form.selectedDeductions = []
    form.remark = ''
    fetchInspections()
  } catch (e) {
    message.error(e.response?.data?.error || '提交失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  fetchRoads()
  fetchInspections()
})
</script>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
}

.deduction-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.score-display {
  font-size: 32px;
  font-weight: bold;
}

.score-high {
  color: #52c41a;
}

.score-medium {
  color: #1890ff;
}

.score-low {
  color: #ff4d4f;
}
</style>
