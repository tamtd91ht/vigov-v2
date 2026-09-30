import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  BULK_MAX,
  BULK_NONE_CONSENTED,
  BULK_NONE_SELECTED,
  BULK_REPLAYED,
  BULK_TOO_MANY,
  addSelected,
  bulkListRows,
  bulkRequest,
  bulkResultLines,
  bulkResultText,
  eligibleForBulk,
  removeSelected,
  setConsent,
  type BulkSelection,
} from "./bulk-publication";
import { BulkOutcomeView, BulkPublicationPanel } from "./bulk-publication-panel";
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

describe("ai được đưa vào danh sách chọn", () => {
  it("chỉ người CHƯA hiện và KHÔNG bị khoá — người khoá bị ẩn (máy chủ vẫn quyết định)", () => {
    expect(eligibleForBulk(A)).toBe(true);
    expect(eligibleForBulk(LOCKED)).toBe(false);
    expect(eligibleForBulk(PUBLIC)).toBe(false);
    expect(bulkListRows([], [A, LOCKED, PUBLIC, B]).map((r) => r.candidate?.id)).toEqual([A.id, B.id]);
  });

  it("người đã chọn ở TRANG KHÁC vẫn hiện (để còn đổi ô tick), và không hiện hai lần", () => {
    const sel = addSelected([], B);
    const rows = bulkListRows(sel, [A, B]);
    expect(rows.map((r) => r.selected?.id ?? r.candidate?.id)).toEqual([B.id, A.id]);
  });

  it("chọn mới thì ô 'đã hỏi ý' TRỐNG; bỏ chọn là bỏ luôn dấu tick", () => {
    let sel: BulkSelection = addSelected([], A);
    expect(sel[0]?.consentAsked).toBe(false);
    sel = setConsent(sel, A.id, true);
    expect(sel[0]?.consentAsked).toBe(true);
    sel = removeSelected(sel, A.id);
    sel = addSelected(sel, A);
    expect(sel[0]?.consentAsked).toBe(false);
  });
});

describe("bấm gửi — từ chối tại chỗ hoặc các dòng gửi đi", () => {
  it("chưa chọn ai / chưa tick ai / quá trần → câu từ chối, không có dòng nào", () => {
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

  it("dòng chưa tick VẪN được gửi, với consentAsked false — danh sách kết quả mới đủ người", () => {
    const sel = setConsent(addSelected(addSelected([], A), B), A.id, true);
    expect(bulkRequest(sel)).toEqual({
      rows: [
        { id: A.id, consentAsked: true },
        { id: B.id, consentAsked: false },
      ],
    });
  });
});

describe("kết quả từng dòng — tiếng Việt, theo reason_code", () => {
  it.each([
    [{ id: "x", result: "published" }, "Đã công khai"],
    [{ id: "x", result: "skipped", reason_code: "consent_required" }, "Bỏ qua — chưa hỏi ý"],
    [{ id: "x", result: "skipped", reason_code: "staff_locked" }, "Bỏ qua — tài khoản đang khoá"],
    [{ id: "x", result: "skipped", reason_code: "staff_not_found" }, "Không tìm thấy"],
    [{ id: "x", result: "skipped", reason_code: "something_new" }, "Bỏ qua"],
  ])("%j → %s", (item, text) => {
    expect(bulkResultText(item)).toBe(text);
  });

  it("nối theo id với người đã chọn: họ tên + mã cán bộ, KHÔNG số điện thoại", () => {
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

  it("phát lại → câu nói danh bạ đã tải lại, không có danh sách", () => {
    const html = renderToStaticMarkup(<BulkOutcomeView outcome={{ kind: "replayed" }} />);
    expect(html).toContain(BULK_REPLAYED);
    expect(html).not.toContain("<li");
  });
});

describe("khung công khai nhiều người — cái ra tới trang", () => {
  function render(selection: BulkSelection) {
    return renderToStaticMarkup(
      <BulkPublicationPanel
        selection={selection}
        pageRows={[A, B, LOCKED, PUBLIC]}
        onSelect={() => {}}
        onUnselect={() => {}}
        onSetConsent={() => {}}
        error=""
        sending={false}
        outcome={null}
        onSubmit={() => {}}
        onClose={() => {}}
      />,
    );
  }

  it("nêu cảnh báo dữ liệu cá nhân nguyên văn; không vẽ người bị khoá hay đã hiện; không số điện thoại", () => {
    const html = render([]);
    expect(html).toContain(CANH_BAO_CONG_KHAI);
    expect(html).toContain("Nguyễn Văn A");
    expect(html).not.toContain("Lê Văn Khoá");
    expect(html).not.toContain("Phạm Đã Hiện");
    expect(html).not.toMatch(/09\d{8}|0235\d{7}/);
  });

  it("ô 'Đã hỏi ý người này' chỉ có ở dòng ĐÃ CHỌN, không tick sẵn; nút gửi mờ khi chưa ai được tick", () => {
    const html = render(addSelected([], A));
    expect(html).toContain(`id="bulk-consent-${A.id}"`);
    expect(html).not.toContain(`id="bulk-consent-${B.id}"`);
    expect(/<input[^>]*id="bulk-consent-[^"]*"[^>]*>/.exec(html)?.[0]).not.toMatch(/checked/);
    expect(html).toMatch(/<button type="submit"[^>]*disabled/);
  });
});
