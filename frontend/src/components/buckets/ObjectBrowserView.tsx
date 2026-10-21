import {useEffect, useState} from 'react';
import {useDropzone} from 'react-dropzone';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {ObjectsTable} from './ObjectsTable';
import {CreateDirectoryDialog} from './CreateDirectoryDialog';
import {DeleteObjectDialog} from './DeleteObjectDialog';
import {ConfirmDialog} from '@/components/ui/confirm-dialog';
import {UploadProgress} from './UploadProgress';
import {ArrowLeft, ChevronRight, FolderPlus, Home, RotateCwIcon, ScanSearch, Search, Trash, Upload} from 'lucide-react';
import {getBreadcrumbs} from '@/lib/file-utils';
import type {S3Object, UploadTask} from '@/types';
import { toast } from 'sonner';
import { useTranslation } from '@/lib/i18n';

interface ObjectBrowserViewProps {
  bucketName: string;
  objects: S3Object[];
  currentPath: string;
  searchQuery: string;
  filterQuery: string;
  deepSearch: boolean;
  error?: Error | null;
  isLoading?: boolean;
  isTruncated?: boolean;
  nextContinuationToken?: string;
  itemsPerPage: number;
  onSearchChange: (query: string) => void;
  onDeepSearchChange: (enabled: boolean) => void;
  onNavigateToFolder: (path: string) => void;
  onBackToBuckets: () => void;
  onUploadFiles?: (files: File[]) => Promise<boolean>;
  uploadTasks: UploadTask[];
  onDeleteObject?: (key: string) => Promise<boolean>;
  onDeleteMultipleObjects?: (keys: string[], prefixes?: string[]) => Promise<boolean>;
  onCreateDirectory?: (name: string) => Promise<boolean>;
  onRefresh: () => Promise<void>;
  onPageChange: (token?: string) => void;
  onItemsPerPageChange: (count: number) => void;
  isRefreshing: boolean;
  isNavigating: boolean;
  initialPageToken?: string;
  initialItemsPerPage?: number;
}

export function ObjectBrowserView({
  bucketName,
  objects,
  currentPath,
  searchQuery,
  filterQuery,
  deepSearch,
  error,
  isLoading = false,
  isTruncated = false,
  nextContinuationToken,
  itemsPerPage,
  onSearchChange,
  onDeepSearchChange,
  onNavigateToFolder,
  onBackToBuckets,
  onUploadFiles,
  uploadTasks,
  onDeleteObject,
  onDeleteMultipleObjects,
  onCreateDirectory,
  onRefresh,
  onPageChange,
  onItemsPerPageChange,
  isRefreshing,
  isNavigating,
  initialPageToken,
  initialItemsPerPage,
}: ObjectBrowserViewProps) {
  const { t, language } = useTranslation();
  const [showUploadZone, setShowUploadZone] = useState(false);
  const [deleteObjectDialogOpen, setDeleteObjectDialogOpen] = useState(false);
  const [selectedObject, setSelectedObject] = useState<S3Object | null>(null);
  const [createDirDialogOpen, setCreateDirDialogOpen] = useState(false);
  const [selectedFileKeys, setSelectedFileKeys] = useState<Set<string>>(new Set());
  const [selectedFolderKeys, setSelectedFolderKeys] = useState<Set<string>>(new Set());
  // Holds the keys/prefixes awaiting confirmation in the bulk-delete dialog.
  const [pendingDelete, setPendingDelete] = useState<{ keys: string[]; prefixes: string[] } | null>(null);
  const [bulkDeleting, setBulkDeleting] = useState(false);

  // H1: clear selection when navigating between folders
  useEffect(() => {
    setSelectedFileKeys(new Set());
    setSelectedFolderKeys(new Set());
  }, [currentPath]);

  const {
    getRootProps: getTableRootProps,
    getInputProps: getTableInputProps,
    isDragActive,
  } = useDropzone({
    onDrop: async (acceptedFiles, _fileRejections, event) => {
      if (!onUploadFiles) return;

      // Get files with their full paths from DataTransferItems API
      const filesWithPaths: File[] = [];

      // Type cast event to DragEvent to access dataTransfer
      const dragEvent = event as DragEvent;

      if (dragEvent.dataTransfer?.items) {
        // Use DataTransferItemList API to preserve folder structure
        const items = Array.from(dragEvent.dataTransfer.items);
        await Promise.all(items.map(async (item: DataTransferItem) => {
          if (item.kind === 'file') {
            const entry = item.webkitGetAsEntry?.();
            if (entry) {
              await traverseFileTree(entry, '', filesWithPaths);
            }
          }
        }));
      } else {
        // Fallback to standard files
        filesWithPaths.push(...acceptedFiles);
      }

      await onUploadFiles(filesWithPaths.length > 0 ? filesWithPaths : acceptedFiles);
      setShowUploadZone(false);
    },
    noClick: true,
    disabled: !onUploadFiles,
  });

  const { getRootProps: getPanelRootProps } = useDropzone({
    onDrop: async (acceptedFiles) => {
      if (!onUploadFiles) return;
      await onUploadFiles(acceptedFiles);
      setShowUploadZone(false);
    },
    noClick: true,
    disabled: !onUploadFiles,
  });

  // Helper function to traverse file/directory tree
  const traverseFileTree = async (item: FileSystemEntry, path: string, files: File[]): Promise<void> => {
    if (item.isFile) {
      await new Promise<void>((resolve) => {
        (item as FileSystemFileEntry).file(
          (file) => {
            const fullPath = path + file.name;
            Object.defineProperty(file, 'webkitRelativePath', { value: fullPath, writable: false });
            files.push(file);
            resolve();
          },
          () => {
            toast.error(t('buckets.upload.errors.read_file', { name: `${path}${item.name}` }));
            resolve();
          },
        );
      });
      return;
    }
    if (!item.isDirectory) return;

    const dirReader = (item as FileSystemDirectoryEntry).createReader();
    const readAllEntries = () => new Promise<FileSystemEntry[]>((resolve, reject) => {
      const entries: FileSystemEntry[] = [];
      const step = () => dirReader.readEntries((batch) => {
        if (batch.length === 0) return resolve(entries);
        entries.push(...batch);
        step();
      }, reject);
      step();
    });
    try {
      const entries = await readAllEntries();
      for (const entry of entries) await traverseFileTree(entry, path + item.name + '/', files);
    } catch {
      toast.error(t('buckets.upload.errors.read_folder', { name: item.name }));
    }
  };

  const selectedCount = selectedFileKeys.size + selectedFolderKeys.size;
  const breadcrumbs = getBreadcrumbs(currentPath);

  const toggleInSet = (set: Set<string>, key: string) => {
    const next = new Set(set);
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    return next;
  };

  const handleToggleFileSelection = (key: string) => {
    setSelectedFileKeys(prev => toggleInSet(prev, key));
  };

  const handleToggleFolderSelection = (key: string) => {
    setSelectedFolderKeys(prev => toggleInSet(prev, key));
  };

  // Select/deselect the currently visible (filtered) rows. The table passes the
  // keys it is actually showing so this stays aligned with the search filter
  // instead of operating on the full, unfiltered object list.
  const handleSelectAll = (fileKeys: string[], folderKeys: string[]) => {
    const allVisibleSelected =
      fileKeys.length + folderKeys.length > 0 &&
      fileKeys.every(k => selectedFileKeys.has(k)) &&
      folderKeys.every(k => selectedFolderKeys.has(k));

    if (allVisibleSelected) {
      // Drop only the visible rows, leaving any off-screen selection intact.
      setSelectedFileKeys(prev => {
        const next = new Set(prev);
        fileKeys.forEach(k => next.delete(k));
        return next;
      });
      setSelectedFolderKeys(prev => {
        const next = new Set(prev);
        folderKeys.forEach(k => next.delete(k));
        return next;
      });
    } else {
      setSelectedFileKeys(prev => new Set([...prev, ...fileKeys]));
      setSelectedFolderKeys(prev => new Set([...prev, ...folderKeys]));
    }
  };

  // Open the confirmation dialog for the current multi-selection.
  const handleRequestBulkDelete = () => {
    if (selectedCount === 0) return;
    setPendingDelete({
      keys: Array.from(selectedFileKeys),
      prefixes: Array.from(selectedFolderKeys),
    });
  };

  // Open the confirmation dialog for a single folder (recursive delete).
  const handleDeleteFolder = (folderKey: string) => {
    setPendingDelete({ keys: [], prefixes: [folderKey] });
  };

  const handleConfirmBulkDelete = async () => {
    if (!pendingDelete || !onDeleteMultipleObjects) return;

    setBulkDeleting(true);
    const success = await onDeleteMultipleObjects(pendingDelete.keys, pendingDelete.prefixes);
    setBulkDeleting(false);

    if (success) {
      // Drop the deleted folders/files from the live selection.
      setSelectedFileKeys(prev => {
        const next = new Set(prev);
        pendingDelete.keys.forEach(k => next.delete(k));
        pendingDelete.prefixes.forEach(p => {
          for (const k of next) if (k.startsWith(p)) next.delete(k);
        });
        return next;
      });
      setSelectedFolderKeys(prev => {
        const next = new Set(prev);
        pendingDelete.prefixes.forEach(k => next.delete(k));
        return next;
      });
      setPendingDelete(null);
    }
  };

  const handleDeleteObject = async (key: string): Promise<boolean> => {
    if (!onDeleteObject) return false;
    const success = await onDeleteObject(key);
    if (success) {
      setDeleteObjectDialogOpen(false);
      setSelectedObject(null);
    }
    return success;
  };

  const uploadFiles = async (files: File[]) => {
    if (!onUploadFiles) return;
    await onUploadFiles(files);
    setShowUploadZone(false);
  };

  return (
    <div>
      <div className="p-4 sm:p-6 space-y-4 sm:space-y-6">
        {/* Back Button */}
        <Button variant="secondary" onClick={onBackToBuckets} className="text-sm sm:text-base">
          <ArrowLeft className="h-4 w-4" />
          <span className="hidden sm:inline">{t('buckets.actions.back_to_buckets')}</span>
          <span className="sm:hidden">{t('buckets.actions.back')}</span>
        </Button>

        {/* Breadcrumb Navigation */}
        <div className="flex items-center gap-2 text-xs sm:text-sm overflow-x-auto">
          <Home className="h-4 w-4 text-muted-foreground" />
          {breadcrumbs.map((crumb, index) => (
            <div key={index} className="flex items-center gap-2">
              {index > 0 && <ChevronRight className="h-4 w-4 text-muted-foreground" />}
              <button
                onClick={() => onNavigateToFolder(crumb.path)}
                className={
                  index === breadcrumbs.length - 1
                    ? 'font-medium'
                    : 'text-muted-foreground hover:text-foreground'
                }
              >
                {crumb.label}
              </button>
            </div>
          ))}
        </div>

        {/* Toolbar */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="flex flex-1 items-center gap-2 max-w-full sm:max-w-md">
            <div className="relative flex-1">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={deepSearch ? t('buckets.objects.deep_search_placeholder') : t('buckets.objects.prefix_search_placeholder')}
                value={searchQuery}
                onChange={(e) => onSearchChange(e.target.value)}
                className="pl-8"
              />
            </div>
            <Button
              type="button"
              variant={deepSearch ? 'primary' : 'secondary'}
              onClick={() => onDeepSearchChange(!deepSearch)}
              aria-pressed={deepSearch}
              title={
                deepSearch
                  ? t('buckets.objects.deep_search_on_tooltip')
                  : t('buckets.objects.prefix_search_tooltip')
              }
              className="shrink-0"
            >
              <ScanSearch className="h-4 w-4" />
              <span className="hidden sm:inline">{t('buckets.objects.deep_search_button')}</span>
            </Button>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            {onDeleteMultipleObjects && selectedCount > 0 && (
              <Button
                onClick={handleRequestBulkDelete}
                title={t('buckets.bulk_delete.selected_tooltip', { count: selectedCount.toLocaleString(language) })}
                className="bg-transparent border border-red-500 text-red-500 hover:bg-red-500/5"
              >
                <Trash className="h-4 w-4" />
                {t('buckets.bulk_delete.selected_button', { count: selectedCount.toLocaleString(language) })}
              </Button>
            )}
            {onUploadFiles && (
              <Button variant="secondary" onClick={() => setShowUploadZone(!showUploadZone)} className="flex-1 sm:flex-initial">
                <Upload className="h-4 w-4" />
                <span className="hidden sm:inline">{t('buckets.actions.upload')}</span>
              </Button>
            )}
            {onCreateDirectory && (
              <Button onClick={() => setCreateDirDialogOpen(true)} className="flex-1 sm:flex-initial">
                <FolderPlus className="h-4 w-4" />
                <span className="hidden sm:inline">{t('buckets.actions.add_directory')}</span>
              </Button>
            )}
            <Button variant="secondary" size="icon" onClick={onRefresh} title={t('buckets.actions.refresh')} aria-label={t('buckets.actions.refresh')} disabled={isRefreshing}>
              <RotateCwIcon className={`h-4 w-4 transition-transform duration-500 ${isRefreshing ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </div>

        {/* Upload Zone */}
        {onUploadFiles && showUploadZone && uploadTasks.length === 0 && (
          <div className="border rounded-lg p-6 bg-muted/30 space-y-4">
            <div className="flex gap-6">
              <div className="flex-shrink-0 flex items-center justify-center">
                <div className="w-20 h-20 bg-primary/10 rounded-lg flex items-center justify-center">
                  <svg
                    className="w-12 h-12 text-primary"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="17 8 12 3 7 8" />
                    <line x1="12" y1="3" x2="12" y2="15" />
                  </svg>
                </div>
              </div>

              <div className="flex-1 space-y-3">
                <div
                  {...getPanelRootProps()}
                  className={`border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ${
                    isDragActive
                      ? 'border-primary bg-primary/5'
                      : 'border-muted-foreground/25 hover:border-muted-foreground/50'
                  }`}
                >
                  <p className="text-sm">
                    {t('buckets.upload.drop_or')}{' '}
                    <label
                      htmlFor="file-input"
                      className="font-medium text-primary hover:underline cursor-pointer"
                    >
                      {t('buckets.upload.select_files')}
                    </label>
                    {' / '}
                    <label
                      htmlFor="folder-input"
                      className="font-medium text-primary hover:underline cursor-pointer"
                    >
                      {t('buckets.upload.select_folder')}
                    </label>
                  </p>
                  <input
                    id="file-input"
                    type="file"
                    multiple
                    onChange={(e) => {
                      if (e.target.files) {
                        const files = Array.from(e.target.files);
                        uploadFiles(files);
                        e.target.value = '';
                      }
                    }}
                    style={{ display: 'none' }}
                  />
                  <input
                    id="folder-input"
                    type="file"
                    {...({ webkitdirectory: '', directory: '', mozdirectory: '' } as any)}
                    onChange={(e) => {
                      if (e.target.files) {
                        const files = Array.from(e.target.files);
                        uploadFiles(files);
                        e.target.value = '';
                      }
                    }}
                    style={{ display: 'none' }}
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Upload Progress */}
        {uploadTasks.length > 0 && <UploadProgress tasks={uploadTasks} />}

        {/* Objects Table with Drag & Drop */}
        <div
          {...getTableRootProps()}
          className={`relative border rounded-lg transition-all duration-200 overflow-visible ${
            isDragActive
              ? 'border-primary bg-primary/5 border-2 shadow-lg'
              : 'border-border'
          }`}
        >
          <input {...getTableInputProps()} />

          {/* Drag & Drop Overlay */}
          {isDragActive && (
            <div className="absolute inset-0 z-50 bg-primary/10 backdrop-blur-sm rounded-lg flex items-center justify-center pointer-events-none">
              <div className="bg-background/95 border-2 border-primary border-dashed rounded-lg p-8 shadow-xl">
                <div className="flex flex-col items-center gap-4">
                  <div className="relative">
                    <Upload className="h-16 w-16 text-primary animate-bounce" />
                    <div className="absolute inset-0 h-16 w-16 text-primary opacity-30 animate-ping">
                      <Upload className="h-16 w-16" />
                    </div>
                  </div>
                  <div className="text-center space-y-2">
                    <p className="text-lg font-semibold text-primary">{t('buckets.upload.drop_here')}</p>
                    <p className="text-sm text-muted-foreground">{t('buckets.upload.destination', { location: currentPath || t('buckets.common.root') })}</p>
                  </div>
                </div>
              </div>
            </div>
          )}

          <ObjectsTable
            bucketName={bucketName}
            objects={objects}
            currentPath={currentPath}
            searchQuery={searchQuery}
            filterQuery={filterQuery}
            deepSearch={deepSearch}
            selectedFileKeys={selectedFileKeys}
            selectedFolderKeys={selectedFolderKeys}
            isDragActive={isDragActive}
            error={error}
            isLoading={isLoading && !isRefreshing && !isNavigating}
            isTruncated={isTruncated}
            nextContinuationToken={nextContinuationToken}
            itemsPerPage={itemsPerPage}
            onNavigateToFolder={onNavigateToFolder}
            onDeleteObject={onDeleteObject ? (obj) => {
              setSelectedObject(obj);
              setDeleteObjectDialogOpen(true);
            } : undefined}
            onDeleteFolder={onDeleteMultipleObjects ? (obj) => handleDeleteFolder(obj.key) : undefined}
            onToggleFileSelection={handleToggleFileSelection}
            onToggleFolderSelection={handleToggleFolderSelection}
            onSelectAll={handleSelectAll}
            onPageChange={onPageChange}
            onItemsPerPageChange={onItemsPerPageChange}
            initialPageToken={initialPageToken}
            initialItemsPerPage={initialItemsPerPage}
          />
        </div>
      </div>

      {/* Create Directory Dialog */}
      {onCreateDirectory && (
        <CreateDirectoryDialog
          open={createDirDialogOpen}
          onOpenChange={setCreateDirDialogOpen}
          currentPath={currentPath}
          onCreateDirectory={onCreateDirectory}
        />
      )}

      {/* Delete Object Dialog */}
      <DeleteObjectDialog
        open={deleteObjectDialogOpen}
        onOpenChange={setDeleteObjectDialogOpen}
        object={selectedObject}
        onDeleteObject={handleDeleteObject}
      />

      {/* Bulk / Folder Delete Confirmation */}
      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => {
          if (!open && !bulkDeleting) setPendingDelete(null);
        }}
        title={getBulkDeleteTitle(pendingDelete, t, language)}
        description={getBulkDeleteDescription(pendingDelete, t, language)}
        confirmLabel={t('buckets.actions.delete')}
        loading={bulkDeleting}
        onConfirm={handleConfirmBulkDelete}
      />
    </div>
  );
}

// Builds a concise title summarising what the bulk-delete dialog will remove.
type Translate = (key: string, values?: Record<string, string | number>) => string;

function getBulkDeleteTitle(pending: { keys: string[]; prefixes: string[] } | null, t: Translate, language: string): string {
  if (!pending) return t('buckets.bulk_delete.title.items');
  const { keys, prefixes } = pending;
  const total = keys.length + prefixes.length;
  if (keys.length === 0 && prefixes.length === 1) {
    return t('buckets.bulk_delete.title.folder');
  }
  return t('buckets.bulk_delete.title.count', { count: total.toLocaleString(language) });
}

// Spells out the file/folder counts and warns that folders are removed recursively.
function getBulkDeleteDescription(
  pending: { keys: string[]; prefixes: string[] } | null,
  t: Translate,
  language: string,
): string {
  if (!pending) return '';
  const { keys, prefixes } = pending;
  if (prefixes.length > 0) {
    return t('buckets.bulk_delete.description.with_folders', {
      files: keys.length.toLocaleString(language),
      folders: prefixes.length.toLocaleString(language),
    });
  }
  return t('buckets.bulk_delete.description.files_only', { files: keys.length.toLocaleString(language) });
}
