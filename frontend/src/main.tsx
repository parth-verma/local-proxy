import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './app.css'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '@/lib/query-client'

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>  
    <App />
    </QueryClientProvider>
  </React.StrictMode>,
)
