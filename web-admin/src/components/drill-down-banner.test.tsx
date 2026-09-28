import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { SoNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { SoPhanAnh } from "@/features/phan-anh/so-phan-anh";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { SoVanBanDen } from "@/features/van-ban/so-van-ban-den";
import { INVALID_DRILL_DOWN_LINE, parseDrillDown } from "@/lib/drill-down";

import { CLEAR_DRILL_DOWN_LABEL, DrillDownBanner } from "./drill-down-banner";

/**
 * Dải lọc Tổng quan và việc nó RA TỚI ba màn danh sách (SRS M7.2.2).
 *
 * Nhóm màn hình canh vế dễ gãy nhất: lọc bật thì hàng lọc riêng của màn KHÔNG vẽ — một ô lọc còn
 * đứng đó là một lối ghép thêm điều kiện, và con số trên Tổng quan thôi bằng số dòng. Và vế ngược
 * lại: không có lọc thì màn vẽ hàng lọc như cũ, không có dải nào.
 */

const FROM = "2026-09-20T17:00:00Z";
const TO = "2026-09-27T17:00:00Z";
const NOTE = "Câu của màn.";

describe("DrillDownBanner", () => {
  it("số liệu theo kỳ: 'Đang xem: <nhãn> — <kỳ>', câu của màn, và liên kết Bỏ lọc về đường dẫn trơn", () => {
    const html = renderToStaticMarkup(
      <DrillDownBanner
        drillDown={parseDrillDown("tasks", { metric: "completed", from: FROM, to: TO })}
        clearHref="/nhiem-vu"
        note={NOTE}
      />,
    );
    expect(html).toContain("Đang xem: Hoàn thành trong kỳ — 21/09/2026–27/09/2026");
    expect(html).toContain(NOTE);
    expect(html).toMatch(new RegExp(`<a[^>]*href="/nhiem-vu"[^>]*>${CLEAR_DRILL_DOWN_LABEL}</a>`));
    expect(CLEAR_DRILL_DOWN_LABEL).toBe("Bỏ lọc");
    expect(html).toContain('role="status"');
  });

  it("số liệu tồn: kỳ in 'tính đến hiện tại'", () => {
    const html = renderToStaticMarkup(
      <DrillDownBanner
        drillDown={parseDrillDown("incoming-documents", { metric: "overdue" })}
        clearHref="/van-ban"
        note={NOTE}
      />,
    );
    expect(html).toContain("Đang xem: Quá hạn xử lý — tính đến hiện tại");
  });

  it("đường dẫn hỏng: đúng câu cảnh báo, KHÔNG có 'Đang xem'", () => {
    const html = renderToStaticMarkup(
      <DrillDownBanner
        drillDown={parseDrillDown("tasks", { metric: "bogus" })}
        clearHref="/nhiem-vu"
        note={NOTE}
      />,
    );
    expect(html).toContain(INVALID_DRILL_DOWN_LINE);
    expect(INVALID_DRILL_DOWN_LINE).toBe(
      "Đường dẫn lọc không hợp lệ — đang hiện toàn bộ danh sách.",
    );
    expect(html).not.toContain("Đang xem");
  });

  it("đường dẫn hỏng mà cán bộ đã tự lọc: câu 'đang hiện toàn bộ' không còn đúng nên không hiện", () => {
    const html = renderToStaticMarkup(
      <DrillDownBanner
        drillDown={parseDrillDown("tasks", { metric: "bogus" })}
        clearHref="/nhiem-vu"
        note={NOTE}
        showInvalid={false}
      />,
    );
    expect(html).toBe("");
  });

  it("không có lọc: không vẽ gì", () => {
    const html = renderToStaticMarkup(
      <DrillDownBanner drillDown={parseDrillDown("tasks", {})} clearHref="/nhiem-vu" note={NOTE} />,
    );
    expect(html).toBe("");
  });
});

function render(node: ReactNode): string {
  return renderToStaticMarkup(<PhienProvider>{node}</PhienProvider>);
}

describe("sổ nhiệm vụ nhận lọc", () => {
  it("lọc bật: có dải, KHÔNG hàng lọc, KHÔNG ô tìm, KHÔNG nút Kanban", () => {
    const html = render(<SoNhiemVu drillDown={parseDrillDown("tasks", { metric: "suspended" })} />);
    expect(html).toContain("Đang xem: Tạm dừng — tính đến hiện tại");
    expect(html).not.toContain('id="tim-nhiem-vu"');
    expect(html).not.toContain('id="loc-trang-thai"');
    expect(html).not.toContain('aria-label="Chế độ xem"');
    expect(html).not.toContain('aria-label="Phạm vi"');
  });

  it("không lọc: hàng lọc và nút chế độ xem như cũ, không có dải", () => {
    const html = render(<SoNhiemVu />);
    expect(html).toContain('id="tim-nhiem-vu"');
    expect(html).toContain('aria-label="Chế độ xem"');
    expect(html).not.toContain("Đang xem:");
    expect(html).not.toContain(INVALID_DRILL_DOWN_LINE);
  });

  it("đường dẫn hỏng: câu cảnh báo, hàng lọc vẫn có", () => {
    const html = render(<SoNhiemVu drillDown={parseDrillDown("tasks", { metric: "completed" })} />);
    expect(html).toContain(INVALID_DRILL_DOWN_LINE);
    expect(html).toContain('id="tim-nhiem-vu"');
  });
});

describe("sổ phản ánh nhận lọc", () => {
  it("lọc bật: có dải, KHÔNG hàng lọc, KHÔNG ô tìm, KHÔNG tab phạm vi", () => {
    const html = render(
      <SoPhanAnh
        drillDown={parseDrillDown("citizen-reports", { metric: "received", from: FROM, to: TO })}
      />,
    );
    expect(html).toContain("Đang xem: Nhận vào trong kỳ — 21/09/2026–27/09/2026");
    expect(html).not.toContain('id="tim-phan-anh"');
    expect(html).not.toContain('id="loc-trang-thai"');
    expect(html).not.toContain('aria-label="Phạm vi"');
    expect(html).toMatch(/href="\/phan-anh"/);
  });

  it("không lọc: hàng lọc như cũ", () => {
    const html = render(<SoPhanAnh />);
    expect(html).toContain('id="tim-phan-anh"');
    expect(html).not.toContain("Đang xem:");
  });

  it("đường dẫn hỏng: câu cảnh báo", () => {
    const html = render(
      <SoPhanAnh drillDown={parseDrillDown("citizen-reports", { metric: "suspended" })} />,
    );
    expect(html).toContain(INVALID_DRILL_DOWN_LINE);
    expect(html).toContain('id="tim-phan-anh"');
  });
});

describe("sổ văn bản đến nhận lọc", () => {
  it("lọc bật: có dải, KHÔNG ô năm, KHÔNG hàng lọc, KHÔNG ô tìm", () => {
    const html = render(
      <SoVanBanDen drillDown={parseDrillDown("incoming-documents", { metric: "open" })} />,
    );
    expect(html).toContain("Đang xem: Chưa xử lý xong — tính đến hiện tại");
    expect(html).not.toContain('id="nam-so-van-ban-den"');
    expect(html).not.toContain('id="loc-trang-thai-den"');
    expect(html).not.toContain('id="tim-van-ban-den"');
    expect(html).toMatch(/href="\/van-ban"/);
  });

  it("không lọc: ô năm và hàng lọc như cũ", () => {
    const html = render(<SoVanBanDen />);
    expect(html).toContain('id="nam-so-van-ban-den"');
    expect(html).toContain('id="loc-trang-thai-den"');
    expect(html).not.toContain("Đang xem:");
  });

  it("đường dẫn hỏng (kỳ trên số liệu tồn): câu cảnh báo, hàng lọc vẫn có", () => {
    const html = render(
      <SoVanBanDen
        drillDown={parseDrillDown("incoming-documents", { metric: "overdue", from: FROM, to: TO })}
      />,
    );
    expect(html).toContain(INVALID_DRILL_DOWN_LINE);
    expect(html).toContain('id="loc-trang-thai-den"');
  });
});
