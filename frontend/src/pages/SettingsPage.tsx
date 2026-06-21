import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  clearCompletedJobs,
  getSettings,
  resetLocalCache,
  revokeUpstreams,
  updateSettings,
  validateCredentials,
  type SettingsSummary,
} from '../api'
import FixedStepControl from '../components/FixedStepControl'

const HEALTH_CHECK_OPTIONS = [
  { value: 30, label: '30 sec' },
  { value: 60, label: '1 min' },
  { value: 180, label: '3 min' },
  { value: 300, label: '5 min' },
  { value: 600, label: '10 min' },
]

const QUEUE_POLL_OPTIONS = [
  { value: 5, label: '5 sec' },
  { value: 10, label: '10 sec' },
  { value: 30, label: '30 sec' },
  { value: 60, label: '1 min' },
]

const RETRY_BACKOFF_OPTIONS = [
  { value: 15, label: '15 sec' },
  { value: 30, label: '30 sec' },
  { value: 60, label: '1 min' },
  { value: 300, label: '5 min' },
  { value: 900, label: '15 min' },
]

const LOG_ROWS_OPTIONS = [
  { value: 50, label: '50 rows' },
  { value: 100, label: '100 rows' },
  { value: 200, label: '200 rows' },
  { value: 500, label: '500 rows' },
]

const DATA_RETENTION_OPTIONS = [
  { value: 7, label: '7 days' },
  { value: 30, label: '30 days' },
  { value: 90, label: '90 days' },
  { value: 0, label: 'forever' },
]

const ALERT_THRESHOLD_OPTIONS = [
  { value: 5, label: '5 min' },
  { value: 10, label: '10 min' },
  { value: 30, label: '30 min' },
  { value: 60, label: '1 hr' },
]

const DEFAULT_SETTINGS: SettingsSummary = {
  general: {
    systemName: 'Ashan FRP',
    timezone: 'Asia/Shanghai',
    language: 'zh-CN',
    autoRefresh: true,
  },
  syncStrategy: {
    syncInterval: 300,
    syncJitter: 60,
    healthInterval: 60,
    healthJitter: 15,
    fastestCooldown: 900,
  },
  rateAndThreshold: {
    healthCheckInterval: 60,
    queuePollInterval: 10,
    retryBackoff: 15,
    logDefaultRows: 100,
    dataRetention: 30,
  },
  notifications: {
    emailEnabled: false,
    emailRecipients: [],
    webhookEnabled: false,
    webhookUrl: '',
    alertOnStalled: true,
    alertOnFailed: true,
    alertThresholdMinutes: 10,
  },
  credentials: {
    lastVerifiedAt: null,
    lastError: null,
  },
}

function cloneSettings(value: SettingsSummary): SettingsSummary {
  return {
    general: { ...value.general },
    syncStrategy: { ...value.syncStrategy },
    rateAndThreshold: { ...value.rateAndThreshold },
    notifications: {
      ...value.notifications,
      emailRecipients: [...value.notifications.emailRecipients],
    },
    credentials: { ...value.credentials },
  }
}

function mergeSettings(value?: Partial<SettingsSummary> | null): SettingsSummary {
  const base = cloneSettings(DEFAULT_SETTINGS)
  if (!value) return base

  return {
    general: { ...base.general, ...(value.general ?? {}) },
    syncStrategy: { ...base.syncStrategy, ...(value.syncStrategy ?? {}) },
    rateAndThreshold: { ...base.rateAndThreshold, ...(value.rateAndThreshold ?? {}) },
    notifications: {
      ...base.notifications,
      ...(value.notifications ?? {}),
      emailRecipients: value.notifications?.emailRecipients
        ? [...value.notifications.emailRecipients]
        : [...base.notifications.emailRecipients],
    },
    credentials: { ...base.credentials, ...(value.credentials ?? {}) },
  }
}

function Banner({ kind, children }: { kind: 'error' | 'info'; children: React.ReactNode }) {
  return <div className={kind === 'error' ? 'error-banner' : 'info-banner'}>{children}</div>
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<SettingsSummary>(cloneSettings(DEFAULT_SETTINGS))
  const [baseline, setBaseline] = useState<SettingsSummary>(cloneSettings(DEFAULT_SETTINGS))
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [info, setInfo] = useState<string | null>(null)
  const [newEmail, setNewEmail] = useState('')

  useEffect(() => {
    let cancelled = false

    const load = async () => {
      try {
        const remote = await getSettings()
        if (cancelled) return
        const merged = mergeSettings(remote)
        setSettings(merged)
        setBaseline(cloneSettings(merged))
        setError(null)
      } catch {
        if (cancelled) return
        const fallback = cloneSettings(DEFAULT_SETTINGS)
        setSettings(fallback)
        setBaseline(cloneSettings(fallback))
        setError('加载设置失败，已使用本地默认值')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [])

  const dirty = useMemo(
    () => JSON.stringify(settings) !== JSON.stringify(baseline),
    [settings, baseline],
  )

  const updateGeneral = (patch: Partial<SettingsSummary['general']>) => {
    setSettings((prev) => ({ ...prev, general: { ...prev.general, ...patch } }))
  }

  const updateSyncStrategy = (patch: Partial<SettingsSummary['syncStrategy']>) => {
    setSettings((prev) => ({ ...prev, syncStrategy: { ...prev.syncStrategy, ...patch } }))
  }

  const updateThresholds = (patch: Partial<SettingsSummary['rateAndThreshold']>) => {
    setSettings((prev) => ({
      ...prev,
      rateAndThreshold: { ...prev.rateAndThreshold, ...patch },
    }))
  }

  const updateNotifications = (patch: Partial<SettingsSummary['notifications']>) => {
    setSettings((prev) => ({
      ...prev,
      notifications: { ...prev.notifications, ...patch },
    }))
  }

  const updateCredentials = (patch: Partial<SettingsSummary['credentials']>) => {
    setSettings((prev) => ({
      ...prev,
      credentials: { ...prev.credentials, ...patch },
    }))
  }

  const addEmail = useCallback(() => {
    const email = newEmail.trim()
    if (!email) return
    if (settings.notifications.emailRecipients.includes(email)) {
      setNewEmail('')
      return
    }
    updateNotifications({
      emailRecipients: [...settings.notifications.emailRecipients, email],
    })
    setNewEmail('')
  }, [newEmail, settings.notifications.emailRecipients])

  const removeEmail = (email: string) => {
    updateNotifications({
      emailRecipients: settings.notifications.emailRecipients.filter((item) => item !== email),
    })
  }

  const saveSettings = async () => {
    setSaving(true)
    setError(null)
    try {
      const saved = await updateSettings(settings)
      const merged = mergeSettings(saved)
      setSettings(merged)
      setBaseline(cloneSettings(merged))
      setInfo('设置已保存')
    } catch {
      setError('保存设置失败，请检查后端服务')
    } finally {
      setSaving(false)
    }
  }

  const discardChanges = () => {
    setSettings(cloneSettings(baseline))
    setNewEmail('')
    setInfo('已放弃未保存更改')
  }

  const handleValidateCredentials = async () => {
    try {
      const result = await validateCredentials()
      updateCredentials({
        lastVerifiedAt: new Date().toISOString(),
        lastError: result.valid ? null : result.message,
      })
      setInfo(result.message || (result.valid ? '凭据验证通过' : '凭据验证失败'))
    } catch {
      setError('重新验证失败')
    }
  }

  const handleClearCompleted = async () => {
    if (!window.confirm('确认清空所有已完成作业记录？')) return
    try {
      const result = await clearCompletedJobs()
      setInfo(`已清空 ${result.deleted} 条已完成作业记录`)
    } catch {
      setError('清空已完成作业失败')
    }
  }

  const handleResetCache = async () => {
    if (!window.confirm('确认重置本地缓存？')) return
    try {
      const result = await resetLocalCache()
      setInfo(result.message)
    } catch {
      setError('重置本地缓存失败')
    }
  }

  const handleRevoke = async () => {
    if (!window.confirm('确认吊销上游凭据？此操作不可逆。')) return
    try {
      const result = await revokeUpstreams()
      setInfo(result.message)
    } catch {
      setError('吊销上游凭据失败')
    }
  }

  if (loading) {
    return (
      <div>
        <div className="page-header">
          <h1>设置</h1>
          <p>管理频率、阈值与通知策略</p>
        </div>
        <div className="dashboard-loading">加载中...</div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>设置</h1>
        <p>管理频率、阈值、通知与危险操作</p>
      </div>

      {error && <Banner kind="error">{error}</Banner>}
      {!error && info && <Banner kind="info">{info}</Banner>}

      <section className="settings-section">
        <h2 className="settings-section-title">通用</h2>
        <div className="settings-row">
          <div className="settings-field">
            <label>系统名称</label>
            <input
              className="settings-input"
              value={settings.general.systemName}
              onChange={(e) => updateGeneral({ systemName: e.target.value })}
            />
          </div>
          <div className="settings-field">
            <label>时区</label>
            <select
              className="settings-select"
              value={settings.general.timezone}
              onChange={(e) => updateGeneral({ timezone: e.target.value })}
            >
              <option value="Asia/Shanghai">Asia/Shanghai</option>
              <option value="UTC">UTC</option>
              <option value="Asia/Singapore">Asia/Singapore</option>
              <option value="America/Los_Angeles">America/Los_Angeles</option>
            </select>
          </div>
          <div className="settings-field">
            <label>语言</label>
            <select
              className="settings-select"
              value={settings.general.language}
              onChange={(e) => updateGeneral({ language: e.target.value })}
            >
              <option value="zh-CN">简体中文</option>
              <option value="en-US">English</option>
            </select>
          </div>
          <div className="settings-field inline">
            <label>
              <input
                type="checkbox"
                checked={settings.general.autoRefresh}
                onChange={(e) => updateGeneral({ autoRefresh: e.target.checked })}
              />
              自动刷新
            </label>
          </div>
        </div>
      </section>

      <section className="settings-section">
        <h2 className="settings-section-title">同步策略</h2>
        <div className="settings-row">
          <FixedStepControl
            label="同步间隔"
            unit="秒"
            options={HEALTH_CHECK_OPTIONS}
            value={settings.syncStrategy.syncInterval}
            onChange={(value) => updateSyncStrategy({ syncInterval: value })}
          />
          <FixedStepControl
            label="同步抖动"
            unit="秒"
            options={QUEUE_POLL_OPTIONS}
            value={settings.syncStrategy.syncJitter}
            onChange={(value) => updateSyncStrategy({ syncJitter: value })}
          />
          <FixedStepControl
            label="健康检查间隔"
            unit="秒"
            options={HEALTH_CHECK_OPTIONS}
            value={settings.syncStrategy.healthInterval}
            onChange={(value) => updateSyncStrategy({ healthInterval: value })}
          />
          <FixedStepControl
            label="健康检查抖动"
            unit="秒"
            options={QUEUE_POLL_OPTIONS}
            value={settings.syncStrategy.healthJitter}
            onChange={(value) => updateSyncStrategy({ healthJitter: value })}
          />
          <FixedStepControl
            label="最快冷却"
            unit="秒"
            options={RETRY_BACKOFF_OPTIONS}
            value={settings.syncStrategy.fastestCooldown}
            onChange={(value) => updateSyncStrategy({ fastestCooldown: value })}
          />
        </div>
      </section>

      <section className="settings-section">
        <h2 className="settings-section-title">频率与阈值</h2>
        <div className="settings-row">
          <FixedStepControl
            label="健康检查间隔"
            unit="秒"
            options={HEALTH_CHECK_OPTIONS}
            value={settings.rateAndThreshold.healthCheckInterval}
            onChange={(value) => updateThresholds({ healthCheckInterval: value })}
          />
          <FixedStepControl
            label="队列轮询间隔"
            unit="秒"
            options={QUEUE_POLL_OPTIONS}
            value={settings.rateAndThreshold.queuePollInterval}
            onChange={(value) => updateThresholds({ queuePollInterval: value })}
          />
          <FixedStepControl
            label="重试退避"
            unit="秒"
            options={RETRY_BACKOFF_OPTIONS}
            value={settings.rateAndThreshold.retryBackoff}
            onChange={(value) => updateThresholds({ retryBackoff: value })}
          />
          <FixedStepControl
            label="日志默认条数"
            unit="条"
            options={LOG_ROWS_OPTIONS}
            value={settings.rateAndThreshold.logDefaultRows}
            onChange={(value) => updateThresholds({ logDefaultRows: value })}
          />
          <FixedStepControl
            label="数据保留期"
            unit="天"
            options={DATA_RETENTION_OPTIONS}
            value={settings.rateAndThreshold.dataRetention}
            onChange={(value) => updateThresholds({ dataRetention: value })}
          />
        </div>
      </section>

      <section className="settings-section">
        <h2 className="settings-section-title">通知与告警</h2>
        <div className="settings-row">
          <div className="settings-field inline">
            <label>
              <input
                type="checkbox"
                checked={settings.notifications.emailEnabled}
                onChange={(e) => updateNotifications({ emailEnabled: e.target.checked })}
              />
              启用邮件通知
            </label>
          </div>

          {settings.notifications.emailEnabled && (
            <div className="settings-field">
              <label>邮件收件人</label>
              <div className="email-list">
                {settings.notifications.emailRecipients.map((email) => (
                  <span key={email} className="email-tag">
                    {email}
                    <button className="email-remove" type="button" onClick={() => removeEmail(email)}>
                      ×
                    </button>
                  </span>
                ))}
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                <input
                  type="email"
                  className="settings-input"
                  placeholder="输入邮箱按回车添加"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      addEmail()
                    }
                  }}
                />
                <button className="btn-primary btn-sm" type="button" onClick={addEmail}>
                  添加
                </button>
              </div>
            </div>
          )}

          <div className="settings-field inline">
            <label>
              <input
                type="checkbox"
                checked={settings.notifications.webhookEnabled}
                onChange={(e) => updateNotifications({ webhookEnabled: e.target.checked })}
              />
              启用 Webhook
            </label>
          </div>

          {settings.notifications.webhookEnabled && (
            <div className="settings-field">
              <label>Webhook URL</label>
              <input
                type="url"
                className="settings-input"
                value={settings.notifications.webhookUrl}
                onChange={(e) => updateNotifications({ webhookUrl: e.target.value })}
                placeholder="https://..."
              />
            </div>
          )}

          <div className="settings-field inline">
            <label>
              <input
                type="checkbox"
                checked={settings.notifications.alertOnStalled}
                onChange={(e) => updateNotifications({ alertOnStalled: e.target.checked })}
              />
              任务卡住时告警
            </label>
          </div>

          <div className="settings-field inline">
            <label>
              <input
                type="checkbox"
                checked={settings.notifications.alertOnFailed}
                onChange={(e) => updateNotifications({ alertOnFailed: e.target.checked })}
              />
              任务失败时告警
            </label>
          </div>

          <FixedStepControl
            label="告警阈值"
            unit="分钟"
            options={ALERT_THRESHOLD_OPTIONS}
            value={settings.notifications.alertThresholdMinutes}
            onChange={(value) => updateNotifications({ alertThresholdMinutes: value })}
          />
        </div>
      </section>

      <section className="settings-section">
        <h2 className="settings-section-title">凭据与集成</h2>
        <div className="settings-row">
          <div className="settings-field">
            <label>凭据摘要</label>
            <div className="credential-card">
              <div className="credential-row">
                <span>最后验证时间</span>
                <span className="credential-value">
                  {settings.credentials.lastVerifiedAt
                    ? new Date(settings.credentials.lastVerifiedAt).toLocaleString()
                    : '未验证'}
                </span>
              </div>
              <div className="credential-row">
                <span>最近错误</span>
                <span className="credential-value error">{settings.credentials.lastError ?? '无'}</span>
              </div>
              <div className="credential-actions">
                <button className="btn-primary btn-sm" type="button" onClick={handleValidateCredentials}>
                  重新验证
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="settings-section danger">
        <h2 className="settings-section-title danger">危险操作</h2>
        <div className="settings-row danger-row">
          <div className="danger-item">
            <div className="danger-info">
              <h4>清空已完成作业</h4>
              <p>删除所有状态为已完成或已取消的作业记录。</p>
            </div>
            <button
              className="btn-secondary btn-sm btn-danger-text"
              type="button"
              onClick={handleClearCompleted}
            >
              清空
            </button>
          </div>

          <div className="danger-item">
            <div className="danger-info">
              <h4>重置本地缓存</h4>
              <p>清除前端状态缓存，下次访问时重新加载。</p>
            </div>
            <button
              className="btn-secondary btn-sm btn-danger-text"
              type="button"
              onClick={handleResetCache}
            >
              重置
            </button>
          </div>

          <div className="danger-item">
            <div className="danger-info">
              <h4>吊销上游凭据</h4>
              <p>使所有上游节点的连接凭据失效，需要重新配置。</p>
            </div>
            <button
              className="btn-secondary btn-sm btn-danger-text"
              type="button"
              onClick={handleRevoke}
            >
              吊销
            </button>
          </div>
        </div>
      </section>

      <div className="settings-footer">
        <div className="settings-footer-inner">
          {dirty ? (
            <span className="dirty-indicator">有未保存的更改</span>
          ) : (
            <span className="saved-indicator">所有设置已保存</span>
          )}
          <div className="settings-footer-actions">
            {dirty && (
              <button className="btn-secondary" type="button" onClick={discardChanges} disabled={saving}>
                取消
              </button>
            )}
            {dirty && (
              <button className="btn-primary" type="button" onClick={saveSettings} disabled={saving}>
                {saving ? '保存中...' : '保存设置'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
