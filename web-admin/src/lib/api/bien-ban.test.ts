import { readFileSync } from "node:fs";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  duongDanSoBienBan,
  laySoBienBan,
  tachKetLuanThanhNhiemVu,
  taoBienBan,
  themKetLuan,
  type TachKetLuanVao,
  type TaoBienBanVao,
} from "./bien-ban";
import type { petitions_ketLuanRa } from "./schema.gen";

/**
 * Bốn tuyến của màn Biên bản họp.
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

describe("POST …/conclusions/{stt}/task — tách một kết luận thành nhiệm vụ (§3)", () => {
  function ketLuan(sua: Partial<petitions_ketLuanRa> = {}): petitions_ketLuanRa {
    return {
      id: "01JKL3",
      ordinal: 3,
      content: "Giao Tài chính – Kế toán đối chiếu số liệu giải ngân sáu tháng đầu năm.",
      task_count: 0,
      task_done_count: 0,
      created_at: "2026-08-05T02:00:00Z",
      ...sua,
    };
  }

  const THAN_TACH: TachKetLuanVao = {
    auto_code: true,
    type: "theo-van-ban",
    title: "Đối chiếu số liệu giải ngân sáu tháng đầu năm.",
  };

  /* ⚠ NHÓM QUAN TRỌNG NHẤT CỦA TUYẾN NÀY. Số trên đường dẫn là `ordinal` MÁY CHỦ TRẢ; gửi vị trí
   * trong mảng thì lời gọi vẫn 201 và vẫn lập một nhiệm vụ — chỉ là gắn vào kết luận khác, giao
   * cho người khác, trong một quyển sổ không xoá được. Dữ liệu mẫu ở đây CỐ Ý lệch giữa vị trí và
   * số đã cấp: một ca dùng ①③④ liên tục sẽ xanh với cả hai cách viết. */
  it("số trên đường dẫn là `ordinal` của kết luận, KHÔNG phải vị trí trong mảng", async () => {
    const gia = batFetch(traJSON(201, { code: "NV12" }));
    const ds = [ketLuan({ id: "k1", ordinal: 1 }), ketLuan({ id: "k3", ordinal: 3 })];

    // Kết luận ở VỊ TRÍ THỨ HAI (index 1) mang số ĐÃ CẤP là 3 — ② đã bị gỡ khỏi biên bản.
    const kl = ds[1] as petitions_ketLuanRa;
    await tachKetLuanThanhNhiemVu("01JBB", kl, THAN_TACH, "k");

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/meetings/01JBB/conclusions/3/task");
    expect(loiGoi(gia, 0).duongDan).not.toContain("/conclusions/2/task");
  });

  it("đọc `ordinal` từ chính dòng kết luận — số nào trên bản ghi, số ấy lên đường dẫn", async () => {
    const gia = batFetch(traJSON(201, {}));
    await tachKetLuanThanhNhiemVu("01JBB", ketLuan({ ordinal: 47 }), THAN_TACH, "k");

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/meetings/01JBB/conclusions/47/task");
  });

  it("mã hoá id biên bản vào đường dẫn thay vì ghép thẳng", async () => {
    const gia = batFetch(traJSON(201, {}));
    await tachKetLuanThanhNhiemVu("01J BB/2026", ketLuan(), THAN_TACH, "k");

    expect(loiGoi(gia, 0).duongDan).toBe(
      "/api/v1/meetings/01J%20BB%2F2026/conclusions/3/task",
    );
  });

  it("KHÔNG gửi `source`/`source_id` — cặp nguồn giao là của máy chủ, suy từ đường dẫn", async () => {
    // Biểu mẫu dùng chung khai kiểu `petitions_taoNhiemVuVao`, kiểu ấy CÓ hai trường này, và
    // TypeScript cho gán sang `TachKetLuanVao` vì nó chỉ thừa chứ không thiếu. Nếu hàm trải
    // `...than` thì hai trường ấy lên dây, và §3 nói ô "Nguồn giao" là ô KHOÁ.
    const gia = batFetch(traJSON(201, {}));
    const thanCoThua = {
      ...THAN_TACH,
      source: "tu-nhap",
      source_id: "01JMOT-BAN-GHI-KHAC",
    } as TachKetLuanVao;

    await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), thanCoThua, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("source");
    expect(than).not.toHaveProperty("source_id");
    expect(than).not.toHaveProperty("tenant_id");
  });

  it("KHÔNG gửi `documents` — `petitions.tachKetLuanVao` không có trường ấy", async () => {
    // Biểu mẫu dùng chung khai kiểu `petitions_taoNhiemVuVao`, và từ TASK-02 kiểu ấy mang được ba
    // nhóm văn bản §7.2. Form ở màn Biên bản không vẽ chúng; đây là lớp thứ hai, ở chỗ dựng thân.
    const gia = batFetch(traJSON(201, {}));
    const thanCoVanBan = {
      ...THAN_TACH,
      documents: [{ group: "cap-tren-giao", summary: "Thông báo giả" }],
    } as TachKetLuanVao;

    await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), thanCoVanBan, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("documents");
  });

  it("trường bỏ trống thì VẮNG khỏi thân, không thành chuỗi rỗng", async () => {
    // `due_at` là con trỏ ở máy chủ và "không có hạn" là một trạng thái thật. Một chuỗi rỗng ở đây
    // là một mốc thời gian không phân giải được, tức 400 thay vì một nhiệm vụ không hạn.
    const gia = batFetch(traJSON(201, {}));
    await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), THAN_TACH, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual(["auto_code", "title", "type"]);
  });

  it("mang `Idempotency-Key`, và lần bấm lại dùng LẠI đúng khoá ấy", async () => {
    // Hai lần bấm Tách không được sinh hai nhiệm vụ: lần gửi đầu CÓ THỂ đã cấp một số sổ.
    const gia = batFetch(traJSON(500, { code: "internal", message: "Đã xảy ra lỗi." }));

    await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), THAN_TACH, "k-cua-lan-mo-nay");
    await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), THAN_TACH, "k-cua-lan-mo-nay");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-cua-lan-mo-nay");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-cua-lan-mo-nay");
  });

  it("404 của máy chủ ra nguyên văn — kết luận không có thì nói đúng câu ấy", async () => {
    batFetch(
      traJSON(404, {
        code: "not_found",
        message: "Không tìm thấy kết luận này trong biên bản.",
      }),
    );

    const kq = await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), THAN_TACH, "k");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("Không tìm thấy kết luận này trong biên bản.");
  });

  it("201 thì đọc thân thành nhiệm vụ — SỐ SỔ là thứ bên gọi không biết trước", async () => {
    batFetch(traJSON(201, { code: "NV12", title: "Đối chiếu số liệu giải ngân." }));

    const kq = await tachKetLuanThanhNhiemVu("01JBB", ketLuan(), THAN_TACH, "k");
    expect(kq.ok).toBe(true);
    if (kq.ok) expect(kq.duLieu.code).toBe("NV12");
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

  it("`POST …/{stt}/task` gửi ĐÚNG bộ trường của `petitions.tachKetLuanVao`", async () => {
    const luocDo = hopDong().components.schemas["petitions.tachKetLuanVao"];
    expect(luocDo).toBeDefined();

    // Thân đầy đủ: mọi trường tuỳ chọn đều có giá trị, nên bộ key gửi đi phải trùng KHÍT bộ key
    // của hợp đồng. Thừa một trường là gửi thứ máy chủ không nhận — và ở tuyến này, thừa `source`
    // là đúng thứ §3 khoá lại.
    const thanDayDu: TachKetLuanVao = {
      code: "NV99",
      auto_code: false,
      type: "theo-van-ban",
      bloc: "kinh-te",
      title: "Đối chiếu số liệu giải ngân sáu tháng đầu năm.",
      description: "Theo kết luận ③ của biên bản giao ban tháng 8.",
      priority: "cao",
      unit: "01JBOPHAN",
      assignee: "CB-2026-7K3M9Q",
      assigner: "CB-2026-1A2B3C",
      lead_unit: "01JBOPHAN2",
      monitor: "CB-2026-9Z8Y7X",
      due_at: "2026-08-20T16:59:59Z",
      parent: "01JNV-CHA",
    };

    const keys = await keyDaGui(() =>
      tachKetLuanThanhNhiemVu(
        "01JBB",
        {
          id: "01JKL3",
          ordinal: 3,
          content: "x",
          task_count: 0,
          task_done_count: 0,
          created_at: "2026-08-05T02:00:00Z",
        },
        thanDayDu,
        "k",
      ),
    );

    expect(keys).toEqual(Object.keys(luocDo?.properties ?? {}).sort());
  });

  it("bốn tuyến vẫn đứng sau `task.read` / `task.create` như màn đang giả định", () => {
    // MÀN NÀY KHÔNG CÓ CỔNG QUYỀN Ở CLIENT (xem `PHAN_CHUA_DUNG`), nên không có hằng nào trong
    // `lib/quyen.ts` để canh. Bài kiểm này là chỗ duy nhất phát hiện được ngày máy chủ tách một
    // khoá `meeting.*` riêng — hôm ấy `PHAN_CHUA_DUNG` và thanh menu đều phải sửa theo.
    const p = hopDong().paths;
    expect(p["/api/v1/meetings"]?.["get"]?.["x-vigov-permission"]?.key).toBe("task.read");
    expect(p["/api/v1/meetings"]?.["post"]?.["x-vigov-permission"]?.key).toBe("task.create");
    expect(
      p["/api/v1/meetings/{id}/conclusions"]?.["post"]?.["x-vigov-permission"]?.key,
    ).toBe("task.create");
    expect(
      p["/api/v1/meetings/{id}/conclusions/{stt}/task"]?.["post"]?.["x-vigov-permission"]?.key,
    ).toBe("task.create");
  });
});
