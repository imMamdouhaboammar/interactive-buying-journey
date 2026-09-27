// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SDK_PATH = path.resolve(__dirname, "../public/sdk.js");

interface ProductFixture {
  id: string;
  title_en: string;
  title_ar: string;
  price_usd: number;
  in_stock: boolean;
  weight_g: number | null;
  battery_wh: number | null;
  usb_c_pd: boolean | null;
}

const PRODUCTS: ProductFixture[] = [
  {
    id: "lap_001",
    title_en: "Light 13 Demo",
    title_ar: "حاسوب محمول خفيف 13 تجريبي",
    price_usd: 1099,
    in_stock: true,
    weight_g: 1270,
    battery_wh: 58,
    usb_c_pd: true,
  },
  {
    id: "lap_002",
    title_en: "Travel 14 Demo",
    title_ar: "حاسوب محمول للسفر 14 تجريبي",
    price_usd: 1249,
    in_stock: true,
    weight_g: 1450,
    battery_wh: 65,
    usb_c_pd: true,
  },
  {
    id: "lap_003",
    title_en: "Heavy 17 Demo",
    title_ar: "حاسوب محمول مكتبي 17 تجريبي",
    price_usd: 1699,
    in_stock: true,
    weight_g: 2650,
    battery_wh: 83,
    usb_c_pd: false,
  },
  {
    id: "lap_004",
    title_en: "Everyday 15 Demo",
    title_ar: "حاسوب محمول للاستخدام اليومي 15 تجريبي",
    price_usd: 799,
    in_stock: true,
    weight_g: 1840,
    battery_wh: 45,
    usb_c_pd: true,
  },
  {
    id: "lap_005",
    title_en: "Out of stock Light",
    title_ar: "حاسوب خفيف (نفد من المخزون)",
    price_usd: 899,
    in_stock: false,
    weight_g: 1200,
    battery_wh: 55,
    usb_c_pd: true,
  },
  {
    id: "lap_006",
    title_en: "Unknown Weight Demo",
    title_ar: "حاسوب بمواصفات غير محددة تجريبي",
    price_usd: 999,
    in_stock: true,
    weight_g: null,
    battery_wh: null,
    usb_c_pd: null,
  },
  {
    id: "lap_007",
    title_en: "Ultra Light Arabic 14 Demo",
    title_ar: "حاسوب محمول فائق الخفة 14",
    price_usd: 1150,
    in_stock: true,
    weight_g: 1190,
    battery_wh: 60,
    usb_c_pd: true,
  },
];

function renderPage(locale: "en" | "ar", ibjEndpoint: string): string {
  const isAr = locale === "ar";
  const dir = isAr ? "rtl" : "ltr";
  const title = isAr ? "متجر الحواسيب المحمولة التجريبي" : "Demo Laptop Storefront";
  const switchLabel = isAr ? "English" : "العربية";
  const switchHref = isAr ? "/en" : "/ar";
  const unknownLabel = isAr ? "غير محدد" : "Unknown";
  const stockLabel = (inStock: boolean) =>
    inStock ? (isAr ? "متوفر" : "In Stock") : (isAr ? "نفد من المخزون" : "Out of Stock");

  const productCards = PRODUCTS.map((p) => {
    const pTitle = isAr ? p.title_ar : p.title_en;
    const weightStr = p.weight_g !== null ? `${p.weight_g.toLocaleString()} g` : unknownLabel;
    const batteryStr = p.battery_wh !== null ? `${p.battery_wh} Wh` : unknownLabel;
    const usbStr =
      p.usb_c_pd !== null
        ? p.usb_c_pd
          ? isAr
            ? "نعم"
            : "Yes"
          : isAr
            ? "لا"
            : "No"
        : unknownLabel;

    return `
      <article class="product-card" data-product-id="${p.id}">
        <h3>${pTitle}</h3>
        <p class="price">$${p.price_usd.toLocaleString()}.00</p>
        <p class="status ${p.in_stock ? "in-stock" : "out-of-stock"}">${stockLabel(p.in_stock)}</p>
        <ul class="specs">
          <li><strong>${isAr ? "الوزن:" : "Weight:"}</strong> <span>${weightStr}</span></li>
          <li><strong>${isAr ? "البطارية:" : "Battery:"}</strong> <span>${batteryStr}</span></li>
          <li><strong>${isAr ? "شحن USB-C:" : "USB-C PD:"}</strong> <span>${usbStr}</span></li>
        </ul>
        <button type="button" class="btn-buy" ${!p.in_stock ? "disabled" : ""}>
          ${isAr ? "إضافة إلى السلة" : "Add to Cart"}
        </button>
      </article>
    `;
  }).join("\n");

  return `<!DOCTYPE html>
<html lang="${locale}" dir="${dir}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${title}</title>
  <style>
    :root {
      --bg-color: #f8fafc;
      --card-bg: #ffffff;
      --text-main: #0f172a;
      --text-muted: #475569;
      --primary: #2563eb;
      --border-color: #e2e8f0;
      --focus-ring: #3b82f6;
    }
    body {
      margin: 0;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background-color: var(--bg-color);
      color: var(--text-main);
      line-height: 1.5;
    }
    header {
      background: var(--card-bg);
      border-bottom: 1px solid var(--border-color);
      padding: 1rem 2rem;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    header h1 {
      margin: 0;
      font-size: 1.25rem;
    }
    nav a {
      color: var(--primary);
      text-decoration: none;
      font-weight: 600;
      padding: 0.5rem 1rem;
      border: 1px solid var(--border-color);
      border-radius: 0.375rem;
      min-height: 44px;
      display: inline-flex;
      align-items: center;
    }
    nav a:focus, button:focus {
      outline: 3px solid var(--focus-ring);
      outline-offset: 2px;
    }
    main {
      max-width: 1200px;
      margin: 2rem auto;
      padding: 0 1rem;
    }
    #collection_top {
      margin-bottom: 1.5rem;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 1.5rem;
    }
    .product-card {
      background: var(--card-bg);
      border: 1px solid var(--border-color);
      border-radius: 0.5rem;
      padding: 1.5rem;
      display: flex;
      flex-direction: column;
      box-shadow: 0 1px 3px rgba(0,0,0,0.05);
    }
    .product-card h3 {
      margin-top: 0;
      font-size: 1.15rem;
      margin-bottom: 0.5rem;
      color: var(--text-main);
    }
    .section-title {
      font-size: 1.25rem;
      font-weight: 700;
      margin: 1.5rem 0 1rem 0;
      color: var(--text-main);
    }
    .price {
      font-size: 1.25rem;
      font-weight: 700;
      color: var(--primary);
      margin: 0 0 0.5rem 0;
    }
    .status {
      font-weight: 600;
      font-size: 0.875rem;
      margin: 0 0 1rem 0;
    }
    .status.in-stock { color: #166534; }
    .status.out-of-stock { color: #991b1b; }
    .specs {
      list-style: none;
      padding: 0;
      margin: 0 0 1.5rem 0;
      font-size: 0.9rem;
      color: var(--text-muted);
      flex-grow: 1;
    }
    .specs li {
      margin-bottom: 0.35rem;
    }
    .btn-buy {
      background-color: var(--primary);
      color: #ffffff;
      border: none;
      padding: 0.625rem 1rem;
      border-radius: 0.375rem;
      font-size: 0.95rem;
      font-weight: 600;
      cursor: pointer;
      min-height: 44px;
    }
    .btn-buy:disabled {
      background-color: #94a3b8;
      cursor: not-allowed;
    }

    /* Discovery Card & Intent Picker */
    .intent-card, .ibj-intent-picker {
      background: var(--card-bg);
      border: 1px solid var(--border-color);
      border-radius: 0.75rem;
      padding: 1.25rem 1.5rem;
      margin-bottom: 1.5rem;
      box-shadow: 0 1px 3px rgba(0,0,0,0.05);
    }
    .ibj-intent-header {
      margin-bottom: 0.875rem;
    }
    .ibj-intent-title {
      margin: 0;
      font-size: 1.15rem;
      font-weight: 700;
      color: var(--text-main);
    }
    .ibj-chip-list {
      display: flex;
      flex-wrap: wrap;
      gap: 0.5rem;
      margin-bottom: 1rem;
    }
    .chip-btn, .ibj-chip, .ibj-budget-chip, .ibj-reset-button {
      background: #f1f5f9;
      color: #0f172a;
      border: 1px solid #94a3b8;
      border-radius: 9999px;
      padding: 0.5rem 1rem;
      font-size: 0.875rem;
      font-weight: 600;
      cursor: pointer;
      min-height: 44px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: background 0.15s ease, border-color 0.15s ease;
    }
    .chip-btn:hover, .ibj-chip:hover, .ibj-budget-chip:hover {
      background: #e2e8f0;
      border-color: #64748b;
    }
    .chip-btn:focus, .ibj-chip:focus, .ibj-budget-chip:focus, .ibj-reset-button:focus {
      outline: 3px solid var(--focus-ring);
      outline-offset: 2px;
    }
    .ibj-budget-group {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 0.5rem;
      padding-top: 0.75rem;
      border-top: 1px solid var(--border-color);
    }
    .ibj-budget-label {
      font-size: 0.875rem;
      font-weight: 600;
      color: var(--text-muted);
      margin-inline-end: 0.5rem;
    }
    .ibj-reset-button {
      background: #fee2e2;
      color: #991b1b;
      border: 1px solid #f87171;
      border-radius: 0.375rem;
    }
    .ibj-reset-button:hover {
      background: #fecaca;
    }

    /* Product Strip (Adapted Section) */
    .ibj-product-strip {
      background: #f0fdf4;
      border: 2px solid #86efac;
      border-radius: 0.75rem;
      padding: 1.25rem 1.5rem;
      margin-top: 1rem;
    }
    .ibj-strip-header {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 1rem;
    }
    .ibj-strip-title {
      margin: 0;
      font-size: 1.15rem;
      font-weight: 700;
      color: #14532d;
    }
    .ibj-badges {
      display: flex;
      flex-wrap: wrap;
      gap: 0.375rem;
    }
    .ibj-badge {
      background: #dcfce7;
      color: #15803d;
      border: 1px solid #86efac;
      border-radius: 9999px;
      padding: 0.25rem 0.625rem;
      font-size: 0.8rem;
      font-weight: 700;
    }
    .ibj-cards-container {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
      gap: 1rem;
    }
    .ibj-product-card {
      background: #ffffff;
      border: 1px solid #bbf7d0;
      border-radius: 0.5rem;
      padding: 1rem;
      display: flex;
      flex-direction: column;
      box-shadow: 0 1px 2px rgba(0,0,0,0.05);
    }
    .ibj-product-card h4 {
      margin: 0 0 0.5rem 0;
      font-size: 1rem;
      color: var(--text-main);
    }
    .ibj-card-price {
      font-size: 1.1rem;
      font-weight: 700;
      color: var(--primary);
      margin: 0 0 0.5rem 0;
    }
    .ibj-match-tag {
      display: inline-block;
      align-self: flex-start;
      background: #f0fdf4;
      color: #166534;
      font-size: 0.75rem;
      font-weight: 600;
      padding: 0.15rem 0.5rem;
      border-radius: 0.25rem;
      border: 1px solid #86efac;
    }

    /* Empty State */
    .ibj-empty-state {
      background: #fef2f2;
      border: 1px dashed #f87171;
      border-radius: 0.75rem;
      padding: 1.5rem;
      text-align: center;
      margin-top: 1rem;
    }
    .ibj-empty-message {
      margin: 0 0 1rem 0;
      font-size: 0.95rem;
      font-weight: 600;
      color: #991b1b;
    }

    @media (max-width: 640px) {
      header {
        padding: 0.75rem 1rem;
      }
      main {
        margin: 1rem auto;
        padding: 0 0.75rem;
      }
      .intent-card, .ibj-intent-picker, .ibj-product-strip {
        padding: 1rem;
      }
      .grid {
        grid-template-columns: 1fr;
      }
      .ibj-cards-container {
        grid-template-columns: 1fr;
      }
    }
  </style>
</head>
<body>
  <header>
    <h1>${title}</h1>
    <nav aria-label="${isAr ? "التنقل في الموقع" : "Site navigation"}">
      <a href="${switchHref}">${switchLabel}</a>
    </nav>
  </header>

  <main>
    <div id="collection_top" data-ibj-slot="collection_top">
      <div class="ibj-intent-picker intent-card">
        <div class="ibj-intent-header">
          <h2 class="ibj-intent-title">${isAr ? "ما الذي يهمك أكثر؟" : "What matters most?"}</h2>
        </div>
        <div class="ibj-chip-list" role="group" aria-label="${isAr ? "ما الذي يهمك أكثر؟" : "What matters most?"}">
          <button type="button" class="ibj-chip chip-btn" data-ibj-intent="portable_work" aria-label="${isAr ? "عمل متنقل" : "Portable Work"}">
            ${isAr ? "عمل متنقل" : "Portable Work"}
          </button>
          <button type="button" class="ibj-chip chip-btn" data-ibj-intent="performance" aria-label="${isAr ? "أداء عالي" : "Performance"}">
            ${isAr ? "أداء عالي" : "Performance"}
          </button>
          <button type="button" class="ibj-chip chip-btn" data-ibj-intent="everyday_value" aria-label="${isAr ? "استخدام يومي اقتصادي" : "Everyday Value"}">
            ${isAr ? "استخدام يومي اقتصادي" : "Everyday Value"}
          </button>
        </div>
        <div class="ibj-budget-group">
          <span class="ibj-budget-label">${isAr ? "الميزانية:" : "Budget:"}</span>
          <button type="button" class="ibj-budget-chip chip-btn" data-ibj-budget="50000" aria-label="${isAr ? "حتى $500" : "Up to $500"}">
            ${isAr ? "حتى $500" : "Up to $500"}
          </button>
          <button type="button" class="ibj-budget-chip chip-btn" data-ibj-budget="100000" aria-label="${isAr ? "حتى $1,000" : "Up to $1,000"}">
            ${isAr ? "حتى $1,000" : "Up to $1,000"}
          </button>
          <button type="button" class="ibj-budget-chip chip-btn" data-ibj-budget="120000" aria-label="${isAr ? "حتى $1,200" : "Up to $1,200"}">
            ${isAr ? "حتى $1,200" : "Up to $1,200"}
          </button>
          <button type="button" class="ibj-budget-chip chip-btn" data-ibj-budget="150000" aria-label="${isAr ? "حتى $1,500" : "Up to $1,500"}">
            ${isAr ? "حتى $1,500" : "Up to $1,500"}
          </button>
          <button type="button" class="ibj-reset-button chip-btn" data-ibj-action="reset" aria-label="${isAr ? "إعادة الضبط" : "Reset preferences"}">
            ${isAr ? "إعادة الضبط" : "Reset"}
          </button>
        </div>
      </div>
    </div>
    <h2 class="section-title">${isAr ? "جميع الحواسيب المحمولة" : "All Laptops"}</h2>
    <section class="grid" aria-label="${isAr ? "قائمة المنتجات" : "Product listing"}">
      ${productCards}
    </section>
  </main>

  <script type="module">
    import { IBJClient } from "/sdk.js";
    const endpoint = "${ibjEndpoint}";
    try {
      const client = new IBJClient({
        tenantKey: "demo_store",
        endpoint: endpoint,
        timeoutMs: 350,
      });
      window.__ibjClient = client;

      let currentPreferences = {};

      const applyPrefs = async (prefs) => {
        currentPreferences = prefs;
        await client.apply({
          requestId: "req_" + Math.random().toString(36).substring(2, 10),
          sessionToken: "sess_demo_storefront",
          locale: "${locale}",
          currency: "USD",
          page: {
            kind: "collection",
            categoryId: "laptops"
          },
          preferences: currentPreferences,
          allowedSlots: ["collection_top"]
        });
      };

      const topSlot = document.getElementById("collection_top");
      if (topSlot) {
        topSlot.addEventListener("click", (e) => {
          const btn = e.target.closest("button");
          if (!btn) return;
          if (btn.closest(".ibj-experience-container")) return;

          const intent = btn.getAttribute("data-ibj-intent");
          if (intent) {
            applyPrefs({ ...currentPreferences, purpose: intent });
            return;
          }

          const budget = btn.getAttribute("data-ibj-budget");
          if (budget) {
            applyPrefs({ ...currentPreferences, maxBudgetMinor: parseInt(budget, 10) });
            return;
          }

          const action = btn.getAttribute("data-ibj-action");
          if (action === "reset") {
            applyPrefs({});
            return;
          }
        });
      }
    } catch (err) {
      // SDK guarantees fail-open: never block or throw to host page
    }
  </script>
</body>
</html>`;
}

export function createStorefrontServer(defaultIbjEndpoint: string = "http://127.0.0.1:8080/v1/journeys/compose") {
  return http.createServer((req, res) => {
    const url = new URL(req.url || "/", "http://localhost");

    if (url.pathname === "/sdk.js") {
      if (fs.existsSync(SDK_PATH)) {
        res.writeHead(200, { "Content-Type": "application/javascript" });
        fs.createReadStream(SDK_PATH).pipe(res);
      } else {
        res.writeHead(404, { "Content-Type": "text/plain" });
        res.end("SDK bundle not built yet");
      }
      return;
    }

    if (url.pathname === "/en") {
      const endpoint = url.searchParams.get("ibj_endpoint") || defaultIbjEndpoint;
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(renderPage("en", endpoint));
      return;
    }

    if (url.pathname === "/ar") {
      const endpoint = url.searchParams.get("ibj_endpoint") || defaultIbjEndpoint;
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(renderPage("ar", endpoint));
      return;
    }

    // Default redirect to /en
    res.writeHead(302, { Location: "/en" });
    res.end();
  });
}

// Start standalone if executed directly
if (process.argv[1] && process.argv[1].endsWith("server.ts")) {
  const port = parseInt(process.env.STOREFRONT_PORT || "3000", 10);
  const server = createStorefrontServer();
  server.listen(port, () => {
    console.log(`Demo storefront server running at http://localhost:${port}`);
  });
}
