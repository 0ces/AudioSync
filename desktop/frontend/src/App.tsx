import { useEffect, useState } from "react";
import { api } from "./lib/api";
import { useSnapshot } from "./lib/useSnapshot";
import { Sidebar, View } from "./components/Sidebar";
import { ChannelStrip } from "./components/ChannelStrip";
import { Meter } from "./components/Meter";
import { DeviceMenu } from "./components/DeviceMenu";
import { AddMachineModal } from "./components/AddMachineModal";
import { ConnectionsView, DiagnosticsView, SettingsView } from "./components/views";

export default function App() {
  const snap = useSnapshot(33);
  const [view, setView] = useState<View>("mixer");
  const [addOpen, setAddOpen] = useState(false);
  const [theme, setTheme] = useState(() => localStorage.getItem("theme") || "dark");
  const [version, setVersion] = useState("dev");

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("theme", theme);
  }, [theme]);

  useEffect(() => {
    api.Version().then((v) => setVersion(v === "dev" ? "dev build" : `v${v}`)).catch(() => {});
  }, []);

  const running = snap?.running ?? false;
  const sources = snap?.sources ?? [];
  const device = snap?.outputDevice ?? "Default Output";
  const deviceId = snap?.outputDeviceId ?? "";
  const listen = snap?.listenAddr ?? ":4010";
  const masterVol = snap?.masterVolume ?? 1;

  const onVolume = (id: number, v: number) => api.SetVolume(id, v);
  const onMute = (id: number, m: boolean) => api.SetMuted(id, m);
  const onSolo = (id: number, s: boolean) => api.SetSolo(id, s);
  const onToggle = () => (running ? api.Stop() : api.Start());

  const prog = sources.reduce(
    (m, s) => (s.muted ? m : { l: Math.max(m.l, s.peakL), r: Math.max(m.r, s.peakR) }),
    { l: 0, r: 0 }
  );

  return (
    <div className="app">
      <Sidebar view={view} setView={setView} running={running} onToggle={onToggle} version={version} />

      <main className="main">
        <header className="topbar">
          <div>
            <h2>{titleFor(view)}</h2>
            <div className="sub">{subFor(view, sources.length)}</div>
          </div>
          <div className="actions">
            <DeviceMenu current={device} currentId={deviceId} />
            <button className="pill accent" onClick={() => setAddOpen(true)}>+ Add machine</button>
          </div>
        </header>

        {view === "connections" && snap ? (
          <ConnectionsView snap={snap} />
        ) : view === "diagnostics" && snap ? (
          <DiagnosticsView snap={snap} />
        ) : view === "settings" && snap ? (
          <SettingsView snap={snap} theme={theme} setTheme={setTheme} />
        ) : sources.length === 0 ? (
          <EmptyState listen={listen} />
        ) : (
          <div className="mixer">
            <div className="strips">
              {sources.map((s) => (
                <ChannelStrip key={s.id} src={s} onVolume={onVolume} onMute={onMute} onSolo={onSolo} />
              ))}

              {/* Master / output strip */}
              <div className="strip master">
                <span className="badge">Master</span>
                <div className="name">Output</div>
                <div className="conn" style={{ color: "var(--text-mut)" }}>{device}</div>
                <div className="body">
                  <Meter peakL={prog.l} peakR={prog.r} />
                  <div className="fader">
                    <input
                      className="vfader"
                      type="range"
                      min={0}
                      max={1.5}
                      step={0.01}
                      value={masterVol}
                      onChange={(e) => api.SetMasterVolume(parseFloat(e.target.value))}
                      onDoubleClick={() => api.SetMasterVolume(1)}
                    />
                  </div>
                </div>
                <div className="pct">{Math.round(masterVol * 100)}%</div>
                <div className="mfoot">
                  {sources.length} source{sources.length === 1 ? "" : "s"}
                  <br />
                  port {portOf(listen)} · 48 kHz
                </div>
              </div>
            </div>
          </div>
        )}
      </main>

      {addOpen && <AddMachineModal listen={listen} onClose={() => setAddOpen(false)} />}
    </div>
  );
}

function EmptyState({ listen }: { listen: string }) {
  const addr = `${hostHint()}${listen.startsWith(":") ? listen : ":" + portOf(listen)}`;
  return (
    <div className="empty">
      <div className="ico">
        <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
          <line x1="6" y1="20" x2="6" y2="12" /><line x1="12" y1="20" x2="12" y2="5" /><line x1="18" y1="20" x2="18" y2="9" />
        </svg>
      </div>
      <h3>No machines yet</h3>
      <p>Run AudioSync on another computer and point it at this machine's address.</p>
      <div className="addr-card">
        <div>
          <div className="lbl">This machine</div>
          <div className="val">{addr}</div>
        </div>
        <button onClick={() => navigator.clipboard?.writeText(addr)}>Copy</button>
      </div>
    </div>
  );
}

const titleFor = (v: View) => (v === "mixer" ? "Mixer" : v.charAt(0).toUpperCase() + v.slice(1));
const subFor = (v: View, n: number) =>
  v === "mixer" ? `${n} source${n === 1 ? "" : "s"} receiving · mixing to one output` : "";
const portOf = (addr: string) => addr.split(":").pop() || "4010";
const hostHint = () => "this-mac.local";
