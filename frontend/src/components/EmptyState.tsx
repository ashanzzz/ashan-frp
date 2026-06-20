interface EmptyStateProps {
  icon?: string
  title: string
  description?: string
  primaryAction?: {
    label: string
    onClick: () => void
  }
  secondaryAction?: {
    label: string
    onClick: () => void
  }
}

export default function EmptyState({
  icon = '📭',
  title,
  description = '',
  primaryAction,
  secondaryAction,
}: EmptyStateProps) {
  return (
    <div className="empty-state">
      <div className="empty-state-icon">{icon}</div>
      <h3 className="empty-state-title">{title}</h3>
      {description && <p className="empty-state-desc">{description}</p>}
      {(primaryAction || secondaryAction) && (
        <div className="empty-state-actions">
          {primaryAction && (
            <button className="btn-primary" onClick={primaryAction.onClick}>
              {primaryAction.label}
            </button>
          )}
          {secondaryAction && (
            <button className="btn-secondary" onClick={secondaryAction.onClick}>
              {secondaryAction.label}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
