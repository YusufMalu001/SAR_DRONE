import { useEffect, useState } from "react";
import { fetchOfflineStatus, setOffline } from "../lib/api";

// Demo control for showing offline-then-sync behavior live. Only rendered
// in `npm run dev` — import.meta.env.DEV is a build-time constant, so this
// never ships in a production build.
export default function DebugPanel() {
  const [offline, setOfflineState] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchOfflineStatus()
      .then((res) => setOfflineState(res.offline))
      .catch((err) => console.error("fetchOfflineStatus failed:", err));
  }, []);

  if (!import.meta.env.DEV) return null;

  async function toggle() {
    setPending(true);
    setError(null);
    try {
      const result = await setOffline(!offline);
      setOfflineState(result.offline);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="debug-panel">
      <span className="debug-panel__label">Dev only — demo controls</span>
      <button
        className={
          offline
            ? "debug-panel__button debug-panel__button--active"
            : "debug-panel__button"
        }
        onClick={toggle}
        disabled={pending}
      >
        {pending
          ? "Toggling…"
          : offline
            ? "Go online"
            : "Simulate offline"}
      </button>
      {error && <span className="debug-panel__error">{error}</span>}
    </div>
  );
}
