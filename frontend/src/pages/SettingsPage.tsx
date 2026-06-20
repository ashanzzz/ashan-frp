import EmptyState from '../components/EmptyState'

export default function SettingsPage() {
  return (
    <div>
      <div className="page-header">
        <h1>设置</h1>
        <p>管理频率、阈值与通知策略</p>
      </div>
      <EmptyState
        title="设置模块开发中"
        description="此处将包含同步策略、频率与阈值、通知与告警、凭据与集成、危险操作等功能分区。"
      />
    </div>
  )
}