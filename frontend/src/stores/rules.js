import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listRules, createRule, updateRule, deleteRule, toggleRule } from '../api/rules'

export const useRulesStore = defineStore('rules', () => {
  const rules = ref([])
  const loading = ref(false)

  async function fetchRules() {
    loading.value = true
    try {
      rules.value = await listRules() || []
    } finally {
      loading.value = false
    }
  }

  async function addRule(data) {
    const rule = await createRule(data)
    rules.value.push(rule)
    return rule
  }

  async function editRule(id, data) {
    const rule = await updateRule(id, data)
    const idx = rules.value.findIndex(r => r.id === id)
    if (idx !== -1) rules.value[idx] = rule
    return rule
  }

  async function removeRule(id) {
    await deleteRule(id)
    rules.value = rules.value.filter(r => r.id !== id)
  }

  async function toggleRuleEnabled(id, enabled) {
    await toggleRule(id, enabled)
    const rule = rules.value.find(r => r.id === id)
    if (rule) rule.enabled = enabled
  }

  return { rules, loading, fetchRules, addRule, editRule, removeRule, toggleRuleEnabled }
})
