import type { NodeStatus, TunnelStatus, WebsiteStatus, JobStatus } from '../api'

type StatusVariant = 'normal' | 'warning' | 'error' | 'info' | 'unknown'

interface StatusTagProps {
  text: string
  variant?: StatusVariant
}

const variantMap: Record<string, StatusVariant> = {
  // Node statuses
  online: 'normal',
  degraded: 'warning',
  offline: 'error',
  banned: 'error',
  archived: 'unknown',
  // Tunnel statuses
  enabled: 'normal',
  pending: 'info',
  conflict: 'error',
  overridden: 'warning',
  failed: 'error',
  // Website statuses
  synced: 'normal',
  'https_error': 'error',
  // Job statuses
  queued: 'info',
  running: 'info',
  retry_wait: 'warning',
  blocked: 'warning',
  succeeded: 'normal',
  canceled: 'unknown',
  stalled: 'error',
}

export default function StatusTag({ text, variant }: StatusTagProps) {
  const resolved = variant ?? variantMap[text.toLowerCase()] ?? 'unknown'
  return <span className={`status-tag ${resolved}`}>{text}</span>
}

// Helper to get variant from typed status enums
export function getStatusVariant(status: NodeStatus | TunnelStatus | WebsiteStatus | JobStatus | string): StatusVariant {
  return variantMap[status.toLowerCase()] ?? 'unknown'
}
