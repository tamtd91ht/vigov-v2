import { readFileSync } from "node:fs";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  duongDanSoBienBan,
  laySoBienBan,
  taoBienBan,
  themKetLuan,
  type TaoBienBanVao,
} from "./bien-ban";

/**
 * Ba tuyến của màn Biên bản họp.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM "SO VỚI HỢP ĐỒNG", và nó có mặt vì một khiếm khuyết đã
 * biết: `schema.gen.ts` đang LỆCH khỏi `kb/20-contracts/openapi.json`, nên hai kiểu thân yêu cầu
 * trong `bien-ban.ts` là bản chép tay tạm. Một bản chép tay không có ai canh là một bản chép tay
 * sẽ trôi mà không bài kiểm nào đỏ (luật 9, cấm #2) — nên ở đây nó ĐƯỢC canh, bằng chính tệp hợp
 * đồng, so trên KEY THẬT SỰ ĐI TRÊN DÂY chứ không so với một danh sách chép lần thứ hai.
 */

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)` không tham số
  // làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

function traJSON(ma: number, than: unknown): Response {
  return new Response(JSON.stringify(than), {
    status: ma,
    headers: { "Content-Type": "application/json" },
  });
}

function loiGoi(gia: ReturnType<typeof batFetch>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const THAN_DAY_DU: TaoBienBanVao = {
  title: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
  held_on: "2026-08-05",
  reference_no: "31/BB-UBND",
  location: "Phòng họp UBND xã",
  chaired_by: "",
  content: "Toàn văn biên bản.",
  attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"],
  conclusions: ["Giao bộ phận Địa chính rà soát tiến độ tuyến đường."],
};

describe("GET /api/v1/meetings — đường dẫn", () => {
  it("đường dẫn tương đối, không host, không `tenant_id`", async () => {
    const gia = batFetch(traJSON(200, { items: [], next_cursor: "", has_more: false }));

    await laySoBienBan({ limit: 10, cursor: null });
    const duong = String(loiGoi(gia, 0).duongDan);

    expect(duong.startsWith("/api/")).toBe(true);
    expect(duong).not.toMatch(/tenant/i);
    expect(duong).not.toMatch(/^https?:/);
  });

  it("trang đầu KHÔNG mang `cursor` rỗng — `cursor=` là 400 ở máy chủ", () => {
    expect(duongDanSoBienBan({ limit: 10, cursor: null })).toBe("/api/v1/meetings?limit=10");
    expect(duongDanSoBienBan({ limit: 10, cursor: "" })).toBe("/api/v1/meetings?limit=10");
  });

  it("trang sau mang đúng con trỏ máy chủ trả", () => {
    expect(duongDanSoBienBan({ limit: 10, cursor: "MOC-2" })).toBe(
      "/api/v1/meetings?limit=10&cursor=MOC-2",
    );
  });

  it("KHÔNG gửi `sort` hay `order` — mặc định của máy chủ đã là `created_at` giảm dần", () => {
    const duong = duongDanSoBienBan({ limit: 10, cursor: "MOC-2" });
    expect(duong).not.toContain("sort");
    expect(duong).not.toContain("order");
  });

  it("không đọc được thì trả một câu cho người dùng, không ném", async () => {
    batFetch(traJSON(403, { code: "forbidden", message: "Bạn không có quyền xem nhiệm vụ." }));

    const kq = await laySoBienBan({ limit: 10, cursor: null });
    expect(kq.ok).toBe(false);
    // NGUYÊN VĂN câu máy chủ: màn hình không dựng lại một câu 403 của riêng nó.
    if (!kq.ok) expect(kq.thongBao).toBe("Bạn không có quyền xem nhiệm vụ.");
  });
});

describe("POST /api/v1/meetings — nhập biên bản", () => {
  it("gửi POST tới đúng tuyến, 201 thì đọc thân thành tấm thẻ", async () => {
    const gia = batFetch(
      traJSON(201, { id: "01JBB", title: "x", conclusions: [], task_count: 0 }),
    );

    const kq = await taoBienBan(THAN_DAY_DU, "k-co-dinh");
    const { duongDan, tuyChon } = loiGoi(gia, 0);

    expect(duongDan).toBe("/api/v1/meetings");
    expect(tuyChon.method).toBe("POST");
    expect(kq.ok).toBe(true);
  });

  it("mang `Idempotency-Key` đúng khoá truyền vào, và lần gửi lại dùng LẠI khoá ấy", async () => {
    // Khoá do biểu mẫu giữ. Sinh khoá mới ở lần bấm lại là bỏ đúng cái chống trùng sinh ra để
    // chặn: lần gửi đầu CÓ THỂ đã ghi một biên bản vào sổ lưu trữ.
    const gia = batFetch(traJSON(500, { code: "internal", message: "Đã xảy ra lỗi." }));

    await taoBienBan(THAN_DAY_DU, "k-co-dinh");
    await taoBienBan(THAN_DAY_DU, "k-co-dinh");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-co-dinh");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-co-dinh");
  });

  it("KHÔNG gửi `created_by`, `attachments` hay `tenant_id`", async () => {
    const gia = batFetch(traJSON(201, {}));
    await taoBienBan(THAN_DAY_DU, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("created_by");
    expect(than).not.toHaveProperty("attachments");
    expect(than).not.toHaveProperty("tenant_id");
  });

  it("mã lỗi khác 201 thành một câu, không thành dữ liệu", async () => {
    batFetch(
      traJSON(400, {
        code: "invalid_request",
        message: "biên bản họp: thiếu ngày họp",
      }),
    );

    const kq = await taoBienBan(THAN_DAY_DU, "k");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("biên bản họp: thiếu ngày họp");
  });
});

describe("POST /api/v1/meetings/{id}/conclusions — thêm một kết luận", () => {
  it("mã hoá id vào đường dẫn thay vì ghép thẳng", async () => {
    const gia = batFetch(traJSON(201, { id: "01JKL", ordinal: 4, content: "x" }));

    await themKetLuan("01J BB/2026", { content: "Giao Tài chính đối chiếu số liệu." }, "k");
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/meetings/01J%20BB%2F2026/conclusions");
  });

  it("thân chỉ có `content` — KHÔNG có `ordinal`, vì số thứ tự là của máy chủ", async () => {
    // §7.2 nối tiếp số ĐÃ CẤP dưới khoá của biên bản. Một client tự khai số thứ tự là một client
    // ghi đè cách đánh số của một văn bản đã in.
    const gia = batFetch(traJSON(201, { id: "01JKL", ordinal: 4 }));
    await themKetLuan("01JBB", { content: "Giao Tài chính đối chiếu số liệu." }, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(Object.keys(than)).toEqual(["content"]);
  });

  it("mang `Idempotency-Key`", async () => {
    const gia = batFetch(traJSON(201, {}));
    await themKetLuan("01JBB", { content: "x" }, "k-cua-hang-nay");
    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-cua-hang-nay");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * SO VỚI HỢP ĐỒNG — đọc thẳng `kb/20-contracts/openapi.json`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

type LuocDo = {
  properties?: Record<string, unknown>;
  required?: string[];
};

type HopDong = {
  paths: Record<string, Record<string, { "x-vigov-permission"?: { key?: string } }>>;
  components: { schemas: Record<string, LuocDo> };
};

function hopDong(): HopDong {
  // Bốn cấp: `src/lib/api` → `src/lib` → `src` → `web-admin` → gốc kho.
  const duong = new URL("../../../../kb/20-contracts/openapi.json", import.meta.url);
  return JSON.parse(readFileSync(duong, "utf8")) as HopDong;
}

async function keyDaGui(chay: () => Promise<unknown>): Promise<string[]> {
  const gia = batFetch(traJSON(201, {}));
  await chay();
  const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
  return Object.keys(than).sort();
}

describe("thân yêu cầu khớp hợp đồng — canh bản chép tay của `bien-ban.ts`", () => {
  it("`POST /meetings` gửi ĐÚNG bộ trường của `petitions.taoBienBanVao`", async () => {
    const luocDo = hopDong().components.schemas["petitions.taoBienBanVao"];
    expect(luocDo).toBeDefined();

    const cuaHopDong = Object.keys(luocDo?.properties ?? {}).sort();
    // Thân đầy đủ: mọi trường tuỳ chọn đều có giá trị, nên bộ key gửi đi phải trùng KHÍT bộ key
    // của hợp đồng. Thừa một trường là gửi thứ máy chủ không nhận; thiếu một trường là một ô
    // biểu mẫu không bao giờ tới nơi.
    expect(await keyDaGui(() => taoBienBan(THAN_DAY_DU, "k"))).toEqual(cuaHopDong);
  });

  it("`POST …/conclusions` gửi ĐÚNG bộ trường của `petitions.themKetLuanVao`", async () => {
    const luocDo = hopDong().components.schemas["petitions.themKetLuanVao"];
    expect(luocDo).toBeDefined();

    expect(await keyDaGui(() => themKetLuan("01JBB", { content: "x" }, "k"))).toEqual(
      Object.keys(luocDo?.properties ?? {}).sort(),
    );
  });

  it("ba tuyến vẫn đứng sau `task.read` / `task.create` như màn đang giả định", () => {
    // MÀN NÀY KHÔNG CÓ CỔNG QUYỀN Ở CLIENT (xem `PHAN_CHUA_DUNG`), nên không có hằng nào trong
    // `lib/quyen.ts` để canh. Bài kiểm này là chỗ duy nhất phát hiện được ngày máy chủ tách một
    // khoá `meeting.*` riêng — hôm ấy `PHAN_CHUA_DUNG` và thanh menu đều phải sửa theo.
    const p = hopDong().paths;
    expect(p["/api/v1/meetings"]?.["get"]?.["x-vigov-permission"]?.key).toBe("task.read");
    expect(p["/api/v1/meetings"]?.["post"]?.["x-vigov-permission"]?.key).toBe("task.create");
    expect(
      p["/api/v1/meetings/{id}/conclusions"]?.["post"]?.["x-vigov-permission"]?.key,
    ).toBe("task.create");
  });
});
