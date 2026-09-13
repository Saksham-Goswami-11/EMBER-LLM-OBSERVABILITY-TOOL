import { Navigate, Route, Routes } from 'react-router-dom'
import Nav from './components/Nav'
import Traces from './pages/Traces'
import TraceDetail from './pages/TraceDetail'
import Sessions from './pages/Sessions'
import Analytics from './pages/Analytics'

export default function App() {
  return (
    <div className="app">
      <Nav />
      <main className="content">
        <Routes>
          <Route path="/" element={<Navigate to="/traces" replace />} />
          <Route path="/traces" element={<Traces />} />
          <Route path="/traces/:id" element={<TraceDetail />} />
          <Route path="/sessions" element={<Sessions />} />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="*" element={<Navigate to="/traces" replace />} />
        </Routes>
      </main>
    </div>
  )
}
