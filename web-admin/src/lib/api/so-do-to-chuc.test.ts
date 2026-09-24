import { afterEach, describe, expect, it, vi } from "vitest";

import { suaBoPhan, themBoPhan, type SuaBoPhanVao } from "./so-do-to-chuc";

/**
 * Hai tuyến ghi của Sơ đồ tổ chức. Tệp này canh những gì KHÔNG nhìn thấy trên một màn hình chạy tốt:
 * thân yêu cầu dựng từng trường, trường tuỳ chọn rỗng thì vắng, `code` không bao giờ đi lên trong
 * PATCH (máy chủ 400 cả yêu cầu — `service-identity/internal/http/bo_phan.go:208`), và "dời lên gốc"
 * là `parent_id: ""` chứ không phải vắng mặt.
 */

function phanHoi(status: number, than: unknown) {
  return new Response(JSON.stringify(than), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const DA_GHI = { id: "01JBP1", code: "van-phong", name: "VĂN PHÒNG", parent_id: "", order: 0 };

function batFetch(tra: () => Response) {
  const gia = vi.fn(async () => tra());
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof batFetch>, i = 0) {
  const [duongDan, init] = gia.mock.calls[i] as unknown as [string, RequestInit];
  const than = JSON.parse(String(init.body)) as Record<string, unknown>;
  return { duongDan, phuongThuc: init.method, header: new Headers(init.headers), than };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("themBoPhan — POST /api/v1/org-units", () => {
  it("gửi đúng bốn trường đã nhập, kèm Idempotency-Key", async () => {
    const gia = batFetch(() => phanHoi(201, DA_GHI));
    const kq = await themBoPhan(
      { name: "VĂN PHÒNG", parent_id: "01JCHA", order: 3, code: "van-phong" },
      "khoa-1",
    );

    expect(kq).toEqual({ ok: true, duLieu: DA_GHI });
    const g = loiGoi(gia);
    expect(g.duongDan).toBe("/api/v1/org-units");
    expect(g.phuongThuc).toBe("POST");
    expect(g.header.get("Idempotency-Key")).toBe("khoa-1");
    expect(g.than).toEqual({ name: "VĂN PHÒNG", parent_id: "01JCHA", order: 3, code: "van-phong" });
  });

  it("mã rỗng, cha rỗng, thứ tự vắng thì VẮNG MẶT — để máy chủ tự sinh mã và đặt ở gốc", async () => {
    const gia = batFetch(() => phanHoi(201, DA_GHI));
    await themBoPhan({ name: "VĂN PHÒNG", parent_id: "", code: "" }, "khoa-1");
    expect(Object.keys(loiGoi(gia).than)).toEqual(["name"]);
  });

  it("không trải đối tượng nguồn: trường lạ không đi lên", async () => {
    const gia = batFetch(() => phanHoi(201, DA_GHI));
    const nguon = { name: "A", staff_count: 4, id: "01JX" } as unknown as Parameters<typeof themBoPhan>[0];
    await themBoPhan(nguon, "khoa-1");
    expect(Object.keys(loiGoi(gia).than)).toEqual(["name"]);
  });

  it("409 mã đã dùng ra tới người dùng bằng CÂU của máy chủ", async () => {
    const cau = "Mã bộ phận này đã được dùng trong xã. Hãy chọn mã khác.";
    batFetch(() => phanHoi(409, { code: "org_unit_code_taken", message: cau, trace_id: "t" }));
    expect(await themBoPhan({ name: "A", code: "a" }, "k")).toEqual({ ok: false, thongBao: cau });
  });
});

describe("suaBoPhan — PATCH /api/v1/org-units/{id}", () => {
  it("chỉ gửi những trường được truyền vào, id đi vào đường dẫn", async () => {
    const gia = batFetch(() => phanHoi(200, DA_GHI));
    await suaBoPhan("01J/BP", { name: "TÊN MỚI" });
    const g = loiGoi(gia);
    expect(g.duongDan).toBe("/api/v1/org-units/01J%2FBP");
    expect(g.phuongThuc).toBe("PATCH");
    expect(g.than).toEqual({ name: "TÊN MỚI" });
    // Không có khoá chống trùng: hợp đồng không đòi cho PATCH.
    expect(g.header.get("Idempotency-Key")).toBeNull();
  });

  it("dời lên gốc gửi parent_id: \"\" — chuỗi rỗng KHÔNG bị bỏ", async () => {
    const gia = batFetch(() => phanHoi(200, DA_GHI));
    await suaBoPhan("01JBP1", { parent_id: "" });
    expect(loiGoi(gia).than).toEqual({ parent_id: "" });
  });

  it("`code` không bao giờ đi lên, kể cả khi bên gọi lách kiểu", async () => {
    const gia = batFetch(() => phanHoi(200, DA_GHI));
    const lach = { name: "A", code: "ma-moi" } as unknown as SuaBoPhanVao;
    await suaBoPhan("01JBP1", lach);
    expect(loiGoi(gia).than).not.toHaveProperty("code");
  });

  it("409 vòng lặp ra tới người dùng NGUYÊN VĂN", async () => {
    const cau = "Không thể dời một bộ phận vào dưới chính nó hay dưới một bộ phận con của nó.";
    batFetch(() => phanHoi(409, { code: "org_unit_cycle", message: cau, trace_id: "t" }));
    expect(await suaBoPhan("01JBP1", { parent_id: "01JCON" })).toEqual({ ok: false, thongBao: cau });
  });
});
