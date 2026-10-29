const STORAGE_KEY = 'k-watch.clusterBookmarks'

export interface ClusterBookmark {
  name: string
  url: string
}

export function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

function isBookmark(value: unknown): value is ClusterBookmark {
  if (!value || typeof value !== 'object') return false
  const bookmark = value as ClusterBookmark
  return typeof bookmark.name === 'string' && bookmark.name.trim() !== '' && isHttpUrl(bookmark.url)
}

export function loadBookmarks(): ClusterBookmark[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isBookmark)
  } catch {
    return []
  }
}

export function saveBookmarks(bookmarks: ClusterBookmark[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(bookmarks))
}
