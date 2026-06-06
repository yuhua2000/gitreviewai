import client from './client'

export function listMRs(page = 1, pageSize = 20) {
  return client.get('/mrs', { params: { page, page_size: pageSize } })
}

export function getMR(id) {
  return client.get(`/mrs/${id}`)
}

export function submitComment(id) {
  return client.post(`/comments/${id}/submit`)
}

export function submitReport(id) {
  return client.post(`/reports/${id}/submit`)
}

export function submitAll(mrId) {
  return client.post(`/mrs/${mrId}/submit-all`)
}

export function getMRChanges(id, page = 1, pageSize = 20) {
  return client.get(`/mrs/${id}/changes`, { params: { page, page_size: pageSize } })
}

export function getReviewLogs(id) {
  return client.get(`/mrs/${id}/review-logs`)
}

export function retryReview(id) {
  return client.post(`/mrs/${id}/retry`)
}
