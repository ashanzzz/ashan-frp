import { useCallback, useEffect, useMemo, useState } from 'react'
import { getJob, listJobEvents, listJobs, retryJob, cancelJob, unlockJob, type JobEventSummary, type JobStatus, type JobSummary } from '../api'
import DetailDrawer from '../components/DetailDrawer'
import EmptyState from '../components/EmptyState'
import StatusTag from '../components/StatusTag'

const JOB_STATUS_LABELS: Record<JobStatus, string> = {
  queued: '排队中',
  running: '运行中',
  retry_wait: '等待重试',
  blocked: '已阻塞',
  stalled: '卡住',
  succeeded: '已完成',
  failed: '失败',
  canceled: '已取消',
}

interface JobDetail extends JobSummary {
  events?: JobEventSummary[]
  eventsLoading?: boolean
}

const STATUS_OPTIONS: Array<{ value: JobStatus | 'all'; label: string }> = [
  { value: 'all', label: '全部状态' },
  { value: 'queued', label: '排队中' },
  { value: 'running', label: '运行中' },
  { value: 'retry_wait', label: '等待重试' },
  { value: 'blocked', label: '已阻塞' },
  { value: 'stalled', label: '卡住' },
  { value: 'failed', label: '失败' },
  { value: 'canceled', label: '已取消' },
  { value: 'succeeded', label: '已完成' },
]

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}

export default function JobsPage() {
  const [jobs, setJobs] = useState<JobSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedJob, setSelectedJob] = useState<JobDetail | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [statusFilter, setStatusFilter] = useState<JobStatus | 'all'>('all')
  const [searchTerm, setSearchTerm] = useState('')
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null)

  const fetchJobs = useCallback(async () => {
    setLoading(true)
    try {
      setError(null)
      const data = await listJobs()
      setJobs(data)
      setLastRefreshed(new Date())
    } catch {
      setError('加载任务列表失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchJobs()
  }, [fetchJobs])

  const openDetail = useCallback(async (job: JobSummary) => {
    setDrawerOpen(true)
    setSelectedJob({ ...job, eventsLoading: true })

    try {
      const [fullJob, events] = await Promise.all([getJob(job.id), listJobEvents(job.id)])
      setSelectedJob({ ...fullJob, events, eventsLoading: false })
    } catch {
      setSelectedJob((prev) => (prev ? { ...prev, eventsLoading: false } : null))
    }
  }, [])

  const handleRetry = async (id: string) => {
    setActionLoading(id)
    try {
      await retryJob(id)
      await fetchJobs()
    } catch {
      setError('重试操作失败')
    } finally {
      setActionLoading(null)
    }
  }

  const handleCancel = async (id: string) => {
    setActionLoading(id)
    try {
      await cancelJob(id)
      await fetchJobs()
    } catch {
      setError('取消操作失败')
    } finally {
      setActionLoading(null)
    }
  }

  const handleUnlock = async (id: string) => {
    setActionLoading(id)
    try {
      await unlockJob(id)
      await fetchJobs()
    } catch {
      setError('解锁操作失败')
    } finally {
      setActionLoading(null)
    }
  }

  const filtered = useMemo(() => {
    const q = searchTerm.trim().toLowerCase()
    return jobs.filter((job) => {
      if (statusFilter !== 'all' && job.status !== statusFilter) return false
      if (!q) return true
      return [job.id, job.type, job.targetId, job.targetName, job.targetType, job.idempotencyKey]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(q))
    })
  }, [jobs, searchTerm, statusFilter])

  const statusOrder: Record<JobStatus, number> = {
    running: 0,
    queued: 1,
    retry_wait: 2,
    blocked: 3,
    stalled: 4,
    failed: 5,
    canceled: 6,
    succeeded: 7,
  }

  const sorted = useMemo(
    () =>
      [...filtered].sort((a, b) => {
        const oa = statusOrder[a.status] ?? 99
        const ob = statusOrder[b.status] ?? 99
        if (oa !== ob) return oa - ob
        const ta = a.createdAt ? new Date(a.createdAt).getTime() : 0
        const tb = b.createdAt ? new Date(b.createdAt).getTime() : 0
        return tb - ta
      }),
    [filtered],
  )

  const renderEventTimeline = (events: JobEventSummary[]) => {
    if (!events || events.length === 0) {
      return <p className="text-muted">暂无作业事件</p>
    }
    return (
      <ul className="event-timeline">
        {events.map((evt) => (
          <li key={evt.id} className={`event-item level-${evt.level}`}>
            <div className="event-meta">
              <span className="event-type">{evt.eventType}</span>
              <span className="event-time">{formatDate(evt.createdAt)}</span>
            </div>
            <div className="event-message">{evt.message}</div>
            {evt.payload && (
              <details className="event-details">
                <summary>详情</summary>
                <pre>{JSON.stringify(evt.payload, null, 2)}</pre>
              </details>
            )}
          </li>
        ))}
      </ul>
    )
  }

  if (loading) {
    return (
      <div>
        <div className="page-header">
          <h1>任务队列</h1>
          <p>观察排队、重试与执行状态</p>
        </div>
        <div className="dashboard-loading">加载中...</div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>任务队列</h1>
        <p>观察排队、重试与执行状态</p>
      </div>

      {error && (
        <div className="error-banner">
          {error}
          <button className="btn-text" onClick={fetchJobs} type="button">
            重试
          </button>
        </div>
      )}

      {lastRefreshed && (
        <div className="last-refreshed">
          <span>最后刷新：{lastRefreshed.toLocaleTimeString()}</span>
          <button className="btn-text" onClick={fetchJobs} type="button">
            刷新
          </button>
        </div>
      )}

      <div className="job-filters">
        <input
          type="text"
          className="search-input"
          placeholder="搜索任务 ID / 类型 / 目标..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
        <select
          className="filter-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as JobStatus | 'all')}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <button className="btn-secondary btn-sm" onClick={fetchJobs} type="button">
          刷新
        </button>
      </div>

      {sorted.length === 0 ? (
        <EmptyState
          title="当前没有待执行任务"
          description={
            statusFilter !== 'all' || searchTerm
              ? '当前筛选条件下无匹配任务，请尝试放宽筛选条件。'
              : '当系统没有冲突、同步或重试需要时，这里会保持为空。'
          }
          primaryAction={{ label: '刷新', onClick: fetchJobs }}
        />
      ) : (
        <div className="data-table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>作业类型</th>
                <th>目标</th>
                <th>优先级</th>
                <th>创建时间</th>
                <th>尝试</th>
                <th>锁定者</th>
                <th>最近结果</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((job) => (
                <tr key={job.id} className="data-table-row" onClick={() => void openDetail(job)}>
                  <td>
                    <StatusTag text={JOB_STATUS_LABELS[job.status]} />
                  </td>
                  <td>{job.type}</td>
                  <td>
                    <div className="table-cell-stack">
                      <strong>{job.targetName || job.targetId || '—'}</strong>
                      <span className="mono text-muted">
                        {job.targetType ? `${job.targetType} #${job.targetId ?? '—'}` : '—'}
                      </span>
                    </div>
                  </td>
                  <td>{job.priority}</td>
                  <td>{formatDate(job.createdAt)}</td>
                  <td>
                    {job.attempts} / {job.maxAttempts}
                  </td>
                  <td>{job.lockedBy || '—'}</td>
                  <td>
                    {job.errorMessage ? (
                      <span className="text-error">
                        {job.errorMessage.length > 40 ? `${job.errorMessage.slice(0, 40)}...` : job.errorMessage}
                      </span>
                    ) : job.result ? (
                      <span className="text-success">
                        {job.result.length > 40 ? `${job.result.slice(0, 40)}...` : job.result}
                      </span>
                    ) : (
                      <span className="text-muted">—</span>
                    )}
                  </td>
                  <td>
                    <div className="table-actions" onClick={(e) => e.stopPropagation()}>
                      {(job.status === 'failed' || job.status === 'blocked') && (
                        <button
                          className="btn-sm btn-action"
                          onClick={() => void handleRetry(job.id)}
                          disabled={actionLoading === job.id}
                          type="button"
                        >
                          {actionLoading === job.id ? '...' : '重试'}
                        </button>
                      )}
                      {(job.status === 'queued' || job.status === 'running' || job.status === 'retry_wait') && (
                        <button
                          className="btn-sm btn-action btn-danger-text"
                          onClick={() => void handleCancel(job.id)}
                          disabled={actionLoading === job.id}
                          type="button"
                        >
                          {actionLoading === job.id ? '...' : '取消'}
                        </button>
                      )}
                      {job.status === 'stalled' && (
                        <button
                          className="btn-sm btn-action btn-danger-text"
                          onClick={() => void handleUnlock(job.id)}
                          disabled={actionLoading === job.id}
                          type="button"
                        >
                          {actionLoading === job.id ? '...' : '解锁'}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <DetailDrawer
        isOpen={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        title={`作业详情 — ${selectedJob?.id || ''}`}
      >
        {selectedJob && (
          <div className="job-detail">
            <section className="detail-section">
              <h4>作业摘要</h4>
              <div className="detail-grid">
                <div className="detail-item">
                  <span className="detail-label">状态</span>
                  <StatusTag text={JOB_STATUS_LABELS[selectedJob.status]} />
                </div>
                <div className="detail-item">
                  <span className="detail-label">类型</span>
                  <span>{selectedJob.type}</span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">目标对象</span>
                  <span>
                    {selectedJob.targetName || selectedJob.targetId || '—'}
                    {selectedJob.targetType ? ` (${selectedJob.targetType})` : ''}
                  </span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">优先级</span>
                  <span>{selectedJob.priority}</span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">尝试次数</span>
                  <span>
                    {selectedJob.attempts} / {selectedJob.maxAttempts}
                  </span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">创建时间</span>
                  <span>{formatDate(selectedJob.createdAt)}</span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">计划执行</span>
                  <span>
                    {selectedJob.runAfter
                      ? formatDate(selectedJob.runAfter)
                      : selectedJob.scheduledAt
                        ? formatDate(selectedJob.scheduledAt)
                        : '—'}
                  </span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">开始时间</span>
                  <span>{formatDate(selectedJob.startedAt)}</span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">完成时间</span>
                  <span>{formatDate(selectedJob.completedAt)}</span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">锁定者</span>
                  <span>{selectedJob.lockedBy || '—'}</span>
                </div>
                <div className="detail-item">
                  <span className="detail-label">幂等键</span>
                  <span className="mono">{selectedJob.idempotencyKey || '—'}</span>
                </div>
              </div>
              {selectedJob.errorCode && (
                <div className="detail-item error-block">
                  <span className="detail-label">错误码</span>
                  <span className="text-error">
                    {selectedJob.errorCode}: {selectedJob.errorMessage}
                  </span>
                </div>
              )}
              {selectedJob.result && (
                <div className="detail-item success-block">
                  <span className="detail-label">结果</span>
                  <pre className="result-payload">{selectedJob.result}</pre>
                </div>
              )}
            </section>

            <div className="detail-actions">
              {(selectedJob.status === 'failed' || selectedJob.status === 'blocked') && (
                <button
                  className="btn-primary btn-sm"
                  onClick={() => void handleRetry(selectedJob.id)}
                  disabled={actionLoading === selectedJob.id}
                  type="button"
                >
                  {actionLoading === selectedJob.id ? '重试中...' : '立即重试'}
                </button>
              )}
              {(selectedJob.status === 'queued' ||
                selectedJob.status === 'running' ||
                selectedJob.status === 'retry_wait') && (
                <button
                  className="btn-secondary btn-sm"
                  onClick={() => void handleCancel(selectedJob.id)}
                  disabled={actionLoading === selectedJob.id}
                  type="button"
                >
                  {actionLoading === selectedJob.id ? '取消中...' : '取消执行'}
                </button>
              )}
              {selectedJob.lockedBy && (
                <button
                  className="btn-secondary btn-sm btn-danger-text"
                  onClick={() => void handleUnlock(selectedJob.id)}
                  disabled={actionLoading === selectedJob.id}
                  type="button"
                >
                  {actionLoading === selectedJob.id ? '解锁中...' : '强制解锁'}
                </button>
              )}
            </div>

            <section className="detail-section">
              <h4>事件时间线</h4>
              {selectedJob.eventsLoading ? <p className="text-muted">加载中...</p> : renderEventTimeline(selectedJob.events || [])}
            </section>
          </div>
        )}
      </DetailDrawer>
    </div>
  )
}
