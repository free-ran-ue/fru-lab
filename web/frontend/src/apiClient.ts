import { Configuration, DefaultApi } from './api'

const apiBasePath = import.meta.env.VITE_API_BASE_URL || `${window.location.protocol}//${window.location.hostname}:8888`

export const api = new DefaultApi(new Configuration({
  basePath: apiBasePath,
  accessToken: () => localStorage.getItem('token') ?? '',
}))

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
