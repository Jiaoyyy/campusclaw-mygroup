import { useState, type FormEvent } from 'react'
import { api, clearToken, messageFor, setToken, type Profile } from '../api'

export default function LoginPage({ onLogin }: { onLogin: (profile: Profile) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const login = await api<{ status: string, access_token: string, token_type: string }>('/api/login?mode=token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      if (login.token_type !== 'Bearer' || !login.access_token) throw new Error('missing bearer token')
      setToken(login.access_token)
      onLogin(await api<Profile>('/api/me'))
    } catch (cause) {
      clearToken()
      setError(messageFor(cause))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="login-shell">
      <section className="login-intro">
        <span className="eyebrow">CampusClaw · 教学材料库</span>
        <h1>让每一份教学材料<br />留在自己的班级。</h1>
        <p>教师上传课程文件，学生查看本班材料与正文。请使用预置账号登录。</p>
      </section>
      <section className="login-card" aria-labelledby="login-title">
        <div className="card-mark">CC</div>
        <h2 id="login-title">登录材料库</h2>
        <p className="muted">请输入你的课程账号与密码</p>
        <form onSubmit={submit}>
          <label htmlFor="username">账号</label>
          <input id="username" autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} required />
          <label htmlFor="password">密码</label>
          <input id="password" type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
          {error && <p className="alert" role="alert">{error}</p>}
          <button className="primary-button" type="submit" disabled={busy}>{busy ? '正在登录…' : '登录'}</button>
        </form>
        <p className="fine-print">账号由课程管理员预置；本页不提供公开注册。</p>
      </section>
    </main>
  )
}
