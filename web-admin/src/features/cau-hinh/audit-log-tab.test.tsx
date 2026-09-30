import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { audit_EntryView } from "@/lib/api/schema.gen";

let fakeSession: PhienDaDoc = null;

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
}));

const { AuditLogTab, AuditLogView } = await import("./audit-log-tab");
const { applyPage, initialSources } = await import("./audit-log-merge");
const { EMPTY_AUDIT_FILTER } = await import("./audit-log-view");

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: [...permissions],
    },
  } as PhienDaDoc;
}

function entry(patch: Partial<audit_EntryView>): audit_EntryView {
  return {
    at: "2026-09-28T03:00:00Z",
    actor_kind: "staff",
    actor_code: "CB-00123",
    actor_ip: "10.0.0.1",
    action: "khoa_tai_khoan_can_bo",
    subject: "CB-00200",
    delta: null,
    ...patch,
  };
}

const done = (items: audit_EntryView[]) =>
  ({ ok: true, duLieu: { items, has_more: false, next_cursor: "" } }) as const;

function view(sources: ReturnType<typeof initialSources>) {
  return renderToStaticMarkup(
    <AuditLogView
      sources={sources}
      draft={EMPTY_AUDIT_FILTER}
      setDraft={() => {}}
      filterError={null}
      onFilter={() => {}}
      onClear={() => {}}
      onLoad={() => {}}
    />,
  );
}

afterEach(() => {
  fakeSession = null;
  vi.unstubAllGlobals();
});

describe("tab Nhật ký hệ thống — cổng quyền", () => {
  it("CA BỊ TỪ CHỐI: thiếu `admin.audit` → câu từ chối, KHÔNG gọi tuyến nào", () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    fakeSession = sessionWith(["admin.user", "admin.role", "admin.lookup"]);
    const html = renderToStaticMarkup(<AuditLogTab />);
    expect(html).toContain("không có quyền xem nhật ký hệ thống");
    expect(html).not.toContain("<table");
    expect(fake).not.toHaveBeenCalled();
  });

  it("phiên đọc hỏng → câu của máy chủ, đóng khi không chắc", () => {
    fakeSession = { ok: false, thongBao: "Phiên làm việc đã hết hạn" } as PhienDaDoc;
    const html = renderToStaticMarkup(<AuditLogTab />);
    expect(html).toContain("Phiên làm việc đã hết hạn");
    expect(html).not.toContain("Lọc nhật ký");
  });

  it("có `admin.audit` → biểu mẫu lọc hiện, đang tải", () => {
    fakeSession = sessionWith(["admin.audit"]);
    const html = renderToStaticMarkup(<AuditLogTab />);
    expect(html).toContain('aria-label="Lọc nhật ký hệ thống"');
    expect(html).toContain("Đang tải nhật ký");
  });
});

describe("tab Nhật ký hệ thống — dựng các mục", () => {
  it("bảy cột, phân hệ ghi trên từng dòng, thao tác là mã nguyên văn", () => {
    let s = initialSources();
    for (const k of ["identity", "documents", "finance", "comms", "petitions"] as const) {
      s = applyPage(s, k, done(k === "finance" ? [entry({ action: "them_du_an", subject: "DA-1" })] : []));
    }
    const html = view(s);
    for (const col of ["Thời điểm", "Phân hệ", "Người thực hiện", "Thao tác", "Đối tượng", "Địa chỉ IP", "Chi tiết"]) {
      expect(html).toContain(`<th scope="col">${col}</th>`);
    }
    expect(html).toContain("<td>Tài chính</td>");
    expect(html).toContain("them_du_an");
    expect(html).toContain("10:00:00 28/09/2026");
  });

  it("một phân hệ lỗi: các phân hệ khác VẪN hiện, và một dòng lỗi NÊU TÊN phân hệ ấy — không ghép im lặng", () => {
    let s = initialSources();
    s = applyPage(s, "identity", done([entry({ action: "dang_nhap" })]));
    s = applyPage(s, "documents", done([]));
    s = applyPage(s, "finance", { ok: false, thongBao: "Không kết nối được máy chủ. Vui lòng thử lại." });
    s = applyPage(s, "comms", done([]));
    s = applyPage(s, "petitions", done([]));
    const html = view(s);
    expect(html).toContain("dang_nhap");
    expect(html).toContain(
      "Không tải được nhật ký của phân hệ Tài chính: Không kết nối được máy chủ. Vui lòng thử lại.",
    );
    expect(html).toMatch(/role="alert"[^>]*>Không tải được nhật ký của phân hệ Tài chính/);
    expect(html).toContain("Tải lại");
  });

  it("công dân: 'Công dân' và IP không hiển thị; hệ thống: 'Hệ thống'", () => {
    let s = initialSources();
    for (const k of ["identity", "documents", "finance", "comms"] as const) s = applyPage(s, k, done([]));
    s = applyPage(
      s,
      "petitions",
      done([
        entry({ actor_kind: "citizen", actor_code: "", actor_ip: "", action: "gui_phan_anh" }),
        entry({ actor_kind: "system", actor_code: "system", actor_ip: "", at: "2026-09-28T02:00:00Z" }),
      ]),
    );
    const html = view(s);
    expect(html).toContain("<td>Công dân</td>");
    expect(html).toContain("<td>Không hiển thị</td>");
    expect(html).toContain("<td>Hệ thống</td>");
  });

  it("delta hiện là VĂN BẢN trong <details>, không bao giờ là HTML", () => {
    let s = initialSources();
    for (const k of ["identity", "documents", "finance", "comms"] as const) s = applyPage(s, k, done([]));
    s = applyPage(s, "petitions", done([entry({ delta: { after: "<img src=x onerror=alert(1)>" } })]));
    const html = view(s);
    expect(html).toContain("<details><summary>Xem</summary>");
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
  });

  it("mục bị giữ bởi ranh giới an toàn: nói ra, và có nút Xem thêm", () => {
    let s = initialSources();
    s = applyPage(s, "identity", {
      ok: true,
      duLieu: { items: [entry({ at: "2026-09-28T10:00:00Z", action: "i1" })], has_more: true, next_cursor: "c" },
    });
    s = applyPage(s, "documents", done([entry({ at: "2026-09-28T09:00:00Z", action: "d1" })]));
    for (const k of ["finance", "comms", "petitions"] as const) s = applyPage(s, k, done([]));
    const html = view(s);
    expect(html).toContain("i1");
    expect(html).not.toContain(">d1<");
    expect(html).toContain("Còn 1 mục đã tải đang chờ");
    expect(html).toContain("Xem thêm");
  });

  it("không mục nào và không còn gì để tải: câu rỗng, không có Xem thêm", () => {
    let s = initialSources();
    for (const k of ["identity", "documents", "finance", "comms", "petitions"] as const) s = applyPage(s, k, done([]));
    const html = view(s);
    expect(html).toContain("Không có mục nhật ký nào khớp bộ lọc.");
    expect(html).not.toContain("Xem thêm");
  });
});
