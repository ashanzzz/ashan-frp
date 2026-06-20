/**
 * 前端 API 层 - 对后端的 fetch 封装。
 * 当前为预留接口，返回模拟响应用于框架验证。
 * 等后端就绪后实现真实请求。
 */

const BASE_URL = '/api'

async function request<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
    },
    ...options,
  })
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`)
  }
  return res.json()
}

// --- SSE ---
export function sseUrl(channel: string): string {
  return `${BASE_URL}/sse/${channel}`
}

// --- Nodes ---
export interface NodeSummary {
  id: string
  name: string
  status: string
}

export async function listNodes(): Promise<NodeSummary[]> {
  return request('/nodes')
}

// --- Tunnels ---
export interface TunnelSummary {
  id: string
  name: string
  status: string
}

export async function listTunnels(): Promise<TunnelSummary[]> {
  return request('/tunnels')
}

// --- Website Mappings ---
export interface WebsiteMappingSummary {
  id: string
  domain: string
  status: string
}

export async function listWebsiteMappings(): Promise<WebsiteMappingSummary[]> {
  return request('/website-mappings')
}

// --- Jobs ---
export interface JobSummary {
  id: string
  type: string
  status: string
}

export async function listJobs(): Promise<JobSummary[]> {
  return request('/jobs')
}

// --- Settings ---
export async function getSettings(): Promise<Record<string, unknown>> {
  return request('/settings')
}