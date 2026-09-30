import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_boPhanRa } from "@/lib/api/schema.gen";

import { holdingLines } from "./org-unit-delete";
import { OrgUnitDeleteForm } from "./org-unit-delete-form";
import type { DeleteRefusal } from "./org-unit-delete-form";

/**
 * "Xoá bộ phận" (ADR 0056): the refusal a working screen rarely shows — the unit still holds staff
 * or open records — must reach the page with its counts, only the non-zero kinds.
 */

const UNIT: identity_boPhanRa = { id: "01JVP", code: "van-phong", name: "VĂN PHÒNG", parent_id: "", order: 1, staff_count: 3 };

function form(refusal: DeleteRefusal | null, localError = "") {
  return renderToStaticMarkup(
    <OrgUnitDeleteForm
      unit={UNIT}
      reason="Sáp nhập"
      setReason={() => {}}
      localError={localError}
      refusal={refusal}
      sending={false}
      onSubmit={() => {}}
      onCancel={() => {}}
    />,
  );
}

describe("holdingLines", () => {
  it("only non-zero kinds, people and sub-units first", () => {
    expect(
      holdingLines({ staff: 3, child_units: 0, open_petitions: 1, open_tasks: 2, open_incoming_documents: 0 }),
    ).toEqual(["3 cán bộ", "1 phản ánh chưa xử lý xong", "2 nhiệm vụ chưa hoàn thành"]);
  });

  it("all zero → nothing", () => {
    expect(
      holdingLines({ staff: 0, child_units: 0, open_petitions: 0, open_tasks: 0, open_incoming_documents: 0 }),
    ).toEqual([]);
  });
});

describe("delete form", () => {
  it("409 org_unit_in_use: the server's sentence AND the non-zero counts", () => {
    const html = form({
      message: "Bộ phận còn 3 cán bộ, 2 nhiệm vụ chưa hoàn thành — chuyển trước khi xoá.",
      holdings: { staff: 3, child_units: 0, open_petitions: 0, open_tasks: 2, open_incoming_documents: 0 },
    });
    expect(html).toContain("chuyển trước khi xoá");
    expect(html).toContain("<li>3 cán bộ</li>");
    expect(html).toContain("<li>2 nhiệm vụ chưa hoàn thành</li>");
    expect(html).not.toContain("bộ phận con</li>");
    expect(html).not.toContain("văn bản đến chưa xử lý xong</li>");
    expect(html).toContain('role="alert"');
  });

  it("503: the sentence only, no counts list", () => {
    const html = form({ message: "Chưa kiểm được hồ sơ bộ phận đang giữ ở phân hệ khác. Vui lòng thử lại sau.", holdings: null });
    expect(html).toContain("Vui lòng thử lại sau");
    expect(html).not.toContain("<ul>");
  });

  it("a required reason box with a real label, and the explanation says the code is not reused", () => {
    const html = form(null);
    expect(html).toMatch(/<label for="o-ly-do-xoa-bo-phan">Lý do xoá<\/label>/);
    expect(html).toContain('required=""');
    expect(html).toContain("không được cấp lại");
    expect(html).toContain("Xoá bộ phận VĂN PHÒNG");
  });

  it("the local 'reason missing' error shows before any request", () => {
    const html = form(null, "Hãy ghi lý do xoá.");
    expect(html).toContain("Hãy ghi lý do xoá.");
    expect(html).toContain('aria-invalid="true"');
  });
});
