// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import { test, expect } from "@playwright/test";
import AxeBuilderPkg from "@axe-core/playwright";
import http from "node:http";
import { spawn, type ChildProcess } from "node:child_process";
import path from "node:path";
import fs from "node:fs";
import { fileURLToPath } from "node:url";
import { createStorefrontServer } from "../src/server.js";

const AxeBuilder = ((AxeBuilderPkg as any).default || AxeBuilderPkg) as any;

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const REPO_ROOT = path.resolve(__dirname, "../..");
const SCREENSHOTS_DIR = path.resolve(__dirname, "../screenshots");

let apiProcess: ChildProcess | null = null;
let storefrontServer: http.Server | null = null;
let slowServer: http.Server | null = null;

const API_PORT = 8089;
const STOREFRONT_PORT = 3009;
const SLOW_PORT = 8099;

test.beforeAll(async () => {
  fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });

  // 1. Build SDK bundle if needed
  const sdkPath = path.resolve(REPO_ROOT, "demo-storefront/public/sdk.js");
  if (!fs.existsSync(sdkPath)) {
    const { execSync } = await import("node:child_process");
    execSync(`bun build sdk/src/index.ts --outfile demo-storefront/public/sdk.js --target browser`, {
      cwd: REPO_ROOT,
      stdio: "inherit",
    });
  }

  // 2. Start Go API server
  apiProcess = spawn("go", ["run", "./cmd/ibj-api"], {
    cwd: REPO_ROOT,
    env: {
      ...process.env,
      PORT: String(API_PORT),
      SCHEMAS_DIR: path.resolve(REPO_ROOT, "contracts/schemas"),
      ALLOWED_TENANTS: "demo_store",
      ALLOWED_SLOTS: "collection_top,collection_grid,collection_comparison,collection_filters,pdp_related,cart_accessory",
    },
    stdio: "pipe",
  });

  // Wait for API to be ready
  let apiReady = false;
  for (let i = 0; i < 30; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${API_PORT}/readyz`);
      if (res.ok) {
        apiReady = true;
        break;
      }
    } catch {
      await new Promise((r) => setTimeout(r, 200));
    }
  }
  if (!apiReady) {
    throw new Error("Go API server failed to become ready in time");
  }

  // 3. Start Slow Mock Server (simulates timeout by delaying response 1000ms > 350ms deadline)
  slowServer = http.createServer((_req, res) => {
    setTimeout(() => {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "baseline" }));
    }, 1000);
  });
  await new Promise<void>((resolve) => slowServer!.listen(SLOW_PORT, resolve));

  // 4. Start Storefront Server
  storefrontServer = createStorefrontServer(`http://127.0.0.1:${API_PORT}/v1/journeys/compose`);
  await new Promise<void>((resolve) => storefrontServer!.listen(STOREFRONT_PORT, resolve));
});

test.afterAll(async () => {
  if (storefrontServer) {
    await new Promise<void>((resolve) => storefrontServer!.close(() => resolve()));
  }
  if (slowServer) {
    await new Promise<void>((resolve) => slowServer!.close(() => resolve()));
  }
  if (apiProcess && apiProcess.pid) {
    apiProcess.kill("SIGTERM");
  }
});

test.describe("Interactive Buying Journey Storefront Baseline", () => {
  test("English page with engine running", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" && !msg.text().includes("net::ERR_CONNECTION_REFUSED")) {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto(`http://127.0.0.1:${STOREFRONT_PORT}/en`);

    // Verify HTML dir and lang
    const html = page.locator("html");
    await expect(html).toHaveAttribute("dir", "ltr");
    await expect(html).toHaveAttribute("lang", "en");

    // Verify product listing is visible
    const productCards = page.locator(".product-card");
    await expect(productCards).toHaveCount(7);

    // Verify missing attribute rendered as Unknown
    const lap006 = page.locator('.product-card[data-product-id="lap_006"]');
    await expect(lap006).toContainText("Unknown");

    // Accessibility check
    const accessibilityScanResults = await new AxeBuilder({ page }).analyze();
    expect(accessibilityScanResults.violations).toEqual([]);

    // Check no console errors
    expect(consoleErrors).toEqual([]);

    // Take screenshot
    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, "storefront-en-engine-on.png") });
  });

  test("Arabic page with engine running", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" && !msg.text().includes("net::ERR_CONNECTION_REFUSED")) {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto(`http://127.0.0.1:${STOREFRONT_PORT}/ar`);

    // Verify HTML dir and lang
    const html = page.locator("html");
    await expect(html).toHaveAttribute("dir", "rtl");
    await expect(html).toHaveAttribute("lang", "ar");

    // Verify product cards count
    const productCards = page.locator(".product-card");
    await expect(productCards).toHaveCount(7);

    // Verify Arabic product title rendered
    const lap007 = page.locator('.product-card[data-product-id="lap_007"]');
    await expect(lap007).toContainText("حاسوب محمول فائق الخفة 14");

    // Verify missing attribute rendered as "غير محدد" (never invented)
    const lap006 = page.locator('.product-card[data-product-id="lap_006"]');
    await expect(lap006).toContainText("غير محدد");

    // Accessibility check
    const accessibilityScanResults = await new AxeBuilder({ page }).analyze();
    expect(accessibilityScanResults.violations).toEqual([]);

    // Check no console errors
    expect(consoleErrors).toEqual([]);

    // Take screenshot
    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, "storefront-ar-engine-on.png") });
  });

  test("English page with engine stopped (outage)", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" && !msg.text().includes("net::ERR_CONNECTION_REFUSED")) {
        consoleErrors.push(msg.text());
      }
    });

    // Pass invalid offline port for engine
    await page.goto(`http://127.0.0.1:${STOREFRONT_PORT}/en?ibj_endpoint=http://127.0.0.1:9999/v1/journeys/compose`);

    // Product list remains fully visible and navigable
    const productCards = page.locator(".product-card");
    await expect(productCards).toHaveCount(7);
    await expect(page.locator("h1")).toHaveText("Demo Laptop Storefront");

    // No uncaught console errors
    expect(consoleErrors).toEqual([]);

    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, "storefront-en-engine-stopped.png") });
  });

  test("Arabic page with engine stopped (outage)", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" && !msg.text().includes("net::ERR_CONNECTION_REFUSED")) {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto(`http://127.0.0.1:${STOREFRONT_PORT}/ar?ibj_endpoint=http://127.0.0.1:9999/v1/journeys/compose`);

    const productCards = page.locator(".product-card");
    await expect(productCards).toHaveCount(7);
    await expect(page.locator("h1")).toHaveText("متجر الحواسيب المحمولة التجريبي");

    expect(consoleErrors).toEqual([]);

    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, "storefront-ar-engine-stopped.png") });
  });

  test("English page with engine slower than deadline (timeout)", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error") consoleErrors.push(msg.text());
    });

    // Point to slow server that takes 1000ms (client deadline is 350ms)
    await page.goto(`http://127.0.0.1:${STOREFRONT_PORT}/en?ibj_endpoint=http://127.0.0.1:${SLOW_PORT}/v1/journeys/compose`);

    // Wait past deadline to ensure timeout completed cleanly
    await page.waitForTimeout(500);

    const productCards = page.locator(".product-card");
    await expect(productCards).toHaveCount(7);
    await expect(page.locator("h1")).toHaveText("Demo Laptop Storefront");

    expect(consoleErrors).toEqual([]);

    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, "storefront-en-engine-timeout.png") });
  });

  test("Arabic page with engine slower than deadline (timeout)", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error") consoleErrors.push(msg.text());
    });

    await page.goto(`http://127.0.0.1:${STOREFRONT_PORT}/ar?ibj_endpoint=http://127.0.0.1:${SLOW_PORT}/v1/journeys/compose`);

    await page.waitForTimeout(500);

    const productCards = page.locator(".product-card");
    await expect(productCards).toHaveCount(7);
    await expect(page.locator("h1")).toHaveText("متجر الحواسيب المحمولة التجريبي");

    expect(consoleErrors).toEqual([]);

    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, "storefront-ar-engine-timeout.png") });
  });
});
