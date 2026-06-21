import { useCallback, useEffect, useMemo, useState } from 'react'
import { getWebsiteMapping, listWebsiteMappings, type WebsiteMappingSummary } from '../api'
import DetailDrawer from '../components/DetailDrawer'
import EmptyState from '../components/EmptyState'
import StatusTag from '../components/StatusTag'

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}

function aliasCount(mapping: WebsiteMappingSummary) {
  return mapping.aliasDomains?.length ?? 0
}

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'synced', label: '已同步' },
  { value: 'pending', label: '待同步' },
  { value: 'https_error', label: 'HTTPS 异常' },
  { value: 'overridden', label: '已覆盖' },
]

export default function WebsiteMappingsPage() {
  const [mappings, setMappings] = useState<WebsiteMappingSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('all')
  const [searchTerm, setSearchTerm] = useState('')
  const [overriddenOnly, setOverriddenOnly] = useState(false)
  const [httpsOnly, setHttpsOnly] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [selectedMapping, setSelectedMapping] = useState<WebsiteMappingSummary | null>(null)
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null)

  const fetchMappings = useCallback(async () => {
    setLoading(true)
    try {
      setError(null)
      const data = await listWebsiteMappings()
      setMappings(data)
      setLastRefreshed(new Date())
    } catch {
      setError('加载网站映射失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchMappings()
  }, [fetchMappings])

  const filtered = useMemo(() => {
    const query = searchTerm.trim().toLowerCase()
    return mappings.filter((mapping) => {
      if (statusFilter !== 'all' && mapping.status !== statusFilter) return false
      if (overriddenOnly && !mapping.isOverridden) return false
      if (httpsOnly && !mapping.httpsEnabled) return false
      if (!query) return true
      return [
        mapping.id,
        mapping.primaryDomain,
        mapping.aliasDomains?.join(' '),
        mapping.nodeName,
        mapping.tunnelName,
        mapping.panelWebsiteId,
        mapping.overrideSource,
        mapping.overrideLastModifiedBy,
      ]
        .filter(Boolean)
        .some((item) => String(item).toLowerCase().includes(query))
    })
  }, [httpsOnly, mappings, overriddenOnly, searchTerm, statusFilter])

  const statusOrder: Record<string, number> = {
    synced: 0,
    pending: 1,
    overridden: 2,
    https_error: 3,
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

  const openDetail = useCallback(async (mapping: WebsiteMappingSummary) => {
    setDrawerOpen(true)
    setSelectedMapping(mapping)
    setDetailLoading(true)
    try {
      const full = await getWebsiteMapping(mapping.id)
      setSelectedMapping(full)
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
          <h1>网站映射</h1>
          <p>管理域名映射与 HTTPS 配置</p>
        </div>
        <div className="dashboard-loading">加载中...</div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>网站映射</h1>
        <p>管理域名映射与 HTTPS 配置</p>
      </div>

      {error && (
        <div className="error-banner">
          {error}
          <button className="btn-text" onClick={fetchMappings} type="button">
            重试
          </button>
        </div>
      )}

      {lastRefreshed && (
        <div className="last-refreshed">
          <span>最后刷新：{lastRefreshed.toLocaleTimeString()}</span>
          <button className="btn-text" onClick={fetchMappings} type="button">
            刷新
          </button>
        </div>
      )}

      <div className="job-filters">
        <input
          type="text"
          className="search-input"
          placeholder="搜索主域名 / 备用域名 / 节点 / 隧道..."
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
          <input
            type="checkbox"
            checked={overriddenOnly}
            onChange={(e) => setOverriddenOnly(e.target.checked)}
          />
          仅看覆盖
        </label>
        <label className="filter-toggle">
          <input type="checkbox" checked={httpsOnly} onChange={(e) => setHttpsOnly(e.target.checked)} />
          仅看 HTTPS
        </label>
        <button className="btn-secondary btn-sm" onClick={fetchMappings} type="button">
          刷新
        </button>
      </div>

      {sorted.length === 0 ? (
        <EmptyState
          title="还没有网站映射"
          description={
            statusFilter !== 'all' || searchTerm || overriddenOnly || httpsOnly
              ? '当前筛选条件下没有匹配的网站映射，请尝试放宽筛选条件。'
              : '先把节点和隧道连起来，再创建域名映射。'
          }
          primaryAction={{ label: '刷新', onClick: fetchMappings }}
        />
      ) : (
        <div className="data-table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>主域名</th>
                <th>备用域名</th>
                <th>节点</th>
                <th>隧道</th>
                <th>HTTPS</th>
                <th>代理</th>
                <th>面板 ID</th>
                <th>覆盖</th>
                <th>最后同步</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((mapping) => (
                <tr key={mapping.id} className="data-table-row" onClick={() => openDetail(mapping)}>
                  <td>
                    <StatusTag text={mapping.status} />
                  </td>
                  <td>
                    <div className="table-cell-stack">
                      <strong>{mapping.primaryDomain}</strong>
                      <span className="mono text-muted">{mapping.id}</span>
                    </div>
                  </td>
                  <td>{aliasCount(mapping)}</td>
                  <td>{mapping.nodeName || mapping.nodeId}</td>
                  <td>{mapping.tunnelName || mapping.tunnelId}</td>
                  <td>
                    <StatusTag
                      text={mapping.httpsEnabled ? '启用' : '关闭'}
                      variant={mapping.httpsEnabled ? 'normal' : 'unknown'}
                    />
                  </td>
                  <td>
                    <StatusTag
                      text={mapping.proxyEnabled ? '启用' : '关闭'}
                      variant={mapping.proxyEnabled ? 'normal' : 'unknown'}
                    />
                  </td>
                  <td className="mono">{mapping.panelWebsiteId || '—'}</td>
                  <td>
                    <StatusTag
                      text={mapping.isOverridden ? '已覆盖' : '正常'}
                      variant={mapping.isOverridden ? 'warning' : 'normal'}
                    />
                  </td>
                  <td>{formatDate(mapping.lastSyncedAt)}</td>
                  <td>
                    <button
                      className="btn-secondary btn-sm"
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        void openDetail(mapping)
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
        title={`网站映射详情 — ${selectedMapping?.primaryDomain ?? ''}`}
      >
        {detailLoading && !selectedMapping ? (
          <p className="text-muted">加载中...</p>
        ) : selectedMapping ? (
          <div className="detail-section">
            <div className="detail-grid">
              <div className="detail-item">
                <span className="detail-label">状态</span>
                <StatusTag text={selectedMapping.status} />
              </div>
              <div className="detail-item">
                <span className="detail-label">主域名</span>
                <span>{selectedMapping.primaryDomain}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">备用域名</span>
                <span>{selectedMapping.aliasDomains?.join(' / ') || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">节点</span>
                <span>{selectedMapping.nodeName || selectedMapping.nodeId}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">隧道</span>
                <span>{selectedMapping.tunnelName || selectedMapping.tunnelId}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">HTTPS</span>
                <StatusTag
                  text={selectedMapping.httpsEnabled ? '启用' : '关闭'}
                  variant={selectedMapping.httpsEnabled ? 'normal' : 'unknown'}
                />
              </div>
              <div className="detail-item">
                <span className="detail-label">代理</span>
                <StatusTag
                  text={selectedMapping.proxyEnabled ? '启用' : '关闭'}
                  variant={selectedMapping.proxyEnabled ? 'normal' : 'unknown'}
                />
              </div>
              <div className="detail-item">
                <span className="detail-label">面板网站 ID</span>
                <span className="mono">{selectedMapping.panelWebsiteId || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">覆盖状态</span>
                <StatusTag
                  text={selectedMapping.isOverridden ? '已覆盖' : '正常'}
                  variant={selectedMapping.isOverridden ? 'warning' : 'normal'}
                />
              </div>
              <div className="detail-item">
                <span className="detail-label">覆盖来源</span>
                <span>{selectedMapping.overrideSource || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">最后修改人</span>
                <span>{selectedMapping.overrideLastModifiedBy || '—'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">最后同步</span>
                <span>{formatDate(selectedMapping.lastSyncedAt)}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">创建时间</span>
                <span>{formatDate(selectedMapping.createdAt)}</span>
              </div>
            </div>
          </div>
        ) : (
          <EmptyState title="未选择映射" description="请选择一条网站映射记录查看详情。" />
        )}
      </DetailDrawer>
    </div>
  )
}
