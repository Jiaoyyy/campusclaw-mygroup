import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, ApiError, messageFor, streamMessage, type Material, type Profile, type RetrievalHit } from '../api'

type Skill = { id: number, name: string, enabled: boolean, source_text: string }
type Agent = { id: number, name: string, prompt: string, enabled: boolean, skill_ids: number[], material_ids: number[] }
type ChatMessage = { id: number, role: 'user' | 'assistant', content: string, citations: RetrievalHit[], skill_name?: string }
type Conversation = { id: number, agent_id: number, title: string, messages?: ChatMessage[] }
const sampleSkill = '名称：数据结构解题引导\n适用条件：二叉搜索树,查找,插入,节点\n资料：数据结构\n工具：knowledge_search\n启用：是\n\n先判断学生已有的思路，只指出下一步应检查的节点或比较关系，不直接给最终答案。'

export default function TutorPage({ profile, onAuthLost, onNavigate }: { profile: Profile, onAuthLost: () => void, onNavigate: () => void }) {
  const [agents, setAgents] = useState<Agent[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [materials, setMaterials] = useState<Material[]>([])
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [selected, setSelected] = useState<Conversation | null>(null)
  const [agentChoice, setAgentChoice] = useState('')
  const [question, setQuestion] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [partial, setPartial] = useState('')
  const [liveCitations, setLiveCitations] = useState<RetrievalHit[]>([])
  const [liveSkill, setLiveSkill] = useState('')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [skillText, setSkillText] = useState(sampleSkill)
  const [editingSkill, setEditingSkill] = useState<number | null>(null)
  const [agentForm, setAgentForm] = useState<Agent>({ id: 0, name: '', prompt: '', enabled: true, skill_ids: [], material_ids: [] })

  const handleError = useCallback((cause: unknown) => {
    if (cause instanceof ApiError && cause.status === 401) { onAuthLost(); return }
    setError(messageFor(cause) + (cause instanceof ApiError ? `（${cause.message}）` : ''))
  }, [onAuthLost])

  const refresh = useCallback(async () => {
    try {
      const [a, s, c, m] = await Promise.all([
        api<{ agents: Agent[] }>('/api/agents'), api<{ skills: Skill[] }>('/api/skills'),
        api<{ conversations: Conversation[] }>('/api/conversations'), api<{ items: Material[] }>('/api/materials'),
      ])
      setAgents(a.agents); setSkills(s.skills); setConversations(c.conversations); setMaterials(m.items)
      setAgentChoice((old) => old || String(a.agents.find((item) => item.enabled)?.id ?? ''))
    } catch (cause) { handleError(cause) }
  }, [handleError])
  useEffect(() => { void refresh() }, [refresh])

  async function openConversation(id: number) {
    try { setSelected(await api<Conversation>(`/api/conversations/${id}`)); setPartial(''); setLiveCitations([]) }
    catch (cause) { handleError(cause) }
  }
  async function createConversation() {
    if (!agentChoice) { setError('请先选择一个智能体。'); return }
    try {
      const result = await api<Conversation>('/api/conversations', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify({ agent_id: Number(agentChoice) }),
      })
      setSelected({ ...result, messages: [] }); setError(''); await refresh()
    } catch (cause) { handleError(cause) }
  }
  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selected || !question.trim() || streaming) return
    const text = question.trim(); setQuestion(''); setStreaming(true); setPartial(''); setLiveCitations([]); setLiveSkill(''); setError('')
    setSelected({ ...selected, messages: [...(selected.messages ?? []), { id: -Date.now(), role: 'user', content: text, citations: [] }] })
    try {
      await streamMessage(selected.id, text, profile.csrf_token, (eventName, data) => {
        if (eventName === 'meta') { setLiveCitations((data.citations as RetrievalHit[]) ?? []); setLiveSkill((data.skill as { name?: string })?.name ?? '') }
        if (eventName === 'delta') setPartial((previous) => previous + String(data.text ?? ''))
        if (eventName === 'error') setError(String(data.error ?? '回答失败'))
      })
      await openConversation(selected.id); await refresh()
    } catch (cause) { handleError(cause); await openConversation(selected.id) }
    finally { setStreaming(false) }
  }
  async function saveSkill() {
    try {
      await api(`/api/skills${editingSkill ? `/${editingSkill}` : ''}`, {
        method: editingSkill ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify({ source_text: skillText }),
      })
      setNotice('技能已保存。'); setError(''); setEditingSkill(null); await refresh()
    } catch (cause) { handleError(cause) }
  }
  async function saveAgent() {
    try {
      await api(`/api/agents${agentForm.id ? `/${agentForm.id}` : ''}`, {
        method: agentForm.id ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify(agentForm),
      })
      setNotice('智能体已保存，绑定对下一次提问立即生效。'); setError(''); await refresh()
    } catch (cause) { handleError(cause) }
  }
  function toggle(key: 'skill_ids' | 'material_ids', id: number) {
    setAgentForm((old) => ({ ...old, [key]: old[key].includes(id) ? old[key].filter((value) => value !== id) : [...old[key], id] }))
  }

  return <div className="app-shell">
    <header className="topbar"><div className="brand"><span className="brand-icon">CC</span><span>CampusClaw</span></div><div className="session-info"><button className="text-button" onClick={onNavigate}>教学材料</button><span className="class-pill">{profile.class_name} 班</span><span>{profile.username}</span></div></header>
    <main className="workspace tutor-page">
      <div className="page-heading"><div><span className="eyebrow">第 5 课 · 流式会话</span><h1>解题助手</h1><p>先选本班智能体，再提问；回答会逐段显示，引用可追溯到材料。</p></div></div>
      {error && <p className="alert" role="alert">{error}</p>}{notice && <p className="success" role="status">{notice}</p>}
      <div className="tutor-grid">
        <section className="panel"><h2>我的会话</h2><div className="tutor-create"><select aria-label="选择智能体" value={agentChoice} onChange={(e) => setAgentChoice(e.target.value)}><option value="">选择智能体</option>{agents.filter((a) => a.enabled).map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}</select><button onClick={() => void createConversation()}>新建会话</button></div><div className="tutor-conversation-list">{conversations.map((c) => <button key={c.id} className={selected?.id === c.id ? 'active' : ''} onClick={() => void openConversation(c.id)}>{c.title}</button>)}</div></section>
        <section className="panel tutor-chat"><h2>{selected ? selected.title : '选择或新建会话'}</h2><div className="tutor-messages">{selected?.messages?.map((m) => <article key={m.id} className={`tutor-message ${m.role}`}><strong>{m.role === 'user' ? '我' : '助手'}</strong><p>{m.content}</p>{m.skill_name && <small>技能：{m.skill_name}</small>}{m.citations?.length > 0 && <div className="tutor-sources">{m.citations.map((hit) => <button key={hit.chunk_id} onClick={onNavigate}>[{hit.citation_number}] {hit.title} · 切片 {hit.chunk_index}</button>)}</div>}</article>)}{streaming && <article className="tutor-message assistant"><strong>助手正在回答…</strong><p aria-live="polite">{partial || '正在检索本班材料…'}</p>{liveSkill && <small>技能：{liveSkill}</small>}{liveCitations.length > 0 && <div className="tutor-sources">{liveCitations.map((hit) => <button key={hit.chunk_id} onClick={onNavigate}>[{hit.citation_number}] {hit.title} · 切片 {hit.chunk_index}</button>)}</div>}</article>}</div><form className="tutor-send" onSubmit={(e) => void send(e)}><input aria-label="输入问题" placeholder="例如：这道方程去括号，我下一步该检查什么？" minLength={2} maxLength={500} value={question} onChange={(e) => setQuestion(e.target.value)} disabled={!selected || streaming} /><button disabled={!selected || streaming}>{streaming ? '回答中…' : '发送'}</button></form></section>
      </div>
      {profile.role === 'teacher' && <div className="tutor-config">
        <section className="panel"><h2>技能文件</h2><p className="muted">空行前填写名称、适用条件、资料、工具、启用；空行后写引导说明。解题引导技能只提示下一步。</p><select aria-label="编辑技能" value={editingSkill ?? ''} onChange={(e) => { const x = skills.find((item) => item.id === Number(e.target.value)); setEditingSkill(x?.id ?? null); setSkillText(x?.source_text ?? sampleSkill) }}><option value="">新建技能</option>{skills.map((x) => <option key={x.id} value={x.id}>{x.name}{x.enabled ? '' : '（停用）'}</option>)}</select><textarea aria-label="技能文件正文" rows={10} value={skillText} onChange={(e) => setSkillText(e.target.value)} /><button onClick={() => void saveSkill()}>保存技能</button></section>
        <section className="panel"><h2>智能体与绑定</h2><p className="muted">提示词决定助手角色；只有勾选的技能与材料会参与该智能体的回答。</p><select aria-label="编辑智能体" value={agentForm.id || ''} onChange={(e) => { const x = agents.find((item) => item.id === Number(e.target.value)); setAgentForm(x ? { ...x } : { id: 0, name: '', prompt: '', enabled: true, skill_ids: [], material_ids: [] }) }}><option value="">新建智能体</option>{agents.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}</select><input aria-label="智能体名称" placeholder="例如：数学解题助手" maxLength={64} value={agentForm.name} onChange={(e) => setAgentForm({ ...agentForm, name: e.target.value })} /><textarea aria-label="角色提示词" rows={4} maxLength={2000} placeholder="你是本班的解题助手……" value={agentForm.prompt} onChange={(e) => setAgentForm({ ...agentForm, prompt: e.target.value })} /><label><input type="checkbox" checked={agentForm.enabled} onChange={(e) => setAgentForm({ ...agentForm, enabled: e.target.checked })} /> 启用智能体</label><strong>绑定技能</strong><div className="tutor-checks">{skills.map((x) => <label key={x.id}><input type="checkbox" checked={agentForm.skill_ids.includes(x.id)} onChange={() => toggle('skill_ids', x.id)} /> {x.name}</label>)}</div><strong>绑定材料</strong><div className="tutor-checks">{materials.map((x) => <label key={x.id}><input type="checkbox" checked={agentForm.material_ids.includes(x.id)} onChange={() => toggle('material_ids', x.id)} /> {x.original_filename}</label>)}</div><button onClick={() => void saveAgent()}>保存智能体</button></section>
      </div>}
    </main>
  </div>
}
