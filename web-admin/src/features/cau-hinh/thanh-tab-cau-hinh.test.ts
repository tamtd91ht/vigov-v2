import { afterEach, describe, expect, it, vi } from "vitest";

import { layPhienHienTai } from "@/lib/api/phien";

import { quyetDinhTabNguoiDung } from "./quyen-tab";
import {
  cacTabHien,
  coThanhTab,
  TAB_CAU_HINH,
  tabKeTheoPhim,
  type MoTaTab,
} from "./thanh-tab-cau-hinh";

/**
 * Phiên đi qua đúng đường thật — phản hồi HTTP → `layPhienHienTai` — như `quyen-tab.test.ts`: ca
 * phiên hết hạn đến từ một phản hồi 401, không từ một `KetQua` gõ tay.
 */
function phanHoiPhien(quyen: readonly string[]) {
  return new Response(
    JSON.stringify({
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: quyen,
    }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

async function phienVoi(tra: Response) {
  vi.stubGlobal("fetch", vi.fn(async () => tra));
  return layPhienHienTai();
}

const nhan = (ds: readonly MoTaTab[]) => ds.map((t) => t.nhan);

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("tab nào của màn Cấu hình được hiện", () => {
  it("đủ bốn khoá cổng → tám tab, đúng thứ tự và nhãn của §0", async () => {
    const phien = await phienVoi(phanHoiPhien(["admin.user", "admin.role", "asset.read", "admin.lookup"]));
    const hien = cacTabHien(TAB_CAU_HINH, phien);
    expect(nhan(hien)).toEqual([
      "Sơ đồ tổ chức",
      "Thôn / Tổ dân phố",
      "Người dùng",
      "Phân quyền",
      "Danh mục",
      "Trường bản đồ",
      "Thời hạn xử lý",
      "Máy chủ thư",
    ]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("Trường bản đồ đi theo `asset.read` (khoá ĐỌC), không theo `admin.lookup`", async () => {
    // `admin.lookup` là khoá GHI của tab ấy: có nó mà không có `asset.read` thì GET vẫn 403.
    const writeOnly = await phienVoi(phanHoiPhien(["admin.lookup"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, writeOnly))).not.toContain("Trường bản đồ");
    const readOnly = await phienVoi(phanHoiPhien(["asset.read"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, readOnly))).toContain("Trường bản đồ");
  });

  it("Máy chủ thư đi theo `admin.lookup` — khoá của cả ba tuyến mail-settings", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.lookup"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Máy chủ thư");
    const denied = await phienVoi(phanHoiPhien(["asset.read", "admin.org", "admin.sla"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Máy chủ thư");
  });

  it("thiếu mọi khoá cổng → bốn tab; Người dùng, Phân quyền, Trường bản đồ, Máy chủ thư ẩn", async () => {
    const phien = await phienVoi(phanHoiPhien(["task.read"]));
    const hien = cacTabHien(TAB_CAU_HINH, phien);
    expect(nhan(hien)).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục", "Thời hạn xử lý"]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("chỉ `admin.role` → Phân quyền hiện, Người dùng ẩn — mỗi tab đúng một khoá", async () => {
    const phien = await phienVoi(phanHoiPhien(["admin.role"]));
    const hien = nhan(cacTabHien(TAB_CAU_HINH, phien));
    expect(hien).toContain("Phân quyền");
    expect(hien).not.toContain("Người dùng");
  });

  it("KHÔNG thêm cổng cho bốn tab đọc mở: `admin.org` / `admin.sla` không quyết định tab nào hiện", async () => {
    // Tuyến đọc của bốn tab này mở cho mọi tài khoản đã đăng nhập (GET /sla thì máy chủ tự 403).
    // Giao diện ẩn chúng là giao diện từ chối điều máy chủ không từ chối — luật 5, cấm #1.
    const khong = await phienVoi(phanHoiPhien([]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, khong))).toEqual([
      "Sơ đồ tổ chức",
      "Thôn / Tổ dân phố",
      "Danh mục",
      "Thời hạn xử lý",
    ]);
  });

  it("phiên hết hạn (401) → đóng khi không chắc: mọi tab có cổng ẩn", async () => {
    const phien = await phienVoi(
      new Response(JSON.stringify({ code: "unauthenticated", message: "Phiên đã hết hạn" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      }),
    );
    expect(phien.ok).toBe(false);
    const hien = nhan(cacTabHien(TAB_CAU_HINH, phien));
    expect(hien).not.toContain("Người dùng");
    expect(hien).not.toContain("Phân quyền");
    expect(hien).not.toContain("Trường bản đồ");
    expect(hien).not.toContain("Máy chủ thư");
  });

  it("chưa đọc xong phiên → tab có cổng chưa hiện, và CHƯA dựng thanh", () => {
    const hien = cacTabHien(TAB_CAU_HINH, null);
    expect(nhan(hien)).not.toContain("Người dùng");
    expect(nhan(hien)).not.toContain("Phân quyền");
    expect(coThanhTab(null, hien.length)).toBe(false);
  });
});

describe("có thanh tab hay không (quyết định 26/09)", () => {
  // Với sáu tab hôm nay, bốn tab không cổng nên không có ca một tab. Dựng ca ấy bằng một danh sách
  // khác qua CHÍNH hàm quyết định — để ngày tab thứ bảy (hoặc một cổng mới) làm ra ca này, luật đã
  // có bài kiểm.
  const MOT_KHONG_CONG_MOT_CO_CONG: readonly MoTaTab<"a" | "b">[] = [
    { ma: "a", nhan: "Tab A", cong: null },
    { ma: "b", nhan: "Tab B", cong: quyetDinhTabNguoiDung },
  ];

  it("chỉ mở được MỘT tab → không có thanh", async () => {
    const phien = await phienVoi(phanHoiPhien(["task.read"]));
    const hien = cacTabHien(MOT_KHONG_CONG_MOT_CO_CONG, phien);
    expect(hien.map((t) => t.ma)).toEqual(["a"]);
    expect(coThanhTab(phien, hien.length)).toBe(false);
  });

  it("mở được HAI tab → có thanh", async () => {
    const phien = await phienVoi(phanHoiPhien(["admin.user"]));
    const hien = cacTabHien(MOT_KHONG_CONG_MOT_CO_CONG, phien);
    expect(hien.map((t) => t.ma)).toEqual(["a", "b"]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });
});

describe("phím trên thanh tab (mẫu tabs WAI-ARIA)", () => {
  it("← / → đi vòng; Home / End về hai đầu", () => {
    expect(tabKeTheoPhim("ArrowRight", 0, 6)).toBe(1);
    expect(tabKeTheoPhim("ArrowRight", 5, 6)).toBe(0);
    expect(tabKeTheoPhim("ArrowLeft", 0, 6)).toBe(5);
    expect(tabKeTheoPhim("ArrowLeft", 3, 6)).toBe(2);
    expect(tabKeTheoPhim("Home", 4, 6)).toBe(0);
    expect(tabKeTheoPhim("End", 1, 6)).toBe(5);
  });

  it("phím khác (Tab, Enter) để trình duyệt xử lý", () => {
    expect(tabKeTheoPhim("Tab", 2, 6)).toBeNull();
    expect(tabKeTheoPhim("Enter", 2, 6)).toBeNull();
    expect(tabKeTheoPhim("ArrowRight", 0, 0)).toBeNull();
  });
});
