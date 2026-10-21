import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, clearToken, getToken, messageFor, type Profile } from './api'
import LoginPage from './pages/LoginPage'
import MaterialsPage from './pages/MaterialsPage'
import TutorPage from './pages/TutorPage'
import './styles.css'

export default function App() {
  const [profile, setProfile] = useState<Profile | null>(null)
  const [loading, setLoading] = useState(true)
  const [startupError, setStartupError] = useState('')
  const [page, setPage] = useState(window.location.pathname === '/tutor' ? 'tutor' : 'materials')

  const navigate = (next: 'materials' | 'tutor', materialId?: number, tutorView?: 'chat' | 'settings') => {
    const query = next === 'materials' && materialId ? `?material=${materialId}` : next === 'tutor' && tutorView === 'settings' ? '?view=settings' : ''
    window.history.pushState(null, '', `/${next}${query}`)
    setPage(next)
  }

  useEffect(() => {
    const changed = () => setPage(window.location.pathname === '/tutor' ? 'tutor' : 'materials')
    window.addEventListener('popstate', changed)
    return () => window.removeEventListener('popstate', changed)
  }, [])

  const checkSession = useCallback(() => {
    setStartupError('')
    setLoading(true)
    if (!getToken()) {
      setProfile(null)
      window.history.replaceState(null, '', '/login')
      setLoading(false)
      return
    }
    api<Profile>('/api/me')
      .then((current) => {
        setProfile(current)
        if (window.location.pathname === '/login') window.history.replaceState(null, '', '/materials')
      })
      .catch((error: unknown) => {
        if (error instanceof ApiError && error.status === 401) {
          clearToken()
          setProfile(null)
          window.history.replaceState(null, '', '/login')
        } else {
          setStartupError(messageFor(error))
        }
      })
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { checkSession() }, [checkSession])

  if (loading) return <main className="center-screen"><p>正在检查登录状态…</p></main>
  if (startupError) return <main className="center-screen"><div className="startup-error"><p role="alert">{startupError}</p><button className="primary-button" onClick={checkSession}>重试连接</button></div></main>

  if (!profile) {
    return <LoginPage onLogin={(current) => {
      setProfile(current)
      window.history.replaceState(null, '', '/materials')
    }} />
  }

  const onAuthLost = () => {
    clearToken()
    setProfile(null)
    window.history.replaceState(null, '', '/login')
  }
  return page === 'tutor' ? <TutorPage profile={profile} onAuthLost={onAuthLost} onNavigate={(materialId) => navigate('materials', materialId)} /> : <MaterialsPage profile={profile} onAuthLost={onAuthLost} onNavigate={(view) => navigate('tutor', undefined, view)} />
}
