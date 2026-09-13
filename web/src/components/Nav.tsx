import { NavLink } from 'react-router-dom'

export default function Nav() {
  return (
    <header className="nav">
      <div className="nav-brand">
        <span className="nav-mark" aria-hidden="true" />
        Ember
      </div>
      <nav className="nav-links">
        <NavLink to="/traces" className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}>
          Traces
        </NavLink>
        <NavLink to="/sessions" className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}>
          Sessions
        </NavLink>
        <NavLink to="/analytics" className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}>
          Analytics
        </NavLink>
      </nav>
      <a
        className="nav-docs"
        href="https://github.com/sakshamgoswami/ember#readme"
        target="_blank"
        rel="noreferrer"
      >
        Docs
      </a>
    </header>
  )
}
