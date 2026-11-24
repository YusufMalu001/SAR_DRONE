import { test } from "@playwright/test";
import * as path from "path";
import * as fs from "fs";

test("Capture full interactive demo recording and screenshot", async ({ page }) => {
  // Mock live backend endpoints
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
          id: "survivor-p1",
          type: "survivor",
          class: "person",
          lat: 20.002,
          lon: 78.003,
          uncertainty_radius: 3.8,
          observation_count: 14,
          confidence: 0.96,
          modalities: ["rgb", "thermal"],
          priority_score: 0.92,
          status: "active",
          first_seen: new Date(Date.now() - 180000).toISOString(),
          last_seen: new Date().toISOString(),
        },
        {
          id: "hazard-f1",
          type: "hazard",
          class: "fire",
          lat: 20.004,
          lon: 78.005,
          uncertainty_radius: 50.0,
          observation_count: 22,
          confidence: 0.94,
          modalities: ["thermal"],
          priority_score: 0.81,
          status: "active",
          first_seen: new Date(Date.now() - 300000).toISOString(),
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
          id: "det-1",
          class: "person",
          confidence: 0.96,
          lat: 20.002,
          lon: 78.003,
          frame_id: "f1",
          source: "rgb",
          detected_at: new Date(Date.now() - 60000).toISOString(),
          priority_score: 0.92,
          status: "live",
          modalities: ["rgb", "thermal"],
        },
        {
          id: "det-2",
          class: "fire",
          confidence: 0.94,
          lat: 20.004,
          lon: 78.005,
          frame_id: "f2",
          source: "thermal",
          detected_at: new Date(Date.now() - 30000).toISOString(),
          priority_score: 0.81,
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
          id: 301,
          detection_id: "det-1",
          priority_tier: "critical",
          message: "Critical: Survivor located inside flood zone perimeter",
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
        state: "SURVIVOR_CONFIRMATION",
        target_waypoints_count: 26,
        current_waypoint_idx: 12,
        armed: true,
        battery_pct: 82.0,
      }),
    });
  });

  await page.route("**/tiles/*", async (route) => {
    const transparentPng = Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
      "base64"
    );
    await route.fulfill({ status: 200, contentType: "image/png", body: transparentPng });
  });

  await page.goto("/");
  await page.waitForTimeout(500);

  // 1. Switch to Thermal Tab
  await page.getByRole("tab", { name: "LWIR Thermal" }).click();
  await page.waitForTimeout(400);

  // 2. Select Survivor
  await page.getByText("Score: 0.92").click();
  await page.waitForTimeout(500);

  // 3. Switch to Replay Mode and scrub
  await page.getByText("Replay Mode").click();
  await page.waitForTimeout(400);

  // Take screenshot
  const docsDir = path.resolve(process.cwd(), "../docs");
  if (!fs.existsSync(docsDir)) {
    fs.mkdirSync(docsDir, { recursive: true });
  }
  const screenshotPath = path.join(docsDir, "dashboard_screenshot.png");
  await page.screenshot({ path: screenshotPath });

  // Get recorded video and copy to docs/demo_recording.webm
  const video = page.video();
  if (video) {
    const videoPath = await video.path();
    const destPath = path.join(docsDir, "demo_recording.webm");
    // Ensure video is written on close
    await page.close();
    if (fs.existsSync(videoPath)) {
      fs.copyFileSync(videoPath, destPath);
    }
  }
});
