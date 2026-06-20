type StatusVariant = 'normal' | 'warning' | 'error' | 'info' | 'unknown'

interface StatusTagProps {
  text: string
  variant?: StatusVariant
}

export default function StatusTag({ text, variant = 'unknown' }: StatusTagProps) {
  return <span className={`status-tag ${variant}`}>{text}</span>
}
