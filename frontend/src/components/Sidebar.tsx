import { NavLink } from 'react-router-dom'
import { useGlobalState } from '../hooks/useGlobalState'

interface NavItem {
  to: string
  icon: string
  label: string
}

const navGroups: { group: string; items: NavItem[] }[] = [
  {
    group: '概览',
    items: [{ to: '/dashboard', icon: '📊', label: '仪表盘' }],
  },
  {
    group: '资源',
    items: [
      { to: '/nodes', icon: '🖥️', label: '节点列表' },
      { to: '/tunnels', icon: '🔒', label: '隧道列表' },
      { to: '/website-mappings', icon: '🌐', label: '网站映射' },
    ],
  },
  {
    group: '运营',
    items: [
      { to: '/jobs', icon: '⚙️', label: '任务队列' },
      { to: '/logs', icon: '📋', label: '日志' },
    ],
  },
  {
    group: '系统',
    items: [{ to: '/settings', icon: '🔧', label: '设置' }],
  },
]

export default function Sidebar() {
  const { state, toggleSidebar } = useGlobalState()

  return (
    <aside className={`sidebar ${state.sidebarCollapsed ? 'collapsed' : ''}`}>
      <div className="sidebar-header">
        <h1>🚀 Ashan Frp</h1>
      </div>
      <nav className="sidebar-nav">
        {navGroups.map((g) => (
          <div key={g.group}>
            <div className="sidebar-group">{g.group}</div>
            {g.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `sidebar-link${isActive ? ' active' : ''}`
                }
                onClick={() => {
                  if (window.innerWidth < 768) toggleSidebar()
                }}
              >
                <span>{item.icon}</span>
                <span>{item.label}</span>
              </NavLink>
            ))}
          </div>
        ))}
      </nav>
    </aside>
  )
}
