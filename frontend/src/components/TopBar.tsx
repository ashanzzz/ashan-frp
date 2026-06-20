import { type Location, useLocation } from 'react-router-dom'
import { useGlobalState } from '../hooks/useGlobalState'
import { type PageName } from '../hooks/useGlobalState'

const pageTitles: Record<PageName, { title: string; desc: string }> = {
  dashboard: { title: '仪表盘', desc: '全局健康概览与待处理事项' },
  nodes: { title: '节点列表', desc: '管理上游节点与健康状态' },
  tunnels: { title: '隧道列表', desc: '管理固定隧道与运行态差异' },
  'website-mappings': { title: '网站映射', desc: '管理域名映射与 HTTPS 配置' },
  jobs: { title: '任务队列', desc: '观察排队、重试与执行状态' },
  logs: { title: '日志', desc: '查看运行日志与审计记录' },
  settings: { title: '设置', desc: '管理频率、阈值与通知策略' },
}

function getPageFromPath(path: string): PageName {
  if (path.startsWith('/dashboard')) return 'dashboard'
  if (path.startsWith('/nodes')) return 'nodes'
  if (path.startsWith('/tunnels')) return 'tunnels'
  if (path.startsWith('/website-mappings')) return 'website-mappings'
  if (path.startsWith('/jobs')) return 'jobs'
  if (path.startsWith('/logs')) return 'logs'
  if (path.startsWith('/settings')) return 'settings'
  return 'dashboard'
}

function getStatusDotClass(status: string) {
  switch (status) {
    case 'connected':
      return 'sse-indicator connected'
    case 'polling':
      return 'sse-indicator polling'
    default:
      return 'sse-indicator disconnected'
  }
}

function getStatusText(status: string) {
  switch (status) {
    case 'connected':
      return '实时接收'
    case 'polling':
      return '轮询中'
    default:
      return '已断开'
  }
}

export default function TopBar() {
  const { state, toggleSidebar, touchRefresh } = useGlobalState()
  const location: Location = useLocation()
  const page = getPageFromPath(location.pathname)
  const info = pageTitles[page] ?? { title: '管理控制台', desc: '' }

  const handleRefresh = () => {
    touchRefresh()
  }

  return (
    <header className="topbar">
      <div className="topbar-left">
        <button className="topbar-toggle" onClick={toggleSidebar} aria-label="切换侧栏">
          ☰
        </button>
        <div className="topbar-title">
          <h2>{info.title}</h2>
        </div>
      </div>
      <div className="topbar-right">
        <div className={getStatusDotClass(state.sseStatus)}>
          <span className="sse-dot" />
          <span>{getStatusText(state.sseStatus)}</span>
        </div>
        <button className="btn-secondary" onClick={handleRefresh}>
          刷新
        </button>
        <div className="topbar-user">
          <span style={{ fontSize: 14, color: '#333' }}>👤 admin</span>
        </div>
      </div>
    </header>
  )
}
