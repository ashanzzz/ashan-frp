import { useState, useCallback, createContext, useContext, type ReactNode } from 'react'

export type PageName =
  | 'dashboard'
  | 'nodes'
  | 'tunnels'
  | 'website-mappings'
  | 'jobs'
  | 'logs'
  | 'settings'

interface GlobalState {
  currentPage: PageName
  sidebarCollapsed: boolean
  sseStatus: 'connected' | 'polling' | 'disconnected'
  lastRefreshAt: Date | null
}

interface GlobalStateCtx {
  state: GlobalState
  setPage: (page: PageName) => void
  toggleSidebar: () => void
  setSSEStatus: (status: GlobalState['sseStatus']) => void
  touchRefresh: () => void
}

function useGlobalStateInternal(): GlobalStateCtx {
  const [state, setState] = useState<GlobalState>({
    currentPage: 'dashboard',
    sidebarCollapsed: false,
    sseStatus: 'disconnected',
    lastRefreshAt: null,
  })

  const setPage = useCallback((page: PageName) => {
    setState((prev) => ({ ...prev, currentPage: page }))
  }, [])

  const toggleSidebar = useCallback(() => {
    setState((prev) => ({ ...prev, sidebarCollapsed: !prev.sidebarCollapsed }))
  }, [])

  const setSSEStatus = useCallback((status: GlobalState['sseStatus']) => {
    setState((prev) => ({ ...prev, sseStatus: status }))
  }, [])

  const touchRefresh = useCallback(() => {
    setState((prev) => ({ ...prev, lastRefreshAt: new Date() }))
  }, [])

  return { state, setPage, toggleSidebar, setSSEStatus, touchRefresh }
}

export const GlobalStateContext = createContext<GlobalStateCtx | null>(null)

export function GlobalStateProvider({ children }: { children: ReactNode }) {
  const value = useGlobalStateInternal()
  return <GlobalStateContext.Provider value={value}>{children}</GlobalStateContext.Provider>
}

export function useGlobalState(): GlobalStateCtx {
  const ctx = useContext(GlobalStateContext)
  if (!ctx) throw new Error('useGlobalState must be used within GlobalStateProvider')
  return ctx
}
