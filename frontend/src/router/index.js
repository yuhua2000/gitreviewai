import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/LoginView.vue'),
    meta: { guest: true },
  },
  {
    path: '/',
    component: () => import('../components/LayoutView.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'Home', redirect: '/mrs' },
      { path: 'mrs', name: 'MRList', component: () => import('../views/MRListView.vue') },
      { path: 'mrs/:id', name: 'MRDetail', component: () => import('../views/MRDetailView.vue') },
      { path: 'projects', name: 'Projects', component: () => import('../views/ProjectsView.vue') },
      { path: 'projects/:id', name: 'ProjectDetail', component: () => import('../views/ProjectDetailView.vue') },
      { path: 'settings', name: 'Settings', component: () => import('../views/SettingsView.vue') },
      { path: 'settings/general', name: 'SettingsGeneral', component: () => import('../views/SettingsGeneralView.vue') },
      { path: 'settings/models', name: 'SettingsModels', component: () => import('../views/SettingsModelsView.vue') },
      { path: 'settings/rules', name: 'SettingsRules', component: () => import('../views/SettingsRulesView.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  if (to.path === '/') {
    // 根路径：已登录去首页，未登录去登录页
    next(authStore.isAuthenticated ? '/mrs' : '/login')
  } else if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login')
  } else if (to.meta.guest && authStore.isAuthenticated) {
    next('/mrs')
  } else {
    next()
  }
})

export default router
