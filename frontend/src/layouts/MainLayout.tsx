import { Routes, Route, Navigate } from 'react-router-dom'
import { useContext } from 'react'
import { GlobalStateContext } from '../hooks/useGlobalState'
import TopBar from '../components/TopBar'
import Sidebar from '../components/Sidebar'
import DashboardPage from '../pages/DashboardPage'
import NodesPage from '../pages/NodesPage'
import TunnelsPage from '../pages/TunnelsPage'
import WebsiteMappingsPage from '../pages/WebsiteMappingsPage'
import JobsPage from '../pages/JobsPage'
import LogsPage from '../pages/LogsPage'
import SettingsPage from '../pages/SettingsPage'

function LayoutContent() {
  const ctx = useContext(GlobalStateContext)
  const collapsed = ctx?.state.sidebarCollapsed ?? false

  return (
    <>
      <Sidebar />
      <div className={`main-container ${collapsed ? 'ml-0' : 'ml-sidebar'}`}>
        <TopBar />
        <main className="page-content">
          <Routes>
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/nodes" element={<NodesPage />} />
            <Route path="/tunnels" element={<TunnelsPage />} />
            <Route path="/website-mappings" element={<WebsiteMappingsPage />} />
            <Route path="/jobs" element={<JobsPage />} />
            <Route path="/logs" element={<LogsPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </main>
      </div>
    </>
  )
}

export default function MainLayout() {
  return (
    <div className="app-layout">
      <LayoutContent />
    </div>
  )
}