import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {PageHeader} from '@/components/ui/page-header';
import {formatBytes} from '@/lib/file-utils';
import {Activity, AlertCircle, CheckCircle2, Clock, Cpu, Database, Info, Network, Server, XCircle,} from 'lucide-react';
import {useQuery} from '@tanstack/react-query';
import {garageApi} from '@/lib/api';
import {Badge} from '@/components/ui/badge';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import type {ClusterNode, LocalNodeInfo, NodeStatistics} from '@/types';
import {useState} from 'react';
import { useCapabilities } from '@/hooks/useCapabilities';
import { useClusterStore } from '@/store/cluster-store';
import { Navigate } from 'react-router-dom';
import { useTranslation } from '@/lib/i18n';
import { queryKeys } from '@/lib/query-client';
import { EmptyState } from '@/components/ui/empty-state';
import { Button } from '@/components/ui/button';

function UnsupportedFeatureCard({ title, description }: { title: string; description?: string }) {
  const { t } = useTranslation();
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-[var(--muted-foreground)]">
          <Info className="h-4 w-4" />
          {title}
        </CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>
        <p className="text-sm text-[var(--muted-foreground)]">
          {t('cluster.requires_garage_v2')}
        </p>
      </CardContent>
    </Card>
  );
}

export function Cluster() {
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const { t, language } = useTranslation();
  const number = new Intl.NumberFormat(language);
  const formatLocalizedBytes = (bytes: number) => formatBytes(bytes).replace(/^[\d.]+/, (value) => number.format(Number(value)));

  const { data: capabilities } = useCapabilities();
  const features = capabilities?.features;

  const { clusters, activeClusterId } = useClusterStore();

  const { data: health, isLoading: healthLoading, isError: healthError, refetch: refetchHealth } = useQuery({
    queryKey: queryKeys.cluster.health(activeClusterId),
    queryFn: () => garageApi.getClusterHealth(),
    refetchInterval: 10000,
    enabled: !!activeClusterId,
  });

  const { data: status, isLoading: statusLoading, isError: statusError, refetch: refetchStatus } = useQuery({
    queryKey: queryKeys.cluster.status(activeClusterId),
    queryFn: () => garageApi.getClusterStatus(),
    refetchInterval: 15000,
    enabled: !!activeClusterId,
  });

  const { data: statistics, isLoading: statisticsLoading, isError: statisticsError, refetch: refetchStatistics } = useQuery({
    queryKey: queryKeys.cluster.statistics(activeClusterId),
    queryFn: () => garageApi.getClusterStatistics(),
    refetchInterval: 30000,
    enabled: !!activeClusterId && !!features && features.clusterStatistics !== false,
  });

  const { data: nodeInfo, isLoading: nodeInfoLoading } = useQuery({
    queryKey: ['cluster', activeClusterId ?? 'none', 'node-info', selectedNodeId || '*'],
    queryFn: () => garageApi.getNodeInfo(selectedNodeId || '*'),
    enabled: !!activeClusterId && !!features && features.nodeInfo !== false && (!!selectedNodeId || selectedNodeId === null),
  });

  const { data: nodeStats } = useQuery({
    queryKey: ['cluster', activeClusterId ?? 'none', 'node-statistics', selectedNodeId || '*'],
    queryFn: () => garageApi.getNodeStatistics(selectedNodeId || '*'),
    enabled: !!activeClusterId && !!features && features.nodeStatistics !== false && !!selectedNodeId,
  });

  if (clusters.length === 0) {
    return <Navigate to="/" replace />;
  }

  if (healthError || statusError || statisticsError) {
    return (
      <div>
        <PageHeader title={t('nav.cluster')} subtitle={t('cluster.load_error_subtitle')} />
        <div className="p-6">
          <EmptyState
            icon={<AlertCircle />}
            title={t('cluster.unavailable_title')}
            description={t('cluster.unavailable_description')}
            tone="destructive"
            action={<Button onClick={() => void Promise.all([refetchHealth(), refetchStatus(), refetchStatistics()])}>{t('common.retry')}</Button>}
          />
        </div>
      </div>
    );
  }

  const isLoading = healthLoading || statusLoading || statisticsLoading;

  const getHealthStatus = () => {
    if (!health) return { color: 'text-gray-500', bgColor: 'bg-gray-100', label: t('cluster.health_unknown'), icon: AlertCircle };
    if (
      health.storageNodesUp === health.storageNodes &&
      health.partitionsAllOk === health.partitions &&
      health.connectedNodes === health.knownNodes
    ) {
      return { color: 'text-green-600', bgColor: 'bg-green-100', label: t('cluster.health_healthy'), icon: CheckCircle2 };
    }
    if (health.storageNodesUp > 0 && health.partitionsQuorum > 0) {
      return { color: 'text-yellow-600', bgColor: 'bg-yellow-100', label: t('cluster.health_degraded'), icon: AlertCircle };
    }
    return { color: 'text-red-600', bgColor: 'bg-red-100', label: t('cluster.health_unhealthy'), icon: XCircle };
  };

  const healthStatus = getHealthStatus();
  const HealthIcon = healthStatus.icon;

  const getNodeStatus = (node: ClusterNode) => {
    if (!node.isUp) {
      return { color: 'text-red-600', bgColor: 'bg-red-100', label: t('cluster.node_down'), icon: XCircle };
    }
    if (node.draining) {
      return { color: 'text-yellow-600', bgColor: 'bg-yellow-100', label: t('cluster.node_draining'), icon: AlertCircle };
    }
    return { color: 'text-green-600', bgColor: 'bg-green-100', label: t('cluster.node_up'), icon: CheckCircle2 };
  };

  const formatUptime = (seconds?: number) => {
    if (!seconds) return t('common.not_available');
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return t('cluster.uptime_days_hours_minutes')
      .replace('{{days}}', number.format(days))
      .replace('{{hours}}', number.format(hours))
      .replace('{{minutes}}', number.format(minutes));
  };

  if (isLoading) {
    return (
      <div>
        <PageHeader title={t('nav.cluster')} />
        <div className="p-4 sm:p-6 flex items-center justify-center min-h-[400px]">
          <div className="text-center">
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
            <p className="mt-2 text-sm text-muted-foreground">{t('cluster.loading_information')}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader title={t('nav.status')} subtitle={t('cluster.page_subtitle')} />
      <div className="p-4 sm:p-6 space-y-4 sm:space-y-6">
        {/* Cluster Health Overview */}
        <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t('cluster.status_title')}</CardTitle>
              <HealthIcon className={`h-4 w-4 ${healthStatus.color}`} />
            </CardHeader>
            <CardContent>
              <div className={`text-2xl font-bold ${healthStatus.color}`}>{healthStatus.label}</div>
              <p className="text-xs text-muted-foreground mt-2">
                {t('cluster.layout_version').replace('{{version}}', number.format(status?.layoutVersion || 0))}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t('cluster.connected_nodes_title')}</CardTitle>
              <Network className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {number.format(health?.connectedNodes || 0)}/{number.format(health?.knownNodes || 0)}
              </div>
              <p className="text-xs text-muted-foreground">
                {t('cluster.nodes_online')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t('cluster.storage_nodes_title')}</CardTitle>
              <Server className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {number.format(health?.storageNodesUp || 0)}/{number.format(health?.storageNodes || 0)}
              </div>
              <p className="text-xs text-muted-foreground">
                {t('cluster.healthy_storage_nodes')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t('cluster.partitions_title')}</CardTitle>
              <Database className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {number.format(health?.partitionsAllOk || 0)}/{number.format(health?.partitions || 0)}
              </div>
              <p className="text-xs text-muted-foreground">
                {t('cluster.healthy_partitions')}
              </p>
            </CardContent>
          </Card>
        </div>

        {/* Tabs for different views */}
        <Tabs defaultValue="nodes" className="space-y-4">
          <TabsList>
            <TabsTrigger value="nodes">{t('cluster.nodes_tab')}</TabsTrigger>
            <TabsTrigger value="statistics">{t('cluster.statistics_tab')}</TabsTrigger>
            <TabsTrigger value="details">{t('cluster.details_tab')}</TabsTrigger>
          </TabsList>

          {/* Nodes Tab */}
          <TabsContent value="nodes" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>{t('cluster.nodes_title')}</CardTitle>
                <CardDescription>
                  {t('cluster.nodes_description')}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {status?.nodes && status.nodes.length > 0 ? (
                    status.nodes.map((node) => {
                      const nodeStatus = getNodeStatus(node);
                      const NodeIcon = nodeStatus.icon;
                      const dataUsage = node.dataPartition && node.dataPartition.total > 0
                        ? ((node.dataPartition.total - node.dataPartition.available) / node.dataPartition.total) * 100
                        : 0;
                      const metadataUsage = node.metadataPartition && node.metadataPartition.total > 0
                        ? ((node.metadataPartition.total - node.metadataPartition.available) / node.metadataPartition.total) * 100
                        : 0;

                      return (
                        <Card
                          key={node.id}
                          className={`cursor-pointer transition-all hover:shadow-md ${
                            selectedNodeId === node.id ? 'ring-2 ring-primary' : ''
                          }`}
                          onClick={() => setSelectedNodeId(node.id)}
                        >
                          <CardContent className="pt-6">
                            <div className="flex items-start justify-between">
                              <div className="flex-1 space-y-2">
                                <div className="flex items-center gap-3">
                                  <NodeIcon className={`h-5 w-5 ${nodeStatus.color}`} />
                                  <div>
                                    <div className="font-mono text-sm font-medium">
                                      {node.id.substring(0, 16)}...
                                    </div>
                                    {node.hostname && (
                                      <div className="text-xs text-muted-foreground">{node.hostname}</div>
                                    )}
                                  </div>
                                </div>

                                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-2">
                                  <div>
                                     <div className="text-xs text-muted-foreground">{t('cluster.node_status_label')}</div>
                                    <Badge variant={node.isUp ? 'primary' : 'danger'} className="mt-1">
                                      {nodeStatus.label}
                                    </Badge>
                                  </div>

                                  {node.addr && (
                                    <div>
                                       <div className="text-xs text-muted-foreground">{t('cluster.node_address_label')}</div>
                                      <div className="text-sm font-mono">{node.addr}</div>
                                    </div>
                                  )}

                                  {node.garageVersion && (
                                    <div>
                                       <div className="text-xs text-muted-foreground">{t('cluster.node_version_label')}</div>
                                      <div className="text-sm">{node.garageVersion}</div>
                                    </div>
                                  )}

                                  {node.role && (
                                    <div>
                                       <div className="text-xs text-muted-foreground">{t('cluster.node_zone_label')}</div>
                                      <div className="text-sm">{node.role.zone}</div>
                                    </div>
                                  )}
                                </div>

                                {node.role?.capacity && (
                                  <div className="pt-2">
                                    <div className="text-xs text-muted-foreground mb-1">
                                       {t('cluster.node_capacity').replace('{{capacity}}', formatLocalizedBytes(node.role.capacity))}
                                    </div>
                                  </div>
                                )}

                                {(node.dataPartition || node.metadataPartition) && (
                                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                                    {node.dataPartition && (
                                      <div>
                                        <div className="text-xs text-muted-foreground mb-1">
                                           {t('cluster.data_partition_usage')
                                             .replace('{{used}}', formatLocalizedBytes(node.dataPartition.total - node.dataPartition.available))
                                             .replace('{{total}}', formatLocalizedBytes(node.dataPartition.total))}
                                        </div>
                                        <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                                          <div
                                            className={`h-full transition-all ${
                                              dataUsage > 90 ? 'bg-red-500' : dataUsage > 70 ? 'bg-yellow-500' : 'bg-green-500'
                                            }`}
                                            style={{ width: `${dataUsage}%` }}
                                          />
                                        </div>
                                      </div>
                                    )}

                                    {node.metadataPartition && (
                                      <div>
                                        <div className="text-xs text-muted-foreground mb-1">
                                           {t('cluster.metadata_partition_usage')
                                             .replace('{{used}}', formatLocalizedBytes(node.metadataPartition.total - node.metadataPartition.available))
                                             .replace('{{total}}', formatLocalizedBytes(node.metadataPartition.total))}
                                        </div>
                                        <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                                          <div
                                            className={`h-full transition-all ${
                                              metadataUsage > 90 ? 'bg-red-500' : metadataUsage > 70 ? 'bg-yellow-500' : 'bg-green-500'
                                            }`}
                                            style={{ width: `${metadataUsage}%` }}
                                          />
                                        </div>
                                      </div>
                                    )}
                                  </div>
                                )}

                                {!node.isUp && node.lastSeenSecsAgo !== undefined && (
                                  <div className="text-xs text-muted-foreground pt-2">
                                    <Clock className="inline h-3 w-3 mr-1" />
                                     {node.lastSeenSecsAgo === null
                                       ? t('cluster.last_seen_never')
                                       : t('cluster.last_seen_ago').replace('{{duration}}', formatUptime(node.lastSeenSecsAgo))}
                                  </div>
                                )}
                              </div>
                            </div>
                          </CardContent>
                        </Card>
                      );
                    })
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      <Server className="h-12 w-12 mx-auto mb-2 opacity-50" />
                      <p>{t('cluster.no_nodes')}</p>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Statistics Tab */}
          <TabsContent value="statistics" className="space-y-4">
            {features?.clusterStatistics === false ? (
              <UnsupportedFeatureCard title={t('cluster.statistics_title')} description={t('cluster.statistics_global_description')} />
            ) : (
              <Card>
                <CardHeader>
                  <CardTitle>{t('cluster.statistics_title')}</CardTitle>
                  <CardDescription>
                    {t('cluster.statistics_description')}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {statistics ? (
                    <div className="space-y-4">
                      <div className="rounded-lg bg-muted p-4">
                        <pre className="text-xs overflow-x-auto whitespace-pre-wrap font-mono">
                          {statistics.freeform}
                        </pre>
                      </div>
                    </div>
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      <Activity className="h-12 w-12 mx-auto mb-2 opacity-50" />
                      <p>{t('cluster.no_statistics')}</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}
          </TabsContent>

          {/* Details Tab */}
          <TabsContent value="details" className="space-y-4">
            {selectedNodeId ? (
              <>
                {features?.nodeInfo === false ? (
                  <UnsupportedFeatureCard title={t('cluster.node_details_title')} description={t('cluster.node_details_description')} />
                ) : (
                  <Card>
                    <CardHeader>
                      <CardTitle>{t('cluster.node_information_title')}</CardTitle>
                      <CardDescription>
                        {t('cluster.node_information_description').replace('{{nodeId}}', `${selectedNodeId.substring(0, 16)}...`)}
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      {nodeInfoLoading ? (
                        <div className="text-center py-8">
                          <div className="inline-block h-6 w-6 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
                          <p className="mt-2 text-sm text-muted-foreground">{t('cluster.loading_node_information')}</p>
                        </div>
                      ) : nodeInfo ? (
                        <div className="space-y-4">
                          {/* Success responses */}
                          {Object.entries(nodeInfo.success || {}).map(([nodeId, info]) => (
                            <div key={nodeId} className="space-y-3">
                              <div className="flex items-center gap-2 mb-3">
                                <Info className="h-4 w-4 text-primary" />
                                <h4 className="font-medium">
                                   {t('cluster.node_heading').replace('{{nodeId}}', `${nodeId.substring(0, 16)}...`)}
                                </h4>
                              </div>

                              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                <div className="rounded-lg border p-3">
                                   <div className="text-xs text-muted-foreground mb-1">{t('cluster.node_id_label')}</div>
                                  <div className="font-mono text-sm break-all">{(info as LocalNodeInfo).nodeId}</div>
                                </div>

                                <div className="rounded-lg border p-3">
                                   <div className="text-xs text-muted-foreground mb-1">{t('cluster.garage_version_label')}</div>
                                  <div className="text-sm">{(info as LocalNodeInfo).garageVersion}</div>
                                </div>

                                <div className="rounded-lg border p-3">
                                   <div className="text-xs text-muted-foreground mb-1">{t('cluster.rust_version_label')}</div>
                                  <div className="text-sm">{(info as LocalNodeInfo).rustVersion}</div>
                                </div>

                                <div className="rounded-lg border p-3">
                                   <div className="text-xs text-muted-foreground mb-1">{t('cluster.database_engine_label')}</div>
                                  <div className="text-sm">{(info as LocalNodeInfo).dbEngine}</div>
                                </div>
                              </div>

                              {(info as LocalNodeInfo).garageFeatures && (info as LocalNodeInfo).garageFeatures!.length > 0 && (
                                <div className="rounded-lg border p-3">
                                   <div className="text-xs text-muted-foreground mb-2">{t('cluster.garage_features_label')}</div>
                                  <div className="flex flex-wrap gap-2">
                                    {(info as LocalNodeInfo).garageFeatures!.map((feature) => (
                                      <Badge key={feature} variant="neutral">
                                        {feature}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                              )}
                            </div>
                          ))}

                          {/* Error responses */}
                          {Object.entries(nodeInfo.error || {}).map(([nodeId, error]) => (
                            <div key={nodeId} className="rounded-lg border border-red-200 bg-red-50 p-3">
                              <div className="flex items-center gap-2 text-red-600 mb-1">
                                <XCircle className="h-4 w-4" />
                                 <div className="font-medium">{t('cluster.node_error').replace('{{nodeId}}', `${nodeId.substring(0, 16)}...`)}</div>
                              </div>
                              <div className="text-sm text-red-800">{error}</div>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="text-center text-muted-foreground py-8">
                          <Info className="h-12 w-12 mx-auto mb-2 opacity-50" />
                           <p>{t('cluster.no_node_information')}</p>
                        </div>
                      )}
                    </CardContent>
                  </Card>
                )}

                {features?.nodeStatistics === false ? (
                   <UnsupportedFeatureCard title={t('cluster.node_statistics_title')} description={t('cluster.node_statistics_description')} />
                ) : nodeStats ? (
                  <Card>
                    <CardHeader>
                       <CardTitle>{t('cluster.node_statistics_title')}</CardTitle>
                      <CardDescription>
                         {t('cluster.selected_node_performance_description')}
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-4">
                        {/* Success responses */}
                        {Object.entries(nodeStats.success || {}).map(([nodeId, stats]) => (
                          <div key={nodeId} className="space-y-3">
                            <div className="flex items-center gap-2 mb-3">
                              <Cpu className="h-4 w-4 text-primary" />
                              <h4 className="font-medium">
                                 {t('cluster.statistics_for_node').replace('{{nodeId}}', `${nodeId.substring(0, 16)}...`)}
                              </h4>
                            </div>

                            <div className="rounded-lg bg-muted p-4">
                              <pre className="text-xs overflow-x-auto whitespace-pre-wrap font-mono">
                                {(stats as NodeStatistics).freeform}
                              </pre>
                            </div>
                          </div>
                        ))}

                        {/* Error responses */}
                        {Object.entries(nodeStats.error || {}).map(([nodeId, error]) => (
                          <div key={nodeId} className="rounded-lg border border-red-200 bg-red-50 p-3">
                            <div className="flex items-center gap-2 text-red-600 mb-1">
                              <XCircle className="h-4 w-4" />
                               <div className="font-medium">{t('cluster.node_error').replace('{{nodeId}}', `${nodeId.substring(0, 16)}...`)}</div>
                            </div>
                            <div className="text-sm text-red-800">{error}</div>
                          </div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                ) : null}
              </>
            ) : (
              <Card>
                <CardContent className="pt-6">
                  <div className="text-center text-muted-foreground py-12">
                    <Server className="h-16 w-16 mx-auto mb-4 opacity-50" />
                     <p className="text-lg font-medium mb-2">{t('cluster.select_node_title')}</p>
                    <p className="text-sm">
                       {t('cluster.select_node_description')}
                    </p>
                  </div>
                </CardContent>
              </Card>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
