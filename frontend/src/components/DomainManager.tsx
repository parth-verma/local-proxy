import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { DatabaseService } from '../../bindings/changeme/db_service';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Badge } from './ui/badge';
import { Trash2, Plus, Shield, ShieldOff, RefreshCw } from 'lucide-react';
import { toast } from "sonner"


interface DomainManagerProps {
  className?: string;
}

export function DomainManager({ className }: DomainManagerProps) {
  const [newDomain, setNewDomain] = useState('');

  const queryClient = useQueryClient();

  // Query for loading domains
  const { 
    data: domains = [], 
    isLoading: isLoadingDomains, 
    error: domainsError,
    refetch: refetchDomains
  } = useQuery({
    queryKey: ['domains'],
    queryFn: () => DatabaseService.ListBlockedDomains(''),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });

  // Mutation for adding a domain
  const addDomainMutation = useMutation({
    mutationFn: (domain: string) => DatabaseService.BlockDomain(domain),
    onSuccess: (success, domain) => {
      if (success) {
        setNewDomain('');
        queryClient.invalidateQueries({ queryKey: ['domains'] });
        toast.success(`Domain "${domain}" blocked successfully`);
      } else {
        toast.error('Failed to block domain');
      }
    },
    onError: (err) => {
      console.error('Failed to add domain:', err);
      toast.error('Failed to block domain');
    },
  });

  // Mutation for removing a domain
  const removeDomainMutation = useMutation({
    mutationFn: (domain: string) => DatabaseService.UnblockDomain(domain),
    onSuccess: (success, domain) => {
      if (success) {
        queryClient.invalidateQueries({ queryKey: ['domains'] });
        toast.success(`Domain "${domain}" unblocked successfully`);
      } else {
        toast.error('Failed to unblock domain');
      }
    },
    onError: (err) => {
      console.error('Failed to remove domain:', err);
      toast.error('Failed to unblock domain');
    },
  });

  const addDomain = () => {
    if (!newDomain.trim()) {
      toast.error('Please enter a domain name');
      return;
    }
    addDomainMutation.mutate(newDomain.trim());
  };

  const removeDomain = (domain: string) => {
    removeDomainMutation.mutate(domain);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      addDomain();
    }
  };


  return (
    <div className={`space-y-6 ${className}`}>
      {/* Header */}
      <div className="flex items-center space-x-2">
        <Shield className="h-6 w-6 text-blue-600" />
        <h2 className="text-2xl font-bold">Blocked Domains</h2>
        <Badge variant="secondary" className="ml-2">
          {domains.length} blocked
        </Badge>
      </div>

      {/* Add Domain Form */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Plus className="h-5 w-5" />
            <span>Add Blocked Domain</span>
          </CardTitle>
          <CardDescription>
            Enter a domain name to block. You can use wildcards like *.example.com
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex space-x-2">
            <Input
              placeholder="example.com or *.example.com"
              value={newDomain}
              onChange={(e) => setNewDomain(e.target.value)}
              onKeyPress={handleKeyPress}
              disabled={addDomainMutation.isPending}
              className="flex-1"
            />
            <Button 
              onClick={addDomain} 
              disabled={addDomainMutation.isPending || !newDomain.trim()}
              className="px-6"
            >
              <Plus className="h-4 w-4 mr-2" />
              {addDomainMutation.isPending ? 'Blocking...' : 'Block Domain'}
            </Button>
          </div>
        </CardContent>
      </Card>


      {/* Domains List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center space-x-2">
                <ShieldOff className="h-5 w-5" />
                <span>Blocked Domains List</span>
              </CardTitle>
              <CardDescription>
                Manage your blocked domains. Click the trash icon to unblock a domain.
              </CardDescription>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetchDomains()}
              disabled={isLoadingDomains}
              className="ml-4"
            >
              <RefreshCw className={`h-4 w-4 ${isLoadingDomains ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {isLoadingDomains ? (
            <div className="text-center py-8 text-gray-500">
              Loading domains...
            </div>
          ) : domainsError ? (
            <div className="text-center py-8 text-red-500">
              <p>Failed to load domains</p>
              <p className="text-sm">Please try refreshing the page</p>
            </div>
          ) : domains.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              <Shield className="h-12 w-12 mx-auto mb-4 text-gray-300" />
              <p>No domains are currently blocked.</p>
              <p className="text-sm">Add a domain above to get started.</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Domain</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {domains.map((domain, index) => (
                  <TableRow key={index}>
                    <TableCell className="font-medium">
                      <code className="bg-gray-100 px-2 py-1 rounded text-sm">
                        {domain}
                      </code>
                    </TableCell>
                    <TableCell>
                      <Badge variant={domain.includes('*') ? 'default' : 'secondary'}>
                        {domain.includes('*') ? 'Wildcard' : 'Exact'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => removeDomain(domain)}
                        disabled={removeDomainMutation.isPending}
                        className="text-red-600 hover:text-red-700 hover:bg-red-50"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

    </div>
  );
}
