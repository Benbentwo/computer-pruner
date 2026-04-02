/**
 * TreeNode represents a file or directory in the scan tree
 */
export interface TreeNode {
  name: string
  path: string
  size: number
  isDir: boolean
  isProtected: boolean
  modTime?: string
  children?: TreeNode[]
  fileCount?: number
  dirCount?: number
}

/**
 * VolumeInfo represents a mounted volume/disk
 */
export interface VolumeInfo {
  name: string
  mountPoint: string
  fsType: string
  totalBytes: number
  freeBytes: number
  usedBytes: number
}

/**
 * ScanProgress represents real-time scanning progress
 */
export interface ScanProgress {
  scannedItems: number
  totalSize: number
  currentPath: string
  elapsedMs: number
  isComplete: boolean
}

/**
 * DeleteResult represents the outcome of a deletion operation
 */
export interface DeleteResult {
  deletedCount: number
  freedBytes: number
  errors: string[]
}

/**
 * AppPreferences represents user settings
 */
export interface AppPreferences {
  theme: 'light' | 'dark'
  defaultDeleteBehavior: 'trash' | 'permanent'
  scanDepthLimit?: number
  exclusionPaths: string[]
}

/**
 * CollectorItem represents an item selected for deletion
 */
export interface CollectorItem {
  path: string
  name: string
  size: number
  isDir: boolean
}
