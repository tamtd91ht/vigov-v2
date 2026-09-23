import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { comms_thongBaoRa } from "@/lib/api/schema.gen";

import {
  CANH_BAO_CHUA_GUI_THU,
  CHIP_BAT_BUOC_XAC_NHAN,
  CHO_DANH_SACH_NGUOI_NHAN,
  CHO_NUT_GO,
  CHUA_CHON_THONG_BAO,
  GHI_CHU_GHIM_TRONG_TRANG,
  NHAN_NUT_PHAT_HANH,
  PHAN_CHUA_DUNG,
  SO_RONG,
} from "./nhan-thong-bao";
import {
  ChiTietThongBao,
  DanhSachThongBao,
  FormSoanThongBao,
  KhoiChuaDung,
  TheThongBao,
} from "./so-thong-bao";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **BỘ PHẬN NHẬN**, và nó canh một sự VẮNG MẶT — thứ khó
 * canh nhất, vì không có gì để tìm trên trang. Máy chủ trả 501 cho mọi thân mang `org_unit_ids`,
 * nên một ô chọn bộ phận vẽ ra ở §5 là năm cái nút mà mọi lần bấm đều hỏng, sau khi cán bộ đã gõ
 * xong cả nội dung. Bài kiểm ở đây đòi hai điều cùng lúc: ô ấy KHÔNG có trên biểu mẫu, và lý do
 * thì CÓ, ở khối đầu màn.
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ `renderToStaticMarkup` thoát `"` thành `&quot;` và `&` thành `&amp;`, nên một phép
 * `not.toContain` với chuỗi thô sẽ XANH kể cả khi chữ ấy đang nằm chình ình trên trang — tức là
 * canh đúng con số không. Đã đo ở `features/thu-chi/bang-thu-chi.test.tsx`.
 */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

function thongBao(sua: Partial<comms_thongBaoRa> = {}): comms_thongBaoRa {
  return {
    id: "01JTB1",
    title: "Thông báo về việc triển khai hệ thống an ninh",
    body: "Đề nghị các bộ phận cử cán bộ dự buổi tập huấn lúc 8h00.",
    status: "da-phat-hanh",
    pinned: false,
    ack_required: true,
    email_requested: true,
    email_status: "chua-gui",
    author_code: "CB-2026-7K3M9Q",
    recipient_count: 12,
    ack_count: 2,
    issued_at: "2026-09-07T09:35:00Z",
    created_at: "2026-09-07T09:34:00Z",
    ...sua,
  };
}

function veThe(tb: comms_thongBaoRa, dangChon = false): string {
  return renderToStaticMarkup(
    <TheThongBao thongBao={tb} dangChon={dangChon} chon={() => {}} />,
  );
}

describe("thẻ thông báo §3", () => {
  it("tiêu đề, trích nội dung, mốc phát hành và bộ đếm cùng ra một thẻ", () => {
    const html = veThe(thongBao());

    expect(html).toContain("Thông báo về việc triển khai hệ thống an ninh");
    expect(html).toContain("Đề nghị các bộ phận cử cán bộ");
    expect(html).toContain("16:35 07/09/2026");
    expect(html).toContain("2/12 đã xác nhận");
  });

  it("chip `Bắt buộc xác nhận` chỉ hiện khi bật cờ, và bộ đếm biến mất cùng nó", () => {
    expect(veThe(thongBao())).toContain(CHIP_BAT_BUOC_XAC_NHAN);

    const tat = veThe(thongBao({ ack_required: false }));
    expect(tat).not.toContain(CHIP_BAT_BUOC_XAC_NHAN);
    // `0/12 đã xác nhận` trên một thông báo không đòi xác nhận là nói với cả xã rằng mười hai
    // người đang nợ một việc không ai giao.
    expect(tat).not.toContain("đã xác nhận");
  });

  it("KHÔNG BAO GIỜ có chip thư hôm nay — `chua-gui` không có nhãn nào ở §3", () => {
    const html = veThe(thongBao());

    expect(html).not.toContain("Đang gửi thư");
    expect(html).not.toContain("Đã gửi thư");
    expect(html).not.toContain("Gửi thư lỗi");
  });

  it("thông báo `da-phat-hanh` KHÔNG mang chip trạng thái, `da-go` thì có", () => {
    // Một chip "Đã phát hành" trên mọi thẻ là một chip không nói gì.
    expect(veThe(thongBao())).not.toContain("Đã phát hành");
    expect(veThe(thongBao({ status: "da-go" }))).toContain("Đã gỡ");
  });

  it("thẻ ghim mang dấu ghim, thẻ thường thì không", () => {
    expect(veThe(thongBao({ pinned: true }))).toContain("📌");
    expect(veThe(thongBao())).not.toContain("📌");
  });

  it("thẻ đang chọn đánh dấu bằng `aria-current` — không mượn một lớp CSS của thứ khác", () => {
    expect(veThe(thongBao(), true)).toContain('aria-current="true"');
    expect(veThe(thongBao(), false)).not.toContain("aria-current");
  });
});

describe("danh sách thẻ §2", () => {
  it("sổ rỗng thì nói ra, không để trang trắng", () => {
    const html = renderToStaticMarkup(
      <DanhSachThongBao thongBao={[]} dangChon={null} chon={() => {}} />,
    );

    expect(html).toContain(SO_RONG);
  });

  it("nói thẳng rằng ghim chỉ nâng TRONG TRANG, không phải thứ tự cả sổ", () => {
    // `core/page` mang đúng một cột sắp xếp; thứ tự toàn sổ không diễn đạt được. Một cán bộ tin
    // rằng thẻ ghim luôn ở đầu quyển sổ sẽ đi tìm một thông báo ở chỗ nó không nằm.
    const html = renderToStaticMarkup(
      <DanhSachThongBao thongBao={[thongBao()]} dangChon={null} chon={() => {}} />,
    );

    expect(html).toContain(GHI_CHU_GHIM_TRONG_TRANG);
  });

  it("vẽ đúng thứ tự được truyền vào, không tự sắp xếp lại lần nữa", () => {
    const html = renderToStaticMarkup(
      <DanhSachThongBao
        thongBao={[
          thongBao({ id: "a", title: "Thông báo A" }),
          thongBao({ id: "b", title: "Thông báo B" }),
        ]}
        dangChon="b"
        chon={() => {}}
      />,
    );

    expect(html.indexOf("Thông báo A")).toBeLessThan(html.indexOf("Thông báo B"));
    // Đúng MỘT thẻ mang dấu đang chọn.
    expect(html.split('aria-current="true"').length - 1).toBe(1);
  });
});

describe("panel chi tiết §4", () => {
  it("chưa chọn gì thì nguyên văn câu của đặc tả", () => {
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={null} />);
    expect(html).toContain(CHUA_CHON_THONG_BAO);
  });

  it("hiện TOÀN VĂN nội dung, không phải bản trích của thẻ", () => {
    const dai = "Kính gửi các bộ phận. ".repeat(20);
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao({ body: dai })} />);

    expect(html).toContain(dai.trim());
    // Không có dấu ba chấm cắt bớt ở cột phải: đó là chỗ duy nhất đọc lại được toàn văn.
    expect(html).not.toContain("…");
  });

  it("người soạn là MÃ NGHIỆP VỤ, không phải id nội bộ và không phải họ tên", () => {
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao()} />);
    expect(html).toContain("CB-2026-7K3M9Q");
  });

  it("hai sự thật về thư đứng RIÊNG: đã yêu cầu, và đã xảy ra", () => {
    // Gộp chúng lại sẽ làm "chưa gửi được" không phân biệt được với "không ai yêu cầu gửi thư".
    const co = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao()} />);
    expect(co).toContain("Có yêu cầu gửi");
    expect(co).toContain("Chưa gửi");

    const khong = renderToStaticMarkup(
      <ChiTietThongBao thongBao={thongBao({ email_requested: false })} />,
    );
    expect(khong).toContain("Không yêu cầu gửi");
  });

  it("chỗ của danh sách người nhận và nút Gỡ là DÒNG CHỮ, không phải danh sách rỗng hay nút mờ", () => {
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao()} />);

    expect(html).toContain(nhuTrongHTML(CHO_DANH_SACH_NGUOI_NHAN));
    expect(html).toContain(nhuTrongHTML(CHO_NUT_GO));
    // Không một `<button>` nào trong panel: một nút mờ nói "bạn không có quyền", và đó là một câu
    // khác hẳn với "màn hình chưa dựng".
    expect(html).not.toContain("<button");
  });
});

describe("biểu mẫu Soạn thông báo §5", () => {
  function veForm(loi: string | null = null): string {
    return renderToStaticMarkup(
      <FormSoanThongBao dangGui={false} loi={loi} huy={() => {}} phatHanh={() => {}} />,
    );
  }

  it("có đủ các ô dựng được", () => {
    const html = veForm();

    expect(html).toContain("Tiêu đề *");
    expect(html).toContain("Nội dung *");
    expect(html).toContain("Gửi thêm đích danh *");
    expect(html).toContain("Ghim lên đầu danh sách");
    expect(html).toContain("Bắt buộc xác nhận đã đọc");
    expect(html).toContain("Gửi thư điện tử cho người nhận");
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_PHAT_HANH));
  });

  it("KHÔNG CÓ Ô CHỌN BỘ PHẬN — máy chủ trả 501 cho mọi thân mang `org_unit_ids`", () => {
    const html = veForm();

    // Năm bộ phận §5 liệt kê. Một trong năm xuất hiện trên biểu mẫu là dấu hiệu ô ấy đã được vẽ.
    expect(html).not.toContain("THƯỜNG TRỰC ĐẢNG UỶ");
    expect(html).not.toContain("Bộ phận nhận thông báo");
    expect(html).not.toContain("org_unit");
  });

  it("KHÔNG CÓ NÚT `Lưu nháp` — không có tuyến nào tạo nháp", () => {
    // Vẽ nút ấy rồi cho nó phát hành luôn là biến một nút "lưu để sửa tiếp" thành một nút gửi đi
    // cả xã.
    expect(veForm()).not.toContain("Lưu nháp");
  });

  it("nút Phát hành TẮT khi chưa gõ gì — ba trường, không phải hai", () => {
    // Hợp đồng chỉ đánh dấu `title` và `body` bắt buộc, nhưng máy chủ còn từ chối một thông báo
    // không có người nhận nào.
    expect(veForm()).toContain("disabled");
  });

  it("ô tick gửi thư mặc định BẬT, kèm câu nói rõ chưa có thư nào đi", () => {
    const html = veForm();

    expect(html).toContain(nhuTrongHTML(CANH_BAO_CHUA_GUI_THU));
    // `checked` phải nằm trên đúng ô ấy, không phải trên một ô tick nào khác của biểu mẫu.
    expect(html).toContain('id="gui-thu-dien-tu"');
    expect(html).toMatch(/id="gui-thu-dien-tu"[^>]*checked/);
    expect(html).not.toMatch(/id="ghim-thong-bao"[^>]*checked/);
    expect(html).not.toMatch(/id="bat-buoc-xac-nhan"[^>]*checked/);
  });

  it("nói rõ ô người nhận nhận MÃ, không phải họ tên", () => {
    // Một ô text để cán bộ tự gõ sẽ được điền bằng họ tên nếu không ai nói khác, và giá trị ấy đi
    // thẳng vào cột mã của sổ lưu trữ.
    const html = veForm();

    expect(html).toContain("CB-2026-7K3M9Q");
    expect(html).toContain("không phải họ tên");
  });

  it("câu từ chối 501 của máy chủ hiện NGUYÊN VĂN", () => {
    const html = veForm(
      "Chưa gửi được thông báo theo bộ phận: hệ thống chưa lấy được danh sách cán bộ của một " +
        "bộ phận. Hãy chọn từng người ở mục \"Gửi thêm đích danh\".",
    );

    expect(html).toContain("Chưa gửi được thông báo theo bộ phận");
    expect(html).toContain(nhuTrongHTML('mục "Gửi thêm đích danh"'));
    expect(html).toContain('role="alert"');
  });
});

describe("phần chưa dựng được — ra tới màn hình, không giấu trong chú thích mã", () => {
  it("mọi mục có mặt", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(nhuTrongHTML(p.ten));
    }
  });

  it("bốn phát hiện nặng nhất gọi đích danh thứ còn thiếu", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);

    // 501 theo bộ phận — điều quan trọng nhất của màn này.
    expect(html).toContain("501");
    // Cổng quyền client: thứ người đọc màn sẽ đi tìm.
    expect(html).toContain("announcement.create");
    // Vì sao không có bộ lọc `Gửi cho tôi`: bảng `quyen` thiếu khoá đọc, và đó là câu hỏi mở #27.
    expect(html).toContain("#27");
    // Vì sao ghim không sắp lại cả sổ.
    expect(html).toContain("core/page");
  });
});
