// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { IBJClient } from "../src/client.js";
import { renderExperiencePlan } from "../src/renderer.js";
import type { ExperiencePlan } from "../src/types.js";

describe("SDK Renderer & Interactive Tracer", () => {
  const initialHTML = `
    <div id="collection_top" data-ibj-slot="collection_top">Original Content</div>
    <div id="merchant-grid">Merchant Products</div>
  `;

  const adaptedPlan: ExperiencePlan = {
    contract_version: "1.0",
    request_id: "req_test_adapted",
    plan_id: "pl_test_01",
    tenant_id: "demo_store",
    status: "adapted",
    locale: "en",
    catalog_version: "cat_01",
    policy_version: "pol_01",
    expires_at: "2026-09-27T12:00:00Z",
    sections: [
      {
        section_id: "intent_selector",
        kind: "intent-picker",
        slot_id: "collection_top",
        priority: 10,
        items: [],
        reason_codes: ["data_available"],
        config: {
          label_key: "what_matters_most",
          options: [
            { id: "portable_work", label_key: "Portable Work" },
            { id: "performance", label_key: "Performance" },
            { id: "everyday_value", label_key: "Everyday Value" },
          ],
        },
      },
      {
        section_id: "shortlist_strip",
        kind: "product-strip",
        slot_id: "collection_top",
        priority: 20,
        items: [
          { variant_id: "lap_001", catalog_version: "cat_01" },
          { variant_id: "lap_007", catalog_version: "cat_01" },
        ],
        reason_codes: ["matches_budget", "matches_declared_portability"],
        config: {
          label_key: "recommended_laptops",
        },
      },
    ],
    provenance: {
      strategy: "deterministic",
      model_used: null,
      plugins: ["ibj.preference-ranker"],
    },
  };

  beforeEach(() => {
    document.body.innerHTML = initialHTML;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("TC-SDK-01: renders intent chips and product strip in collection_top slot", () => {
    renderExperiencePlan(adaptedPlan, document);

    const slot = document.getElementById("collection_top");
    expect(slot).not.toBeNull();
    const chips = slot!.querySelectorAll("[data-ibj-intent]");
    expect(chips.length).toBe(3);

    const strip = slot!.querySelector(".ibj-product-strip");
    expect(strip).not.toBeNull();
    const cards = strip!.querySelectorAll(".ibj-product-card");
    expect(cards.length).toBe(2);
    expect(cards[0].getAttribute("data-variant-id")).toBe("lap_001");
  });

  it("TC-SDK-03: retains focus on clicked chip after re-render", () => {
    renderExperiencePlan(adaptedPlan, document);

    const chip = document.querySelector<HTMLButtonElement>('[data-ibj-intent="portable_work"]');
    expect(chip).not.toBeNull();
    chip!.focus();
    expect(document.activeElement).toBe(chip);

    // Re-render
    renderExperiencePlan(adaptedPlan, document);
    const activeAfter = document.activeElement;
    expect(activeAfter?.getAttribute("data-ibj-intent")).toBe("portable_work");
  });

  it("TC-SDK-04: supports Arabic locale with RTL and localized text", () => {
    const arabicPlan: ExperiencePlan = {
      ...adaptedPlan,
      locale: "ar",
      sections: [
        {
          ...adaptedPlan.sections[0],
          config: {
            label_key: "ما الذي يهمك أكثر؟",
            options: [
              { id: "portable_work", label_key: "عمل متنقل" },
              { id: "performance", label_key: "أداء عالي" },
            ],
          },
        },
      ],
    };

    renderExperiencePlan(arabicPlan, document);
    const slot = document.getElementById("collection_top");
    const container = slot!.querySelector(".ibj-experience-container");
    expect(container?.getAttribute("dir")).toBe("rtl");
    expect(slot!.textContent).toContain("عمل متنقل");
  });

  it("TC-SDK-05: IBJClient aborts inflight request when new request is dispatched", async () => {
    const client = new IBJClient({
      tenantKey: "demo_store",
      endpoint: "http://localhost:8080/v1/journeys/compose",
    });

    let abortCount = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((_url, options) => {
        return new Promise((resolve, reject) => {
          options?.signal?.addEventListener("abort", () => {
            abortCount++;
            const err = new Error("aborted");
            err.name = "AbortError";
            reject(err);
          });
        });
      }),
    );

    const p1 = client.apply({
      requestId: "req_01",
      sessionToken: "sess_1",
      locale: "en",
      page: { kind: "collection", categoryId: "laptops" },
      allowedSlots: ["collection_top"],
      preferences: { purpose: "portable_work" },
    }, document);

    const p2 = client.apply({
      requestId: "req_02",
      sessionToken: "sess_1",
      locale: "en",
      page: { kind: "collection", categoryId: "laptops" },
      allowedSlots: ["collection_top"],
      preferences: { purpose: "performance" },
    }, document);

    await Promise.allSettled([p1, p2]);
    expect(abortCount).toBeGreaterThanOrEqual(1);
  });
});
