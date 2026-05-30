<template>
  <div class="login-container">
    <n-card title="GitReviewAI" class="login-card">
      <n-space vertical :size="24">
        <n-text depth="3" style="text-align: center; display: block;">
          AI 驱动的 GitLab 代码审查助手
        </n-text>
        <n-input
          v-model:value="password"
          type="password"
          show-password-on="click"
          placeholder="请输入登录密码"
          size="large"
          @keyup.enter="handleLogin"
        />
        <n-button
          type="primary"
          block
          size="large"
          :loading="loading"
          @click="handleLogin"
        >
          登录
        </n-button>
        <n-text v-if="error" type="error" style="text-align: center; display: block;">
          {{ error }}
        </n-text>
      </n-space>
    </n-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()

const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  if (!password.value) {
    error.value = '请输入密码'
    return
  }

  loading.value = true
  error.value = ''

  try {
    await authStore.login(password.value)
    message.success('登录成功')
    router.push('/mrs')
  } catch (e) {
    error.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  width: 400px;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}
</style>
