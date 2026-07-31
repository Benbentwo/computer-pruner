/**
 * Typed wrappers around Wails auto-generated bindings.
 *
 * These import directly from the wailsjs/ generated code, which is the proper
 * way to call Go backend methods from the frontend.
 */

// Import Wails-generated Go bindings
import { ListVolumes as _ListVolumes, GetVolumeInfo as _GetVolumeInfo } from '../../../wailsjs/go/volume/VolumeService'
import { StartScan as _StartScan, StartFolderScan as _StartFolderScan, CancelScan as _CancelScan, GetScanTree as _GetScanTree, GetScanProgress as _GetScanProgress } from '../../../wailsjs/go/scanner/ScannerService'
import { DeletePaths as _DeletePaths, DeletePathsPermanently as _DeletePathsPermanently, RevealInFileManager as _RevealInFileManager, GetFileInfo as _GetFileInfo, PreviewFile as _PreviewFile } from '../../../wailsjs/go/fileops/FileOpsService'
import { GetPreferences as _GetPreferences, SetPreferences as _SetPreferences } from '../../../wailsjs/go/preferences/PreferencesService'

// Import Wails runtime for events and dialogs
import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime'

import type { VolumeInfo, TreeNode, ScanProgress, AppPreferences, DeleteResult } from '../types'

// ─── Volume Service ───────────────────────────────────────────

export async function listVolumes(): Promise<VolumeInfo[]> {
  return _ListVolumes()
}

export async function getVolumeInfo(mountPoint: string): Promise<VolumeInfo> {
  return _GetVolumeInfo(mountPoint)
}

// ─── Scanner Service ──────────────────────────────────────────

export async function startScan(mountPoint: string, forceRescan: boolean = false): Promise<void> {
  return _StartScan(mountPoint, forceRescan)
}

export async function startFolderScan(path: string, forceRescan: boolean = false): Promise<void> {
  return _StartFolderScan(path, forceRescan)
}

export async function cancelScan(): Promise<void> {
  return _CancelScan()
}

export async function getScanTree(): Promise<TreeNode | null> {
  return _GetScanTree()
}

export async function getScanProgress(): Promise<ScanProgress> {
  return _GetScanProgress()
}

// ─── File Ops Service ─────────────────────────────────────────

export async function deletePaths(paths: string[]): Promise<DeleteResult> {
  return _DeletePaths(paths)
}

export async function deletePathsPermanently(paths: string[]): Promise<DeleteResult> {
  return _DeletePathsPermanently(paths)
}

export async function revealInFileManager(path: string): Promise<void> {
  return _RevealInFileManager(path)
}

export async function getFileInfo(path: string): Promise<any> {
  return _GetFileInfo(path)
}

export async function previewFile(path: string): Promise<void> {
  return _PreviewFile(path)
}

// ─── Preferences Service ─────────────────────────────────────

export async function getPreferences(): Promise<AppPreferences> {
  return _GetPreferences() as any
}

export async function setPreferences(prefs: AppPreferences): Promise<void> {
  return _SetPreferences(prefs as any)
}

// ─── App Methods (on main.App) ───────────────────────────────

import { OpenDirectoryDialog as _OpenDirectoryDialog, ListDirectory as _ListDirectory } from '../../../wailsjs/go/main/App'

export async function openDirectoryDialog(title: string = 'Select Folder'): Promise<string> {
  return _OpenDirectoryDialog(title)
}

export interface DirEntry {
  name: string
  path: string
  isDir: boolean
  size: number
}

export async function listDirectory(path: string): Promise<DirEntry[]> {
  return _ListDirectory(path)
}

// ─── Event Helpers ───────────────────────────────────────────

export function onEvent(eventName: string, callback: (...data: any[]) => void): () => void {
  return EventsOn(eventName, callback)
}

export function offEvent(eventName: string): void {
  EventsOff(eventName)
}
