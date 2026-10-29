import { useState } from 'react'
import { isHttpUrl, loadBookmarks, saveBookmarks, type ClusterBookmark } from '../services/clusters'

function ClusterSwitcher() {
  const [open, setOpen] = useState(false)
  const [bookmarks, setBookmarks] = useState<ClusterBookmark[]>(() => loadBookmarks())
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [error, setError] = useState('')

  const persist = (next: ClusterBookmark[]) => {
    setBookmarks(next)
    saveBookmarks(next)
  }

  const addBookmark = () => {
    const nextName = name.trim()
    const nextUrl = url.trim().replace(/\/$/, '')
    if (!nextName) {
      setError('Name is required.')
      return
    }
    if (!isHttpUrl(nextUrl)) {
      setError('Use an http or https address.')
      return
    }
    const withoutDuplicate = bookmarks.filter((bookmark) => bookmark.url !== nextUrl)
    persist([...withoutDuplicate, { name: nextName, url: nextUrl }])
    setName('')
    setUrl('')
    setError('')
  }

  return (
    <div className="cluster-switcher">
      <label htmlFor="cluster-bookmark">Cluster</label>
      <select
        id="cluster-bookmark"
        value=""
        onChange={(event) => {
          if (event.target.value) window.location.assign(event.target.value)
        }}
      >
        <option value="">This cluster</option>
        {bookmarks.map((bookmark) => (
          <option key={bookmark.url} value={bookmark.url}>{bookmark.name}</option>
        ))}
      </select>
      <button type="button" className="cluster-add" onClick={() => setOpen((value) => !value)}>
        {open ? 'Close' : 'Add'}
      </button>
      {open && (
        <div className="cluster-menu">
          <p>Address of K-Watch installed in another cluster. Choosing it opens that installation.</p>
          <input value={name} placeholder="Name" onChange={(event) => setName(event.target.value)} />
          <input value={url} placeholder="https://k-watch.example.com" onChange={(event) => setUrl(event.target.value)} />
          {error && <p className="map-error">{error}</p>}
          <button type="button" onClick={addBookmark}>Save</button>
          {bookmarks.length > 0 && (
            <ul>
              {bookmarks.map((bookmark) => (
                <li key={bookmark.url}>
                  <span>{bookmark.name}</span>
                  <button
                    type="button"
                    onClick={() => persist(bookmarks.filter((item) => item.url !== bookmark.url))}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}

export default ClusterSwitcher
