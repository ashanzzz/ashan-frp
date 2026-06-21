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

// ===================== Types =====================

export type NodeStatus = 'online' | 'degraded' | 'offline' | 'banned' | 'archived'

export interface NodeSummary {
  id: string
  name: string
  displayName?: string
  status: NodeStatus
  provider: string
  endpoint?: string
  region?: string
  lastHeartbeatAt?: string | null
  lastSuccessAt?: string | null
  lastFailureReason?: string | null
  dependentCount: number
  createdAt?: string
}

export type TunnelStatus = 'enabled' | 'pending' | 'conflict' | 'overridden' | 'failed'

export interface TunnelSummary {
  id: string
  name: string
  status: TunnelStatus
  nodeId: string
  nodeName?: string
  localAddress: string
  localPort: number
  remotePort?: number
  domain?: string
  expectedStatus: string
  observedStatus: string
  lastAppliedAt?: string | null
  hasDrift: boolean
  isOverridden: boolean
  createdAt?: string
}

export type WebsiteStatus = 'synced' | 'pending' | 'https_error' | 'overridden'

export interface WebsiteMappingSummary {
  id: string
  primaryDomain: string
  aliasDomains: string[]
  status: WebsiteStatus
  nodeId: string
  nodeName?: string
  tunnelId: string
  tunnelName?: string
  httpsEnabled: boolean
  proxyEnabled: boolean
  panelWebsiteId?: string
  lastSyncedAt?: string | null
  isOverridden: boolean
  overrideSource?: string | null
  overrideLastModifiedBy?: string | null
  createdAt?: string
}

// Job status aligned with backend schema: queued/running/retry_wait/blocked/succeeded/failed/canceled
export type JobStatus = 'queued' | 'running' | 'retry_wait' | 'blocked' | 'stalled' | 'succeeded' | 'failed' | 'canceled'

export interface JobSummary {
  id: string
  type: string
  status: JobStatus
  targetType?: string
  targetId?: string
  targetName?: string
  priority: number
  scheduledAt?: string
  startedAt?: string | null
  completedAt?: string | null
  attempts: number
  maxAttempts: number
  runAfter?: string | null
  lockedBy?: string | null
  lockedAt?: string | null
  result?: string | null
  errorCode?: string | null
  errorMessage?: string | null
  idempotencyKey?: string
  createdAt?: string
}

export interface JobEventSummary {
  id: string
  jobId: string
  eventType: string
  sequenceNo: number
  message: string
  level: 'debug' | 'info' | 'warn' | 'error'
  payload?: Record<string, unknown> | null
  traceId?: string | null
  createdAt: string
}

// Log types for LogsPage
export type LogCategory = 'job_events' | 'audit_log' | 'sync_state' | 'snapshots'

export interface LogEntry {
  id: string
  timestamp: string
  source: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  category: LogCategory
  targetType?: string | null
  targetId?: string | null
  payload?: Record<string, unknown> | null
}

export interface AuditLogSummary {
  id: string
  actorType: string
  actorId: string
  action: string
  resourceType: string
  resourceId: string
  oldValue?: Record<string, unknown> | null
  newValue?: Record<string, unknown> | null
  ipAddress?: string | null
  userAgent?: string | null
  createdAt: string
}

export interface SyncTrailSummary {
  id: string
  subjectType: string
  subjectId: string
  status: string
  desiredHash: string
  observedHash: string
  diffSummary?: string | null
  appliedSnapshotId?: string | null
  nextRetryAt?: string | null
  createdAt: string
}

// Settings
export type FixedPreset =
  | 'health_check_interval'
  | 'queue_poll_interval'
  | 'retry_backoff'
  | 'log_default_rows'
  | 'data_retention'

export interface SettingPresets {
  healthCheckInterval: number // seconds
  queuePollInterval: number // seconds
  retryBackoff: number // seconds
  logDefaultRows: number
  dataRetention: number // days, 0 = forever
}

export interface SettingsSummary {
  general: {
    systemName: string
    timezone: string
    language: string
    autoRefresh: boolean
  }
  syncStrategy: {
    syncInterval: number // seconds
    syncJitter: number // seconds
    healthInterval: number // seconds
    healthJitter: number // seconds
    fastestCooldown: number // seconds
  }
  rateAndThreshold: SettingPresets
  notifications: {
    emailEnabled: boolean
    emailRecipients: string[]
    webhookEnabled: boolean
    webhookUrl: string
    alertOnStalled: boolean
    alertOnFailed: boolean
    alertThresholdMinutes: number
  }
  credentials: {
    lastVerifiedAt?: string | null
    lastError?: string | null
  }
}

export interface DashboardStats {
  nodes: {
    total: number
    online: number
    degraded: number
    offline: number
    banned: number
  }
  tunnels: {
    total: number
    enabled: number
    pending: number
    conflict: number
    overridden: number
    failed: number
  }
  websites: {
    total: number
    synced: number
    pending: number
    httpsError: number
    overridden: number
  }
  jobs: {
    queued: number
    running: number
    failed: number
    stalled: number
    completed: number
  }
  sync: {
    lastSuccessAt?: string | null
    nextScheduledAt?: string | null
    isStale: boolean
  }
}

export interface RecentActivity {
  id: string
  type: 'job' | 'audit' | 'sync'
  title: string
  description: string
  status: string
  createdAt: string
}

// ===================== API Functions =====================

// --- Nodes ---
export async function listNodes(): Promise<NodeSummary[]> {
  return request('/nodes')
}

export async function getNode(id: string): Promise<NodeSummary> {
  return request(`/nodes/${id}`)
}

// --- Tunnels ---
export async function listTunnels(): Promise<TunnelSummary[]> {
  return request('/tunnels')
}

export async function getTunnel(id: string): Promise<TunnelSummary> {
  return request(`/tunnels/${id}`)
}

// --- Website Mappings ---
export async function listWebsiteMappings(): Promise<WebsiteMappingSummary[]> {
  return request('/website-mappings')
}

export async function getWebsiteMapping(id: string): Promise<WebsiteMappingSummary> {
  return request(`/website-mappings/${id}`)
}

// --- Jobs ---
export async function listJobs(): Promise<JobSummary[]> {
  return request('/jobs')
}

export async function getJob(id: string): Promise<JobSummary> {
  return request(`/jobs/${id}`)
}

export async function listJobEvents(jobId: string): Promise<JobEventSummary[]> {
  return request(`/jobs/${jobId}/events`)
}

export async function retryJob(id: string): Promise<JobSummary> {
  return request(`/jobs/${id}/retry`, { method: 'POST' })
}

export async function cancelJob(id: string): Promise<JobSummary> {
  return request(`/jobs/${id}/cancel`, { method: 'POST' })
}

export async function unlockJob(id: string): Promise<JobSummary> {
  return request(`/jobs/${id}/unlock`, { method: 'POST' })
}

// --- Logs ---
export async function listLogs(params?: {
  category?: LogCategory
  level?: string
  targetType?: string
  targetId?: string
  since?: string
  until?: string
  search?: string
  limit?: number
}): Promise<LogEntry[]> {
  const query = params ? new URLSearchParams(params as Record<string, string>).toString() : ''
  return request(`/logs?${query}`)
}

export async function listAuditLogs(): Promise<AuditLogSummary[]> {
  return request('/logs/audit')
}

export async function listSyncTrails(): Promise<SyncTrailSummary[]> {
  return request('/logs/sync-trails')
}

// --- Dashboard ---
export async function getDashboardStats(): Promise<DashboardStats> {
  return request('/dashboard/stats')
}

export async function getRecentActivity(): Promise<RecentActivity[]> {
  return request('/dashboard/activity')
}

// --- Settings ---
export async function getSettings(): Promise<SettingsSummary> {
  return request('/settings')
}

export async function updateSettings(data: Partial<SettingsSummary>): Promise<SettingsSummary> {
  return request('/settings', { method: 'PUT', body: JSON.stringify(data) })
}

export async function validateCredentials(): Promise<{ valid: boolean; message: string }> {
  return request('/settings/validate-credentials', { method: 'POST' })
}

export async function clearCompletedJobs(): Promise<{ deleted: number }> {
  return request('/settings/clear-completed-jobs', { method: 'POST' })
}

export async function resetLocalCache(): Promise<{ message: string }> {
  return request('/settings/reset-cache', { method: 'POST' })
}

export async function revokeUpstreams(): Promise<{ message: string }> {
  return request('/settings/revoke-upstreams', { method: 'POST' })
}
