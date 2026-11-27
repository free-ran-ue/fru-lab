import LoginPage from './page/login/LoginPage'
import { Navigate, Route, Routes } from 'react-router-dom'
import DashboardPage from './page/dashboard/DashboardPage'
import SubscribersPage from './page/subscribers/SubscribersPage'
import SubscriberFormPage from './page/subscribers/SubscriberFormPage'
import SubscriberViewPage from './page/subscribers/SubscriberViewPage'
import LogsPage from './page/logs/LogsPage'
import ImagesPage from './page/images/ImagesPage'
import CoreNetworkPage from './page/core/CoreNetworkPage'
import FreeRanPage from './page/ran/FreeRanPage'
import FreeUePage from './page/ue/FreeUePage'

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
        path="/core-network"
        element={(
          <RequireAuth>
            <CoreNetworkPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/free-ran"
        element={(
          <RequireAuth>
            <FreeRanPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/free-ue"
        element={(
          <RequireAuth>
            <FreeUePage />
          </RequireAuth>
        )}
      />
      <Route
        path="/core-network/subscribers"
        element={(
          <RequireAuth>
            <SubscribersPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/core-network/subscribers/new"
        element={(
          <RequireAuth>
            <SubscriberFormPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/core-network/subscribers/:ueId/:plmnId/edit"
        element={(
          <RequireAuth>
            <SubscriberFormPage />
          </RequireAuth>
        )}
      />
      <Route
        path="/core-network/subscribers/:ueId/:plmnId"
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
      {/* the Status page replaced the System page */}
      {/* Subscribers moved under Core Network */}
      <Route path="/subscribers/*" element={<Navigate to="/core-network/subscribers" replace />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
