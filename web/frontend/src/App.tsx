import LoginPage from './page/login/LoginPage'
import { Navigate, Route, Routes } from 'react-router-dom'
import DashboardPage from './page/dashboard/DashboardPage'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('token')
  if (!token) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={(
          <RequireAuth>
            <DashboardPage />
          </RequireAuth>
        )}
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
