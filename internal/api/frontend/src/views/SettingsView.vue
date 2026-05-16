<template>
  <div class="settings">
    <n-card title="设置">
      <n-space vertical :size="24">
        <n-card title="自动提交" size="small">
          <n-space align="center" :size="16">
            <n-switch
              :value="settingsStore.autoSubmit"
              :loading="settingsStore.loading"
              @update:value="handleToggle"
            />
            <n-text>
              {{ settingsStore.autoSubmit ? '已开启：AI 生成的评论和报告将自动提交到 GitLab' : '已关闭：AI 生成的评论和报告需要手动审核后提交' }}
            </n-text>
          </n-space>
        </n-card>
      </n-space>
    </n-card>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useSettingsStore } from '../stores/settings'

const message = useMessage()
const settingsStore = useSettingsStore()

async function handleToggle(value) {
  try {
    await settingsStore.updateAutoSubmit(value)
    message.success(value ? '已开启自动提交' : '已关闭自动提交')
  } catch (e) {
    message.error('更新设置失败')
  }
}

onMounted(() => {
  settingsStore.fetchSettings()
})
</script>
