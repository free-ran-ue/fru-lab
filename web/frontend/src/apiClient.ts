import axios from 'axios'
import { Configuration, DefaultApi } from './api'

const apiBasePath = import.meta.env.VITE_API_BASE_URL || `${window.location.protocol}//${window.location.hostname}:8888`

const axiosInstance = axios.create()

// a 401 here means the JWT itself is gone/expired (not a bad login attempt -
// that's handled by the caller's own catch), so the session is unrecoverable
// and the only sane move is to drop the stale token and send the user back
// to /login, same as a manual logout. Module-level, so this can't use
// react-router's useNavigate - a hard redirect is the simplest way to force
// every component back to a clean, logged-out state.
axiosInstance.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      if (window.location.pathname !== '/login') {
        window.location.assign('/login')
      }
    }
    return Promise.reject(error)
  },
)

export const api = new DefaultApi(new Configuration({
  basePath: apiBasePath,
  accessToken: () => localStorage.getItem('token') ?? '',
}), apiBasePath, axiosInstance)

export function extractErrorMessage(error: unknown, fallback: string): string {
  const message =
    typeof error === 'object' &&
    error !== null &&
    'response' in error &&
    typeof (error as { response?: { data?: { message?: string } } }).response?.data?.message === 'string'
      ? (error as { response?: { data?: { message?: string } } }).response?.data?.message
      : undefined

  return message || fallback
}
