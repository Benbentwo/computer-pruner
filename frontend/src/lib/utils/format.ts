/**
 * Format bytes to human-readable format (B, KB, MB, GB, TB)
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

/**
 * Format a file path, truncating long paths with ellipsis in the middle
 */
export function formatPath(path: string, maxLen: number = 60): string {
  if (path.length <= maxLen) {
    return path
  }

  const start = Math.floor((maxLen - 3) / 2)
  const end = path.length - (maxLen - 3 - start)

  return path.substring(0, start) + '...' + path.substring(end)
}

/**
 * Format milliseconds to a readable duration string (Xs or Xm Ys)
 */
export function formatDuration(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60

  if (minutes === 0) {
    return `${seconds}s`
  }

  return `${minutes}m ${seconds}s`
}
