import { afterEach, describe, expect, it, vi } from "vitest";

import { layPhienHienTai } from "@/lib/api/phien";

import { quyetDinhTabNguoiDung } from "./quyen-tab";
import {
  cacTabHien,
  coThanhTab,
  SYSTEM_TABS,
  systemTabFromQuery,
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

const nhan = (ds: readonly MoTaTab<string>[]) => ds.map((t) => t.nhan);

/** The prototype's nine (`ConfigWorkspace.tsx:61-73`), in its order — owner 12/10/2026, S27. */
const PROTOTYPE_NINE = [
  "Sơ đồ tổ chức",
  "Thôn / Tổ dân phố",
  "Danh mục",
  "Trường bản đồ",
  "Lời hệ thống",
  "Thời hạn xử lý",
  "Tự động hoá",
  "Máy chủ thư",
  "Kênh Zalo",
];

/**
 * The bar: the prototype's nine, then "Kênh SMS" (ADR 0099 #2 #3, owner 17/10/2026) right after "Kênh Zalo",
 * then "Thuộc tính danh bạ" (ADR 0101 #5, owner 17/10/2026).
 */
const CONFIG_TABS = [...PROTOTYPE_NINE, "Kênh SMS", "Thuộc tính danh bạ"];

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("tab nào của màn Cấu hình được hiện", () => {
  it("mọi khoá → chín tab của prototype rồi Kênh SMS, Thuộc tính danh bạ, đúng thứ tự — không còn ba tab dư (chủ dự án 12/10/2026, S27)", async () => {
    const phien = await phienVoi(
      phanHoiPhien([
        "admin.user",
        "admin.role",
        "asset.read",
        "admin.lookup",
        "admin.sla",
        "admin.audit",
        "admin.org",
        "admin.message",
        "admin.mail",
        "admin.zalo",
        "admin.sms",
        "admin.citizen_field",
      ]),
    );
    const hien = cacTabHien(TAB_CAU_HINH, phien);
    expect(nhan(hien)).toEqual(CONFIG_TABS);
    expect(nhan(TAB_CAU_HINH)).toEqual(CONFIG_TABS);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("ba tab dư KHÔNG còn ở /cau-hinh, và mỗi tab chỉ ở một nơi", () => {
    const here = TAB_CAU_HINH.map((t) => t.ma as string);
    for (const id of ["lich-lam-viec", "nhat-ky-he-thong", "nhan-dien-xa"]) expect(here).not.toContain(id);
    for (const t of SYSTEM_TABS) expect(here).not.toContain(t.ma);
  });

  it("Thời hạn xử lý đi theo `admin.sla` (ADR 0079 D1) — ca bị từ chối trước", async () => {
    // `GET /api/v1/sla` declares `admin.sla`: without it the tab's one read is a 403, so the tab hides.
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "asset.read", "admin.lookup", "admin.audit", "admin.org", "admin.slas", "ADMIN.SLA"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Thời hạn xử lý");
    const holder = await phienVoi(phanHoiPhien(["admin.sla"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Thời hạn xử lý");
  });

  it("Kênh Zalo đi theo `admin.zalo` (chủ dự án 15/10/2026) — ca bị từ chối trước, kể cả `admin.lookup`", async () => {
    const denied = await phienVoi(
      phanHoiPhien([
        "admin.user",
        "admin.role",
        "asset.read",
        "admin.sla",
        "admin.audit",
        "admin.org",
        "admin.lookup",
        "admin.mail",
        "admin.message",
        "admin.village",
        "admin.zalos",
        "ADMIN.ZALO_CHANNEL",
      ]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Kênh Zalo");
    const holder = await phienVoi(phanHoiPhien(["admin.zalo"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Kênh Zalo");
  });

  it("Kênh SMS đi theo `admin.sms` (ADR 0099 #3) — ca bị từ chối trước, kể cả `admin.zalo`", async () => {
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.lookup", "admin.mail", "admin.zalo", "admin.message", "admin.smss", "ADMIN.SMS"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Kênh SMS");
    const holder = await phienVoi(phanHoiPhien(["admin.sms"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục", "Kênh SMS"]);
  });

  it("Thuộc tính danh bạ đi theo `admin.citizen_field` (ADR 0101 #5) — ca bị từ chối trước: `citizen.read`, `admin.zalo` không mở", async () => {
    const denied = await phienVoi(
      phanHoiPhien(["citizen.read", "citizen.update", "admin.zalo", "admin.sms", "admin.lookup", "admin.citizen_fields", "ADMIN.CITIZEN_FIELD"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Thuộc tính danh bạ");
    const holder = await phienVoi(phanHoiPhien(["admin.citizen_field"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục", "Thuộc tính danh bạ"]);
  });

  it("Tự động hoá đi theo `admin.sla` — khoá của cả ba tuyến automation-jobs; ca bị từ chối", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.sla"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Tự động hoá");
    // `admin.lookup` / `admin.org` / a prefix look-alike do not open it (rule 5, invariant 3b).
    const denied = await phienVoi(phanHoiPhien(["admin.lookup", "admin.org", "admin.audit", "admin.slas", "ADMIN.SLA"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Tự động hoá");
  });


  it("Trường bản đồ đi theo `asset.read` (khoá ĐỌC), không theo `admin.lookup`", async () => {
    // `admin.lookup` là khoá GHI của tab ấy: có nó mà không có `asset.read` thì GET vẫn 403.
    const writeOnly = await phienVoi(phanHoiPhien(["admin.lookup"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, writeOnly))).not.toContain("Trường bản đồ");
    const readOnly = await phienVoi(phanHoiPhien(["asset.read"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, readOnly))).toContain("Trường bản đồ");
  });

  it("Máy chủ thư đi theo `admin.mail` (15/10/2026) — ca bị từ chối: `admin.lookup` không còn mở", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.mail"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Máy chủ thư");
    const denied = await phienVoi(
      phanHoiPhien(["asset.read", "admin.org", "admin.sla", "admin.lookup", "admin.zalo", "admin.mails"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Máy chủ thư");
  });

  it("Lời hệ thống đi theo `admin.message` (15/10/2026) — ca bị từ chối: `admin.lookup` không còn mở", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.message"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toContain("Lời hệ thống");
    const denied = await phienVoi(
      phanHoiPhien(["admin.audit", "admin.org", "admin.sla", "asset.read", "admin.lookup", "admin.mail"]),
    );
    expect(nhan(cacTabHien(TAB_CAU_HINH, denied))).not.toContain("Lời hệ thống");
  });

  it("Thôn / Tổ dân phố vẫn không cổng: `admin.village` chỉ là khoá nút ghi trong tab", () => {
    expect(TAB_CAU_HINH.find((t) => t.ma === "thon-to-dan-pho")?.cong).toBeNull();
  });

  it("`admin.audit` không mở tab có cổng nào ở /cau-hinh (luật 5, bất biến 3b)", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.audit"]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, holder))).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục"]);
  });

  it("thiếu mọi khoá cổng → ba tab đọc mở; mọi tab có cổng ẩn", async () => {
    const phien = await phienVoi(phanHoiPhien(["task.read"]));
    const hien = cacTabHien(TAB_CAU_HINH, phien);
    expect(nhan(hien)).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục"]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("Người dùng và Phân quyền không còn trong danh sách tab (05/10/2026 → /nguoi-dung)", () => {
    // Hai cổng `quyetDinhTabNguoiDung` / `quyetDinhTabPhanQuyen` vẫn sống — ở màn mới
    // (`tab-nguoi-dung.tsx`, `tab-phan-quyen.tsx`) — nhưng không tab nào ở đây gọi chúng nữa.
    expect(nhan(TAB_CAU_HINH)).not.toContain("Người dùng");
    expect(nhan(TAB_CAU_HINH)).not.toContain("Phân quyền");
  });

  it("KHÔNG thêm cổng cho ba tab đọc mở: không khoá nào thì ba tab ấy vẫn hiện", async () => {
    // Tuyến đọc của ba tab này mở cho mọi tài khoản đã đăng nhập.
    // Giao diện ẩn chúng là giao diện từ chối điều máy chủ không từ chối — luật 5, cấm #1.
    const khong = await phienVoi(phanHoiPhien([]));
    expect(nhan(cacTabHien(TAB_CAU_HINH, khong))).toEqual(["Sơ đồ tổ chức", "Thôn / Tổ dân phố", "Danh mục"]);
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
    expect(hien).not.toContain("Kênh SMS");
    expect(hien).not.toContain("Thuộc tính danh bạ");
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

describe("tab nào của màn Hệ thống được hiện (chủ dự án 12/10/2026, S27)", () => {
  it("mọi khoá → ba tab, đúng thứ tự: Lịch làm việc · Nhật ký hệ thống · Nhận diện xã", async () => {
    const phien = await phienVoi(phanHoiPhien(["admin.sla", "admin.audit", "admin.org"]));
    const hien = cacTabHien(SYSTEM_TABS, phien);
    expect(nhan(hien)).toEqual(["Lịch làm việc", "Nhật ký hệ thống", "Nhận diện xã"]);
    expect(coThanhTab(phien, hien.length)).toBe(true);
  });

  it("Lịch làm việc không cổng — ba tuyến đọc lịch any-authenticated; cổng giữ nguyên khi chuyển màn", async () => {
    const none = await phienVoi(phanHoiPhien([]));
    expect(nhan(cacTabHien(SYSTEM_TABS, none))).toEqual(["Lịch làm việc"]);
    expect(SYSTEM_TABS.find((t) => t.ma === "lich-lam-viec")?.cong).toBeNull();
  });

  it("Nhật ký hệ thống đi theo `admin.audit`, và chỉ khoá ấy — ca bị từ chối", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.audit"]));
    expect(nhan(cacTabHien(SYSTEM_TABS, holder))).toEqual(["Lịch làm việc", "Nhật ký hệ thống"]);
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "admin.lookup", "admin.sla", "admin.org", "admin.audits", "ADMIN.AUDIT"]),
    );
    expect(nhan(cacTabHien(SYSTEM_TABS, denied))).not.toContain("Nhật ký hệ thống");
  });

  it("Nhận diện xã đi theo `admin.org` — khoá của cả bảy tuyến commune-branding; ca bị từ chối", async () => {
    const holder = await phienVoi(phanHoiPhien(["admin.org"]));
    expect(nhan(cacTabHien(SYSTEM_TABS, holder))).toEqual(["Lịch làm việc", "Nhận diện xã"]);
    // Other admin keys and look-alikes do not open it (rule 5, invariant 3b).
    const denied = await phienVoi(
      phanHoiPhien(["admin.user", "admin.role", "admin.lookup", "admin.sla", "admin.audit", "admin.orgs", "ADMIN.ORG"]),
    );
    expect(nhan(cacTabHien(SYSTEM_TABS, denied))).not.toContain("Nhận diện xã");
  });

  it("chỉ `admin.sla` → một tab (Lịch làm việc) → không dựng thanh (quyết định 26/09)", async () => {
    const phien = await phienVoi(phanHoiPhien(["admin.sla"]));
    const hien = cacTabHien(SYSTEM_TABS, phien);
    expect(nhan(hien)).toEqual(["Lịch làm việc"]);
    expect(coThanhTab(phien, hien.length)).toBe(false);
  });

  it("phiên hết hạn (401) → đóng khi không chắc: hai tab có cổng ẩn", async () => {
    const phien = await phienVoi(
      new Response(JSON.stringify({ code: "unauthenticated", message: "Phiên đã hết hạn" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      }),
    );
    expect(nhan(cacTabHien(SYSTEM_TABS, phien))).toEqual(["Lịch làm việc"]);
  });

  it("?tab= chỉ chọn tab đầu khi khớp đúng một mã của màn này", () => {
    expect(systemTabFromQuery("lich-lam-viec")).toBe("lich-lam-viec");
    expect(systemTabFromQuery("nhat-ky-he-thong")).toBe("nhat-ky-he-thong");
    expect(systemTabFromQuery("nhan-dien-xa")).toBe("nhan-dien-xa");
    for (const v of [undefined, "", "danh-muc", "Nhan-Dien-Xa", "constructor", "__proto__", ["nhan-dien-xa"]]) {
      expect(systemTabFromQuery(v)).toBeNull();
    }
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
