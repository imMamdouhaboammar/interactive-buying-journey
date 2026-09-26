// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import type { ComposeParams, ExperiencePlan, IBJClientOptions } from "./types.js";
import { validateExperiencePlan } from "./validator.js";
import { renderExperiencePlan } from "./renderer.js";

export class IBJClient {
  private readonly options: Required<IBJClientOptions>;

  constructor(options: IBJClientOptions) {
    if (!options.tenantKey) {
      throw new Error("tenantKey is required");
    }
    if (!options.endpoint) {
      throw new Error("endpoint is required");
    }

    this.options = {
      tenantKey: options.tenantKey,
      endpoint: options.endpoint,
      timeoutMs: options.timeoutMs ?? 450,
    };
  }

  /**
   * Calls the compose endpoint with a hard client deadline.
   * On timeout, 4xx/5xx, malformed JSON, schema invalidity, or network errors,
   * it fails open and returns null, never throwing or interrupting the host storefront.
   */
  async compose(params: ComposeParams): Promise<ExperiencePlan | null> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      controller.abort();
    }, this.options.timeoutMs);

    const payload = {
      contract_version: "1.0",
      request_id: params.requestId,
      tenant_id: this.options.tenantKey,
      session_token: params.sessionToken,
      page: {
        kind: params.page.kind,
        category_id: params.page.categoryId ?? null,
        product_id: params.page.productId ?? null,
      },
      locale: params.locale,
      currency: params.currency,
      consent: params.consent ?? {
        version: "1.0",
        purposes: ["necessary"],
      },
      preferences: {
        purpose: params.preferences?.purpose,
        max_budget_minor: params.preferences?.maxBudgetMinor,
        min_battery_hours: params.preferences?.minBatteryHours,
        max_weight_grams: params.preferences?.maxWeightGrams,
        brand_ids: params.preferences?.brandIds,
      },
      allowed_slots: params.allowedSlots,
    };

    try {
      const response = await fetch(this.options.endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Request-ID": params.requestId,
          "X-Tenant-Key": this.options.tenantKey,
        },
        body: JSON.stringify(payload),
        signal: controller.signal,
      });

      if (!response.ok) {
        // 4xx or 5xx: fail open
        return null;
      }

      let data: unknown;
      try {
        data = await response.json();
      } catch {
        // Malformed JSON: fail open
        return null;
      }

      const validation = validateExperiencePlan(data);
      if (!validation.valid || !validation.plan) {
        // Schema invalid or unknown section kind: fail open
        return null;
      }

      return validation.plan;
    } catch {
      // AbortError (timeout), network error, or unexpected fetch rejection: fail open
      return null;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Composes and applies the experience plan to the DOM.
   * If compose fails for any reason, leaves DOM completely untouched.
   */
  async apply(params: ComposeParams, rootDoc?: Document): Promise<ExperiencePlan | null> {
    const plan = await this.compose(params);
    if (!plan) {
      return null;
    }

    renderExperiencePlan(plan, rootDoc);
    return plan;
  }
}
