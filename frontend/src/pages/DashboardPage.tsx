import { useEffect, useState } from 'react'
import { getDashboardStats, getRecentActivity, type DashboardStats, type RecentActivity } from '../api'
import StatusTag from '../components/StatusTag'
import EmptyState from '../components/EmptyState'

function DashboardCard({
  title,
  count,
  children,
  emptyText,
}: {
  title: string
  count?: number
  children: React.ReactNode
  emptyText?: string
}) {
  return (
    <div className="dashboard-card">
      <div className="dashboard-card-header">
        <h3>{title}</h3>
        {count !== undefined && <span className="dashboard-card-count">{count}</span>}
      </div>
      <div className="dashboard-card-body">
        {children || <p className="dashboard-card-empty">{emptyText}</p>}
      </div>
    </div>
  )
}

function StatRow({ label, value, variant }: { label: string; value: number; variant?: string }) {
  return (
    <div className="stat-row">
      <span className="stat-label">{label}</span>
      <span className={`stat-value ${variant || ''}`}>{value}</span>
    </div>
  )
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [activity, setActivity] = useState<RecentActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchData = async () => {
    try {
      setError(null)
      const [statsData, activityData] = await Promise.all([
        getDashboardStats().catch(() => null),
        getRecentActivity().catch(() => []),
      ])
      setStats(statsData)
      setActivity(activityData)
    } catch {
      setError('加载仪表盘数据失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  if (loading) {
    return (
      <div>
        <div className="page-header">
          <h1>仪表盘</h1>
          <p>全局健康概览与待处理事项</p>
        </div>
        <div className="dashboard-loading">加载中...</div>
      </div>
    )
  }

  if (!stats) {
    return (
      <div>
        <div className="page-header">
          <h1>仪表盘</h1>
          <p>全局健康概览与待处理事项</p>
        </div>
        <EmptyState
          title="暂无数据"
          description="后端 API 尚未就绪或返回空数据。请确认后端服务已启动。"
          primaryAction={{ label: '重试', onClick: fetchData }}
        />
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>仪表盘</h1>
        <p>全局健康概览与待处理事项</p>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="dashboard-grid">
        {/* Nodes */}
        <DashboardCard title="节点健康" count={stats.nodes.total}>
          <StatRow label="在线" value={stats.nodes.online} variant="normal" />
          <StatRow label="降级" value={stats.nodes.degraded} variant="warning" />
          <StatRow label="离线" value={stats.nodes.offline} variant="error" />
          <StatRow label="封禁" value={stats.nodes.banned} variant="error" />
        </DashboardCard>

        {/* Tunnels */}
        <DashboardCard title="隧道状态" count={stats.tunnels.total}>
          <StatRow label="已启用" value={stats.tunnels.enabled} variant="normal" />
          <StatRow label="待应用" value={stats.tunnels.pending} variant="warning" />
          <StatRow label="冲突" value={stats.tunnels.conflict} variant="error" />
          <StatRow label="手动覆盖" value={stats.tunnels.overridden} variant="warning" />
          <StatRow label="失败" value={stats.tunnels.failed} variant="error" />
        </DashboardCard>

        {/* Websites */}
        <DashboardCard title="网站映射" count={stats.websites.total}>
          <StatRow label="已同步" value={stats.websites.synced} variant="normal" />
          <StatRow label="待同步" value={stats.websites.pending} variant="warning" />
          <StatRow label="HTTPS 异常" value={stats.websites.httpsError} variant="error" />
          <StatRow label="手动覆盖" value={stats.websites.overridden} variant="warning" />
        </DashboardCard>

        {/* Jobs */}
        <DashboardCard title="任务队列">
          <StatRow label="等待中" value={stats.jobs.queued} variant="info" />
          <StatRow label="运行中" value={stats.jobs.running} variant="info" />
          <StatRow label="失败" value={stats.jobs.failed} variant="error" />
          <StatRow label="卡住" value={stats.jobs.stalled} variant="error" />
          <StatRow label="已完成" value={stats.jobs.completed} variant="normal" />
        </DashboardCard>

        {/* Sync */}
        <DashboardCard title="同步新鲜度">
          <div className="sync-info">
            <div className="sync-row">
              <span>最近成功同步</span>
              <span>{stats.sync.lastSuccessAt ? new Date(stats.sync.lastSuccessAt).toLocaleString() : '从未'}</span>
            </div>
            <div className="sync-row">
              <span>下次计划检查</span>
              <span>{stats.sync.nextScheduledAt ? new Date(stats.sync.nextScheduledAt).toLocaleString() : '未计划'}</span>
            </div>
            <div className="sync-row">
              <span>数据状态</span>
              <StatusTag text={stats.sync.isStale ? '已过期' : '最新'} />
            </div>
          </div>
        </DashboardCard>

        {/* Alerts placeholder */}
        <DashboardCard title="异常与告警" emptyText="当前无未关闭告警">
          <p className="dashboard-card-empty">当前无未关闭告警</p>
        </DashboardCard>
      </div>

      {/* Recent Activity */}
      <div className="dashboard-activity">
        <h2>最近动态</h2>
        {activity.length === 0 ? (
          <p className="activity-empty">暂无最近动态</p>
        ) : (
          <ul className="activity-list">
            {activity.map((item) => (
              <li key={item.id} className="activity-item">
                <div className="activity-meta">
                  <StatusTag text={item.status} />
                  <span className="activity-time">{new Date(item.createdAt).toLocaleString()}</span>
                </div>
                <div className="activity-title">{item.title}</div>
                <div className="activity-desc">{item.description}</div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
