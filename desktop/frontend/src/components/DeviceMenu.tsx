import { useEffect, useRef, useState } from "react";
import { api, OutputDevice } from "../lib/api";

// Output-device picker. Renders as the header pill; click opens a dropdown of
// the system's playback devices and switches on select.
export function DeviceMenu({ current, currentId }: { current: string; currentId: string }) {
  const [open, setOpen] = useState(false);
  const [devices, setDevices] = useState<OutputDevice[]>([]);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    api.ListOutputDevices().then(setDevices).catch(() => setDevices([]));
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  const pick = (d: OutputDevice) => {
    api.SetOutputDevice(d.default ? "" : d.id, d.name);
    setOpen(false);
  };

  return (
    <div className="devmenu" ref={ref}>
      <button className="pill" onClick={() => setOpen((o) => !o)}>
        <span className="dot"><DiscIcon /></span>
        {current}
        <span className="caret">▾</span>
      </button>
      {open && (
        <div className="devlist">
          {devices.length === 0 && <div className="devitem muted">No devices</div>}
          {devices.map((d) => {
            const selected = d.id === currentId || (d.default && currentId === "");
            return (
              <button key={d.id} className={`devitem ${selected ? "sel" : ""}`} onClick={() => pick(d)}>
                <span className="check">{selected ? "✓" : ""}</span>
                <span className="dname">{d.name}</span>
                {d.default && <span className="dtag">default</span>}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function DiscIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="12" r="8" /><circle cx="12" cy="12" r="2.5" fill="currentColor" stroke="none" />
    </svg>
  );
}
