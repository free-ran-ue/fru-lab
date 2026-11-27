import { Navigate, Outlet, Route, Routes } from 'react-router-dom'
import LoginPage from './page/login/LoginPage'
import DashboardPage from './page/dashboard/DashboardPage'
import SubscribersPage from './page/subscribers/SubscribersPage'
import SubscriberFormPage from './page/subscribers/SubscriberFormPage'
import SubscriberViewPage from './page/subscribers/SubscriberViewPage'
import LogsPage from './page/overview/LogsPage'
import EventsPage from './page/overview/EventsPage'
import ImagesPage from './page/images/ImagesPage'
import CoreNetworkPage from './page/core/CoreNetworkPage'
import FreeRanPage from './page/ran/FreeRanPage'
import FreeUePage from './page/ue/FreeUePage'
import { LabProvider } from './lab/LabContext'
import { EventsProvider } from './lab/console/EventsContext'

// Signed in: one lab poller and one set of log streams for every view.
function SignedIn() {
  if (!localStorage.getItem('token')) return <Navigate to="/login" replace />
  return (
    <LabProvider>
      <EventsProvider>
        <Outlet />
      </EventsProvider>
    </LabProvider>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<SignedIn />}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/core-network" element={<CoreNetworkPage />} />
        <Route path="/core-network/subscribers" element={<SubscribersPage />} />
        <Route path="/core-network/subscribers/new" element={<SubscriberFormPage />} />
        <Route path="/core-network/subscribers/:ueId/:plmnId/edit" element={<SubscriberFormPage />} />
        <Route path="/core-network/subscribers/:ueId/:plmnId" element={<SubscriberViewPage />} />
        <Route path="/free-ran" element={<FreeRanPage />} />
        {/* profiles are edited on the free-ran / free-ue pages themselves */}
        <Route path="/free-ran/profiles" element={<Navigate to="/free-ran" replace />} />
        <Route path="/free-ue" element={<FreeUePage />} />
        <Route path="/free-ue/profiles" element={<Navigate to="/free-ue" replace />} />
        <Route path="/events" element={<EventsPage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/images" element={<ImagesPage />} />
      </Route>
      {/* Subscribers moved under Core Network */}
      <Route path="/subscribers/*" element={<Navigate to="/core-network/subscribers" replace />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
