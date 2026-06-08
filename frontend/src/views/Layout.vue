<template>
  <a-layout style="min-height: 100vh">
    <a-layout-sider v-model:collapsed="collapsed" theme="dark" collapsible>
      <div class="logo">
        <h2 v-if="!collapsed">环卫管理平台</h2>
        <h2 v-else>环卫</h2>
      </div>
      <a-menu
        theme="dark"
        mode="inline"
        :selected-keys="selectedKeys"
        @click="handleMenuClick"
      >
        <a-menu-item key="dashboard">
          <span>📊</span>
          <span>仪表盘</span>
        </a-menu-item>
        <a-menu-item key="work-monitor">
          <span>👷</span>
          <span>作业监控</span>
        </a-menu-item>
        <a-menu-item key="quality-inspection">
          <span>🔍</span>
          <span>质量抽查</span>
        </a-menu-item>
        <a-menu-item key="assessment">
          <span>📈</span>
          <span>考核报表</span>
        </a-menu-item>
        <a-menu-item key="vehicle">
          <span>🚛</span>
          <span>车辆管理</span>
        </a-menu-item>
        <a-menu-item key="complaint">
          <span>📋</span>
          <span class="menu-text">投诉工单</span>
          <a-badge :count="pendingCount" :offset="[10, -2]" size="small" class="menu-badge" />
        </a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header class="header">
        <div class="header-left">
          <span>{{ pageTitle }}</span>
        </div>
        <div class="header-right">
          <a-dropdown>
            <span class="user-info">
              <span style="margin-right: 8px;">👤</span>
              {{ user?.real_name || user?.username }}
              <span class="role-badge">{{ roleText }}</span>
            </span>
            <template #overlay>
              <a-menu>
                <a-menu-item key="logout" @click="handleLogout">
                  退出登录
                </a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>
      </a-layout-header>
      <a-layout-content class="content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { getDashboard } from '../api'

const router = useRouter()
const route = useRoute()
const collapsed = ref(false)
const pendingCount = ref(0)

const user = ref(JSON.parse(localStorage.getItem('user') || '{}'))

const selectedKeys = computed(() => {
  const path = route.path.replace('/', '')
  return [path || 'dashboard']
})

const pageTitle = computed(() => {
  return route.meta?.title || '仪表盘'
})

const roleText = computed(() => {
  const roleMap = {
    admin: '管理员',
    area_manager: '片区长'
  }
  return roleMap[user.value.role] || user.value.role
})

const handleMenuClick = ({ key }) => {
  router.push('/' + key)
}

const handleLogout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  message.success('已退出登录')
  router.push('/login')
}

const fetchPendingCount = async () => {
  try {
    const res = await getDashboard()
    pendingCount.value = res.pending_complaints || 0
  } catch (e) {
    console.error(e)
  }
}

onMounted(() => {
  fetchPendingCount()
})

watch(() => route.path, () => {
  fetchPendingCount()
})
</script>

<style scoped>
.logo {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  background: rgba(255, 255, 255, 0.1);
}

.logo h2 {
  margin: 0;
  font-size: 18px;
  color: white;
}

.header {
  background: white;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.header-left {
  font-size: 18px;
  font-weight: 500;
  color: #333;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-info {
  cursor: pointer;
  display: flex;
  align-items: center;
  color: #333;
}

.role-badge {
  margin-left: 8px;
  padding: 2px 8px;
  background: #1890ff;
  color: white;
  border-radius: 4px;
  font-size: 12px;
}

.content {
  margin: 16px;
  padding: 24px;
  background: white;
  border-radius: 8px;
  min-height: calc(100vh - 112px);
}

:deep(.ant-menu) {
  border-right: none;
}

:deep(.ant-menu-item) {
  color: rgba(255, 255, 255, 0.85) !important;
}

:deep(.ant-menu-item-selected) {
  color: #fff !important;
}

.menu-badge {
  margin-left: 4px;
}
</style>
