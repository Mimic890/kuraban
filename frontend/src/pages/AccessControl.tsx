import {useEffect, useMemo, useRef, useState} from 'react';
import {cn} from '@/lib/utils';
import {PageHeader} from '@/components/ui/page-header';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Badge} from '@/components/ui/badge';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { IconTile } from '@/components/ui/icon-tile';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {Tabs, TabsContent} from '@/components/ui/tabs';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Checkbox} from '@/components/ui/checkbox';
import {Select, SelectOption} from '@/components/ui/select';
import {useQueryClient} from '@tanstack/react-query';
import { useClusterStore } from '@/store/cluster-store';
import { Navigate } from 'react-router-dom';
import {accessApi, bucketsApi} from '@/lib/api';
import {queryKeys} from '@/lib/query-client';
import { useTranslation } from '@/lib/i18n';
import {dateTimeInputToIso, dateTimeInputValue} from '@/lib/utils';
import type {AccessKey, Bucket, BucketPermission} from '@/types';
import {AlertTriangle, Calendar, Check, Copy, Database, Edit, Eye, EyeOff, Key, KeyRound, Loader2, MoreVertical, Plus, Search, ShieldCheck, ShieldX, Trash2,} from 'lucide-react';
import {toast} from 'sonner';
import { copyText } from '@/lib/clipboard';

function CredentialField({
  label,
  value,
  mono = true,
  breakAll = false,
  maskable = false,
  loading = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
  breakAll?: boolean;
  maskable?: boolean;
  loading?: boolean;
}) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(!maskable);
  const copy = async () => {
    if (!value) return;
    copyText(value, t('access_control.credential_copied').replace('{{credential}}', label));
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };
  const display = loading ? '' : revealed || !maskable ? value : '•'.repeat(Math.min(40, value.length || 40));
  return (
    <div className="space-y-1.5">
      <label className="text-[0.75rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
        {label}
      </label>
      <div className="flex items-stretch gap-2">
        <button
          type="button"
          onClick={copy}
          disabled={loading || !value}
          title={t('access_control.click_to_copy_title')}
          className={cn(
            'flex-1 min-w-0 rounded-md border border-[var(--border)] bg-[var(--surface-sunken)]',
            'px-3 py-2 text-left text-[0.8438rem] transition-colors hover:bg-[var(--accent)]',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]',
            'disabled:cursor-not-allowed disabled:opacity-70 disabled:hover:bg-[var(--surface-sunken)]',
            mono && 'font-mono',
            breakAll ? 'break-all' : 'truncate',
          )}
        >
          {loading ? (
            <span className="inline-flex items-center gap-2 text-[var(--muted-foreground)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              {t('common.loading')}
            </span>
          ) : (
            display
          )}
        </button>
        {maskable && (
          <Button
            variant="secondary"
            size="icon"
            onClick={() => setRevealed((r) => !r)}
            aria-label={revealed ? t('access_control.hide_secret_aria') : t('access_control.reveal_secret_aria')}
            disabled={loading || !value}
          >
            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        )}
        <Button variant="secondary" size="icon" onClick={copy} aria-label={t('access_control.copy_credential_aria').replace('{{credential}}', label)} disabled={loading || !value}>
          {copied ? <Check className="h-4 w-4 text-[var(--primary)]" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}

export function AccessControl() {
  const queryClient = useQueryClient();
  const { t, language } = useTranslation();
  const formatLocalizedDate = (date: Date | string) => new Intl.DateTimeFormat(language, {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  }).format(new Date(date));
  const [keys, setKeys] = useState<AccessKey[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<AccessKey | null>(null);
  const [newKeyName, setNewKeyName] = useState('');
  const { clusters } = useClusterStore();

  // Create key with permissions state
  const [createAvailableBuckets, setCreateAvailableBuckets] = useState<Bucket[]>([]);
  const [createSelectedBucket, setCreateSelectedBucket] = useState<string>('');
  const [createPermissionRead, setCreatePermissionRead] = useState(false);
  const [createPermissionWrite, setCreatePermissionWrite] = useState(false);
  const [createPermissionOwner, setCreatePermissionOwner] = useState(false);
  const [createGrantPermissions, setCreateGrantPermissions] = useState(false);
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<AccessKey | null>(null);

  // Edit permissions state
  const [editPermissionsDialogOpen, setEditPermissionsDialogOpen] = useState(false);
  const [editingKey, setEditingKey] = useState<AccessKey | null>(null);
  const [availableBuckets, setAvailableBuckets] = useState<Bucket[]>([]);
  const [selectedBucket, setSelectedBucket] = useState<string>('');
  const [permissionRead, setPermissionRead] = useState(false);
  const [permissionWrite, setPermissionWrite] = useState(false);
  const [permissionOwner, setPermissionOwner] = useState(false);
  const [savingPermissions, setSavingPermissions] = useState(false);

  // Key settings state (activation/expiration)
  const [settingsDialogOpen, setSettingsDialogOpen] = useState(false);
  const [settingsKey, setSettingsKey] = useState<AccessKey | null>(null);
  const [keyStatus, setKeyStatus] = useState<'active' | 'inactive'>('active');
  const [expirationDate, setExpirationDate] = useState<string>('');
  const [neverExpires, setNeverExpires] = useState(true);

  // Secret key dialog state
  const [secretKeyDialogOpen, setSecretKeyDialogOpen] = useState(false);
  const [revealedSecretKey, setRevealedSecretKey] = useState<string>('');
  const [isLoadingSecretKey, setIsLoadingSecretKey] = useState(false);

  // Key details dialog state
  const [keyDetailsDialogOpen, setKeyDetailsDialogOpen] = useState(false);
  const [viewingKey, setViewingKey] = useState<AccessKey | null>(null);
  const [detailsSecretKey, setDetailsSecretKey] = useState<string>('');
  const [isLoadingDetailsSecretKey, setIsLoadingDetailsSecretKey] = useState(false);
  const secretRequestRef = useRef(0);
  const creatingKeyRef = useRef(false);

  useEffect(() => {
    const fetchKeys = async () => {
      try {
        setIsLoading(true);
        const data = await accessApi.listKeys();
        setKeys(data);
      } catch (error) {
        console.error('Failed to fetch keys:', error);
      } finally {
        setIsLoading(false);
      }
    };

    fetchKeys();
  }, []);

  const filteredKeys = useMemo(
    () =>
      keys.filter(
        (key) =>
          key.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          key.accessKeyId.toLowerCase().includes(searchQuery.toLowerCase())
      ),
    [keys, searchQuery]
  );

  if (clusters.length === 0) {
    return <Navigate to="/" replace />;
  }

  const handleCreateKey = async () => {
    if (creatingKeyRef.current) return;
    if (!newKeyName) {
      toast.error(t('access_control.key_name_required_error'));
      return;
    }

    try {
      creatingKeyRef.current = true;
      const newKey = await accessApi.createKey(newKeyName);

      // If user wants to grant permissions inline
      if (createGrantPermissions && createSelectedBucket) {
        if (createPermissionRead || createPermissionWrite || createPermissionOwner) {
          try {
            await bucketsApi.grantPermission(createSelectedBucket, newKey.accessKeyId, {
              read: createPermissionRead,
              write: createPermissionWrite,
              owner: createPermissionOwner,
            });
          } catch (error) {
            console.error('Failed to grant permissions:', error);
            toast.warning(t('access_control.key_created_permission_failed_warning').replace('{{bucket}}', createSelectedBucket));
            // Continue even if permission grant fails - key is already created
          }
        }
      }

      // Store the newly created key to show the secret key
      setNewlyCreatedKey(newKey);

      // Refresh keys list
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      if (createGrantPermissions && createSelectedBucket) {
         queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      }
      toast.success(t('access_control.key_created_success').replace('{{name}}', newKeyName));
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Create key error:', error);
    } finally {
      creatingKeyRef.current = false;
    }
  };

  const handleCloseCreateDialog = () => {
    setCreateDialogOpen(false);
    setNewKeyName('');
    setCreateSelectedBucket('');
    setCreatePermissionRead(false);
    setCreatePermissionWrite(false);
    setCreatePermissionOwner(false);
    setCreateGrantPermissions(false);
    setNewlyCreatedKey(null);
  };

  const handleOpenCreateDialog = async () => {
    setCreateDialogOpen(true);
    setNewKeyName('');
    setCreateSelectedBucket('');
    setCreatePermissionRead(false);
    setCreatePermissionWrite(false);
    setCreatePermissionOwner(false);
    setCreateGrantPermissions(false);

    // Load available buckets
    try {
      const buckets = await bucketsApi.list();
      setCreateAvailableBuckets(buckets);
    } catch (error) {
      console.error('Failed to load buckets:', error);
    }
  };

  const handleDeleteKey = async () => {
    if (!selectedKey) return;

    try {
      await accessApi.deleteKey(selectedKey.accessKeyId);
      const keyName = selectedKey.name;
      setDeleteDialogOpen(false);
      setSelectedKey(null);

      // Refresh keys list
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      toast.success(t('access_control.key_deleted_success').replace('{{name}}', keyName));
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Delete key error:', error);
    }
  };

  const handleOpenSettings = (key: AccessKey) => {
    setSettingsKey(key);
    setKeyStatus(key.status);
    setSettingsDialogOpen(true);

    // Set expiration date if it exists
    if (key.expiration) {
      const expDate = new Date(key.expiration);
      // Format as YYYY-MM-DDTHH:mm for datetime-local input
      const formattedDate = dateTimeInputValue(expDate);
      setExpirationDate(formattedDate);
      setNeverExpires(false);
    } else {
      setExpirationDate('');
      setNeverExpires(true);
    }
  };

  const handleSaveKeySettings = async () => {
    if (!settingsKey) return;

    try {
      const updates: { status?: string; expiration?: string | null } = {};

      updates.status = keyStatus;

      if (!neverExpires && expirationDate) {
         updates.expiration = dateTimeInputToIso(expirationDate);
      } else if (neverExpires) {
        // Clear any previously-set expiration so the backend removes it.
        updates.expiration = null;
      }

      await accessApi.updateKey(settingsKey.accessKeyId, updates);

      // Refresh keys list
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });

      setSettingsDialogOpen(false);
      toast.success(t('access_control.key_settings_updated_success'));
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Update key settings error:', error);
    }
  };

  const handleRevealSecretKey = async (key: AccessKey) => {
    const request = ++secretRequestRef.current;
    setSelectedKey(key);
    setIsLoadingSecretKey(true);
    setSecretKeyDialogOpen(true);
    setRevealedSecretKey('');

    try {
      const secretKey = await accessApi.getSecretKey(key.accessKeyId);
      if (request === secretRequestRef.current) setRevealedSecretKey(secretKey);
    } catch (error) {
      console.error('Failed to fetch secret key:', error);
      setSecretKeyDialogOpen(false);
    } finally {
      if (request === secretRequestRef.current) setIsLoadingSecretKey(false);
    }
  };

  const handleOpenEditPermissions = async (key: AccessKey) => {
    setEditingKey(key);
    setEditPermissionsDialogOpen(true);
    setSelectedBucket('');
    setPermissionRead(false);
    setPermissionWrite(false);
    setPermissionOwner(false);

    // Load available buckets
    try {
      const buckets = await bucketsApi.list();
      setAvailableBuckets(buckets);
    } catch (error) {
      console.error('Failed to load buckets:', error);
    }
  };

  const handleBucketChange = (bucketName: string) => {
    setSelectedBucket(bucketName);

    if (!bucketName || !editingKey) {
      // Reset permissions if no bucket selected
      setPermissionRead(false);
      setPermissionWrite(false);
      setPermissionOwner(false);
      return;
    }

    // Find if this key already has permissions on the selected bucket
    const bucketPermission = editingKey.permissions.find(
      perm => perm.bucketName === bucketName || perm.bucketId === bucketName
    );

    if (bucketPermission) {
      // Set the checkboxes to reflect current permissions
      setPermissionRead(bucketPermission.read);
      setPermissionWrite(bucketPermission.write);
      setPermissionOwner(bucketPermission.owner);
    } else {
      // No permissions set yet, reset checkboxes
      setPermissionRead(false);
      setPermissionWrite(false);
      setPermissionOwner(false);
    }
  };

  const handleGrantBucketPermission = async () => {
    if (!editingKey || !selectedBucket) {
      toast.error(t('access_control.bucket_required_error'));
      return;
    }

    setSavingPermissions(true);
    try {
      // Call backend API to grant bucket permissions
      await bucketsApi.grantPermission(selectedBucket, editingKey.accessKeyId, {
        read: permissionRead,
        write: permissionWrite,
        owner: permissionOwner,
      });

      toast.success(t('access_control.permissions_granted_success').replace('{{bucket}}', selectedBucket));
      setEditPermissionsDialogOpen(false);
      setSelectedBucket('');
      setPermissionRead(false);
      setPermissionWrite(false);
      setPermissionOwner(false);

      // Refresh keys list to update permissions
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
       queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Grant permission error:', error);
    } finally {
      setSavingPermissions(false);
    }
  };

  // Helper function to format permission flags as a readable string
  const formatPermissions = (perm: BucketPermission): string => {
    const perms = [];
    if (perm.read) perms.push(t('access_control.permission_read'));
    if (perm.write) perms.push(t('access_control.permission_write'));
    if (perm.owner) perms.push(t('access_control.permission_owner'));
    return perms.join(', ') || t('access_control.permission_none');
  };

  const handleRowClick = (key: AccessKey) => {
    // Opening the details dialog must NOT fetch the secret key. The secret is
    // loaded only when the user explicitly clicks the reveal control below.
    setViewingKey(key);
    setKeyDetailsDialogOpen(true);
    setDetailsSecretKey('');
    setIsLoadingDetailsSecretKey(false);
  };

  const handleRevealDetailsSecret = async () => {
    if (!viewingKey) return;
    const request = ++secretRequestRef.current;
    setIsLoadingDetailsSecretKey(true);
    try {
      const secretKey = await accessApi.getSecretKey(viewingKey.accessKeyId);
      if (request === secretRequestRef.current) setDetailsSecretKey(secretKey);
    } catch (error) {
      console.error('Failed to fetch secret key:', error);
    } finally {
      if (request === secretRequestRef.current) setIsLoadingDetailsSecretKey(false);
    }
  };

  return (
    <div>
      <PageHeader title={t('nav.access')} subtitle={t('access_control.page_subtitle')} />
      <div className="p-4 sm:p-6 space-y-4 sm:space-y-6">
        <Tabs defaultValue="keys">
          <TabsContent value="keys" className="space-y-4 sm:space-y-6 mt-4 sm:mt-6">
            {/* Stats */}
            <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">{t('access_control.total_keys')}</CardTitle>
                  <Key className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                   <div className="text-2xl font-bold">{keys.length.toLocaleString(language)}</div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">{t('access_control.active_keys')}</CardTitle>
                  <ShieldCheck className="h-4 w-4 text-green-600" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                     {keys.filter((k) => k.status === 'active').length.toLocaleString(language)}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">{t('access_control.inactive_keys')}</CardTitle>
                  <ShieldX className="h-4 w-4 text-red-600" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                     {keys.filter((k) => k.status === 'inactive').length.toLocaleString(language)}
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Toolbar */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
              <div className="relative flex-1 max-w-full sm:max-w-xs">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder={t('access_control.search_keys_placeholder')}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8"
                />
              </div>
              <Button onClick={handleOpenCreateDialog} className="w-full sm:w-auto">
                <Plus className="h-4 w-4" />
                 {t('access_control.create_key_button')}
              </Button>
            </div>

            {/* Keys Table */}
            <div className="border rounded-lg overflow-visible">
              <div className="overflow-x-auto">
                <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('access_control.name_column')}</TableHead>
                    <TableHead className="hidden sm:table-cell">{t('access_control.access_key_id_label')}</TableHead>
                    <TableHead>{t('access_control.status_label')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('access_control.created_label')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('access_control.permissions_label')}</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-12">
                        <div className="flex items-center justify-center gap-2 text-muted-foreground">
                          <Loader2 className="h-5 w-5 animate-spin" />
                           <span>{t('access_control.loading_keys')}</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : filteredKeys.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-12 text-muted-foreground">
                        {searchQuery ? t('access_control.no_search_results') : t('access_control.no_keys')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredKeys.map((key) => (
                      <TableRow
                        key={key.accessKeyId}
                        onClick={() => handleRowClick(key)}
                        className="cursor-pointer hover:bg-muted/50"
                      >
                        <TableCell className="font-medium truncate max-w-[150px]">{key.name}</TableCell>
                        <TableCell className="hidden sm:table-cell">
                          <div className="flex items-center gap-2">
                            <code
                              className="text-xs bg-muted px-2 py-1 rounded truncate max-w-[150px] block cursor-pointer hover:bg-muted/80 transition-colors"
                              onClick={(e) => {
                                e.stopPropagation();
                                copyText(key.accessKeyId, t('access_control.access_key_id_copied'));
                              }}
                            >
                              {key.accessKeyId}
                            </code>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 flex-shrink-0"
                              onClick={(e) => {
                                e.stopPropagation();
                                copyText(key.accessKeyId, t('access_control.access_key_id_copied'));
                              }}
                            >
                              <Copy className="h-3 w-3" />
                            </Button>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant={key.status === 'active' ? 'primary' : 'neutral'}>
                             {t(`access_control.status_${key.status}`)}
                          </Badge>
                        </TableCell>
                        <TableCell className="hidden md:table-cell">{formatLocalizedDate(key.createdAt)}</TableCell>
                        <TableCell className="hidden md:table-cell">
                          <div className="flex flex-wrap gap-1">
                            {key.permissions.slice(0, 2).map((perm, idx) => (
                              <Badge key={idx} variant="neutral" className="text-xs">
                                {perm.bucketName}: {formatPermissions(perm)}
                              </Badge>
                            ))}
                            {key.permissions.length > 2 && (
                              <Badge variant="neutral" className="text-xs">
                                 {t('access_control.more_permissions').replace('{{count}}', (key.permissions.length - 2).toLocaleString(language))}
                              </Badge>
                            )}
                            {key.permissions.length === 0 && (
                               <span className="text-xs text-muted-foreground">{t('access_control.no_permissions')}</span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon">
                                <MoreVertical className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem onClick={() => handleRevealSecretKey(key)}>
                                <Key className="h-4 w-4" />
                                 {t('access_control.view_secret_key_action')}
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem onClick={() => handleOpenEditPermissions(key)}>
                                <Edit className="h-4 w-4" />
                                 {t('access_control.edit_permissions_action')}
                              </DropdownMenuItem>
                              <DropdownMenuItem onClick={() => handleOpenSettings(key)}>
                                {key.status === 'active' ? (
                                  <>
                                    <ShieldX className="h-4 w-4" />
                                     {t('access_control.manage_status_action')}
                                  </>
                                ) : (
                                  <>
                                    <ShieldCheck className="h-4 w-4" />
                                     {t('access_control.manage_status_action')}
                                  </>
                                )}
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                className="text-destructive"
                                onClick={() => {
                                  setSelectedKey(key);
                                  setDeleteDialogOpen(true);
                                }}
                              >
                                <Trash2 className="h-4 w-4" />
                                 {t('common.delete')}
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </div>

      {/* Create Key Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={handleCloseCreateDialog} size="form">
        <DialogContent>
          {newlyCreatedKey ? (
            <>
              <DialogHeader>
                <IconTile icon={<ShieldCheck />} tone="primary" size="md" />
                <div className="min-w-0 flex-1">
                   <DialogTitle>{t('access_control.created_dialog_title')}</DialogTitle>
                  <DialogDescription>
                     {t('access_control.created_dialog_description')}
                  </DialogDescription>
                </div>
              </DialogHeader>
              <DialogBody className="space-y-5">
                <div className="space-y-1.5">
                  <label className="text-[0.75rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                     {t('access_control.key_name_label')}
                  </label>
                  <div className="text-[0.875rem] font-medium">{newlyCreatedKey.name}</div>
                </div>
                 <CredentialField label={t('access_control.access_key_id_label')} value={newlyCreatedKey.accessKeyId} />
                <CredentialField
                   label={t('access_control.secret_access_key_label')}
                  value={newlyCreatedKey.secretKey || ''}
                  breakAll
                />
                <div className="flex gap-3 rounded-lg border border-[var(--accent-primary-border)] bg-[var(--accent-primary-soft)] px-3.5 py-3">
                  <AlertTriangle className="h-4 w-4 flex-shrink-0 text-[var(--primary)] mt-0.5" />
                  <div className="space-y-0.5">
                    <p className="text-[0.8438rem] font-medium text-[var(--foreground)]">
                       {t('access_control.save_key_warning_title')}
                    </p>
                    <p className="text-[0.7812rem] leading-[1.5] text-[var(--muted-foreground)]">
                       {t('access_control.save_key_warning_description')}
                    </p>
                  </div>
                </div>
              </DialogBody>
              <DialogFooter>
                 <Button onClick={handleCloseCreateDialog}>{t('common.done')}</Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <DialogHeader>
                <IconTile icon={<KeyRound />} tone="primary" size="md" />
                <div className="min-w-0 flex-1">
                   <DialogTitle>{t('access_control.create_dialog_title')}</DialogTitle>
                  <DialogDescription>
                     {t('access_control.create_dialog_description')}
                  </DialogDescription>
                </div>
              </DialogHeader>
              <DialogBody className="space-y-6">
                <div className="space-y-1.5">
                  <label htmlFor="new-key-name" className="text-[0.8125rem] font-medium">
                     {t('access_control.key_name_label')}
                  </label>
                  <Input
                    id="new-key-name"
                     placeholder={t('access_control.key_name_placeholder')}
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    autoFocus
                  />
                  <p className="text-[0.7812rem] text-[var(--muted-foreground)]">
                     {t('access_control.key_name_help')}
                  </p>
                </div>

                <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
                  <label className="flex cursor-pointer items-start gap-3">
                    <Checkbox
                      id="grant-permissions-on-create"
                      checked={createGrantPermissions}
                      className="mt-0.5"
                      onCheckedChange={(checked) => {
                        setCreateGrantPermissions(checked as boolean);
                        if (!checked) {
                          setCreateSelectedBucket('');
                          setCreatePermissionRead(false);
                          setCreatePermissionWrite(false);
                          setCreatePermissionOwner(false);
                        }
                      }}
                    />
                    <div className="flex-1">
                       <div className="text-[0.8438rem] font-medium">{t('access_control.grant_permissions_now')}</div>
                      <p className="mt-0.5 text-[0.7812rem] text-[var(--muted-foreground)]">
                         {t('access_control.grant_permissions_help')}
                      </p>
                    </div>
                  </label>

                  {createGrantPermissions && (
                    <div className="space-y-4 border-t border-[var(--border)] pt-4">
                      <div className="space-y-1.5">
                         <label className="text-[0.8125rem] font-medium">{t('access_control.bucket_label')}</label>
                        <Select
                          value={createSelectedBucket}
                          onChange={(value) => setCreateSelectedBucket(value)}
                        >
                           <SelectOption value="">{t('access_control.select_bucket_placeholder')}</SelectOption>
                          {createAvailableBuckets.map((bucket) => (
                            <SelectOption key={bucket.name} value={bucket.name}>
                              {bucket.name}
                            </SelectOption>
                          ))}
                        </Select>
                      </div>

                      {createSelectedBucket && (
                        <div className="space-y-1.5">
                           <label className="text-[0.8125rem] font-medium">{t('access_control.permissions_label')}</label>
                          <div className="divide-y divide-[var(--border)] rounded-md border border-[var(--border)]">
                            {[
                              {
                                id: 'create-permission-read',
                                 label: t('access_control.permission_read'),
                                desc: 'GetObject, HeadObject, ListObjects',
                                checked: createPermissionRead,
                                setChecked: setCreatePermissionRead,
                              },
                              {
                                id: 'create-permission-write',
                                 label: t('access_control.permission_write'),
                                desc: 'PutObject, DeleteObject',
                                checked: createPermissionWrite,
                                setChecked: setCreatePermissionWrite,
                              },
                              {
                                id: 'create-permission-owner',
                                 label: t('access_control.permission_owner'),
                                desc: 'DeleteBucket, PutBucketPolicy',
                                checked: createPermissionOwner,
                                setChecked: setCreatePermissionOwner,
                              },
                            ].map((p) => (
                              <label
                                key={p.id}
                                htmlFor={p.id}
                                className="flex cursor-pointer items-start gap-3 px-3.5 py-3 transition-colors hover:bg-[var(--accent)]"
                              >
                                <Checkbox
                                  id={p.id}
                                  checked={p.checked}
                                  className="mt-0.5"
                                  onCheckedChange={(checked) => p.setChecked(checked as boolean)}
                                />
                                <div className="flex-1">
                                  <div className="text-[0.8438rem] font-medium">{p.label}</div>
                                  <p className="mt-0.5 font-mono text-[0.75rem] text-[var(--muted-foreground)]">
                                    {p.desc}
                                  </p>
                                </div>
                              </label>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </DialogBody>
              <DialogFooter>
                <Button variant="secondary" onClick={handleCloseCreateDialog}>
                   {t('common.cancel')}
                </Button>
                <Button onClick={handleCreateKey} disabled={!newKeyName}>
                   {t('access_control.create_key_button')}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete Key Dialog */}
      <ConfirmDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        title={t('access_control.delete_dialog_title').replace('{{name}}', selectedKey?.name ?? '')}
        description={t('access_control.delete_dialog_description')}
        confirmLabel={t('access_control.delete_key_button')}
        onConfirm={handleDeleteKey}
      />

      {/* Secret Key Dialog */}
      <Dialog open={secretKeyDialogOpen} onOpenChange={setSecretKeyDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<KeyRound />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
               <DialogTitle className="truncate">{selectedKey?.name || t('access_control.access_key_fallback')}</DialogTitle>
              <DialogDescription>
                 {t('access_control.secret_dialog_description')}
              </DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-4">
            <CredentialField
               label={t('access_control.access_key_id_label')}
              value={selectedKey?.accessKeyId || ''}
              breakAll
            />
            <CredentialField
               label={t('access_control.secret_access_key_label')}
              value={revealedSecretKey}
              breakAll
              maskable
              loading={isLoadingSecretKey}
            />
          </DialogBody>
          <DialogFooter>
             <Button onClick={() => setSecretKeyDialogOpen(false)}>{t('common.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Key Settings Dialog */}
      <Dialog open={settingsDialogOpen} onOpenChange={setSettingsDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<ShieldCheck />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
               <DialogTitle className="truncate">{t('access_control.settings_dialog_title').replace('{{name}}', settingsKey?.name ?? '')}</DialogTitle>
               <DialogDescription>{t('access_control.settings_dialog_description')}</DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-6">
            <div className="space-y-2">
              <label className="text-[0.75rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                 {t('access_control.status_label')}
              </label>
              <div className="grid grid-cols-2 gap-2">
                {(['active', 'inactive'] as const).map((s) => {
                  const selected = keyStatus === s;
                  return (
                    <button
                      key={s}
                      type="button"
                      onClick={() => setKeyStatus(s)}
                      className={cn(
                        'flex items-center justify-center gap-2 rounded-md border px-3 py-2.5 text-[0.8438rem] font-medium transition-colors',
                        selected
                          ? 'border-[var(--primary)] bg-[var(--accent-primary-soft)] text-[var(--foreground)]'
                          : 'border-[var(--border)] text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)]',
                      )}
                    >
                      {s === 'active' ? <ShieldCheck className="h-4 w-4" /> : <ShieldX className="h-4 w-4" />}
                       {t(`access_control.status_${s}`)}
                    </button>
                  );
                })}
              </div>
              <p className="text-[0.7812rem] text-[var(--muted-foreground)]">
                 {t('access_control.inactive_help')}
              </p>
            </div>

            <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
              <label className="flex cursor-pointer items-start gap-3">
                <Checkbox
                  id="never-expires"
                  checked={neverExpires}
                  className="mt-0.5"
                  onCheckedChange={(checked) => setNeverExpires(checked as boolean)}
                />
                <div className="flex-1">
                   <div className="text-[0.8438rem] font-medium">{t('access_control.never_expires_label')}</div>
                  <p className="mt-0.5 text-[0.7812rem] text-[var(--muted-foreground)]">
                     {t('access_control.never_expires_help')}
                  </p>
                </div>
              </label>

              {!neverExpires && (
                <div className="space-y-1.5 border-t border-[var(--border)] pt-4">
                   <label className="text-[0.8125rem] font-medium">{t('access_control.expiration_datetime_label')}</label>
                  <Input
                    type="datetime-local"
                    value={expirationDate}
                    onChange={(e) => setExpirationDate(e.target.value)}
                    className="w-full"
                  />
                  <p className="text-[0.7812rem] text-[var(--muted-foreground)]">
                     {t('access_control.expiration_help')}
                  </p>
                </div>
              )}
            </div>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setSettingsDialogOpen(false)}>
               {t('common.cancel')}
            </Button>
             <Button onClick={handleSaveKeySettings}>{t('access_control.save_settings_button')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Key Details Dialog */}
      <Dialog open={keyDetailsDialogOpen} onOpenChange={setKeyDetailsDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<KeyRound />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
               <DialogTitle className="truncate">{viewingKey?.name || t('access_control.api_key_fallback')}</DialogTitle>
               <DialogDescription>{t('access_control.details_dialog_description')}</DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-5">
            {/* Meta strip */}
            <div className="flex flex-wrap items-center gap-x-5 gap-y-2 rounded-lg border border-[var(--border)] bg-[var(--surface-sunken)] px-4 py-3">
              <div className="flex items-center gap-2">
                <span className="text-[0.6875rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                   {t('access_control.status_label')}
                </span>
                <Badge variant={viewingKey?.status === 'active' ? 'success' : 'neutral'}>
                   {viewingKey?.status && t(`access_control.status_${viewingKey.status}`)}
                </Badge>
              </div>
              <div className="flex items-center gap-2 text-[0.8125rem] text-[var(--muted-foreground)]">
                <Calendar className="h-3.5 w-3.5" />
                 <span className="text-[0.6875rem] font-medium uppercase tracking-[0.06em]">{t('access_control.created_label')}</span>
                <span className="text-[var(--foreground)]">
                   {viewingKey && formatLocalizedDate(viewingKey.createdAt)}
                </span>
              </div>
              {viewingKey?.expiration && (
                <div className="flex items-center gap-2 text-[0.8125rem] text-[var(--muted-foreground)]">
                  <Calendar className="h-3.5 w-3.5" />
                   <span className="text-[0.6875rem] font-medium uppercase tracking-[0.06em]">{t('access_control.expires_label')}</span>
                   <span className="text-[var(--foreground)]">{formatLocalizedDate(viewingKey.expiration)}</span>
                </div>
              )}
            </div>

            {/* Credentials */}
            <div className="space-y-4">
              <CredentialField
                 label={t('access_control.access_key_id_label')}
                value={viewingKey?.accessKeyId || ''}
                breakAll
              />
              <div className="space-y-1.5">
                <label className="text-[0.75rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                   {t('access_control.secret_access_key_label')}
                </label>
                {detailsSecretKey ? (
                  <CredentialField
                    label={t('access_control.secret_access_key_label')}
                    value={detailsSecretKey}
                    breakAll
                    maskable
                    loading={isLoadingDetailsSecretKey}
                  />
                ) : (
                  <Button
                    variant="secondary"
                    onClick={handleRevealDetailsSecret}
                    disabled={isLoadingDetailsSecretKey}
                    className="w-full"
                  >
                    {isLoadingDetailsSecretKey ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Eye className="h-4 w-4" />
                    )}
                     {t('access_control.reveal_secret_action')}
                  </Button>
                )}
              </div>
            </div>

            {/* Bucket Permissions */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-[0.75rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                   {t('access_control.bucket_permissions_label')}
                </label>
                {viewingKey && viewingKey.permissions.length > 0 && (
                  <span className="text-[0.75rem] text-[var(--muted-foreground)]">
                     {t(viewingKey.permissions.length === 1 ? 'access_control.one_bucket_count' : 'access_control.buckets_count')
                       .replace('{{count}}', viewingKey.permissions.length.toLocaleString(language))}
                  </span>
                )}
              </div>
              {viewingKey && viewingKey.permissions.length > 0 ? (
                <div className="divide-y divide-[var(--border)] overflow-hidden rounded-lg border border-[var(--border)]">
                  {viewingKey.permissions.map((perm, idx) => (
                    <div
                      key={idx}
                      className="flex items-center justify-between gap-3 px-3.5 py-2.5"
                    >
                      <div className="flex min-w-0 items-center gap-2.5">
                        <Database className="h-3.5 w-3.5 flex-shrink-0 text-[var(--muted-foreground)]" />
                        <span className="truncate font-mono text-[0.8125rem]">{perm.bucketName}</span>
                      </div>
                      <div className="flex flex-shrink-0 gap-1">
                         {perm.read && <Badge variant="neutral">{t('access_control.permission_read')}</Badge>}
                         {perm.write && <Badge variant="neutral">{t('access_control.permission_write')}</Badge>}
                         {perm.owner && <Badge variant="warning">{t('access_control.permission_owner')}</Badge>}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="rounded-lg border border-dashed border-[var(--border)] px-4 py-6 text-center">
                  <p className="text-[0.8125rem] text-[var(--muted-foreground)]">
                     {t('access_control.no_bucket_permissions')}
                  </p>
                </div>
              )}
            </div>
          </DialogBody>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                setKeyDetailsDialogOpen(false);
                if (viewingKey) {
                  handleOpenEditPermissions(viewingKey);
                }
              }}
            >
              <Edit className="h-4 w-4" />
               {t('access_control.edit_permissions_action')}
            </Button>
             <Button onClick={() => setKeyDetailsDialogOpen(false)}>{t('common.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Permissions Dialog */}
      <Dialog open={editPermissionsDialogOpen} onOpenChange={setEditPermissionsDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<Edit />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
               <DialogTitle className="truncate">{t('access_control.permissions_dialog_title').replace('{{name}}', editingKey?.name ?? '')}</DialogTitle>
              <DialogDescription>
                 {t('access_control.permissions_dialog_description')}
              </DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-5">
            <div className="space-y-1.5">
               <label className="text-[0.8125rem] font-medium">{t('access_control.bucket_label')}</label>
              <Select value={selectedBucket} onChange={(value) => handleBucketChange(value)}>
                 <SelectOption value="">{t('access_control.select_bucket_placeholder')}</SelectOption>
                {availableBuckets.map((bucket) => (
                  <SelectOption key={bucket.name} value={bucket.name}>
                    {bucket.name}
                  </SelectOption>
                ))}
              </Select>
            </div>

            {selectedBucket && (
              <>
                <div className="space-y-1.5">
                   <label className="text-[0.8125rem] font-medium">{t('access_control.permissions_label')}</label>
                  <div className="divide-y divide-[var(--border)] rounded-md border border-[var(--border)]">
                    {[
                      {
                        id: 'edit-permission-read',
                       label: t('access_control.permission_read'),
                        desc: 'GetObject, HeadObject, ListObjects',
                        checked: permissionRead,
                        setChecked: setPermissionRead,
                      },
                      {
                        id: 'edit-permission-write',
                       label: t('access_control.permission_write'),
                        desc: 'PutObject, DeleteObject',
                        checked: permissionWrite,
                        setChecked: setPermissionWrite,
                      },
                      {
                        id: 'edit-permission-owner',
                       label: t('access_control.permission_owner'),
                        desc: 'DeleteBucket, PutBucketPolicy',
                        checked: permissionOwner,
                        setChecked: setPermissionOwner,
                      },
                    ].map((p) => (
                      <label
                        key={p.id}
                        htmlFor={p.id}
                        className="flex cursor-pointer items-start gap-3 px-3.5 py-3 transition-colors hover:bg-[var(--accent)]"
                      >
                        <Checkbox
                          id={p.id}
                          checked={p.checked}
                          className="mt-0.5"
                          onCheckedChange={(checked) => p.setChecked(checked as boolean)}
                        />
                        <div className="flex-1">
                          <div className="text-[0.8438rem] font-medium">{p.label}</div>
                          <p className="mt-0.5 font-mono text-[0.75rem] text-[var(--muted-foreground)]">
                            {p.desc}
                          </p>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>

                {editingKey && (() => {
                  const current = editingKey.permissions.find(
                    (perm) => perm.bucketName === selectedBucket || perm.bucketId === selectedBucket,
                  );
                  const hasAny = current && (current.read || current.write || current.owner);
                  return (
                    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-sunken)] px-3.5 py-3">
                      <div className="text-[0.6875rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                         {t('access_control.currently_granted')}
                      </div>
                      {hasAny ? (
                        <div className="mt-1.5 flex flex-wrap gap-1.5">
                           {current!.read && <Badge variant="neutral">{t('access_control.permission_read')}</Badge>}
                           {current!.write && <Badge variant="neutral">{t('access_control.permission_write')}</Badge>}
                           {current!.owner && <Badge variant="warning">{t('access_control.permission_owner')}</Badge>}
                        </div>
                      ) : (
                        <p className="mt-1 text-[0.7812rem] text-[var(--muted-foreground)]">
                           {t('access_control.no_permissions_for_bucket')}
                        </p>
                      )}
                    </div>
                  );
                })()}
              </>
            )}

            {editingKey && editingKey.permissions.length > 0 && (
              <div className="space-y-1.5">
                <label className="text-[0.75rem] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                   {t('access_control.all_bucket_permissions')}
                </label>
                <div className="max-h-48 divide-y divide-[var(--border)] overflow-y-auto rounded-md border border-[var(--border)]">
                  {editingKey.permissions.map((perm, idx) => (
                    <div key={idx} className="flex items-center justify-between gap-3 px-3.5 py-2.5">
                      <span className="truncate font-mono text-[0.8125rem]">{perm.bucketName}</span>
                      <div className="flex flex-shrink-0 gap-1">
                         {perm.read && <Badge variant="neutral">{t('access_control.permission_read_short')}</Badge>}
                         {perm.write && <Badge variant="neutral">{t('access_control.permission_write_short')}</Badge>}
                         {perm.owner && <Badge variant="warning">{t('access_control.permission_owner_short')}</Badge>}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setEditPermissionsDialogOpen(false)}>
               {t('common.cancel')}
            </Button>
            <Button onClick={handleGrantBucketPermission} disabled={!selectedBucket || savingPermissions}>
              {savingPermissions && <Loader2 className="h-4 w-4 animate-spin" />}
               {savingPermissions ? t('access_control.saving_permissions') : t('access_control.save_permissions_button')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
