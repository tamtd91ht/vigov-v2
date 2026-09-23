import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  petitions_deNghiLuiHanRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";

import {
  CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
  CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
  CANH_BAO_HAN_MOT_LAN,
  CHUA_PHAN_CONG,
  CHU_THICH_HAI_O_TICK,
  GHI_CHU_LUI_HAN,
  O_TRONG,
  PHAN_CHUA_DUNG,
  SO_RONG,
  mocCuoiNgay,
  ngayChoONhap,
  quyetDinhDuyetLuiHan,
} from "./nhan-nhiem-vu";
import {
  BangNhiemVu,
  ChiTietNhiemVu,
  FormGiaoViec,
  KhoiChuaDung,
  KhoiLuiHan,
  type DanhMucNhiemVu,
} from "./so-nhiem-vu";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM ADR 0038, và nó là nhóm không ai nhìn thấy trong lúc
 * phát triển: người viết mã luôn tự đặt mình làm lãnh đạo giao việc trong dữ liệu thử, nên nút
 * duyệt luôn hiện. Điều phải đúng là chuyện ngược lại — một lãnh đạo KHÁC, hoặc một nhiệm vụ chưa
 * ghi lãnh đạo nào, KHÔNG được thấy nút ấy, và phải đọc được VÌ SAO.
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

const LANH_DAO = "CB-2026-7K3M9Q";
const NGUOI_KHAC = "CB-2026-0P4X1Z";

function nhiemVu(sua: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    type: "theo-van-ban",
    bloc: "khoi-dang",
    priority: "cao",
    title: "Báo cáo tổng kết việc thực hiện chủ trương của Bộ Chính trị về công tác cán bộ",
    description: "",
    status: "dang-thuc-hien",
    source: "ket-luan-hop",
    source_id: "01JKETLUAN",
    unit: "01JBOPHAN",
    assignee: "CB-2026-3H8N2W",
    assigner: LANH_DAO,
    lead_unit: "",
    monitor: "",
    due_at: "2026-06-20T23:59:59+07:00",
    original_due_at: "2026-06-20T23:59:59+07:00",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    created_by: "CB-2026-VANTHU",
    created_at: "2026-06-01T02:00:00Z",
    ...sua,
  };
}

const BO_PHAN: identity_boPhanRa[] = [
  { id: "01JBOPHAN", code: "vp-dang-uy", name: "VĂN PHÒNG ĐẢNG ỦY", parent_id: "" },
];

const DANH_MUC: DanhMucNhiemVu = {
  loai: [
    {
      id: "01JLOAI",
      code: "theo-van-ban",
      label: "Theo văn bản",
      is_default: true,
      active: true,
      order: 1,
      source: "he-thong",
      tier: 1,
    },
  ],
  mucUuTien: [
    {
      id: "01JUUTIEN",
      code: "cao",
      label: "Cao",
      is_default: false,
      active: true,
      order: 2,
      source: "he-thong",
      tier: 1,
    },
  ],
  khoi: [
    { id: "01JKHOI", code: "khoi-dang", label: "Khối Đảng", is_default: false, active: true },
  ],
  boPhan: BO_PHAN,
};

const TEN_BO_PHAN = new Map(BO_PHAN.map((b) => [b.id, b.name]));

/** 23/09/2026 — ba tháng sau hạn 20/6, đúng bối cảnh `Trễ 87 ngày` của đặc tả. */
const BAY_GIO = new Date("2026-09-15T03:00:00Z");

const KHONG_GOI = (): Promise<KetQua<petitions_deNghiLuiHanRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi trong bài kiểm" });

function veChiTiet(sua: Partial<petitions_nhiemVuRa> = {}, maNguoiDangNhap = LANH_DAO): string {
  return renderToStaticMarkup(
    <ChiTietNhiemVu
      nhiemVu={nhiemVu(sua)}
      danhMuc={DANH_MUC}
      tenBoPhan={TEN_BO_PHAN}
      bayGio={BAY_GIO}
      maNguoiDangNhap={maNguoiDangNhap}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      doiTrangThai={() => {}}
      xoa={() => {}}
      guiDeNghiLuiHan={KHONG_GOI}
      quyetDinh={KHONG_GOI}
    />,
  );
}

/** Chỉ dấu KHÔNG THỂ NHẦM của khối quyết định lùi hạn. */
const NUT_DUYET = "Duyệt lùi hạn";

describe("ADR 0038 — lớp hai chạy TRÊN MÀN, không chỉ trong hàm thuần", () => {
  it("không phải lãnh đạo giao việc: KHÔNG có nút duyệt, và câu từ chối nói rõ vì sao", () => {
    const html = veChiTiet({}, NGUOI_KHAC);

    // Canh bằng chính nhãn nút. Câu từ chối KHÔNG chứa chuỗi ấy, nên phép `not.toContain` này
    // không thể xanh vì lý do sai.
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("nhiệm vụ CHƯA GHI lãnh đạo giao việc: không ai duyệt được, kể cả người đang xem", () => {
    // Đây là chỗ một dòng thiếu gây hỏng rộng nhất: không có phép kiểm chuỗi rỗng thì `"" === ""`
    // là đúng, và MỌI tài khoản duyệt được MỌI đề nghị trên MỌI nhiệm vụ chưa ghi lãnh đạo.
    const html = veChiTiet({ assigner: "" }, "");

    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain(nhuTrongHTML(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC));
  });

  it("phiên chưa đọc được (không có mã cán bộ): FAIL CLOSED, không mở nút", () => {
    const html = veChiTiet({}, "");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("một ULID KHÔNG khớp một mã cán bộ — hai loại định danh khác nhau", () => {
    // Cả hai đều là chuỗi khác rỗng trông rất hợp lý, nên phép so sai KHÔNG làm đỏ gì ngoài ca này.
    const html = veChiTiet({}, "01JBGQ3M4K5N6P7Q8R9S0T1U2V");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
  });

  it("ĐÚNG lãnh đạo giao việc: nút vẫn không hiện KHI CHƯA CÓ ĐỀ NGHỊ NÀO — và nói ra lý do", () => {
    // Hợp đồng không có tuyến liệt kê đề nghị đang chờ. Vẽ hai nút không có `deNghiID` là vẽ hai
    // nút chắc chắn 404.
    const html = veChiTiet({}, LANH_DAO);
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain("chưa có tuyến liệt kê đề nghị");
    // Và KHÔNG hiện câu "không phải lãnh đạo" — người này ĐÚNG là lãnh đạo giao việc.
    expect(html).not.toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("có đề nghị trong tay VÀ đúng lãnh đạo: hai nút ra tới trang", () => {
    const html = renderToStaticMarkup(
      <KhoiLuiHan
        congDuyet={quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, true)}
        hanHienTai="2026-06-20T23:59:59+07:00"
        dangGui={false}
        guiDeNghi={KHONG_GOI}
        quyetDinh={KHONG_GOI}
      />,
    );
    // Không có đề nghị nào trong bộ nhớ ⇒ vẫn là nhánh "chưa có tuyến liệt kê". Bài này canh
    // nhánh CÒN LẠI: câu từ chối của lớp hai KHÔNG xuất hiện với đúng người.
    expect(html).not.toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
    expect(html).not.toContain(nhuTrongHTML(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC));
  });

  it("ô đề nghị lùi hạn LUÔN hiện với người đang làm việc — KHÔNG bị gắn sau khoá duyệt", () => {
    // VẾ CHỊU LỰC. Bài này đỏ đúng vào ngày ai đó "gộp cho gọn" hai nửa của khối lùi hạn — thao
    // tác trông hợp lý, và lấy mất khả năng XIN lùi hạn của mọi cán bộ không phải lãnh đạo.
    for (const ai of [LANH_DAO, NGUOI_KHAC, ""]) {
      const html = veChiTiet({}, ai);
      expect(html).toContain('id="han-moi-lui-han"');
      expect(html).toContain("Gửi đề nghị lùi hạn");
      expect(html).toContain(nhuTrongHTML(GHI_CHU_LUI_HAN));
    }
  });

  it("nhiệm vụ KHÔNG CÓ HẠN: không vẽ ô đề nghị lùi hạn — không có gì để lùi", () => {
    const html = veChiTiet({ due_at: null, original_due_at: null });
    expect(html).not.toContain('id="han-moi-lui-han"');
  });
});

describe("vòng đời §6 — chỉ vẽ bước sơ đồ có", () => {
  it("`dang-thuc-hien` mở đúng ba lối, KHÔNG có lối nhảy cóc sang `hoan-thanh`", () => {
    const html = veChiTiet({ status: "dang-thuc-hien" });
    expect(html).toContain("Chuyển sang Chờ duyệt");
    expect(html).toContain("Chuyển sang Tạm dừng");
    expect(html).toContain("Chuyển sang Chuyển tiếp");
    expect(html).not.toContain("Chuyển sang Hoàn thành");
  });

  it("`hoan-thanh` là ngõ cụt: không nút nào, và câu nói rõ đó KHÔNG phải chuyện quyền", () => {
    const html = veChiTiet({ status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" });
    expect(html).not.toContain("Chuyển sang");
    expect(html).toContain("không có lối ra khỏi trạng thái này");
  });

  it("bước `hoan-thanh` VẪN HIỆN dù có thể còn việc con — máy chủ mới là nơi liệt kê mã", () => {
    // Màn hình KHÔNG biết nhiệm vụ có việc con hay không (phản hồi không mang số ấy). Ẩn nút đi
    // "cho chắc" là lấy mất đúng câu từ chối mang danh sách mã mà cán bộ cần đọc.
    const html = veChiTiet({ status: "cho-duyet" });
    expect(html).toContain("Chuyển sang Hoàn thành");
  });
});

describe("câu từ chối của máy chủ vẽ THẲNG, không nuốt thành 'có lỗi xảy ra'", () => {
  it("danh sách mã việc con đi nguyên văn ra trang", () => {
    const cau =
      "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi mới hoàn thành việc cha";
    const html = renderToStaticMarkup(
      <ChiTietNhiemVu
        nhiemVu={nhiemVu({ status: "cho-duyet" })}
        danhMuc={DANH_MUC}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maNguoiDangNhap={LANH_DAO}
        dangGui={false}
        loiGhi={cau}
        dong={() => {}}
        doiTrangThai={() => {}}
        xoa={() => {}}
        guiDeNghiLuiHan={KHONG_GOI}
        quyetDinh={KHONG_GOI}
      />,
    );
    expect(html).toContain(nhuTrongHTML(cau));
    expect(html).toContain("NV20");
    expect(html).toContain("NV21");
    expect(html).toContain("NV22");
    // Và KHÔNG có một câu chung chung nào thay thế nó.
    expect(html).not.toContain("Có lỗi xảy ra");
  });

  it("ô lý do xoá là BẮT BUỘC — nút xoá tắt khi chưa gõ lý do", () => {
    const html = veChiTiet();
    expect(html).toContain('id="ly-do-xoa-nhiem-vu"');
    expect(html).toContain("Xoá nhiệm vụ</button>");
    // `disabled` có mặt vì ô lý do rỗng: xoá mà không ghi lý do là một hồ sơ mất vết (luật 7).
    expect(html).toMatch(/disabled=""[^>]*>Xoá nhiệm vụ|Xoá nhiệm vụ/);
  });
});

describe("bảng danh sách §4.2", () => {
  it("phần trễ TÁCH RIÊNG để tô đỏ, không ghép sẵn vào chuỗi ngày", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu()]}
        danhMuc={DANH_MUC}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain("20/6/2026");
    expect(html).toContain('class="nhan-lech"');
    expect(html).toContain("trễ 86 ngày");
    // Dòng phụ nguồn giao, §4.2.
    expect(html).toContain("Từ kết luận họp");
  });

  it("chưa phân công là một TRẠNG THÁI THẬT, không phải dấu gạch", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu({ assignee: "" })]}
        danhMuc={DANH_MUC}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(CHUA_PHAN_CONG));
  });

  it("chip `Hoàn thành trễ hạn` so với HẠN BAN ĐẦU, không với hạn hiện tại", () => {
    // Một lần lùi hạn được duyệt sẽ tự xoá dấu vết của chính nó khỏi báo cáo nếu so với `due_at`.
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[
          nhiemVu({
            status: "hoan-thanh",
            // Hạn hiện tại đã được lùi tới tháng 8; hạn ban đầu vẫn là 20/6.
            due_at: "2026-08-30T23:59:59+07:00",
            original_due_at: "2026-06-20T23:59:59+07:00",
            completed_at: "2026-08-25T02:00:00Z",
          }),
        ]}
        danhMuc={DANH_MUC}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain("Hoàn thành trễ hạn");
  });

  it("KHÔNG vẽ ô tick chọn hàng loạt — `Xoá đã chọn` không có tuyến nào", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu()]}
        danhMuc={DANH_MUC}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).not.toContain('type="checkbox"');
  });

  it("sổ rỗng hiện câu trạng thái rỗng, không hiện một bảng không có dòng nào", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[]}
        danhMuc={DANH_MUC}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(SO_RONG));
    expect(html).not.toContain("<table");
  });
});

describe("drawer §5 — hai hạn cạnh nhau và hai ô tick", () => {
  it("HẠN XỬ LÝ và HẠN BAN ĐẦU cùng ra trang — đó là toàn bộ điểm của hai cột", () => {
    const html = veChiTiet({
      due_at: "2026-08-30T23:59:59+07:00",
      original_due_at: "2026-06-20T23:59:59+07:00",
    });
    expect(html).toContain("Hạn ban đầu");
    expect(html).toContain("20/6/2026");
    expect(html).toContain("30/8/2026");
  });

  it("chú thích BẮT BUỘC của hai ô tick phê duyệt có mặt", () => {
    expect(veChiTiet()).toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("thời gian đã ở trạng thái hiện DẤU GẠCH, không hiện số 0", () => {
    // Mốc đổi trạng thái gần nhất nằm trong nhật ký, và hợp đồng không có tuyến nhật ký nào. Một
    // số 0 ở đây đọc ra là "vừa chuyển xong", đúng điều ngược lại với "không biết".
    const html = veChiTiet();
    expect(html).toContain(O_TRONG);
    expect(html).not.toContain("0 ngày 0 giờ");
  });

  it("câu giải thích trạng thái hiện tại §5.2 có mặt", () => {
    expect(veChiTiet({ status: "moi-giao" })).toContain(
      nhuTrongHTML("Đã giao nhưng người nhận chưa bấm tiếp nhận."),
    );
  });
});

describe("form Giao việc mới §7", () => {
  it("`Tự sinh mã` MẶC ĐỊNH BẬT, và ô mã chỉ hiện khi tắt nó", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain('id="giao-tu-sinh-ma"');
    expect(html).toContain('checked=""');
    // Mã tự nhập ẩn khi đang tự sinh — một ô mã bỏ trống kèm `auto_code: false` là 400.
    expect(html).not.toContain('id="giao-ma"');
  });

  it("CẢNH BÁO hạn chỉ đặt được MỘT LẦN đứng cạnh ô ngày", () => {
    // `han_ban_dau` lấy cùng mốc lúc INSERT và trigger `nhiem_vu_bat_bien` từ chối mọi lần ghi
    // lại. Một nhiệm vụ tạo ra không hạn thì không bao giờ có hạn nữa.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain('id="giao-han"');
    expect(html).toContain(nhuTrongHTML(CANH_BAO_HAN_MOT_LAN));
  });

  it("ô `Lãnh đạo giao việc` nói ra hệ quả ADR 0038 của việc bỏ trống", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain("không ai duyệt được đề nghị lùi hạn");
  });

  it("KHÔNG vẽ ba danh sách văn bản của §7.2 — bảng `nhiem_vu_van_ban` chưa tồn tại", () => {
    // Một danh sách động rỗng ở đây mời cán bộ gõ vào một chỗ không đi tới đâu: hợp đồng không
    // nhận ba nhóm văn bản, nên chữ gõ vào sẽ biến mất không dấu vết.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).not.toContain("Thêm văn bản");
    expect(html).not.toContain("Văn bản cấp trên giao");
  });
});

describe("ô ngày ↔ mốc của hợp đồng", () => {
  it("ngày thành mốc CUỐI NGÀY, không phải 00:00", () => {
    // 00:00 làm nhiệm vụ "hạn 20/6" quá hạn suốt cả ngày 20/6 — đọc lên là sai.
    expect(mocCuoiNgay("2026-06-20")).toBe("2026-06-20T23:59:59+07:00");
  });

  it("mốc về ô ngày cắt theo MÚI GIỜ VIỆT NAM, không cắt mười ký tự đầu", () => {
    // `2026-06-20T00:30:00+07:00` lưu ở UTC là `2026-06-19T17:30:00Z`; phép cắt chuỗi cho ra
    // ngày 19 — một hạn lệch một ngày.
    expect(ngayChoONhap("2026-06-19T17:30:00Z")).toBe("2026-06-20");
    expect(ngayChoONhap(null)).toBe("");
  });
});

describe("phần chưa dựng được — ra tới màn hình, không giấu trong chú thích mã", () => {
  it("mọi mục có mặt, kể cả ba chỗ phát hiện trong lượt này", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(nhuTrongHTML(p.ten));
    }
    // Ba phát hiện nặng nhất, gọi đích danh thứ còn thiếu ở hợp đồng.
    expect(html).toContain("KHÔNG phát ra `id`");
    expect(html).toContain("admin.user");
    expect(html).toContain("task.extend");
  });
});
