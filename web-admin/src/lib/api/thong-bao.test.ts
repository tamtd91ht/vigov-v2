import { readFileSync } from "node:fs";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  duongDanSoThongBao,
  laySoThongBao,
  phatHanhThongBao,
  type PhatHanhThongBaoVao,
} from "./thong-bao";

/**
 * Hai tuyến của màn Thông báo.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM `org_unit_ids`, và nó canh một thứ không bài kiểm chức
 * năng nào thấy: trường ấy CÓ trong hợp đồng, và máy chủ trả **501** cho bất kỳ thân nào mang nó.
 * Một lần ai đó "cho đủ trường" — kể cả bằng một mảng rỗng — sẽ không làm đỏ màn hình nào trong
 * lúc phát triển, vì `fetch` giả trả 201 cho mọi thân. Nó chỉ hỏng ở lần bấm thật, của một cán bộ
 * đang cần phát một thông báo đi.
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

const THAN_DAY_DU: PhatHanhThongBaoVao = {
  title: "Thông báo về việc triển khai hệ thống an ninh",
  body: "Đề nghị các bộ phận cử cán bộ dự buổi tập huấn lúc 8h00 ngày 10/9/2026.",
  recipient_codes: ["CB-2026-7K3M9Q", "CB-2026-4H2N8P"],
  pinned: true,
  ack_required: true,
  email_requested: true,
};

describe("GET /api/v1/announcements — đường dẫn", () => {
  it("đường dẫn tương đối, không host, không `tenant_id`", async () => {
    const gia = batFetch(traJSON(200, { items: [], next_cursor: "", has_more: false }));

    await laySoThongBao({ limit: 10, cursor: null });
    const duong = String(loiGoi(gia, 0).duongDan);

    expect(duong.startsWith("/api/")).toBe(true);
    expect(duong).not.toMatch(/tenant/i);
    expect(duong).not.toMatch(/^https?:/);
  });

  it("trang đầu KHÔNG mang `cursor` rỗng — `cursor=` là 400 ở máy chủ", () => {
    expect(duongDanSoThongBao({ limit: 10, cursor: null })).toBe("/api/v1/announcements?limit=10");
    expect(duongDanSoThongBao({ limit: 10, cursor: "" })).toBe("/api/v1/announcements?limit=10");
  });

  it("trang sau mang đúng con trỏ máy chủ trả", () => {
    expect(duongDanSoThongBao({ limit: 10, cursor: "MOC-2" })).toBe(
      "/api/v1/announcements?limit=10&cursor=MOC-2",
    );
  });

  it("KHÔNG gửi `sort`, `order` hay `pham_vi`", () => {
    // `sort`/`order`: mặc định máy chủ đã là `created_at` giảm dần. `pham_vi`: tuyến KHÔNG nhận
    // tham số ấy, và gửi nó lên là dựng ảo tưởng rằng bộ lọc "Gửi cho tôi" đang có tác dụng.
    const duong = duongDanSoThongBao({ limit: 10, cursor: "MOC-2" });
    expect(duong).not.toContain("sort");
    expect(duong).not.toContain("order");
    expect(duong).not.toContain("pham_vi");
  });

  it("không đọc được thì trả một câu cho người dùng, không ném", async () => {
    batFetch(
      traJSON(403, { code: "forbidden", message: "Bạn không có quyền soạn và gửi thông báo." }),
    );

    const kq = await laySoThongBao({ limit: 10, cursor: null });
    expect(kq.ok).toBe(false);
    // NGUYÊN VĂN câu máy chủ: màn hình không dựng lại một câu 403 của riêng nó.
    if (!kq.ok) expect(kq.thongBao).toBe("Bạn không có quyền soạn và gửi thông báo.");
  });
});

describe("POST /api/v1/announcements — phát hành", () => {
  it("gửi POST tới đúng tuyến, 201 thì đọc thân thành tấm thẻ", async () => {
    const gia = batFetch(traJSON(201, { id: "01JTB", title: "x", recipient_count: 2 }));

    const kq = await phatHanhThongBao(THAN_DAY_DU, "k-co-dinh");
    const { duongDan, tuyChon } = loiGoi(gia, 0);

    expect(duongDan).toBe("/api/v1/announcements");
    expect(tuyChon.method).toBe("POST");
    expect(kq.ok).toBe(true);
  });

  it("mang `Idempotency-Key` đúng khoá truyền vào, và lần gửi lại dùng LẠI khoá ấy", async () => {
    // Khoá do biểu mẫu giữ. Sinh khoá mới ở lần bấm lại là bỏ đúng cái chống trùng sinh ra để
    // chặn: lần gửi đầu CÓ THỂ đã phát một thông báo đi khắp xã.
    const gia = batFetch(traJSON(500, { code: "internal", message: "Đã xảy ra lỗi." }));

    await phatHanhThongBao(THAN_DAY_DU, "k-co-dinh");
    await phatHanhThongBao(THAN_DAY_DU, "k-co-dinh");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-co-dinh");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-co-dinh");
  });

  it("KHÔNG gửi `org_unit_ids` — kể cả một mảng rỗng", async () => {
    // Máy chủ trả 501 cho MỌI thân mang trường ấy có phần tử, và một mảng rỗng đi trên dây là lời
    // mời dựng một ô chọn bộ phận ở màn hình. Xem `ErrGuiTheoBoPhanChuaCo`.
    const gia = batFetch(traJSON(201, {}));
    // Hợp đồng CHO PHÉP trường này, nên người gọi truyền được nó vào mà TypeScript không cản —
    // đó chính là lần hỏng cần chặn, và nó bị chặn ở `phatHanhThongBao` chứ không ở kiểu.
    await phatHanhThongBao({ ...THAN_DAY_DU, org_unit_ids: ["01JBP1"] }, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("org_unit_ids");
  });

  it("KHÔNG gửi `status`, `author_code` hay `tenant_id`", async () => {
    const gia = batFetch(traJSON(201, {}));
    await phatHanhThongBao(THAN_DAY_DU, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("status");
    expect(than).not.toHaveProperty("author_code");
    expect(than).not.toHaveProperty("tenant_id");
  });

  it("câu 501 của máy chủ ra nguyên văn, không thành dữ liệu", async () => {
    // Câu này CHỈ ĐƯỜNG: nó nói đúng mục biểu mẫu còn dùng được. Viết lại nó ở client là dựng bản
    // sao thứ hai của một quy tắc nghiệp vụ, và bản sao ấy trôi mà không ai thấy.
    batFetch(
      traJSON(501, {
        code: "not_implemented",
        message:
          "Chưa gửi được thông báo theo bộ phận: hệ thống chưa lấy được danh sách cán bộ của " +
          'một bộ phận. Hãy chọn từng người ở mục "Gửi thêm đích danh".',
      }),
    );

    const kq = await phatHanhThongBao(THAN_DAY_DU, "k");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain("Hãy chọn từng người");
  });

  it("câu 400 `không có người nhận` ra nguyên văn", async () => {
    batFetch(
      traJSON(400, {
        code: "invalid_request",
        message: "Thông báo phải có ít nhất một người nhận.",
      }),
    );

    const kq = await phatHanhThongBao({ ...THAN_DAY_DU, recipient_codes: [] }, "k");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("Thông báo phải có ít nhất một người nhận.");
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

describe("thân yêu cầu khớp hợp đồng", () => {
  it("gửi ĐÚNG bộ trường của `comms.phatHanhThongBaoVao`, TRỪ `org_unit_ids`", async () => {
    const luocDo = hopDong().components.schemas["comms.phatHanhThongBaoVao"];
    expect(luocDo).toBeDefined();

    const cuaHopDong = Object.keys(luocDo?.properties ?? {});
    // Trường ấy PHẢI còn trong hợp đồng: ngày nó biến mất, bài kiểm này đỏ và người đọc biết
    // rằng máy chủ đã đổi ý về cách gửi theo bộ phận.
    expect(cuaHopDong).toContain("org_unit_ids");

    const gia = batFetch(traJSON(201, {}));
    // THÂN TRUYỀN VÀO MANG CẢ `org_unit_ids`, có chủ ý: nếu ở đây truyền một thân thiếu trường ấy
    // thì `JSON.stringify` bỏ khoá `undefined` đi, và phép so bộ khoá dưới sẽ XANH kể cả khi
    // `phatHanhThongBao` đang chuyền thẳng trường ấy lên máy chủ — tức canh đúng con số không.
    await phatHanhThongBao({ ...THAN_DAY_DU, org_unit_ids: ["01JBP1"] }, "k");
    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;

    // Thân đầy đủ: mọi trường tuỳ chọn còn lại đều có giá trị, nên bộ khoá gửi đi phải trùng KHÍT
    // bộ khoá hợp đồng trừ đúng một trường máy chủ đang từ chối. Thừa một trường là gửi thứ máy
    // chủ không nhận; thiếu một trường là một ô biểu mẫu không bao giờ tới nơi.
    expect(Object.keys(than).sort()).toEqual(cuaHopDong.filter((k) => k !== "org_unit_ids").sort());
  });

  it("hai trường bắt buộc của hợp đồng đúng là `title` và `body`", () => {
    const luocDo = hopDong().components.schemas["comms.phatHanhThongBaoVao"];
    expect((luocDo?.required ?? []).slice().sort()).toEqual(["body", "title"]);
  });

  it("cả hai tuyến vẫn đứng sau `announcement.create` như màn đang giả định", () => {
    // MÀN NÀY KHÔNG CÓ CỔNG QUYỀN Ở CLIENT (xem `PHAN_CHUA_DUNG`), nên không có hằng nào trong
    // `lib/quyen.ts` để canh. Bài kiểm này là chỗ duy nhất phát hiện được ngày máy chủ tách một
    // khoá `announcement.read` riêng — hôm ấy `PHAN_CHUA_DUNG` và thanh menu đều phải sửa theo.
    const p = hopDong().paths;
    expect(p["/api/v1/announcements"]?.["get"]?.["x-vigov-permission"]?.key).toBe(
      "announcement.create",
    );
    expect(p["/api/v1/announcements"]?.["post"]?.["x-vigov-permission"]?.key).toBe(
      "announcement.create",
    );
  });
});
