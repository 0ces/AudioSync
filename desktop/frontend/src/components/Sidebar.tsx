type View = "mixer" | "connections" | "diagnostics" | "settings";

const NAV: { id: View; label: string; icon: JSX.Element }[] = [
  { id: "mixer", label: "Mixer", icon: <BarsIcon /> },
  { id: "connections", label: "Connections", icon: <LinkIcon /> },
  { id: "diagnostics", label: "Diagnostics", icon: <PulseIcon /> },
  { id: "settings", label: "Settings", icon: <GearIcon /> },
];

export function Sidebar({
  view,
  setView,
  running,
  onToggle,
  version,
}: {
  view: View;
  setView: (v: View) => void;
  running: boolean;
  onToggle: () => void;
  version: string;
}) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="logo"><BarsIcon /></span>
        <h1>AudioSync</h1>
      </div>

      <div className={`engine-card ${running ? "" : "off"}`} onClick={onToggle} title="Toggle engine">
        <span className="power"><PowerIcon /></span>
        <div className="lines">
          <span className="state">{running ? "Running" : "Stopped"}</span>
          <span className="sub">Audio engine</span>
        </div>
      </div>

      <nav className="nav">
        {NAV.map((n) => (
          <button
            key={n.id}
            className={view === n.id ? "active" : ""}
            onClick={() => setView(n.id)}
          >
            <span className="nico">{n.icon}</span>
            {n.label}
          </button>
        ))}
      </nav>

      <div className="spacer" />
      <div className="foot">{version}</div>
    </aside>
  );
}

export type { View };

/* --- inline icons (stroke = currentColor) --- */
function BarsIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
      <line x1="6" y1="20" x2="6" y2="12" /><line x1="12" y1="20" x2="12" y2="5" /><line x1="18" y1="20" x2="18" y2="9" />
    </svg>
  );
}
function PowerIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M12 3v9" /><path d="M6.4 6.4a8 8 0 1 0 11.2 0" />
    </svg>
  );
}
function LinkIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M9 12h6" /><path d="M9 7H6a5 5 0 0 0 0 10h3" /><path d="M15 7h3a5 5 0 0 1 0 10h-3" />
    </svg>
  );
}
function PulseIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 12h4l3 8 4-16 3 8h4" />
    </svg>
  );
}
function GearIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19 12a7 7 0 0 0-.1-1l2-1.5-2-3.5-2.4 1a7 7 0 0 0-1.7-1l-.3-2.5h-4l-.3 2.5a7 7 0 0 0-1.7 1l-2.4-1-2 3.5 2 1.5a7 7 0 0 0 0 2l-2 1.5 2 3.5 2.4-1a7 7 0 0 0 1.7 1l.3 2.5h4l.3-2.5a7 7 0 0 0 1.7-1l2.4 1 2-3.5-2-1.5a7 7 0 0 0 .1-1Z" />
    </svg>
  );
}
