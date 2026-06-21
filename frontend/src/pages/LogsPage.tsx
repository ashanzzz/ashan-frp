import { useState, useEffect, useCallback, useRef } from 'react'
import {
  listLogs,
  listAuditLogs,
  listSyncTrails,
  type LogEntry,
  type AuditLogSummary,
  type SyncTrailSummary,
} from '../api'
import StatusTag from '../components/StatusTag'
import EmptyState from '../components/EmptyState'
import DetailDrawer from '../components/DetailDrawer'

/* ────────── types ────────── */

type LogTab = 'job_events' | 'audit' | 'sync' | 'snapshots'

/* ────────── component ────────── */

export default function LogsPage() {
  const [activeTab, setActiveTab] = useState<LogTab>('job_events')
  const [jobLogs, setJobLogs] = useState<LogEntry[]>([])
  const [auditLogs, setAuditLogs] = useState<AuditLogSummary[]>([])
  const [syncTrails, setSyncTrails] = useState<SyncTrailSummary[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sseStatus, setSseStatus] = useState<'connected' | 'polling' | 'disconnected'>('disconnected')

  // filters
  const [levelFilter, setLevelFilter] = useState<string>('all')
  const [searchTerm, setSearchTerm] = useState('')
  const [sinceFilter, setSinceFilter] = useState('')
  const [untilFilter, setUntilFilter] = useState('')

  // drawer
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedEntry, setSelectedEntry] = useState<LogEntry | null>(null)

  // polling / SSE refs
  const sseRef = useRef<EventSource | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  /* ── fetch data ── */

  const fetchJobLogs = useCallback(async () => {
    try {
      const params: Parameters<typeof listLogs>[0] = { category: 'job_events' }
      if (levelFilter !== 'all') params.level = levelFilter
      if (searchTerm) params.search = searchTerm
      if (sinceFilter) params.since = sinceFilter
      if (untilFilter) params.until = untilFilter
      const data = await listLogs(params)
      setJobLogs(data)
    } catch {
      setError('加载作业日志失败')
    }
  }, [levelFilter, searchTerm, sinceFilter, untilFilter])

  const fetchAuditLogs = useCallback(async () => {
    try {
      const data = await listAuditLogs()
      setAuditLogs(data)
    } catch {
      setError('加载审计日志失败')
    }
  }, [])

  const fetchSyncTrails = useCallback(async () => {
    try {
      const data = await listSyncTrails()
      setSyncTrails(data)
    } catch {
      setError('加载同步轨迹失败')
    }
  }, [])

  const fetchByTab = useCallback(async () => {
    setLoading(true)
    setError(null)
    switch (activeTab) {
      case 'job_events':
        await fetchJobLogs()
        break
      case 'audit':
        await fetchAuditLogs()
        break
      case 'sync':
        await fetchSyncTrails()
        break
      case 'snapshots':
        // snapshots are placeholder for now
        break
    }
    setLoading(false)
  }, [activeTab, fetchJobLogs, fetchAuditLogs, fetchSyncTrails])

  /* ── SSE with polling fallback ── */

  const connectSSE = useCallback(() => {
    if (sseRef.current) {
      sseRef.current.close()
      sseRef.current = null
    }
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }

    if (activeTab === 'snapshots') return // no streaming for snapshots yet

    try {
      const url = `/api/sse/logs/${activeTab}`
      const es = new EventSource(url)
      sseRef.current = es

      es.onopen = () => {
        setSseStatus('connected')
      }

      es.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data) as (LogEntry | AuditLogSummary | SyncTrailSummary)
          if (activeTab === 'job_events') {
            setJobLogs((prev) => {
              const newEntry = parsed as LogEntry
              if (prev.some((e) => e.id === newEntry.id)) return prev
              return [newEntry, ...prev].slice(0, 500)
            })
          } else if (activeTab === 'audit') {
            const newAudit = parsed as AuditLogSummary
            setAuditLogs((prev) => {
              if (prev.some((e) => e.id === newAudit.id)) return prev
              return [newAudit, ...prev].slice(0, 500)
            })
          } else if (activeTab === 'sync') {
            const newSync = parsed as SyncTrailSummary
            setSyncTrails((prev) => {
              if (prev.some((e) => e.id === newSync.id)) return prev
              return [newSync, ...prev].slice(0, 500)
            })
          }
        } catch {
          // ignore malformed events
        }
      }

      es.onerror = () => {
        es.close()
        sseRef.current = null
        setSseStatus('polling')
        // fallback to polling
        pollRef.current = setInterval(() => {
          if (activeTab === 'job_events') fetchJobLogs()
          else if (activeTab === 'audit') fetchAuditLogs()
          else if (activeTab === 'sync') fetchSyncTrails()
        }, 5000)
      }
    } catch {
      setSseStatus('polling')
      pollRef.current = setInterval(() => {
        if (activeTab === 'job_events') fetchJobLogs()
        else if (activeTab === 'audit') fetchAuditLogs()
        else if (activeTab === 'sync') fetchSyncTrails()
      }, 5000)
    }
  }, [activeTab, fetchJobLogs, fetchAuditLogs, fetchSyncTrails])

  /* ── effects ── */

  useEffect(() => {
    fetchByTab()
  }, [fetchByTab])

  useEffect(() => {
    connectSSE()
    return () => {
      if (sseRef.current) {
        sseRef.current.close()
        sseRef.current = null
      }
      if (pollRef.current) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
    }
  }, [connectSSE])

  /* ── drawer ── */

  const openDetail = (entry: LogEntry) => {
    setSelectedEntry(entry)
    setDrawerOpen(true)
  }

  /* ── render helpers ── */

  const renderStatusBadge = () => {
    const cls =
      sseStatus === 'connected'
        ? 'log-status connected'
        : sseStatus === 'polling'
          ? 'log-status polling'
          : 'log-status disconnected'
    const text = sseStatus === 'connected' ? '实时接收' : sseStatus === 'polling' ? '轮询中' : '已断开'
    return <span className={cls}>{text}</span>
  }

  const renderJobEvents = () => {
    if (loading) return <div className="dashboard-loading">加载中...</div>
    if (jobLogs.length === 0) {
      return (
        <EmptyState
          title="暂无作业日志"
          description="当前没有作业日志记录。请尝试调整筛选条件或等待系统运行。"
          secondaryAction={{ label: '刷新', onClick: fetchByTab }}
        />
      )
    }
    return (
      <div className="data-table-wrapper">
        <table className="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>来源</th>
              <th>级别</th>
              <th>目标</th>
              <th>消息</th>
            </tr>
          </thead>
          <tbody>
            {jobLogs.map((entry) => (
              <tr
                key={entry.id}
                className="data-table-row"
                onClick={() => openDetail(entry)}
                style={{ cursor: 'pointer' }}
              >
                <td>{new Date(entry.timestamp).toLocaleString()}</td>
                <td>{entry.source}</td>
                <td>
                  <StatusTag text={entry.level} />
                </td>
                <td>
                  {entry.targetType && entry.targetId
                    ? `${entry.targetType} #${entry.targetId}`
                    : '—'}
                </td>
                <td>{entry.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  const renderAuditLogs = () => {
    if (loading) return <div className="dashboard-loading">加载中...</div>
    if (auditLogs.length === 0) {
      return (
        <EmptyState
          title="暂无审计日志"
          description="当前筛选范围内没有审计记录。"
          secondaryAction={{ label: '刷新', onClick: fetchByTab }}
        />
      )
    }
    return (
      <div className="data-table-wrapper">
        <table className="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>操作者</th>
              <th>动作</th>
              <th>资源类型</th>
              <th>资源 ID</th>
            </tr>
          </thead>
          <tbody>
            {auditLogs.map((log) => (
              <tr key={log.id} className="data-table-row">
                <td>{new Date(log.createdAt).toLocaleString()}</td>
                <td>
                  {log.actorType} #{log.actorId}
                </td>
                <td>{log.action}</td>
                <td>{log.resourceType}</td>
                <td>{log.resourceId}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  const renderSyncTrails = () => {
    if (loading) return <div className="dashboard-loading">加载中...</div>
    if (syncTrails.length === 0) {
      return (
        <EmptyState
          title="暂无同步轨迹"
          description="尚未产生同步轨迹记录。"
          secondaryAction={{ label: '刷新', onClick: fetchByTab }}
        />
      )
    }
    return (
      <div className="data-table-wrapper">
        <table className="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>对象类型</th>
              <th>对象 ID</th>
              <th>状态</th>
              <th>期望哈希</th>
              <th>观测哈希</th>
            </tr>
          </thead>
          <tbody>
            {syncTrails.map((trail) => (
              <tr key={trail.id} className="data-table-row">
                <td>{new Date(trail.createdAt).toLocaleString()}</td>
                <td>{trail.subjectType}</td>
                <td>{trail.subjectId}</td>
                <td>
                  <StatusTag text={trail.status} />
                </td>
                <td className="mono">{trail.desiredHash.slice(0, 8)}...</td>
                <td className="mono">{trail.observedHash.slice(0, 8)}...</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  const renderSnapshots = () => {
    return (
      <EmptyState
        title="原始快照"
        description="原始快照功能开发中。此处将展示系统各时间点的完整状态快照，用于排障与回滚。"
      />
    )
  }

  const renderDetailContent = () => {
    if (!selectedEntry) return null
    return (
      <div className="log-detail">
        <div className="detail-grid">
          <div className="detail-item">
            <span className="detail-label">ID</span>
            <span className="mono">{selectedEntry.id}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">时间</span>
            <span>{new Date(selectedEntry.timestamp).toLocaleString()}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">来源</span>
            <span>{selectedEntry.source}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">级别</span>
            <StatusTag text={selectedEntry.level} />
          </div>
          <div className="detail-item">
            <span className="detail-label">消息</span>
            <span>{selectedEntry.message}</span>
          </div>
          {selectedEntry.targetType && (
            <div className="detail-item">
              <span className="detail-label">目标类型</span>
              <span>{selectedEntry.targetType}</span>
            </div>
          )}
          {selectedEntry.targetId && (
            <div className="detail-item">
              <span className="detail-label">目标 ID</span>
              <span>{selectedEntry.targetId}</span>
            </div>
          )}
        </div>
        {selectedEntry.payload && (
          <div className="log-payload">
            <h4>Payload</h4>
            <pre>{JSON.stringify(selectedEntry.payload, null, 2)}</pre>
          </div>
        )}
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>日志</h1>
        <p>查看运行日志、审计记录与同步轨迹</p>
      </div>

      {/* Status & refresh */}
      <div className="log-status-bar">
        {renderStatusBadge()}
        <button className="btn-secondary btn-sm" onClick={fetchByTab}>
          刷新
        </button>
      </div>

      {error && (
        <div className="error-banner">
          {error}
          <button className="btn-text" onClick={() => setError(null)}>
            ×
          </button>
        </div>
      )}

      {/* Tab bar */}
      <div className="log-tabs">
        {(['job_events', 'audit', 'sync', 'snapshots'] as LogTab[]).map((tab) => {
          const labels: Record<LogTab, string> = {
            job_events: '作业日志',
            audit: '审计日志',
            sync: '同步轨迹',
            snapshots: '原始快照',
          }
          return (
            <button
              key={tab}
              className={`log-tab ${activeTab === tab ? 'active' : ''}`}
              onClick={() => setActiveTab(tab)}
            >
              {labels[tab]}
            </button>
          )
        })}
      </div>

      {/* Filters */}
      {activeTab === 'job_events' && (
        <div className="log-filters">
          <input
            type="text"
            className="search-input"
            placeholder="搜索关键词..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
          <select className="filter-select" value={levelFilter} onChange={(e) => setLevelFilter(e.target.value)}>
            <option value="all">全部级别</option>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="error">Error</option>
          </select>
          <input
            type="datetime-local"
            className="filter-date"
            value={sinceFilter}
            onChange={(e) => setSinceFilter(e.target.value)}
          />
          <input
            type="datetime-local"
            className="filter-date"
            value={untilFilter}
            onChange={(e) => setUntilFilter(e.target.value)}
          />
        </div>
      )}

      {/* Content */}
      {activeTab === 'job_events' && renderJobEvents()}
      {activeTab === 'audit' && renderAuditLogs()}
      {activeTab === 'sync' && renderSyncTrails()}
      {activeTab === 'snapshots' && renderSnapshots()}

      {/* Detail Drawer */}
      <DetailDrawer isOpen={drawerOpen} onClose={() => setDrawerOpen(false)} title="日志详情">
        {renderDetailContent()}
      </DetailDrawer>
    </div>
  )
}
