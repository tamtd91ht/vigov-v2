import { describe, expect, it } from "vitest";

import type { comms_danhMucRa, comms_noiDungRa } from "@/lib/api/schema.gen";
import { QUYEN_CONG_KHAI_DANH_BA } from "@/lib/quyen";
import { TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";

import {
  attachmentState,
  canEditContent,
  CANH_BAO_HTML_THO,
  categoryCellLabel,
  categoryPath,
  CREATED_DRAFT_TOAST,
  CREATED_PUBLISHED_TOAST,
  dialogTypeLabel,
  PAGE_SIZE,
  pageRange,
  publishToggleBody,
  SAVED_TOAST,
  saveToast,
  SO_RONG,
  CONTENT_UPDATE_PERMISSION,
  MO_TA_THE_DANH_BA,
  PUBLISHED_STAFF_UNREAD,
  publishedStaffText,
  coThayDoi,
  DAU_GACH,
  DELETE_REASON_MAX_CHARS,
  deleteReasonCounter,
  deleteReasonLength,
  deleteReasonReady,
  pageAfterDelete,
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
  groupCategoriesByParent,
  instantToLocalInput,
  isValidLinkTo,
  localInputToInstant,
  parseDisplayOrder,
  validateTypeFields,
  viewCountText,
  LOAI_MAC_DINH,
  lopChipTrangThai,
  MOI_LOAI,
  nhanLoai,
  nhanMoc,
  nhanMucDanhMuc,
  nhanNgayDang,
  nhanNguon,
  nhanTrangThai,
  PHAN_CHUA_DUNG,
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
  it("`Tệp đính kèm` per type — the prototype's cell (banner: image, broadcast/video: file, rest: cover)", () => {
    expect(attachmentState(hang({ type: "banner", has_image: true }))).toEqual({ label: "Đã có ảnh", tone: "ok" });
    expect(attachmentState(hang({ type: "banner", has_image: false }))).toEqual({ label: "Thiếu ảnh", tone: "missing" });
    expect(attachmentState(hang({ type: "truyen-thanh", audio_file_id: "01JAUD" }))).toEqual({ label: "Đã có tệp", tone: "ok" });
    expect(attachmentState(hang({ type: "truyen-thanh" }))).toEqual({ label: "Thiếu tệp", tone: "missing" });
    // A broadcast's COVER is not its file: an image alone still says the file is missing.
    expect(attachmentState(hang({ type: "truyen-thanh", has_image: true })).tone).toBe("missing");
    expect(attachmentState(hang({ type: "video", video_url: "https://youtu.be/x" }))).toEqual({ label: "Đã có tệp", tone: "ok" });
    expect(attachmentState(hang({ type: "video", video_url: "" }))).toEqual({ label: "Thiếu tệp", tone: "missing" });
    for (const type of ["tin-tuc", "su-kien", "thong-bao"]) {
      expect(attachmentState(hang({ type, has_image: true }))).toEqual({ label: "Có ảnh", tone: "muted" });
      expect(attachmentState(hang({ type, has_image: false }))).toEqual({ label: DAU_GACH, tone: "none" });
    }
  });

  it("the empty table's sentence is the prototype's, verbatim", () => {
    expect(SO_RONG).toBe("Chưa có nội dung nào ở mục này.");
  });

  it("tóm tắt gộp về một dòng, and is NOT cut — the one-line cut is CSS's (`.summary-one-line`)", () => {
    expect(trichTomTat("  hai   dòng\nnối lại  ")).toBe("hai dòng nối lại");
    expect(trichTomTat("")).toBe("");
    const long = "ạ".repeat(500);
    expect(trichTomTat(long)).toBe(long);
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

describe("tên danh mục của một bài — `Cha › Con`", () => {
  const ds = [
    danhMuc({ id: "P1", name: "Tin hoạt động" }),
    danhMuc({ id: "P2", name: "Chính quyền" }),
    danhMuc({ id: "C1", name: "Chuyển đổi số", parent_id: "P1" }),
    danhMuc({ id: "C2", name: "Chuyển đổi số", parent_id: "P2" }),
    danhMuc({ id: "T", name: "Tin tức" }),
    danhMuc({ id: "T1", name: "Tin tức", parent_id: "T" }),
  ];

  it("id rỗng là dấu gạch của bản mẫu — bài chưa xếp danh mục", () => {
    expect(categoryCellLabel("", ds)).toBe(DAU_GACH);
  });

  it("two children with the same name are told apart by their parent", () => {
    expect(categoryCellLabel("C1", ds)).toBe("Tin hoạt động › Chuyển đổi số");
    expect(categoryCellLabel("C2", ds)).toBe("Chính quyền › Chuyển đổi số");
    expect(categoryCellLabel("P1", ds)).toBe("Tin hoạt động");
  });

  it("a parent of the same name is not repeated; a parent not in the list is left out", () => {
    expect(categoryPath(ds[5]!, ds)).toBe("Tin tức");
    expect(categoryPath(danhMuc({ id: "X", name: "Lẻ", parent_id: "KHONG-CO" }), ds)).toBe("Lẻ");
  });

  it("id KHÔNG tra được hiện chính id, không hiện dấu gạch", () => {
    // Dấu gạch nói "chưa xếp danh mục"; sự thật là cây danh mục chưa nạp xong. Hai câu khác nhau.
    expect(categoryCellLabel("01JLA", ds)).toBe("01JLA");
  });
});

describe("Đăng / Gỡ — the PATCH body is `{publish}` ONLY (owner D1, 09/10/2026)", () => {
  it("`Gỡ` on a showing row, `Đăng` on a hidden or a pending one — and nothing else in the body", () => {
    expect(publishToggleBody("dang-hien")).toEqual({ publish: false });
    expect(publishToggleBody("an")).toEqual({ publish: true });
    expect(publishToggleBody("cho-duyet")).toEqual({ publish: true });
    expect(Object.keys(publishToggleBody("an"))).toEqual(["publish"]);
  });
});

describe("the editor dialog's words", () => {
  it("save toasts — the prototype's three, by edit / new + published / new draft", () => {
    expect(saveToast(true, true)).toBe(SAVED_TOAST);
    expect(saveToast(true, false)).toBe("Đã lưu thay đổi.");
    expect(saveToast(false, true)).toBe(CREATED_PUBLISHED_TOAST);
    expect(CREATED_PUBLISHED_TOAST).toBe("Đã đăng lên Mini App.");
    expect(saveToast(false, false)).toBe(CREATED_DRAFT_TOAST);
    expect(CREATED_DRAFT_TOAST).toBe("Đã lưu bản nháp.");
  });

  it("the type select's labels are the prototype's longer ones; the tabs keep the short ones", () => {
    expect(MOI_LOAI.map(dialogTypeLabel)).toEqual([
      "Tin tức",
      "Sự kiện",
      "Thông báo",
      "Bản tin truyền thanh",
      "Video",
      "Banner trang chủ",
    ]);
    expect(nhanLoai("truyen-thanh")).toBe("Truyền thanh");
    expect(nhanLoai("banner")).toBe("Banner");
  });
});

describe("pager — 25 rows, `a–b` known, the total not", () => {
  it("page size is the prototype's 25", () => {
    expect(PAGE_SIZE).toBe(25);
  });

  it("`a–b` from the page index and this page's rows", () => {
    expect(pageRange(0, 25)).toBe("1–25");
    expect(pageRange(1, 25)).toBe("26–50");
    expect(pageRange(2, 7)).toBe("51–57");
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

describe("`formatVietnamDateTime` — dd/MM/yyyy HH:mm, Vietnam time", () => {
  it("formats an instant", () => {
    expect(formatVietnamDateTime("2026-10-01T02:05:00Z")).toBe("01/10/2026 09:05");
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

describe("viewCountText", () => {
  it("groups thousands the vi-VN way and prints 0 as 0", () => {
    expect(viewCountText(1234)).toBe("1.234");
    expect(viewCountText(1234567)).toBe("1.234.567");
    expect(viewCountText(0)).toBe("0");
    expect(viewCountText(42)).toBe("42");
  });
});

describe("phần chưa dựng được", () => {
  it("the list is PINNED: rich text, category edit/delete, banner, broadcast audio and the portal sync card left it when built", () => {
    // EXACT, not a floor: an item silently dropped and an item silently kept are both a block that
    // lies to the commune about what the screen does. ADR 0067 built five of them (§1, §2, §3, §4, §5);
    // the portal sync card brought two narrower gaps of its own; the status filter left on 02/10/2026
    // (`status` on GET content-items, C1), the meta count stays. Also 02/10/2026, following the
    // prototype: the §4 count, the §2 layout and the `content.update` gate left it — and then the
    // per-item delete (soft, with a reason, DELETE /api/v1/content-items/{id}). The `Lượt xem` column
    // left it the same day: the owner decided to show the server's count (ADR 0047, row 02/10/2026).
    // 06/10/2026: the prototype's thumbnail column joined it — the list route serves no image link.
    // 09/10/2026: the prototype's pager — its total and its page count; the list route never counts.
    // 09/10/2026 (owner D4): the config dialog's image download box, and the `Danh mục tin` dialog's
    // portal import, item count and `Từ Cổng` mark — no route behind any of the four.
    // 09/10/2026 (prototype parity): the config dialog's `Mỗi 15 phút` interval and the `Đang dùng {hint}`
    // of the stored key — the server counts whole hours and returns nothing derived from the key.
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual([
      "Số chuyên mục đang đồng bộ",
      "Tải ảnh về kho của xã",
      "Lấy danh mục từ Cổng",
      "Số bài của mỗi danh mục",
      "Dấu danh mục lấy từ Cổng",
      "Ảnh thu nhỏ trong bảng",
      "Tổng số mục của danh sách",
      "Tổng số trang của danh sách",
      "Đồng bộ mỗi 15 phút",
      "Gợi ý mã bảo mật đang dùng",
    ]);
  });

  it("the thumbnail's reason no longer points to a cover preview in the edit form (there is none)", () => {
    const thumb = PHAN_CHUA_DUNG.find((p) => p.ten === "Ảnh thu nhỏ trong bảng")!;
    expect(thumb.viSao).not.toContain("biểu mẫu sửa");
    expect(thumb.viSao).not.toContain("xem ở");
  });

  it("no item still claims the view count is not shown", () => {
    const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");
    expect(all).not.toContain("Lượt xem");
    expect(all).not.toContain("lượt xem");
  });

  it("no item still claims an item cannot be deleted", () => {
    const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");
    expect(all).not.toContain("Nút xoá");
    expect(all).not.toContain("DELETE");
  });

  it("no item still claims the §4 count, the §2 layout or the permission gate are missing", () => {
    const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");
    expect(all).not.toContain("Đang hiện 26 cán bộ");
    expect(all).not.toContain("aria-pressed");
    expect(all).not.toContain("không được thêm CSS");
    expect(all).not.toContain("Cổng quyền");
  });
});

describe("`canEditContent` — who sees the write controls (fail closed, no flash)", () => {
  const session = (permissions: string[]) =>
    ({ ok: true, duLieu: { permissions } }) as unknown as Parameters<typeof canEditContent>[0];

  it("the key is `content.update`, the SAME literal as `lib/quyen.ts` holds — not a second copy", () => {
    expect(CONTENT_UPDATE_PERMISSION).toBe("content.update");
    expect(CONTENT_UPDATE_PERMISSION).toBe(QUYEN_CONG_KHAI_DANH_BA);
  });

  it("session not read yet → no (the buttons appear later, never vanish)", () => {
    expect(canEditContent(null)).toBe(false);
  });

  it("session read with an error → no", () => {
    expect(canEditContent({ ok: false, thongBao: "Phiên đã hết hạn." })).toBe(false);
  });

  it("`content.read` alone → no; neighbouring keys do not open it", () => {
    expect(canEditContent(session(["content.read"]))).toBe(false);
    expect(canEditContent(session(["admin.user", "announcement.create", "content.updat"]))).toBe(false);
  });

  it("`content.update` held → yes", () => {
    expect(canEditContent(session(["content.read", "content.update"]))).toBe(true);
  });
});

describe("delete reason — counted as the server counts it (trimmed, runes, ≤ 500)", () => {
  it("blank and whitespace-only are not ready", () => {
    expect(deleteReasonReady("")).toBe(false);
    expect(deleteReasonReady("   \n\t ")).toBe(false);
  });

  it("500 characters are ready, 501 are not; surrounding spaces do not count", () => {
    expect(DELETE_REASON_MAX_CHARS).toBe(500);
    expect(deleteReasonReady("ạ".repeat(500))).toBe(true);
    expect(deleteReasonReady(`  ${"ạ".repeat(500)}  `)).toBe(true);
    expect(deleteReasonReady("a".repeat(501))).toBe(false);
  });

  it("a character outside the BMP is ONE character, as a Go rune is", () => {
    expect(deleteReasonLength("𠀀")).toBe(1);
    expect(deleteReasonReady("𠀀".repeat(500))).toBe(true);
    expect(deleteReasonCounter(" Trùng bài ")).toBe("9/500");
  });
});

describe("`pageAfterDelete` — stay on the page, one back when it empties", () => {
  const page3 = { daQua: [null, "c2"], hienTai: "c3" } as const;

  it("rows left on the page → the same page is read again", () => {
    expect(pageAfterDelete(page3, 5)).toBe(page3);
  });

  it("the last row of a later page → one page back", () => {
    expect(pageAfterDelete(page3, 1)).toEqual({ daQua: [null], hienTai: "c2" });
  });

  it("the last row of the first page → still the first page", () => {
    expect(pageAfterDelete(TRANG_DAU, 1)).toBe(TRANG_DAU);
  });
});

describe("`publishedStaffText` — §4's line in three states", () => {
  it("a number only after a successful read", () => {
    expect(publishedStaffText({ ok: true, duLieu: 26 })).toEqual({
      line: "Đang hiện 26 cán bộ cho bà con. Chọn thêm hoặc bớt ở ngăn Danh bạ cán bộ.",
      note: null,
    });
    expect(publishedStaffText(null)).toEqual({ line: MO_TA_THE_DANH_BA, note: null });
    expect(publishedStaffText({ ok: false, thongBao: "x" })).toEqual({
      line: MO_TA_THE_DANH_BA,
      note: PUBLISHED_STAFF_UNREAD,
    });
    expect(MO_TA_THE_DANH_BA).not.toMatch(/\d/);
  });
});

describe("phần chưa dựng được — the remaining items", () => {

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
    // Body images are kept since ADR 0067 §Sửa đổi 03/10/2026: the note must not say images are dropped.
    expect(CANH_BAO_HTML_THO).not.toContain("— ảnh, bảng");
    expect(CANH_BAO_HTML_THO).toContain("ảnh chèn bằng nút “Chèn ảnh”");
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
 * `⊞ Danh mục tin` — the list grouped by parent (prototype `CategoryManagerDialog.tsx:319-338`)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("category groups", () => {
  it("no-parent categories FIRST under the prototype's heading, then one group per parent, named by it", () => {
    const groups = groupCategoriesByParent([
      danhMuc({ id: "C", name: "Tổ 1", parent_id: "B" }),
      danhMuc({ id: "B", name: "Thôn Một", parent_id: "A" }),
      danhMuc({ id: "A", name: "Tin xã" }),
      danhMuc({ id: "D", name: "An ninh", order: 2 }),
    ]);
    expect(groups.map((g) => [g.name, g.items.map((d) => d.id)])).toEqual([
      ["Đứng riêng, không thuộc mục cha nào", ["A", "D"]],
      ["Tin xã", ["B"]],
      ["Thôn Một", ["C"]],
    ]);
  });

  it("a parent the list does not hold makes its child stand alone — never dropped", () => {
    const groups = groupCategoriesByParent([danhMuc({ id: "X", name: "Mồ côi", parent_id: "GONE" })]);
    expect(groups).toEqual([{ parentId: "", name: "Đứng riêng, không thuộc mục cha nào", items: [expect.objectContaining({ id: "X" })] }]);
  });

  it("an empty list draws no group, not an empty heading", () => {
    expect(groupCategoriesByParent([])).toEqual([]);
  });
});
