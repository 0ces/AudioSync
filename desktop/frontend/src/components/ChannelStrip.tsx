import { Source } from "../lib/api";
import { Meter } from "./Meter";

const PLATFORM_BADGE: Record<string, string> = {
  windows: "WIN",
  macos: "MAC",
  linux: "LNX",
  unknown: "NET",
};

const RATE = (hz: number) => (hz ? `${(hz / 1000).toFixed(hz % 1000 ? 1 : 0)} kHz` : "—");

const HEALTH_LABEL: Record<string, string> = {
  good: "Good",
  filling: "Filling",
  starving: "Starving",
};

export function ChannelStrip({
  src,
  onVolume,
  onMute,
  onSolo,
}: {
  src: Source;
  onVolume: (id: number, v: number) => void;
  onMute: (id: number, m: boolean) => void;
  onSolo: (id: number, s: boolean) => void;
}) {
  return (
    <div className="strip">
      <span className="badge">{PLATFORM_BADGE[src.platform] ?? "NET"}</span>
      <div className="name" title={src.name}>{src.name}</div>
      <div className="conn">
        <span className={`sdot ${src.conn}`} />
        {connLabel(src.conn)}
      </div>

      <div className="body">
        <Meter peakL={src.muted ? 0 : src.peakL} peakR={src.muted ? 0 : src.peakR} />
        <div className="fader">
          <input
            className="vfader"
            type="range"
            min={0}
            max={1.5}
            step={0.01}
            value={src.volume}
            onChange={(e) => onVolume(src.id, parseFloat(e.target.value))}
            onDoubleClick={() => onVolume(src.id, 1)}
          />
        </div>
      </div>

      <div className="pct">{Math.round(src.volume * 100)}%</div>

      <div className="ms">
        <button
          className={`m ${src.muted ? "on" : ""}`}
          onClick={() => onMute(src.id, !src.muted)}
          title="Mute"
        >
          M
        </button>
        <button
          className={`s ${src.solo ? "on" : ""}`}
          onClick={() => onSolo(src.id, !src.solo)}
          title="Solo"
        >
          S
        </button>
      </div>

      <div className={`health ${src.health}`}>
        <span className="hbars">
          <i style={{ height: 5 }} />
          <i style={{ height: 8 }} />
          <i style={{ height: 11 }} />
        </span>
        {HEALTH_LABEL[src.health] ?? "—"}
      </div>
      <div className="rate">{RATE(src.sampleRate)}</div>
    </div>
  );
}

function connLabel(c: string) {
  return c === "live" ? "Live" : c === "stalled" ? "Connecting" : "Offline";
}
