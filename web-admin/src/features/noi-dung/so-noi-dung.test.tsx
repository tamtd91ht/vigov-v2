import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { comms_danhMucRa, comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  CANH_BAO_HTML_THO,
  CANH_BAO_LIEN_KET_ANH,
  CANH_BAO_XEM_MA_NGUON,
  CHUA_XEP_DANH_MUC,
  FORM_TRONG,
  GHI_CHU_KHONG_CO_XOA,
  GHI_CHU_LUOT_XEM,
  giaTriTuHang,
  MOI_DANH_MUC,
  MOI_LOAI_NHAN,
  NHAN_DA_SUA_TAY,
  NHAN_NUT_SUA,
  NHAN_NUT_THEM,
  NHAN_O_DANG,
  PHAN_CHUA_DUNG,
  SO_RONG,
} from "./nhan-noi-dung";
import {
  BangNoiDung,
  FormDanhMuc,
  FormNoiDung,
  HangLocNoiDung,
  KhoiChuaDung,
  ThanhTabLoai,
  TheDanhBaChinhQuyen,
  ThongTinChiDoc,
} from "./so-noi-dung";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **HTML KHÔNG ĐƯỢC DỰNG**. Máy chủ lưu `noi_dung` nguyên
 * văn và không có bộ làm sạch nào phía sau, nên một ô xem trước dựng chuỗi ấy là chạy mã của người
 * vừa gõ, trên màn hình quản trị của xã. Bài kiểm dưới đòi thân bài ra tới trang ở dạng ĐÃ THOÁT —
 * tức là dữ liệu, không phải mã.
 *
 * Nhóm thứ hai canh một sự VẮNG MẶT, thứ khó canh nhất vì không có gì để tìm trên trang: KHÔNG có
 * nút xoá ở cột hành động, và KHÔNG có ô chọn trạng thái trong biểu mẫu.
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ `renderToStaticMarkup` thoát `&`, `<`, `>`, `"` và `'`, nên một phép `not.toContain` với chuỗi
 * thô sẽ XANH kể cả khi chữ ấy đang nằm chình ình trên trang — tức là canh đúng con số không. Đã
 * đo ở `features/thu-chi/bang-thu-chi.test.tsx`.
 *
 * NĂM KÝ TỰ, KHÔNG PHẢI HAI như bản ở màn Thông báo — và sự khác ấy do chính màn này gây ra: chữ
 * trên màn ở đây NHẮC TỚI HTML (`` `<script>` ``, `<p>`), nên `<` và `>` xuất hiện thật trong văn
 * bản chứ không chỉ trong dữ liệu thử. Đo 24/09/2026: thiếu hai ký tự ấy thì ca "TỪNG mục ra tới
 * trang" đỏ ngay ở mục đầu tiên — mục quan trọng nhất của khối.
 *
 * `&` PHẢI THAY TRƯỚC, nếu không `&lt;` vừa sinh ra sẽ bị thay tiếp thành `&amp;lt;`.
 */
function nhuTrongHTML(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;");
}

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
    updated_at: "2026-09-14T09:35:00Z",
    ...sua,
  };
}

function veBang(ds: readonly comms_noiDungRa[], dm: readonly comms_danhMucRa[] = [danhMuc()]) {
  return renderToStaticMarkup(<BangNoiDung ds={ds} danhMuc={dm} sua={() => {}} />);
}

function veForm(gt = FORM_TRONG, h?: comms_noiDungRa) {
  return renderToStaticMarkup(
    <FormNoiDung
      tieuDeForm="Thêm nội dung cho Mini App"
      moTa="mô tả"
      giaTriDau={gt}
      hang={h}
      danhMuc={[danhMuc()]}
      dangGui={false}
      loi={null}
      huy={() => {}}
      luu={() => {}}
    />,
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HTML KHÔNG ĐƯỢC DỰNG — nhóm quan trọng nhất của màn
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("thân bài ra tới trang dưới dạng DỮ LIỆU, không phải mã", () => {
  const DOC = '<script>alert("xin chao")</script><img src=x onerror=alert(1)>';

  it("mã nguồn HTML của bài hiện ĐÃ THOÁT trong ô nhập", () => {
    const html = veForm({ ...FORM_TRONG, body: DOC });

    // Thẻ `<script>` phải nằm trên trang ở dạng ĐÃ THOÁT — cán bộ vẫn đọc và sửa được nó.
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");

    // Và KHÔNG được MỞ một thẻ nào từ chữ ấy. Đây là phép so canh đúng điều màn này sinh ra để
    // tránh: một cán bộ có `content.update` chạy mã trên màn hình quản trị của xã.
    //
    // SO THEO DẤU MỞ THẺ (`<script`, `<img`), KHÔNG so theo chuỗi `onerror=alert`: chuỗi ấy VẪN
    // nằm trên trang, đúng và an toàn, ở giữa `&lt;` và `&gt;`. Một phép so bắt nó sẽ đỏ oan —
    // và một test đỏ oan là một test có người tắt đi.
    expect(html).not.toContain("<script");
    expect(html).not.toContain("<img");
  });

  it("hai câu cảnh báo đứng NGAY CẠNH ô nội dung, không nằm trong chú thích mã", () => {
    const html = veForm();
    expect(html).toContain(nhuTrongHTML(CANH_BAO_HTML_THO));
    expect(html).toContain(nhuTrongHTML(CANH_BAO_XEM_MA_NGUON));
  });

  it("KHÔNG có ô xem trước nào — không `dangerouslySetInnerHTML`, không khối `preview`", () => {
    // Phép quét mã nguồn nằm ở `ranh-gioi-html.test.ts`; ca này canh cùng điều ấy ở đầu ra.
    const html = veForm({ ...FORM_TRONG, body: "<b>đậm</b>" });
    expect(html).not.toContain("<b>đậm</b>");
    expect(html).toContain("&lt;b&gt;");
  });

  it("tiêu đề mang dấu ngoặc nhọn cũng ra dạng đã thoát ở BẢNG", () => {
    // Bảng §6 không hiện thân bài, nhưng tiêu đề và tóm tắt cũng là chữ cán bộ gõ.
    const html = veBang([hang({ title: "<b>Tin nóng</b>", summary: "<i>ngay</i>" })]);
    expect(html).toContain("&lt;b&gt;Tin nóng&lt;/b&gt;");
    expect(html).not.toContain("<b>Tin nóng</b>");
  });

  it("liên kết bài gốc hiện dưới dạng CHỮ, không thành một `href` bấm được", () => {
    // Danh sách trắng lược đồ ở máy chủ chỉ chặn lúc GHI. Một hàng cũ trong CSDL không có gì bảo
    // đảm điều đó, và `javascript:…` trong một `href` là thực thi mã.
    const html = renderToStaticMarkup(
      <ThongTinChiDoc hang={hang({ source_url: "javascript:alert(1)" })} />,
    );
    expect(html).toContain("javascript:alert(1)");
    expect(html).not.toContain('href="javascript:alert(1)"');
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢNG §6
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("bảng nội dung §6", () => {
  it("bảy cột dữ liệu của đặc tả cùng ra một hàng", () => {
    const html = veBang([hang()]);

    expect(html).toContain("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số");
    expect(html).toContain("Hội nghị diễn ra sáng 14/9");
    expect(html).toContain("Tin tức");
    expect(html).toContain("Chuyển đổi số");
    expect(html).toContain("🔗 Có ảnh");
    expect(html).toContain("14/9/2026");
    expect(html).toContain("👁 0");
    expect(html).toContain("Đang hiện");
  });

  it("cột hành động CHỈ có `✎` — không nút xoá, không thùng rác", () => {
    // Hợp đồng không có tuyến `DELETE` nào. Một nút xoá vẽ ra ở đây là một nút không có đường nào
    // phía sau, và luật 7 biến việc xoá thành xoá MỀM bắt buộc có lý do — thứ màn này không thu.
    const html = veBang([hang()]);

    expect(html).toContain(NHAN_NUT_SUA);
    expect(html).not.toContain("🗑");
    expect(html).not.toContain("Xoá");
    expect(html).not.toContain("Xóa");
  });

  it("nút sửa mang nhãn nói rõ đang sửa bài nào — sáu hàng cùng một ký hiệu thì đọc không ra", () => {
    const html = veBang([hang()]);
    expect(html).toContain(nhuTrongHTML("Sửa: Xã tổ chức hội nghị tổng kết công tác chuyển đổi số"));
  });

  it("hai dòng chữ dưới bảng nói vì sao lượt xem là 0 và vì sao không có nút xoá", () => {
    const html = veBang([hang()]);
    expect(html).toContain(nhuTrongHTML(GHI_CHU_LUOT_XEM));
    expect(html).toContain(nhuTrongHTML(GHI_CHU_KHONG_CO_XOA));
  });

  it("bài chưa xếp danh mục hiện đúng chữ đặc tả, không hiện ô trống", () => {
    expect(veBang([hang({ category_id: "" })])).toContain(CHUA_XEP_DANH_MUC);
  });

  it("cờ §10.4 ra tới hàng — cán bộ phải thấy bài đã thoát khỏi lượt đồng bộ", () => {
    expect(veBang([hang({ hand_edited: true })])).toContain(NHAN_DA_SUA_TAY);
    expect(veBang([hang({ hand_edited: false })])).not.toContain(NHAN_DA_SUA_TAY);
  });

  it("chip `Chờ duyệt` và `Ẩn` hiện đúng chữ, không lẫn với `Đang hiện`", () => {
    expect(veBang([hang({ status: "cho-duyet" })])).toContain("Chờ duyệt");
    expect(veBang([hang({ status: "an" })])).toContain("Ẩn");
    expect(veBang([hang({ status: "an" })])).not.toContain("Đang hiện");
  });

  it("sổ rỗng thì nói ra, không để trang trắng", () => {
    expect(veBang([])).toContain(SO_RONG);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BIỂU MẪU §7
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("biểu mẫu nội dung §7", () => {
  it("bảy ô của đặc tả có mặt, và ô tích mang đúng chữ đặc tả", () => {
    const html = veForm();

    expect(html).toContain("Loại nội dung");
    expect(html).toContain("Danh mục");
    expect(html).toContain("Tiêu đề");
    expect(html).toContain("Tóm tắt");
    expect(html).toContain("Nội dung");
    expect(html).toContain("Ảnh đại diện");
    expect(html).toContain(NHAN_O_DANG);
  });

  it("KHÔNG có ô chọn trạng thái — máy chủ suy trạng thái từ ô tích", () => {
    // Một thân tự khai trạng thái là một bài đăng vượt qua bước duyệt mà §10.2 dành cho lượt đồng
    // bộ, hoặc một bài tự nhận `cho-duyet`.
    const html = veForm();
    expect(html).not.toContain('id="trang-thai-noi-dung"');
    expect(html).not.toContain("Chờ duyệt");
  });

  it("KHÔNG có nút chọn tệp — `core/storage` chưa có, chỉ nhập được LIÊN KẾT", () => {
    const html = veForm();
    expect(html).not.toContain('type="file"');
    expect(html).not.toContain("Chọn tệp từ máy");
    expect(html).toContain(nhuTrongHTML(CANH_BAO_LIEN_KET_ANH));
  });

  it("sáu loại của §5 đều có trong ô chọn", () => {
    const html = veForm();
    for (const nhan of ["Tin tức", "Sự kiện", "Thông báo", "Truyền thanh", "Video", "Banner"]) {
      expect(html).toContain(`>${nhan}</option>`);
    }
  });

  it("ô tích BẬT khi mở một bài đang hiện, TẮT khi mở một bài chờ duyệt", () => {
    expect(veForm(giaTriTuHang(hang({ status: "dang-hien", body: "" })))).toContain("checked");
    expect(veForm(giaTriTuHang(hang({ status: "cho-duyet", body: "" })))).not.toContain("checked");
  });

  it("nút Lưu TẮT khi tiêu đề rỗng — máy chủ vẫn là nơi từ chối thật", () => {
    expect(veForm()).toContain("disabled");
    expect(veForm({ ...FORM_TRONG, title: "Có tiêu đề" })).toContain(">Lưu</button>");
  });

  it("khối chỉ đọc chỉ xuất hiện khi đang SỬA", () => {
    expect(veForm()).not.toContain("Cập nhật lúc");
    expect(veForm(FORM_TRONG, hang())).toContain("Cập nhật lúc");
  });
});

describe("khối thông tin chỉ đọc", () => {
  it("mốc cập nhật in giờ Việt Nam kể cả khi máy chạy ở UTC", () => {
    const html = renderToStaticMarkup(<ThongTinChiDoc hang={hang()} />);
    expect(html).toContain("16:35 14/09/2026");
  });

  it("mã nghiệp vụ cán bộ, không phải id nội bộ", () => {
    expect(renderToStaticMarkup(<ThongTinChiDoc hang={hang()} />)).toContain("CB-2026-7K3M9Q");
  });

  it("lượt xem chỉ HIỆN — không có ô nhập nào cho nó", () => {
    const html = renderToStaticMarkup(<ThongTinChiDoc hang={hang({ view_count: 42 })} />);
    expect(html).toContain("👁 42");
    expect(html).not.toContain("<input");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TAB, BỘ LỌC, HAI THẺ ĐẦU MÀN, DANH MỤC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("sáu tab loại §5", () => {
  it("bảy nút: `Tất cả` cộng sáu loại", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="" datLoai={() => {}} />);
    expect(html).toContain(MOI_LOAI_NHAN);
    for (const nhan of ["Tin tức", "Sự kiện", "Thông báo", "Truyền thanh", "Video", "Banner"]) {
      expect(html).toContain(nhan);
    }
  });

  it("tab đang chọn đánh dấu bằng `aria-pressed`, không mượn lớp CSS của thứ khác", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="video" datLoai={() => {}} />);
    // Đúng một nút đang bật.
    expect(html.match(/aria-pressed="true"/g)?.length).toBe(1);
  });
});

describe("hàng lọc §6", () => {
  it("ô tìm và ô chọn danh mục cùng ra một hàng", () => {
    const html = renderToStaticMarkup(
      <HangLocNoiDung
        danhMucID=""
        datDanhMucID={() => {}}
        tim=""
        datTim={() => {}}
        timNgay={() => {}}
        danhMuc={[danhMuc()]}
      />,
    );

    expect(html).toContain("Tìm theo tiêu đề");
    expect(html).toContain(MOI_DANH_MUC);
    expect(html).toContain("Chuyển đổi số");
  });

  it("ô tìm nằm trong một `form` — gửi bằng submit, không gửi theo từng phím", () => {
    // Mỗi phím là một lời gọi mang chữ cán bộ đang gõ vào một URL, và một URL đi vào mọi log
    // truy cập (luật 3, cấm #4).
    const html = renderToStaticMarkup(
      <HangLocNoiDung
        danhMucID=""
        datDanhMucID={() => {}}
        tim=""
        datTim={() => {}}
        timNgay={() => {}}
        danhMuc={[]}
      />,
    );
    expect(html).toContain('role="search"');
    expect(html).toContain('type="submit"');
  });
});

describe("thẻ Danh bạ chính quyền §4", () => {
  it("liên kết sang `/danh-ba` có, con số cán bộ thì KHÔNG", () => {
    // Con số đếm cờ `hien_tren_mini_app` ở `service-identity`, không tuyến nào trả về. Hiện một
    // số 0 ở đó là nói với xã rằng bà con không thấy cán bộ nào.
    const html = renderToStaticMarkup(<TheDanhBaChinhQuyen />);
    expect(html).toContain('href="/danh-ba"');
    expect(html).not.toMatch(/\d+ cán bộ/);
  });
});

describe("biểu mẫu danh mục tin", () => {
  it("chỉ THÊM — không nút sửa, không nút xoá", () => {
    // Hợp đồng chỉ có `GET` và `POST` trên `/api/v1/content-categories`.
    const html = renderToStaticMarkup(
      <FormDanhMuc
        danhMuc={[danhMuc()]}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain("Thêm danh mục tin");
    expect(html).not.toContain("🗑");
    expect(html).not.toContain(">Xoá<");
    expect(html).not.toContain(">Sửa<");
  });

  it("nói trước hai điều máy chủ sẽ từ chối: khuôn slug, và slug đã cấp không cấp lại", () => {
    const html = renderToStaticMarkup(
      <FormDanhMuc danhMuc={[]} dangGui={false} loi={null} huy={() => {}} luu={() => {}} />,
    );
    expect(html).toContain("chữ thường a-z");
    expect(html).toContain("KHÔNG cấp lại");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PHẦN CHƯA DỰNG ĐƯỢC — mỗi mục PHẢI ra HTML
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("khối `phần chưa dựng được`", () => {
  const html = renderToStaticMarkup(<KhoiChuaDung />);

  it("số mục trên nhãn khớp danh sách thật", () => {
    expect(html).toContain(`${PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được`);
  });

  it("TỪNG mục ra tới trang, cả tên lẫn lý do", () => {
    // Ca này là lý do khối ấy tồn tại: một mục nằm trong mảng mà không ra HTML là một phần thiếu
    // mà không ai biết là thiếu — đúng loại lỗi màn hình này sinh ra để tránh.
    for (const p of PHAN_CHUA_DUNG) {
      expect(html, `thiếu tên: ${p.ten}`).toContain(nhuTrongHTML(p.ten));
      expect(html, `thiếu lý do của: ${p.ten}`).toContain(nhuTrongHTML(p.viSao));
    }
  });

  it("bốn thứ chặn lớn nhất đều được gọi tên", () => {
    expect(html).toContain("core/crypto");
    expect(html).toContain("core/storage");
    expect(html).toContain(nhuTrongHTML("KHÔNG CÓ TÊN MIỀN"));
    expect(html).toContain("content.update");
  });
});

describe("nhãn nút chính của màn", () => {
  it("đúng chữ đặc tả, kể cả dấu cộng", () => {
    expect(NHAN_NUT_THEM).toBe("+ Thêm nội dung");
  });
});
