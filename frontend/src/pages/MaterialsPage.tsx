import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { api, ApiError, downloadMaterial, messageFor, type Material, type MaterialDetail, type Profile, type RetrievalHit } from '../api'

export default function MaterialsPage({ profile, onAuthLost }: { profile: Profile, onAuthLost: () => void }) {
  const [items, setItems] = useState<Material[]>([])
  const [selected, setSelected] = useState<MaterialDetail | null>(null)
  const [search, setSearch] = useState('')
  const [appliedSearch, setAppliedSearch] = useState('')
  const [retrievalQuery, setRetrievalQuery] = useState('')
  const [retrievalMode, setRetrievalMode] = useState<'keyword' | 'vector' | 'hybrid'>('keyword')
  const [vectorEnabled, setVectorEnabled] = useState(false)
  const [answerEnabled, setAnswerEnabled] = useState(false)
  const [retrievalHits, setRetrievalHits] = useState<RetrievalHit[]>([])
  const [retrievalDone, setRetrievalDone] = useState(false)
  const [retrievalBusy, setRetrievalBusy] = useState(false)
  const [answer, setAnswer] = useState('')
  const [citations, setCitations] = useState<RetrievalHit[]>([])
  const [asking, setAsking] = useState(false)
  const [strategy, setStrategy] = useState<'auto' | 'custom' | 'hierarchy'>('auto')
  const [maxChars, setMaxChars] = useState(800)
  const [overlapPercent, setOverlapPercent] = useState(10)
  const [separator, setSeparator] = useState<'newline' | 'blankline' | 'period'>('newline')
  const [removeURLs, setRemoveURLs] = useState(false)
  const [removeEmails, setRemoveEmails] = useState(false)
  const [collapseWhitespace, setCollapseWhitespace] = useState(false)
  const [reindexBusy, setReindexBusy] = useState(false)
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
  useEffect(() => {
    void api<{ vector_enabled: boolean, answer_enabled: boolean, default_mode: 'keyword' | 'hybrid' }>('/api/retrieval/capabilities')
      .then((result) => {
        setVectorEnabled(result.vector_enabled)
        setAnswerEnabled(result.answer_enabled)
        setRetrievalMode(result.default_mode)
      })
      .catch(handleError)
  }, [handleError])

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
      const result = await api<{ hits: RetrievalHit[] }>(`/api/retrieval?q=${encodeURIComponent(query)}&mode=${retrievalMode}`)
      setRetrievalHits(result.hits)
      setRetrievalDone(true)
      setAnswer('')
      setCitations([])
    } catch (cause) {
      setRetrievalDone(false)
      handleError(cause)
    } finally {
      setRetrievalBusy(false)
    }
  }

  async function ask() {
    const question = retrievalQuery.trim()
    if (!question) { setError('请先输入问题。'); return }
    setAsking(true)
    setError('')
    try {
      const result = await api<{ answer: string, citations: RetrievalHit[] }>('/api/ask', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token },
        body: JSON.stringify({ question }),
      })
      setAnswer(result.answer)
      setCitations(result.citations)
    } catch (cause) { handleError(cause) } finally { setAsking(false) }
  }

  function chunkOptions() {
    return { strategy, max_chars: maxChars, overlap_percent: overlapPercent, separator, remove_urls: removeURLs, remove_emails: removeEmails, collapse_whitespace: collapseWhitespace }
  }

  async function reindex() {
    if (!selected) return
    setReindexBusy(true)
    setError('')
    try {
      await api(`/api/materials/${selected.id}/reindex`, {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token },
        body: JSON.stringify(chunkOptions()),
      })
      setNotice(vectorEnabled ? '索引已按当前策略重建。' : '切片已重建，关键词检索可用。')
      await refresh(appliedSearch)
      await openMaterial(selected.id)
    } catch (cause) { handleError(cause) } finally { setReindexBusy(false) }
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
      for (const [key, value] of Object.entries(chunkOptions())) body.set(key, String(value))
      const result = await api<{ id: number, index_status: string }>('/api/materials', {
        method: 'POST',
        headers: { 'X-CSRF-Token': profile.csrf_token },
        body,
      })
      setFile(null)
      const input = document.getElementById('material-file') as HTMLInputElement | null
      if (input) input.value = ''
      setNotice(result.index_status === 'ready' ? `上传成功，材料 #${result.id} 已完成索引。` : !vectorEnabled ? `上传成功，材料 #${result.id} 可用关键词检索；向量索引待配置。` : `上传成功，材料 #${result.id} 已保存；向量索引暂未就绪。`)
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
          <div className="panel-heading"><div><h2 id="retrieval-title">本班知识库检索</h2><p className="muted">{vectorEnabled ? '选择检索模式，查找本班切片；每条结果都能追溯到原文件。' : '当前使用关键词检索；配置嵌入模型后可开启向量与混合检索。'}</p></div></div>
          <form className="retrieval-form" onSubmit={(event) => void retrieve(event)}>
            <input aria-label="检索本班材料正文" placeholder="例如：认证规则是什么？" value={retrievalQuery} onChange={(event) => setRetrievalQuery(event.target.value)} maxLength={100} required />
            <select aria-label="检索模式" value={retrievalMode} onChange={(event) => setRetrievalMode(event.target.value as typeof retrievalMode)}><option value="keyword">关键词{!vectorEnabled ? '（当前可用）' : ''}</option>{vectorEnabled && <><option value="hybrid">混合（默认）</option><option value="vector">向量</option></>}</select>
            <button type="submit" disabled={retrievalBusy}>{retrievalBusy ? '检索中…' : '检索'}</button>
            {answerEnabled && <button type="button" disabled={asking} onClick={() => void ask()}>{asking ? '回答中…' : '简短回答'}</button>}
          </form>
          {retrievalDone && (retrievalHits.length === 0 ? <p className="retrieval-empty" role="status">资料中未找到相关内容。</p> : <ul className="retrieval-results">{retrievalHits.map((hit) => <li key={hit.chunk_id}><button type="button" onClick={() => void openMaterial(hit.material_id, true)}><strong>{hit.title}</strong><span>第 {hit.chunk_index} 段 · 字符 {hit.start_offset + 1}–{hit.end_offset} · 查看原文 →</span><small>{hit.snippet}</small></button></li>)}</ul>)}
          {answer && <div className="answer-panel"><h3>依据本班材料的回答</h3><p>{answer}</p>{citations.length > 0 && <ol>{citations.map((hit) => <li key={hit.chunk_id}><button type="button" onClick={() => void openMaterial(hit.material_id, true)}>[{citations.indexOf(hit) + 1}] {hit.title} · 第 {hit.chunk_index} 段</button></li>)}</ol>}</div>}
        </section>
        <div className="content-grid">
          <section className="panel" aria-labelledby="materials-title">
            <div className="panel-heading"><h2 id="materials-title">材料列表</h2><form className="search" onSubmit={(event) => { event.preventDefault(); setAppliedSearch(search) }}><input aria-label="按文件名筛选" placeholder="按文件名筛选" value={search} onChange={(event) => setSearch(event.target.value)} /><button type="submit">筛选</button></form></div>
            {items.length === 0 ? <div className="empty-state">本班暂无符合条件的材料。</div> : <ul className="material-list">{items.map((item) => <li key={item.id}><button className={selected?.id === item.id ? 'material-row active' : 'material-row'} onClick={() => void openMaterial(item.id)}><span className="file-icon">{item.original_filename.toLowerCase().endsWith('.md') ? 'MD' : 'TXT'}</span><span className="file-info"><strong>{item.original_filename}</strong><small>{item.uploader} · {new Date(item.created_at).toLocaleString('zh-CN')} · {item.index_status === 'ready' ? '索引就绪' : !vectorEnabled ? '关键词可用' : item.index_status === 'failed' ? '向量索引失败' : '索引处理中'}</small></span><span className="arrow">→</span></button></li>)}</ul>}
          </section>
          <div className="side-column">
            {profile.role === 'teacher' && <section className="panel upload-panel" aria-labelledby="upload-title"><h2 id="upload-title">上传材料与切分策略</h2><p className="muted">支持 UTF-8 .txt / .md；索引就绪后本班师生可检索。</p><form onSubmit={(event) => void upload(event)}><label className="file-picker" htmlFor="material-file">{file ? file.name : '选择 TXT 或 Markdown 文件'}</label><input id="material-file" type="file" accept=".txt,.md" onChange={(event) => setFile(event.target.files?.[0] ?? null)} required /><label>切分策略 <select value={strategy} onChange={(event) => setStrategy(event.target.value as typeof strategy)}><option value="auto">自动窗口</option><option value="custom">自定义分隔符</option><option value="hierarchy">Markdown 标题</option></select></label>{strategy === 'custom' && <div className="chunk-settings"><label>最大字符数 <input type="number" min="100" max="2000" value={maxChars} onChange={(event) => setMaxChars(Number(event.target.value))} /></label><label>重叠比例（%） <input type="number" min="0" max="50" value={overlapPercent} onChange={(event) => setOverlapPercent(Number(event.target.value))} /></label><label>分隔符 <select value={separator} onChange={(event) => setSeparator(event.target.value as typeof separator)}><option value="newline">换行</option><option value="blankline">空行</option><option value="period">句号</option></select></label><label><input type="checkbox" checked={removeURLs} onChange={(event) => setRemoveURLs(event.target.checked)} /> 移除 URL</label><label><input type="checkbox" checked={removeEmails} onChange={(event) => setRemoveEmails(event.target.checked)} /> 移除邮箱</label><label><input type="checkbox" checked={collapseWhitespace} onChange={(event) => setCollapseWhitespace(event.target.checked)} /> 合并空白</label></div>}<button className="primary-button" type="submit" disabled={busy || !file}>{busy ? '正在上传…' : '上传到本班'}</button></form></section>}
            <section ref={detailRef} className="panel detail-panel" aria-labelledby="detail-title"><div className="panel-heading"><h2 id="detail-title">材料详情</h2>{selected && <button className="download-link" type="button" onClick={() => void downloadMaterial(selected.id, selected.original_filename).catch(handleError)}>下载原文件 ↗</button>}</div>{selected ? <><h3>{selected.original_filename}</h3><p className="detail-meta">{selected.uploader} · {new Date(selected.created_at).toLocaleString('zh-CN')} · {(selected.size_bytes / 1024).toFixed(1)} KB · {selected.index_status === 'ready' ? '索引就绪' : !vectorEnabled ? '关键词可用' : selected.index_status === 'failed' ? '向量索引失败' : '索引处理中'}</p>{profile.role === 'teacher' && <button className="reindex-button" type="button" disabled={reindexBusy} onClick={() => void reindex()}>{reindexBusy ? '重建中…' : '按当前策略重建索引'}</button>}<div className="document-body">{selected.original_filename.toLowerCase().endsWith('.md') ? <ReactMarkdown skipHtml>{selected.body}</ReactMarkdown> : <pre>{selected.body}</pre>}</div></> : <div className="empty-state">选择左侧材料后查看正文。</div>}</section>
          </div>
        </div>
      </main>
    </div>
  )
}
