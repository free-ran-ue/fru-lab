import LoginPage from './page/login/LoginPage'
import { Navigate, Route, Routes } from 'react-router-dom'
import DashboardPage from './page/dashboard/DashboardPage'
import SubscribersPage from './page/subscribers/SubscribersPage'
import SubscriberFormPage from './page/subscribers/SubscriberFormPage'
import SubscriberViewPage from './page/subscribers/SubscriberViewPage'

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
      <Route
        path="/subscribers"
        element={(
          <RequireAuth>
            <SubscribersPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/subscribers/new"
        element={(
          <RequireAuth>
            <SubscriberFormPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/subscribers/:ueId/:plmnId/edit"
        element={(
          <RequireAuth>
            <SubscriberFormPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/subscribers/:ueId/:plmnId"
        element={(
          <RequireAuth>
            <SubscriberViewPage />
          </RequireAuth>
        )}
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
