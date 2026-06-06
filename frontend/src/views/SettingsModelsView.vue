<template>
  <div class="settings-models">
    <n-card title="AI 模型管理">
      <template #header-extra>
        <n-button type="primary" @click="openAddModal">
          添加模型
        </n-button>
      </template>

      <n-data-table
        :columns="columns"
        :data="store.models"
        :loading="store.loading"
        :bordered="false"
      />
    </n-card>

    <n-modal
      v-model:show="modalVisible"
      preset="card"
      :title="editingModel ? '编辑模型' : '添加模型'"
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
          <n-input v-model:value="formData.name" placeholder="例如: GPT-4o" />
        </n-form-item>
        <n-form-item label="Base URL" path="base_url">
          <n-input v-model:value="formData.base_url" placeholder="https://api.openai.com/v1" />
        </n-form-item>
        <n-form-item label="API Key" path="api_key">
          <n-input
            v-model:value="formData.api_key"
            type="password"
            show-password-on="click"
            placeholder="sk-..."
          />
        </n-form-item>
        <n-form-item label="模型名称" path="model_name">
          <n-input v-model:value="formData.model_name" placeholder="gpt-4o" />
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
import { ref, h, onMounted } from 'vue'
import { NTag, NButton, NSpace, NPopconfirm } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { useAIModelsStore } from '../stores/aiModels'

const message = useMessage()
const store = useAIModelsStore()

const modalVisible = ref(false)
const editingModel = ref(null)
const saving = ref(false)
const formRef = ref(null)

const formData = ref({
  name: '',
  base_url: '',
  api_key: '',
  model_name: '',
})

const formRules = {
  name: { required: true, message: '请输入名称', trigger: 'blur' },
  base_url: { required: true, message: '请输入 Base URL', trigger: 'blur' },
  model_name: { required: true, message: '请输入模型名称', trigger: 'blur' },
}

const columns = [
  {
    title: '名称',
    key: 'name',
    width: 160,
  },
  {
    title: 'Base URL',
    key: 'base_url',
    ellipsis: { tooltip: true },
    width: 240,
  },
  {
    title: '模型名称',
    key: 'model_name',
    width: 150,
  },
  {
    title: '默认',
    key: 'is_default',
    width: 80,
    render(row) {
      if (row.is_default) {
        return h(NTag, { type: 'success', size: 'small' }, { default: () => '默认' })
      }
      return null
    },
  },
  {
    title: '启用',
    key: 'enabled',
    width: 80,
    render(row) {
      return h(
        NTag,
        { type: row.enabled !== false ? 'success' : 'default', size: 'small' },
        { default: () => (row.enabled !== false ? '启用' : '禁用') }
      )
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 240,
    render(row) {
      return h(NSpace, { size: 'small' }, {
        default: () => [
          h(
            NButton,
            { size: 'small', onClick: () => openEditModal(row) },
            { default: () => '编辑' }
          ),
          h(
            NButton,
            {
              size: 'small',
              type: row.is_default ? 'success' : 'default',
              disabled: row.is_default,
              onClick: () => handleSetDefault(row),
            },
            { default: () => '设为默认' }
          ),
          h(
            NPopconfirm,
            { onPositiveClick: () => handleDelete(row) },
            {
              trigger: () => h(
                NButton,
                { size: 'small', type: 'error' },
                { default: () => '删除' }
              ),
              default: () => '确认删除此模型？',
            }
          ),
        ],
      })
    },
  },
]

function openAddModal() {
  editingModel.value = null
  formData.value = { name: '', base_url: '', api_key: '', model_name: '' }
  modalVisible.value = true
}

function openEditModal(model) {
  editingModel.value = model
  formData.value = {
    name: model.name || '',
    base_url: model.base_url || '',
    api_key: '',
    model_name: model.model_name || '',
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
    const payload = { ...formData.value }
    // Don't send empty api_key on edit (preserves existing)
    if (editingModel.value && !payload.api_key) {
      delete payload.api_key
    }

    if (editingModel.value) {
      await store.editModel(editingModel.value.id, payload)
      message.success('模型已更新')
    } else {
      await store.addModel(payload)
      message.success('模型已添加')
    }
    modalVisible.value = false
  } catch (e) {
    message.error(e.message || '操作失败')
  } finally {
    saving.value = false
  }
}

async function handleSetDefault(model) {
  try {
    await store.makeDefault(model.id)
    message.success(`已将 ${model.name} 设为默认模型`)
  } catch (e) {
    message.error(e.message || '操作失败')
  }
}

async function handleDelete(model) {
  try {
    await store.removeModel(model.id)
    message.success('模型已删除')
  } catch (e) {
    message.error(e.message || '删除失败')
  }
}

onMounted(() => {
  store.fetchModels()
})
</script>

<style scoped>
.settings-models {
  padding: 0;
}
</style>
