import { test, expect } from "@playwright/test";
import * as path from "path";
import * as fs from "fs";

test.describe("Command Centre Dashboard E2E", () => {
  test.beforeEach(async ({ page }) => {
    // Intercept backend API calls with realistic scenario payloads
    await page.route("**/health", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "ok",
          service: "fusion-service",
          mode: "online",
          queue_depth: 0,
          oldest_item_age_seconds: 0,
          last_successful_sync: new Date().toISOString(),
        }),
      });
    });

    await page.route("**/entities*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "survivor-alpha-1",
            type: "survivor",
            class: "person",
            lat: 20.0015,
            lon: 78.0025,
            uncertainty_radius: 4.2,
            observation_count: 8,
            confidence: 0.94,
            modalities: ["rgb", "thermal"],
            priority_score: 0.88,
            status: "active",
            first_seen: new Date(Date.now() - 60000).toISOString(),
            last_seen: new Date().toISOString(),
          },
          {
            id: "hazard-beta-1",
            type: "hazard",
            class: "fire",
            lat: 20.0035,
            lon: 78.0045,
            uncertainty_radius: 50.0,
            observation_count: 12,
            confidence: 0.91,
            modalities: ["thermal"],
            priority_score: 0.75,
            status: "active",
            first_seen: new Date(Date.now() - 120000).toISOString(),
            last_seen: new Date().toISOString(),
          },
        ]),
      });
    });

    await page.route("**/detections*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "det-rgb-1",
            class: "person",
            confidence: 0.94,
            lat: 20.0015,
            lon: 78.0025,
            frame_id: "frame_001",
            source: "rgb",
            detected_at: new Date().toISOString(),
            priority_score: 0.88,
            status: "live",
            modalities: ["rgb", "thermal"],
          },
          {
            id: "det-thermal-1",
            class: "fire",
            confidence: 0.91,
            lat: 20.0035,
            lon: 78.0045,
            frame_id: "frame_002",
            source: "thermal",
            detected_at: new Date().toISOString(),
            priority_score: 0.75,
            status: "live",
            modalities: ["thermal"],
          },
        ]),
      });
    });

    await page.route("**/alerts*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: 201,
            detection_id: "det-rgb-1",
            priority_tier: "critical",
            message: "High priority survivor identified in flood evacuation zone",
            created_at: new Date().toISOString(),
            state: "active",
          },
        ]),
      });
    });

    await page.route("**/mission/status", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          state: "COVERAGE_SURVEY",
          target_waypoints_count: 24,
          current_waypoint_idx: 7,
          armed: true,
          battery_pct: 88.5,
          telemetry: {
            lat: 20.002,
            lon: 78.003,
            alt: 50.0,
            heading: 180.0,
            timestamp: new Date().toISOString(),
          },
        }),
      });
    });

    await page.route("**/hazards/geojson", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          type: "FeatureCollection",
          features: [
            {
              type: "Feature",
              geometry: {
                type: "Polygon",
                coordinates: [
                  [
                    [78.003, 20.003],
                    [78.005, 20.003],
                    [78.005, 20.005],
                    [78.003, 20.005],
                    [78.003, 20.003],
                  ],
                ],
              },
              properties: { class: "fire", buffer_meters: 50 },
            },
          ],
        }),
      });
    });

    await page.route("**/mission/coverage", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ type: "FeatureCollection", features: [] }),
      });
    });

    await page.route("**/mission/routes/*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ type: "FeatureCollection", features: [] }),
      });
    });

    await page.route("**/tiles/*", async (route) => {
      // 1x1 transparent png
      const transparentPng = Buffer.from(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
        "base64"
      );
      await route.fulfill({
        status: 200,
        contentType: "image/png",
        body: transparentPng,
      });
    });
  });

  test("loads command center, renders map, survivors, and alert feed", async ({ page }) => {
    await page.goto("/");

    // 1. Check title & headers
    await expect(page.getByText("SAR Drone Command Center")).toBeVisible();
    await expect(page.getByText("SitRep (HTML)")).toBeVisible();
    await expect(page.getByText("SitRep (JSON)")).toBeVisible();

    // 2. Check Survivors list
    await expect(page.getByText("Score: 0.88")).toBeVisible();
    await expect(page.getByText("Survivors (1)")).toBeVisible();

    // 3. Check Hazards list
    await expect(page.getByText("Hazards (1)")).toBeVisible();

    // 4. Check Alerts feed
    await expect(
      page.getByText("High priority survivor identified in flood evacuation zone")
    ).toBeVisible();

    // 5. Check Live feed panel tabs
    await expect(page.getByText("RGB Optical")).toBeVisible();
    await page.getByRole("tab", { name: "LWIR Thermal" }).click();
    await expect(page.getByText("THERMAL SENSOR ACTIVE")).toBeVisible();

    // 6. Check Mission control panel
    await expect(page.getByText("Mission Control")).toBeVisible();
    await expect(page.getByText("COVERAGE_SURVEY")).toBeVisible();
    await expect(page.getByText("Preview Coverage Plan")).toBeVisible();

    // 7. Click survivor item to open detail drawer
    await page.getByText("Score: 0.88").click();
    await expect(page.getByText("SURVIVOR: PERSON")).toBeVisible();
    await expect(page.getByText("Mark Survivor Rescued")).toBeVisible();

    // 8. Capture screenshot for README and docs
    const docsDir = path.resolve(process.cwd(), "../docs");
    if (!fs.existsSync(docsDir)) {
      fs.mkdirSync(docsDir, { recursive: true });
    }
    await page.screenshot({ path: path.join(docsDir, "dashboard_screenshot.png") });
  });
});
