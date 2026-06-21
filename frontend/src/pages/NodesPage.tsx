import { useCallback, useEffect, useMemo, useState } from 'react'
import { getNode, listNodes, type NodeSummary } from '../api'
import DetailDrawer from '../components/DetailDrawer'
import EmptyState from '../components/EmptyState'
import StatusTag from '../components/StatusTag'

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}

function displayName(node: NodeSummary) {
  return node.displayName?.trim() || node.name
}

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'online', label: '在线' },
  { value: 'degraded', label: '降级' },
  { value: 'offline', label: '离线' },
  { value: 'banned', label: '封禁' },
  { value: 'archived', label: '归档' },
]

export default function NodesPage() {
  const [nodes, setNodes] = useState<NodeSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('all')
  const [providerFilter, setProviderFilter] = useState('all')
  const [searchTerm, setSearchTerm] = useState('')
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [selectedNode, setSelectedNode] = useState<NodeSummary | null>(null)

  const fetchNodes = useCallback(async () => {
    setLoading(true)
    try {
      setError(null)
      const data = await listNodes()
      setNodes(data)
      setLastRefreshed(new Date())
    } catch {
      setError('加载节点列表失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchNodes()
  }, [fetchNodes])

  const providers = useMemo(
    () => Array.from(new Set(nodes.map((node) => node.provider))).sort((a, b) => a.localeCompare(b)),
    [nodes],
  )

  const filtered = useMemo(() => {
    const query = searchTerm.trim().toLowerCase()
    return nodes.filter((node) => {
      if (statusFilter !== 'all' && node.status !== statusFilter) return false
      if (providerFilter !== 'all' && node.provider !== providerFilter) return false
      if (!query) return true
      return [
        node.id,
        node.name,
        node.displayName,
        node.provider,
        node.endpoint,
        node.region,
        node.lastFailureReason,
      ]
        .filter(Boolean)
        .some((item) => String(item).toLowerCase().includes(query))
    })
  }, [nodes, providerFilter, searchTerm, statusFilter])

  const statusOrder: Record<string, number> = {
    online: 0,
    degraded: 1,
    offline: 2,
    banned: 3,
    archived: 4,
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

  const openDetail = useCallback(async (node: NodeSummary) => {
    setDrawerOpen(true)
    setSelectedNode(node)
    setDetailLoading(true)
    try {
      const full = await getNode(node.id)
      setSelectedNode(full)
    } catch {
      // keep summary data if detail fetch fails
    } finally {
      setDetailLoading(false)
    }
  }, [])

  if (loading) {
    return (
      <div>
        <div className="page-header">
          <h1>节点列表</h1>
          <p>管理上游节点与健康状态</p>
        </div>
        <div className="dashboard-loading">加载中...</div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>节点列表</h1>
        <p>管理上游节点与健康状态</p>
      </div>

      {error && (
        <div className="error-banner">
          {error}
          <button className="btn-text" onClick={fetchNodes} type="button">
            重试
          </button>
        </div>
      )}

      {lastRefreshed && (
        <div className="last-refreshed">
          <span>最后刷新：{lastRefreshed.toLocaleTimeString()}</span>
          <button className="btn-text" onClick={fetchNodes} type="button">
            刷新
          </button>
        </div>
      )}

      <div className="job-filters">
        <input
          type="text"
          className="search-input"
          placeholder="搜索节点名称 / provider / 地址 / 备注..."
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
        <select
          className="filter-select"
          value={providerFilter}
          onChange={(e) => setProviderFilter(e.target.value)}
        >
          <option value="all">全部 provider</option>
          {providers.map((provider) => (
            <option key={provider} value={provider}>
              {provider}
            </option>
          ))}
        </select>
        <button className="btn-secondary btn-sm" onClick={fetchNodes} type="button">
          刷新
        </button>
      </div>

      {sorted.length === 0 ? (
        <EmptyState
          title="还没有节点"
          description={
            statusFilter !== 'all' || providerFilter !== 'all' || searchTerm
              ? '当前筛选条件下没有匹配节点，请尝试放宽筛选条件。'
              : '先添加一个节点，系统才能做健康检查和自动同步。'
          }
          primaryAction={{ label: '刷新', onClick: fetchNodes }}
        />
      ) : (
        <div className="data-table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>节点</th>
                <th>Provider</th>
                <th>地址</th>
                <th>区域</th>
                <th>最后心跳</th>
                <th>最近失败</th>
                <th>关联数</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((node) => (
                <tr key={node.id} className="data-table-row" onClick={() => openDetail(node)}>
                  <td>
                    <StatusTag text={node.status} />
                  </td>
                  <td>
                    <div className="table-cell-stack">
                      <strong>{displayName(node)}</strong>
                      <span className="mono text-muted">{node.id}</span>
                    </div>
                  </td>
                  <td>{node.provider}</td>
                  <td className="mono">{node.endpoint || '—'}</td>
                  <td>{node.region || '—'}</td>
                  <td>{formatDate(node.lastHeartbeatAt)}</td>
                  <td>
                    <span className={node.lastFailureReason ? 'text-error' : 'text-muted'}>
                      {node.lastFailureReason || '—'}
                    </span>
                  </td>
                  <td>{node.dependentCount}</td>
                  <td>
                    <button
                      className="btn-secondary btn-sm"
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        void openDetail(node)
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
        title={`节点详情 — ${selectedNode ? displayName(selectedNode) : ''}`}
      >
        {detailLoading && !selectedNode ? (
          <p className="text-muted">加载中...</p>
        ) : selectedNode ? (
          <div className="detail-section">
            <div className="detail-grid">
              <div className="detail-item">
                <span className="detail-label">状态</span>
                <StatusTag text={selectedNode.status} />
              </div>
              <div className="detail-item">
                <span className="detail-label">节点名称</span>
                <span>{displayName(selectedNode)}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">节点 ID</span>
                <span className="mono">{selectedNode.id}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">Provider</span>
                <span>{selectedNode.provider}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">地址</span>
                <span className="mono">{selectedNode.endpoint || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">区域</span>
                <span>{selectedNode.region || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">最后心跳</span>
                <span>{formatDate(selectedNode.lastHeartbeatAt)}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">最后成功</span>
                <span>{formatDate(selectedNode.lastSuccessAt)}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">最近失败原因</span>
                <span className={selectedNode.lastFailureReason ? 'text-error' : 'text-muted'}>
                  {selectedNode.lastFailureReason || '无'}
                </span>
              </div>
              <div className="detail-item">
                <span className="detail-label">关联数</span>
                <span>{selectedNode.dependentCount}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">创建时间</span>
                <span>{formatDate(selectedNode.createdAt)}</span>
              </div>
            </div>
          </div>
        ) : (
          <EmptyState title="未选择节点" description="请选择一条节点记录查看详情。" />
        )}
      </DetailDrawer>
    </div>
  )
}
