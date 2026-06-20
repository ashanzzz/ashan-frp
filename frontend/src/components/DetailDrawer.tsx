import { ReactNode } from 'react'

interface DetailDrawerProps {
  isOpen: boolean
  onClose: () => void
  title: string
  children: ReactNode
}

export default function DetailDrawer({ isOpen, onClose, title, children }: DetailDrawerProps) {
  if (!isOpen) return null

  return (
    <>
      <div className="drawer-overlay" onClick={onClose} />
      <div className="drawer-panel">
        <div className="drawer-header">
          <h3>{title}</h3>
          <button className="drawer-close" onClick={onClose} aria-label="关闭">
            ×
          </button>
        </div>
        <div className="drawer-body">{children}</div>
      </div>
    </>
  )
}
