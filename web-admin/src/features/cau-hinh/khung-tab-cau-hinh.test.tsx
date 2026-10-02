import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

let phienGia: PhienDaDoc = null;

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => phienGia,
}));

// The real page wraps this frame in `CauHinhXaProvider`; the mail-server tab reads the commune
// name from it (sender-name placeholder, ADR 0068 §13).
vi.mock("@/components/cau-hinh-xa", () => ({
  useCauHinhXa: () => ({ displayName: "UBND xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai", logoUrl: "", webAdminBannerUrl: "" }),
}));

// The Nhận diện xã tab refreshes the server-rendered shell after a change; no app router in a test.
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: () => {} }) }));

const { KhungTabCauHinh } = await import("./khung-tab-cau-hinh");

function phienCo(quyen: readonly string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: [...quyen],
    },
  } as PhienDaDoc;
}

const soNutTab = (html: string) => (html.match(/role="tab"/g) ?? []).length;

afterEach(() => {
  phienGia = null;
});

describe("khung tab màn Cấu hình", () => {
  it("chưa đọc xong phiên → không có thanh tab, tab đầu hiện, các panel khác giữ nhưng ẩn", () => {
    phienGia = null;
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).not.toContain('role="tablist"');
    expect(html).toMatch(/<div id="panel-cau-hinh-so-do-to-chuc" class="panel-cau-hinh">/);
    expect(html).toMatch(/<div id="panel-cau-hinh-danh-muc" class="panel-cau-hinh" hidden="">/);
    expect(html).not.toContain("panel-cau-hinh-nguoi-dung");
  });

  it("đủ quyền → thanh mười tab; tab đầu được chọn và là tab duy nhất có tabindex=0", () => {
    phienGia = phienCo(["admin.user", "admin.role", "asset.read", "admin.lookup", "admin.audit"]);
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).toContain('role="tablist"');
    expect(soNutTab(html)).toBe(10);
    expect(html).toContain('aria-controls="panel-cau-hinh-loi-he-thong"');
    expect(html).toContain('aria-controls="panel-cau-hinh-nhat-ky-he-thong"');
    expect(html).toContain('aria-controls="panel-cau-hinh-truong-ban-do"');
    expect(html).toContain('aria-controls="panel-cau-hinh-may-chu-thu"');
    expect((html.match(/aria-selected="true"/g) ?? []).length).toBe(1);
    expect((html.match(/role="tab"[^>]*tabindex="0"/g) ?? []).length).toBe(1);
    expect(html).toContain('aria-controls="panel-cau-hinh-phan-quyen"');
    expect(html).toContain('role="tabpanel"');
  });

  it("CA BỊ TỪ CHỐI: thiếu `admin.user` và `admin.role` → không nút, không panel Người dùng / Phân quyền", () => {
    phienGia = phienCo(["task.read"]);
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(soNutTab(html)).toBe(4);
    expect(html).not.toContain(">Người dùng</button>");
    expect(html).not.toContain(">Phân quyền</button>");
    expect(html).not.toContain("panel-cau-hinh-nguoi-dung");
    expect(html).not.toContain("panel-cau-hinh-phan-quyen");
    expect(html).not.toContain("panel-cau-hinh-truong-ban-do");
    expect(html).not.toContain("panel-cau-hinh-may-chu-thu");
    expect(html).not.toContain("panel-cau-hinh-loi-he-thong");
    expect(html).not.toContain("panel-cau-hinh-nhat-ky-he-thong");
  });

  it("CA BỊ TỪ CHỐI: `admin.lookup` mà thiếu `admin.audit` → có Lời hệ thống, không có Nhật ký", () => {
    phienGia = phienCo(["admin.lookup"]);
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).toContain(">Lời hệ thống</button>");
    expect(html).not.toContain(">Nhật ký hệ thống</button>");
    expect(html).not.toContain("panel-cau-hinh-nhat-ky-he-thong");
  });

  it("CA BỊ TỪ CHỐI: `admin.lookup` mà thiếu `asset.read` → có Máy chủ thư, không có Trường bản đồ", () => {
    phienGia = phienCo(["admin.lookup"]);
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).toContain(">Máy chủ thư</button>");
    expect(html).not.toContain(">Trường bản đồ</button>");
    expect(html).not.toContain("panel-cau-hinh-truong-ban-do");
  });

  it("`admin.org` → tab Nhận diện xã có nút và panel (ADR 0069 #2)", () => {
    phienGia = phienCo(["admin.org"]);
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).toContain(">Nhận diện xã</button>");
    expect(html).toContain('id="panel-cau-hinh-nhan-dien-xa"');
  });

  it("CA BỊ TỪ CHỐI: thiếu `admin.org` (dù có mọi khoá admin.* khác) → không nút, không panel Nhận diện xã", () => {
    phienGia = phienCo(["admin.user", "admin.role", "admin.lookup", "admin.sla", "admin.audit", "asset.read", "admin.orgs"]);
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).not.toContain(">Nhận diện xã</button>");
    expect(html).not.toContain("panel-cau-hinh-nhan-dien-xa");
  });

  it("phiên đọc hỏng → câu của máy chủ vẫn ra tới màn hình", () => {
    phienGia = { ok: false, thongBao: "Phiên làm việc đã hết hạn" } as PhienDaDoc;
    const html = renderToStaticMarkup(<KhungTabCauHinh />);
    expect(html).toContain("Phiên làm việc đã hết hạn");
    expect(html).not.toContain("panel-cau-hinh-nguoi-dung");
  });
});
