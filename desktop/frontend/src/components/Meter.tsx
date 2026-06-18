import { useEffect, useRef, useState } from "react";

// Stereo vertical level meter. Targets are pushed in via props (from the 30 Hz
// engine poll); a rAF loop applies smooth peak-decay so the bars feel analog.
export function Meter({ peakL, peakR }: { peakL: number; peakR: number }) {
  const target = useRef({ l: 0, r: 0 });
  target.current = { l: clamp01(peakL), r: clamp01(peakR) };
  const [disp, setDisp] = useState({ l: 0, r: 0 });

  useEffect(() => {
    let raf = 0;
    const loop = () => {
      setDisp((d) => ({
        l: Math.max(target.current.l, d.l * 0.86),
        r: Math.max(target.current.r, d.r * 0.86),
      }));
      raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(raf);
  }, []);

  return (
    <div className="meter">
      <Bar level={disp.l} />
      <Bar level={disp.r} />
    </div>
  );
}

function Bar({ level }: { level: number }) {
  // Slight perceptual curve so quiet signal is visible.
  const h = Math.round(Math.pow(level, 0.7) * 100);
  return (
    <div className="ch">
      <div className="fill" style={{ height: `${h}%` }} />
    </div>
  );
}

const clamp01 = (v: number) => (v < 0 ? 0 : v > 1 ? 1 : v || 0);
