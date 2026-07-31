import { writable } from 'svelte/store'
import type { VolumeInfo, TreeNode, ScanProgress, CollectorItem } from '../types'

/**
 * List of mounted volumes
 */
export const volumes = writable<VolumeInfo[]>([])

/**
 * Root of the current scan tree
 */
export const scanTree = writable<TreeNode | null>(null)

/**
 * Real-time scanning progress
 */
export const scanProgress = writable<ScanProgress | null>(null)

/**
 * Current root node for sunburst chart (changes when drilling down)
 */
export const currentRoot = writable<TreeNode | null>(null)

/**
 * Breadcrumb trail for navigation
 */
export const breadcrumbs = writable<TreeNode[]>([])

/**
 * Currently hovered node in the sunburst chart
 */
export const hoveredNode = writable<TreeNode | null>(null)

/**
 * Currently selected node (clicked)
 */
export const selectedNode = writable<TreeNode | null>(null)

/**
 * Items collected for deletion
 */
export const collectorItems = writable<CollectorItem[]>([])

/**
 * Current theme
 */
export const theme = writable<'light' | 'dark'>('dark')

/**
 * Whether a scan is currently in progress
 */
export const isScanning = writable<boolean>(false)

/**
 * Toast notifications queue
 */
export interface Notification {
  id: string
  type: 'success' | 'error' | 'info'
  message: string
}

export const notifications = writable<Notification[]>([])

/**
 * Last scan error
 */
export const scanError = writable<string | null>(null)
