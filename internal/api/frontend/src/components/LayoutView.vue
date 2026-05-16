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
  if (route.path === '/settings') return '/settings'
  return '/mrs'
})

const menuOptions = [
  {
    label: '合并请求',
    key: '/mrs',
  },
  {
    label: '设置',
    key: '/settings',
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
