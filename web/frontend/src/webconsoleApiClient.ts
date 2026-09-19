import axios from 'axios'
import { Configuration, WebconsoleApi } from './webconsoleApi'

// free5GC's webconsole is a separate service (its own compose target,
// exposed on :5000) with its own auth entirely independent of fru-lab's own
// login. Its admin user is recreated by webconsole itself on every restart
// (backend/webui_service/webui_init.go calls SetAdmin() unconditionally), so
// admin/free5gc always works and isn't a secret worth prompting the operator
// for - we just log in behind the scenes and attach the token it wants
// (a custom "Token" header, not "Authorization: Bearer").
const WEBCONSOLE_USERNAME = 'admin'
const WEBCONSOLE_PASSWORD = 'free5gc'

const webconsoleBasePath = import.meta.env.VITE_WEBCONSOLE_BASE_URL
  || `${window.location.protocol}//${window.location.hostname}:5000`

const axiosInstance = axios.create()

let cachedToken: string | null = null
let pendingLogin: Promise<string> | null = null

async function login(): Promise<string> {
  const response = await axios.post(`${webconsoleBasePath}/api/login`, {
    username: WEBCONSOLE_USERNAME,
    password: WEBCONSOLE_PASSWORD,
  })
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

export const webconsoleApi = new WebconsoleApi(new Configuration({ basePath: webconsoleBasePath }), webconsoleBasePath, axiosInstance)

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
