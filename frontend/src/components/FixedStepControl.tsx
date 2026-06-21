/**
 * FixedStepControl — 固定刻度按钮组
 *
 * 所有参与自动化决策的数值都必须用固定刻度按钮或分段控件表达，
 * 不使用连续滑块（因为滑块掩盖边界值，用户难以在审计和协作时准确复述设置）。
 *
 * 若必须支持自由输入，只能放在"高级"折叠区，并通过 showCustom 启用。
 */

import { useState } from 'react'

export interface FixedStepOption<T extends string | number = string> {
  value: T
  label: string
}

interface FixedStepControlProps<T extends string | number = string> {
  label: string
  description?: string
  options: FixedStepOption<T>[]
  value: T
  onChange: (value: T) => void
  /** 显示当前值单位，放在选中按钮之后 */
  unit?: string
  /** 是否允许自定义（高级）输入 */
  showCustom?: boolean
  /** 自定义输入时的占位提示 */
  customPlaceholder?: string
  /** 推荐档位展示在自定义输入旁边 */
  recommendations?: string[]
  disabled?: boolean
}

export default function FixedStepControl<T extends string | number = string>({
  label,
  description,
  options,
  value,
  onChange,
  unit,
  showCustom = false,
  customPlaceholder,
  recommendations,
  disabled = false,
}: FixedStepControlProps<T>) {
  const [customOpen, setCustomOpen] = useState(false)
  const [customValue, setCustomValue] = useState('')

  const handleCustomSubmit = () => {
    const cleaned = customValue.trim()
    if (!cleaned) return
    // If T extends number, parse as number; otherwise keep as string
    const parsed = typeof options[0]?.value === 'number'
      ? (Number(cleaned) as unknown as T)
      : (cleaned as unknown as T)
    if (typeof parsed === 'number' && isNaN(parsed)) return
    onChange(parsed)
    setCustomValue('')
    setCustomOpen(false)
  }

  const isSelected = (optValue: T) => String(optValue) === String(value)

  return (
    <div className="fixed-step-control">
      <div className="fixed-step-label">
        <span className="fixed-step-title">{label}</span>
        {unit && <span className="fixed-step-unit">（{unit}）</span>}
      </div>
      {description && <p className="fixed-step-desc">{description}</p>}
      <div className="fixed-step-options">
        {options.map((opt) => (
          <button
            key={String(opt.value)}
            className={`fixed-step-btn ${isSelected(opt.value) ? 'active' : ''}`}
            onClick={() => onChange(opt.value)}
            disabled={disabled}
            type="button"
          >
            {opt.label}
          </button>
        ))}
        {showCustom && (
          <button
            className={`fixed-step-btn fixed-step-custom-toggle ${customOpen || isSelected(value as unknown as T) ? 'active' : ''}`}
            onClick={() => setCustomOpen(!customOpen)}
            disabled={disabled}
            type="button"
          >
            自定义
          </button>
        )}
      </div>
      {showCustom && customOpen && (
        <div className="fixed-step-custom">
          <input
            type="text"
            className="fixed-step-input"
            placeholder={customPlaceholder ?? '输入自定义值'}
            value={customValue}
            onChange={(e) => setCustomValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleCustomSubmit()
              if (e.key === 'Escape') setCustomOpen(false)
            }}
          />
          <button
            className="btn-primary btn-sm"
            onClick={handleCustomSubmit}
            type="button"
          >
            应用
          </button>
          {recommendations && recommendations.length > 0 && (
            <span className="fixed-step-recommendations">
              推荐：{recommendations.join(' / ')}
            </span>
          )}
        </div>
      )}
    </div>
  )
}