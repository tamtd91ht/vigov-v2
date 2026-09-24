import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type { identity_boPhanRa, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  CAU_THIEU_QUYEN_DONG,
  CAU_THIEU_QUYEN_PHAN_CONG,
  CAU_THIEU_QUYEN_PHAN_LOAI,
  congThaoTac,
  NHAN_O_KET_QUA,
  NHAN_TIEN_TRANG_THAI,
  PHAN_CHUA_DUNG,
  SO_RONG,
} from "./nhan-phieu";
import { ChiTietPhieu, DanhSachThe, KhoiChuaDung, ThePhieu } from "./so-phan-anh";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **HAI CỔNG KHÁC NHAU**, và nó là nhóm không ai nhìn thấy
 * trong lúc phát triển: tài khoản người viết mã có cả bốn khoá, nên cả bốn nút luôn hiện. Điều
 * phải đúng là chuyện ngược lại — một trưởng thôn chỉ có `feedback.read` và đang giữ một phiếu
 * **tiến được trạng thái phiếu ấy** nhưng **KHÔNG đóng được nó**.
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

function phieu(sua: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-xu-ly",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.",
    address: "Tổ 6, thôn Hà Lam",
    // SỐ ĐÃ CHE SẴN Ở MÁY CHỦ. Số mẫu theo luật 3, bất biến 5.
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: "2026-09-09T09:20:00Z",
    resolve_due: "2026-09-10T09:20:00Z",
    classify_due: "2026-09-09T11:20:00Z",
    unit: "01JBOPHAN",
    assignee: "",
    result: "",
    public: false,
    ...sua,
  };
}

const BO_PHAN: identity_boPhanRa[] = [
  { id: "01JBOPHAN", code: "vp-dang-uy", name: "VĂN PHÒNG ĐẢNG ỦY", parent_id: "", order: 0, staff_count: 0 },
];
const TEN_BO_PHAN = new Map(BO_PHAN.map((b) => [b.id, b.name]));

const BAY_GIO = new Date("2026-09-10T02:00:00Z");

function veChiTiet(cong: ReturnType<typeof congThaoTac>, p = phieu()): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={BAY_GIO}
      cong={cong}
      tenBoPhan={TEN_BO_PHAN}
      boPhan={BO_PHAN}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
    />,
  );
}

/** Chỉ dấu KHÔNG THỂ NHẦM của biểu mẫu đóng phiếu: id của ô kết quả. */
const O_KET_QUA = 'id="ket-qua-xu-ly"';

describe("HAI CỔNG KHÁC NHAU — nút tiến trạng thái và nút Đóng phiếu", () => {
  it("chỉ có LUẬT NẮM GIỮ (không `feedback.resolve`): tiến được trạng thái, KHÔNG có nút Đóng phiếu", () => {
    // Đây là tài khoản trưởng thôn: xem được sổ, đang giữ một phiếu, không có khoá toàn xã.
    const html = veChiTiet(congThaoTac(false, false, false));

    // Nút tiến trạng thái CÓ — luật nắm giữ mở nó, và giao diện không được lấy mất.
    expect(html).toContain(nhuTrongHTML(NHAN_TIEN_TRANG_THAI));

    // Biểu mẫu đóng phiếu KHÔNG ra tới trang. Canh bằng id của ô kết quả và bằng nhãn của nó, chứ
    // KHÔNG bằng chuỗi "Đóng phiếu": chính câu từ chối `CAU_THIEU_QUYEN_DONG` cũng chứa hai chữ
    // ấy, nên một phép `not.toContain("Đóng phiếu")` sẽ đỏ vì lý do sai — hoặc xanh vì lý do sai
    // vào ngày câu ấy đổi.
    expect(html).not.toContain(O_KET_QUA);
    expect(html).not.toContain(nhuTrongHTML(NHAN_O_KET_QUA));

    // Và cán bộ được nói cho biết mình thiếu ĐÚNG khoá nào.
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DONG));
    expect(html).toContain("feedback.resolve");
  });

  it("có `feedback.resolve`: biểu mẫu đóng phiếu ra tới trang, kèm ô kết quả người dân đọc", () => {
    const html = veChiTiet(congThaoTac(false, false, true));

    expect(html).toContain(O_KET_QUA);
    expect(html).toContain(nhuTrongHTML(NHAN_O_KET_QUA));
    // Nút tiến trạng thái vẫn còn: khoá toàn xã KHÔNG thay thế luật nắm giữ, nó chỉ thêm vào.
    expect(html).toContain(nhuTrongHTML(NHAN_TIEN_TRANG_THAI));
    expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DONG));
  });

  it("nút tiến trạng thái KHÔNG bị gắn sau `feedback.resolve` — bốn bộ quyền, bốn lần vẫn có", () => {
    // VẾ CHỊU LỰC. Bài này đỏ đúng vào ngày ai đó "gộp cho gọn" hai cổng làm một — thao tác trông
    // hợp lý, không làm hỏng màn hình của người viết mã, và lấy mất khả năng xử lý việc của mọi
    // trưởng thôn trong xã.
    for (const cong of [
      congThaoTac(false, false, false),
      congThaoTac(true, false, false),
      congThaoTac(false, true, false),
      congThaoTac(true, true, true),
    ]) {
      expect(veChiTiet(cong)).toContain(nhuTrongHTML(NHAN_TIEN_TRANG_THAI));
    }
  });

  it("phiếu đã ở bước cuối luồng chính thì không còn nút tiến — không phải vì quyền", () => {
    const html = veChiTiet(congThaoTac(true, true, true), phieu({ status: "da-dong" }));
    expect(html).not.toContain(nhuTrongHTML(NHAN_TIEN_TRANG_THAI));
  });
});

describe("CỔNG QUYỀN — phân loại và chuyển xử lý", () => {
  it("thiếu `feedback.classify`: không có ô chọn lĩnh vực, và câu từ chối gọi đúng tên khoá", () => {
    const html = veChiTiet(congThaoTac(false, true, true), phieu({ status: "da-tiep-nhan" }));
    expect(html).not.toContain('id="chon-linh-vuc"');
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_PHAN_LOAI));
    expect(html).toContain("feedback.classify");
  });

  it("thiếu `feedback.assign`: không có khối Chuyển xử lý", () => {
    const html = veChiTiet(congThaoTac(true, false, true));
    expect(html).not.toContain('id="chon-bo-phan"');
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_PHAN_CONG));
    expect(html).toContain("feedback.assign");
  });

  it("có `feedback.classify` nhưng phiếu đã qua bước phân loại: không vẽ ô chọn", () => {
    // Máy chủ chốt lĩnh vực bằng câu UPDATE mang `trang_thai = 'da-tiep-nhan'`, nên lần thứ hai là
    // 409. Ẩn ô chọn là nói ra điều ấy trước, không phải dựng thêm một luật.
    const html = veChiTiet(congThaoTac(true, true, true), phieu({ status: "dang-xu-ly" }));
    expect(html).not.toContain('id="chon-linh-vuc"');
    // Và KHÔNG hiện câu "thiếu quyền" — tài khoản có quyền, chỉ là phiếu đã qua bước ấy.
    expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_PHAN_LOAI));
  });

  it("thiếu `feedback.read`: cả màn không dựng", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen
        quyetDinh={{ hien: false, vi: "khong-du-quyen" }}
        cauThieuQuyen="Tài khoản của bạn không có quyền xem phản ánh của người dân (feedback.read)."
      >
        <ThePhieu
          phieu={phieu()}
          tenBoPhan={TEN_BO_PHAN}
          bayGio={BAY_GIO}
          dangMo={false}
          mo={() => {}}
        />
      </KhungQuyen>,
    );
    expect(html).toContain("feedback.read");
    expect(html).not.toContain("PA-2026-0021");
  });
});

describe("dữ liệu cá nhân — màn hình hiện đúng thứ máy chủ gửi", () => {
  it("số điện thoại ra màn hình đúng dạng đã che, không ghép lại chữ số nào", () => {
    const html = renderToStaticMarkup(
      <ThePhieu
        phieu={phieu()}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        dangMo={false}
        mo={() => {}}
      />,
    );
    expect(html).toContain("09****0000");
    // Không có phép "làm đẹp" nào biến dấu sao thành chữ số.
    expect(html).not.toMatch(/09\d{8}/);
  });

  it("phiếu ẩn danh: KHÔNG có tên, KHÔNG có số, và không bù vào bằng gì cả", () => {
    const html = renderToStaticMarkup(
      <ThePhieu
        phieu={phieu({ anonymous: true, reporter_name: "", reporter_phone: "" })}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        dangMo={false}
        mo={() => {}}
      />,
    );
    expect(html).toContain("Người gửi ẩn danh");
    expect(html).not.toContain("Nguyễn");
    expect(html).not.toContain("09****0000");
  });
});

describe("lĩnh vực hạn chế — màn hình KHÔNG nói ra rằng có phiếu bị giấu", () => {
  it("sổ rỗng thì hiện câu trạng thái rỗng, không một chữ nào về phiếu bị ẩn", () => {
    // Máy chủ loại hẳn phiếu `can-bo` khỏi trang VÀ khỏi con trỏ khi tài khoản thiếu
    // `feedback.restricted`. Một câu kiểu "có n phiếu bị ẩn" ở đây là nói cho một đồng nghiệp của
    // người bị phản ánh biết rằng phiếu ấy tồn tại (luật 4, cấm #2).
    const html = renderToStaticMarkup(
      <DanhSachThe
        phieu={[]}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moPhieu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(SO_RONG));
    expect(html).not.toMatch(/bị ẩn|hạn chế|feedback\.restricted/);
  });
});

describe("phần chưa dựng được — ra tới màn hình, không giấu trong chú thích mã", () => {
  it("khối ấy nêu đích danh bảng ảnh còn thiếu và hệ quả với luật “phải có ảnh sau”", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);
    expect(html).toContain("anh_phan_anh");
    expect(html).toContain("nhat_ky_phan_anh");
    // Hệ quả nặng nhất phải có mặt: không cưỡng chế được luật “không đóng phiếu khi thiếu ảnh sau”.
    expect(html).toContain("bat_buoc_anh_nghiem_thu");
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(nhuTrongHTML(p.ten));
    }
  });
});
