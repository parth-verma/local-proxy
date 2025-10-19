import { useState } from 'react';
import { DomainManager } from './components/DomainManager';
import { Dashboard } from './components/Dashboard';
import { Navigation } from './components/Navigation';
import { Toaster } from "@/components/ui/sonner"

function App() {
  const [activeTab, setActiveTab] = useState<'domains' | 'dashboard'>('domains');

  return (
    <div className="min-h-screen bg-gray-50">
      <Navigation activeTab={activeTab} onTabChange={setActiveTab} />
      
      <div className="container mx-auto px-4 py-8 max-w-6xl">
        {activeTab === 'domains' && (
          <>
            <div className="mb-8 text-center">
              <h1 className="text-3xl font-bold text-gray-900 mb-2">Local Proxy</h1>
              <p className="text-gray-600">Manage your blocked domains and proxy settings</p>
            </div>
            <DomainManager />
          </>
        )}
        
        {activeTab === 'dashboard' && (
          <>
            <div className="mb-8 text-center">
              <h1 className="text-3xl font-bold text-gray-900 mb-2">Proxy Analytics</h1>
              <p className="text-gray-600">Monitor connection trends and request details</p>
            </div>
            <Dashboard />
          </>
        )}
      </div>
      
      <Toaster />
    </div>
  );
}

export default App
