import { useCallback, useEffect, useMemo, useState } from 'react'
import { getTunnel, listTunnels, type TunnelSummary } from '../api'
import DetailDrawer from '../components/DetailDrawer'
import EmptyState from '../components/EmptyState'
import StatusTag from '../components/StatusTag'

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}

function tunnelTarget(tunnel: TunnelSummary) {
  if (tunnel.domain) return tunnel.domain
  if (tunnel.remotePort) return `:${tunnel.remotePort}`
  return '—'
}

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'enabled', label: '已启用' },
  { value: 'pending', label: '待应用' },
  { value: 'conflict', label: '冲突' },
  { value: 'overridden', label: '已覆盖' },
  { value: 'failed', label: '失败' },
]

export default function TunnelsPage() {
  const [tunnels, setTunnels] = useState<TunnelSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('all')
  const [searchTerm, setSearchTerm] = useState('')
  const [driftOnly, setDriftOnly] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [selectedTunnel, setSelectedTunnel] = useState<TunnelSummary | null>(null)
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null)

  const fetchTunnels = useCallback(async () => {
    setLoading(true)
    try {
      setError(null)
      const data = await listTunnels()
      setTunnels(data)
      setLastRefreshed(new Date())
    } catch {
      setError('加载隧道列表失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchTunnels()
  }, [fetchTunnels])

  const filtered = useMemo(() => {
    const query = searchTerm.trim().toLowerCase()
    return tunnels.filter((tunnel) => {
      if (statusFilter !== 'all' && tunnel.status !== statusFilter) return false
      if (driftOnly && !tunnel.hasDrift) return false
      if (!query) return true
      return [
        tunnel.id,
        tunnel.name,
        tunnel.nodeName,
        tunnel.domain,
        tunnel.localAddress,
        String(tunnel.localPort),
        String(tunnel.remotePort ?? ''),
        tunnel.expectedStatus,
        tunnel.observedStatus,
      ]
        .filter(Boolean)
        .some((item) => String(item).toLowerCase().includes(query))
    })
  }, [driftOnly, searchTerm, statusFilter, tunnels])

  const statusOrder: Record<string, number> = {
    enabled: 0,
    pending: 1,
    overridden: 2,
    conflict: 3,
    failed: 4,
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

  const openDetail = useCallback(async (tunnel: TunnelSummary) => {
    setDrawerOpen(true)
    setSelectedTunnel(tunnel)
    setDetailLoading(true)
    try {
      const full = await getTunnel(tunnel.id)
      setSelectedTunnel(full)
    } catch {
      // keep summary data
    } finally {
      setDetailLoading(false)
    }
  }, [])

  if (loading) {
    return (
      <div>
        <div className="page-header">
          <h1>隧道列表</h1>
          <p>管理固定隧道与运行态差异</p>
        </div>
        <div className="dashboard-loading">加载中...</div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>隧道列表</h1>
        <p>管理固定隧道与运行态差异</p>
      </div>

      {error && (
        <div className="error-banner">
          {error}
          <button className="btn-text" onClick={fetchTunnels} type="button">
            重试
          </button>
        </div>
      )}

      {lastRefreshed && (
        <div className="last-refreshed">
          <span>最后刷新：{lastRefreshed.toLocaleTimeString()}</span>
          <button className="btn-text" onClick={fetchTunnels} type="button">
            刷新
          </button>
        </div>
      )}

      <div className="job-filters">
        <input
          type="text"
          className="search-input"
          placeholder="搜索隧道 / 节点 / 域名 / 端口..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
        <select
          className="filter-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <label className="filter-toggle">
          <input type="checkbox" checked={driftOnly} onChange={(e) => setDriftOnly(e.target.checked)} />
          仅看漂移
        </label>
        <button className="btn-secondary btn-sm" onClick={fetchTunnels} type="button">
          刷新
        </button>
      </div>

      {sorted.length === 0 ? (
        <EmptyState
          title="还没有隧道"
          description={
            statusFilter !== 'all' || searchTerm || driftOnly
              ? '当前筛选条件下没有匹配隧道，请尝试放宽筛选条件。'
              : '创建隧道后，系统才能把本地服务映射到远端。'
          }
          primaryAction={{ label: '刷新', onClick: fetchTunnels }}
        />
      ) : (
        <div className="data-table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>隧道</th>
                <th>节点</th>
                <th>本地地址</th>
                <th>远端</th>
                <th>期望态</th>
                <th>观测态</th>
                <th>漂移</th>
                <th>覆盖</th>
                <th>最后应用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((tunnel) => (
                <tr key={tunnel.id} className="data-table-row" onClick={() => openDetail(tunnel)}>
                  <td>
                    <StatusTag text={tunnel.status} />
                  </td>
                  <td>
                    <div className="table-cell-stack">
                      <strong>{tunnel.name}</strong>
                      <span className="mono text-muted">{tunnel.id}</span>
                    </div>
                  </td>
                  <td>{tunnel.nodeName || tunnel.nodeId}</td>
                  <td className="mono">
                    {tunnel.localAddress}:{tunnel.localPort}
                  </td>
                  <td>{tunnelTarget(tunnel)}</td>
                  <td>
                    <StatusTag text={tunnel.expectedStatus} />
                  </td>
                  <td>
                    <StatusTag text={tunnel.observedStatus} />
                  </td>
                  <td>
                    <StatusTag text={tunnel.hasDrift ? '有漂移' : '一致'} variant={tunnel.hasDrift ? 'warning' : 'normal'} />
                  </td>
                  <td>
                    <StatusTag text={tunnel.isOverridden ? '已覆盖' : '正常'} variant={tunnel.isOverridden ? 'warning' : 'normal'} />
                  </td>
                  <td>{formatDate(tunnel.lastAppliedAt)}</td>
                  <td>
                    <button
                      className="btn-secondary btn-sm"
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        void openDetail(tunnel)
                      }}
                    >
                      详情
                    </button>
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
        title={`隧道详情 — ${selectedTunnel?.name ?? ''}`}
      >
        {detailLoading && !selectedTunnel ? (
          <p className="text-muted">加载中...</p>
        ) : selectedTunnel ? (
          <div className="detail-section">
            <div className="detail-grid">
              <div className="detail-item">
                <span className="detail-label">状态</span>
                <StatusTag text={selectedTunnel.status} />
              </div>
              <div className="detail-item">
                <span className="detail-label">名称</span>
                <span>{selectedTunnel.name}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">隧道 ID</span>
                <span className="mono">{selectedTunnel.id}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">节点</span>
                <span>{selectedTunnel.nodeName || selectedTunnel.nodeId}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">本地地址</span>
                <span className="mono">
                  {selectedTunnel.localAddress}:{selectedTunnel.localPort}
                </span>
              </div>
              <div className="detail-item">
                <span className="detail-label">远端端口</span>
                <span>{selectedTunnel.remotePort ?? '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">域名</span>
                <span>{selectedTunnel.domain || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">期望态</span>
                <StatusTag text={selectedTunnel.expectedStatus} />
              </div>
              <div className="detail-item">
                <span className="detail-label">观测态</span>
                <StatusTag text={selectedTunnel.observedStatus} />
              </div>
              <div className="detail-item">
                <span className="detail-label">是否漂移</span>
                <span>{selectedTunnel.hasDrift ? '是' : '否'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">是否覆盖</span>
                <span>{selectedTunnel.isOverridden ? '是' : '否'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">最后应用</span>
                <span>{formatDate(selectedTunnel.lastAppliedAt)}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">创建时间</span>
                <span>{formatDate(selectedTunnel.createdAt)}</span>
              </div>
            </div>
          </div>
        ) : (
          <EmptyState title="未选择隧道" description="请选择一条隧道记录查看详情。" />
        )}
      </DetailDrawer>
    </div>
  )
}
