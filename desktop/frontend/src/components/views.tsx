import { useState } from "react";
import { api, Source, Snapshot } from "../lib/api";

const PLATFORM: Record<string, string> = { windows: "WIN", macos: "MAC", linux: "LNX", unknown: "NET" };
const connLabel = (c: string) => (c === "live" ? "Live" : c === "stalled" ? "Connecting" : "Offline");
const rate = (hz: number) => (hz ? `${(hz / 1000).toFixed(hz % 1000 ? 1 : 0)} kHz` : "—");

/* ---------------- Connections ---------------- */
export function ConnectionsView({ snap }: { snap: Snapshot }) {
  const sources = snap.sources ?? [];
  return (
    <div className="view">
      <div className="self-card">
        <div>
          <div className="lbl">This machine · receiver</div>
          <div className="val">{snap.listenAddr}</div>
        </div>
        <button className="pill" onClick={() => navigator.clipboard?.writeText(snap.listenAddr)}>Copy address</button>
      </div>

      {sources.length === 0 ? (
        <div className="muted pad">No machines connected. Start a sender and point it at the address above.</div>
      ) : (
        <div className="rows">
          {sources.map((s) => <MachineRow key={s.id} src={s} />)}
        </div>
      )}
    </div>
  );
}

function MachineRow({ src }: { src: Source }) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(src.name);
  const commit = () => { setEditing(false); if (name.trim()) api.Rename(src.id, name.trim()); };
  return (
    <div className="mrow">
      <span className="badge">{PLATFORM[src.platform] ?? "NET"}</span>
      {editing ? (
        <input className="rename" autoFocus value={name}
          onChange={(e) => setName(e.target.value)} onBlur={commit}
          onKeyDown={(e) => e.key === "Enter" && commit()} />
      ) : (
        <span className="mname" onDoubleClick={() => setEditing(true)} title="Double-click to rename">{src.name}</span>
      )}
      <span className={`sdot ${src.conn}`} />
      <span className="mmeta">{connLabel(src.conn)}</span>
      <span className="mmeta">{rate(src.sampleRate)}</span>
      <button className="mini" onClick={() => setEditing(true)}>Rename</button>
    </div>
  );
}

/* ---------------- Diagnostics ---------------- */
export function DiagnosticsView({ snap }: { snap: Snapshot }) {
  const sources = snap.sources ?? [];
  if (sources.length === 0) return <div className="muted pad">No streams to diagnose yet.</div>;
  return (
    <div className="view">
      <table className="diag">
        <thead>
          <tr><th>Source</th><th>State</th><th>Health</th><th>Peak L/R</th><th>Drift</th><th>Rate</th></tr>
        </thead>
        <tbody>
          {sources.map((s) => (
            <tr key={s.id}>
              <td><span className="badge sm">{PLATFORM[s.platform] ?? "NET"}</span> {s.name}</td>
              <td><span className={`sdot ${s.conn}`} /> {connLabel(s.conn)}</td>
              <td><span className={`tag ${s.health}`}>{s.health}</span></td>
              <td className="mono">{db(s.peakL)} / {db(s.peakR)}</td>
              <td className="mono">{s.driftPpm >= 0 ? "+" : ""}{s.driftPpm.toFixed(0)} ppm</td>
              <td className="mono">{rate(s.sampleRate)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const db = (lin: number) => (lin <= 0.0001 ? "−∞" : `${(20 * Math.log10(lin)).toFixed(0)} dB`);

/* ---------------- Settings ---------------- */
export function SettingsView({
  snap,
  theme,
  setTheme,
}: {
  snap: Snapshot;
  theme: string;
  setTheme: (t: string) => void;
}) {
  // Single "Latency <-> Stability" knob -> prefill ms. Buffer scales with it.
  const [prefill, setPrefill] = useState(snap.prefillMs || 20);
  const commit = (ms: number) => api.SetLatencyProfile(ms, Math.max(200, ms * 8));

  return (
    <div className="view">
      <div className="setrow"><span>Role</span><b>{snap.role}</b></div>
      <div className="setrow"><span>Listen address</span><b className="mono">{snap.listenAddr}</b></div>
      <div className="setrow"><span>Output device</span><b>{snap.outputDevice}</b></div>
      <div className="setrow"><span>Engine</span><b>{snap.running ? "Running" : "Stopped"}</b></div>

      <div className="setblock">
        <div className="setlabel">
          <span>Latency profile</span>
          <b className="mono">{prefill} ms buffer</b>
        </div>
        <input
          className="hslider"
          type="range"
          min={5}
          max={60}
          step={5}
          value={prefill}
          onChange={(e) => setPrefill(parseInt(e.target.value))}
          onMouseUp={() => commit(prefill)}
          onKeyUp={() => commit(prefill)}
        />
        <div className="sliderends"><span>Lower latency</span><span>More stable</span></div>
      </div>

      <div className="setrow">
        <span>Theme</span>
        <div className="seg">
          <button className={theme === "dark" ? "on" : ""} onClick={() => setTheme("dark")}>Dark</button>
          <button className={theme === "light" ? "on" : ""} onClick={() => setTheme("light")}>Light</button>
        </div>
      </div>

      <p className="muted pad">Launch-at-login is coming next.</p>
    </div>
  );
}
