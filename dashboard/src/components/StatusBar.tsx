import type { ConnectivityStatus } from "../hooks/useFusionData";

interface StatusBarProps {
  socketConnected: boolean;
  connectivityStatus: ConnectivityStatus;
}

export default function StatusBar({
  socketConnected,
  connectivityStatus,
}: StatusBarProps) {
  let label: string;
  let tone: "live" | "offline" | "down";

  if (!socketConnected) {
    label = "Disconnected";
    tone = "down";
  } else if (connectivityStatus === "offline-queue") {
    label = "Offline — Queueing";
    tone = "offline";
  } else if (connectivityStatus === "live") {
    label = "Live";
    tone = "live";
  } else {
    label = "Connecting…";
    tone = "down";
  }

  return (
    <div className="status-bar">
      <span className={`status-pill status-pill--${tone}`}>
        <span className="status-pill__dot" />
        {label}
      </span>
      <span className="status-bar__source">fusion-service /ws/live</span>
    </div>
  );
}
