// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { IBJClient } from "../src/client.js";
import type { ComposeParams } from "../src/types.js";

describe("IBJClient fail-open guarantees", () => {
  const baseParams: ComposeParams = {
    requestId: "req_test_001",
    sessionToken: "sess_12345",
    locale: "en",
    page: {
      kind: "collection",
      categoryId: "laptops",
    },
    allowedSlots: ["collection_top"],
  };

  const initialHTML = `
    <div id="merchant-header">Header</div>
    <div id="collection_top" data-ibj-slot="collection_top">Original Baseline Content</div>
    <div id="product-grid">
      <div class="product-card">Laptop 1</div>
    </div>
  `;

  let client: IBJClient;

  beforeEach(() => {
    document.body.innerHTML = initialHTML;
    client = new IBJClient({
      tenantKey: "demo_store",
      endpoint: "http://localhost:8080/v1/journeys/compose",
      timeoutMs: 50, // Short timeout for test speed
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("leaves DOM unchanged on timeout", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((_url, options) => {
        return new Promise((_, reject) => {
          options?.signal?.addEventListener("abort", () => {
            const err = new Error("This operation was aborted");
            err.name = "AbortError";
            reject(err);
          });
        });
      }),
    );

    const plan = await client.apply(baseParams, document);
    expect(plan).toBeNull();
    expect(document.body.innerHTML).toBe(initialHTML);
  });

  it("leaves DOM unchanged on 500 internal server error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        statusText: "Internal Server Error",
        json: async () => ({ error: { code: "INTERNAL_ERROR" } }),
      }),
    );

    const plan = await client.apply(baseParams, document);
    expect(plan).toBeNull();
    expect(document.body.innerHTML).toBe(initialHTML);
  });

  it("leaves DOM unchanged on malformed JSON response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => {
          throw new SyntaxError("Unexpected token < in JSON at position 0");
        },
      }),
    );

    const plan = await client.apply(baseParams, document);
    expect(plan).toBeNull();
    expect(document.body.innerHTML).toBe(initialHTML);
  });

  it("leaves DOM unchanged on schema-invalid experience plan", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          contract_version: "1.0",
          // missing required fields: request_id, plan_id, tenant_id, sections, etc.
          status: "baseline",
        }),
      }),
    );

    const plan = await client.apply(baseParams, document);
    expect(plan).toBeNull();
    expect(document.body.innerHTML).toBe(initialHTML);
  });

  it("leaves DOM unchanged on plan with unknown section kind", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          contract_version: "1.0",
          request_id: "req_test_001",
          plan_id: "pl_test_001",
          tenant_id: "demo_store",
          status: "adapted",
          locale: "en",
          catalog_version: "cat_01",
          policy_version: "pol_01",
          expires_at: "2026-09-26T12:05:00Z",
          sections: [
            {
              section_id: "sec_unknown_01",
              kind: "unauthorized_promo_takeover",
              slot_id: "collection_top",
              priority: 10,
              items: [],
              reason_codes: ["matches_budget"],
            },
          ],
          provenance: {
            strategy: "merchant_baseline",
            model_used: null,
            plugins: [],
          },
        }),
      }),
    );

    const plan = await client.apply(baseParams, document);
    expect(plan).toBeNull();
    expect(document.body.innerHTML).toBe(initialHTML);
  });

  it("leaves DOM unchanged on valid baseline plan", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          contract_version: "1.0",
          request_id: "req_test_001",
          plan_id: "pl_test_001",
          tenant_id: "demo_store",
          status: "baseline",
          locale: "en",
          catalog_version: "cat_01",
          policy_version: "pol_01",
          expires_at: "2026-09-26T12:05:00Z",
          sections: [],
          provenance: {
            strategy: "merchant_baseline",
            model_used: null,
            plugins: [],
          },
        }),
      }),
    );

    const plan = await client.apply(baseParams, document);
    expect(plan).not.toBeNull();
    expect(plan?.status).toBe("baseline");
    expect(document.body.innerHTML).toBe(initialHTML);
  });
});
