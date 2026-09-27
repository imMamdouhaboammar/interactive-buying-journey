// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

import type { ExperiencePlan, Locale, PlanSection } from "./types.js";

export interface RenderCallbacks {
  onSelectIntent?: (intentId: string) => void;
  onResetPreferences?: () => void;
  onSelectBudget?: (budgetMinor: number) => void;
}

export function renderExperiencePlan(
  plan: ExperiencePlan,
  rootDoc: Document = document,
  callbacks?: RenderCallbacks,
): void {
  // If plan is baseline or has no sections, leave DOM completely untouched
  if (plan.status === "baseline" || plan.sections.length === 0) {
    return;
  }

  // Focus preservation: capture focused element attributes before re-rendering
  const activeIntent = rootDoc.activeElement?.getAttribute("data-ibj-intent");
  const activeAction = rootDoc.activeElement?.getAttribute("data-ibj-action");
  const activeBudget = rootDoc.activeElement?.getAttribute("data-ibj-budget");

  // Group sections by target slot_id
  const slotSections = new Map<string, PlanSection[]>();
  for (const section of plan.sections) {
    const list = slotSections.get(section.slot_id) || [];
    list.push(section);
    slotSections.set(section.slot_id, list);
  }

  for (const [slotId, sections] of slotSections.entries()) {
    const slotEl =
      rootDoc.querySelector(`[data-ibj-slot="${slotId}"]`) ||
      rootDoc.getElementById(slotId);

    if (!slotEl) {
      continue;
    }

    // Clear previous slot contents
    while (slotEl.firstChild) {
      slotEl.removeChild(slotEl.firstChild);
    }

    const container = rootDoc.createElement("div");
    container.className = "ibj-experience-container";
    container.setAttribute("dir", plan.locale === "ar" ? "rtl" : "ltr");
    container.setAttribute("lang", plan.locale);

    for (const section of sections) {
      if (section.kind === "intent-picker") {
        const pickerEl = renderIntentPicker(section, plan.locale, rootDoc, callbacks);
        container.appendChild(pickerEl);
      } else if (section.kind === "product-strip") {
        const stripEl = renderProductStrip(section, plan.locale, rootDoc);
        container.appendChild(stripEl);
      } else if (section.kind === "empty-state") {
        const emptyEl = renderEmptyState(section, plan.locale, rootDoc, callbacks);
        container.appendChild(emptyEl);
      }
    }

    slotEl.appendChild(container);

    // Focus restoration: restore focus if active element was within this slot
    if (activeIntent) {
      const el = slotEl.querySelector<HTMLElement>(`[data-ibj-intent="${activeIntent}"]`);
      el?.focus();
    } else if (activeAction) {
      const el = slotEl.querySelector<HTMLElement>(`[data-ibj-action="${activeAction}"]`);
      el?.focus();
    } else if (activeBudget) {
      const el = slotEl.querySelector<HTMLElement>(`[data-ibj-budget="${activeBudget}"]`);
      el?.focus();
    }
  }
}

function renderIntentPicker(
  section: PlanSection,
  locale: Locale,
  rootDoc: Document,
  callbacks?: RenderCallbacks,
): HTMLElement {
  const isAr = locale === "ar";
  const wrapper = rootDoc.createElement("div");
  wrapper.className = "ibj-intent-picker";
  wrapper.setAttribute("role", "region");
  wrapper.setAttribute("aria-label", isAr ? "اختيار تفضيلات الحاسوب" : "Laptop Preferences");

  // Header / Title
  const header = rootDoc.createElement("div");
  header.className = "ibj-intent-header";
  const title = rootDoc.createElement("h3");
  title.className = "ibj-intent-title";
  const labelKey =
    (section.config?.label_key as string) ||
    (isAr ? "ما الذي يهمك أكثر؟" : "What matters most?");
  title.textContent = labelKey;
  header.appendChild(title);
  wrapper.appendChild(header);

  // Chip options list
  const chipList = rootDoc.createElement("div");
  chipList.className = "ibj-chip-list";
  chipList.setAttribute("role", "group");
  chipList.setAttribute("aria-label", labelKey);

  const rawOptions = (section.config?.options as Array<{ id: string; label_key: string }>) || [];
  const options =
    rawOptions.length > 0
      ? rawOptions
      : [
          { id: "portable_work", label_key: isAr ? "عمل متنقل" : "Portable Work" },
          { id: "performance", label_key: isAr ? "أداء عالي" : "Performance" },
          { id: "everyday_value", label_key: isAr ? "استخدام يومي اقتصادي" : "Everyday Value" },
        ];

  for (const opt of options) {
    const btn = rootDoc.createElement("button");
    btn.type = "button";
    btn.className = "ibj-chip";
    btn.setAttribute("data-ibj-intent", opt.id);
    btn.setAttribute("aria-label", opt.label_key);
    btn.textContent = opt.label_key;
    if (callbacks?.onSelectIntent) {
      btn.addEventListener("click", () => callbacks.onSelectIntent!(opt.id));
    }
    chipList.appendChild(btn);
  }
  wrapper.appendChild(chipList);

  // Budget quick filters
  const budgetGroup = rootDoc.createElement("div");
  budgetGroup.className = "ibj-budget-group";
  const budgetTitle = rootDoc.createElement("span");
  budgetTitle.className = "ibj-budget-label";
  budgetTitle.textContent = isAr ? "الميزانية:" : "Budget:";
  budgetGroup.appendChild(budgetTitle);

  const budgetTiers = [
    { label: isAr ? "حتى $1,000" : "Up to $1,000", minor: 100000 },
    { label: isAr ? "حتى $1,200" : "Up to $1,200", minor: 120000 },
    { label: isAr ? "حتى $1,500" : "Up to $1,500", minor: 150000 },
  ];
  for (const b of budgetTiers) {
    const bBtn = rootDoc.createElement("button");
    bBtn.type = "button";
    bBtn.className = "ibj-budget-chip";
    bBtn.setAttribute("data-ibj-budget", String(b.minor));
    bBtn.setAttribute("aria-label", b.label);
    bBtn.textContent = b.label;
    if (callbacks?.onSelectBudget) {
      bBtn.addEventListener("click", () => callbacks.onSelectBudget!(b.minor));
    }
    budgetGroup.appendChild(bBtn);
  }

  // Reset button
  const resetBtn = rootDoc.createElement("button");
  resetBtn.type = "button";
  resetBtn.className = "ibj-reset-button";
  resetBtn.setAttribute("data-ibj-action", "reset");
  resetBtn.setAttribute("aria-label", isAr ? "إعادة الضبط" : "Reset preferences");
  resetBtn.textContent = isAr ? "إعادة الضبط" : "Reset";
  if (callbacks?.onResetPreferences) {
    resetBtn.addEventListener("click", () => callbacks.onResetPreferences!());
  }
  budgetGroup.appendChild(resetBtn);

  wrapper.appendChild(budgetGroup);
  return wrapper;
}

function renderProductStrip(
  section: PlanSection,
  locale: Locale,
  rootDoc: Document,
): HTMLElement {
  const isAr = locale === "ar";
  const strip = rootDoc.createElement("section");
  strip.className = "ibj-product-strip";
  strip.setAttribute("role", "region");
  strip.setAttribute("aria-label", isAr ? "الحواسيب المقترحة" : "Recommended Laptops");

  // Header with title and reason badges
  const stripHeader = rootDoc.createElement("div");
  stripHeader.className = "ibj-strip-header";

  const stripTitle = rootDoc.createElement("h3");
  stripTitle.className = "ibj-strip-title";
  stripTitle.textContent = isAr ? "المقترحات المتطابقة مع اختياراتك" : "Recommended for You";
  stripHeader.appendChild(stripTitle);

  const badgeContainer = rootDoc.createElement("div");
  badgeContainer.className = "ibj-badges";
  for (const rc of section.reason_codes) {
    const badge = rootDoc.createElement("span");
    badge.className = "ibj-badge";
    badge.textContent = localizeReasonCode(rc, locale);
    badgeContainer.appendChild(badge);
  }
  stripHeader.appendChild(badgeContainer);
  strip.appendChild(stripHeader);

  // Cards container
  const cardsContainer = rootDoc.createElement("div");
  cardsContainer.className = "ibj-cards-container";

  for (const item of section.items) {
    const card = rootDoc.createElement("article");
    card.className = "ibj-product-card";
    card.setAttribute("data-variant-id", item.variant_id);

    // Resolve authoritative display props from merchant baseline cards if available
    const merchantCard = rootDoc.querySelector(
      `[data-product-id="${item.variant_id}"], [data-variant-id="${item.variant_id}"]`,
    );
    const titleText = merchantCard?.querySelector("h2, h3")?.textContent || item.variant_id;
    const priceText = merchantCard?.querySelector(".price")?.textContent || "";

    const cardTitle = rootDoc.createElement("h4");
    cardTitle.textContent = titleText;
    card.appendChild(cardTitle);

    if (priceText) {
      const cardPrice = rootDoc.createElement("p");
      cardPrice.className = "ibj-card-price";
      cardPrice.textContent = priceText;
      card.appendChild(cardPrice);
    }

    const badge = rootDoc.createElement("span");
    badge.className = "ibj-match-tag";
    badge.textContent = isAr ? "مطابق لاختيارك" : "Matched to your preferences";
    card.appendChild(badge);

    cardsContainer.appendChild(card);
  }

  strip.appendChild(cardsContainer);
  return strip;
}

function renderEmptyState(
  _section: PlanSection,
  locale: Locale,
  rootDoc: Document,
  callbacks?: RenderCallbacks,
): HTMLElement {
  const isAr = locale === "ar";
  const emptyContainer = rootDoc.createElement("div");
  emptyContainer.className = "ibj-empty-state";
  emptyContainer.setAttribute("role", "status");
  emptyContainer.setAttribute("aria-live", "polite");

  const msg = rootDoc.createElement("p");
  msg.className = "ibj-empty-message";
  msg.textContent = isAr
    ? "لم يتم العثور على حواسيب محمولة تطابق ميزانيتك وتفضيلاتك بدقة."
    : "No laptops found matching your exact budget and preferences.";
  emptyContainer.appendChild(msg);

  const resetBtn = rootDoc.createElement("button");
  resetBtn.type = "button";
  resetBtn.className = "ibj-reset-button";
  resetBtn.setAttribute("data-ibj-action", "reset");
  resetBtn.setAttribute("aria-label", isAr ? "مسح التفضيلات وعرض الكل" : "Reset preferences to view all");
  resetBtn.textContent = isAr ? "مسح التفضيلات وعرض الكل" : "Reset preferences to view all";
  if (callbacks?.onResetPreferences) {
    resetBtn.addEventListener("click", () => callbacks.onResetPreferences!());
  }
  emptyContainer.appendChild(resetBtn);

  return emptyContainer;
}

function localizeReasonCode(code: string, locale: Locale): string {
  const isAr = locale === "ar";
  switch (code) {
    case "matches_budget":
      return isAr ? "ضمن ميزانيتك" : "Within budget";
    case "matches_declared_portability":
      return isAr ? "خفيف ومناسب للتنقل" : "Lightweight & portable";
    case "matches_declared_performance":
      return isAr ? "أداء فائق" : "High performance";
    case "data_available":
      return isAr ? "بيانات معتمدة" : "Verified specs";
    case "insufficient_evidence":
      return isAr ? "لا توجد نتائج مطابقة" : "No matches";
    default:
      return code;
  }
}
