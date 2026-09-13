export default function StatusPill({ status }: { status: string }) {
  const cls = status === 'error' ? 'pill pill-bad' : status === 'ok' ? 'pill pill-good' : 'pill pill-neutral'
  return <span className={cls}>{status}</span>
}
