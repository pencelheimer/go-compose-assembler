import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Monitor, Moon, Sun } from 'lucide-react'
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

function RootLayout() {
  // Re-render on navigation so the builder link carries its latest state.
  useRouterState({ select: (s) => s.location.pathname })

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
        <ThemeToggle />
      </header>
      <Outlet />
    </div>
  )
}
