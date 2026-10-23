import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Monitor, Moon, RefreshCw, Sun } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useLiveUpdates, type Changed, type LiveStatus } from '@/lib/live'
import { useTheme } from '@/lib/theme'
import { builderSearch } from '@/lib/useUrlValues'

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  notFoundComponent: () => (
    <div className="page narrow">
      <h1>Page not found</h1>
      <p className="muted">
        <Link to="/">Back to the builder</Link>
      </p>
    </div>
  ),
})

function Logo() {
  return (
    <svg viewBox="0 0 32 32" width="22" height="22" aria-hidden>
      <rect width="32" height="32" rx="8" className="logo-bg" />
      <g fill="none" stroke="#fff" strokeWidth="2.2" strokeLinejoin="round">
        <rect x="7" y="7" width="8" height="8" rx="1.5" />
        <rect x="17" y="7" width="8" height="8" rx="1.5" opacity=".55" />
        <rect x="7" y="17" width="8" height="8" rx="1.5" opacity=".55" />
        <rect x="17" y="17" width="8" height="8" rx="1.5" />
      </g>
    </svg>
  )
}

function ThemeToggle() {
  const { pref, cycle } = useTheme()
  const Icon = pref === 'light' ? Sun : pref === 'dark' ? Moon : Monitor
  return (
    <button type="button" className="icon-btn" onClick={cycle} title={`Theme: ${pref} (click to change)`}>
      <Icon size={16} />
      <span className="sr-only">Theme: {pref}</span>
    </button>
  )
}

const liveText: Record<LiveStatus, { label: string; title: string }> = {
  live: { label: 'Live', title: 'Watching definition files: edits on disk show up here immediately' },
  connecting: { label: 'Reconnecting', title: 'Lost the connection to the server; retrying' },
  offline: { label: 'Offline', title: 'Live updates are unavailable; switching back to this tab still refreshes' },
}

function LiveIndicator({ status }: { status: LiveStatus }) {
  return (
    <span className="live" data-status={status} title={liveText[status].title} role="status">
      <span className="live-dot" aria-hidden />
      {liveText[status].label}
    </span>
  )
}

function useNotice() {
  const [notice, setNotice] = useState<{ text: string; key: number }>()
  useEffect(() => {
    if (!notice) return
    const t = setTimeout(() => setNotice(undefined), 3500)
    return () => clearTimeout(t)
  }, [notice])
  const show = (what: Changed) =>
    setNotice({ text: `${what === 'components' ? 'Components' : 'Presets'} changed on disk — refreshed`, key: Date.now() })
  return { notice, show }
}

function RootLayout() {
  // Re-render on navigation so the builder link carries its latest state.
  useRouterState({ select: (s) => s.location.pathname })
  const { notice, show } = useNotice()
  const live = useLiveUpdates(show)

  return (
    <div className="app">
      <header className="topbar">
        <Link to="/" search={builderSearch.get()} className="brand">
          <Logo />
          <span>Compose Assembler</span>
        </Link>
        <nav className="topnav" aria-label="Main">
          <Link to="/" search={builderSearch.get()} activeOptions={{ exact: true, includeSearch: false }}>
            Builder
          </Link>
          <Link to="/catalog">Catalog</Link>
        </nav>
        <span className="spacer" />
        <LiveIndicator status={live} />
        <ThemeToggle />
      </header>
      <Outlet />
      {notice && (
        <div key={notice.key} className="toast" role="status">
          <RefreshCw size={14} aria-hidden /> {notice.text}
        </div>
      )}
    </div>
  )
}
