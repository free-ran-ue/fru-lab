import axios from 'axios'
import { Configuration, WebconsoleApi } from './webconsoleApi'
import { apiBaseUrl } from './apiClient'

// free5GC's webconsole is a separate service (part of each free5GC
// deployment) with its own auth entirely independent of fru-lab's own
// login. The browser reaches it through fru-lab - /api/webconsole for the
// lab free5GC, /api/core-networks/{id}/webconsole for a free5GC core
// network (see the backend's webconsole proxies) - so whichever host port it
// is published on the page needs no port of its own; the proxy sits behind
// fru-lab's login, so every request also carries fru-lab's JWT. Its admin
// user is recreated by webconsole itself on every restart
// (backend/webui_service/webui_init.go calls SetAdmin() unconditionally), so
// admin/free5gc always works and isn't a secret worth prompting the operator
// for - we just log in behind the scenes and attach the token it wants
// (a custom "Token" header, not "Authorization: Bearer").
const WEBCONSOLE_USERNAME = 'admin'
const WEBCONSOLE_PASSWORD = 'free5gc'

const labWebconsoleBasePath = import.meta.env.VITE_WEBCONSOLE_BASE_URL || `${apiBaseUrl}/api/webconsole`

// webconsoleBasePath is where a core's webconsole is proxied: the lab
// free5GC's without a core network id, else that free5GC core network's.
export function webconsoleBasePath(coreId?: string): string {
  return coreId ? `${apiBaseUrl}/api/core-networks/${encodeURIComponent(coreId)}/webconsole` : labWebconsoleBasePath
}

function fruLabAuth(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

// newClient is one webconsole's client with its own cached login token -
// every free5GC has its own webconsole and so its own token.
function newClient(basePath: string): WebconsoleApi {
  const axiosInstance = axios.create()
  let cachedToken: string | null = null
  let pendingLogin: Promise<string> | null = null

  // the login goes through the generated client too, on its own axios
  // instance: it carries fru-lab's JWT but not the webconsole token it is
  // about to get (so it never loops through the interceptor below)
  const loginAxios = axios.create()
  loginAxios.interceptors.request.use((config) => {
    for (const [key, value] of Object.entries(fruLabAuth())) config.headers.set(key, value)
    return config
  })
  const loginApi = new WebconsoleApi(new Configuration({ basePath }), basePath, loginAxios)

  async function login(): Promise<string> {
    const response = await loginApi.login({ username: WEBCONSOLE_USERNAME, password: WEBCONSOLE_PASSWORD })
    const token = response.data.access_token ?? ''
    cachedToken = token
    return token
  }

  async function getToken(forceRefresh = false): Promise<string> {
    if (!forceRefresh && cachedToken) {
      return cachedToken
    }
    if (!pendingLogin) {
      pendingLogin = login().finally(() => {
        pendingLogin = null
      })
    }
    return pendingLogin
  }

  axiosInstance.interceptors.request.use(async (config) => {
    config.headers.set('Token', await getToken())
    for (const [key, value] of Object.entries(fruLabAuth())) config.headers.set(key, value)
    return config
  })

  axiosInstance.interceptors.response.use(
    (response) => response,
    async (error) => {
      const original = error.config
      if (error.response?.status === 401 && original && !original._retriedAfterRelogin) {
        original._retriedAfterRelogin = true
        original.headers.Token = await getToken(true)
        return axiosInstance(original)
      }
      return Promise.reject(error)
    },
  )

  return new WebconsoleApi(new Configuration({ basePath }), basePath, axiosInstance)
}

const clients = new Map<string, WebconsoleApi>()

// webconsoleClient is the webconsole client of the lab free5GC (no id) or of
// a free5GC core network; one per base path, kept for the session.
export function webconsoleClient(coreId?: string): WebconsoleApi {
  const basePath = webconsoleBasePath(coreId)
  let client = clients.get(basePath)
  if (!client) {
    client = newClient(basePath)
    clients.set(basePath, client)
  }
  return client
}

export function extractWebconsoleErrorMessage(error: unknown, fallback: string): string {
  const cause =
    typeof error === 'object' &&
    error !== null &&
    'response' in error &&
    typeof (error as { response?: { data?: { cause?: string } } }).response?.data?.cause === 'string'
      ? (error as { response?: { data?: { cause?: string } } }).response?.data?.cause
      : undefined

  return cause || fallback
}
