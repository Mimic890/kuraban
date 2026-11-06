// Bucket types
export interface BucketQuotas {
  maxSize?: number;
  maxObjects?: number;
}

export interface Bucket {
  id?: string;
  name: string;
  creationDate: string;
  objectCount?: number;
  size?: number;
  region?: string;
  websiteAccess: boolean;
  websiteConfig?: {
    indexDocument: string;
    errorDocument?: string;
  };
  quotas?: BucketQuotas | null;
  globalAliases?: string[];
  localAliases?: string[];
  unfinishedUploads?: number;
  unfinishedUploadBytes?: number;
  effective_permissions?: string[];
}

export interface BucketKeyAccess {
  accessKeyId: string;
  name: string;
  permissions: { read: boolean; write: boolean; owner: boolean };
  bucketLocalAliases: string[];
}

export interface BucketDetails extends Bucket {
  keys?: BucketKeyAccess[];
  versioning?: boolean;
  encryption?: boolean;
  publicAccess?: boolean;
  lifecycleRules?: LifecycleRule[];
}

export interface LifecycleRule {
  id: string;
  enabled: boolean;
  prefix?: string;
  expirationDays?: number;
  transitions?: Transition[];
}

export interface Transition {
  days: number;
  storageClass: string;
}

// Object types
export interface S3Object {
  key: string;
  size: number;
  lastModified: string;
  etag?: string;
  contentType?: string;
  storageClass?: string;
  isFolder?: boolean;
}

export interface ObjectListResponse {
  bucket: string;
  objects: S3Object[];
  prefixes: string[];
  count: number;
  isTruncated: boolean;
  nextContinuationToken?: string;
}

export interface ObjectMetadata {
  key: string;
  size: number;
  lastModified: string;
  contentType: string;
  etag: string;
  storageClass?: string;
  metadata?: Record<string, string>;
  versionId?: string;
}

// Access Control types
export interface AccessKey {
  accessKeyId: string;
  name: string;
  secretKey?: string;
  createdAt: string;
  status: 'active' | 'inactive';
  permissions: BucketPermission[];
  expiration?: string;
}

export interface BucketPermission {
  bucketId: string;
  bucketName: string;
  read: boolean;
  write: boolean;
  owner: boolean;
}


// User types
export interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'readonly';
  createdAt: string;
}

// Storage Analytics types
export interface StorageMetrics {
  totalSize: number;
  objectCount: number;
  bucketCount: number;
  usageByBucket: BucketUsage[];
  requestMetrics: RequestMetrics;
}

export interface BucketUsage {
  bucketName: string;
  size: number;
  objectCount: number;
  percentage: number;
}

export interface RequestMetrics {
  getRequests: number;
  putRequests: number;
  deleteRequests: number;
  listRequests: number;
  period: string;
}

// Upload types
export interface UploadTask {
  id: string;
  file: File;
  key: string;
  bucket: string;
  progress: number;
  status: 'pending' | 'uploading' | 'completed' | 'error';
  error?: string;
}

// API Response types
export interface ApiResponse<T> {
  data?: T;
  error?: string;
  message?: string;
}


// Garage Cluster types
export interface ClusterHealth {
  status: string;
  connectedNodes: number;
  knownNodes: number;
  storageNodes: number;
  storageNodesUp: number;
  partitions: number;
  partitionsQuorum: number;
  partitionsAllOk: number;
}

export interface ClusterStatistics {
  timestamp: number;
  uptime: number;
  freeform: string;
  [key: string]: any;
}

export interface ClusterStatus {
  layoutVersion: number;
  nodes: ClusterNode[];
}

export interface ClusterNode {
  id: string;
  isUp: boolean;
  lastSeenSecsAgo?: number;
  hostname?: string;
  addr?: string;
  garageVersion?: string;
  role?: NodeRole;
  draining: boolean;
  dataPartition?: FreeSpaceInfo;
  metadataPartition?: FreeSpaceInfo;
}

export interface NodeRole {
  zone: string;
  capacity?: number;
  tags: string[];
}

export interface FreeSpaceInfo {
  available: number;
  total: number;
}

export interface LocalNodeInfo {
  nodeId: string;
  garageVersion: string;
  rustVersion: string;
  dbEngine: string;
  garageFeatures?: string[];
}

export interface MultiNodeResponse {
  success: Record<string, LocalNodeInfo>;
  error: Record<string, string>;
}

export interface NodeStatistics {
  freeform: string;
}

export interface MultiNodeStatisticsResponse {
  success: Record<string, NodeStatistics>;
  error: Record<string, string>;
}

export interface AccessControlBinding {
  bucket_prefixes: string[];
  permissions: string[];
}

export interface AccessControlInfo {
  enabled: boolean;
  subject?: string;
  is_admin?: boolean;
  bindings?: AccessControlBinding[];
  cluster_permissions?: string[];
}

export interface GarageCapabilities {
  garageApiVersion: string;
  features: {
    clusterStatistics: boolean;
    nodeInfo: boolean;
    nodeStatistics: boolean;
    cleanupUploads: boolean;
    workersAndBlocks: boolean;
    bucketAliases: boolean;
    clusterLayout: boolean;
  };
  access_control?: AccessControlInfo;
}

// Answers from every node, keyed by node ID, with the nodes that did not answer in error.
export interface PerNode<T> {
  success: Record<string, T>;
  error: Record<string, string>;
}

export interface WorkerInfo {
  id: number;
  name: string;
  state: 'busy' | 'idle' | 'done' | { throttled: { durationSecs: number } };
  errors: number;
  consecutiveErrors: number;
  lastError?: { message: string; secsAgo: number } | null;
  tranquility?: number | null;
  progress?: string | null;
  queueLength?: number | null;
  persistentErrors?: number | null;
  freeform: string[];
}

export interface BlockError {
  blockHash: string;
  refcount: number;
  errorCount: number;
  lastTrySecsAgo: number;
  nextTryInSecs: number;
}

export interface BlockVersion {
  versionId: string;
  refDeleted: boolean;
  versionDeleted: boolean;
  garbageCollected: boolean;
  backlink?:
    | { object: { bucketId: string; key: string } }
    | { upload: { uploadId: string; uploadDeleted: boolean; uploadGarbageCollected: boolean; bucketId?: string | null; key?: string | null } }
    | null;
}

export interface BlockInfo {
  blockHash: string;
  refcount: number;
  versions: BlockVersion[];
}

export interface NodeInfo {
  nodeId: string;
  version: string;
  rustVersion: string;
  uptime: number;
  dbSize: number;
  blockReferenceTableSize: number;
  blockMetricsTableSize: number;
  objectTableSize: number;
  objectVersionTableSize: number;
  bucketTableSize: number;
  bucketAliasTableSize: number;
  [key: string]: any;
}

export interface RequestTypeMetrics {
  read: number;
  write: number;
  delete: number;
  list: number;
}

export interface GarageMetrics extends StorageMetrics {
  clusterHealth?: ClusterHealth;
  clusterStatistics?: ClusterStatistics;
  nodeInfo?: NodeInfo;
  requestTypeMetrics?: RequestTypeMetrics;
}

export interface LayoutParameters {
  zoneRedundancy: 'maximum' | { atLeast: number };
}

export interface LayoutNodeRole {
  id: string;
  zone: string;
  tags: string[];
  // Absent on a gateway, which stores no data.
  capacity: number | null;
  storedPartitions: number | null;
  usableCapacity: number | null;
}

export type LayoutRoleChange =
  | { id: string; remove: true }
  | { id: string; remove?: undefined; zone: string; tags: string[]; capacity: number | null };

export interface ClusterLayout {
  version: number;
  roles: LayoutNodeRole[];
  parameters: LayoutParameters;
  partitionSize: number;
  stagedRoleChanges: LayoutRoleChange[];
  stagedParameters: LayoutParameters | null;
}

// Garage before 2.5 sends no statistics.
export interface LayoutStatistics {
  effectiveCapacity: number;
  lowPartitionSize: boolean;
  lowUsableCapacity: boolean;
  totalMovedPartitions: number | null;
}

export type LayoutPreview =
  | { error: string }
  | { message: string[]; statistics?: LayoutStatistics; newLayout: ClusterLayout };

export interface LayoutHistory {
  currentVersion: number;
  minAck: number;
  versions: { version: number; status: 'Current' | 'Draining' | 'Historical'; storageNodes: number; gatewayNodes: number }[];
  // Present only while several layout versions are live, which means data is moving.
  updateTrackers: Record<string, { ack: number; sync: number; syncAck: number }> | null;
}
