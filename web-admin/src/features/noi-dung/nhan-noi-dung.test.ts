import { describe, expect, it } from "vitest";

import type { comms_danhMucRa, comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  CANH_BAO_HTML_THO,
  categoryPatchBody,
  CHUA_XEP_DANH_MUC,
  coThayDoi,
  DAU_GACH,
  dungCayDanhMuc,
  ERR_EVENT_END_BEFORE_START,
  ERR_EVENT_END_WITHOUT_START,
  ERR_BANNER_COVER_REQUIRED,
  ERR_DISPLAY_ORDER_CANNOT_CLEAR,
  ERR_DISPLAY_ORDER_INVALID,
  ERR_EVENT_PLACE_TOO_LONG,
  ERR_LINK_TO_INVALID,
  ERR_VIDEO_URL_INVALID,
  EVENT_PLACE_MAX_CHARS,
  FORM_TRONG,
  formatVietnamDateTime,
  giaTriTuHang,
  instantToLocalInput,
  isValidLinkTo,
  localInputToInstant,
  parentChoices,
  parseDisplayOrder,
  publishedAtLabel,
  validateTypeFields,
  LOAI_MAC_DINH,
  lopChipTrangThai,
  MOI_LOAI,
  nhanLoai,
  nhanMoc,
  nhanMucDanhMuc,
  nhanNgayDang,
  nhanNguon,
  nhanTepDinhKem,
  nhanTrangThai,
  PHAN_CHUA_DUNG,
  selfAndDescendants,
  tenDanhMuc,
  thanBaiNeuCo,
  thanSua,
  thanThem,
  trichTomTat,
  type GiaTriFormNoiDung,
} from "./nhan-noi-dung";

/**
 * Phép quyết định của màn Nội dung Mini App, kiểm từng phép một.
 *
 * HAI NHÓM QUAN TRỌNG NHẤT Ở ĐÂY:
 *
 *   `thanSua`   — ở máy chủ mỗi trường là một CON TRỎ. Gửi thừa một trường là đổi một thứ không ai
 *                 yêu cầu, và với `publish` thì đó là gỡ một bài khỏi Mini App của cả xã trong im
 *                 lặng. Không màn hình nào nhìn thấy được điều đó.
 *   `nhanNgayDang` — `published_on` là một NGÀY, không phải một mốc. Một phép đưa nó qua `Date` in
 *                 đúng trên máy người viết và lệch một ngày ở múi giờ khác.
 */

function danhMuc(sua: Partial<comms_danhMucRa> = {}): comms_danhMucRa {
  return {
    id: "01JDM1",
    name: "Chuyển đổi số",
    slug: "chuyen-doi-so",
    parent_id: "",
    order: 0,
    created_at: "2026-09-01T02:00:00Z",
    hidden: false,
    ...sua,
  };
}

function hang(sua: Partial<comms_noiDungRa> = {}): comms_noiDungRa {
  return {
    id: "01JND1",
    type: "tin-tuc",
    category_id: "01JDM1",
    title: "Xã tổ chức hội nghị tổng kết công tác chuyển đổi số",
    summary: "Hội nghị diễn ra sáng 14/9 tại hội trường UBND xã.",
    image_url: "https://cdn.example.vn/anh/hoi-nghi.jpg",
    has_image: true,
    published_on: "2026-09-14",
    view_count: 0,
    status: "dang-hien",
    source: "thu-cong",
    source_url: "",
    source_ref: "",
    hand_edited: false,
    author_code: "CB-2026-7K3M9Q",
    created_at: "2026-09-14T02:00:00Z",
    updated_at: "2026-09-14T02:00:00Z",
    ...sua,
  };
}

describe("ngày đăng §6 — `d/M/yyyy`, KHÔNG đi qua `Date`", () => {
  it("cắt chuỗi ngày thành khuôn của đặc tả, không đệm số 0", () => {
    expect(nhanNgayDang("2026-09-14")).toBe("14/9/2026");
    expect(nhanNgayDang("2026-01-05")).toBe("5/1/2026");
    expect(nhanNgayDang("2026-12-31")).toBe("31/12/2026");
  });

  it("ngày ĐẦU THÁNG không lùi một ngày — đây là ca mà một phép qua `Date` sẽ hỏng", () => {
    // `new Date("2026-03-01")` là nửa đêm UTC; in lại ở một múi giờ âm cho ra 28/2. Phép cắt chuỗi
    // không có gì để lệch. `vitest.config.mts` ghim `TZ=UTC` nên ca này không tự bắt được lỗi ấy —
    // nó đứng đây để nói rõ vì sao hàm không dùng `Date`.
    expect(nhanNgayDang("2026-03-01")).toBe("1/3/2026");
  });

  it("rỗng thì dấu gạch, sai khuôn thì hiện NGUYÊN VĂN", () => {
    expect(nhanNgayDang("")).toBe(DAU_GACH);
    expect(nhanNgayDang(null)).toBe(DAU_GACH);
    // Một ngày đoán sai trông y hệt một ngày đúng; dấu gạch thì nói dối rằng máy chủ không gửi gì.
    expect(nhanNgayDang("14/09/2026")).toBe("14/09/2026");
    expect(nhanNgayDang("2026-09-14T02:00:00Z")).toBe("2026-09-14T02:00:00Z");
  });
});

describe("mốc `date-time` — múi giờ GHIM", () => {
  it("in giờ Việt Nam kể cả khi máy chạy ở UTC", () => {
    // `vitest.config.mts` ghim `TZ=UTC`, nên bỏ `timeZone` khỏi bộ định dạng thì ca này đỏ ngay.
    expect(nhanMoc("2026-09-07T09:35:00Z")).toBe("16:35 07/09/2026");
  });

  it("qua nửa đêm giờ Việt Nam thì sang ngày hôm sau", () => {
    expect(nhanMoc("2026-09-07T17:30:00Z")).toBe("00:30 08/09/2026");
  });

  it("rỗng thì dấu gạch, không đọc được thì nguyên văn", () => {
    expect(nhanMoc(null)).toBe(DAU_GACH);
    expect(nhanMoc("")).toBe(DAU_GACH);
    expect(nhanMoc("hôm qua")).toBe("hôm qua");
  });
});

describe("sáu loại §5 và ba trạng thái §6", () => {
  it("đủ sáu mã, đúng thứ tự tab đặc tả vẽ", () => {
    expect([...MOI_LOAI]).toEqual([
      "tin-tuc",
      "su-kien",
      "thong-bao",
      "truyen-thanh",
      "video",
      "banner",
    ]);
    expect(LOAI_MAC_DINH).toBe("tin-tuc");
  });

  it("nhãn đúng chữ đặc tả; mã lạ hiện NGUYÊN VĂN chứ không thành dấu gạch", () => {
    expect(nhanLoai("truyen-thanh")).toBe("Truyền thanh");
    expect(nhanLoai("podcast")).toBe("podcast");

    expect(nhanTrangThai("dang-hien")).toBe("Đang hiện");
    expect(nhanTrangThai("cho-duyet")).toBe("Chờ duyệt");
    expect(nhanTrangThai("an")).toBe("Ẩn");
    expect(nhanTrangThai("da-go")).toBe("da-go");
  });

  it("chỉ `dang-hien` mang lớp chip xanh", () => {
    expect(lopChipTrangThai("dang-hien")).toBe("chip chip-hoat-dong");
    expect(lopChipTrangThai("cho-duyet")).toBe("chip chip-ngung");
    expect(lopChipTrangThai("an")).toBe("chip chip-ngung");
  });

  it("hai nguồn §8, mã lạ nguyên văn", () => {
    expect(nhanNguon("thu-cong")).toBe("Soạn tay");
    expect(nhanNguon("dong-bo-cong")).toBe("Đồng bộ từ Cổng");
    expect(nhanNguon("khac")).toBe("khac");
  });
});

describe("hai ô nhỏ của bảng §6", () => {
  it("`Tệp đính kèm` suy từ `has_image`", () => {
    expect(nhanTepDinhKem(true)).toBe("🔗 Có ảnh");
    expect(nhanTepDinhKem(false)).toBe(DAU_GACH);
  });

  it("tóm tắt gộp về một dòng rồi cắt", () => {
    expect(trichTomTat("  hai   dòng\nnối lại  ")).toBe("hai dòng nối lại");
    expect(trichTomTat("")).toBe("");
    expect(trichTomTat("abcdefghij", 5)).toBe("abcde…");
  });
});

describe("thân bài: ba kết quả, không hai", () => {
  it("hàng của DANH SÁCH không mang thân — trả `null`, không trả chuỗi rỗng", () => {
    // Trộn hai ca lại là hiện một bài rỗng mà không dòng nào nói ra rằng màn hình chưa đi hỏi.
    expect(thanBaiNeuCo(hang())).toBeNull();
    expect(thanBaiNeuCo(hang({ body: null }))).toBeNull();
  });

  it("hàng của CHI TIẾT mang thân — kể cả khi thân rỗng", () => {
    expect(thanBaiNeuCo(hang({ body: "" }))).toBe("");
    expect(thanBaiNeuCo(hang({ body: "<p>x</p>" }))).toBe("<p>x</p>");
  });
});

describe("cây danh mục dựng từ danh sách phẳng", () => {
  it("xếp theo `order` rồi theo tên, con nằm ngay dưới cha", () => {
    const cay = dungCayDanhMuc([
      danhMuc({ id: "B", name: "Kinh tế", order: 2 }),
      danhMuc({ id: "A", name: "Danh mục", order: 1 }),
      danhMuc({ id: "A1", name: "Chuyển đổi số", parent_id: "A", order: 1 }),
      danhMuc({ id: "A2", name: "Công khai ngân sách", parent_id: "A", order: 0 }),
    ]);

    expect(cay.map((m) => m.dm.id)).toEqual(["A", "A2", "A1", "B"]);
    expect(cay.map((m) => m.muc)).toEqual([0, 1, 1, 0]);
  });

  it("danh mục có cha KHÔNG nằm trong danh sách vẫn ra màn hình, ở mức gốc", () => {
    // Bỏ nó đi là làm một danh mục biến mất khỏi ô chọn mà không dòng nào nói ra.
    const cay = dungCayDanhMuc([danhMuc({ id: "X", parent_id: "KHONG-CO" })]);
    expect(cay.map((m) => m.dm.id)).toEqual(["X"]);
    expect(cay[0]?.muc).toBe(0);
  });

  it("một chu trình `parent_id` KHÔNG làm mất hàng và KHÔNG làm treo phép duyệt", () => {
    const cay = dungCayDanhMuc([
      danhMuc({ id: "P", parent_id: "Q" }),
      danhMuc({ id: "Q", parent_id: "P" }),
    ]);
    expect(cay.map((m) => m.dm.id).sort()).toEqual(["P", "Q"]);
  });

  it("danh sách rỗng cho cây rỗng", () => {
    expect(dungCayDanhMuc([])).toEqual([]);
  });

  it("nhãn ô chọn thụt đầu dòng theo độ sâu", () => {
    expect(nhanMucDanhMuc({ dm: danhMuc({ name: "Danh mục" }), muc: 0 })).toBe("Danh mục");
    expect(nhanMucDanhMuc({ dm: danhMuc({ name: "Chuyển đổi số" }), muc: 1 })).toContain(
      "└ Chuyển đổi số",
    );
  });
});

describe("tên danh mục của một bài", () => {
  const ds = [danhMuc({ id: "01JDM1", name: "Chuyển đổi số" })];

  it("id rỗng là một trạng thái BÌNH THƯỜNG, không phải một giá trị thiếu", () => {
    expect(tenDanhMuc("", ds)).toBe(CHUA_XEP_DANH_MUC);
  });

  it("id tra được thì hiện tên", () => {
    expect(tenDanhMuc("01JDM1", ds)).toBe("Chuyển đổi số");
  });

  it("id KHÔNG tra được hiện chính id, không hiện dấu gạch", () => {
    // Dấu gạch nói "chưa xếp danh mục"; sự thật là cây danh mục chưa nạp xong. Hai câu khác nhau.
    expect(tenDanhMuc("01JLA", ds)).toBe("01JLA");
  });
});

describe("biểu mẫu §7 — giá trị ban đầu", () => {
  it("biểu mẫu trống mặc định loại `Tin tức` và ô tích TẮT", () => {
    expect(FORM_TRONG.type).toBe("tin-tuc");
    expect(FORM_TRONG.publish).toBe(false);
  });

  it("ô tích BẬT chỉ khi bài đang `dang-hien`", () => {
    expect(giaTriTuHang(hang({ status: "dang-hien", body: "" })).publish).toBe(true);
    expect(giaTriTuHang(hang({ status: "an", body: "" })).publish).toBe(false);
    // `cho-duyet` hiện ra là ô TẮT — đúng nghĩa "bà con chưa thấy".
    expect(giaTriTuHang(hang({ status: "cho-duyet", body: "" })).publish).toBe(false);
  });

  it("thân bài lấy từ hàng chi tiết; hàng danh sách cho ô rỗng", () => {
    expect(giaTriTuHang(hang({ body: "<p>x</p>" })).body).toBe("<p>x</p>");
    expect(giaTriTuHang(hang()).body).toBe("");
  });

  it("the cover starts as the article's own `cover_image_file_id`, `\"\"` when it has none", () => {
    expect(giaTriTuHang(hang({ cover_image_file_id: "01JCOVER0" })).cover_image_file_id).toBe("01JCOVER0");
    expect(giaTriTuHang(hang()).cover_image_file_id).toBe("");
    // The legacy link is not a form value: no box edits it, so nothing can send it back.
    expect(Object.keys(giaTriTuHang(hang()))).not.toContain("image_url");
  });
});

describe("thân POST", () => {
  it("cắt hai đầu tiêu đề và tóm tắt — KHÔNG cắt thân bài", () => {
    const than = thanThem({
      ...FORM_TRONG,
      type: "su-kien",
      category_id: "01JDM1",
      title: "  Hội nghị  ",
      summary: "  tóm tắt  ",
      body: "  <p>x</p>  ",
      cover_image_file_id: "01JCOVER1",
      publish: true,
    });

    expect(than.title).toBe("Hội nghị");
    expect(than.summary).toBe("tóm tắt");
    expect(than.cover_image_file_id).toBe("01JCOVER1");
    // Máy chủ tự cắt hai đầu thân bài khi lưu. Cắt thêm ở đây chỉ tạo một chỗ nữa để hai bên lệch.
    expect(than.body).toBe("  <p>x</p>  ");
  });

  it("gửi đủ sáu trường, không thừa trường nào — không `image_url`, không ảnh khi chưa tải", () => {
    const than = thanThem(FORM_TRONG);
    expect(Object.keys(than).sort()).toEqual([
      "body",
      "category_id",
      "publish",
      "summary",
      "title",
      "type",
    ]);
  });

  it("an uploaded cover goes with the create as `cover_image_file_id`", () => {
    expect(Object.keys(thanThem({ ...FORM_TRONG, cover_image_file_id: "01JCOVER1" }))).toContain(
      "cover_image_file_id",
    );
  });
});

describe("thân PATCH — CHỈ những ô thật sự đổi", () => {
  const dau: GiaTriFormNoiDung = {
    ...FORM_TRONG,
    type: "tin-tuc",
    category_id: "01JDM1",
    title: "Tiêu đề cũ",
    summary: "Tóm tắt cũ",
    body: "<p>thân cũ</p>",
    cover_image_file_id: "01JCOVER0",
    publish: true,
  };

  it("không đổi gì thì thân RỖNG, và màn không được gọi `PATCH`", () => {
    const than = thanSua(dau, dau);
    expect(than).toEqual({});
    expect(coThayDoi(than)).toBe(false);
  });

  it("sửa mỗi tiêu đề thì thân CHỈ có `title`", () => {
    const than = thanSua(dau, { ...dau, title: "Tiêu đề mới" });
    expect(than).toEqual({ title: "Tiêu đề mới" });
    expect(coThayDoi(than)).toBe(true);
  });

  it("⚠ KHÔNG gửi `publish` khi cán bộ không đụng ô tích — đây là ca đắt nhất của màn", () => {
    // Bài đang `cho-duyet` hiện ra với ô tích TẮT. Gửi `publish: false` kèm một lần sửa chính tả
    // sẽ lặng lẽ chuyển nó thành `an`: bà con không còn thấy bài, không gì đỏ, không ai được báo.
    const dauChoDuyet: GiaTriFormNoiDung = { ...dau, publish: false };
    const than = thanSua(dauChoDuyet, { ...dauChoDuyet, title: "Sửa chính tả" });

    expect(than).not.toHaveProperty("publish");
    expect(Object.keys(than)).toEqual(["title"]);
  });

  it("gửi `publish: false` khi cán bộ THẬT SỰ tắt ô tích — đó là cách gỡ bài khỏi Mini App", () => {
    expect(thanSua(dau, { ...dau, publish: false })).toEqual({ publish: false });
  });

  it("xoá trắng tóm tắt gửi `\"\"`, không phải bỏ trường đi", () => {
    // `""` là "không có tóm tắt"; trường VẮNG là "để nguyên". Lẫn hai cái là một ô cán bộ vừa xoá
    // mà không xoá được.
    expect(thanSua(dau, { ...dau, summary: "" })).toEqual({ summary: "" });
  });

  it("chỉ thêm khoảng trắng vào tiêu đề thì KHÔNG coi là đổi", () => {
    // Máy chủ cắt hai đầu khi lưu, nên `"  Tiêu đề cũ  "` lưu xuống vẫn là giá trị cũ. Gửi nó đi
    // là một lần ghi không thay đổi gì — và một lần ghi như thế vẫn gắn cờ §10.4 lên bài.
    expect(thanSua(dau, { ...dau, title: "  Tiêu đề cũ  " })).toEqual({});
  });

  it("thân bài KHÔNG bị cắt hai đầu khi so sánh", () => {
    // Nếu so bằng `body.trim()` thì mọi lần mở lại một bài có khoảng trắng đầu dòng đều báo "đã
    // đổi", và mọi lần Lưu đều ghi đè thân bài dù không ai sửa.
    expect(thanSua(dau, { ...dau, body: " <p>thân cũ</p> " })).toEqual({
      body: " <p>thân cũ</p> ",
    });
  });

  it("đổi hết bảy ô thì gửi đủ bảy trường", () => {
    const than = thanSua(dau, {
      ...FORM_TRONG,
      type: "video",
      category_id: "",
      title: "T",
      summary: "S",
      body: "B",
      cover_image_file_id: "",
      publish: false,
    });
    expect(Object.keys(than).sort()).toEqual([
      "body",
      "category_id",
      "cover_image_file_id",
      "publish",
      "summary",
      "title",
      "type",
    ]);
  });

  it("`Gỡ ảnh` on an article that had a cover sends `\"\"` — the server's DETACH", () => {
    expect(thanSua(dau, { ...dau, cover_image_file_id: "" })).toEqual({ cover_image_file_id: "" });
  });

  it("a new upload sends its id; an untouched cover is absent (leave alone)", () => {
    expect(thanSua(dau, { ...dau, cover_image_file_id: "01JCOVER2" })).toEqual({
      cover_image_file_id: "01JCOVER2",
    });
    expect(thanSua(dau, { ...dau, title: "x" })).not.toHaveProperty("cover_image_file_id");
  });

  it("no cover before, none after: nothing about the cover is sent", () => {
    const bare: GiaTriFormNoiDung = { ...dau, cover_image_file_id: "" };
    expect(thanSua(bare, { ...bare, title: "x" })).toEqual({ title: "x" });
  });

  it("NEVER sends `image_url` — the legacy link has no box on this form", () => {
    expect(thanSua(dau, { ...dau, title: "x", cover_image_file_id: "" })).not.toHaveProperty("image_url");
  });

  it("KHÔNG BAO GIỜ gửi `status`, `source`, `author_code` hay `view_count`", () => {
    // Bốn thứ máy chủ tự quyết. `thanSua` chỉ biết bảy ô của biểu mẫu, nên không có đường nào cho
    // chúng đi qua — ca này canh chính điều đó, phòng ngày ai đó nới `GiaTriFormNoiDung`.
    const than = thanSua(dau, { ...dau, title: "x" });
    for (const cam of ["status", "source", "author_code", "view_count", "hand_edited"]) {
      expect(than).not.toHaveProperty(cam);
    }
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PER-TYPE FIELDS (ADR 0047 §6) — `vitest.config.mts` pins TZ=UTC, so a conversion that consulted
 * the machine's zone would be off by seven hours here and turn red.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("event instants — Vietnam wall-clock, sent with +07:00", () => {
  it("a datetime-local value is sent as the same wall-clock time with +07:00, not the machine's zone", () => {
    expect(localInputToInstant("2026-10-05T08:00")).toBe("2026-10-05T08:00:00+07:00");
    expect(localInputToInstant("2026-10-05T08:00:30")).toBe("2026-10-05T08:00:30+07:00");
    expect(localInputToInstant("")).toBe("");
  });

  it("the sent instant is the right moment: 08:00 in Vietnam is 01:00 UTC", () => {
    expect(new Date(localInputToInstant("2026-10-05T08:00")).toISOString()).toBe(
      "2026-10-05T01:00:00.000Z",
    );
  });

  it("an instant from the server (UTC) comes back into the box in Vietnam time", () => {
    expect(instantToLocalInput("2026-10-05T01:00:00Z")).toBe("2026-10-05T08:00");
    // Crossing midnight: 17:30 UTC is 00:30 the next day in Vietnam, and midnight is `00`, not `24`.
    expect(instantToLocalInput("2026-10-04T17:30:00Z")).toBe("2026-10-05T00:30");
    expect(instantToLocalInput("2026-10-04T17:00:00Z")).toBe("2026-10-05T00:00");
    expect(instantToLocalInput(undefined)).toBe("");
    expect(instantToLocalInput(null)).toBe("");
  });

  it("round trip is the identity", () => {
    const typed = "2026-12-31T23:45";
    const iso = new Date(localInputToInstant(typed)).toISOString();
    expect(instantToLocalInput(iso)).toBe(typed);
  });
});

describe("`Đăng lần đầu lúc` — published_at in dd/MM/yyyy HH:mm, Vietnam time", () => {
  it("formats the first-publication instant", () => {
    expect(formatVietnamDateTime("2026-10-01T02:05:00Z")).toBe("01/10/2026 09:05");
    expect(publishedAtLabel(hang({ published_at: "2026-10-01T02:05:00Z" }))).toBe(
      "Đăng lần đầu lúc 01/10/2026 09:05",
    );
  });

  it("never published → no label, not a dash pretending it was", () => {
    expect(publishedAtLabel(hang())).toBeNull();
    expect(publishedAtLabel(hang({ published_at: null }))).toBeNull();
  });
});

describe("giá trị ban đầu — per-type fields from the detail row", () => {
  it("event instants become Vietnam wall-clock input values; place and video copied", () => {
    const gt = giaTriTuHang(
      hang({
        type: "su-kien",
        body: "",
        event_starts_at: "2026-10-05T01:00:00Z",
        event_ends_at: "2026-10-05T04:30:00Z",
        event_place: "Hội trường",
      }),
    );
    expect(gt.event_starts_local).toBe("2026-10-05T08:00");
    expect(gt.event_ends_local).toBe("2026-10-05T11:30");
    expect(gt.event_place).toBe("Hội trường");
    expect(gt.video_url).toBe("");
  });
});

describe("thân POST — per-type fields only for their type", () => {
  const event: GiaTriFormNoiDung = {
    ...FORM_TRONG,
    type: "su-kien",
    title: "Hội nghị",
    event_starts_local: "2026-10-05T08:00",
    event_ends_local: "2026-10-05T11:30",
    event_place: "  Hội trường UBND xã  ",
    video_url: "https://video.example.vn/x",
  };

  it("`Sự kiện` sends the two instants with +07:00 and the trimmed place, and no video link", () => {
    const than = thanThem(event);
    expect(than.event_starts_at).toBe("2026-10-05T08:00:00+07:00");
    expect(than.event_ends_at).toBe("2026-10-05T11:30:00+07:00");
    expect(than.event_place).toBe("Hội trường UBND xã");
    expect(than).not.toHaveProperty("video_url");
  });

  it("switching to `Tin tức` leaves the hidden event boxes behind — none is sent", () => {
    const than = thanThem({ ...event, type: "tin-tuc" });
    for (const k of ["event_starts_at", "event_ends_at", "event_place", "video_url"]) {
      expect(than).not.toHaveProperty(k);
    }
  });

  it("`Video` sends only the trimmed link", () => {
    const than = thanThem({ ...event, type: "video", video_url: " https://video.example.vn/x " });
    expect(than.video_url).toBe("https://video.example.vn/x");
    expect(than).not.toHaveProperty("event_starts_at");
    expect(than).not.toHaveProperty("event_place");
  });
});

describe("thân PATCH — per-type fields", () => {
  const dau: GiaTriFormNoiDung = {
    ...FORM_TRONG,
    type: "su-kien",
    title: "Hội nghị",
    event_starts_local: "2026-10-05T08:00",
    event_ends_local: "2026-10-05T11:30",
    event_place: "Hội trường",
  };

  it("untouched event boxes are not sent", () => {
    expect(thanSua(dau, { ...dau, title: "Hội nghị mới" })).toEqual({ title: "Hội nghị mới" });
  });

  it("a changed start is sent with +07:00", () => {
    expect(thanSua(dau, { ...dau, event_starts_local: "2026-10-05T07:30" })).toEqual({
      event_starts_at: "2026-10-05T07:30:00+07:00",
    });
  });

  it("⚠ emptying a box that had a value sends `\"\"` — absent would mean `leave it`", () => {
    expect(thanSua(dau, { ...dau, event_ends_local: "", event_place: "" })).toEqual({
      event_ends_at: "",
      event_place: "",
    });
  });

  it("emptying the video link sends `\"\"`", () => {
    const video: GiaTriFormNoiDung = { ...FORM_TRONG, type: "video", video_url: "https://v.vn/a" };
    expect(thanSua(video, { ...video, video_url: "" })).toEqual({ video_url: "" });
  });

  it("moving the type away sends only `type` — the server clears the old type's fields", () => {
    // Sending the leftover values would be a 400; sending `""` for them is redundant.
    expect(thanSua(dau, { ...dau, type: "tin-tuc" })).toEqual({ type: "tin-tuc" });
  });

  it("moving the type TO `Sự kiện` sends the boxes the author filled", () => {
    const news: GiaTriFormNoiDung = { ...FORM_TRONG, type: "tin-tuc", title: "T" };
    expect(
      thanSua(news, { ...news, type: "su-kien", event_starts_local: "2026-10-05T08:00" }),
    ).toEqual({ type: "su-kien", event_starts_at: "2026-10-05T08:00:00+07:00" });
  });
});

describe("client-side check of the per-type fields — mirrors the server", () => {
  const event: GiaTriFormNoiDung = { ...FORM_TRONG, type: "su-kien", title: "T" };

  it("an empty event is valid — every per-type field is optional", () => {
    expect(validateTypeFields(event)).toBeNull();
  });

  it("an end needs a start", () => {
    expect(validateTypeFields({ ...event, event_ends_local: "2026-10-05T11:30" })).toBe(
      ERR_EVENT_END_WITHOUT_START,
    );
  });

  it("an end before the start is refused; an end equal to the start is not", () => {
    expect(
      validateTypeFields({
        ...event,
        event_starts_local: "2026-10-05T08:00",
        event_ends_local: "2026-10-05T07:59",
      }),
    ).toBe(ERR_EVENT_END_BEFORE_START);
    expect(
      validateTypeFields({
        ...event,
        event_starts_local: "2026-10-05T08:00",
        event_ends_local: "2026-10-05T08:00",
      }),
    ).toBeNull();
  });

  it("the place is bounded in characters, as the server counts them", () => {
    expect(validateTypeFields({ ...event, event_place: "ạ".repeat(EVENT_PLACE_MAX_CHARS) })).toBeNull();
    expect(
      validateTypeFields({ ...event, event_place: "ạ".repeat(EVENT_PLACE_MAX_CHARS + 1) }),
    ).toBe(ERR_EVENT_PLACE_TOO_LONG);
  });

  it("the video link must be http(s), any case", () => {
    const video: GiaTriFormNoiDung = { ...FORM_TRONG, type: "video", title: "T" };
    expect(validateTypeFields({ ...video, video_url: "javascript:alert(1)" })).toBe(
      ERR_VIDEO_URL_INVALID,
    );
    expect(validateTypeFields({ ...video, video_url: "www.youtube.com/x" })).toBe(
      ERR_VIDEO_URL_INVALID,
    );
    expect(validateTypeFields({ ...video, video_url: "https://a.vn/x y" })).toBe(
      ERR_VIDEO_URL_INVALID,
    );
    expect(validateTypeFields({ ...video, video_url: "HTTPS://a.vn/x" })).toBeNull();
    expect(validateTypeFields({ ...video, video_url: "" })).toBeNull();
  });

  it("hidden boxes of another type never block a save", () => {
    expect(
      validateTypeFields({ ...FORM_TRONG, type: "tin-tuc", event_ends_local: "2026-10-05T11:30" }),
    ).toBeNull();
    expect(validateTypeFields({ ...event, video_url: "javascript:x" })).toBeNull();
  });
});

describe("phần chưa dựng được", () => {
  it("the list is PINNED: rich text, category edit/delete, banner, broadcast audio and the portal sync card left it when built", () => {
    // EXACT, not a floor: an item silently dropped and an item silently kept are both a block that
    // lies to the commune about what the screen does. ADR 0067 built five of them (§1, §2, §3, §4, §5);
    // the portal sync card brought two narrower gaps of its own; the status filter left on 02/10/2026
    // (`status` on GET content-items, C1), the meta count stays.
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual([
      "Con số `{n} chuyên mục` trên dòng tóm tắt của thẻ Đồng bộ Cổng (§3)",
      "Con số `Đang hiện 26 cán bộ cho bà con` trên thẻ Danh bạ chính quyền (§4)",
      "Cột `Lượt xem` (§6)",
      "Nút xoá một bài (§9 đề xuất `DELETE`)",
      "Bố cục §2: tabs thật, hai thẻ đầu màn, modal dạng lớp phủ, cắt tóm tắt đúng 1 dòng",
      "Cổng quyền `content.read` / `content.update` ở phía giao diện",
    ]);
  });

  it("no item still claims the built parts are missing, or that raw script reaches residents", () => {
    const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");
    expect(all).not.toContain("rich text");
    expect(all).not.toContain("Sửa và xoá một danh mục");
    expect(all).not.toContain("không có `PATCH`");
    expect(all).not.toContain("chưa có cột nào ở máy chủ");
    expect(all).not.toContain("<script>");
    expect(all).not.toContain("KHÔNG làm sạch");
    // The broadcast audio upload is built (ADR 0067 §4): no item may still say it is missing.
    expect(all).not.toContain("tệp âm thanh");
    expect(all).not.toContain("Truyền thanh");
    // The portal sync is built (ADR 0067 §2, comms fa7b8377): nothing may still name it as missing.
    expect(all).not.toContain("bộ lập lịch");
    expect(all).not.toContain("adapter HTTP");
    expect(all).not.toContain("Toàn bộ thẻ");
  });

  it("the note under the editor says sanitising is the SERVER's, and no longer calls it raw HTML", () => {
    expect(CANH_BAO_HTML_THO).toContain("Máy chủ làm sạch");
    expect(CANH_BAO_HTML_THO).not.toContain("MÃ NGUỒN HTML");
    expect(CANH_BAO_HTML_THO).not.toContain("KHÔNG làm sạch");
  });

  it("mỗi mục đều có lý do, không mục nào là một dòng tên suông", () => {
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten.length).toBeGreaterThan(10);
      expect(p.viSao.length).toBeGreaterThan(80);
    }
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * Banner fields (ADR 0067 §5) — mirror of domain.NormalizeLinkTo / CheckDisplayOrder / CheckBannerCover
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("banner `link_to`", () => {
  it("accepts an in-app path, an https URL (any case), and empty", () => {
    expect(isValidLinkTo("")).toBe(true);
    expect(isValidLinkTo("/tin-tuc")).toBe(true);
    expect(isValidLinkTo("/")).toBe(true);
    expect(isValidLinkTo("https://xa.gov.vn/tin/1")).toBe(true);
    expect(isValidLinkTo("HTTPS://xa.gov.vn")).toBe(true);
  });

  it("refuses what the server refuses", () => {
    for (const bad of [
      "//evil.example/x", // protocol-relative leaves the app
      "http://xa.gov.vn", // https only
      "javascript:alert(1)",
      "tin-tuc", // neither a path nor a URL
      "https://", // no host
      "https:///x",
      "https://gov.vn@evil.example/x", // userinfo
      "/a b", // whitespace
      "/a\\b", // backslash
      "/" + "a".repeat(500), // 501 characters
    ]) {
      expect(isValidLinkTo(bad), bad).toBe(false);
    }
  });

  it("`display_order` is a non-negative INT", () => {
    expect(parseDisplayOrder("0")).toBe(0);
    expect(parseDisplayOrder(" 12 ")).toBe(12);
    expect(parseDisplayOrder("-1")).toBeNull();
    expect(parseDisplayOrder("1.5")).toBeNull();
    expect(parseDisplayOrder("2147483648")).toBeNull();
    expect(parseDisplayOrder("")).toBeNull();
  });
});

describe("banner validation and bodies", () => {
  const banner: GiaTriFormNoiDung = {
    ...FORM_TRONG,
    type: "banner",
    title: "Ngày hội chuyển đổi số",
    cover_image_file_id: "01JCOVER1",
  };

  it("a banner without a cover is held, with the sentence", () => {
    expect(validateTypeFields({ ...banner, cover_image_file_id: "" })).toBe(ERR_BANNER_COVER_REQUIRED);
    expect(validateTypeFields(banner)).toBeNull();
  });

  it("a LEGACY coverless banner can still be edited — the trigger lets it through too", () => {
    const before = { ...banner, cover_image_file_id: "" };
    expect(validateTypeFields(before, before)).toBeNull();
    // ...but turning a news item into a coverless banner is refused.
    expect(validateTypeFields(before, { ...FORM_TRONG, title: "x" })).toBe(ERR_BANNER_COVER_REQUIRED);
  });

  it("bad link and bad order are named", () => {
    expect(validateTypeFields({ ...banner, link_to: "//x" })).toBe(ERR_LINK_TO_INVALID);
    expect(validateTypeFields({ ...banner, display_order: "-3" })).toBe(ERR_DISPLAY_ORDER_INVALID);
  });

  it("an order already set cannot be emptied — a PATCH has no spelling for that", () => {
    const before = { ...banner, display_order: "2" };
    expect(validateTypeFields({ ...before, display_order: "" }, before)).toBe(
      ERR_DISPLAY_ORDER_CANNOT_CLEAR,
    );
    expect(validateTypeFields({ ...banner, display_order: "" }, banner)).toBeNull();
  });

  it("banner rules never block another type, whose banner boxes are hidden", () => {
    expect(validateTypeFields({ ...FORM_TRONG, title: "x", link_to: "//bad" })).toBeNull();
  });

  it("POST carries `link_to` / `display_order` for a banner only, and only with a value", () => {
    const full = thanThem({ ...banner, link_to: " /tin-tuc ", display_order: "3" });
    expect(full.link_to).toBe("/tin-tuc");
    expect(full.display_order).toBe(3);
    const bare = thanThem(banner);
    expect(bare).not.toHaveProperty("link_to");
    expect(bare).not.toHaveProperty("display_order");
    const news = thanThem({ ...banner, type: "tin-tuc", link_to: "/x", display_order: "1" });
    expect(news).not.toHaveProperty("link_to");
    expect(news).not.toHaveProperty("display_order");
  });

  it("PATCH: changed banner fields only; `link_to: \"\"` clears; a type change away sends neither", () => {
    const before = { ...banner, link_to: "/tin-tuc", display_order: "2" };
    expect(thanSua(before, { ...before, link_to: "" })).toEqual({ link_to: "" });
    expect(thanSua(before, { ...before, display_order: "5" })).toEqual({ display_order: 5 });
    expect(thanSua(before, before)).toEqual({});
    // Away from banner: the server clears both itself; sending them would be a 422.
    expect(thanSua(before, { ...before, type: "tin-tuc" })).toEqual({ type: "tin-tuc" });
  });

  it("`giaTriTuHang` reads the banner fields back", () => {
    const gt = giaTriTuHang(hang({ type: "banner", body: "", link_to: "/su-kien", display_order: 0 }));
    expect(gt.link_to).toBe("/su-kien");
    expect(gt.display_order).toBe("0");
    const none = giaTriTuHang(hang({ body: "", display_order: null }));
    expect(none.display_order).toBe("");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * `⊞ Danh mục tin` — edit (ADR 0067 §3)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("category edit", () => {
  const tree = [
    danhMuc({ id: "A", name: "A" }),
    danhMuc({ id: "B", name: "B", parent_id: "A" }),
    danhMuc({ id: "C", name: "C", parent_id: "B" }),
    danhMuc({ id: "D", name: "D" }),
  ];

  it("the parent select leaves out the category and all its descendants", () => {
    expect([...selfAndDescendants("A", tree)].sort()).toEqual(["A", "B", "C"]);
    expect(parentChoices("A", tree).map((m) => m.dm.id)).toEqual(["D"]);
    expect(parentChoices("C", tree).map((m) => m.dm.id)).toEqual(["A", "B", "D"]);
  });

  it("a cycle already in the data does not hang the walk", () => {
    const loop = [danhMuc({ id: "X", parent_id: "Y" }), danhMuc({ id: "Y", parent_id: "X" })];
    expect([...selfAndDescendants("X", loop)].sort()).toEqual(["X", "Y"]);
  });

  it("the PATCH body holds only what changed, never `slug` or `hidden`", () => {
    const b = tree[1]!;
    expect(categoryPatchBody(b, { name: "B", parentId: "A", order: "0" })).toEqual({});
    expect(categoryPatchBody(b, { name: "  Tên mới ", parentId: "A", order: "0" })).toEqual({
      name: "Tên mới",
    });
    // "" is the server's spelling of "move to the root".
    expect(categoryPatchBody(b, { name: "B", parentId: "", order: "4" })).toEqual({
      parent_id: "",
      order: 4,
    });
    const body = categoryPatchBody(b, { name: "Z", parentId: "D", order: "1" });
    expect(Object.keys(body).sort()).toEqual(["name", "order", "parent_id"]);
  });
});
