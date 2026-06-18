import { useEffect, useRef, useState } from "react";
import { api, Snapshot } from "./api";

// Polls the Go engine for the full UI state at a fixed rate (~30 Hz). The
// engine owns all state; the frontend only renders it.
export function useSnapshot(intervalMs = 33): Snapshot | null {
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const busy = useRef(false);

  useEffect(() => {
    let live = true;
    const tick = async () => {
      if (busy.current) return;
      busy.current = true;
      try {
        const s = await api.GetSnapshot();
        if (live) setSnap(s);
      } catch {
        /* engine not ready yet */
      } finally {
        busy.current = false;
      }
    };
    const id = setInterval(tick, intervalMs);
    tick();
    return () => {
      live = false;
      clearInterval(id);
    };
  }, [intervalMs]);

  return snap;
}
