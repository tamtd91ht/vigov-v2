import { readFileSync } from "node:fs";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  duongDanMotNoiDung,
  duongDanSoNoiDung,
  layDanhMucNoiDung,
  layMotNoiDung,
  laySoNoiDung,
  suaNoiDung,
  themDanhMucNoiDung,
  themNoiDung,
  type SuaNoiDungVao,
  type ThemDanhMucVao,
  type ThemNoiDungVao,
} from "./noi-dung";

/**
 * Sáu tuyến của màn Nội dung Mini App.
 *
 * HAI NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY, và cả hai canh thứ không bài kiểm chức năng nào thấy:
 *
 *   1. `ba tên bộ lọc đọc thẳng từ mã máy chủ` — hợp đồng KHÔNG khai `type`/`category`/`q`, nên
 *      `tsc` không canh được tên nào. Gõ `loai` thay `type` thì không gì đỏ: máy chủ bỏ qua trong
 *      im lặng và trả CẢ QUYỂN SỔ, trong khi cán bộ tin mình đang xem một lát cắt. Bài kiểm ở đây
 *      là chỗ duy nhất phát hiện được điều đó.
 *   2. `thân ghi không mang trường máy chủ tự quyết` — `status`, `source`, `author_code`,
 *      `view_count`. Một lần ai đó "cho đủ trường" bằng cách trải một hàng vừa đọc về sẽ không
 *      làm đỏ màn hình nào trong lúc phát triển, vì `fetch` giả nhận mọi thân.
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

function thanDaGui(gia: ReturnType<typeof batFetch>, n: number): Record<string, unknown> {
  return JSON.parse(String(loiGoi(gia, n).tuyChon.body)) as Record<string, unknown>;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

/** Thân THÊM đầy đủ: mọi trường tuỳ chọn đều có giá trị, để phép so bộ khoá nói được điều gì. */
const THEM_DAY_DU: ThemNoiDungVao = {
  type: "tin-tuc",
  title: "Xã tổ chức hội nghị tổng kết công tác chuyển đổi số năm 2026",
  category_id: "01JDM1",
  summary: "Hội nghị diễn ra sáng 14/9 tại hội trường UBND xã.",
  body: "<p>Sáng 14/9, UBND xã tổ chức hội nghị tổng kết.</p>",
  image_url: "https://cdn.example.vn/anh/hoi-nghi.jpg",
  publish: true,
};

/** Thân SỬA đầy đủ — cùng lý do. */
const SUA_DAY_DU: SuaNoiDungVao = {
  type: "su-kien",
  category_id: "",
  title: "Hội nghị tổng kết chuyển đổi số",
  summary: "",
  body: "<p>Nội dung đã sửa.</p>",
  image_url: "",
  publish: false,
};

const THEM_DANH_MUC_DAY_DU: ThemDanhMucVao = {
  name: "Chuyển đổi số",
  slug: "chuyen-doi-so",
  parent_id: "01JDM0",
  order: 3,
};

describe("GET /api/v1/content-items — đường dẫn và bộ lọc §6", () => {
  it("đường dẫn tương đối, không host, không `tenant_id`", async () => {
    const gia = batFetch(traJSON(200, { items: [], next_cursor: "", has_more: false }));

    await laySoNoiDung({ limit: 20, cursor: null });
    const duong = String(loiGoi(gia, 0).duongDan);

    expect(duong.startsWith("/api/")).toBe(true);
    expect(duong).not.toMatch(/tenant/i);
    expect(duong).not.toMatch(/^https?:/);
  });

  it("trang đầu KHÔNG mang `cursor` rỗng — `cursor=` là 400 ở máy chủ", () => {
    expect(duongDanSoNoiDung({ limit: 20, cursor: null })).toBe("/api/v1/content-items?limit=20");
    expect(duongDanSoNoiDung({ limit: 20, cursor: "" })).toBe("/api/v1/content-items?limit=20");
  });

  it("không bộ lọc nào thì không có dấu `?`", () => {
    expect(duongDanSoNoiDung({})).toBe("/api/v1/content-items");
  });

  it("ba bộ lọc §6 đi đúng ba tên máy chủ đọc: `type` · `category` · `q`", () => {
    const duong = duongDanSoNoiDung({
      loai: "su-kien",
      danhMucID: "01JDM1",
      tim: "hội nghị",
      limit: 20,
    });
    const q = new URL(duong, "https://xa.example").searchParams;

    expect(q.get("type")).toBe("su-kien");
    expect(q.get("category")).toBe("01JDM1");
    expect(q.get("q")).toBe("hội nghị");
    // Ba tên của §9 KHÔNG đi trên dây: bề mặt hợp đồng là tiếng Anh (ADR 0011). Gửi chúng là gửi
    // ba tham số máy chủ bỏ qua, tức một bộ lọc trông như đang lọc.
    expect(q.has("loai")).toBe(false);
    expect(q.has("danh_muc")).toBe(false);
  });

  it("bộ lọc rỗng thì VẮNG khỏi query, không đi thành `type=`", () => {
    // `type=` rỗng vẫn là một tham số có mặt, và `thamSoLoc` trả `""` — cùng kết quả hôm nay,
    // nhưng nó là một hình dạng mời người sau thêm một nhánh "rỗng nghĩa là gì".
    const duong = duongDanSoNoiDung({ loai: "", danhMucID: "", tim: "" });
    expect(duong).toBe("/api/v1/content-items");
  });

  it("KHÔNG gửi `sort` hay `order` — mặc định máy chủ đã đúng thứ tự §6", () => {
    const duong = duongDanSoNoiDung({ limit: 20, cursor: "MOC-2" });
    expect(duong).not.toContain("sort");
    expect(duong).not.toContain("order");
  });

  it("không đọc được thì trả một câu cho người dùng, không ném", async () => {
    batFetch(
      traJSON(403, { code: "forbidden", message: "Bạn không có quyền xem nội dung Mini App." }),
    );

    const kq = await laySoNoiDung({ limit: 20, cursor: null });
    expect(kq.ok).toBe(false);
    // NGUYÊN VĂN câu máy chủ: màn hình không dựng lại một câu 403 của riêng nó.
    if (!kq.ok) expect(kq.thongBao).toBe("Bạn không có quyền xem nội dung Mini App.");
  });
});

describe("GET /api/v1/content-items/{id} — toàn văn", () => {
  it("id được mã hoá vào đường dẫn, không ghép thẳng", () => {
    expect(duongDanMotNoiDung("01JND1")).toBe("/api/v1/content-items/01JND1");
    expect(duongDanMotNoiDung("a/b?c")).toBe("/api/v1/content-items/a%2Fb%3Fc");
  });

  it("gọi GET đúng tuyến chi tiết, 200 thì đọc thân", async () => {
    const gia = batFetch(traJSON(200, { id: "01JND1", title: "x", body: "<p>y</p>" }));

    const kq = await layMotNoiDung("01JND1");
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/content-items/01JND1");
    expect(loiGoi(gia, 0).tuyChon.method).toBe("GET");
    expect(kq.ok).toBe(true);
  });

  it("404 của xã khác ra nguyên văn — không phân biệt được với `không tồn tại`, có chủ ý", async () => {
    batFetch(traJSON(404, { code: "not_found", message: "Không tìm thấy nội dung này." }));

    const kq = await layMotNoiDung("01JKHAC");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("Không tìm thấy nội dung này.");
  });
});

describe("POST /api/v1/content-items — soạn", () => {
  it("POST đúng tuyến, 201 thì đọc thân thành cả hàng", async () => {
    const gia = batFetch(traJSON(201, { id: "01JND1", status: "dang-hien" }));

    const kq = await themNoiDung(THEM_DAY_DU, "k-co-dinh");
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/content-items");
    expect(loiGoi(gia, 0).tuyChon.method).toBe("POST");
    expect(kq.ok).toBe(true);
  });

  it("mang `Idempotency-Key` đúng khoá truyền vào, và lần gửi lại dùng LẠI khoá ấy", async () => {
    // Lần gửi đầu CÓ THỂ đã tới máy chủ và đã đăng một bài lên Mini App của cả xã. Sinh khoá mới ở
    // lần bấm lại là bỏ đúng cái chống trùng sinh ra để chặn.
    const gia = batFetch(traJSON(500, { code: "internal", message: "Đã xảy ra lỗi." }));

    await themNoiDung(THEM_DAY_DU, "k-co-dinh");
    await themNoiDung(THEM_DAY_DU, "k-co-dinh");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-co-dinh");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-co-dinh");
  });

  it("KHÔNG gửi `status`, `source`, `author_code`, `view_count`, `id` hay `tenant_id`", async () => {
    const gia = batFetch(traJSON(201, {}));
    // Thân truyền vào mang CẢ một hàng vừa đọc về — đúng hình dạng sẽ xảy ra ngày ai đó sửa một
    // bài rồi đăng lại. `themNoiDung` dựng từng trường nên sáu trường ấy không có đường nào đi qua.
    const thanBanTay = {
      ...THEM_DAY_DU,
      status: "dang-hien",
      source: "dong-bo-cong",
      author_code: "CB-2026-7K3M9Q",
      view_count: 9,
      id: "01JND1",
      tenant_id: "01JXA",
    } as unknown as ThemNoiDungVao;
    await themNoiDung(thanBanTay, "k");

    const than = thanDaGui(gia, 0);
    for (const cam of ["status", "source", "author_code", "view_count", "id", "tenant_id"]) {
      expect(than).not.toHaveProperty(cam);
    }
  });
});

describe("PATCH /api/v1/content-items/{id} — sửa", () => {
  it("PATCH đúng tuyến chi tiết, 200 thì đọc thân", async () => {
    const gia = batFetch(traJSON(200, { id: "01JND1", hand_edited: true }));

    const kq = await suaNoiDung("01JND1", SUA_DAY_DU);
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/content-items/01JND1");
    expect(loiGoi(gia, 0).tuyChon.method).toBe("PATCH");
    expect(kq.ok).toBe(true);
  });

  it("KHÔNG mang `Idempotency-Key` — hợp đồng không đòi", async () => {
    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", SUA_DAY_DU);
    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBeNull();
  });

  it("trường KHÔNG nhắc tới thì VẮNG khỏi thân — `publish` vắng không được thành `false`", async () => {
    // Đây là chỗ nguy hiểm nhất của tuyến này. Ở máy chủ mỗi trường là một con trỏ: vắng nghĩa là
    // "để nguyên", `false` nghĩa là "gỡ khỏi Mini App". Một màn hình sửa mỗi tiêu đề mà gửi kèm
    // `publish: false` sẽ âm thầm gỡ bài khỏi Mini App của cả xã.
    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", { title: "Tiêu đề mới" });

    const than = thanDaGui(gia, 0);
    expect(Object.keys(than)).toEqual(["title"]);
    expect(than).not.toHaveProperty("publish");
  });

  it("`publish: false` ĐƯỢC gửi khi người dùng thật sự tắt ô tích", async () => {
    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", { publish: false });
    expect(thanDaGui(gia, 0)).toEqual({ publish: false });
  });

  it("`summary: \"\"` ĐƯỢC gửi — chuỗi rỗng là `xoá tóm tắt`, không phải `không nhắc tới`", async () => {
    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", { summary: "" });
    expect(thanDaGui(gia, 0)).toEqual({ summary: "" });
  });

  it("KHÔNG gửi `status` kể cả khi người gọi nhét vào", async () => {
    const gia = batFetch(traJSON(200, {}));
    const thanBanTay = {
      ...SUA_DAY_DU,
      status: "dang-hien",
      hand_edited: false,
    } as unknown as SuaNoiDungVao;
    await suaNoiDung("01JND1", thanBanTay);

    const than = thanDaGui(gia, 0);
    expect(than).not.toHaveProperty("status");
    expect(than).not.toHaveProperty("hand_edited");
  });
});

describe("danh mục tin §6", () => {
  it("GET đúng tuyến, không tham số nào", async () => {
    const gia = batFetch(traJSON(200, { items: [] }));
    await layDanhMucNoiDung();
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/content-categories");
  });

  it("POST mang `Idempotency-Key` và giữ nguyên khoá khi gửi lại", async () => {
    const gia = batFetch(traJSON(500, { code: "internal", message: "Đã xảy ra lỗi." }));

    await themDanhMucNoiDung(THEM_DANH_MUC_DAY_DU, "k-dm");
    await themDanhMucNoiDung(THEM_DANH_MUC_DAY_DU, "k-dm");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-dm");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-dm");
  });

  it("câu 409 `slug đã dùng` ra NGUYÊN VĂN — nó giải thích một điều màn hình không tự nói được", async () => {
    batFetch(
      traJSON(409, {
        code: "code_taken",
        message:
          "Slug này đã được dùng trong xã — kể cả khi danh mục mang slug đó đã bị xoá. " +
          "Mã đã cấp thì không cấp lại. Hãy chọn một slug khác.",
      }),
    );

    const kq = await themDanhMucNoiDung(THEM_DANH_MUC_DAY_DU, "k");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain("Mã đã cấp thì không cấp lại");
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

function khoaCuaLuocDo(ten: string): string[] {
  const luocDo = hopDong().components.schemas[ten];
  expect(luocDo, `hợp đồng phải còn lược đồ \`${ten}\``).toBeDefined();
  return Object.keys(luocDo?.properties ?? {});
}

describe("thân yêu cầu khớp hợp đồng", () => {
  it("`themNoiDung` gửi ĐÚNG bộ trường của `comms.themNoiDungVao`", async () => {
    const cuaHopDong = khoaCuaLuocDo("comms.themNoiDungVao");

    const gia = batFetch(traJSON(201, {}));
    await themNoiDung(THEM_DAY_DU, "k");

    // Thân đầy đủ: mọi trường tuỳ chọn đều có giá trị, nên bộ khoá gửi đi phải trùng KHÍT bộ khoá
    // hợp đồng. Thừa một trường là gửi thứ máy chủ không nhận; thiếu một trường là một ô biểu mẫu
    // không bao giờ tới nơi.
    expect(Object.keys(thanDaGui(gia, 0)).sort()).toEqual(cuaHopDong.slice().sort());
  });

  it("`suaNoiDung` gửi ĐÚNG bộ trường của `comms.suaNoiDungVao`", async () => {
    const cuaHopDong = khoaCuaLuocDo("comms.suaNoiDungVao");

    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", SUA_DAY_DU);

    expect(Object.keys(thanDaGui(gia, 0)).sort()).toEqual(cuaHopDong.slice().sort());
  });

  it("`themDanhMucNoiDung` gửi ĐÚNG bộ trường của `comms.themDanhMucVao`", async () => {
    const cuaHopDong = khoaCuaLuocDo("comms.themDanhMucVao");

    const gia = batFetch(traJSON(201, {}));
    await themDanhMucNoiDung(THEM_DANH_MUC_DAY_DU, "k");

    expect(Object.keys(thanDaGui(gia, 0)).sort()).toEqual(cuaHopDong.slice().sort());
  });

  it("hai trường bắt buộc của `comms.themNoiDungVao` đúng là `title` và `type`", () => {
    const luocDo = hopDong().components.schemas["comms.themNoiDungVao"];
    expect((luocDo?.required ?? []).slice().sort()).toEqual(["title", "type"]);
  });

  it("`body` KHÔNG bắt buộc trên `comms.noiDungRa` — nó VẮNG ở danh sách và CÓ ở chi tiết", () => {
    // Đây là hợp đồng của quyết định #1 của máy chủ. Ngày `body` thành bắt buộc, phản hồi danh
    // sách bắt đầu mang thân bài — và bài kiểm này đỏ trước khi màn hình bắt đầu hiện toàn văn
    // trong một bảng.
    const luocDo = hopDong().components.schemas["comms.noiDungRa"];
    expect(Object.keys(luocDo?.properties ?? {})).toContain("body");
    expect(luocDo?.required ?? []).not.toContain("body");
  });

  it("KHÔNG có tuyến `DELETE` nào cho nội dung — §6 chỉ có `✎`", () => {
    // Ngày tuyến xoá xuất hiện, bài kiểm này đỏ và người đọc biết rằng phải quay lại luật 7: xoá
    // một bản ghi nghiệp vụ là xoá MỀM, bắt buộc có `delete_reason`, mà màn này không thu câu ấy.
    const p = hopDong().paths;
    expect(Object.keys(p["/api/v1/content-items/{id}"] ?? {})).not.toContain("delete");
    expect(Object.keys(p["/api/v1/content-categories"] ?? {})).not.toContain("delete");
  });

  it("sáu tuyến vẫn đứng sau đúng hai khoá màn đang giả định", () => {
    // MÀN NÀY KHÔNG CÓ CỔNG QUYỀN Ở CLIENT (xem `PHAN_CHUA_DUNG`), nên không có hằng nào trong
    // `lib/quyen.ts` để canh. Bài kiểm này là chỗ duy nhất phát hiện được ngày máy chủ tách khoá.
    const p = hopDong().paths;
    const khoa = (duong: string, pt: string) => p[duong]?.[pt]?.["x-vigov-permission"]?.key;

    expect(khoa("/api/v1/content-items", "get")).toBe("content.read");
    expect(khoa("/api/v1/content-items/{id}", "get")).toBe("content.read");
    expect(khoa("/api/v1/content-categories", "get")).toBe("content.read");
    expect(khoa("/api/v1/content-items", "post")).toBe("content.update");
    expect(khoa("/api/v1/content-items/{id}", "patch")).toBe("content.update");
    expect(khoa("/api/v1/content-categories", "post")).toBe("content.update");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BA TÊN BỘ LỌC — ĐỌC THẲNG MÃ MÁY CHỦ, VÌ HỢP ĐỒNG KHÔNG KHAI CHÚNG
 *
 * `tools/apidoc/truyvan.go` đọc được tên tham số ở một CLOSURE CỤC BỘ nhưng không đọc được ở một
 * HÀM CẤP GÓI, mà `thamSoLoc(q, "type")` là hàm cấp gói. Nên `comms_get_content_items["truyVan"]`
 * chỉ có bốn khoá phân trang, và ba bộ lọc của §6 không có kiểu nào canh.
 *
 * Bài kiểm dưới thay chỗ cho cái kiểu ấy. Nó so BỘ TÊN, không so từng tên: ngày máy chủ thêm một
 * bộ lọc thứ tư, nó đỏ và có người phải quyết định màn hình có vẽ ô ấy hay không — thay vì một bộ
 * lọc mới sống ở máy chủ mà không màn nào biết.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("ba tên bộ lọc khớp handler thật", () => {
  it("`themLocVaoTruyVan` gửi đúng bộ tên `thamSoLoc(q, …)` đọc", () => {
    const duong = new URL(
      "../../../../service-comms/internal/http/noi_dung_mini_app.go",
      import.meta.url,
    );
    const nguon = readFileSync(duong, "utf8");

    const cuaMayChu = [...nguon.matchAll(/thamSoLoc\(q,\s*"([^"]+)"\)/g)]
      .map((m) => m[1] ?? "")
      .filter((t) => t !== "");
    // Bộ quét rỗng là bộ quét luôn xanh: nếu hàm phụ ấy đổi tên thì không khớp gì cả, và phép so
    // dưới vẫn phải đỏ chứ không được im lặng đi qua.
    expect(cuaMayChu.length).toBeGreaterThan(0);

    const gui = new URL(
      duongDanSoNoiDung({ loai: "su-kien", danhMucID: "01JDM1", tim: "x" }),
      "https://xa.example",
    ).searchParams;

    expect(cuaMayChu.slice().sort()).toEqual(["category", "q", "type"]);
    for (const ten of cuaMayChu) {
      expect(gui.has(ten), `màn hình phải gửi được bộ lọc \`${ten}\``).toBe(true);
    }
  });
});
