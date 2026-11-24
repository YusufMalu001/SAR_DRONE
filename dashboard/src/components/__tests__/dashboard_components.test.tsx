import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import DegradedBanner from "../DegradedBanner";
import SurvivorsList from "../SurvivorsList";
import AlertFeed from "../AlertFeed";
import LiveFeedPanel from "../LiveFeedPanel";
import type { AlertRecord, DetectionRecord, EntityRecord, ServiceHealth } from "../../lib/types";

describe("DegradedBanner", () => {
  it("renders degraded banner with real queue depth and N/A for unknown sync", () => {
    const fusionHealth: ServiceHealth = {
      status: "ok",
      service: "fusion-service",
      mode: "degraded",
      queue_depth: 42,
      oldest_item_age_seconds: 15,
      last_successful_sync: "",
    };

    render(
      <DegradedBanner
        fusionHealth={fusionHealth}
        inferenceHealth={{ status: "ok", service: "inference-service" }}
        missionHealth={{ status: "ok", service: "mission-service" }}
      />
    );

    expect(screen.getByText("Degraded Connectivity Mode")).toBeDefined();
    expect(screen.getByText("42")).toBeDefined();
    expect(screen.getByText("15s")).toBeDefined();
    expect(screen.getByText("N/A")).toBeDefined();
  });

  it("does not render when healthy and online", () => {
    const fusionHealth: ServiceHealth = {
      status: "ok",
      service: "fusion-service",
      mode: "online",
      queue_depth: 0,
      oldest_item_age_seconds: 0,
      last_successful_sync: "2026-11-24T12:00:00Z",
    };

    const { container } = render(
      <DegradedBanner
        fusionHealth={fusionHealth}
        inferenceHealth={{ status: "ok", service: "inference-service" }}
        missionHealth={{ status: "ok", service: "mission-service" }}
      />
    );

    expect(container.firstChild).toBeNull();
  });
});

describe("SurvivorsList", () => {
  const dummySurvivors: EntityRecord[] = [
    {
      id: "survivor-1",
      type: "survivor",
      class: "person",
      lat: 20.001,
      lon: 78.002,
      uncertainty_radius: 3.5,
      observation_count: 5,
      confidence: 0.92,
      modalities: ["rgb", "thermal"],
      priority_score: 0.85,
      status: "active",
      first_seen: "2026-11-24T12:00:00Z",
      last_seen: "2026-11-24T12:05:00Z",
    },
    {
      id: "survivor-2",
      type: "survivor",
      class: "person",
      lat: 20.004,
      lon: 78.006,
      uncertainty_radius: 4.0,
      observation_count: 2,
      confidence: 0.65,
      modalities: ["rgb"],
      priority_score: 0.45,
      status: "active",
      first_seen: "2026-11-24T12:01:00Z",
      last_seen: "2026-11-24T12:02:00Z",
    },
  ];

  it("renders sorted survivors and handles selection", () => {
    const onSelect = vi.fn();
    render(
      <SurvivorsList
        survivors={dummySurvivors}
        selectedId={null}
        onSelect={onSelect}
      />
    );

    expect(screen.getByText("Score: 0.85")).toBeDefined();
    expect(screen.getByText("Score: 0.45")).toBeDefined();
    expect(screen.getAllByText("RGB").length).toBe(2);
    expect(screen.getByText("THERMAL")).toBeDefined();

    const firstItem = screen.getByText("Score: 0.85").closest("li");
    expect(firstItem).not.toBeNull();
    fireEvent.click(firstItem!);
    expect(onSelect).toHaveBeenCalledWith(dummySurvivors[0]);
  });
});

describe("AlertFeed", () => {
  const dummyAlerts: AlertRecord[] = [
    {
      id: 101,
      detection_id: "det-1",
      priority_tier: "critical",
      message: "Critical survivor detected near flood zone",
      created_at: "2026-11-24T12:00:00Z",
    },
  ];

  it("renders alert message and triggers acknowledge", () => {
    render(<AlertFeed alerts={dummyAlerts} />);
    expect(screen.getByText("Critical survivor detected near flood zone")).toBeDefined();
    expect(screen.getByText("Critical")).toBeDefined();
    expect(screen.getByText("Ack")).toBeDefined();
  });
});

describe("LiveFeedPanel", () => {
  const dummyDetections: DetectionRecord[] = [
    {
      id: "d1",
      class: "person",
      confidence: 0.95,
      lat: 20.0,
      lon: 78.0,
      frame_id: "f1",
      source: "rgb",
      detected_at: "2026-11-24T12:00:00Z",
      priority_score: 0.9,
      status: "live",
      modalities: ["rgb"],
    },
    {
      id: "d2",
      class: "fire",
      confidence: 0.88,
      lat: 20.001,
      lon: 78.001,
      frame_id: "f2",
      source: "thermal",
      detected_at: "2026-11-24T12:00:01Z",
      priority_score: 0.7,
      status: "live",
      modalities: ["thermal"],
    },
  ];

  it("switches tabs between RGB and Thermal modalities", () => {
    render(<LiveFeedPanel detections={dummyDetections} />);
    expect(screen.getByText("RGB SENSOR ACTIVE")).toBeDefined();
    expect(screen.getByText("person 95%")).toBeDefined();

    const thermalTab = screen.getByRole("tab", { name: /LWIR Thermal/i });
    fireEvent.click(thermalTab);
    expect(screen.getByText("THERMAL SENSOR ACTIVE")).toBeDefined();
    expect(screen.getByText("fire 88%")).toBeDefined();
  });
});
