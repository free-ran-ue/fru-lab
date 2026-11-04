import { useState, type FormEvent } from 'react'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import { useNavigate } from 'react-router-dom'
import { LogoMark } from '../../components/sidebar/Sidebar'
import logoBanner from '../../assets/frulab-logo.png'
import styles from './login-page.module.css'

// FEATURES are the brand panel's three lines under the banner.
const FEATURES = [
  { title: 'Deploy a 5G lab', text: 'free5GC, gNB and UEs on Docker in one click.' },
  { title: 'Load-test any core', text: 'Tens of thousands of UEs and line-rate GTP-U.' },
  { title: 'Keep the evidence', text: "Every run's report, per tester, ready to compare." },
]

export default function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsLoading(true)

    try {
      const response = await api.login({ username, password })
      const token = response.data.token || ''
      localStorage.setItem('token', token)
      localStorage.setItem('username', username)
      addSuccess(response.data.message || 'Login successful')
      navigate('/', { replace: true })
    } catch (error: unknown) {
      addError(extractErrorMessage(error, 'Login failed'))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className={styles.page}>
      <NotificationContainer
        errors={errors}
        successes={successes}
        onClose={removeNotification}
      />

      <section className={styles.brandPanel} aria-label="fru-lab">
        <img
          className={styles.banner}
          src={logoBanner}
          alt="fru-lab: free5GC and free-ran-ue in containers, connected over 5G"
        />
        <div className={styles.features}>
          {FEATURES.map((f) => (
            <div key={f.title} className={styles.feature}>
              <span className={styles.featureTitle}>{f.title}</span>
              <span className={styles.featureText}>{f.text}</span>
            </div>
          ))}
        </div>
      </section>

      <main className={styles.formPanel}>
        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.brand}>
            <LogoMark size={36} />
            <span className={styles.wordmark}>fru-<span className={styles.wordmarkLab}>lab</span></span>
          </div>

          <div className={styles.headerBlock}>
            <h1 className={styles.title}>Sign in</h1>
            <p className={styles.subtitle}>Use the account in fru-lab&apos;s config.yaml.</p>
          </div>

          <div className={styles.field}>
            <label className={styles.label} htmlFor="username">Username</label>
            <input
              id="username"
              className={styles.input}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              required
            />
          </div>

          <div className={styles.field}>
            <label className={styles.label} htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              className={styles.input}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
            />
          </div>

          <button type="submit" className={styles.submit} disabled={isLoading}>
            {isLoading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </main>
    </div>
  )
}
