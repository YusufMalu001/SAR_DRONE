import { useFusionData } from "../hooks/useFusionData";

export default function Detections() {
  const { detections } = useFusionData();

  return (
    <section>
      <h1>Detections</h1>
      {detections.length === 0 ? (
        <p>No detections recorded yet.</p>
      ) : (
        <table className="detections-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Class</th>
              <th>Confidence</th>
              <th>Coordinates</th>
              <th>Source</th>
              <th>Time</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {detections.map((d) => (
              <tr key={d.id}>
                <td>{d.id.slice(0, 8)}...</td>
                <td>{d.class}</td>
                <td>{(d.confidence * 100).toFixed(1)}%</td>
                <td>
                  {d.lat.toFixed(5)}, {d.lon.toFixed(5)}
                </td>
                <td>{d.source}</td>
                <td>{new Date(d.detected_at).toLocaleTimeString()}</td>
                <td>{d.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
