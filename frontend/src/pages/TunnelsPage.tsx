import EmptyState from '../components/EmptyState'

export default function TunnelsPage() {
  return (
    <div>
      <div className="page-header">
        <h1>隧道列表</h1>
        <p>管理固定隧道与运行态差异</p>
      </div>
      <EmptyState
        title="还没有隧道"
        description="创建隧道后，系统才能把本地服务映射到远端。"
        primaryAction={{ label: '新建隧道', onClick: () => {} }}
      />
    </div>
  )
}