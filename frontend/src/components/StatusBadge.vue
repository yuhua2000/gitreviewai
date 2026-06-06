<template>
  <n-tag :type="tagType" size="small" :bordered="false">
    {{ label }}
  </n-tag>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: {
    type: String,
    default: 'pending',
  },
})

const tagType = computed(() => {
  const map = {
    pending: 'warning',
    submitted: 'success',
    failed: 'error',
  }
  return map[props.status] || 'default'
})

const label = computed(() => {
  const map = {
    pending: '待提交',
    submitted: '已提交',
    failed: '失败',
  }
  return map[props.status] || props.status
})
</script>
