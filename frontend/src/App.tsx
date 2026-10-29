import { useState, useEffect } from 'react'
import Header from './components/Header'
import NamespaceMap from './components/NamespaceMap'
import Overview from './components/Overview'
import { healthCheck, getOverview, handleApiError } from './services/api'
import type { OverviewResponse } from './types/api-types'
import './styles/App.css'

function App() {
  const [isConnected, setIsConnected] = useState(false)
  const [lastCheck, setLastCheck] = useState<Date | null>(null)
  const [overviewData, setOverviewData] = useState<OverviewResponse | null>(null)
  const [overviewError, setOverviewError] = useState('')
  const [activeMenu, setActiveMenu] = useState('overview')
  const [refreshKey, setRefreshKey] = useState(0)

  const checkHealth = async () => {
    try {
      await healthCheck()
      setIsConnected(true)
      setLastCheck(new Date())
    } catch (error) {
      setIsConnected(false)
      setLastCheck(new Date())
      console.error('Health check failed:', handleApiError(error))
    }
  }

  useEffect(() => {
    checkHealth()
    const interval = setInterval(checkHealth, 30000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (!isConnected || activeMenu !== 'overview') return
    let cancelled = false
    getOverview()
      .then((data) => {
        if (cancelled) return
        setOverviewData(data)
        setOverviewError('')
      })
      .catch((error) => {
        if (!cancelled) setOverviewError(handleApiError(error))
      })
    return () => {
      cancelled = true
    }
  }, [isConnected, activeMenu, refreshKey])

  const renderContent = () => {
    switch (activeMenu) {
      case 'overview':
        return (
          <Overview
            isConnected={isConnected}
            overviewData={overviewData}
            overviewError={overviewError}
            lastCheck={lastCheck}
          />
        )
      case 'resources':
        return <NamespaceMap isConnected={isConnected} refreshKey={refreshKey} />
      default:
        return <div>Select a menu item</div>
    }
  }

  return (
    <div className="app">
      <Header
        activeMenu={activeMenu}
        setActiveMenu={setActiveMenu}
        onManualRefresh={() => setRefreshKey((value) => value + 1)}
      />
      <main className="main-container">
        {renderContent()}
      </main>
    </div>
  )
}

export default App
