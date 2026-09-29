import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { COMMUNE_APP_REPORTS } from "./copy";
import { CommuneSendScreen, restoreDraft } from "./CommuneAppReports";
import type { FeedbackDraftStore, ReportDraft } from "./commune-app-model";

/** Field CODES as the commune's catalogue offers them (`my-citizen-report-fields`). */
const OFFERED = ["rac-thai", "giao-thong", "khac"];

/**
 * The commune app's feedback draft on the send screen (ADR 0050 #7). Rendered with `react-dom/server`,
 * so no effect runs and no button is pressed: these cases pin what the citizen SEES on opening the screen,
 * and the pure "Tiếp tục" mapping. Saving as one types runs in an effect and is not covered here.
 */

const DRAFT: ReportDraft = {
  linh_vuc: "rac-thai",
  noi_dung: "Rác tồn đọng đầu ngõ 12",
  dia_chi: "Ngõ 12",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: false,
};

function storeWith(draft: ReportDraft | null) {
  const calls: string[] = [];
  const store: FeedbackDraftStore = {
    load: () => {
      calls.push("load");
      return draft;
    },
    save: () => void calls.push("save"),
    clear: () => void calls.push("clear"),
  };
  return { store, calls };
}

const render = (draftStore?: FeedbackDraftStore) =>
  renderToStaticMarkup(
    createElement(CommuneSendScreen, {
      commune_name: "Xã Thăng Bình",
      full_name: null,
      onBack: () => {},
      onSessionLost: () => {},
      onSent: () => {},
      onOpenReport: () => {},
      ...(draftStore ? { draftStore } : {}),
    }),
  );

describe("send screen with a saved draft", () => {
  it("asks first: 'Tiếp tục' / 'Bỏ nháp', says the draft stays on this phone, and hides the form", () => {
    const { store, calls } = storeWith(DRAFT);
    const html = render(store);
    expect(html).toContain(COMMUNE_APP_REPORTS.draft_title);
    expect(html).toContain(COMMUNE_APP_REPORTS.draft_kept_on_phone);
    // Both answers are full-size tap targets (`.xa-nut`, calc(var(--tap-min) + 4px) — accessibility.test.ts).
    expect(html).toContain(`<button type="button" class="xa-nut">${COMMUNE_APP_REPORTS.draft_resume}</button>`);
    expect(html).toContain(`<button type="button" class="xa-nut xa-nut--phu">${COMMUNE_APP_REPORTS.draft_discard}</button>`);
    // Step 1 is not offered until the citizen has answered — no overwriting the draft by accident.
    expect(html).not.toContain(COMMUNE_APP_REPORTS.choose_field);
    expect(html).not.toContain(COMMUNE_APP_REPORTS.fields_loading);
    // Opening the screen reads; it writes and clears nothing.
    expect(calls).toEqual(["load"]);
    // Nothing of the draft's content is shown before the citizen asks for it.
    expect(html).not.toContain(DRAFT.noi_dung);
  });

  it("no saved draft, or no store at all: no question; step 1 opens on the commune's catalogue, loading", () => {
    for (const html of [render(storeWith(null).store), render()]) {
      expect(html).not.toContain(COMMUNE_APP_REPORTS.draft_title);
      // The catalogue loads after mount (never a built-in list meanwhile).
      expect(html).toContain(COMMUNE_APP_REPORTS.fields_loading);
    }
  });

  it("'Tiếp tục' restores every field and opens the writing step when the code is still OFFERED", () => {
    expect(restoreDraft(DRAFT, null, OFFERED)).toEqual({ form: DRAFT, step: 2 });
    // A code the commune no longer offers → step 1, field empty, the rest kept.
    const gone = restoreDraft({ ...DRAFT, linh_vuc: "an-ninh" }, null, OFFERED);
    expect(gone.step).toBe(1);
    expect(gone.form).toEqual({ ...DRAFT, linh_vuc: "" });
    // A draft written with the old temporary list holds a NAME, not a code → dropped the same way.
    expect(restoreDraft({ ...DRAFT, linh_vuc: "Rác thải – Vệ sinh môi trường" }, null, OFFERED).form.linh_vuc).toBe("");
    // Catalogue not loaded yet: kept for now — the screen drops it when the catalogue arrives without it.
    expect(restoreDraft(DRAFT, null, null)).toEqual({ form: DRAFT, step: 2 });
    // No field chosen yet → step 1.
    expect(restoreDraft({ ...DRAFT, linh_vuc: "" }, null, OFFERED).step).toBe(1);
    // An anonymous draft keeps no name; the name from Zalo in this session fills in.
    expect(restoreDraft({ ...DRAFT, ho_ten: "", an_danh: true }, "Trần Thị Đào", OFFERED).form.ho_ten).toBe("Trần Thị Đào");
    expect(restoreDraft({ ...DRAFT, ho_ten: "" }, null, OFFERED).form.ho_ten).toBe("");
  });
});
