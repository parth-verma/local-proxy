import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { LoggingService } from '../../bindings/changeme/logging_service';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { 
  LineChart, 
  Line, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  Legend, 
  ResponsiveContainer
} from 'recharts';
import { 
  Activity, 
  CheckCircle, 
  XCircle, 
  Clock, 
  TrendingUp,
  RefreshCw,
  Filter
} from 'lucide-react';

interface DashboardProps {
  className?: string;
}

const TIME_RANGES = [
  { value: '1h', label: 'Last Hour' },
  { value: '6h', label: 'Last 6 Hours' },
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
];

const DECISION_FILTERS = [
  { value: 'all', label: 'All Requests' },
  { value: 'approved', label: 'Approved Only' },
  { value: 'rejected', label: 'Rejected Only' },
];

export function Dashboard({ className }: DashboardProps) {
  const [selectedTimeRange, setSelectedTimeRange] = useState('24h');
  const [decisionFilter, setDecisionFilter] = useState('all');

  // Query for dashboard data
  const { 
    data: dashboardData, 
    isLoading, 
    error,
    refetch 
  } = useQuery({
    queryKey: ['dashboard', selectedTimeRange],
    queryFn: () => LoggingService.GetDashboardData(selectedTimeRange),
    staleTime: 30 * 1000, // 30 seconds
    refetchInterval: 30 * 1000, // Auto-refresh every 30 seconds
  });

  // Filter requests based on decision filter
  const filteredRequests = dashboardData?.requests?.filter(request => {
    if (decisionFilter === 'all') return true;
    return request.decision === decisionFilter;
  }) || [];

  // Format timestamp for display
  const formatTimestamp = (timestamp: number) => {
    return new Date(timestamp).toLocaleString();
  };

  // Format duration for display (duration is stored in nanoseconds)
  const formatDuration = (duration: number) => {
    // Convert nanoseconds to milliseconds
    const milliseconds = duration / 1000000;
    if (milliseconds < 1000) {
      return `${milliseconds.toFixed(1)}ms`;
    }
    return `${(milliseconds / 1000).toFixed(2)}s`;
  };

  // Prepare chart data
  const chartData = dashboardData?.connections?.map(conn => ({
    time: new Date(conn.timestamp).toLocaleTimeString(),
    timestamp: conn.timestamp,
    total: conn.count,
    approved: conn.approved,
    rejected: conn.rejected,
  })) || [];

  if (isLoading) {
    return (
      <div className={`space-y-6 ${className}`}>
        <div className="text-center py-8">
          <RefreshCw className="h-8 w-8 animate-spin mx-auto mb-4 text-blue-600" />
          <p className="text-gray-600">Loading dashboard data...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className={`space-y-6 ${className}`}>
        <div className="text-center py-8 text-red-500">
          <XCircle className="h-12 w-12 mx-auto mb-4" />
          <p>Failed to load dashboard data</p>
          <Button onClick={() => refetch()} className="mt-4">
            <RefreshCw className="h-4 w-4 mr-2" />
            Retry
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className={`space-y-6 ${className}`}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <Activity className="h-6 w-6 text-blue-600" />
          <h2 className="text-2xl font-bold">Dashboard</h2>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetch()}
          disabled={isLoading}
        >
          <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {/* Time Range Selector */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Clock className="h-5 w-5" />
            <span>Time Range</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {TIME_RANGES.map((range) => (
              <Button
                key={range.value}
                variant={selectedTimeRange === range.value ? 'default' : 'outline'}
                size="sm"
                onClick={() => setSelectedTimeRange(range.value)}
              >
                {range.label}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Summary Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center space-x-2">
              <Activity className="h-8 w-8 text-blue-600" />
              <div>
                <p className="text-sm font-medium text-gray-600">Total Requests</p>
                <p className="text-2xl font-bold">{dashboardData?.totalRequests || 0}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center space-x-2">
              <CheckCircle className="h-8 w-8 text-green-600" />
              <div>
                <p className="text-sm font-medium text-gray-600">Approved</p>
                <p className="text-2xl font-bold text-green-600">
                  {dashboardData?.approvedCount || 0}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center space-x-2">
              <XCircle className="h-8 w-8 text-red-600" />
              <div>
                <p className="text-sm font-medium text-gray-600">Rejected</p>
                <p className="text-2xl font-bold text-red-600">
                  {dashboardData?.rejectedCount || 0}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Connection Chart */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <TrendingUp className="h-5 w-5" />
            <span>Connection Trends</span>
          </CardTitle>
          <CardDescription>
            Number of connections over the selected time period
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis 
                  dataKey="time" 
                  tick={{ fontSize: 12 }}
                  angle={-45}
                  textAnchor="end"
                  height={60}
                />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip 
                  formatter={(value, name) => [value, name === 'approved' ? 'Approved' : name === 'rejected' ? 'Rejected' : 'Total']}
                  labelFormatter={(label) => `Time: ${label}`}
                />
                <Legend />
                <Line 
                  type="monotone" 
                  dataKey="total" 
                  stroke="#3b82f6" 
                  strokeWidth={2}
                  name="Total"
                />
                <Line 
                  type="monotone" 
                  dataKey="approved" 
                  stroke="#10b981" 
                  strokeWidth={2}
                  name="Approved"
                />
                <Line 
                  type="monotone" 
                  dataKey="rejected" 
                  stroke="#ef4444" 
                  strokeWidth={2}
                  name="Rejected"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      {/* Requests Table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center space-x-2">
                <Filter className="h-5 w-5" />
                <span>Request Details</span>
              </CardTitle>
              <CardDescription>
                Detailed view of all requests in the selected time period
              </CardDescription>
            </div>
            <div className="flex items-center space-x-2">
              <span className="text-sm text-gray-600">Filter:</span>
              <div className="flex space-x-1">
                {DECISION_FILTERS.map((filter) => (
                  <Button
                    key={filter.value}
                    variant={decisionFilter === filter.value ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => setDecisionFilter(filter.value)}
                  >
                    {filter.label}
                  </Button>
                ))}
              </div>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {filteredRequests.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              <Activity className="h-12 w-12 mx-auto mb-4 text-gray-300" />
              <p>No requests found for the selected time period and filter.</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Timestamp</TableHead>
                    <TableHead>Host</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead>Path</TableHead>
                    <TableHead>Port</TableHead>
                    <TableHead>Decision</TableHead>
                    <TableHead>Duration</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredRequests.map((request, index) => (
                    <TableRow key={index}>
                      <TableCell className="font-mono text-sm">
                        {formatTimestamp(request.timestamp)}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {request.host}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">
                          {request.method}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-sm max-w-xs truncate">
                        {request.path}
                      </TableCell>
                      <TableCell className="text-center">
                        {request.port}
                      </TableCell>
                      <TableCell>
                        <Badge 
                          variant={request.decision === 'approved' ? 'default' : 'destructive'}
                          className={request.decision === 'approved' ? 'bg-green-100 text-green-800' : ''}
                        >
                          {request.decision === 'approved' ? (
                            <CheckCircle className="h-3 w-3 mr-1" />
                          ) : (
                            <XCircle className="h-3 w-3 mr-1" />
                          )}
                          {request.decision}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {formatDuration(request.duration)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
