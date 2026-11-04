import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { IconContext } from '@phosphor-icons/react'
import { App } from './App'
import { I18nProvider } from './i18n'
import { SessionProvider } from './session'
import './theme.css'
import './styles.css'
import './shell.css'
import './discovery.css'

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1, staleTime: 15_000, refetchOnWindowFocus: true }, mutations: { retry: false } } })

createRoot(document.getElementById('root')!).render(<StrictMode><IconContext.Provider value={{ weight: 'regular', 'aria-hidden': true, focusable: false }}><QueryClientProvider client={queryClient}><BrowserRouter><I18nProvider><SessionProvider><App /></SessionProvider></I18nProvider></BrowserRouter></QueryClientProvider></IconContext.Provider></StrictMode>)
