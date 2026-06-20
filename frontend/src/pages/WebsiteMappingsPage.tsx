import EmptyState from '../components/EmptyState'

export default function WebsiteMappingsPage() {
  return (
    <div>
      <div className="page-header">
        <h1>网站映射</h1>
        <p>管理域名映射与 HTTPS 配置</p>
      </div>
      <EmptyState
        title="还没有网站映射"
        description="先把节点和隧道连起来，再创建域名映射。"
        primaryAction={{ label: '新建映射', onClick: () => {} }}
      />
    </div>
  )
}