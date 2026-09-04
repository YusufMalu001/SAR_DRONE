import { useEffect, useState } from "react";
import { fetchFusionHealth, fetchInferenceHealth } from "../lib/api";
import type { ServiceHealth } from "../lib/types";

interface Row {
  name: string;
  health?: ServiceHealth;
  error?: string;
}

export default function SystemStatus() {
  const [rows, setRows] = useState<Row[]>([
    { name: "inference-service" },
    { name: "fusion-service" },
  ]);

  useEffect(() => {
    let cancelled = false;

    async function poll() {
      const [inference, fusion] = await Promise.allSettled([
        fetchInferenceHealth(),
        fetchFusionHealth(),
      ]);
      if (cancelled) return;
      setRows([
        inference.status === "fulfilled"
          ? { name: "inference-service", health: inference.value }
          : { name: "inference-service", error: String(inference.reason) },
        fusion.status === "fulfilled"
          ? { name: "fusion-service", health: fusion.value }
          : { name: "fusion-service", error: String(fusion.reason) },
      ]);
    }

    poll();
    const id = setInterval(poll, 5000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  return (
    <section>
      <h1>System Status</h1>
      <table>
        <thead>
          <tr>
            <th>Service</th>
            <th>Status</th>
            <th>Details</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.name}>
              <td>{row.name}</td>
              <td>{row.health?.status ?? (row.error ? "unreachable" : "checking...")}</td>
              <td>
                {row.health
                  ? JSON.stringify(row.health)
                  : row.error ?? ""}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
