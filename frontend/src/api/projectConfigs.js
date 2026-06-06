import client from './client'

export function listProjectConfigs(page = 1, pageSize = 20) {
  return client.get('/project-configs', { params: { page, page_size: pageSize } })
}

export function getProjectConfig(projectId) {
  return client.get(`/project-configs/${projectId}`)
}

export function updateProjectConfig(projectId, data) {
  return client.put(`/project-configs/${projectId}`, data)
}

export function updateProjectRules(projectId, rules) {
  return client.put(`/project-configs/${projectId}/rules`, { rules })
}
