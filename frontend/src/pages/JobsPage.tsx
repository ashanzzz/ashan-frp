import EmptyState from '../components/EmptyState'

export default function JobsPage() {
  return (
    <div>
      <div className="page-header">
        <h1>任务队列</h1>
        <p>观察排队、重试与执行状态</p>
      </div>
      <EmptyState
        title="当前没有待执行任务"
        description="当系统没有冲突、同步或重试需要时，这里会保持为空。"
      />
    </div>
  )
}