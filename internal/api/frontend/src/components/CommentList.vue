<template>
  <n-space vertical :size="12">
    <CommentItem
      v-for="comment in comments"
      :key="comment.id"
      :comment="comment"
      @submit="$emit('submit', comment.id)"
    />
    <n-empty v-if="!comments.length" description="暂无评论" />
  </n-space>
</template>

<script setup>
import CommentItem from './CommentItem.vue'

defineProps({
  comments: {
    type: Array,
    default: () => [],
  },
})

defineEmits(['submit'])
</script>
