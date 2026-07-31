/**
 * Color scale for sunburst segments based on file type / extension.
 */

import type { TreeNode } from '../types'

// Category colors — curated palette for dark theme
const categoryColors: Record<string, string> = {
  directory:  'hsl(220, 15%, 40%)',
  images:     'hsl(340, 70%, 55%)',
  video:      'hsl(280, 65%, 55%)',
  audio:      'hsl(200, 70%, 55%)',
  documents:  'hsl(45, 75%, 55%)',
  archives:   'hsl(25, 70%, 50%)',
  code:       'hsl(160, 60%, 45%)',
  system:     'hsl(0, 0%, 45%)',
  other:      'hsl(210, 20%, 50%)',
}

// Extension to category mapping
const extToCategory: Record<string, string> = {
  // Images
  '.jpg': 'images', '.jpeg': 'images', '.png': 'images', '.gif': 'images',
  '.bmp': 'images', '.svg': 'images', '.webp': 'images', '.ico': 'images',
  '.tiff': 'images', '.tif': 'images', '.heic': 'images', '.heif': 'images',
  '.raw': 'images', '.cr2': 'images', '.nef': 'images', '.psd': 'images',

  // Video
  '.mp4': 'video', '.mkv': 'video', '.avi': 'video', '.mov': 'video',
  '.wmv': 'video', '.flv': 'video', '.webm': 'video', '.m4v': 'video',
  '.mpg': 'video', '.mpeg': 'video', '.3gp': 'video',

  // Audio
  '.mp3': 'audio', '.wav': 'audio', '.flac': 'audio', '.aac': 'audio',
  '.ogg': 'audio', '.wma': 'audio', '.m4a': 'audio', '.aiff': 'audio',
  '.opus': 'audio', '.mid': 'audio',

  // Documents
  '.pdf': 'documents', '.doc': 'documents', '.docx': 'documents',
  '.xls': 'documents', '.xlsx': 'documents', '.ppt': 'documents',
  '.pptx': 'documents', '.txt': 'documents', '.rtf': 'documents',
  '.csv': 'documents', '.md': 'documents', '.pages': 'documents',
  '.numbers': 'documents', '.keynote': 'documents',

  // Archives
  '.zip': 'archives', '.rar': 'archives', '.7z': 'archives',
  '.tar': 'archives', '.gz': 'archives', '.bz2': 'archives',
  '.xz': 'archives', '.dmg': 'archives', '.iso': 'archives',
  '.pkg': 'archives',

  // Code
  '.js': 'code', '.ts': 'code', '.jsx': 'code', '.tsx': 'code',
  '.py': 'code', '.go': 'code', '.rs': 'code', '.java': 'code',
  '.c': 'code', '.cpp': 'code', '.h': 'code', '.hpp': 'code',
  '.cs': 'code', '.rb': 'code', '.php': 'code', '.swift': 'code',
  '.kt': 'code', '.scala': 'code', '.r': 'code', '.sql': 'code',
  '.html': 'code', '.css': 'code', '.scss': 'code', '.less': 'code',
  '.json': 'code', '.xml': 'code', '.yaml': 'code', '.yml': 'code',
  '.toml': 'code', '.sh': 'code', '.bash': 'code', '.zsh': 'code',
  '.svelte': 'code', '.vue': 'code',

  // System
  '.dll': 'system', '.so': 'system', '.dylib': 'system',
  '.sys': 'system', '.plist': 'system', '.log': 'system',
  '.tmp': 'system', '.cache': 'system', '.lock': 'system',
  '.app': 'system', '.exe': 'system', '.bin': 'system',
}

/**
 * Get the color for a TreeNode based on its type/extension
 */
export function getNodeColor(node: TreeNode, depth: number = 0): string {
  if (node.isDir) {
    // Vary directory hue slightly by depth for visual distinction
    const hue = 220 + (depth * 8) % 40
    const lightness = 35 + (depth * 3) % 15
    return `hsl(${hue}, 15%, ${lightness}%)`
  }

  const ext = getExtension(node.name)
  const category = extToCategory[ext] || 'other'
  const baseColor = categoryColors[category]

  // Slight variation by depth to distinguish segments
  return adjustForDepth(baseColor, depth)
}

/**
 * Get the category name for a node
 */
export function getCategory(node: TreeNode): string {
  if (node.isDir) return 'Folder'
  const ext = getExtension(node.name)
  const cat = extToCategory[ext] || 'other'
  return cat.charAt(0).toUpperCase() + cat.slice(1)
}

/**
 * Get a category color (for legend/labels)
 */
export function getCategoryColor(category: string): string {
  return categoryColors[category.toLowerCase()] || categoryColors.other
}

function getExtension(name: string): string {
  const lastDot = name.lastIndexOf('.')
  if (lastDot === -1 || lastDot === 0) return ''
  return name.substring(lastDot).toLowerCase()
}

function adjustForDepth(hslColor: string, depth: number): string {
  // Parse hsl and adjust lightness
  const match = hslColor.match(/hsl\((\d+),\s*(\d+)%,\s*(\d+)%\)/)
  if (!match) return hslColor

  const h = parseInt(match[1])
  const s = parseInt(match[2])
  const l = Math.min(70, parseInt(match[3]) + depth * 4)

  return `hsl(${h}, ${s}%, ${l}%)`
}

export const categories = Object.keys(categoryColors)
