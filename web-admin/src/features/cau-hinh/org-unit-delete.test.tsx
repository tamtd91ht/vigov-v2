import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_boPhanRa } from "@/lib/api/schema.gen";

import { holdingSentences } from "./org-unit-delete";
import { OrgUnitDeleteForm } from "./org-unit-delete-form";
import type { DeleteRefusal } from "./org-unit-delete-form";

/**
 * "Xoá bộ phận" (ADR 0056; bug sheet row 30): a confirm with no reason box, and every refusal — the
 * unit still holds staff or open records, or the deployment cannot ask — shown as its own sentence.
 */

const UNIT: identity_boPhanRa = { id: "01JVP", code: "van-phong", name: "VĂN PHÒNG", parent_id: "", order: 1, staff_count: 3 };

function form(refusal: DeleteRefusal | null) {
  return renderToStaticMarkup(
    <OrgUnitDeleteForm unit={UNIT} refusal={refusal} sending={false} onSubmit={() => {}} onCancel={() => {}} />,
  );
}

describe("holdingSentences", () => {
  it("one sentence per non-zero kind, people and sub-units first", () => {
    expect(
      holdingSentences({ staff: 3, child_units: 1, open_petitions: 0, open_tasks: 2, open_incoming_documents: 0 }),
    ).toEqual([
      "Bộ phận còn 3 cán bộ trực thuộc. Hãy chuyển cán bộ sang bộ phận khác trước.",
      "Bộ phận còn 1 bộ phận con. Hãy xoá hoặc chuyển bộ phận con trước.",
      "Bộ phận còn 2 nhiệm vụ chưa hoàn thành. Hãy chuyển sang bộ phận khác trước.",
    ]);
  });

  it("all zero → nothing", () => {
    expect(
      holdingSentences({ staff: 0, child_units: 0, open_petitions: 0, open_tasks: 0, open_incoming_documents: 0 }),
    ).toEqual([]);
  });
});

describe("delete form", () => {
  it("409 org_unit_in_use with staff: 'cán bộ trực thuộc' as its own sentence, in an alert", () => {
    const html = form({
      message: "Bộ phận còn 3 cán bộ, 2 nhiệm vụ chưa hoàn thành — chuyển trước khi xoá.",
      holdings: { staff: 3, child_units: 0, open_petitions: 0, open_tasks: 2, open_incoming_documents: 0 },
    });
    expect(html).toContain("<p>Bộ phận còn 3 cán bộ trực thuộc. Hãy chuyển cán bộ sang bộ phận khác trước.</p>");
    expect(html).toContain("<p>Bộ phận còn 2 nhiệm vụ chưa hoàn thành. Hãy chuyển sang bộ phận khác trước.</p>");
    expect(html).not.toContain("bộ phận con.");
    expect(html).toContain('role="alert"');
  });

  it("409 without readable counts: the server's sentence verbatim", () => {
    const html = form({ message: "Bộ phận còn giữ hồ sơ.", holdings: null });
    expect(html).toContain("<p>Bộ phận còn giữ hồ sơ.</p>");
  });

  it("503 org_unit_delete_not_configured: the server's sentence, in the dialog", () => {
    const sentence = "Chức năng xoá bộ phận chưa được bật trên hệ thống này. Hãy báo quản trị kỹ thuật.";
    const html = form({ message: sentence, holdings: null });
    expect(html).toContain(sentence);
    expect(html).toContain('role="alert"');
  });

  it("NO reason box (bug sheet row 30): the question, the rule, Huỷ then the red Xoá", () => {
    const html = form(null);
    expect(html).not.toContain("<textarea");
    expect(html).not.toMatch(/Lý do/i);
    expect(html).toMatch(/<h2[^>]*>Xoá bộ phận VĂN PHÒNG\?<\/h2>/);
    expect(html).toContain("Không xoá được khi bộ phận còn cán bộ");
    expect(html).toContain("sm:max-w-lg");
    expect(html).toContain("bg-muted/50");
    expect(html.indexOf(">Huỷ<")).toBeLessThan(html.indexOf('type="submit"'));
    expect(html).not.toContain('role="alert"');
  });
});
