import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useForm } from '@tanstack/react-form';
import { DatabaseService } from '../../bindings/changeme/db_service';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Badge } from './ui/badge';
import { Trash2, Plus, Shield, ShieldOff, RefreshCw, Code, Zap, Hash } from 'lucide-react';
import { toast } from "sonner"


interface DomainManagerProps {
  className?: string;
}

type PatternType = 'exact' | 'glob' | 'regex';

const getPatternExamples = (type: PatternType) => {
    switch (type) {
      case 'exact':
        return ['example.com', 'subdomain.example.com'];
      case 'glob':
        return ['*.example.com', 'sub.*.com', '*.ads.*'];
      case 'regex':
        return ['.*\\.example\\.com$', '.*\\.(ads|tracking)\\..*'];
      default:
        return [];
    }
  };

export function DomainManager({ className }: DomainManagerProps) {
  const queryClient = useQueryClient();

  // TanStack Form setup
  const form = useForm({
    defaultValues: {
      domain: '',
      patternType: 'exact' as PatternType,
    },
    onSubmit: async ({ value }) => {
      await addDomain(value.domain, value.patternType);
    },
    validators: {
      onChange: ({value}) => validateDomain(value.domain, value.patternType),
    },
  });

  // Query for loading domains with pattern type info
  const { 
    data: domains = [], 
    isLoading: isLoadingDomains, 
    error: domainsError,
    refetch: refetchDomains
  } = useQuery({
    queryKey: ['domains'],
    queryFn: () => DatabaseService.ListBlockedDomainsWithInfo(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });

  // Mutation for adding a domain
  const addDomainMutation = useMutation({
    mutationFn: ({ domain, patternType }: { domain: string; patternType: PatternType }) => 
      DatabaseService.BlockDomainWithType(domain, patternType),
    onSuccess: (success, { domain, patternType }) => {
      if (success) {
        form.reset();
        queryClient.invalidateQueries({ queryKey: ['domains'] });
        toast.success(`Domain "${domain}" blocked successfully as ${patternType} pattern`);
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

  const addDomain = async (domain: string, patternType: PatternType) => {
    addDomainMutation.mutate({ domain, patternType });
  };

  const removeDomain = (domain: string) => {
    removeDomainMutation.mutate(domain);
  };

  // Validation functions
  const validateDomain = (value: string, patternType: PatternType) => {
    if (!value.trim()) {
      return 'Domain is required';
    }

    if (patternType === 'regex') {
      try {
        new RegExp(value.trim());
      } catch (err) {
        return 'Invalid regex pattern';
      }
    }

    if (patternType === 'exact') {
      // Basic domain validation for exact matches
      const domainRegex = /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/;
      if (!domainRegex.test(value.trim())) {
        return 'Invalid domain format';
      }
    }

    if (patternType === 'glob') {
      // Basic validation for glob patterns
      if (value.includes('**')) {
        return 'Double wildcards (**) are not allowed';
      }
    }

    return undefined;
  };

  console.log(addDomainMutation.isPending , form.state.isValidating , form.state.isValid);
 


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
            Block domains using exact matching, wildcard patterns, or regex patterns. Select the pattern type above.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Pattern Type Selection */}
            <form.Field name="patternType">
              {(field) => (
                <div className="flex space-x-2">
                  <Button
                    variant={field.state.value === 'exact' ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => field.handleChange('exact')}
                    className="flex items-center space-x-1"
                    title="Block exact domain matches only"
                  >
                    <Hash className="h-4 w-4" />
                    <span>Exact</span>
                  </Button>
                  <Button
                    variant={field.state.value === 'glob' ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => field.handleChange('glob')}
                    className="flex items-center space-x-1"
                    title="Block domains using wildcard patterns (* and ?)"
                  >
                    <Zap className="h-4 w-4" />
                    <span>Wildcard</span>
                  </Button>
                  <Button
                    variant={field.state.value === 'regex' ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => field.handleChange('regex')}
                    className="flex items-center space-x-1"
                    title="Block domains using regex patterns"
                  >
                    <Code className="h-4 w-4" />
                    <span>Regex</span>
                  </Button>
                </div>
              )}
            </form.Field>
            
            {/* Pattern Examples */}
            <form.Subscribe selector={(state) => state.values.patternType}>
              {patternType => (
                <div className="text-sm text-gray-600">
                  <p className="mb-2">
                    {patternType === 'exact' && 'Enter exact domain names:'}
                    {patternType === 'glob' && 'Use wildcards (* for any characters, ? for single character):'}
                    {patternType === 'regex' && 'Use regex patterns:'}
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {getPatternExamples(patternType).map((example, index) => (
                      <code key={index} className="bg-gray-100 px-2 py-1 rounded text-xs">
                        {example}
                      </code>
                    ))}
                  </div>
                </div>
              )}
            </form.Subscribe>
            
            {/* Input and Button */}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                e.stopPropagation();
                form.handleSubmit();
              }}
              className="space-y-2"
            >
              <div className="flex space-x-2">
                <form.Field
                  name="domain"
                  validators={{
                    onChange: ({ value, fieldApi }) => {
                      const patternType = form.getFieldValue('patternType');
                      return validateDomain(value, patternType);
                    },
                  }}
                >
                  {(field) => (
                    <div className="flex-1 space-y-1">
                      <Input
                        placeholder={
                          form.getFieldValue('patternType') === 'exact' ? 'example.com' :
                          form.getFieldValue('patternType') === 'glob' ? '*.example.com' :
                          '.*\\.example\\.com$'
                        }
                        autoCapitalize='off'
                        autoComplete='off'
                        autoCorrect='off'
                        value={field.state.value}
                        onChange={(e) => field.handleChange(e.target.value)}
                        onBlur={field.handleBlur}
                        disabled={addDomainMutation.isPending}
                        className={`flex-1 ${field.state.meta.errors.length > 0 ? 'border-red-500' : ''}`}
                      />
                      {field.state.meta.errors.length > 0 && (
                        <p className="text-sm text-red-500">{field.state.meta.errors[0]}</p>
                      )}
                    </div>
                  )}
                </form.Field>
                <form.Subscribe selector={(state) => ({isValid: state.isValid, isValidating: state.isValidating, isPristine: state.isPristine, isPending: addDomainMutation.isPending})}>
                    {values => (
                
                <Button 
                  type="submit"
                  disabled={values.isValidating || !values.isValid || values.isPristine || values.isPending}
                  className="px-6"
                >
                  <Plus className="h-4 w-4 mr-2" />
                  {values.isPending ? 'Blocking...' : 'Block Domain'}
                </Button>
                )}
                </form.Subscribe>
              </div>
            </form>
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
                  <TableHead>Domain/Pattern</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Examples</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {domains.map((domainInfo, index) => (
                  <TableRow key={index}>
                    <TableCell className="font-medium">
                      <code className="bg-gray-100 px-2 py-1 rounded text-sm">
                        {domainInfo.domain}
                      </code>
                    </TableCell>
                    <TableCell>
                      <Badge 
                        variant={
                          domainInfo.filterType === 'exact' ? 'secondary' :
                          domainInfo.filterType === 'glob' ? 'default' :
                          'destructive'
                        }
                        className="flex items-center space-x-1"
                      >
                        {domainInfo.filterType === 'exact' && <Hash className="h-3 w-3" />}
                        {domainInfo.filterType === 'glob' && <Zap className="h-3 w-3" />}
                        {domainInfo.filterType === 'regex' && <Code className="h-3 w-3" />}
                        <span className="capitalize">{domainInfo.filterType}</span>
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-gray-600">
                      {domainInfo.filterType === 'exact' && (
                        <div>
                          <div className="font-medium">Exact match only</div>
                          <div className="text-xs">Blocks: <code className="bg-gray-100 px-1 rounded">{domainInfo.domain}</code></div>
                        </div>
                      )}
                      {domainInfo.filterType === 'glob' && (
                        <div>
                          <div className="font-medium">Wildcard pattern</div>
                          <div className="text-xs">Blocks domains matching: <code className="bg-gray-100 px-1 rounded">{domainInfo.domain}</code></div>
                        </div>
                      )}
                      {domainInfo.filterType === 'regex' && (
                        <div>
                          <div className="font-medium">Regex pattern</div>
                          <div className="text-xs">Advanced pattern matching</div>
                        </div>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => removeDomain(domainInfo.domain)}
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
