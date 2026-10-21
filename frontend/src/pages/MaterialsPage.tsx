import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import './tutor.css'
import './materials.css'
import { api, ApiError, downloadMaterial, messageFor, type Material, type MaterialDetail, type Profile, type RetrievalHit } from '../api'

export default function MaterialsPage({ profile, onAuthLost, onNavigate }: { profile: Profile, onAuthLost: () => void, onNavigate: (view?: 'chat' | 'settings') => void }) {
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

  useEffect(() => {
    const materialId = Number(new URLSearchParams(window.location.search).get('material'))
    if (Number.isSafeInteger(materialId) && materialId > 0) void openMaterial(materialId, true)
  // Read the deep link when this page mounts after clicking a citation.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

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
    <div className="assistant-app library-app">
      <header className="assistant-topbar">
        <div className="assistant-brand"><span className="assistant-brand-mark">C</span><span>CampusClaw <small>学习空间</small></span></div>
        <nav className="assistant-global-nav" aria-label="主导航">
          <button type="button" className="active" aria-current="page">教学材料</button>
          <button type="button" onClick={() => onNavigate('chat')}>智能助手</button>
          {profile.role === 'teacher' && <button type="button" onClick={() => onNavigate('settings')}>教师配置</button>}
        </nav>
        <div className="assistant-account"><span className="assistant-class">{profile.class_name} 班</span><span className="assistant-account-name">{profile.username} · {profile.role === 'teacher' ? '教师' : '学生'}</span><button type="button" className="assistant-logout" onClick={() => void logout()}>退出</button></div>
      </header>

      <main className="library-main">
        <div className="library-heading">
          <div><span className="assistant-overline">CLASS KNOWLEDGE LIBRARY</span><h1>教学材料</h1><p>搜索、阅读并追溯 {profile.class_name} 班的课程资料。</p></div>
          <div className="library-heading-meta"><span className="library-count"><strong>{items.length}</strong> 份材料</span><span className="library-access"><i /> 仅本班可见</span></div>
        </div>
        {error && <div className="library-flash error" role="alert">{error}<button type="button" onClick={() => setError('')} aria-label="关闭错误提示">×</button></div>}
        {notice && <div className="library-flash notice" role="status">{notice}<button type="button" onClick={() => setNotice('')} aria-label="关闭提示">×</button></div>}

        <section className="library-search-card" aria-labelledby="retrieval-title">
          <div className="library-search-intro"><div><span className="assistant-overline">SEARCH THE LIBRARY</span><h2 id="retrieval-title">本班知识库检索</h2><p>输入关键词或问题，定位相关片段，再打开原文核对。</p></div><span className="library-mode-indicator">{vectorEnabled ? '混合检索可用' : '关键词检索可用'}</span></div>
          <form className="library-retrieval-form" onSubmit={(event) => void retrieve(event)}>
            <div className="library-query-field"><span aria-hidden="true">⌕</span><input aria-label="检索本班材料正文" placeholder="例如：认证规则是什么？" value={retrievalQuery} onChange={(event) => setRetrievalQuery(event.target.value)} maxLength={100} required /></div>
            <select aria-label="检索模式" value={retrievalMode} onChange={(event) => setRetrievalMode(event.target.value as typeof retrievalMode)}><option value="keyword">关键词{!vectorEnabled ? '（当前可用）' : ''}</option>{vectorEnabled && <><option value="hybrid">混合检索</option><option value="vector">向量检索</option></>}</select>
            <button className="library-primary" type="submit" disabled={retrievalBusy}>{retrievalBusy ? '检索中…' : '检索材料 →'}</button>
            {answerEnabled && <button className="library-secondary" type="button" disabled={asking} onClick={() => void ask()}>{asking ? '生成中…' : '简短回答'}</button>}
          </form>
          <p className="library-search-note">{vectorEnabled ? '可切换关键词、向量和混合模式；结果均限定在本班资料。' : '当前为关键词检索；配置嵌入模型后可使用向量与混合检索。'}</p>
          {retrievalDone && <div className="library-result-section"><div className="library-result-heading"><h3>检索结果</h3><span>{retrievalHits.length} 条片段</span></div>{retrievalHits.length === 0 ? <p className="library-no-results" role="status">这次没有找到相关片段。试试更具体的关键词。</p> : <ul className="library-results">{retrievalHits.map((hit) => <li key={hit.chunk_id}><button type="button" onClick={() => void openMaterial(hit.material_id, true)}><span className="library-result-icon">{hit.title.toLowerCase().endsWith('.md') ? 'MD' : 'TXT'}</span><span className="library-result-copy"><strong>{hit.title}</strong><small>第 {hit.chunk_index} 段 · 字符 {hit.start_offset + 1}–{hit.end_offset}</small><span>{hit.snippet}</span></span><span className="library-result-arrow">↗</span></button></li>)}</ul>}</div>}
          {answer && <div className="library-answer"><span className="assistant-overline">QUICK ANSWER</span><h3>根据本班材料的简短回答</h3><p>{answer}</p>{citations.length > 0 && <div className="library-answer-sources"><strong>引用来源</strong>{citations.map((hit) => <button key={hit.chunk_id} type="button" onClick={() => void openMaterial(hit.material_id, true)}>[{hit.citation_number ?? citations.indexOf(hit) + 1}] {hit.title} · 第 {hit.chunk_index} 段 ↗</button>)}</div>}</div>}
        </section>

        <div className="library-content">
          <section className="library-list-card" aria-labelledby="materials-title">
            <div className="library-section-head"><div><span className="assistant-overline">YOUR CLASS FILES</span><h2 id="materials-title">材料列表</h2></div><span>{items.length} 份</span></div>
            {profile.role === 'teacher' && <details className="library-upload"><summary><span className="library-upload-plus">＋</span><span><strong>上传教学材料</strong><small>TXT / Markdown · 仅本班可见</small></span><span className="library-upload-chevron">⌄</span></summary><form onSubmit={(event) => void upload(event)}><label className="library-file-picker" htmlFor="material-file"><span>{file ? file.name : '点击选择 .txt 或 .md 文件'}</span><small>UTF-8 文本</small></label><input id="material-file" type="file" accept=".txt,.md" onChange={(event) => setFile(event.target.files?.[0] ?? null)} required /><label className="library-form-label" htmlFor="chunk-strategy">切分策略</label><select id="chunk-strategy" value={strategy} onChange={(event) => setStrategy(event.target.value as typeof strategy)}><option value="auto">自动窗口</option><option value="custom">自定义分隔符</option><option value="hierarchy">Markdown 标题</option></select>{strategy === 'custom' && <div className="library-chunk-settings"><label>最大字符数 <input type="number" min="100" max="2000" value={maxChars} onChange={(event) => setMaxChars(Number(event.target.value))} /></label><label>重叠比例（%） <input type="number" min="0" max="50" value={overlapPercent} onChange={(event) => setOverlapPercent(Number(event.target.value))} /></label><label>分隔符 <select value={separator} onChange={(event) => setSeparator(event.target.value as typeof separator)}><option value="newline">换行</option><option value="blankline">空行</option><option value="period">句号</option></select></label><label><input type="checkbox" checked={removeURLs} onChange={(event) => setRemoveURLs(event.target.checked)} /> 移除 URL</label><label><input type="checkbox" checked={removeEmails} onChange={(event) => setRemoveEmails(event.target.checked)} /> 移除邮箱</label><label><input type="checkbox" checked={collapseWhitespace} onChange={(event) => setCollapseWhitespace(event.target.checked)} /> 合并空白</label></div>}<button className="library-primary library-upload-submit" type="submit" disabled={busy || !file}>{busy ? '正在上传…' : '上传到本班'}</button></form></details>}
            <form className="library-filter" onSubmit={(event) => { event.preventDefault(); setAppliedSearch(search) }}><input aria-label="按文件名筛选" placeholder="按文件名筛选材料" value={search} onChange={(event) => setSearch(event.target.value)} /><button type="submit" aria-label="筛选材料">筛选</button>{appliedSearch && <button type="button" className="library-filter-clear" onClick={() => { setSearch(''); setAppliedSearch('') }}>清除</button>}</form>
            {items.length === 0 ? <div className="library-empty-list"><span>▤</span><strong>没有找到材料</strong><p>{appliedSearch ? '换个文件名试试，或清除筛选条件。' : '本班还没有上传教学材料。'}</p></div> : <ul className="library-file-list">{items.map((item) => <li key={item.id}><button type="button" className={selected?.id === item.id ? 'active' : ''} onClick={() => void openMaterial(item.id, true)}><span className="library-file-icon">{item.original_filename.toLowerCase().endsWith('.md') ? 'MD' : 'TXT'}</span><span className="library-file-copy"><strong>{item.original_filename}</strong><small>{item.uploader} · {new Date(item.created_at).toLocaleDateString('zh-CN')}</small></span><span className={`library-file-status ${item.index_status === 'ready' ? 'ready' : ''}`}>{item.index_status === 'ready' ? '索引就绪' : !vectorEnabled ? '关键词可用' : item.index_status === 'failed' ? '向量索引失败' : '索引处理中'}</span></button></li>)}</ul>}
          </section>

          <section ref={detailRef} className="library-detail-card" aria-labelledby="detail-title"><div className="library-section-head"><div><span className="assistant-overline">DOCUMENT VIEWER</span><h2 id="detail-title">材料正文</h2></div>{selected && <button className="library-download" type="button" onClick={() => void downloadMaterial(selected.id, selected.original_filename).catch(handleError)}>下载原文件 ↗</button>}</div>{selected ? <><div className="library-document-title"><span className="library-file-icon large">{selected.original_filename.toLowerCase().endsWith('.md') ? 'MD' : 'TXT'}</span><div><h3>{selected.original_filename}</h3><p>{selected.uploader} · {new Date(selected.created_at).toLocaleString('zh-CN')} · {(selected.size_bytes / 1024).toFixed(1)} KB</p></div></div><div className="library-document-toolbar"><span className={selected.index_status === 'ready' ? 'ready' : ''}>{selected.index_status === 'ready' ? '● 索引就绪' : !vectorEnabled ? '● 关键词可用' : selected.index_status === 'failed' ? '● 向量索引失败' : '● 索引处理中'}</span>{profile.role === 'teacher' && <button type="button" disabled={reindexBusy} onClick={() => void reindex()}>{reindexBusy ? '重建中…' : '按当前策略重建索引'}</button>}</div><div className="library-document-body">{selected.original_filename.toLowerCase().endsWith('.md') ? <ReactMarkdown skipHtml>{selected.body}</ReactMarkdown> : <pre>{selected.body}</pre>}</div></> : <div className="library-empty-detail"><span>▤</span><h3>选择一份材料开始阅读</h3><p>点击左侧文件，查看正文、索引状态和下载选项。</p></div>}</section>
        </div>
      </main>
    </div>
  )
}
