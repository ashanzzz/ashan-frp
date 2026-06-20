import EmptyState from '../components/EmptyState'

export default function LogsPage() {
  return (
    <div>
      <div className="page-header">
        <h1>日志</h1>
        <p>查看运行日志与审计记录</p>
      </div>
      <EmptyState
        title="筛选范围内暂无日志"
        description="当前筛选条件内没有匹配的日志条目。若刚初始化系统请等待首次同步完成后重试。"
        secondaryAction={{ label: '清空筛选条件', onClick: () => {} }}
      />
    </div>
  )
}