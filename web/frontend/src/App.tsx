import LoginPage from './page/login/LoginPage'
import { Navigate, Route, Routes } from 'react-router-dom'
import DashboardPage from './page/dashboard/DashboardPage'
import SubscribersPage from './page/subscribers/SubscribersPage'
import SubscriberFormPage from './page/subscribers/SubscriberFormPage'
import SubscriberViewPage from './page/subscribers/SubscriberViewPage'
import LogsPage from './page/logs/LogsPage'
import ImagesPage from './page/images/ImagesPage'
import TesterSetupPage from './page/tester/TesterSetupPage'
import TesterRunPage from './page/tester/TesterRunPage'
import TesterHistoryPage from './page/tester/TesterHistoryPage'

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
      <Route
        path="/logs"
        element={(
          <RequireAuth>
            <LogsPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/images"
        element={(
          <RequireAuth>
            <ImagesPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/tester"
        element={(
          <RequireAuth>
            <TesterSetupPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/tester/run"
        element={(
          <RequireAuth>
            <TesterRunPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/tester/history"
        element={(
          <RequireAuth>
            <TesterHistoryPage />
          </RequireAuth>
        )}
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
