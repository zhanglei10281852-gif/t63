<template>
  <div class="assessment">
    <a-card>
      <template #title>
        <div class="card-title">
          <span>月度考核报表</span>
          <a-date-picker
            v-model:value="selectedMonth"
            picker="month"
            style="margin-left: 16px"
            placeholder="选择月份"
            @change="fetchData"
          />
        </div>
      </template>

      <a-tabs v-model:activeKey="activeTab" @change="handleTabChange">
        <a-tab-pane key="area" tab="片区考核" />
        <a-tab-pane key="worker" tab="工人考核" />
      </a-tabs>

      <a-table :columns="currentColumns" :data-source="assessmentList" :loading="loading" row-key="id" :pagination="{ pageSize: 10 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'rank'">
            <span :class="getRankClass(record._index)">
              {{ record._index + 1 }}
            </span>
          </template>
          <template v-else-if="column.key === 'total_score'">
            <span :class="getScoreClass(record.total_score)">{{ record.total_score?.toFixed(1) }}分</span>
          </template>
          <template v-else-if="column.key === 'grade'">
            <a-tag :color="getGradeColor(record.grade)">
              {{ getGradeText(record.grade) }}
            </a-tag>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card style="margin-top: 16px" title="雷达图对比">
      <a-select
        v-model:value="compareType"
        style="width: 200px; margin-bottom: 16px"
        @change="initRadarChart"
      >
        <a-select-option value="area">片区对比</a-select-option>
        <a-select-option value="worker">工人对比</a-select-option>
      </a-select>
      <div ref="radarChartRef" style="width: 100%; height: 400px"></div>
    </a-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import dayjs from 'dayjs'
import { message } from 'ant-design-vue'
import { getMonthlyAssessment } from '../api'
import * as echarts from 'echarts'

const selectedMonth = ref(dayjs())
const activeTab = ref('area')
const assessmentList = ref([])
const loading = ref(false)
const compareType = ref('area')
const radarChartRef = ref(null)
let radarChart = null

const areaColumns = [
  { title: '排名', key: 'rank', width: 80 },
  { title: '片区名称', dataIndex: 'target_name', key: 'name' },
  { title: '作业完成率', dataIndex: 'completion_rate', key: 'completion', width: 120, customRender: ({ text }) => text?.toFixed(1) + '%' },
  { title: '质量平均分', dataIndex: 'quality_score', key: 'quality', width: 120, customRender: ({ text }) => text?.toFixed(1) + '分' },
  { title: '投诉扣分', dataIndex: 'complaint_deduction', key: 'complaint', width: 100 },
  { title: '总分', key: 'total_score', width: 100 },
  { title: '等级', key: 'grade', width: 100 }
]

const workerColumns = [
  { title: '排名', key: 'rank', width: 80 },
  { title: '工人姓名', dataIndex: 'target_name', key: 'name' },
  { title: '作业完成率', dataIndex: 'completion_rate', key: 'completion', width: 120, customRender: ({ text }) => text?.toFixed(1) + '%' },
  { title: '质量平均分', dataIndex: 'quality_score', key: 'quality', width: 120, customRender: ({ text }) => text?.toFixed(1) + '分' },
  { title: '总分', key: 'total_score', width: 100 },
  { title: '等级', key: 'grade', width: 100 }
]

const currentColumns = computed(() => {
  return activeTab.value === 'area' ? areaColumns : workerColumns
})

const getRankClass = (index) => {
  if (index === 0) return 'rank-1'
  if (index === 1) return 'rank-2'
  if (index === 2) return 'rank-3'
  return ''
}

const getScoreClass = (score) => {
  if (score >= 90) return 'score-high'
  if (score >= 75) return 'score-medium'
  if (score >= 60) return 'score-pass'
  return 'score-fail'
}

const getGradeColor = (grade) => {
  const colors = {
    excellent: 'green',
    good: 'blue',
    pass: 'orange',
    fail: 'red'
  }
  return colors[grade] || 'default'
}

const getGradeText = (grade) => {
  const texts = {
    excellent: '优秀',
    good: '良好',
    pass: '合格',
    fail: '不合格'
  }
  return texts[grade] || grade
}

const fetchData = async () => {
  loading.value = true
  try {
    const month = selectedMonth.value ? selectedMonth.value.format('YYYY-MM') : ''
    const res = await getMonthlyAssessment({ month, target_type: activeTab.value })
    assessmentList.value = res.map((item, index) => ({
      ...item,
      _index: index
    }))
    nextTick(() => initRadarChart())
  } catch (e) {
    message.error('获取考核数据失败')
  } finally {
    loading.value = false
  }
}

const handleTabChange = () => {
  fetchData()
}

const initRadarChart = () => {
  if (!radarChartRef.value) return
  if (radarChart) radarChart.dispose()
  
  radarChart = echarts.init(radarChartRef.value)

  const data = assessmentList.value.slice(0, 5)
  
  if (data.length === 0) {
    radarChart.setOption({
      title: { text: '暂无数据', left: 'center', top: 'center' }
    })
    return
  }

  const indicators = compareType.value === 'area'
    ? [
        { name: '作业完成率', max: 100 },
        { name: '质量评分', max: 100 },
        { name: '投诉扣分(负向)', max: 100 },
        { name: '综合评分', max: 100 }
      ]
    : [
        { name: '作业完成率', max: 100 },
        { name: '质量评分', max: 100 },
        { name: '综合评分', max: 100 }
      ]

  const seriesData = data.map((item, index) => {
    const values = compareType.value === 'area'
      ? [
          item.completion_rate || 0,
          item.quality_score || 0,
          Math.max(0, 100 - (item.complaint_deduction || 0) * 10),
          item.total_score || 0
        ]
      : [
          item.completion_rate || 0,
          item.quality_score || 0,
          item.total_score || 0
        ]
    return {
      value: values,
      name: item.target_name
    }
  })

  const colors = ['#1890ff', '#52c41a', '#faad14', '#722ed1', '#eb2f96']

  const option = {
    tooltip: {},
    legend: {
      data: data.map(item => item.target_name),
      bottom: 0
    },
    radar: {
      indicator: indicators,
      radius: '60%',
      center: ['50%', '45%']
    },
    series: [{
      type: 'radar',
      data: seriesData.map((d, i) => ({
        ...d,
        itemStyle: { color: colors[i] },
        lineStyle: { color: colors[i] },
        areaStyle: { opacity: 0.2, color: colors[i] }
      }))
    }]
  }

  radarChart.setOption(option)
}

onMounted(() => {
  fetchData()
})

watch(compareType, () => {
  nextTick(() => initRadarChart())
})
</script>

<style scoped>
.card-title {
  display: flex;
  align-items: center;
}

.rank-1 {
  color: #faad14;
  font-weight: bold;
  font-size: 18px;
}

.rank-2 {
  color: #8c8c8c;
  font-weight: bold;
}

.rank-3 {
  color: #d48806;
  font-weight: bold;
}

.score-high {
  color: #52c41a;
  font-weight: bold;
}

.score-medium {
  color: #1890ff;
  font-weight: bold;
}

.score-pass {
  color: #faad14;
}

.score-fail {
  color: #ff4d4f;
  font-weight: bold;
}
</style>
