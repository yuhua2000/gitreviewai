<template>
  <n-layout has-sider style="min-height: 100vh;">
    <n-layout-sider
      bordered
      :width="220"
      :native-scrollbar="false"
      content-style="padding: 24px 16px;"
    >
      <n-space vertical :size="24">
        <n-text strong style="font-size: 20px; color: #18a058;">
          GitReviewAI
        </n-text>
        <n-menu
          :value="activeMenu"
          :options="menuOptions"
          @update:value="handleMenuChange"
        />
      </n-space>
    </n-layout-sider>
    <n-layout-content content-style="padding: 24px;" :native-scrollbar="false">
      <n-space justify="end" style="margin-bottom: 16px;">
        <n-button quaternary size="small" @click="handleLogout">
          退出登录
        </n-button>
      </n-space>
      <router-view />
    </n-layout-content>
  </n-layout>
</template>

<script setup>
import { computed, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const activeMenu = computed(() => {
  if (route.path.startsWith('/mrs')) return '/mrs'
  if (route.path.startsWith('/projects')) return '/projects'
  if (route.path.startsWith('/settings/models')) return '/settings/models'
  if (route.path.startsWith('/settings/rules')) return '/settings/rules'
  if (route.path.startsWith('/settings/general')) return '/settings/general'
  if (route.path === '/settings') return '/settings'
  return '/mrs'
})

const menuOptions = [
  {
    label: '合并请求',
    key: '/mrs',
  },
  {
    label: '项目管理',
    key: '/projects',
  },
  {
    label: '全局设置',
    key: '/settings',
    children: [
      {
        label: '基本设置',
        key: '/settings/general',
      },
      {
        label: 'AI 模型',
        key: '/settings/models',
      },
      {
        label: '审核规则',
        key: '/settings/rules',
      },
    ],
  },
]

function handleMenuChange(key) {
  router.push(key)
}

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>
