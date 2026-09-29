import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { XA_PA } from "./noi-dung";
import { GuiPhanAnhTN, restoreDraft } from "./PhanAnhAppXa";
import type { FeedbackDraftStore, NhapPhieu } from "./trai-nghiem";
import { LINH_VUC_TAM } from "./trai-nghiem";

/**
 * The commune app's feedback draft on the send screen (ADR 0050 #7). Rendered with `react-dom/server`,
 * so no effect runs and no button is pressed: these cases pin what the citizen SEES on opening the screen,
 * and the pure "Tiếp tục" mapping. Saving as one types runs in an effect and is not covered here.
 */

const DRAFT: NhapPhieu = {
  linh_vuc: LINH_VUC_TAM[0]!,
  noi_dung: "Rác tồn đọng đầu ngõ 12",
  dia_chi: "Ngõ 12",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: false,
};

function storeWith(draft: NhapPhieu | null) {
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
    createElement(GuiPhanAnhTN, {
      ten_xa: "Xã Thăng Bình",
      ho_ten: null,
      onQuayLai: () => {},
      onDaGui: () => {},
      onXemPhieu: () => {},
      ...(draftStore ? { draftStore } : {}),
    }),
  );

describe("send screen with a saved draft", () => {
  it("asks first: 'Tiếp tục' / 'Bỏ nháp', says the draft stays on this phone, and hides the form", () => {
    const { store, calls } = storeWith(DRAFT);
    const html = render(store);
    expect(html).toContain(XA_PA.draft_title);
    expect(html).toContain(XA_PA.draft_kept_on_phone);
    // Both answers are full-size tap targets (`.xa-nut`, calc(var(--tap-min) + 4px) — accessibility.test.ts).
    expect(html).toContain(`<button type="button" class="xa-nut">${XA_PA.draft_resume}</button>`);
    expect(html).toContain(`<button type="button" class="xa-nut xa-nut--phu">${XA_PA.draft_discard}</button>`);
    // The field grid is not offered until the citizen has answered — no overwriting the draft by accident.
    expect(html).not.toContain(XA_PA.chon_linh_vuc);
    // Opening the screen reads; it writes and clears nothing.
    expect(calls).toEqual(["load"]);
    // Nothing of the draft's content is shown before the citizen asks for it.
    expect(html).not.toContain(DRAFT.noi_dung);
  });

  it("no saved draft, or no store at all (shared app): no question, the field grid opens directly", () => {
    for (const html of [render(storeWith(null).store), render()]) {
      expect(html).not.toContain(XA_PA.draft_title);
      expect(html).toContain(XA_PA.chon_linh_vuc);
    }
  });

  it("'Tiếp tục' restores every field and opens the writing step when the field still exists", () => {
    expect(restoreDraft(DRAFT, null)).toEqual({ form: DRAFT, step: 2 });
    // A field the catalogue no longer has → step 1, field empty, the rest kept.
    const gone = restoreDraft({ ...DRAFT, linh_vuc: "Lĩnh vực đã gỡ" }, null);
    expect(gone.step).toBe(1);
    expect(gone.form).toEqual({ ...DRAFT, linh_vuc: "" });
    // No field chosen yet → step 1.
    expect(restoreDraft({ ...DRAFT, linh_vuc: "" }, null).step).toBe(1);
    // An anonymous draft keeps no name; the name from Zalo in this session fills in.
    expect(restoreDraft({ ...DRAFT, ho_ten: "", an_danh: true }, "Trần Thị Đào").form.ho_ten).toBe("Trần Thị Đào");
    expect(restoreDraft({ ...DRAFT, ho_ten: "" }, null).form.ho_ten).toBe("");
  });
});
