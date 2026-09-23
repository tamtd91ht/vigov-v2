import { describe, expect, it } from "vitest";

import type { comms_danhMucRa, comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  CHUA_XEP_DANH_MUC,
  coThayDoi,
  DAU_GACH,
  dungCayDanhMuc,
  FORM_TRONG,
  giaTriTuHang,
  LOAI_MAC_DINH,
  lopChipTrangThai,
  MOI_LOAI,
  nhanLoai,
  nhanLuotXem,
  nhanMoc,
  nhanMucDanhMuc,
  nhanNgayDang,
  nhanNguon,
  nhanTepDinhKem,
  nhanTrangThai,
  PHAN_CHUA_DUNG,
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

  it("`Lượt xem` luôn có ký hiệu mắt, kể cả khi bằng 0", () => {
    expect(nhanLuotXem(0)).toBe("👁 0");
    expect(nhanLuotXem(1234)).toBe("👁 1234");
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
});

describe("thân POST", () => {
  it("cắt hai đầu tiêu đề, tóm tắt và liên kết ảnh — KHÔNG cắt thân bài", () => {
    const than = thanThem({
      type: "su-kien",
      category_id: "01JDM1",
      title: "  Hội nghị  ",
      summary: "  tóm tắt  ",
      body: "  <p>x</p>  ",
      image_url: "  https://a.vn/b.jpg  ",
      publish: true,
    });

    expect(than.title).toBe("Hội nghị");
    expect(than.summary).toBe("tóm tắt");
    expect(than.image_url).toBe("https://a.vn/b.jpg");
    // Máy chủ tự cắt hai đầu thân bài khi lưu. Cắt thêm ở đây chỉ tạo một chỗ nữa để hai bên lệch.
    expect(than.body).toBe("  <p>x</p>  ");
  });

  it("gửi đủ bảy trường, không thừa trường nào", () => {
    const than = thanThem(FORM_TRONG);
    expect(Object.keys(than).sort()).toEqual([
      "body",
      "category_id",
      "image_url",
      "publish",
      "summary",
      "title",
      "type",
    ]);
  });
});

describe("thân PATCH — CHỈ những ô thật sự đổi", () => {
  const dau: GiaTriFormNoiDung = {
    type: "tin-tuc",
    category_id: "01JDM1",
    title: "Tiêu đề cũ",
    summary: "Tóm tắt cũ",
    body: "<p>thân cũ</p>",
    image_url: "https://a.vn/b.jpg",
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
      type: "video",
      category_id: "",
      title: "T",
      summary: "S",
      body: "B",
      image_url: "",
      publish: false,
    });
    expect(Object.keys(than).sort()).toEqual([
      "body",
      "category_id",
      "image_url",
      "publish",
      "summary",
      "title",
      "type",
    ]);
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

describe("phần chưa dựng được", () => {
  it("có danh sách, và mục đầu tiên nói về HTML không được làm sạch", () => {
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThanOrEqual(10);
    // Thứ tự có ý nghĩa: đây là rủi ro lớn nhất của màn, nên nó đứng đầu khối `<details>`.
    expect(PHAN_CHUA_DUNG[0]?.ten).toContain("rich text");
    expect(PHAN_CHUA_DUNG[0]?.viSao).toContain("content.update");
  });

  it("mỗi mục đều có lý do, không mục nào là một dòng tên suông", () => {
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten.length).toBeGreaterThan(10);
      expect(p.viSao.length).toBeGreaterThan(80);
    }
  });
});
