import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { api, ApiError, messageFor, type Material, type MaterialDetail, type Profile, type RetrievalHit } from '../api'

export default function MaterialsPage({ profile, onAuthLost }: { profile: Profile, onAuthLost: () => void }) {
  const [items, setItems] = useState<Material[]>([])
  const [selected, setSelected] = useState<MaterialDetail | null>(null)
  const [search, setSearch] = useState('')
  const [appliedSearch, setAppliedSearch] = useState('')
  const [retrievalQuery, setRetrievalQuery] = useState('')
  const [retrievalHits, setRetrievalHits] = useState<RetrievalHit[]>([])
  const [retrievalDone, setRetrievalDone] = useState(false)
  const [retrievalBusy, setRetrievalBusy] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const detailRef = useRef<HTMLElement | null>(null)

  const handleError = useCallback((cause: unknown) => {
    if (cause instanceof ApiError && cause.status === 401) {
      onAuthLost()
      return
    }
    setError(messageFor(cause))
  }, [onAuthLost])

  const refresh = useCallback(async (query: string) => {
    try {
      const result = await api<{ items: Material[], total: number }>(`/api/materials?search=${encodeURIComponent(query)}`)
      setItems(result.items)
      setError('')
    } catch (cause) {
      handleError(cause)
    }
  }, [handleError])

  useEffect(() => { void refresh(appliedSearch) }, [appliedSearch, refresh])

  async function openMaterial(id: number, scrollToDetail = false) {
    try {
      setSelected(await api<MaterialDetail>(`/api/materials/${id}`))
      setError('')
      if (scrollToDetail) detailRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    } catch (cause) {
      handleError(cause)
    }
  }

  async function retrieve(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const query = retrievalQuery.trim()
    if (!query) {
      setRetrievalDone(false)
      setError('请输入要检索的词语。')
      return
    }
    setRetrievalBusy(true)
    setError('')
    try {
      const result = await api<{ hits: RetrievalHit[] }>(`/api/retrieval?q=${encodeURIComponent(query)}`)
      setRetrievalHits(result.hits)
      setRetrievalDone(true)
    } catch (cause) {
      setRetrievalDone(false)
      handleError(cause)
    } finally {
      setRetrievalBusy(false)
    }
  }

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!file) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const body = new FormData()
      body.set('file', file)
      const result = await api<{ id: number }>('/api/materials', {
        method: 'POST',
        headers: { 'X-CSRF-Token': profile.csrf_token },
        body,
      })
      setFile(null)
      const input = document.getElementById('material-file') as HTMLInputElement | null
      if (input) input.value = ''
      setNotice(`上传成功，材料 #${result.id} 已加入本班知识库。`)
      await refresh(appliedSearch)
      await openMaterial(result.id)
    } catch (cause) {
      handleError(cause)
    } finally {
      setBusy(false)
    }
  }

  async function logout() {
    try {
      await api<{ status: string }>('/api/logout', { method: 'POST', headers: { 'X-CSRF-Token': profile.csrf_token } })
      onAuthLost()
    } catch (cause) {
      handleError(cause)
    }
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand"><span className="brand-icon">CC</span><span>CampusClaw</span></div>
        <div className="session-info"><span className="class-pill">{profile.class_name} 班</span><span>{profile.username} · {profile.role === 'teacher' ? '教师' : '学生'}</span><button className="text-button" onClick={() => void logout()}>退出登录</button></div>
      </header>
      <main className="workspace">
        <div className="page-heading"><div><span className="eyebrow">本班资料</span><h1>教学材料</h1><p>这里仅展示 {profile.class_name} 班的文件和知识库正文。</p></div><span className="count-badge">{items.length} 份材料</span></div>
        {error && <p className="alert" role="alert">{error}</p>}
        {notice && <p className="success" role="status">{notice}</p>}
        <section className="panel retrieval-panel" aria-labelledby="retrieval-title">
          <div className="panel-heading"><div><h2 id="retrieval-title">本班知识库检索</h2><p className="muted">输入原文中的词语，查找本班材料正文；每条结果都能追溯到原文件。</p></div></div>
          <form className="retrieval-form" onSubmit={(event) => void retrieve(event)}>
            <input aria-label="检索本班材料正文" placeholder="例如：认证" value={retrievalQuery} onChange={(event) => setRetrievalQuery(event.target.value)} maxLength={100} required />
            <button type="submit" disabled={retrievalBusy}>{retrievalBusy ? '检索中…' : '检索'}</button>
          </form>
          {retrievalDone && (retrievalHits.length === 0 ? <p className="retrieval-empty" role="status">本班资料中未找到相关内容。</p> : <ul className="retrieval-results">{retrievalHits.map((hit, index) => <li key={`${hit.material_id}-${hit.start_offset}-${index}`}><button type="button" onClick={() => void openMaterial(hit.material_id, true)}><strong>{hit.title}</strong><span>第 {hit.chunk_index} 段 · 字符 {hit.start_offset + 1}–{hit.end_offset} · 查看原文 →</span><small>{hit.snippet}</small></button></li>)}</ul>)}
        </section>
        <div className="content-grid">
          <section className="panel" aria-labelledby="materials-title">
            <div className="panel-heading"><h2 id="materials-title">材料列表</h2><form className="search" onSubmit={(event) => { event.preventDefault(); setAppliedSearch(search) }}><input aria-label="按文件名筛选" placeholder="按文件名筛选" value={search} onChange={(event) => setSearch(event.target.value)} /><button type="submit">筛选</button></form></div>
            {items.length === 0 ? <div className="empty-state">本班暂无符合条件的材料。</div> : <ul className="material-list">{items.map((item) => <li key={item.id}><button className={selected?.id === item.id ? 'material-row active' : 'material-row'} onClick={() => void openMaterial(item.id)}><span className="file-icon">{item.original_filename.toLowerCase().endsWith('.md') ? 'MD' : 'TXT'}</span><span className="file-info"><strong>{item.original_filename}</strong><small>{item.uploader} · {new Date(item.created_at).toLocaleString('zh-CN')}</small></span><span className="arrow">→</span></button></li>)}</ul>}
          </section>
          <div className="side-column">
            {profile.role === 'teacher' && <section className="panel upload-panel" aria-labelledby="upload-title"><h2 id="upload-title">上传材料</h2><p className="muted">支持 UTF-8 .txt / .md；上传后本班学生即可查看。</p><form onSubmit={(event) => void upload(event)}><label className="file-picker" htmlFor="material-file">{file ? file.name : '选择 TXT 或 Markdown 文件'}</label><input id="material-file" type="file" accept=".txt,.md" onChange={(event) => setFile(event.target.files?.[0] ?? null)} required /><button className="primary-button" type="submit" disabled={busy || !file}>{busy ? '正在上传…' : '上传到本班'}</button></form></section>}
            <section ref={detailRef} className="panel detail-panel" aria-labelledby="detail-title"><div className="panel-heading"><h2 id="detail-title">材料详情</h2>{selected && <a className="download-link" href={`/api/materials/${selected.id}/file`}>下载原文件 ↗</a>}</div>{selected ? <><h3>{selected.original_filename}</h3><p className="detail-meta">{selected.uploader} · {new Date(selected.created_at).toLocaleString('zh-CN')} · {(selected.size_bytes / 1024).toFixed(1)} KB</p><div className="document-body">{selected.original_filename.toLowerCase().endsWith('.md') ? <ReactMarkdown skipHtml>{selected.body}</ReactMarkdown> : <pre>{selected.body}</pre>}</div></> : <div className="empty-state">选择左侧材料后查看正文。</div>}</section>
          </div>
        </div>
      </main>
    </div>
  )
}
