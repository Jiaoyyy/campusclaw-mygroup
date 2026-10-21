import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { api, ApiError, messageFor, streamMessage, type Material, type Profile, type RetrievalHit } from '../api'
import './tutor.css'

type Skill = { id: number, name: string, enabled: boolean, source_text: string }
type Agent = { id: number, name: string, prompt: string, enabled: boolean, skill_ids: number[], material_ids: number[] }
type ChatMessage = { id: number, role: 'user' | 'assistant', content: string, citations: RetrievalHit[], skill_name?: string }
type Conversation = { id: number, agent_id: number, title: string, messages?: ChatMessage[] }
type SkillDraft = { name: string, triggers: string, subject: string, instructions: string, enabled: boolean }
type SettingsSection = 'agents' | 'skills' | 'tools'

const blankAgent: Agent = { id: 0, name: '', prompt: '', enabled: true, skill_ids: [], material_ids: [] }
const blankSkill: SkillDraft = { name: '', triggers: '', subject: '', instructions: '', enabled: true }
const suggestions = ['这道题我应该先检查什么？', '请根据本班材料给我一个解题提示', '我卡在第二步，可以提示思路吗？']

function parseSkill(source: string): SkillDraft {
  const [head, ...body] = source.replace(/\r\n/g, '\n').split('\n\n')
  const values: Record<string, string> = {}
  for (const line of head.split('\n')) {
    const match = line.match(/^([^：:]+)[：:](.*)$/)
    if (match) values[match[1].trim()] = match[2].trim()
  }
  return { name: values['名称'] ?? '', triggers: values['适用条件'] ?? '', subject: values['资料'] ?? '', enabled: values['启用'] !== '否', instructions: body.join('\n\n').trim() }
}

function serializeSkill(draft: SkillDraft): string {
  return [`名称：${draft.name.trim()}`, `适用条件：${draft.triggers.trim()}`, ...(draft.subject.trim() ? [`资料：${draft.subject.trim()}`] : []), '工具：knowledge_search', `启用：${draft.enabled ? '是' : '否'}`, '', draft.instructions.trim()].join('\n')
}

function SourceList({ hits, onNavigate }: { hits: RetrievalHit[], onNavigate: (materialId: number) => void }) {
  if (!hits.length) return null
  return <div className="assistant-sources"><span className="assistant-sources-title">参考材料</span><div className="assistant-source-items">{hits.map((hit) => <button type="button" key={hit.chunk_id} onClick={() => onNavigate(hit.material_id)} title="前往教学材料查看原文"><span>[{hit.citation_number}]</span> {hit.title} · 第 {hit.chunk_index} 段 <span aria-hidden="true">↗</span></button>)}</div></div>
}

export default function TutorPage({ profile, onAuthLost, onNavigate }: { profile: Profile, onAuthLost: () => void, onNavigate: (materialId?: number) => void }) {
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
  const [view, setView] = useState<'chat' | 'settings'>(window.location.search.includes('view=settings') && profile.role === 'teacher' ? 'settings' : 'chat')
  const [settingsSection, setSettingsSection] = useState<SettingsSection>('agents')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [skillDraft, setSkillDraft] = useState<SkillDraft>(blankSkill)
  const [editingSkill, setEditingSkill] = useState<number | null>(null)
  const [agentForm, setAgentForm] = useState<Agent>(blankAgent)
  const messagesEnd = useRef<HTMLDivElement>(null)
  const skillFile = useRef<HTMLInputElement>(null)
  const initializedForms = useRef(false)

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
      if (!initializedForms.current) {
        if (a.agents.length) setAgentForm({ ...a.agents[0] })
        if (s.skills.length) { setEditingSkill(s.skills[0].id); setSkillDraft(parseSkill(s.skills[0].source_text)) }
        initializedForms.current = true
      }
      setAgentChoice((old) => a.agents.some((agent) => agent.enabled && String(agent.id) === old) ? old : String(a.agents.find((agent) => agent.enabled)?.id ?? ''))
    } catch (cause) { handleError(cause) }
  }, [handleError])
  useEffect(() => { void refresh() }, [refresh])
  useEffect(() => { window.history.replaceState(null, '', view === 'settings' ? '/tutor?view=settings' : '/tutor') }, [view])
  useEffect(() => { messagesEnd.current?.scrollIntoView({ behavior: streaming ? 'smooth' : 'instant', block: 'end' }) }, [selected?.messages?.length, partial, streaming])

  async function openConversation(id: number) {
    try { setSelected(await api<Conversation>(`/api/conversations/${id}`)); setPartial(''); setLiveCitations([]); setView('chat') }
    catch (cause) { handleError(cause) }
  }
  async function createConversation() {
    if (!agentChoice) { setError('请先选择一个智能体。'); return }
    try {
      const result = await api<Conversation>('/api/conversations', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify({ agent_id: Number(agentChoice) }),
      })
      setSelected({ ...result, messages: [] }); setError(''); setView('chat'); await refresh()
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
  async function saveSkill(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await api(`/api/skills${editingSkill ? `/${editingSkill}` : ''}`, {
        method: editingSkill ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify({ source_text: serializeSkill(skillDraft) }),
      })
      setNotice('技能已保存。'); setError(''); setEditingSkill(null); setSkillDraft(blankSkill); await refresh()
    } catch (cause) { handleError(cause) }
  }
  async function saveAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await api(`/api/agents${agentForm.id ? `/${agentForm.id}` : ''}`, {
        method: agentForm.id ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify(agentForm),
      })
      setNotice('智能体已保存，绑定对下一次提问生效。'); setError(''); await refresh()
    } catch (cause) { handleError(cause) }
  }
  async function logout() {
    try {
      await api('/api/logout', { method: 'POST', headers: { 'X-CSRF-Token': profile.csrf_token } })
      onAuthLost()
    } catch (cause) { handleError(cause) }
  }
  function toggle(key: 'skill_ids' | 'material_ids', id: number) {
    setAgentForm((old) => ({ ...old, [key]: old[key].includes(id) ? old[key].filter((value) => value !== id) : [...old[key], id] }))
  }
  async function importSkill(file?: File) {
    if (!file) return
    try {
      const text = await file.text()
      const parsed = parseSkill(text)
      if (!parsed.name || !parsed.triggers || !parsed.instructions) throw new Error('技能文件缺少名称、适用条件或引导说明。')
      setSkillDraft(parsed); setEditingSkill(null); setError(''); setNotice(`已读取 ${file.name}，请确认后保存。`)
    } catch (cause) { setError(cause instanceof Error ? cause.message : '无法读取技能文件。') }
    if (skillFile.current) skillFile.current.value = ''
  }

  const activeAgent = agents.find((agent) => agent.id === selected?.agent_id)
  const selectedAgent = agents.find((agent) => String(agent.id) === agentChoice)
  const isTeacher = profile.role === 'teacher'

  return <div className="assistant-app">
    <header className="assistant-topbar">
      <div className="assistant-brand"><span className="assistant-brand-mark">C</span><span>CampusClaw <small>学习空间</small></span></div>
      <nav className="assistant-global-nav" aria-label="主导航"><button type="button" onClick={() => onNavigate()}>教学材料</button><button type="button" className={view === 'chat' ? 'active' : ''} onClick={() => setView('chat')}>智能助手</button>{isTeacher && <button type="button" className={view === 'settings' ? 'active' : ''} onClick={() => { setView('settings'); setNotice('') }}>教师配置</button>}</nav>
      <div className="assistant-account"><span className="assistant-class">{profile.class_name} 班</span><span className="assistant-account-name">{profile.username} · {isTeacher ? '教师' : '学生'}</span><button type="button" className="assistant-logout" onClick={() => void logout()}>退出</button></div>
    </header>

    {error && <div className="assistant-flash error" role="alert">{error}<button type="button" onClick={() => setError('')} aria-label="关闭错误提示">×</button></div>}
    {notice && <div className="assistant-flash notice" role="status">{notice}<button type="button" onClick={() => setNotice('')} aria-label="关闭提示">×</button></div>}

    {view === 'chat' ? <main className="assistant-layout">
      <aside className="assistant-sidebar" aria-label="我的会话">
        <div className="assistant-sidebar-intro"><span className="assistant-overline">LEARNING ASSISTANT</span><h1>解题助手</h1><p>从本班材料出发，陪你找到下一步。</p></div>
        <div className="assistant-new-chat"><label htmlFor="assistant-agent-choice">选择智能体</label><select id="assistant-agent-choice" value={agentChoice} onChange={(event) => setAgentChoice(event.target.value)}><option value="">暂无可用智能体</option>{agents.filter((agent) => agent.enabled).map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}</select><button type="button" className="assistant-primary" onClick={() => void createConversation()} disabled={!agentChoice}>＋ 新建会话</button></div>
        <div className="assistant-history-heading"><span>最近会话</span><span>{conversations.length}</span></div>
        <div className="assistant-history">{conversations.length ? conversations.map((conversation) => <button type="button" key={conversation.id} className={selected?.id === conversation.id ? 'active' : ''} onClick={() => void openConversation(conversation.id)}><span className="assistant-history-icon">◌</span><span>{conversation.title}</span></button>) : <p>新建会话后会显示在这里。</p>}</div>
        <div className="assistant-sidebar-foot"><span className="assistant-scope-dot" /> 仅检索 {profile.class_name} 班已授权材料</div>
      </aside>

      <section className="assistant-chat" aria-label="对话区域">
        <div className="assistant-chat-head"><div className="assistant-avatar">✦</div><div><strong>{activeAgent?.name ?? selectedAgent?.name ?? '解题助手'}</strong><span>{selected ? selected.title : '选择会话后开始提问'}</span></div><span className="assistant-live-badge"><i /> 本班知识库</span></div>
        <div className="assistant-message-scroll">
          {!selected ? <div className="assistant-welcome"><span className="assistant-welcome-mark">✦</span><span className="assistant-overline">A BETTER WAY TO LEARN</span><h2>先思考，再找到线索</h2><p>选择左侧智能体并新建会话。助手会先查找本班资料，再给出可追溯的提示与回答。</p><button type="button" className="assistant-primary" disabled={!agentChoice} onClick={() => void createConversation()}>开始新会话 <span>→</span></button>{isTeacher && !agents.some((agent) => agent.enabled) && <button type="button" className="assistant-link" onClick={() => setView('settings')}>先配置一个智能体 →</button>}</div> : !selected.messages?.length && !streaming ? <div className="assistant-welcome compact"><span className="assistant-welcome-mark">✦</span><h2>从一个问题开始</h2><p>描述你卡住的地方，助手会结合本班资料给出线索。</p><div className="assistant-suggestions">{suggestions.map((suggestion) => <button type="button" key={suggestion} onClick={() => setQuestion(suggestion)}>{suggestion} <span>↗</span></button>)}</div></div> : <div className="assistant-messages">{selected.messages?.map((message) => <article key={message.id} className={`assistant-message ${message.role}`}><span className="assistant-message-avatar">{message.role === 'user' ? '我' : '✦'}</span><div className="assistant-message-body"><span className="assistant-message-author">{message.role === 'user' ? '你' : activeAgent?.name ?? '解题助手'}</span><div className="assistant-message-text"><ReactMarkdown skipHtml>{message.content}</ReactMarkdown></div>{message.skill_name && <span className="assistant-skill-chip">已使用技能 · {message.skill_name}</span>}<SourceList hits={message.citations ?? []} onNavigate={onNavigate} /></div></article>)}{streaming && <article className="assistant-message assistant"><span className="assistant-message-avatar">✦</span><div className="assistant-message-body"><span className="assistant-message-author">{activeAgent?.name ?? '解题助手'} <span className="assistant-thinking">正在回答</span></span><div className="assistant-message-text" aria-live="polite"><ReactMarkdown skipHtml>{partial || '正在检索本班材料…'}</ReactMarkdown></div>{liveSkill && <span className="assistant-skill-chip">已使用技能 · {liveSkill}</span>}<SourceList hits={liveCitations} onNavigate={onNavigate} /></div></article>}<div ref={messagesEnd} /></div>}
        </div>
        <form className="assistant-composer" onSubmit={(event) => void send(event)}><div className="assistant-composer-box"><textarea aria-label="输入问题" placeholder={selected ? '说说你卡在哪里，或者直接提出问题…' : '先新建会话，然后在这里提问…'} minLength={2} maxLength={500} rows={2} value={question} onChange={(event) => setQuestion(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }} disabled={!selected || streaming} /><button type="submit" disabled={!selected || streaming || question.trim().length < 2} aria-label="发送问题">{streaming ? '···' : '↑'}</button></div><p>按 Enter 发送，Shift + Enter 换行 · 回答以本班资料为依据</p></form>
      </section>
    </main> : isTeacher && <main className="assistant-settings">
      <div className="assistant-settings-intro"><span className="assistant-overline">TEACHER WORKSPACE</span><h1>配置你的教学助手</h1><p>先配置智能体角色，再绑定技能与资料。技能决定何时使用哪种引导；工具连接单独管理。</p></div>
      <div className="assistant-settings-stats"><div><span>智能体</span><strong>{agents.length}</strong><small>设定角色和资料范围</small></div><div><span>技能</span><strong>{skills.length}</strong><small>按问题条件触发引导</small></div><div><span>本班材料</span><strong>{materials.length}</strong><small>供检索与引用</small></div></div>
      <div className="assistant-settings-tabs" role="tablist" aria-label="配置分类"><button type="button" role="tab" aria-selected={settingsSection === 'agents'} className={settingsSection === 'agents' ? 'active' : ''} onClick={() => setSettingsSection('agents')}>智能体</button><button type="button" role="tab" aria-selected={settingsSection === 'skills'} className={settingsSection === 'skills' ? 'active' : ''} onClick={() => setSettingsSection('skills')}>技能库</button><button type="button" role="tab" aria-selected={settingsSection === 'tools'} className={settingsSection === 'tools' ? 'active' : ''} onClick={() => setSettingsSection('tools')}>工具与连接</button></div>

      {settingsSection === 'agents' && <div className="assistant-settings-grid"><aside className="assistant-settings-list"><div className="assistant-settings-list-head"><div><h2>智能体</h2><p>一个智能体对应一套角色与资源绑定。</p></div><button type="button" onClick={() => setAgentForm({ ...blankAgent })}>＋ 新建</button></div>{agents.map((agent) => <button type="button" key={agent.id} className={`assistant-config-item ${agentForm.id === agent.id ? 'active' : ''}`} onClick={() => setAgentForm({ ...agent })}><span className="assistant-config-icon">✦</span><span><strong>{agent.name}</strong><small>{agent.skill_ids.length} 个技能 · {agent.material_ids.length} 份材料</small></span><span className={`assistant-status ${agent.enabled ? 'on' : ''}`}>{agent.enabled ? '启用' : '停用'}</span></button>)}{!agents.length && <p className="assistant-list-empty">还没有智能体。点击“新建”开始配置。</p>}</aside><form className="assistant-editor" onSubmit={(event) => void saveAgent(event)}><div className="assistant-editor-head"><div><span className="assistant-overline">AGENT CONFIGURATION</span><h2>{agentForm.id ? '编辑智能体' : '创建智能体'}</h2><p>学生会在新建会话时选择已启用的智能体。</p></div><label className="assistant-switch"><input type="checkbox" checked={agentForm.enabled} onChange={(event) => setAgentForm({ ...agentForm, enabled: event.target.checked })} /><span>启用</span></label></div><div className="assistant-field"><label htmlFor="agent-name">名称</label><input id="agent-name" required maxLength={64} placeholder="例如：数学解题助手" value={agentForm.name} onChange={(event) => setAgentForm({ ...agentForm, name: event.target.value })} /></div><div className="assistant-field"><label htmlFor="agent-prompt">角色提示词</label><p>说明助手的教学方式和回答边界。</p><textarea id="agent-prompt" required rows={5} maxLength={2000} placeholder="你是本班的解题助手。先了解学生的思路，再结合本班材料提供引导……" value={agentForm.prompt} onChange={(event) => setAgentForm({ ...agentForm, prompt: event.target.value })} /></div><div className="assistant-binding"><div><h3>绑定技能 <span>{agentForm.skill_ids.length}</span></h3><p>只有勾选且已启用的技能才可能被触发。</p></div><div className="assistant-binding-options">{skills.length ? skills.map((skill) => <label key={skill.id}><input type="checkbox" checked={agentForm.skill_ids.includes(skill.id)} onChange={() => toggle('skill_ids', skill.id)} /><span><strong>{skill.name}</strong><small>{skill.enabled ? '可用' : '已停用'}</small></span></label>) : <p>暂无技能。可以先保存智能体，再到“技能库”添加。</p>}</div></div><div className="assistant-binding"><div><h3>绑定教学材料 <span>{agentForm.material_ids.length}</span></h3><p>助手只会从所绑定的本班材料中检索。</p></div><div className="assistant-binding-options">{materials.length ? materials.map((material) => <label key={material.id}><input type="checkbox" checked={agentForm.material_ids.includes(material.id)} onChange={() => toggle('material_ids', material.id)} /><span><strong>{material.original_filename}</strong><small>{material.index_status === 'ready' ? '索引就绪' : '处理中或待重试'}</small></span></label>) : <p>本班暂无材料。请先到“教学材料”上传。</p>}</div></div><div className="assistant-editor-actions"><button type="submit" className="assistant-primary">保存智能体</button></div></form></div>}

      {settingsSection === 'skills' && <div className="assistant-settings-grid"><aside className="assistant-settings-list"><div className="assistant-settings-list-head"><div><h2>技能库</h2><p>这里的技能是课程定义的引导规则文件。</p></div><button type="button" onClick={() => { setEditingSkill(null); setSkillDraft({ ...blankSkill }) }}>＋ 新建</button></div>{skills.map((skill) => <button type="button" key={skill.id} className={`assistant-config-item ${editingSkill === skill.id ? 'active' : ''}`} onClick={() => { setEditingSkill(skill.id); setSkillDraft(parseSkill(skill.source_text)) }}><span className="assistant-config-icon skill">◇</span><span><strong>{skill.name}</strong><small>{parseSkill(skill.source_text).triggers || '未设置触发条件'}</small></span><span className={`assistant-status ${skill.enabled ? 'on' : ''}`}>{skill.enabled ? '启用' : '停用'}</span></button>)}{!skills.length && <p className="assistant-list-empty">还没有技能。可以新建，也可以导入课程格式的文本文件。</p>}</aside><form className="assistant-editor" onSubmit={(event) => void saveSkill(event)}><div className="assistant-editor-head"><div><span className="assistant-overline">SKILL CONFIGURATION</span><h2>{editingSkill ? '编辑技能' : '创建技能'}</h2><p>填写规则即可生成课程所需的技能文件格式。</p></div><label className="assistant-switch"><input type="checkbox" checked={skillDraft.enabled} onChange={(event) => setSkillDraft({ ...skillDraft, enabled: event.target.checked })} /><span>启用</span></label></div><div className="assistant-skill-help"><strong>技能如何生效？</strong><p>学生提问命中“适用条件”后，系统从当前智能体绑定的技能中选一个，并结合绑定材料检索。名称包含“解题引导”时，助手只给下一步提示。</p></div><div className="assistant-field"><label htmlFor="skill-name">技能名称</label><input id="skill-name" required maxLength={32} placeholder="例如：数据结构解题引导" value={skillDraft.name} onChange={(event) => setSkillDraft({ ...skillDraft, name: event.target.value })} /></div><div className="assistant-field"><label htmlFor="skill-triggers">适用条件</label><p>用逗号分隔匹配词，例如“二叉树,插入,节点”。</p><input id="skill-triggers" required maxLength={120} placeholder="二叉树,查找,插入" value={skillDraft.triggers} onChange={(event) => setSkillDraft({ ...skillDraft, triggers: event.target.value })} /></div><div className="assistant-field"><label htmlFor="skill-subject">限定资料名称 <em>可选</em></label><p>填写后仅使用标题包含该词的材料。</p><input id="skill-subject" maxLength={32} placeholder="例如：数据结构" value={skillDraft.subject} onChange={(event) => setSkillDraft({ ...skillDraft, subject: event.target.value })} /></div><div className="assistant-field"><label htmlFor="skill-instructions">引导说明</label><textarea id="skill-instructions" required rows={6} maxLength={2000} placeholder="先判断学生已有的思路，只指出下一步应检查的内容，不直接给出最终答案。" value={skillDraft.instructions} onChange={(event) => setSkillDraft({ ...skillDraft, instructions: event.target.value })} /></div><div className="assistant-tool-inline"><span>使用工具</span><strong>knowledge_search</strong><small>当前课程版本仅支持本班知识库检索</small></div><details className="assistant-source-preview"><summary>查看生成的技能文件</summary><pre>{serializeSkill(skillDraft)}</pre></details><div className="assistant-editor-actions"><input ref={skillFile} type="file" accept=".txt,.md,text/plain,text/markdown" className="assistant-hidden-file" aria-label="导入技能文件" onChange={(event) => void importSkill(event.target.files?.[0])} /><button type="button" className="assistant-secondary" onClick={() => skillFile.current?.click()}>导入文本文件</button><button type="submit" className="assistant-primary">保存技能</button></div></form></div>}

      {settingsSection === 'tools' && <div className="assistant-tools"><div className="assistant-tools-intro"><span className="assistant-overline">CAPABILITIES & CONNECTIONS</span><h2>工具与连接</h2><p>技能是教学引导规则；工具是执行能力；MCP 是连接外部工具的协议。它们各自独立配置，不应混成一个“链接”。</p></div><div className="assistant-tool-card"><span className="assistant-tool-card-icon">⌕</span><div><span className="assistant-status on">已接入 · 内置工具</span><h3>本班知识库检索</h3><p>knowledge_search 会检索智能体绑定的本班材料，回答可显示引用来源。技能可以指定使用它。</p></div></div><div className="assistant-tool-card muted-card"><span className="assistant-tool-card-icon">⌘</span><div><span className="assistant-status">当前未接入</span><h3>MCP 外部工具连接</h3><p>这个版本没有 MCP 服务端配置、授权或调用接口，因此这里不提供无效的“连接”按钮。日后接入时，应单独管理服务地址、可用工具和权限。</p></div></div></div>}
    </main>}
  </div>
}
