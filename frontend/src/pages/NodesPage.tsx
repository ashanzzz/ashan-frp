import EmptyState from '../components/EmptyState'

export default function NodesPage() {
  return (
    <div>
      <div className="page-header">
        <h1>节点列表</h1>
        <p>管理上游节点与健康状态</p>
      </div>
      <EmptyState
        title="还没有节点"
        description="先添加一个节点，系统才能做健康检查和自动同步。"
        primaryAction={{ label: '添加节点', onClick: () => {} }}
        secondaryAction={{ label: '查看接入说明', onClick: () => {} }}
      />
    </div>
  )
}