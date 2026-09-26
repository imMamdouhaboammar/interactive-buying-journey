// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import Ajv2020Class from "ajv/dist/2020.js";
import addFormatsFn from "ajv-formats";
import type { ExperiencePlan } from "./types.js";
import planSchema from "../../contracts/schemas/experience-plan.schema.json" with { type: "json" };

// Support both ESM default import and CJS interop
const Ajv = (Ajv2020Class as unknown as { default: typeof Ajv2020Class }).default || Ajv2020Class;
const addFormats = (addFormatsFn as unknown as { default: typeof addFormatsFn }).default || addFormatsFn;

const ajv = new (Ajv as any)({
  allErrors: true,
  strict: false,
});
(addFormats as any)(ajv);

const compiledValidator = ajv.compile(planSchema);

const ALLOWED_SECTION_KINDS = new Set([
  "intent-picker",
  "product-strip",
  "comparison-table",
  "accessory-strip",
  "explanation",
  "empty-state",
  "facet-panel",
]);

export function validateExperiencePlan(data: unknown): { valid: boolean; plan?: ExperiencePlan; error?: string } {
  if (typeof data !== "object" || data === null) {
    return { valid: false, error: "Plan payload must be an object" };
  }

  const isValidSchema = compiledValidator(data);
  if (!isValidSchema) {
    const errors = compiledValidator.errors || [];
    const errorText = errors
      .map((e: { instancePath?: string; message?: string }) => `${e.instancePath || "/"} ${e.message || "invalid"}`)
      .join("; ") || "Schema validation error";
    return { valid: false, error: errorText };
  }

  const plan = data as ExperiencePlan;

  // Strict check on section kinds
  for (const sec of plan.sections) {
    if (!ALLOWED_SECTION_KINDS.has(sec.kind)) {
      return { valid: false, error: `Unknown section kind: ${sec.kind}` };
    }
  }

  return { valid: true, plan };
}
