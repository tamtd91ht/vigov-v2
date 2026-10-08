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
  it("đủ khoá cổng (trừ `admin.org`) → mười một tab theo thứ tự ADR 0079 D1: Lịch làm việc ngay sau Thời hạn xử lý, Kênh Zalo sau Máy chủ thư, Nhật ký hệ thống cuối", async () => {
    const phien = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "asset.read", "admin.lookup", "admin.sla", "admin.audit"]),
    );
    const hien = cacTabHien(TAB_CAU_HINH, phien);
    expect(nhan(hien)).toEqual([
      "Sơ đồ tổ chức",
      "Thôn / Tổ dân phố",
      "Danh mục",
      "Trường bản đồ",
      "Lời hệ thống",
      "Thời hạn xử lý",
      "Lịch làm việc",
      "Tự động hoá",
      "Máy chủ thư",
      "Kênh Zalo",
      "Nhật ký hệ thống",
    ]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("Thời hạn xử lý đi theo `admin.sla` (ADR 0079 D1) — ca bị từ chối trước; Lịch làm việc thì không cổng", async () => {
    // `GET /api/v1/sla` declares `admin.sla`: without it the tab's one read is a 403, so the tab hides.
    // The three calendar reads are any-authenticated, so "Lịch làm việc" shows to every account.
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "asset.read", "admin.lookup", "admin.audit", "admin.org", "admin.slas", "ADMIN.SLA"]),
    );
    const deniedTabs = nhan(cacTabHien(TAB_CAU_HINH, denied));
    expect(deniedTabs).not.toContain("Thời hạn xử lý");
    expect(deniedTabs).toContain("Lịch làm việc");
    const holder = await phienVoi(phanHoiPhien(["admin.sla"]));
    const holderTabs = nhan(cacTabHien(TAB_CAU_HINH, holder));
    expect(holderTabs).toContain("Thời hạn xử lý");
    expect(holderTabs).toContain("Lịch làm việc");
    expect(TAB_CAU_HINH.find((t) => t.ma === "lich-lam-viec")?.cong).toBeNull();
  });

  it("Kênh Zalo đi theo `admin.lookup` (ADR 0074 #6) — ca bị từ chối trước", async () => {
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "asset.read", "admin.sla", "admin.audit", "admin.org", "admin.lookups", "ADMIN.LOOKUP"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Kênh Zalo");
    const holder = await phienVoi(phanHoiPhien(["admin.lookup"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Kênh Zalo");
  });

  it("Tự động hoá đi theo `admin.sla` — khoá của cả ba tuyến automation-jobs; ca bị từ chối", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.sla"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Tự động hoá");
    // `admin.lookup` / `admin.org` / a prefix look-alike do not open it (rule 5, invariant 3b).
    const denied = await phienVoi(phanHoiPhien(["admin.lookup", "admin.org", "admin.audit", "admin.slas", "ADMIN.SLA"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Tự động hoá");
  });

  it("Nhận diện xã đi theo `admin.org` — khoá của cả bảy tuyến commune-branding; ca bị từ chối", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.org"]));
    const shown = nhan(cacTabHien(TAB_CAU_HINH, holder));
    expect(shown).toContain("Nhận diện xã");
    expect(shown[shown.length - 1]).toBe("Nhận diện xã");
    // Other admin keys and look-alikes do not open it (rule 5, invariant 3b).
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "admin.lookup", "admin.sla", "admin.audit", "admin.orgs", "ADMIN.ORG"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Nhận diện xã");
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

  it("Lời hệ thống đi theo `admin.lookup` — khoá của cả sáu tuyến system-messages", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.lookup"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Lời hệ thống");
    const denied = await phienVoi(phanHoiPhien(["admin.audit", "admin.org", "admin.sla", "asset.read"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Lời hệ thống");
  });

  it("Nhật ký hệ thống đi theo `admin.audit`, và chỉ khoá ấy — ca bị từ chối", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.audit"]));
    const shown = nhan(cacTabHien(TAB_CAU_HINH, holder));
    expect(shown).toContain("Nhật ký hệ thống");
    // `admin.audit` không mở tab nào khác có cổng (luật 5, bất biến 3b).
    expect(shown).not.toContain("Người dùng");
    expect(shown).not.toContain("Lời hệ thống");
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "admin.lookup", "admin.audits", "ADMIN.AUDIT"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Nhật ký hệ thống");
  });

  it("thiếu mọi khoá cổng → bốn tab; Người dùng, Phân quyền, Trường bản đồ, Máy chủ thư, Thời hạn xử lý ẩn", async () => {
    const phien = await phienVoi(phanHoiPhien(["task.read"]));
    const hien = cacTabHien(TAB_CAU_HINH, phien);
    expect(nhan(hien)).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục", "Lịch làm việc"]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("Người dùng và Phân quyền không còn trong danh sách tab (05/10/2026 → /nguoi-dung)", () => {
    // Hai cổng `quyetDinhTabNguoiDung` / `quyetDinhTabPhanQuyen` vẫn sống — ở màn mới
    // (`tab-nguoi-dung.tsx`, `tab-phan-quyen.tsx`) — nhưng không tab nào ở đây gọi chúng nữa.
    expect(nhan(TAB_CAU_HINH)).not.toContain("Người dùng");
    expect(nhan(TAB_CAU_HINH)).not.toContain("Phân quyền");
  });

  it("KHÔNG thêm cổng cho bốn tab đọc mở: không khoá nào thì bốn tab ấy vẫn hiện", async () => {
    // Tuyến đọc của bốn tab này mở cho mọi tài khoản đã đăng nhập (ba bảng lịch: any-authenticated).
    // Giao diện ẩn chúng là giao diện từ chối điều máy chủ không từ chối — luật 5, cấm #1.
    const khong = await phienVoi(phanHoiPhien([]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, khong))).toEqual([
      "Sơ đồ tổ chức",
      "Thôn / Tổ dân phố",
      "Danh mục",
      "Lịch làm việc",
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
    expect(hien).not.toContain("Kênh Zalo");
    expect(hien).not.toContain("Lời hệ thống");
    expect(hien).not.toContain("Tự động hoá");
    expect(hien).not.toContain("Thời hạn xử lý");
    expect(hien).not.toContain("Nhật ký hệ thống");
    expect(hien).not.toContain("Nhận diện xã");
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
