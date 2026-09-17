import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { afterEach, describe, expect, it, vi } from "vitest";

import { docDanhMucDanhBa, layDanhMucBoPhan, layDanhMucVaiTro } from "./danh-muc";

/** Một mục bộ phận đúng hình dạng hợp đồng. */
function boPhan(id: string, name: string) {
  return { id, code: name.toLowerCase().replace(/\s+/g, "-"), name, parent_id: "" };
}

function vaiTro(id: string, name: string, is_leader = false) {
  return { id, code: name.toLowerCase().replace(/\s+/g, "-"), name, is_leader };
}

function ok(than: unknown) {
  return new Response(JSON.stringify(than), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function loiCuaMayChu(status: number, code: string, message: string) {
  return new Response(JSON.stringify({ code, message, trace_id: "" }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** Trả lời theo đường dẫn — hai tuyến khác nhau, hai thân khác nhau. */
function batFetchTheoDuongDan(bang: Record<string, () => Response>) {
  const gia = vi.fn(async (duongDan: string) => {
    const tra = bang[duongDan];
    if (tra === undefined) throw new Error(`không mong đợi lời gọi tới ${duongDan}`);
    return tra();
  });
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("hai tuyến danh mục", () => {
  it("gọi đúng đường dẫn tương đối, không tenant_id, không host nào bị nung vào bundle", async () => {
    const gia = batFetchTheoDuongDan({
      "/api/v1/org-units": () => ok({ items: [] }),
      "/api/v1/roles": () => ok({ items: [] }),
    });

    await layDanhMucBoPhan();
    await layDanhMucVaiTro();

    const duongDan = gia.mock.calls.map(([d]) => String(d));
    expect(duongDan).toEqual(["/api/v1/org-units", "/api/v1/roles"]);
    expect(duongDan.join(" ")).not.toMatch(/^https?:|tenant/i);
  });

  it("401 hay 500 đều thành một câu của máy chủ, không rẽ nhánh theo `code`", async () => {
    batFetchTheoDuongDan({
      "/api/v1/org-units": () =>
        loiCuaMayChu(401, "unauthenticated", "Phiên làm việc đã hết hạn."),
    });

    expect(await layDanhMucBoPhan()).toEqual({
      ok: false,
      thongBao: "Phiên làm việc đã hết hạn.",
    });
  });
});

describe("docDanhMucDanhBa — đọc MỘT LẦN cho cả màn hình", () => {
  it("đúng HAI lời gọi, dù danh bạ có bao nhiêu dòng", async () => {
    const gia = batFetchTheoDuongDan({
      "/api/v1/org-units": () =>
        ok({ items: [boPhan("01J0BP", "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ")] }),
      "/api/v1/roles": () => ok({ items: [vaiTro("01J0VT", "Chủ tịch UBND", true)] }),
    });

    const dm = await docDanhMucDanhBa();

    // Hai danh mục là hai lời gọi. Mỗi dòng danh bạ tự gọi thì con số này đi lên theo dữ liệu —
    // đúng mẫu N+1 mà `skills/load-data-once` gọi là dạng 1.
    expect(gia).toHaveBeenCalledTimes(2);
    expect(dm.boPhan.ok && dm.boPhan.duLieu.items).toHaveLength(1);
    expect(dm.vaiTro.ok && dm.vaiTro.duLieu.items).toHaveLength(1);
  });

  it("một danh mục hỏng KHÔNG kéo danh mục kia im theo", async () => {
    batFetchTheoDuongDan({
      "/api/v1/org-units": () => ok({ items: [boPhan("01J0BP", "VĂN PHÒNG ĐẢNG ỦY")] }),
      "/api/v1/roles": () => loiCuaMayChu(500, "internal", "Đã xảy ra lỗi. Vui lòng thử lại."),
    });

    const dm = await docDanhMucDanhBa();

    expect(dm.boPhan.ok).toBe(true);
    expect(dm.vaiTro).toEqual({ ok: false, thongBao: "Đã xảy ra lỗi. Vui lòng thử lại." });
  });

  it("KHÔNG nhớ gì giữa hai lần đọc — danh mục của xã này không được hiện ở xã khác", async () => {
    // Hai lần đọc trong cùng một tiến trình, hai câu trả lời khác nhau: đúng hình dạng một tiến
    // trình phục vụ hai tên miền. Nếu có ngày ai đó thêm một biến ở mức module để "đỡ phải gọi
    // lại", test này đỏ — và nó phải đỏ: đó là tên bộ phận của một cơ quan nhà nước hiện trên
    // màn hình của cơ quan khác (luật 1, cấm #1).
    batFetchTheoDuongDan({
      "/api/v1/org-units": () => ok({ items: [boPhan("01J0XA", "VĂN PHÒNG ĐẢNG ỦY")] }),
      "/api/v1/roles": () => ok({ items: [vaiTro("01J0RA", "Kế toán")] }),
    });
    const xaA = await docDanhMucDanhBa();

    vi.unstubAllGlobals();
    batFetchTheoDuongDan({
      "/api/v1/org-units": () => ok({ items: [boPhan("01J0XB", "THƯỜNG TRỰC ĐẢNG UỶ")] }),
      "/api/v1/roles": () => ok({ items: [vaiTro("01J0RB", "Cán bộ một cửa")] }),
    });
    const xaB = await docDanhMucDanhBa();

    expect(xaA.boPhan.ok && xaA.boPhan.duLieu.items[0]?.name).toBe("VĂN PHÒNG ĐẢNG ỦY");
    expect(xaB.boPhan.ok && xaB.boPhan.duLieu.items[0]?.name).toBe("THƯỜNG TRỰC ĐẢNG UỶ");
    expect(xaA.vaiTro.ok && xaA.vaiTro.duLieu.items[0]?.name).toBe("Kế toán");
    expect(xaB.vaiTro.ok && xaB.vaiTro.duLieu.items[0]?.name).toBe("Cán bộ một cửa");
  });
});

describe("màn hình danh bạ đọc danh mục đúng một chỗ", () => {
  // Đọc thẳng mã nguồn, vì điều phải giữ ở đây không kiểm được bằng một test chức năng: một lời
  // gọi thêm trong thân `danhSach.map(...)` vẫn cho ra màn hình đúng y hệt — chỉ là hai mươi
  // lời gọi thay vì hai, và không ai thấy cho tới khi một xã có nhiều cán bộ.
  const NGUON = readFileSync(
    fileURLToPath(new URL("../../features/cau-hinh/danh-ba-can-bo.tsx", import.meta.url)),
    "utf8",
  );

  it("`docDanhMucDanhBa(` xuất hiện đúng MỘT lần trong màn hình danh bạ", () => {
    expect(NGUON.match(/docDanhMucDanhBa\(/g)).toHaveLength(1);
  });

  it("không dòng nào gọi thẳng một tuyến danh mục ngoài lời gọi chung ấy", () => {
    expect(NGUON).not.toMatch(/layDanhMucBoPhan\(|layDanhMucVaiTro\(/);
  });
});
