import { useState, useCallback, useRef, useEffect } from 'react'

export type SSEStatus = 'connecting' | 'connected' | 'polling' | 'disconnected'

interface SSEState {
  status: SSEStatus
  lastMessageAt: Date | null
  lastRefreshAt: Date | null
}

interface UseSSEOptions {
  /** SSE 端点 URL，若不可用则自动退回到 polling */
  url: string
  /** 轮询间隔（毫秒），默认 5000 */
  pollingInterval?: number
  /** 收到消息时的回调 */
  onMessage?: (data: unknown) => void
  /** 重连最大尝试次数，默认 5 */
  maxReconnectAttempts?: number
}

export function useSSE({
  url,
  pollingInterval = 5000,
  onMessage,
  maxReconnectAttempts = 5,
}: UseSSEOptions) {
  const [state, setState] = useState<SSEState>({
    status: 'connecting',
    lastMessageAt: null,
    lastRefreshAt: null,
  })

  const reconnectRef = useRef(0)
  const esRef = useRef<EventSource | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  // 辅助：更新 lastRefreshAt
  const touchRefresh = useCallback(() => {
    setState((prev) => ({ ...prev, lastRefreshAt: new Date() }))
  }, [])

  // 启动轮询
  const startPolling = useCallback(() => {
    if (pollTimerRef.current) return
    touchRefresh()
    pollTimerRef.current = setInterval(() => {
      touchRefresh()
    }, pollingInterval)
  }, [pollingInterval, touchRefresh])

  // 停止轮询
  const stopPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }, [])

  // 连接到 SSE
  const connect = useCallback(() => {
    // 清理旧的
    stopPolling()
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }

    setState((prev) => ({
      ...prev,
      status: 'connecting',
    }))

    try {
      const es = new EventSource(url)
      esRef.current = es

      es.onopen = () => {
        reconnectRef.current = 0
        setState((prev) => ({
          ...prev,
          status: 'connected',
          lastMessageAt: new Date(),
        }))
      }

      es.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          onMessage?.(data)
        } catch {
          onMessage?.(event.data)
        }
        setState((prev) => ({ ...prev, lastMessageAt: new Date() }))
      }

      es.onerror = () => {
        es.close()
        esRef.current = null
        const attempts = reconnectRef.current
        if (attempts < maxReconnectAttempts) {
          reconnectRef.current += 1
          setState((prev) => ({
            ...prev,
            status: 'polling',
          }))
          startPolling()
        } else {
          setState((prev) => ({
            ...prev,
            status: 'disconnected',
          }))
        }
      }
    } catch {
      // SSE 不受支持，回退到 polling
      setState((prev) => ({ ...prev, status: 'polling' }))
      startPolling()
    }
  }, [url, onMessage, maxReconnectAttempts, startPolling, stopPolling])

  // 手动刷新
  const refresh = useCallback(() => {
    if (state.status === 'connected') {
      // 通过 SSE 已连接，刷新时间戳即可
      touchRefresh()
    } else {
      // polling 模式下触发一次获取
      touchRefresh()
    }
  }, [state.status, touchRefresh])

  useEffect(() => {
    connect()
    return () => {
      stopPolling()
      if (esRef.current) {
        esRef.current.close()
        esRef.current = null
      }
      if (abortRef.current) {
        abortRef.current.abort()
        abortRef.current = null
      }
    }
  }, [connect, stopPolling])

  return { state, refresh, reconnect: connect }
}
