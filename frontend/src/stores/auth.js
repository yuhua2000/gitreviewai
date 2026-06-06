import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi } from '../api/auth'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')

  const isAuthenticated = computed(() => !!token.value)

  async function login(password) {
    const data = await loginApi(password)
    token.value = data.token
    localStorage.setItem('token', data.token)
    return data
  }

  function logout() {
    token.value = ''
    localStorage.removeItem('token')
  }

  return {
    token,
    isAuthenticated,
    login,
    logout,
  }
})
