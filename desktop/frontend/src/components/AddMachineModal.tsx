// Receiver-side "Add machine" helper. Senders connect *into* this receiver, so
// there's nothing to add here — instead we show how to point a sender at this
// machine (auto-discovery, or a manual address + command).
export function AddMachineModal({ listen, onClose }: { listen: string; onClose: () => void }) {
  const port = listen.split(":").pop() || "4010";
  const addr = `this-mac.local:${port}`;
  const cmd = `audiosync -role=sender -source=system -discover`;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>Add a machine</h3>
        <p className="muted">
          Run AudioSync on the other computer as a <b>sender</b>. This receiver advertises
          itself on your network, so the sender finds it automatically:
        </p>
        <div className="cmd">
          <code>{cmd}</code>
          <button onClick={() => navigator.clipboard?.writeText(cmd)}>Copy</button>
        </div>
        <p className="muted">Or point it at this machine directly:</p>
        <div className="cmd">
          <code>{addr}</code>
          <button onClick={() => navigator.clipboard?.writeText(addr)}>Copy</button>
        </div>
        <div className="modal-foot">
          <button className="primary" onClick={onClose}>Done</button>
        </div>
      </div>
    </div>
  );
}
