import { NavLink, Route, Routes } from "react-router-dom";
import "./App.css";
import Detections from "./pages/Detections";
import LiveMap from "./pages/LiveMap";
import SystemStatus from "./pages/SystemStatus";

function App() {
  return (
    <div className="app">
      <header className="app-header">
        <h2>SAR Drone Command Center</h2>
        <nav>
          <NavLink to="/" end>
            Live Map
          </NavLink>
          <NavLink to="/detections">Detections</NavLink>
          <NavLink to="/status">System Status</NavLink>
        </nav>
      </header>
      <main className="app-main">
        <Routes>
          <Route path="/" element={<LiveMap />} />
          <Route path="/detections" element={<Detections />} />
          <Route path="/status" element={<SystemStatus />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
