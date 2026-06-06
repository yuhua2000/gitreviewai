<template>
  <div class="settings-rules">
    <n-card title="审核规则管理">
      <template #header-extra>
        <n-button type="primary" @click="openAddModal">
          添加规则
        </n-button>
      </template>

      <n-spin :show="store.loading">
        <n-tabs type="line" v-model:value="activeTab">
          <n-tab-pane
            v-for="severity in severityLevels"
            :key="severity.value"
            :name="severity.value"
            :tab="`${severity.label} (${rulesBySeverity(severity.value).length})`"
          >
            <n-space vertical :size="12" style="padding-top: 12px;">
              <n-card
                v-for="rule in rulesBySeverity(severity.value)"
                :key="rule.id"
                size="small"
                :bordered="true"
              >
                <n-space align="center" justify="space-between" style="width: 100%;">
                  <n-space vertical :size="4" style="flex: 1;">
                    <n-space align="center" :size="8">
                      <n-text strong>{{ rule.name }}</n-text>
                      <n-tag v-if="rule.is_builtin" type="info" size="small">内置</n-tag>
                      <n-tag v-else type="warning" size="small">自定义</n-tag>
                    </n-space>
                    <n-text depth="3" v-if="rule.description">{{ rule.description }}</n-text>
                  </n-space>
                  <n-space align="center" :size="8">
                    <n-switch
                      :value="rule.enabled"
                      @update:value="(val) => handleToggle(rule, val)"
                    />
                    <template v-if="!rule.is_builtin">
                      <n-button size="small" @click="openEditModal(rule)">编辑</n-button>
                      <n-popconfirm @positive-click="handleDelete(rule)">
                        <template #trigger>
                          <n-button size="small" type="error">删除</n-button>
                        </template>
                        确认删除规则 "{{ rule.name }}"？
                      </n-popconfirm>
                    </template>
                  </n-space>
                </n-space>
              </n-card>

              <n-empty
                v-if="!rulesBySeverity(severity.value).length"
                :description="`暂无${severity.label}级别的规则`"
              />
            </n-space>
          </n-tab-pane>
        </n-tabs>
      </n-spin>
    </n-card>

    <n-modal
      v-model:show="modalVisible"
      preset="card"
      :title="editingRule ? '编辑规则' : '添加规则'"
      style="width: 520px;"
      :bordered="false"
      :segmented="{ content: true, footer: true }"
    >
      <n-form
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-placement="left"
        label-width="80"
      >
        <n-form-item label="名称" path="name">
          <n-input v-model:value="formData.name" placeholder="规则名称" />
        </n-form-item>
        <n-form-item label="描述" path="description">
          <n-input
            v-model:value="formData.description"
            type="textarea"
            placeholder="规则描述"
            :rows="3"
          />
        </n-form-item>
        <n-form-item label="严重级别" path="severity">
          <n-select
            v-model:value="formData.severity"
            :options="severityOptions"
            placeholder="选择严重级别"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="modalVisible = false">取消</n-button>
          <n-button type="primary" :loading="saving" @click="handleSave">
            保存
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useRulesStore } from '../stores/rules'

const message = useMessage()
const store = useRulesStore()

const activeTab = ref('error')
const modalVisible = ref(false)
const editingRule = ref(null)
const saving = ref(false)
const formRef = ref(null)

const formData = ref({
  name: '',
  description: '',
  severity: null,
})

const formRules = {
  name: { required: true, message: '请输入规则名称', trigger: 'blur' },
  severity: { required: true, message: '请选择严重级别', trigger: 'change' },
}

const severityLevels = [
  { label: 'ERROR', value: 'error' },
  { label: 'WARNING', value: 'warning' },
  { label: 'INFO', value: 'info' },
]

const severityOptions = [
  { label: 'ERROR - 错误', value: 'error' },
  { label: 'WARNING - 警告', value: 'warning' },
  { label: 'INFO - 信息', value: 'info' },
]

function rulesBySeverity(severity) {
  return store.rules.filter(r => r.severity === severity)
}

function openAddModal() {
  editingRule.value = null
  formData.value = { name: '', description: '', severity: null }
  modalVisible.value = true
}

function openEditModal(rule) {
  editingRule.value = rule
  formData.value = {
    name: rule.name || '',
    description: rule.description || '',
    severity: rule.severity || null,
  }
  modalVisible.value = true
}

async function handleSave() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    if (editingRule.value) {
      await store.editRule(editingRule.value.id, formData.value)
      message.success('规则已更新')
    } else {
      await store.addRule(formData.value)
      message.success('规则已添加')
    }
    modalVisible.value = false
  } catch (e) {
    message.error(e.message || '操作失败')
  } finally {
    saving.value = false
  }
}

async function handleToggle(rule, enabled) {
  try {
    await store.toggleRuleEnabled(rule.id, enabled)
    message.success(enabled ? '规则已启用' : '规则已禁用')
  } catch (e) {
    message.error(e.message || '操作失败')
  }
}

async function handleDelete(rule) {
  try {
    await store.removeRule(rule.id)
    message.success('规则已删除')
  } catch (e) {
    message.error(e.message || '删除失败')
  }
}

onMounted(() => {
  store.fetchRules()
})
</script>

<style scoped>
.settings-rules {
  padding: 0;
}
</style>
