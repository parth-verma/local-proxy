import { Button } from './ui/button';
import { Shield, Activity } from 'lucide-react';

interface NavigationProps {
  activeTab: 'domains' | 'dashboard';
  onTabChange: (tab: 'domains' | 'dashboard') => void;
}

export function Navigation({ activeTab, onTabChange }: NavigationProps) {
  return (
    <div className="bg-white border-b border-gray-200">
      <div className="container mx-auto px-4">
        <div className="flex space-x-1">
          <Button
            variant={activeTab === 'domains' ? 'default' : 'ghost'}
            onClick={() => onTabChange('domains')}
            className="flex items-center space-x-2"
          >
            <Shield className="h-4 w-4" />
            <span>Domain Manager</span>
          </Button>
          <Button
            variant={activeTab === 'dashboard' ? 'default' : 'ghost'}
            onClick={() => onTabChange('dashboard')}
            className="flex items-center space-x-2"
          >
            <Activity className="h-4 w-4" />
            <span>Dashboard</span>
          </Button>
        </div>
      </div>
    </div>
  );
}
