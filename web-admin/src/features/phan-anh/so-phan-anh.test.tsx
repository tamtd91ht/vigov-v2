import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";

import {
  CANH_BAO_RE_NHANH,
  CAU_THIEU_QUYEN_DONG,
  CAU_THIEU_QUYEN_PHAN_CONG,
  CAU_THIEU_QUYEN_PHAN_LOAI,
  congThaoTac,
  DE_BO_PHAN_PHAN_CONG,
  NHAN_CHUYEN_CAP_TREN,
  NHAN_KHONG_TIEP_NHAN,
  NHAN_O_CO_QUAN,
  NHAN_O_KET_QUA,
  NHAN_O_LY_DO,
  NHAN_TIEN_TRANG_THAI,
  PHAM_VI_GIAO_CHO_TOI,
  PHAM_VI_TOAN_XA,
  PHAN_CHUA_DUNG,
  SO_RONG,
} from "./nhan-phieu";
import {
  BieuMauReNhanh,
  ChiTietPhieu,
  DanhSachThe,
  HangLoc,
  KhoiChuaDung,
  ThePhieu,
} from "./so-phan-anh";

const THU_MUC = fileURLToPath(new URL(".", import.meta.url));

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

const DANH_BA: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: {
    items: [
      { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "01JBOPHAN" },
      { code: "CB-00200", full_name: "Lê Văn C", position: "Trưởng thôn", department_id: "01JKHAC" },
    ],
  },
};

function veChiTiet(
  cong: ReturnType<typeof congThaoTac>,
  p = phieu(),
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null = DANH_BA,
): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={BAY_GIO}
      cong={cong}
      tenBoPhan={TEN_BO_PHAN}
      boPhan={BO_PHAN}
      danhBa={danhBa}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
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
    const html = veChiTiet(congThaoTac(false, false, true), phieu({ status: "cho-dan-xac-nhan" }));

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

  it("thiếu `feedback.assign`: không có khối Chuyển xử lý, và KHÔNG có ô chọn cán bộ", () => {
    const html = veChiTiet(congThaoTac(true, false, true));
    expect(html).not.toContain('id="chon-bo-phan"');
    expect(html).not.toContain('id="chon-can-bo"');
    expect(html).not.toContain(nhuTrongHTML(DE_BO_PHAN_PHAN_CONG));
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

describe("ô chọn cán bộ xử lý (§8.5) và ô `Đang giao cho` (§8.3)", () => {
  it("có `feedback.assign`: ô chọn cán bộ có mặt, mặc định `— Để bộ phận phân công —`", () => {
    const html = veChiTiet(congThaoTac(false, true, false));
    expect(html).toContain('id="chon-can-bo"');
    expect(html).toContain(nhuTrongHTML(DE_BO_PHAN_PHAN_CONG));
    // Chưa chọn bộ phận: ô chọn cán bộ khoá lại và không liệt kê ai — người được liệt kê là người
    // của bộ phận được chọn.
    expect(html).not.toContain('value="CB-00123"');
    expect(html).not.toContain("Lê Văn C");
  });

  it("ô `Đang giao cho` hiện HỌ TÊN tra từ danh bạ, không hiện mã", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ assignee: "CB-00123" }));
    expect(html).toContain("Trần Thị B");
    expect(html).not.toContain("CB-00123");
  });

  it("người giữ phiếu không còn trong danh bạ: hiện mã kèm câu trung tính, không `undefined`", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ assignee: "CB-00999" }));
    expect(html).toContain("CB-00999");
    expect(html).toContain("không có trong danh bạ");
    expect(html).not.toContain("undefined");
  });

  it("danh bạ tải hỏng: vẫn hiện mã người giữ phiếu, và nói ra câu lỗi ở khối chuyển xử lý", () => {
    const hong: KetQua<identity_danhBaChonNguoiRa> = {
      ok: false,
      thongBao: "Đã xảy ra lỗi. Vui lòng thử lại.",
    };
    const html = veChiTiet(congThaoTac(false, true, false), phieu({ assignee: "CB-00123" }), hong);
    expect(html).toContain("CB-00123");
    expect(html).toContain("Không tải được danh bạ cán bộ");
    // Chuyển cho bộ phận vẫn làm được — lỗi danh bạ không chặn đường ấy.
    expect(html).toContain('id="chon-bo-phan"');
  });
});

describe("hai tab phạm vi (§4)", () => {
  function veHangLoc(phamVi?: "all" | "mine"): string {
    return renderToStaticMarkup(
      <HangLoc
        loc={phamVi === undefined ? {} : { phamVi }}
        tim=""
        datTim={() => {}}
        datLoc={() => {}}
        boPhan={BO_PHAN}
        thon={[]}
      />,
    );
  }

  it("có `Toàn xã` và `Giao cho tôi`, KHÔNG có `Liên quan đến tôi`", () => {
    const html = veHangLoc();
    expect(html).toContain(PHAM_VI_TOAN_XA);
    expect(html).toContain(PHAM_VI_GIAO_CHO_TOI);
    expect(html).not.toContain("Liên quan đến tôi");
  });

  it("đúng MỘT tab được đánh dấu, theo bộ lọc đang chọn", () => {
    for (const phamVi of [undefined, "mine"] as const) {
      const html = veHangLoc(phamVi);
      expect(html.match(/aria-pressed="true"/g)?.length).toBe(1);
      const nutDangChon = html.match(/aria-pressed="true"[^>]*>([^<]*)</)?.[1];
      expect(nutDangChon).toBe(phamVi === "mine" ? PHAM_VI_GIAO_CHO_TOI : PHAM_VI_TOAN_XA);
    }
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

/**
 * HAI NHÁNH RẼ (`Không tiếp nhận`, `Chuyển cấp trên`) — ai thấy, ở đâu, và biểu mẫu nói gì.
 *
 * Canh cả CA BỊ TỪ CHỐI, không chỉ ca được phép: tài khoản người viết mã có mọi khoá, nên một nút
 * lọt ra ngoài cổng là thứ không ai thấy trong lúc phát triển.
 */
describe("hai nhánh rẽ — chỉ ở `dang-phan-loai`, chỉ với `feedback.classify`", () => {
  const NUT_KHONG_TIEP_NHAN = `>${NHAN_KHONG_TIEP_NHAN}</button>`;
  const NUT_CHUYEN_CAP_TREN = `>${NHAN_CHUYEN_CAP_TREN}</button>`;

  it("có `feedback.classify` và phiếu ở `dang-phan-loai`: hai nút có mặt", () => {
    const html = veChiTiet(congThaoTac(true, false, false), phieu({ status: "dang-phan-loai" }));
    expect(html).toContain(NUT_KHONG_TIEP_NHAN);
    expect(html).toContain(NUT_CHUYEN_CAP_TREN);
  });

  it("THIẾU `feedback.classify`: không nút nào, và câu thiếu quyền nói đúng tên khoá", () => {
    const html = veChiTiet(congThaoTac(false, true, true), phieu({ status: "dang-phan-loai" }));
    expect(html).not.toContain(NUT_KHONG_TIEP_NHAN);
    expect(html).not.toContain(NUT_CHUYEN_CAP_TREN);
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_PHAN_LOAI));
    expect(html).toContain("feedback.classify");
  });

  it("có khoá nhưng phiếu ở trạng thái khác: không nút nào — máy chủ sẽ trả 409", () => {
    for (const status of [
      "da-tiep-nhan",
      "da-chuyen-xu-ly",
      "dang-xu-ly",
      "da-xu-ly",
      "cho-dan-xac-nhan",
      "da-dong",
      "khong-tiep-nhan",
      "chuyen-cap-tren",
    ]) {
      const html = veChiTiet(congThaoTac(true, true, true), phieu({ status }));
      expect(html, status).not.toContain(NUT_KHONG_TIEP_NHAN);
      expect(html, status).not.toContain(NUT_CHUYEN_CAP_TREN);
    }
  });
});

describe("biểu mẫu nhánh rẽ", () => {
  function veBieuMau(
    loai: "khong-tiep-nhan" | "chuyen-cap-tren",
    lyDo = "",
    coQuan = "",
  ): string {
    return renderToStaticMarkup(
      <BieuMauReNhanh
        loai={loai}
        dangGui={false}
        gui={() => {}}
        huy={() => {}}
        lyDoBanDau={lyDo}
        coQuanBanDau={coQuan}
      />,
    );
  }

  /** Nút gửi là nút `type="submit"`; lấy riêng thẻ ấy để đọc `disabled`. */
  function nutGui(html: string): string {
    return html.match(/<button type="submit"[^>]*>/)?.[0] ?? "";
  }

  it("nhãn ô lý do nói người dân đọc được, và cảnh báo không hoàn tác + người dân được báo", () => {
    const html = veBieuMau("khong-tiep-nhan");
    expect(html).toContain(nhuTrongHTML(NHAN_O_LY_DO));
    expect(html).toContain(nhuTrongHTML(CANH_BAO_RE_NHANH));
    expect(CANH_BAO_RE_NHANH).toContain("không hoàn tác");
    expect(CANH_BAO_RE_NHANH).toContain("Người dân được thông báo");
  });

  it("`Không tiếp nhận` KHÔNG có ô cơ quan; `Chuyển cấp trên` CÓ", () => {
    expect(veBieuMau("khong-tiep-nhan")).not.toContain(nhuTrongHTML(NHAN_O_CO_QUAN));
    const html = veBieuMau("chuyen-cap-tren");
    expect(html).toContain(nhuTrongHTML(NHAN_O_CO_QUAN));
  });

  it("bộ đếm ký tự trực tiếp: 9 ký tự chữ Việt thì khoá nút, 10 thì mở", () => {
    const chin = "Ngập ước!"; // 9 điểm mã
    const muoi = "Ngập nước!"; // 10 điểm mã
    expect(nutGui(veBieuMau("khong-tiep-nhan", chin))).toContain("disabled");
    const html = veBieuMau("khong-tiep-nhan", muoi);
    expect(nutGui(html)).not.toContain("disabled");
    expect(html).toMatch(/10(<!-- -->)?\/(<!-- -->)?2000(<!-- -->)? ký tự/);
  });

  it("2000 ký tự chữ Việt thì mở, 2001 thì khoá", () => {
    expect(nutGui(veBieuMau("khong-tiep-nhan", "ệ".repeat(2000)))).not.toContain("disabled");
    expect(nutGui(veBieuMau("khong-tiep-nhan", "ệ".repeat(2001)))).toContain("disabled");
  });

  it("`Chuyển cấp trên` thiếu cơ quan tiếp nhận thì khoá nút, dù lý do hợp lệ", () => {
    const lyDo = "Vượt thẩm quyền của xã, thuộc ngành điện.";
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, ""))).toContain("disabled");
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, "   "))).toContain("disabled");
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, "Công ty điện lực"))).not.toContain(
      "disabled",
    );
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, "đ".repeat(201)))).toContain("disabled");
  });
});

describe("chi tiết phiếu ở nhánh rẽ — lý do, cơ quan, thời điểm", () => {
  const LY_DO = "Nội dung không thuộc địa bàn xã quản lý.";
  const CO_QUAN = "Công ty điện lực";

  it("`khong-tiep-nhan`: hiện lý do và thời điểm theo giờ Việt Nam, KHÔNG hiện ô cơ quan", () => {
    const html = veChiTiet(
      congThaoTac(true, true, true),
      phieu({
        status: "khong-tiep-nhan",
        reason: LY_DO,
        branch_ended_at: "2026-09-10T03:05:00Z",
      }),
    );
    expect(html).toContain("<dt>Lý do</dt>");
    expect(html).toContain(LY_DO);
    // 03:05Z = 10:05 giờ Việt Nam.
    expect(html).toContain("10:05");
    expect(html).not.toContain("<dt>Cơ quan tiếp nhận</dt>");
  });

  it("`chuyen-cap-tren`: hiện lý do, cơ quan tiếp nhận và thời điểm", () => {
    const html = veChiTiet(
      congThaoTac(false, false, false),
      phieu({
        status: "chuyen-cap-tren",
        reason: LY_DO,
        receiving_body: CO_QUAN,
        branch_ended_at: "2026-09-10T03:05:00Z",
      }),
    );
    expect(html).toContain(LY_DO);
    expect(html).toContain("<dt>Cơ quan tiếp nhận</dt>");
    expect(html).toContain(CO_QUAN);
    expect(html).toContain("10:05");
  });

  it("trạng thái khác: KHÔNG có ô lý do hay cơ quan, dù phản hồi lỡ mang theo", () => {
    for (const status of ["dang-phan-loai", "dang-xu-ly", "da-dong"]) {
      const html = veChiTiet(
        congThaoTac(true, true, true),
        phieu({ status, reason: LY_DO, receiving_body: CO_QUAN }),
      );
      expect(html, status).not.toContain("<dt>Lý do</dt>");
      expect(html, status).not.toContain("<dt>Cơ quan tiếp nhận</dt>");
      expect(html, status).not.toContain(LY_DO);
    }
  });

  it("thanh bước: ô rẽ nhánh đúng trạng thái là `đang ở đây`, luồng chính không ô nào", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ status: "chuyen-cap-tren" }));
    expect(html.match(/đang ở đây/g)?.length).toBe(1);
    expect(html).toMatch(/Chuyển cấp trên<\/span> (<!-- -->)?đang ở đây/);
  });
});

describe("Đóng phiếu — hai điểm đóng", () => {
  it("`cho-dan-xac-nhan`: có biểu mẫu đóng, kênh nào cũng vậy", () => {
    expect(
      veChiTiet(congThaoTac(false, false, true), phieu({ status: "cho-dan-xac-nhan" })),
    ).toContain(O_KET_QUA);
  });

  it("`da-xu-ly` kênh `can-bo-nhap-ho` (không có công dân để xác nhận): có biểu mẫu đóng", () => {
    const html = veChiTiet(
      congThaoTac(false, false, true),
      phieu({ status: "da-xu-ly", channel: "can-bo-nhap-ho" }),
    );
    expect(html).toContain(O_KET_QUA);
  });

  it("`da-xu-ly` kênh công dân: KHÔNG có biểu mẫu đóng — phải qua bước chờ dân xác nhận", () => {
    for (const channel of ["zalo-mini-app", "zalo-oa", "web-xa"]) {
      const html = veChiTiet(
        congThaoTac(false, false, true),
        phieu({ status: "da-xu-ly", channel }),
      );
      expect(html, channel).not.toContain(O_KET_QUA);
    }
  });

  it("`da-xu-ly` nhập hộ nhưng THIẾU `feedback.resolve`: không biểu mẫu, câu thiếu quyền", () => {
    const html = veChiTiet(
      congThaoTac(true, true, false),
      phieu({ status: "da-xu-ly", channel: "can-bo-nhap-ho" }),
    );
    expect(html).not.toContain(O_KET_QUA);
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DONG));
  });

  it("các trạng thái khác: không có biểu mẫu đóng", () => {
    for (const status of ["da-tiep-nhan", "dang-phan-loai", "dang-xu-ly", "da-dong", "khong-tiep-nhan"]) {
      const html = veChiTiet(
        congThaoTac(true, true, true),
        phieu({ status, channel: "can-bo-nhap-ho" }),
      );
      expect(html, status).not.toContain(O_KET_QUA);
    }
  });
});

describe("lý do nhánh rẽ không rời khỏi thân POST", () => {
  it("mã nguồn màn hình không đụng tới bộ nhớ trình duyệt hay console", () => {
    // Lý do là chữ cán bộ gõ về việc của một công dân (luật 3). Nó chỉ được đi vào thân POST.
    for (const tep of ["so-phan-anh.tsx", "nhan-phieu.ts"]) {
      const nguon = readFileSync(join(THU_MUC, tep), "utf-8");
      expect(nguon, tep).not.toMatch(/localStorage|sessionStorage|indexedDB|document\.cookie/);
      expect(nguon, tep).not.toMatch(/console\./);
    }
  });
});
