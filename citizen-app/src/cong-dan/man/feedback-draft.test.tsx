import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { XA_PA } from "./noi-dung";
import { CommuneSendScreen, restoreDraft, verifiedSenderOf, withVerifiedSender } from "./PhanAnhAppXa";
import type { FeedbackDraftStore, NhapPhieu } from "./trai-nghiem";

/** Field CODES as the commune's catalogue offers them (`my-citizen-report-fields`). */
const OFFERED = ["rac-thai", "giao-thong", "khac"];

/**
 * The commune app's feedback draft on the send screen (ADR 0050 #7). Rendered with `react-dom/server`,
 * so no effect runs and no button is pressed: these cases pin what the citizen SEES on opening the screen,
 * and the pure "Tiếp tục" mapping. Saving as one types runs in an effect and is not covered here.
 */

const DRAFT: NhapPhieu = {
  linh_vuc: "rac-thai",
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
    createElement(CommuneSendScreen, {
      ten_xa: "Xã Thăng Bình",
      ho_ten: null,
      onBack: () => {},
      onSessionLost: () => {},
      onSent: () => {},
      onOpenPetition: () => {},
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
    // Step 1 is not offered until the citizen has answered — no overwriting the draft by accident.
    expect(html).not.toContain(XA_PA.chon_linh_vuc);
    expect(html).not.toContain(XA_PA.fields_loading);
    // Opening the screen reads; it writes and clears nothing.
    expect(calls).toEqual(["load"]);
    // Nothing of the draft's content is shown before the citizen asks for it.
    expect(html).not.toContain(DRAFT.noi_dung);
  });

  it("no saved draft, or no store at all: no question; step 1 opens on the commune's catalogue, loading", () => {
    for (const html of [render(storeWith(null).store), render()]) {
      expect(html).not.toContain(XA_PA.draft_title);
      // The catalogue loads after mount (never a built-in list meanwhile).
      expect(html).toContain(XA_PA.fields_loading);
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

  it("on a VERIFIED session the draft's number never comes back; (a) takes the Zalo name, (c) keeps the typed one", () => {
    // (a): no box shows the name or the number, so neither may ride along hidden from the draft.
    const a = restoreDraft(DRAFT, "Trần Thị Đào", OFFERED, { kind: "zalo-name", name: "Trần Thị Đào" });
    expect(a.form).toEqual({ ...DRAFT, ho_ten: "Trần Thị Đào", dien_thoai: "" });
    // (c): the name box shows the draft's name; the number has no box, so it goes.
    const c = restoreDraft(DRAFT, null, OFFERED, { kind: "typed-name" });
    expect(c.form).toEqual({ ...DRAFT, dien_thoai: "" });
    expect(c.step).toBe(2);
  });
});

describe("the sender of a verified session, from the Zalo name request", () => {
  it("settled with a name → (a); settled without one, or never consented → (c); still being asked → pending (null)", () => {
    expect(verifiedSenderOf({ kind: "settled", name: " Trần Thị Đào " })).toEqual({ kind: "zalo-name", name: "Trần Thị Đào" });
    expect(verifiedSenderOf({ kind: "settled", name: null })).toEqual({ kind: "typed-name" });
    expect(verifiedSenderOf({ kind: "settled", name: "  " })).toEqual({ kind: "typed-name" });
    expect(verifiedSenderOf({ kind: "needs-consent" })).toEqual({ kind: "typed-name" });
    expect(verifiedSenderOf({ kind: "checking" })).toBeNull();
    expect(verifiedSenderOf({ kind: "asking" })).toBeNull();
  });

  it("the form as sent: the number always empty; the name is Zalo's in (a), the typed one in (c)", () => {
    expect(withVerifiedSender(DRAFT, { kind: "zalo-name", name: "Trần Thị Đào" })).toEqual({
      ...DRAFT,
      ho_ten: "Trần Thị Đào",
      dien_thoai: "",
    });
    expect(withVerifiedSender(DRAFT, { kind: "typed-name" })).toEqual({ ...DRAFT, dien_thoai: "" });
  });
});
