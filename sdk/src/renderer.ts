// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import type { ExperiencePlan } from "./types.js";

export function renderExperiencePlan(plan: ExperiencePlan, rootDoc: Document = document): void {
  // If plan is baseline or has no sections, leave DOM completely untouched
  if (plan.status === "baseline" || plan.sections.length === 0) {
    return;
  }

  for (const section of plan.sections) {
    // Find target slot element by data-ibj-slot or id
    const slotEl =
      rootDoc.querySelector(`[data-ibj-slot="${section.slot_id}"]`) ||
      rootDoc.getElementById(section.slot_id);

    if (!slotEl) {
      continue;
    }

    // Baseline-safe rendering: only render known components into approved slots
    // In Slice 1, sections are baseline empty. In later slices, components mount here.
  }
}
