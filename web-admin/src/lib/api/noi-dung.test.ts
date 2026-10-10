import { readFileSync } from "node:fs";

import { afterEach, describe, expect, it, vi } from "vitest";

import { stripTechnicalPrefix } from "./goi";

import {
  contentCategoryPath,
  countPublishedStaff,
  publishedStaffPath,
  contentItemDeletePath,
  deleteContentCategory,
  deleteContentItem,
  duongDanMotNoiDung,
  duongDanSoNoiDung,
  layDanhMucNoiDung,
  layMotNoiDung,
  laySoNoiDung,
  portalCategoryName,
  uploadBodyImage,
  uploadBroadcastAudio,
  uploadCoverImage,
  suaNoiDung,
  themDanhMucNoiDung,
  themNoiDung,
  updateContentCategory,
  type SuaNoiDungVao,
  type ThemDanhMucVao,
  type ThemNoiDungVao,
} from "./noi-dung";
import { installFakeUploadXHR, partNames, partValue } from "./upload-test-support";

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
  cover_image_file_id: "01JCOVER1",
  publish: true,
};

/**
 * The two per-type full bodies. No single POST can carry every contract key — an event field and a
 * video link never go on one type — so the contract check below compares the UNION of these two.
 */
const EVENT_FULL: ThemNoiDungVao = {
  ...THEM_DAY_DU,
  type: "su-kien",
  event_starts_at: "2026-10-05T08:00:00+07:00",
  event_ends_at: "2026-10-05T11:30:00+07:00",
  event_place: "Hội trường UBND xã",
};

const VIDEO_FULL: ThemNoiDungVao = {
  ...THEM_DAY_DU,
  type: "video",
  video_url: "https://video.example.vn/hoi-nghi",
};

/** The banner's two fields (ADR 0067 §5), only on `type: banner`. */
const BANNER_FULL: ThemNoiDungVao = {
  ...THEM_DAY_DU,
  type: "banner",
  link_to: "/tin-tuc",
  display_order: 2,
};

/**
 * Thân SỬA đầy đủ — cùng lý do. `suaNoiDung` does not filter by type (a PATCH often carries none),
 * so one body can hold every key; which ones the form puts in is `thanSua`'s test.
 */
const SUA_DAY_DU: SuaNoiDungVao = {
  type: "su-kien",
  category_id: "",
  title: "Hội nghị tổng kết chuyển đổi số",
  summary: "",
  body: "<p>Nội dung đã sửa.</p>",
  image_url: "",
  publish: false,
  event_starts_at: "2026-10-05T08:00:00+07:00",
  event_ends_at: "",
  event_place: "Hội trường UBND xã",
  video_url: "",
  cover_image_file_id: "",
  link_to: "",
  display_order: 4,
  // ADR 0067 §4: `""` is the remove signal; the duration rides along so the body names every key.
  audio_file_id: "",
  audio_duration_seconds: 200,
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
    const duong = duongDanSoNoiDung({ loai: "", danhMucID: "", tim: "", status: "" });
    expect(duong).toBe("/api/v1/content-items");
  });

  it("the status filter goes as `status`, combined with the other three", () => {
    const q = new URL(
      duongDanSoNoiDung({ loai: "tin-tuc", danhMucID: "01JDM1", tim: "hội nghị", status: "cho-duyet", limit: 20 }),
      "https://xa.example",
    ).searchParams;
    expect(q.get("status")).toBe("cho-duyet");
    expect(q.get("type")).toBe("tin-tuc");
    expect(q.get("category")).toBe("01JDM1");
    expect(q.get("q")).toBe("hội nghị");
    // Never the Vietnamese spelling, which the server would ignore and answer the whole register.
    expect(q.has("trang_thai")).toBe(false);
  });

  it("an unknown status comes back as the server's 400 sentence, verbatim", async () => {
    const cau = "Tham số `status` phải là một trong ba trạng thái: an, cho-duyet, dang-hien.";
    batFetch(traJSON(400, { code: "invalid_request", message: cau }));
    const kq = await laySoNoiDung({ status: "xoa" });
    expect(kq).toEqual({ ok: false, thongBao: cau });
  });

  it("`portalCategoryName` reads the synced item's category, `\"\"` when absent", () => {
    const row = { id: "1" } as unknown as Parameters<typeof portalCategoryName>[0];
    expect(portalCategoryName(row)).toBe("");
    expect(portalCategoryName({ ...row, portal_category_name: "Tin địa phương" } as never)).toBe("Tin địa phương");
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

  it("KHÔNG gửi `published_at` — ô tích `publish` ghim nó ở lần đăng đầu, không thân nào đặt được", async () => {
    const gia = batFetch(traJSON(201, {}));
    const withPublishedAt = {
      ...THEM_DAY_DU,
      published_at: "2026-01-01T00:00:00Z",
    } as unknown as ThemNoiDungVao;
    await themNoiDung(withPublishedAt, "k");
    expect(thanDaGui(gia, 0)).not.toHaveProperty("published_at");
  });

  it("event fields go only with `su-kien`, the video link only with `video` — otherwise a 400", async () => {
    const gia = batFetch(traJSON(201, {}));
    // A news item handed every per-type field: none of them may leave, the server refuses each.
    await themNoiDung(
      { ...EVENT_FULL, video_url: "https://video.example.vn/x", type: "tin-tuc" },
      "k",
    );
    await themNoiDung({ ...EVENT_FULL, video_url: "https://video.example.vn/x" }, "k");
    await themNoiDung({ ...VIDEO_FULL, event_place: "Hội trường" }, "k");

    const news = thanDaGui(gia, 0);
    for (const k of ["event_starts_at", "event_ends_at", "event_place", "video_url"]) {
      expect(news).not.toHaveProperty(k);
    }

    const event = thanDaGui(gia, 1);
    expect(event.event_starts_at).toBe("2026-10-05T08:00:00+07:00");
    expect(event.event_ends_at).toBe("2026-10-05T11:30:00+07:00");
    expect(event.event_place).toBe("Hội trường UBND xã");
    expect(event).not.toHaveProperty("video_url");

    const video = thanDaGui(gia, 2);
    expect(video.video_url).toBe("https://video.example.vn/hoi-nghi");
    expect(video).not.toHaveProperty("event_place");
  });

  it("an empty per-type field is left out of the POST, not sent as `\"\"`", async () => {
    const gia = batFetch(traJSON(201, {}));
    await themNoiDung({ ...EVENT_FULL, event_ends_at: "", event_place: "" }, "k");
    const than = thanDaGui(gia, 0);
    expect(than).toHaveProperty("event_starts_at");
    expect(than).not.toHaveProperty("event_ends_at");
    expect(than).not.toHaveProperty("event_place");
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

  it("`\"\"` on a per-type field IS sent — it is the server's one spelling of `clear`", async () => {
    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", { event_ends_at: "", event_place: "", video_url: "" });
    expect(thanDaGui(gia, 0)).toEqual({ event_ends_at: "", event_place: "", video_url: "" });
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

/* ── Ảnh bìa §7 ─────────────────────────────────────────────────────────────────────────────── */

const COVER_READY = { id: "01JCOVER1", content_item_id: "01JND1", mime_type: "image/jpeg", size_bytes: 3, status: "ready" };
const jpg = () => new File([new Uint8Array([0xff, 0xd8, 0xff])], "anh.jpg", { type: "image/jpeg" });

describe("cover upload — ONE multipart request", () => {
  it("POST …/cover-images on this origin, the caller's key, parts in contract order, no `content_item_id` for a new article", async () => {
    const sent = installFakeUploadXHR(() => ({ status: 201, body: COVER_READY }));
    const gia = batFetch(traJSON(500, {}));
    const r = await uploadCoverImage({ file: jpg(), contentType: "image/jpeg", contentItemId: undefined }, "khoa-anh");
    expect(sent).toHaveLength(1);
    expect(sent[0]?.url).toBe("/api/v1/content-items/cover-images");
    expect(sent[0]?.method).toBe("POST");
    expect(sent[0]?.headers["Idempotency-Key"]).toBe("khoa-anh");
    expect(partNames(sent[0])).toEqual(["size", "file_name", "content_type", "file"]);
    expect(partValue(sent[0], "size")).toBe("3");
    expect(gia).not.toHaveBeenCalled();
    expect(r).toEqual({ ok: true, data: COVER_READY });
  });

  it("the edit form names its article; an empty id is left out, not sent as `\"\"`", async () => {
    const sent = installFakeUploadXHR(() => ({ status: 201, body: COVER_READY }));
    await uploadCoverImage({ file: jpg(), contentType: "image/jpeg", contentItemId: "01JND1" }, "k");
    await uploadCoverImage({ file: jpg(), contentType: "image/jpeg", contentItemId: "" }, "k");
    expect(partValue(sent[0], "content_item_id")).toBe("01JND1");
    expect(partNames(sent[0])).toEqual(["size", "file_name", "content_type", "content_item_id", "file"]);
    expect(partNames(sent[1])).not.toContain("content_item_id");
  });

  it("a refusal comes back VERBATIM with its status and code", async () => {
    const cau = "Ảnh bị từ chối: nội dung tệp không phải JPG, PNG hoặc WebP được phép.";
    installFakeUploadXHR(() => ({ status: 422, body: { code: "cover_rejected", message: cau } }));
    expect(await uploadCoverImage({ file: jpg(), contentType: "image/jpeg", contentItemId: undefined }, "k")).toEqual({
      ok: false,
      status: 422,
      code: "cover_rejected",
      message: cau,
    });
  });
});

describe("broadcast audio — ONE multipart request, the typed duration BEFORE the file", () => {
  it("parts: size · file_name · content_type · content_item_id · audio_duration_seconds · file", async () => {
    const READY = { id: "01JA", content_item_id: "01JND1", mime_type: "audio/mpeg", size_bytes: 3, status: "ready", duration_seconds: 750 };
    const sent = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const mp3 = new File([new Uint8Array([0x49, 0x44, 0x33])], "ban-tin.mp3", { type: "audio/mpeg" });
    const r = await uploadBroadcastAudio({ file: mp3, contentType: "audio/mpeg", contentItemId: "01JND1", durationSeconds: 750 }, "k");
    expect(sent[0]?.url).toBe("/api/v1/content-items/audio-files");
    expect(partNames(sent[0])).toEqual(["size", "file_name", "content_type", "content_item_id", "audio_duration_seconds", "file"]);
    expect(partValue(sent[0], "audio_duration_seconds")).toBe("750");
    expect(r).toEqual({ ok: true, data: READY });
  });
});

describe("body image — ONE multipart request", () => {
  it("POST …/body-images, parts in contract order", async () => {
    const READY = { id: "01JB", content_item_id: "01JND1", mime_type: "image/jpeg", size_bytes: 3, status: "ready" };
    const sent = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    await uploadBodyImage({ file: jpg(), contentType: "image/jpeg", contentItemId: "01JND1" }, "k");
    expect(sent[0]?.url).toBe("/api/v1/content-items/body-images");
    expect(partNames(sent[0])).toEqual(["size", "file_name", "content_type", "content_item_id", "file"]);
  });
});

describe("the cover on the article", () => {
  it("POST carries `cover_image_file_id` when given, omits it when empty", async () => {
    const gia = batFetch(traJSON(201, {}));
    await themNoiDung(THEM_DAY_DU, "k");
    await themNoiDung({ ...THEM_DAY_DU, cover_image_file_id: "" }, "k");
    expect(thanDaGui(gia, 0).cover_image_file_id).toBe("01JCOVER1");
    expect(thanDaGui(gia, 1)).not.toHaveProperty("cover_image_file_id");
  });

  it("PATCH `cover_image_file_id: \"\"` IS sent — it is DETACH; absent means leave alone", async () => {
    const gia = batFetch(traJSON(200, {}));
    await suaNoiDung("01JND1", { cover_image_file_id: "" });
    await suaNoiDung("01JND1", { title: "x" });
    expect(thanDaGui(gia, 0)).toEqual({ cover_image_file_id: "" });
    expect(thanDaGui(gia, 1)).not.toHaveProperty("cover_image_file_id");
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

describe("category edit / delete (ADR 0067 §3)", () => {
  it("PATCH goes to the encoded id, sends exactly what it was given, and no Idempotency-Key", async () => {
    const gia = batFetch(traJSON(200, { id: "01JDM1" }));
    // A whole row read back from the list — `slug`, `created_at` — must not ride along: `slug` is a 400.
    const row = { name: "Tên mới", slug: "chuyen-doi-so", created_at: "x" } as unknown as Parameters<
      typeof updateContentCategory
    >[1];
    const kq = await updateContentCategory("a/b", row);
    const { duongDan, tuyChon, header } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/content-categories/a%2Fb");
    expect(tuyChon.method).toBe("PATCH");
    expect(header.get("Idempotency-Key")).toBeNull();
    expect(thanDaGui(gia, 0)).toEqual({ name: "Tên mới" });
    expect(kq.ok).toBe(true);
  });

  it("`parent_id: \"\"` IS sent — it is MOVE TO ROOT; `hidden: false` IS sent — it is SHOW", async () => {
    const gia = batFetch(traJSON(200, {}));
    await updateContentCategory("01JDM1", { parent_id: "" });
    await updateContentCategory("01JDM1", { hidden: false });
    expect(thanDaGui(gia, 0)).toEqual({ parent_id: "" });
    expect(thanDaGui(gia, 1)).toEqual({ hidden: false });
  });

  it("DELETE carries the reason IN THE BODY, never in the URL, and 204 is success", async () => {
    const gia = batFetch(new Response(null, { status: 204 }));
    const kq = await deleteContentCategory("01JDM1", "Trùng với danh mục An ninh");
    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe(contentCategoryPath("01JDM1"));
    expect(duongDan).not.toContain("?");
    expect(tuyChon.method).toBe("DELETE");
    expect(thanDaGui(gia, 0)).toEqual({ reason: "Trùng với danh mục An ninh" });
    expect(kq).toEqual({ ok: true, duLieu: null });
  });

  // Each refusal the card names, with the sentence the server writes for it
  // (service-comms/internal/http/noi_dung_mini_app.go). The screen shows it AS IT CAME: `KetQua` carries
  // no `code` on purpose (`goi.ts`), so a client-side copy of these sentences could only drift.
  const REFUSALS: { name: string; status: number; code: string; message: string; call: () => Promise<{ ok: boolean }> }[] = [
    {
      name: "self parent (400)",
      status: 400,
      code: "invalid_request",
      message: "danh_muc_mini_app: một danh mục không thể là cha của chính nó",
      call: () => updateContentCategory("01JDM1", { parent_id: "01JDM1" }),
    },
    {
      name: "slug sent (400)",
      status: 400,
      code: "invalid_request",
      message: "danh_muc_mini_app: `slug` đã cấp thì không đổi được — sửa `name`, hoặc thêm danh mục mới",
      call: () => updateContentCategory("01JDM1", { name: "x" }),
    },
    {
      name: "category_cycle (409)",
      status: 409,
      code: "category_cycle",
      message: "Không đặt được danh mục cha là một danh mục con (hoặc cháu) của chính nó.",
      call: () => updateContentCategory("01JDM1", { parent_id: "01JDM3" }),
    },
    {
      name: "parent_missing (409)",
      status: 409,
      code: "parent_missing",
      message:
        "Danh mục cha đã chọn không còn trong danh mục tin của xã. Hãy tải lại trang và chọn lại.",
      call: () => updateContentCategory("01JDM1", { parent_id: "01JGONE" }),
    },
    {
      name: "category_not_empty (409)",
      status: 409,
      code: "category_not_empty",
      message:
        "Danh mục còn nội dung hoặc danh mục con nên không xoá được. " +
        "Hãy ẩn danh mục nếu không muốn bà con thấy nó trên Mini App.",
      call: () => deleteContentCategory("01JDM1", "Không dùng nữa"),
    },
  ];

  for (const r of REFUSALS) {
    it(`${r.name}: the server's Vietnamese sentence reaches the screen verbatim`, async () => {
      batFetch(traJSON(r.status, { code: r.code, message: r.message, trace_id: "t-1" }));
      const kq = (await r.call()) as { ok: boolean; thongBao?: string };
      expect(kq.ok).toBe(false);
      // §4.3: verbatim minus one leading technical tag (`danh_muc_mini_app: `), see `goi.ts`.
      expect(kq.thongBao).toBe(stripTechnicalPrefix(r.message));
      expect(kq.thongBao).not.toMatch(/^[a-z_]+: /);
      // Neither the machine code nor the trace id is shown to the officer.
      expect(kq.thongBao).not.toContain(r.code);
      expect(kq.thongBao).not.toContain("t-1");
    });
  }

  it("item DELETE: the reason IN THE BODY, never in the URL; 204 is success", async () => {
    const gia = batFetch(new Response(null, { status: 204 }));
    const r = await deleteContentItem("01J/ND 1", "Đăng nhầm bài của thôn khác");
    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/content-items/01J%2FND%201");
    expect(duongDan).toBe(contentItemDeletePath("01J/ND 1"));
    expect(duongDan).not.toContain("?");
    expect(duongDan).not.toContain("nh%E1%BA%A7m");
    expect(tuyChon.method).toBe("DELETE");
    // same-origin, never "include": the session cookie stays on this commune's host (rule 1, forbidden #3).
    expect(tuyChon.credentials).toBe("same-origin");
    expect(thanDaGui(gia, 0)).toEqual({ reason: "Đăng nhầm bài của thôn khác" });
    expect(r).toEqual({ ok: true, data: null });
  });

  const ITEM_DELETE_REFUSALS: { status: number; code: string; message: string }[] = [
    {
      status: 400,
      code: "invalid_request",
      message: "Hãy nhập lý do xoá. Nội dung đã đăng là hồ sơ lưu trữ, xoá phải ghi rõ vì sao.",
    },
    { status: 400, code: "invalid_request", message: "Lý do xoá quá dài (tối đa 500 ký tự)." },
    { status: 404, code: "not_found", message: "Không tìm thấy nội dung này." },
    { status: 403, code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." },
  ];

  for (const r of ITEM_DELETE_REFUSALS) {
    it(`item DELETE ${r.status} (${r.message.slice(0, 24)}…): status kept, server sentence verbatim`, async () => {
      batFetch(traJSON(r.status, { code: r.code, message: r.message, trace_id: "t-1" }));
      const kq = await deleteContentItem("01JND1", "x");
      expect(kq).toEqual({ ok: false, status: r.status, message: r.message });
    });
  }

  it("item DELETE with no answer at all: status 0 and the generic sentence", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("network"))));
    const kq = await deleteContentItem("01JND1", "x");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.status).toBe(0);
  });

  it("a banner 422 (`banner_cover_required`) on POST reaches the form verbatim too", async () => {
    const message =
      "noi_dung_mini_app: banner phải có ảnh bìa — tải ảnh lên trước khi lưu, và không gỡ ảnh khỏi banner";
    batFetch(traJSON(422, { code: "banner_cover_required", message }));
    const kq = await themNoiDung({ ...BANNER_FULL, cover_image_file_id: "" }, "k");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("Banner phải có ảnh bìa — tải ảnh lên trước khi lưu, và không gỡ ảnh khỏi banner");
  });

  it("POST sends the banner fields for a banner only", async () => {
    const gia = batFetch(traJSON(201, {}));
    await themNoiDung(BANNER_FULL, "k");
    await themNoiDung({ ...BANNER_FULL, type: "tin-tuc" }, "k");
    expect(thanDaGui(gia, 0)).toMatchObject({ link_to: "/tin-tuc", display_order: 2 });
    expect(thanDaGui(gia, 1)).not.toHaveProperty("link_to");
    expect(thanDaGui(gia, 1)).not.toHaveProperty("display_order");
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
  paths: Record<
    string,
    Record<
      string,
      {
        "x-vigov-permission"?: { key?: string };
        requestBody?: { content?: Record<string, { schema?: LuocDo }> };
      }
    >
  >;
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
    await themNoiDung(EVENT_FULL, "k");
    await themNoiDung(VIDEO_FULL, "k");
    await themNoiDung(BANNER_FULL, "k");

    // Thân đầy đủ: mọi trường tuỳ chọn đều có giá trị, nên bộ khoá gửi đi phải trùng KHÍT bộ khoá
    // hợp đồng. Thừa một trường là gửi thứ máy chủ không nhận; thiếu một trường là một ô biểu mẫu
    // không bao giờ tới nơi. UNION of the event, video and banner bodies: no one type carries all.
    const sent = new Set([0, 1, 2].flatMap((n) => Object.keys(thanDaGui(gia, n))));
    expect([...sent].sort()).toEqual(cuaHopDong.slice().sort());
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

  it("an item and a category each have ONE `DELETE`, and both take a reason in the body", () => {
    // Replaces the test that pinned the ABSENCE of the item's DELETE (it was written to go red the day
    // the route landed, 5bb1d962). Both deletes are soft, with a mandatory reason (rule 7).
    const p = hopDong().paths;
    const methods = (path: string) => Object.keys(p[path] ?? {}).filter((k) => k !== "parameters").sort();
    expect(methods("/api/v1/content-items/{id}")).toEqual(["delete", "get", "patch"]);
    expect(methods("/api/v1/content-categories/{id}")).toEqual(["delete", "patch"]);
    expect(Object.keys(p["/api/v1/content-items"] ?? {})).not.toContain("delete");
    expect(Object.keys(p["/api/v1/content-categories"] ?? {})).not.toContain("delete");
    expect(khoaCuaLuocDo("comms.deleteContentItemIn")).toEqual(["reason"]);
  });

  it("`deleteContentItem` sends EXACTLY the keys of `comms.deleteContentItemIn`", async () => {
    const cuaHopDong = khoaCuaLuocDo("comms.deleteContentItemIn");
    const gia = batFetch(new Response(null, { status: 204 }));
    await deleteContentItem("01JND1", "Đăng trùng với bài ngày 14/9");
    expect(Object.keys(thanDaGui(gia, 0)).sort()).toEqual(cuaHopDong.slice().sort());
  });

  it("`updateContentCategory` sends at most the keys of `comms.updateCategoryIn`, never `slug`", async () => {
    const cuaHopDong = khoaCuaLuocDo("comms.updateCategoryIn");
    const gia = batFetch(traJSON(200, {}));
    await updateContentCategory("01JDM1", { name: "A", parent_id: "", order: 1, hidden: true });
    const sent = Object.keys(thanDaGui(gia, 0)).sort();
    expect(sent).toEqual(["hidden", "name", "order", "parent_id"]);
    // The contract still lists `slug` — only so the server can REFUSE it (400). Every other key matches.
    expect(cuaHopDong.filter((k) => k !== "slug").sort()).toEqual(sent);
  });

  it("`deleteContentCategory` sends EXACTLY the keys of `comms.deleteCategoryIn`", async () => {
    const cuaHopDong = khoaCuaLuocDo("comms.deleteCategoryIn");
    const gia = batFetch(new Response(null, { status: 204 }));
    await deleteContentCategory("01JDM1", "Gộp vào danh mục khác");
    expect(Object.keys(thanDaGui(gia, 0)).sort()).toEqual(cuaHopDong.slice().sort());
  });

  it("tám tuyến vẫn đứng sau đúng hai khoá màn đang giả định", () => {
    // The screen hides its write controls behind `content.update` (`canEditContent`, 02/10/2026) on the
    // ASSUMPTION that every write route declares exactly that key. This test is where the day the
    // server splits the key of one of these routes turns red.
    const p = hopDong().paths;
    const khoa = (duong: string, pt: string) => p[duong]?.[pt]?.["x-vigov-permission"]?.key;

    expect(khoa("/api/v1/content-items", "get")).toBe("content.read");
    expect(khoa("/api/v1/content-items/{id}", "get")).toBe("content.read");
    expect(khoa("/api/v1/content-categories", "get")).toBe("content.read");
    expect(khoa("/api/v1/content-items", "post")).toBe("content.update");
    expect(khoa("/api/v1/content-items/{id}", "patch")).toBe("content.update");
    expect(khoa("/api/v1/content-items/{id}", "delete")).toBe("content.update");
    expect(khoa("/api/v1/content-categories", "post")).toBe("content.update");
    expect(khoa("/api/v1/content-items/cover-images", "post")).toBe("content.update");
    // The completion route is GONE (ADR 0052 §Sửa đổi 09/10/2026): nothing here may still call it.
    expect(p["/api/v1/content-items/cover-images/{id}/completion"]).toBeUndefined();
    expect(khoa("/api/v1/content-categories/{id}", "patch")).toBe("content.update");
    expect(khoa("/api/v1/content-categories/{id}", "delete")).toBe("content.update");
  });

  it("the cover upload sends the parts the contract's multipart schema lists, in ITS order", async () => {
    const schema = hopDong().paths["/api/v1/content-items/cover-images"]?.post?.requestBody?.content?.["multipart/form-data"]
      ?.schema as { properties?: Record<string, unknown> } | undefined;
    const cuaHopDong = Object.keys(schema?.properties ?? {});
    expect(cuaHopDong.length).toBeGreaterThan(0);
    const sent = installFakeUploadXHR(() => ({ status: 201, body: COVER_READY }));
    await uploadCoverImage({ file: jpg(), contentType: "image/jpeg", contentItemId: "01JND1" }, "k");
    expect(partNames(sent[0])).toEqual(cuaHopDong);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BA TÊN BỘ LỌC — ĐỌC THẲNG MÃ MÁY CHỦ, VÌ CLIENT KHÔNG DỰNG TRUY VẤN THEO KIỂU HỢP ĐỒNG
 *
 * Hợp đồng nay khai `type` · `category` · `q` trong `comms_get_content_items["truyVan"]`, nhưng
 * đánh dấu cả ba là bắt buộc, trong khi handler coi tham số vắng là "không lọc". Client vì thế chỉ
 * gửi bộ lọc đang có giá trị và không dựng URL theo kiểu ấy — tức ba tên vẫn không có kiểu nào canh.
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
      duongDanSoNoiDung({ loai: "su-kien", danhMucID: "01JDM1", tim: "x", status: "cho-duyet" }),
      "https://xa.example",
    ).searchParams;

    // `status` is the fourth (02/10/2026, C1) — the screen draws it as the `Trạng thái` select.
    expect(cuaMayChu.slice().sort()).toEqual(["category", "q", "status", "type"]);
    for (const ten of cuaMayChu) {
      expect(gui.has(ten), `màn hình phải gửi được bộ lọc \`${ten}\``).toBe(true);
    }
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §4 — how many staff residents see (`GET /api/v1/commune-staff?host=`, identity, public)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("`countPublishedStaff` — §4's number", () => {
  it("asks for exactly the domain it is given, as the one `host` parameter the route reads", () => {
    expect(publishedStaffPath("xa-a.example.vn")).toBe("/api/v1/commune-staff?host=xa-a.example.vn");
    // The name on the wire is the contract's (`identity_get_commune_staff["truyVan"]`), not a guess.
    const p = hopDong().paths["/api/v1/commune-staff"]?.["get"] as unknown as {
      parameters?: { name: string; in: string }[];
    };
    expect(p.parameters?.map((x) => `${x.in}:${x.name}`)).toEqual(["query:host"]);
  });

  it("a 200 gives the COUNT only — the names and numbers of the reply never leave the function", async () => {
    const gia = batFetch(
      traJSON(200, {
        items: [
          { full_name: "Nguyễn Văn A", position: "Chủ tịch", department_name: "", phone: "", mobile: "0900000000", has_zalo: true },
          { full_name: "Trần Thị B", position: "Văn thư", department_name: "", phone: "", mobile: "", has_zalo: false },
        ],
      }),
    );
    const kq = await countPublishedStaff("xa-a.example.vn");
    expect(kq).toEqual({ ok: true, duLieu: 2 });
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/commune-staff?host=xa-a.example.vn");
    expect(loiGoi(gia, 0).tuyChon.method).toBe("GET");
  });

  it("a refusal or a dead network is a failure, NEVER a count of 0", async () => {
    batFetch(traJSON(400, { code: "host_invalid", message: "Tên miền không hợp lệ.", trace_id: "t" }));
    const refused = await countPublishedStaff("localhost");
    expect(refused.ok).toBe(false);

    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("offline"))));
    const offline = await countPublishedStaff("xa-a.example.vn");
    expect(offline.ok).toBe(false);
  });

  it("the route is the public one the Mini App reads — the staff screen counts the SAME set residents see", () => {
    const op = hopDong().paths["/api/v1/commune-staff"]?.["get"] as unknown as {
      "x-vigov-permission"?: { kind?: string };
    };
    expect(op["x-vigov-permission"]?.kind).toBe("public");
  });
});
