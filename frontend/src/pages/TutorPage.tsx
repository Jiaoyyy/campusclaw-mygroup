import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { api, ApiError, messageFor, streamMessage, type Material, type Profile, type RetrievalHit } from '../api'
import './tutor.css'

type Skill = { id: number, name: string, enabled: boolean, source_text: string }
type Agent = { id: number, name: string, prompt: string, enabled: boolean, skill_ids: number[], material_ids: number[], mcp_tool_names: string[] }
type ChatMessage = { id: number, role: 'user' | 'assistant', content: string, citations: RetrievalHit[], skill_name?: string, mcp_tool_name?: string }
type Conversation = { id: number, agent_id: number, title: string, messages?: ChatMessage[] }
type SkillDraft = { source: string, enabled: boolean }
type MCPStatus = { status: 'disabled' | 'unavailable' | 'connected', tools: { name: string, description: string }[] }
type SettingsSection = 'agents' | 'skills' | 'tools'
type PromptTemplate = { id: string, name: string, description: string, prompt: string }

const blankAgent: Agent = { id: 0, name: '', prompt: '', enabled: true, skill_ids: [], material_ids: [], mcp_tool_names: [] }
const blankSkill: SkillDraft = { source: '# 解题引导\n\n## 目标\n帮助学生理解题目并推进思路。\n\n## 工作方式\n- 先了解学生已经尝试的步骤。\n- 结合本班材料给出下一步提示，并标注已有出处。\n- 不直接给出最终答案。\n', enabled: true }
const suggestions = ['这道题我应该先检查什么？', '请根据本班材料给我一个解题提示', '我卡在第二步，可以提示思路吗？']

function SourceList({ hits, onNavigate }: { hits: RetrievalHit[], onNavigate: (materialId: number) => void }) {
  if (!hits.length) return null
  return <div className="assistant-sources"><span className="assistant-sources-title">参考材料</span><div className="assistant-source-items">{hits.map((hit) => <button type="button" key={hit.chunk_id} onClick={() => onNavigate(hit.material_id)} title="前往教学材料查看原文"><span>[{hit.citation_number}]</span> {hit.title} · 第 {hit.chunk_index} 段 <span aria-hidden="true">↗</span></button>)}</div></div>
}

export default function TutorPage({ profile, onAuthLost, onNavigate }: { profile: Profile, onAuthLost: () => void, onNavigate: (materialId?: number) => void }) {
  const [agents, setAgents] = useState<Agent[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [promptTemplates, setPromptTemplates] = useState<PromptTemplate[]>([])
  const [materials, setMaterials] = useState<Material[]>([])
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [selected, setSelected] = useState<Conversation | null>(null)
  const [agentChoice, setAgentChoice] = useState('')
  const [question, setQuestion] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [partial, setPartial] = useState('')
  const [liveCitations, setLiveCitations] = useState<RetrievalHit[]>([])
  const [liveSkill, setLiveSkill] = useState('')
  const [liveMCPTool, setLiveMCPTool] = useState('')
  const [liveMCPWarning, setLiveMCPWarning] = useState('')
  const [mcpStatus, setMCPStatus] = useState<MCPStatus>({ status: 'disabled', tools: [] })
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
      const [a, s, c, m, templates] = await Promise.all([
        api<{ agents: Agent[] }>('/api/agents'), api<{ skills: Skill[] }>('/api/skills'),
        api<{ conversations: Conversation[] }>('/api/conversations'), api<{ items: Material[] }>('/api/materials'),
        api<{ templates: PromptTemplate[] }>('/api/prompt-templates'),
      ])
      setAgents(a.agents); setSkills(s.skills); setConversations(c.conversations); setMaterials(m.items)
      setPromptTemplates(templates.templates)
      if (!initializedForms.current) {
        if (a.agents.length) setAgentForm({ ...a.agents[0] })
        if (s.skills.length) { setEditingSkill(s.skills[0].id); setSkillDraft({ source: s.skills[0].source_text, enabled: s.skills[0].enabled }) }
        initializedForms.current = true
      }
      setAgentChoice((old) => a.agents.some((agent) => agent.enabled && String(agent.id) === old) ? old : String(a.agents.find((agent) => agent.enabled)?.id ?? ''))
    } catch (cause) { handleError(cause) }
  }, [handleError])
  useEffect(() => { void refresh() }, [refresh])
  useEffect(() => {
    if (profile.role !== 'teacher' || view !== 'settings') return
    void api<MCPStatus>('/api/mcp/tools').then(setMCPStatus).catch(handleError)
  }, [profile.role, view, settingsSection, handleError])
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
    const text = question.trim(); setQuestion(''); setStreaming(true); setPartial(''); setLiveCitations([]); setLiveSkill(''); setLiveMCPTool(''); setLiveMCPWarning(''); setError(''); setNotice('')
    setSelected({ ...selected, messages: [...(selected.messages ?? []), { id: -Date.now(), role: 'user', content: text, citations: [] }] })
    try {
      await streamMessage(selected.id, text, profile.csrf_token, (eventName, data) => {
        if (eventName === 'meta') { setLiveCitations((data.citations as RetrievalHit[]) ?? []); setLiveSkill((data.skill as { name?: string })?.name ?? ''); setLiveMCPTool(String(data.mcp_tool_name ?? '')); setLiveMCPWarning(String(data.mcp_warning ?? '')); if (data.mcp_warning) setNotice(String(data.mcp_warning)) }
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
        method: editingSkill ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': profile.csrf_token }, body: JSON.stringify({ source_text: skillDraft.source, enabled: skillDraft.enabled }),
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
  function toggleMCP(name: string) {
    setAgentForm((old) => ({ ...old, mcp_tool_names: old.mcp_tool_names.includes(name) ? old.mcp_tool_names.filter((item) => item !== name) : [...old.mcp_tool_names, name] }))
  }
  async function importSkill(file?: File) {
    if (!file) return
    try {
      const text = await file.text()
      if (!text.trim()) throw new Error('技能文件不能为空。')
      setSkillDraft({ source: text, enabled: true }); setEditingSkill(null); setError(''); setNotice(`已读取 ${file.name} 的完整内容，请确认后保存。`)
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
        <div className="assistant-sidebar-foot"><span className="assistant-scope-dot" /> 本班材料独立授权 · 外部工具只接收调用参数</div>
      </aside>

      <section className="assistant-chat" aria-label="对话区域">
        <div className="assistant-chat-head"><div className="assistant-avatar">✦</div><div><strong>{activeAgent?.name ?? selectedAgent?.name ?? '解题助手'}</strong><span>{selected ? selected.title : '选择会话后开始提问'}</span></div><span className="assistant-live-badge"><i /> {activeAgent?.mcp_tool_names?.length ? "本班知识库 + 外部工具" : "本班知识库"}</span></div>
        <div className="assistant-message-scroll">
          {!selected ? <div className="assistant-welcome"><span className="assistant-welcome-mark">✦</span><span className="assistant-overline">A BETTER WAY TO LEARN</span><h2>先思考，再找到线索</h2><p>选择左侧智能体并新建会话。助手会先查找本班资料，再给出可追溯的提示与回答。</p><button type="button" className="assistant-primary" disabled={!agentChoice} onClick={() => void createConversation()}>开始新会话 <span>→</span></button>{isTeacher && !agents.some((agent) => agent.enabled) && <button type="button" className="assistant-link" onClick={() => setView('settings')}>先配置一个智能体 →</button>}</div> : !selected.messages?.length && !streaming ? <div className="assistant-welcome compact"><span className="assistant-welcome-mark">✦</span><h2>从一个问题开始</h2><p>描述你卡住的地方，助手会结合本班资料给出线索。</p><div className="assistant-suggestions">{suggestions.map((suggestion) => <button type="button" key={suggestion} onClick={() => setQuestion(suggestion)}>{suggestion} <span>↗</span></button>)}</div></div> : <div className="assistant-messages">{selected.messages?.map((message) => <article key={message.id} className={`assistant-message ${message.role}`}><span className="assistant-message-avatar">{message.role === 'user' ? '我' : '✦'}</span><div className="assistant-message-body"><span className="assistant-message-author">{message.role === 'user' ? '你' : activeAgent?.name ?? '解题助手'}</span><div className="assistant-message-text"><ReactMarkdown skipHtml>{message.content}</ReactMarkdown></div>{message.skill_name && <span className="assistant-skill-chip">已使用技能 · {message.skill_name}</span>}{message.mcp_tool_name && <span className="assistant-skill-chip">MCP 工具 · {message.mcp_tool_name}</span>}<SourceList hits={message.citations ?? []} onNavigate={onNavigate} /></div></article>)}{streaming && <article className="assistant-message assistant"><span className="assistant-message-avatar">✦</span><div className="assistant-message-body"><span className="assistant-message-author">{activeAgent?.name ?? '解题助手'} <span className="assistant-thinking">正在回答</span></span><div className="assistant-message-text" aria-live="polite"><ReactMarkdown skipHtml>{partial || '正在检索本班材料…'}</ReactMarkdown></div>{liveSkill && <span className="assistant-skill-chip">已使用技能 · {liveSkill}</span>}{liveMCPTool && <span className="assistant-skill-chip">MCP 工具 · {liveMCPTool}</span>}{liveMCPWarning && <span className="assistant-mcp-warning">{liveMCPWarning}</span>}<SourceList hits={liveCitations} onNavigate={onNavigate} /></div></article>}<div ref={messagesEnd} /></div>}
        </div>
        <form className="assistant-composer" onSubmit={(event) => void send(event)}><div className="assistant-composer-box"><textarea aria-label="输入问题" placeholder={selected ? '说说你卡在哪里，或者直接提出问题…' : '先新建会话，然后在这里提问…'} minLength={2} maxLength={500} rows={2} value={question} onChange={(event) => setQuestion(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }} disabled={!selected || streaming} /><button type="submit" disabled={!selected || streaming || question.trim().length < 2} aria-label="发送问题">{streaming ? '···' : '↑'}</button></div><p>按 Enter 发送，Shift + Enter 换行 · 回答以本班资料为依据</p></form>
      </section>
    </main> : isTeacher && <main className="assistant-settings">
      <div className="assistant-settings-intro"><span className="assistant-overline">TEACHER WORKSPACE</span><h1>配置你的教学助手</h1><p>可直接使用内置的解题、复习和学习计划应用，也可选择提示词模板创建自己的智能体。</p></div>
      <div className="assistant-settings-stats"><div><span>智能体</span><strong>{agents.length}</strong><small>设定角色和资料范围</small></div><div><span>技能</span><strong>{skills.length}</strong><small>按问题条件触发引导</small></div><div><span>本班材料</span><strong>{materials.length}</strong><small>供检索与引用</small></div></div>
      <div className="assistant-settings-tabs" role="tablist" aria-label="配置分类"><button type="button" role="tab" aria-selected={settingsSection === 'agents'} className={settingsSection === 'agents' ? 'active' : ''} onClick={() => setSettingsSection('agents')}>智能体</button><button type="button" role="tab" aria-selected={settingsSection === 'skills'} className={settingsSection === 'skills' ? 'active' : ''} onClick={() => setSettingsSection('skills')}>技能库</button><button type="button" role="tab" aria-selected={settingsSection === 'tools'} className={settingsSection === 'tools' ? 'active' : ''} onClick={() => setSettingsSection('tools')}>工具与连接</button></div>

      {settingsSection === 'agents' && <div className="assistant-template-bar"><label htmlFor="prompt-template">使用提示词模板</label><select id="prompt-template" value="" onChange={(event) => { const template = promptTemplates.find((item) => item.id === event.target.value); if (template) { setAgentForm({ ...agentForm, prompt: template.prompt }); setNotice(`已填入「${template.name}」模板，可继续编辑并保存智能体。`) } }}><option value="">选择模板填入角色提示词</option>{promptTemplates.map((template) => <option key={template.id} value={template.id}>{template.name} · {template.description}</option>)}</select><span>选择后替换当前草稿中的角色提示词。</span></div>}

      {settingsSection === 'agents' && <div className="assistant-settings-grid">
        <aside className="assistant-settings-list">
          <div className="assistant-settings-list-head"><div><h2>智能体 / AI 应用</h2><p>每个应用拥有自己的角色、技能和资料。</p></div><button type="button" onClick={() => setAgentForm({ ...blankAgent })}>＋ 新建</button></div>
          {agents.map((agent) => <button type="button" key={agent.id} className={`assistant-config-item ${agentForm.id === agent.id ? 'active' : ''}`} onClick={() => setAgentForm({ ...agent })}><span className="assistant-config-icon">✦</span><span><strong>{agent.name}</strong><small>{agent.skill_ids.length} 个技能 · {agent.material_ids.length} 份材料</small></span><span className={`assistant-status ${agent.enabled ? 'on' : ''}`}>{agent.enabled ? '启用' : '停用'}</span></button>)}
          {!agents.length && <p className="assistant-list-empty">还没有智能体。点击“新建”开始配置。</p>}
        </aside>
        <form className="assistant-editor" onSubmit={(event) => void saveAgent(event)}>
          <div className="assistant-editor-head"><div><span className="assistant-overline">AGENT CONFIGURATION</span><h2>{agentForm.id ? '编辑智能体' : '创建智能体'}</h2><p>学生会在新建会话时选择已启用的智能体。</p></div><label className="assistant-switch"><input type="checkbox" checked={agentForm.enabled} onChange={(event) => setAgentForm({ ...agentForm, enabled: event.target.checked })} /><span>启用</span></label></div>
          <div className="assistant-field"><label htmlFor="agent-name">名称</label><input id="agent-name" required maxLength={64} placeholder="例如：数学解题助手" value={agentForm.name} onChange={(event) => setAgentForm({ ...agentForm, name: event.target.value })} /></div>
          <div className="assistant-field"><label htmlFor="agent-prompt">角色提示词</label><p>说明助手的教学方式和回答边界，也可以从上方模板填入。</p><textarea id="agent-prompt" required rows={5} maxLength={2000} placeholder="你是本班的解题助手……" value={agentForm.prompt} onChange={(event) => setAgentForm({ ...agentForm, prompt: event.target.value })} /></div>
          <div className="assistant-binding"><div><h3>绑定技能 <span>{agentForm.skill_ids.length}</span></h3><p>只有勾选且已启用的技能才可能被触发。</p></div><div className="assistant-binding-options">{skills.length ? skills.map((skill) => <label key={skill.id}><input type="checkbox" checked={agentForm.skill_ids.includes(skill.id)} onChange={() => toggle('skill_ids', skill.id)} /><span><strong>{skill.name}</strong><small>{skill.enabled ? '可用' : '已停用'}</small></span></label>) : <p>暂无技能。可以先保存智能体，再到“技能库”添加。</p>}</div></div>
          <div className="assistant-binding"><div><h3>绑定教学材料 <span>{agentForm.material_ids.length}</span></h3><p>助手只会从所绑定的本班材料中检索。</p></div><div className="assistant-binding-options">{materials.length ? materials.map((material) => <label key={material.id}><input type="checkbox" checked={agentForm.material_ids.includes(material.id)} onChange={() => toggle('material_ids', material.id)} /><span><strong>{material.original_filename}</strong><small>{material.index_status === 'ready' ? '索引就绪' : '处理中或待重试'}</small></span></label>) : <p>本班暂无材料。请先到“教学材料”上传。</p>}</div></div>
          <div className="assistant-binding"><div><h3>外部 MCP 工具 <span>{agentForm.mcp_tool_names.length}</span></h3><p>由服务端配置连接；仅勾选获准工具。每轮最多调用一次，工具结果与本班材料引用分开显示。</p></div><div className="assistant-binding-options">{mcpStatus.status === 'connected' && mcpStatus.tools.length ? mcpStatus.tools.map((tool) => <label key={tool.name}><input type="checkbox" checked={agentForm.mcp_tool_names.includes(tool.name)} onChange={() => toggleMCP(tool.name)} /><span><strong>{tool.name}</strong><small>{tool.description || '外部只读工具'}</small></span></label>) : <p>{mcpStatus.status === 'disabled' ? '尚未配置 MCP 服务地址与允许的工具。' : 'MCP 服务暂不可用或没有已获准工具。'}</p>}</div></div>
          <div className="assistant-editor-actions"><button type="submit" className="assistant-primary">保存智能体</button></div>
        </form>
      </div>}

      {settingsSection === 'skills' && <div className="assistant-settings-grid">
        <aside className="assistant-settings-list">
          <div className="assistant-settings-list-head"><div><h2>技能文件</h2><p>每条技能保存一份完整 Markdown 原文。</p></div></div>
          <div className="assistant-skill-create-actions"><button type="button" onClick={() => { setEditingSkill(null); setSkillDraft({ ...blankSkill }) }}>＋ 新建</button><input ref={skillFile} type="file" accept=".md,.markdown,.txt,text/plain,text/markdown" className="assistant-hidden-file" aria-label="导入 Markdown 技能文件" onChange={(event) => void importSkill(event.target.files?.[0])} /><button type="button" onClick={() => skillFile.current?.click()}>↑ 导入 Markdown</button></div>
          {skills.map((skill) => <button type="button" key={skill.id} className={`assistant-config-item ${editingSkill === skill.id ? 'active' : ''}`} onClick={() => { setEditingSkill(skill.id); setSkillDraft({ source: skill.source_text, enabled: skill.enabled }) }}><span className="assistant-config-icon skill">◇</span><span><strong>{skill.name}</strong><small>完整 Markdown 文件</small></span><span className={`assistant-status ${skill.enabled ? 'on' : ''}`}>{skill.enabled ? '启用' : '停用'}</span></button>)}
          {!skills.length && <p className="assistant-list-empty">还没有技能。可新建或导入 .md 文件。</p>}
        </aside>
        <form className="assistant-editor" onSubmit={(event) => void saveSkill(event)}>
          <div className="assistant-editor-head"><div><span className="assistant-overline">SKILL MARKDOWN</span><h2>{editingSkill ? '编辑技能文件' : '新建技能文件'}</h2><p>用 # 标题或 frontmatter 的 name 命名；其余内容原样保存，不会被表单重组。</p></div><label className="assistant-switch"><input type="checkbox" checked={skillDraft.enabled} onChange={(event) => setSkillDraft({ ...skillDraft, enabled: event.target.checked })} /><span>启用</span></label></div>
          <div className="assistant-field"><label htmlFor="skill-source">skills.md 完整内容</label><p>可直接粘贴或导入 Markdown；保存后原文参与技能引导。旧版中文字段文件仍可继续使用。</p><textarea id="skill-source" className="assistant-markdown-editor" required rows={20} maxLength={8000} spellCheck={false} value={skillDraft.source} onChange={(event) => setSkillDraft({ ...skillDraft, source: event.target.value })} /></div>
          <div className="assistant-editor-actions"><button type="button" className="assistant-secondary" onClick={() => { const url = URL.createObjectURL(new Blob([skillDraft.source], { type: 'text/markdown;charset=utf-8' })); const link = document.createElement('a'); link.href = url; link.download = 'skills.md'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000) }}>导出 skills.md</button><button type="submit" className="assistant-primary">保存完整文件</button></div>
        </form>
      </div>}

      {settingsSection === 'tools' && <div className="assistant-tools"><div className="assistant-tools-intro"><span className="assistant-overline">CAPABILITIES & CONNECTIONS</span><h2>工具与连接</h2><p>技能是教学引导规则；工具是执行能力；MCP 是连接外部工具的协议。它们各自独立配置，不应混成一个“链接”。</p></div><div className="assistant-tool-card"><span className="assistant-tool-card-icon">⌕</span><div><span className="assistant-status on">已接入 · 内置工具</span><h3>本班知识库检索</h3><p>knowledge_search 会检索智能体绑定的本班材料，回答可显示引用来源。技能可以指定使用它。</p></div></div><div className="assistant-tool-card"><span className="assistant-tool-card-icon">⌘</span><div><span className={`assistant-status ${mcpStatus.status === "connected" ? "on" : ""}`}>{mcpStatus.status === "connected" ? "已连接" : mcpStatus.status === "unavailable" ? "连接不可用" : "尚未配置"}</span><h3>MCP 外部工具连接</h3><p>服务端通过 Streamable HTTP 连接外部 MCP 服务。地址、密钥和允许列表由部署人员在 .env 配置；教师可在智能体中绑定已获准的工具。</p>{mcpStatus.tools.length > 0 && <div className="assistant-mcp-tools">{mcpStatus.tools.map((tool) => <span key={tool.name} title={tool.description}>{tool.name}</span>)}</div>}</div></div></div>}
    </main>}
  </div>
}
