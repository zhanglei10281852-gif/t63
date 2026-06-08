<template>
  <div class="dashboard">
    <a-row :gutter="16">
      <a-col :span="6">
        <a-card class="stat-card">
          <div class="stat-title">今日作业完成率</div>
          <div class="stat-value">{{ stats.today_completion_rate?.toFixed(1) }}%</div>
          <div class="stat-sub">
            已完成 {{ stats.today_completed_tasks }} / {{ stats.today_total_tasks }} 个任务
          </div>
          <a-progress
            type="circle"
            :percent="Math.round(stats.today_completion_rate || 0)"
            :size="120"
            :stroke-color="progressColor"
          />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card class="stat-card">
          <div class="stat-title">本月抽查平均分</div>
          <div class="stat-value">{{ stats.monthly_avg_score?.toFixed(1) }}</div>
          <div class="stat-sub">{{ stats.current_month }}</div>
          <div class="score-bar">
            <div class="score-fill" :style="{ width: (stats.monthly_avg_score || 0) + '%' }"></div>
          </div>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card class="stat-card">
          <div class="stat-title">未处理投诉</div>
          <div class="stat-value warning">{{ stats.pending_complaints }}</div>
          <div class="stat-sub">待处理投诉工单</div>
          <a-button type="link" @click="goToComplaint">去处理 →</a-button>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card class="stat-card">
          <div class="stat-title">管辖片区</div>
          <div class="stat-value">{{ stats.area_stats?.length || 0 }}</div>
          <div class="stat-sub">个作业片区</div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16" style="margin-top: 16px">
      <a-col :span="12">
        <a-card title="各片区今日完成率">
          <div v-for="area in stats.area_stats" :key="area.area_id" class="area-item">
            <div class="area-name">{{ area.area_name }}</div>
            <div class="area-progress">
              <a-progress :percent="Math.round(area.completion_rate)" :show-info="false" />
            </div>
            <div class="area-rate">{{ area.completion_rate.toFixed(1) }}%</div>
          </div>
        </a-card>
      </a-col>
      <a-col :span="12">
        <a-card title="本月抽查平均分趋势">
          <div ref="chartRef" style="width: 100%; height: 300px"></div>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { getDashboard } from '../api'
import * as echarts from 'echarts'

const router = useRouter()
const stats = ref({})
const chartRef = ref(null)
let chart = null

const progressColor = computed(() => {
  const rate = stats.value.today_completion_rate || 0
  if (rate >= 90) return '#52c41a'
  if (rate >= 75) return '#1890ff'
  if (rate >= 60) return '#faad14'
  return '#ff4d4f'
})

const fetchStats = async () => {
  try {
    const res = await getDashboard()
    stats.value = res
    nextTick(() => {
      initChart()
    })
  } catch (e) {
    console.error(e)
  }
}

const initChart = () => {
  if (!chartRef.value) return
  if (chart) chart.dispose()
  
  chart = echarts.init(chartRef.value)
  
  const trend = stats.value.monthly_quality_trend || []
  const dates = trend.map(item => item.date?.slice(5) || '')
  const scores = trend.map(item => item.score || 0)

  const option = {
    tooltip: {
      trigger: 'axis'
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: {
        fontSize: 10,
        rotate: 45
      }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 100
    },
    series: [
      {
        data: scores,
        type: 'line',
        smooth: true,
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(24, 144, 255, 0.3)' },
            { offset: 1, color: 'rgba(24, 144, 255, 0.05)' }
          ])
        },
        lineStyle: {
          color: '#1890ff',
          width: 2
        },
        itemStyle: {
          color: '#1890ff'
        }
      }
    ]
  }

  chart.setOption(option)
}

const goToComplaint = () => {
  router.push('/complaint')
}

onMounted(() => {
  fetchStats()
  const timer = setInterval(fetchStats, 60000)
  return () => clearInterval(timer)
})

watch(() => stats.value.monthly_quality_trend, () => {
  nextTick(() => initChart())
}, { deep: true })
</script>

<style scoped>
.stat-card {
  text-align: center;
  min-height: 180px;
}

.stat-title {
  font-size: 14px;
  color: #666;
  margin-bottom: 12px;
}

.stat-value {
  font-size: 36px;
  font-weight: bold;
  color: #1890ff;
  margin-bottom: 8px;
}

.stat-value.warning {
  color: #faad14;
}

.stat-sub {
  font-size: 12px;
  color: #999;
  margin-bottom: 12px;
}

.score-bar {
  width: 80%;
  height: 8px;
  background: #f0f0f0;
  border-radius: 4px;
  margin: 16px auto 0;
  overflow: hidden;
}

.score-fill {
  height: 100%;
  background: linear-gradient(90deg, #52c41a, #1890ff);
  border-radius: 4px;
  transition: width 0.3s;
}

.area-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}

.area-item:last-child {
  border-bottom: none;
}

.area-name {
  width: 100px;
  font-size: 14px;
  color: #333;
}

.area-progress {
  flex: 1;
  margin: 0 16px;
}

.area-rate {
  width: 60px;
  text-align: right;
  font-size: 14px;
  color: #1890ff;
  font-weight: 500;
}
</style>
