export function cloudflareConfigureFailureMessage(status, apiError = {}) {
  if (status === 401 && apiError?.code === 'UNAUTHORIZED') {
    return '当前登录会话已失效；该请求未发送到 Cloudflare。请重新登录后再试。';
  }

  return `Cloudflare 配置失败: ${apiError?.message || '请求失败'}`;
}
