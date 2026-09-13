import { api } from '../api'
import { useAsync } from '../useAsync'
import { formatTime, shortId } from '../format'

export default function Sessions() {
  const { data: sessions, error, loading } = useAsync(() => api.listSessions(), [])

  return (
    <div className="page">
      <div className="page-head">
        <h1>Sessions</h1>
        <p className="page-sub">
          Traces grouped by conversation, when the instrumentation reports a <code>session.id</code> or{' '}
          <code>gen_ai.conversation.id</code>.
        </p>
      </div>

      {error && <div className="banner banner-error">Couldn't load sessions: {error}</div>}

      {!loading && sessions && sessions.length === 0 && (
        <div className="empty-state">
          <p className="empty-title">No sessions yet</p>
          <p className="empty-body">
            Sessions appear once traces carry a <code>session.id</code> attribute. Single-shot calls with no
            conversation identity still show up in Traces — they just won't group here.
          </p>
        </div>
      )}

      {sessions && sessions.length > 0 && (
        <div className="table-card">
          <table>
            <thead>
              <tr>
                <th>Session</th>
                <th className="num-col">Traces</th>
                <th>First seen</th>
                <th>Last seen</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((s) => (
                <tr key={s.id}>
                  <td className="mono">{shortId(s.id, 16)}</td>
                  <td className="num-col num">{s.traceCount}</td>
                  <td className="mono dim">{formatTime(s.firstSeenMs)}</td>
                  <td className="mono dim">{formatTime(s.lastSeenMs)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
