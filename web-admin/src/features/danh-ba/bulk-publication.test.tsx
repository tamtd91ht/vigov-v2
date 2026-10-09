import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BulkConsentForm, BulkOutcomeView } from "./bulk-consent-form";
import {
  BULK_MAX,
  BULK_NO_CANDIDATES,
  BULK_NONE_CONSENTED,
  BULK_NONE_SELECTED,
  BULK_REPLAYED,
  BULK_TOO_MANY,
  addSelected,
  bulkAddedToast,
  bulkRequest,
  bulkResultLines,
  bulkResultText,
  bulkWithdrawnToast,
  consentSelection,
  eligibleForBulk,
  publishedCount,
  removeSelected,
  selectedCountText,
  setConsent,
  togglePage,
  toggleRow,
  type BulkSelection,
} from "./bulk-publication";
import { CANH_BAO_CONG_KHAI } from "./cong-khai";

/** Fake numbers only (rule 3, invariant 5). */
function staff(over: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Chuyên viên",
    department_id: "BP-LE",
    role_id: "",
    phone: "02350000001",
    mobile: "0900000001",
    has_account: false,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...over,
  };
}

const A = staff();
const B = staff({ id: "01J00000000000000000000002", code: "CB-00124", full_name: "Trần Thị B", mobile: "0900000002" });
const LOCKED = staff({ id: "01J00000000000000000000003", code: "CB-00125", full_name: "Lê Văn Khoá", active: false });
const PUBLIC = staff({ id: "01J00000000000000000000004", code: "CB-00126", full_name: "Phạm Đã Hiện", published: true });

describe("the table selection", () => {
  it("ticking keeps the order of ticking; unticking drops the row; ticking twice is one row", () => {
    let sel = toggleRow([], B, true);
    sel = toggleRow(sel, A, true);
    sel = toggleRow(sel, A, true);
    expect(sel.map((s) => s.id)).toEqual([B.id, A.id]);
    expect(toggleRow(sel, B, false).map((s) => s.id)).toEqual([A.id]);
  });

  it("the page box ticks every row of the page and keeps people ticked on other pages; unticking drops only the page", () => {
    const other = staff({ id: "elsewhere", full_name: "Người trang khác" });
    const on = togglePage([other], [A, B], true);
    expect(on.map((s) => s.id)).toEqual(["elsewhere", A.id, B.id]);
    expect(togglePage(on, [A, B], false).map((s) => s.id)).toEqual(["elsewhere"]);
  });
});

describe("who the consent dialog asks about", () => {
  it("only people NOT on the Mini App and NOT locked (the server still decides)", () => {
    expect(eligibleForBulk(A)).toBe(true);
    expect(eligibleForBulk(LOCKED)).toBe(false);
    expect(eligibleForBulk(PUBLIC)).toBe(false);
    expect(consentSelection([A, LOCKED, PUBLIC, B]).map((s) => s.id)).toEqual([A.id, B.id]);
  });

  it("EVERY consent box starts unticked — consent is asserted per person, never carried over", () => {
    expect(consentSelection([A, B]).every((s) => !s.consentAsked)).toBe(true);
    let sel: BulkSelection = addSelected([], A);
    sel = setConsent(sel, A.id, true);
    sel = removeSelected(sel, A.id);
    sel = addSelected(sel, A);
    expect(sel[0]?.consentAsked).toBe(false);
  });
});

describe("pressing submit — refused on the spot, or the rows to send", () => {
  it("nobody / nobody ticked / over the cap → a refusal sentence, no rows", () => {
    expect(bulkRequest([])).toEqual({ error: BULK_NONE_SELECTED });
    expect(bulkRequest(addSelected([], A))).toEqual({ error: BULK_NONE_CONSENTED });
    const many: BulkSelection = Array.from({ length: BULK_MAX + 1 }, (_, i) => ({
      id: `id-${i}`,
      code: `CB-${i}`,
      fullName: `Người ${i}`,
      consentAsked: true,
    }));
    expect(bulkRequest(many)).toEqual({ error: BULK_TOO_MANY });
  });

  it("an unticked row IS sent, with consentAsked false — the result list then names everyone", () => {
    const sel = setConsent(addSelected(addSelected([], A), B), A.id, true);
    expect(bulkRequest(sel)).toEqual({
      rows: [
        { id: A.id, consentAsked: true },
        { id: B.id, consentAsked: false },
      ],
    });
  });
});

describe("the prototype's wording", () => {
  it("bar count and the two toasts, verbatim", () => {
    expect(selectedCountText(4)).toBe("Đã chọn 4 người");
    expect(bulkAddedToast(2)).toBe("Đã thêm 2 người vào danh bạ trên Mini App.");
    expect(bulkWithdrawnToast(3)).toBe("Đã rút 3 người khỏi danh bạ trên Mini App.");
  });

  it("the added toast counts what the SERVER published, not the selection", () => {
    expect(
      publishedCount([
        { id: "a", result: "published" },
        { id: "b", result: "skipped", reason_code: "consent_required" },
      ]),
    ).toBe(1);
  });
});

describe("per-row results — Vietnamese, by reason_code", () => {
  it.each([
    [{ id: "x", result: "published" }, "Đã công khai"],
    [{ id: "x", result: "skipped", reason_code: "consent_required" }, "Bỏ qua — chưa hỏi ý"],
    [{ id: "x", result: "skipped", reason_code: "staff_locked" }, "Bỏ qua — tài khoản đang khoá"],
    [{ id: "x", result: "skipped", reason_code: "staff_not_found" }, "Không tìm thấy"],
    [{ id: "x", result: "skipped", reason_code: "something_new" }, "Bỏ qua"],
  ])("%j → %s", (item, text) => {
    expect(bulkResultText(item)).toBe(text);
  });

  it("joined on id with the selection: name + staff code, NEVER a phone number", () => {
    const sel = addSelected(addSelected([], A), B);
    const lines = bulkResultLines(
      [
        { id: A.id, result: "published" },
        { id: B.id, result: "skipped", reason_code: "consent_required" },
        { id: "01JLA", result: "skipped", reason_code: "staff_not_found" },
      ],
      sel,
    );
    expect(lines.map((l) => [l.who, l.text])).toEqual([
      ["Nguyễn Văn A (CB-00123)", "Đã công khai"],
      ["Trần Thị B (CB-00124)", "Bỏ qua — chưa hỏi ý"],
      ["Không rõ người", "Không tìm thấy"],
    ]);

    const html = renderToStaticMarkup(<BulkOutcomeView outcome={{ kind: "lines", lines }} />);
    expect(html).toContain("Đã công khai 1/3 người");
    expect(html).not.toMatch(/09\d{8}|0235\d{7}/);
  });

  it("replay → the sentence that the register was re-read, no list", () => {
    const html = renderToStaticMarkup(<BulkOutcomeView outcome={{ kind: "replayed" }} />);
    expect(html).toContain(BULK_REPLAYED);
    expect(html).not.toContain("<li");
  });
});

describe("the consent dialog — what reaches the page", () => {
  function render(selection: BulkSelection) {
    return renderToStaticMarkup(
      <BulkConsentForm
        selection={selection}
        onSetConsent={() => {}}
        error=""
        sending={false}
        outcome={null}
        onSubmit={() => {}}
        onClose={() => {}}
      />,
    );
  }

  it("the personal-data warning verbatim; one consent box per person, UNTICKED; submit disabled; no phone number", () => {
    const html = render(consentSelection([A, B]));
    expect(html).toContain(CANH_BAO_CONG_KHAI);
    expect(html).toContain(`id="bulk-consent-${A.id}"`);
    expect(html).toContain(`id="bulk-consent-${B.id}"`);
    for (const box of html.match(/<input[^>]*id="bulk-consent-[^"]*"[^>]*>/g) ?? []) expect(box).not.toMatch(/checked/);
    expect(html).toMatch(/<button[^>]*type="submit"[^>]*disabled=""/);
    expect(html).not.toMatch(/09\d{8}|0235\d{7}/);
  });

  it("one ticked person → the submit button is enabled", () => {
    const html = render(setConsent(consentSelection([A, B]), A.id, true));
    expect(html).toMatch(/<button[^>]*type="submit"/);
    expect(html).not.toMatch(/<button[^>]*type="submit"[^>]*disabled=""/);
  });

  it("nobody eligible (all on the Mini App or locked) → says so, no submit button at all", () => {
    const html = render(consentSelection([PUBLIC, LOCKED]));
    expect(html).toContain(BULK_NO_CANDIDATES);
    expect(html).not.toContain('type="submit"');
  });
});
