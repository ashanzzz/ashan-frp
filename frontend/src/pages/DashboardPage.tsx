import EmptyState from '../components/EmptyState'

export default function DashboardPage() {
  return (
    <div>
      <div className="page-header">
        <h1>仪表盘</h1>
        <p>全局健康概览与待处理事项</p>
      </div>
      <EmptyState
        title="仪表盘数据模块开发中"
        description="此处将展示系统健康、节点状态、隧道差异、任务队列等卡片视图，待后端就绪后接入。"
      />
    </div>
  )
}