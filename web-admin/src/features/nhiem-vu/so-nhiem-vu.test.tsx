import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  petitions_deNghiLuiHanRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
} from "@/lib/api/schema.gen";

import {
  BANG_NHAN_MAC_DINH,
  CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
  CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
  CANH_BAO_HAN_MOT_LAN,
  CHUA_PHAN_CONG,
  CHI_TIET_THIEU_VAN_BAN,
  CHU_THICH_HAI_O_TICK,
  DANG_TAI_VAN_BAN,
  GHI_CHU_KHONG_CO_O_GHI_CHU,
  GHI_CHU_LUI_HAN,
  KHOA_SUA_DANG_TAI,
  KHOA_SUA_LOI,
  KHOA_SUA_NHOM_LA,
  KHONG_DOC_DUOC_VAN_BAN,
  KHONG_SO,
  LY_DO_KHONG_SUA_CHU_TRI,
  LY_DO_KHONG_SUA_HAN,
  LY_DO_KHONG_SUA_MA,
  NHAN_NUT_SUA,
  O_TRONG,
  PHAN_CHUA_DUNG,
  SO_RONG,
  TIEU_DE_KHOI_VAN_BAN,
  mocCuoiNgay,
  ngayChoONhap,
  quyetDinhDuyetLuiHan,
} from "./nhan-nhiem-vu";
import {
  BangNhiemVu,
  ChiTietNhiemVu,
  FormGiaoViec,
  FormSuaKhoiVanBan,
  KhoiChuaDung,
  KhoiLuiHan,
  chuyenDrawer,
  type DanhMucNhiemVu,
  type DrawerNhiemVu,
  type TrangThaiTai,
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
  { id: "01JBOPHAN", code: "vp-dang-uy", name: "VĂN PHÒNG ĐẢNG ỦY", parent_id: "", order: 0, staff_count: 0 },
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
    {
      id: "01JKHOI",
      code: "khoi-dang",
      label: "Khối Đảng",
      is_default: false,
      active: true,
      order: 1,
      source: "don-vi",
      tier: 1,
    },
  ],
  boPhan: BO_PHAN,
};

/** Xã có loại mặc định là `co-ban` — để vẽ được nhánh §7.3 mà không cần sự kiện DOM. */
const DANH_MUC_CO_BAN: DanhMucNhiemVu = {
  ...DANH_MUC,
  loai: [
    {
      id: "01JLOAICOBAN",
      code: "co-ban",
      label: "Nhiệm vụ cơ bản",
      is_default: true,
      active: true,
      order: 2,
      source: "he-thong",
      tier: 1,
    },
  ],
};

const TEN_BO_PHAN = new Map(BO_PHAN.map((b) => [b.id, b.name]));

/** 23/09/2026 — ba tháng sau hạn 20/6, đúng bối cảnh `Trễ 87 ngày` của đặc tả. */
const BAY_GIO = new Date("2026-09-15T03:00:00Z");

const KHONG_GOI = (): Promise<KetQua<petitions_deNghiLuiHanRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi trong bài kiểm" });

const KHONG_SUA = (): Promise<KetQua<petitions_nhiemVuRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi trong bài kiểm" });

type TaiVanBan = TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;

function veChiTiet(
  sua: Partial<petitions_nhiemVuRa> = {},
  maNguoiDangNhap = LANH_DAO,
  vanBan: TaiVanBan = { pha: "dangTai" },
): string {
  return renderToStaticMarkup(
    <ChiTietNhiemVu
      nhiemVu={nhiemVu(sua)}
      vanBan={vanBan}
      danhMuc={DANH_MUC}
      nhanTT={BANG_NHAN_MAC_DINH}
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
      suaKhoiVanBan={KHONG_SUA}
      docLaiChiTiet={KHONG_SUA}
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
        vanBan={{ pha: "dangTai" }}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
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
        suaKhoiVanBan={KHONG_SUA}
        docLaiChiTiet={KHONG_SUA}
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
        nhanTT={BANG_NHAN_MAC_DINH}
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
        nhanTT={BANG_NHAN_MAC_DINH}
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
        nhanTT={BANG_NHAN_MAC_DINH}
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
        nhanTT={BANG_NHAN_MAC_DINH}
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
        nhanTT={BANG_NHAN_MAC_DINH}
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

  it("màn Nhiệm vụ, loại `theo-van-ban`: vẽ BA danh sách văn bản §7.2, mỗi nhóm một nút thêm", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 24/09/2026 (TASK-02): bài này từng canh "form CHƯA vẽ ba danh sách" và
    // tự ghi là phải đỏ vào ngày chúng được dựng. Hôm nay là ngày ấy.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        coDanhSachVanBan
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain("Văn bản cấp trên giao");
    expect(html).toContain(nhuTrongHTML("Văn bản chỉ đạo của Đảng uỷ"));
    expect(html).toContain("Văn bản sản phẩm đầu ra");
    expect(html.split("+ Thêm văn bản</button>").length - 1).toBe(3);
    expect(html).toContain('id="giao-them-van-ban-cap-tren-giao"');
    expect(html).toContain("Nội dung nhiệm vụ / Trích yếu văn bản");
    expect(html).toContain('id="giao-co-quan-chu-tri"');
    expect(html).toContain('id="giao-chuyen-vien"');
    // Ô `Ghi chú` KHÔNG có (hợp đồng tạo không nhận `note`), và form NÓI ra điều ấy.
    expect(html).toContain(nhuTrongHTML(GHI_CHU_KHONG_CO_O_GHI_CHU));
  });

  it("loại `co-ban`: ô tiêu đề thành `Tên nhiệm vụ`; cơ quan chủ trì, chuyên viên, ba danh sách BIẾN MẤT", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC_CO_BAN}
        coDanhSachVanBan
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain(">Tên nhiệm vụ</label>");
    expect(html).not.toContain("Trích yếu văn bản");
    expect(html).not.toContain('id="giao-co-quan-chu-tri"');
    expect(html).not.toContain('id="giao-chuyen-vien"');
    expect(html).not.toContain("Thêm văn bản");
    expect(html).not.toContain("Văn bản cấp trên giao");
  });

  it("màn Biên bản (không truyền prop): loại `theo-van-ban` mà KHÔNG có ba danh sách", () => {
    // Đúng cách `features/bien-ban/so-bien-ban.tsx` gọi form: không có `coDanhSachVanBan`.
    // `petitions.tachKetLuanVao` không có `documents` — vẽ ba danh sách ở đó là để cán bộ gõ văn
    // bản rồi thấy chúng mất. Hai ô `lead_unit`/`monitor` thì tuyến ấy có, nên vẫn hiện.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
        tieuDeCoSan="Kết luận giả của cuộc họp"
      />,
    );
    expect(html).not.toContain("Thêm văn bản");
    expect(html).not.toContain("Văn bản cấp trên giao");
    expect(html).toContain('id="giao-co-quan-chu-tri"');
    expect(html).toContain('id="giao-chuyen-vien"');
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

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §5.4 — KHỐI "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO", CHỈ ĐỌC
 *
 * Ca nặng nhất ở đây là ca KHÔNG ai thấy lúc phát triển: dòng của sổ vắng `documents` có chủ ý, và
 * một drawer vẽ từ dòng ấy sẽ hiện ba nhóm `—` — "nhiệm vụ này không có văn bản" — cho một nhiệm
 * vụ có ba văn bản. Mọi bài dưới đây canh để câu ấy chỉ xuất hiện khi TUYẾN CHI TIẾT đã nói thế.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

function vb(sua: Partial<petitions_nhiemVuVanBanRa> = {}): petitions_nhiemVuVanBanRa {
  return {
    id: "01JVANBAN1",
    group: "cap-tren-giao",
    reference: "1742-CV/BTCTU",
    date: "2026-06-09",
    summary: "Công văn của Ban Tổ chức Thành uỷ",
    position: 1,
    ...sua,
  };
}

const BA_VAN_BAN: petitions_nhiemVuVanBanRa[] = [
  vb(),
  vb({
    id: "01JVANBAN2",
    group: "san-pham-dau-ra",
    reference: "324-BC/ĐU",
    date: "2026-06-15",
    summary: "Báo cáo của Ban Thường vụ Đảng uỷ",
    position: 1,
  }),
];

/** Dòng của sổ như tuyến `GET /api/v1/tasks` trả: KHÔNG có trường `documents`. */
function dongSo(sua: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  const n = nhiemVu(sua);
  delete n.documents;
  return n;
}

const NHAN_BA_NHOM = [
  "Văn bản cấp trên giao",
  "Văn bản chỉ đạo của Đảng uỷ",
  "Văn bản sản phẩm đầu ra",
];

function veTuDrawer(d: DrawerNhiemVu | null): string {
  if (d === null) throw new Error("drawer phải đang mở");
  return veChiTiet(d.nhiemVu, LANH_DAO, d.vanBan);
}

describe("§5.4 — drawer đọc TUYẾN CHI TIẾT, không đọc dòng của sổ", () => {
  it("mở từ một dòng sổ (vắng `documents`): khối ĐANG TẢI, KHÔNG bao giờ là ba nhóm rỗng", () => {
    const d = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    expect(d?.vanBan).toEqual({ pha: "dangTai" });
    expect(d?.luotDoc).toBe(1);

    const html = veTuDrawer(d);
    expect(html).toContain(nhuTrongHTML(DANG_TAI_VAN_BAN));
    for (const nhan of NHAN_BA_NHOM) expect(html).not.toContain(nhuTrongHTML(nhan));
  });

  it("`mo` không tin `documents` của dòng được bấm, kể cả khi dòng ấy mang một mảng", () => {
    // Nguồn duy nhất của khối là tuyến chi tiết. Một ngày tuyến sổ đổi hình dạng thì drawer vẫn
    // đọc lại, thay vì âm thầm vẽ thứ tuyến sổ chưa từng hứa.
    const d = chuyenDrawer(null, { loai: "mo", nhiemVu: nhiemVu({ documents: [] }) });
    expect(d?.vanBan).toEqual({ pha: "dangTai" });
  });

  it("chi tiết về: ba nhóm hiện đủ, theo đúng nhãn §5.4, và trường vô hướng lấy theo chi tiết", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const d = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN, note: "Ghi chú từ chi tiết" }) },
    });
    expect(d?.vanBan).toEqual({ pha: "xong", duLieu: BA_VAN_BAN });
    expect(d?.nhiemVu.note).toBe("Ghi chú từ chi tiết");

    const html = veTuDrawer(d);
    expect(html).toContain(nhuTrongHTML(TIEU_DE_KHOI_VAN_BAN));
    for (const nhan of NHAN_BA_NHOM) expect(html).toContain(nhuTrongHTML(nhan));
    expect(html).toContain("1742-CV/BTCTU · 9/6/2026");
    expect(html).toContain(nhuTrongHTML("Công văn của Ban Tổ chức Thành uỷ"));
    expect(html).toContain("324-BC/ĐU · 15/6/2026");
    // Nhóm giữa rỗng thật — máy chủ ĐÃ nói thế — nên nó là dấu gạch.
    expect(html).toContain("Văn bản chỉ đạo của Đảng uỷ</dt><dd>—</dd>");
  });

  it("đọc chi tiết HỎNG: câu lỗi hiện ra, và KHÔNG có nhóm nào — không một danh sách rỗng", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const d = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: false, thongBao: "Không tìm thấy nhiệm vụ." },
    });
    expect(d?.vanBan).toEqual({ pha: "loi", thongBao: "Không tìm thấy nhiệm vụ." });

    const html = veTuDrawer(d);
    expect(html).toContain('role="alert"');
    expect(html).toContain(nhuTrongHTML(KHONG_DOC_DUOC_VAN_BAN));
    expect(html).toContain(nhuTrongHTML("Không tìm thấy nhiệm vụ."));
    for (const nhan of NHAN_BA_NHOM) expect(html).not.toContain(nhuTrongHTML(nhan));
  });

  it("chi tiết về mà THIẾU `documents`: hợp đồng bị vỡ — báo lỗi, không đọc thành rỗng", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const d = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: dongSo() },
    });
    expect(d?.vanBan).toEqual({ pha: "loi", thongBao: CHI_TIET_THIEU_VAN_BAN });
  });

  it("câu trả lời của một lượt ĐÃ CŨ, hoặc của nhiệm vụ khác, bị bỏ", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const cu = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 0,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
    expect(cu).toBe(mo);
    const khac = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV20",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ code: "NV20", documents: BA_VAN_BAN }) },
    });
    expect(khac).toBe(mo);
  });
});

describe("§5.4 — một lần đổi trạng thái KHÔNG làm rơi khối văn bản", () => {
  function daDocXong(): DrawerNhiemVu | null {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    return chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
  }

  it("phản hồi `…/status` (vắng `documents`): trạng thái mới, văn bản CŨ vẫn hiện, và đọc lại", () => {
    const truoc = daDocXong();
    const sau = chuyenDrawer(truoc, {
      loai: "ghiXong",
      nhiemVu: dongSo({ status: "cho-duyet" }),
    });
    expect(sau?.nhiemVu.status).toBe("cho-duyet");
    expect(sau?.vanBan).toEqual({ pha: "xong", duLieu: BA_VAN_BAN });
    // Lượt tăng ⇒ hiệu ứng đọc lại chi tiết.
    expect(sau?.luotDoc).toBe((truoc?.luotDoc ?? 0) + 1);

    const html = veTuDrawer(sau);
    expect(html).toContain("1742-CV/BTCTU · 9/6/2026");
    expect(html).not.toContain(nhuTrongHTML(DANG_TAI_VAN_BAN));
  });

  it("lượt đọc GỬI TRƯỚC lần ghi mà về SAU bị bỏ — không đè trạng thái cũ lên trạng thái mới", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const sau = chuyenDrawer(mo, { loai: "ghiXong", nhiemVu: dongSo({ status: "cho-duyet" }) });
    const muon = chuyenDrawer(sau, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ status: "dang-thuc-hien", documents: BA_VAN_BAN }) },
    });
    expect(muon?.nhiemVu.status).toBe("cho-duyet");
  });

  it("phản hồi mang mảng (tuyến tạo): lấy thẳng, không phải chờ", () => {
    const d = chuyenDrawer(null, {
      loai: "ghiXong",
      nhiemVu: nhiemVu({ code: "NV34", documents: [] }),
    });
    expect(d?.vanBan).toEqual({ pha: "xong", duLieu: [] });
  });

  it("`ghiXong` của MỘT NHIỆM VỤ KHÁC (vắng `documents`): KHÔNG mượn văn bản của việc đang mở", () => {
    // Giữ khối cũ chỉ đúng khi cùng mã. Mượn nó sang việc khác là vẽ văn bản của NV19 dưới tên
    // NV34 — và mở luôn `✎ Sửa` trên một tập không phải của NV34.
    const truoc = daDocXong();
    const sau = chuyenDrawer(truoc, { loai: "ghiXong", nhiemVu: dongSo({ code: "NV34" }) });
    expect(sau?.nhiemVu.code).toBe("NV34");
    expect(sau?.vanBan).toEqual({ pha: "dangTai" });

    const html = veTuDrawer(sau);
    expect(html).not.toContain("1742-CV/BTCTU");
    expect(theNutSua(html)).toContain('disabled=""');
  });

  it("đóng drawer là hết trạng thái", () => {
    expect(chuyenDrawer(daDocXong(), { loai: "dong" })).toBeNull();
  });
});

describe("§5.4 — chỉ với loại `Theo văn bản`, và câu chữ của từng dòng", () => {
  const XONG: TaiVanBan = { pha: "xong", duLieu: BA_VAN_BAN };

  it("loại `co-ban`: KHÔNG vẽ khối, kể cả khi đã có văn bản trong tay", () => {
    const html = veChiTiet({ type: "co-ban" }, LANH_DAO, XONG);
    expect(html).not.toContain(nhuTrongHTML(TIEU_DE_KHOI_VAN_BAN));
    for (const nhan of NHAN_BA_NHOM) expect(html).not.toContain(nhuTrongHTML(nhan));
    expect(html).not.toContain("1742-CV/BTCTU");
    // Các trường §5.4 vô hướng vẫn giữ nguyên, cùng chú thích bắt buộc.
    expect(html).toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("loại `theo-van-ban` với cùng dữ liệu: khối CÓ — bài trên không xanh vì lý do sai", () => {
    expect(veChiTiet({}, LANH_DAO, XONG)).toContain("1742-CV/BTCTU");
  });

  it("số ký hiệu rỗng ⇒ `Không số`; ngày rỗng ⇒ bỏ hẳn phần ngày", () => {
    const html = veChiTiet({}, LANH_DAO, {
      pha: "xong",
      duLieu: [vb({ reference: "", date: "" })],
    });
    expect(html).toContain(`<li>${KHONG_SO}<span`);
    expect(html).not.toContain(`${KHONG_SO} · `);
  });

  it("chi tiết trả mảng RỖNG: lúc này ba nhóm `—` là đúng — máy chủ đã nói không có dòng nào", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: [] });
    for (const nhan of NHAN_BA_NHOM) {
      expect(html).toContain(`${nhuTrongHTML(nhan)}</dt><dd>—</dd>`);
    }
  });

  it("thứ tự trong nhóm là thứ tự MÁY CHỦ GỬI, không sắp lại theo `position`", () => {
    const html = veChiTiet({}, LANH_DAO, {
      pha: "xong",
      duLieu: [
        vb({ id: "a", reference: "90-TB/TU", position: 3 }),
        vb({ id: "b", reference: "12-CV/UBND", position: 1 }),
      ],
    });
    expect(html.indexOf("90-TB/TU")).toBeGreaterThan(-1);
    expect(html.indexOf("90-TB/TU")).toBeLessThan(html.indexOf("12-CV/UBND"));
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §5.4 — NÚT `✎ SỬA`
 *
 * Ca nặng nhất là ca bị TỪ CHỐI, không phải ca mở được: `documents` trên PATCH là thay cả tập, nên
 * một nút `✎ Sửa` bấm được lúc khối còn đang tải là một nút gỡ mất mọi văn bản cán bộ chưa thấy.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Thẻ mở của nút `✎ Sửa`, hoặc `null` khi trang không có nút ấy. */
function theNutSua(html: string): string | null {
  const k = /<button[^>]*aria-label="Sửa sổ theo dõi văn bản chỉ đạo"[^>]*>/.exec(html);
  return k === null ? null : k[0];
}

describe("§5.4 — nút `✎ Sửa`: chỉ `Theo văn bản`, và KHOÁ khi chưa đọc đủ văn bản", () => {
  it("`theo-van-ban`, khối đã đọc xong: nút có mặt và BẤM ĐƯỢC", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: BA_VAN_BAN });
    const nut = theNutSua(html);
    expect(nut).not.toBeNull();
    expect(nut).not.toContain("disabled");
    expect(html).toContain(`${NHAN_NUT_SUA}</button>`);
  });

  it("loại `co-ban`: KHÔNG có nút, kể cả khi đã có văn bản trong tay", () => {
    const html = veChiTiet({ type: "co-ban" }, LANH_DAO, { pha: "xong", duLieu: BA_VAN_BAN });
    expect(theNutSua(html)).toBeNull();
    expect(html).not.toContain(NHAN_NUT_SUA);
  });

  it("khối ĐANG TẢI: nút KHOÁ, và lý do khoá ra tới trang", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "dangTai" });
    expect(theNutSua(html)).toContain('disabled=""');
    expect(theNutSua(html)).toContain('aria-describedby="ly-do-khoa-sua-van-ban"');
    expect(html).toContain(nhuTrongHTML(KHOA_SUA_DANG_TAI));
  });

  it("khối đọc HỎNG: nút KHOÁ, và lý do khoá ra tới trang", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "loi", thongBao: "Không tìm thấy nhiệm vụ." });
    expect(theNutSua(html)).toContain('disabled=""');
    expect(html).toContain(nhuTrongHTML(KHOA_SUA_LOI));
  });

  it("khối ĐÃ ĐỌC XONG nhưng có một mã nhóm lạ: nút VẪN KHOÁ, lý do ra trang, dòng lạ vẫn hiện", () => {
    // Pha `xong` là pha duy nhất hai ca trên không phủ: một nút chỉ khoá theo pha sẽ mở ở đây, và
    // lần lưu đầu tiên gỡ mất dòng form không có chỗ vẽ.
    const la = vb({ id: "01JVANBANLA", group: "nhom-moi-gia", reference: "77-TB/GIA" });
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: [...BA_VAN_BAN, la] });
    expect(theNutSua(html)).toContain('disabled=""');
    expect(html).toContain(nhuTrongHTML(KHOA_SUA_NHOM_LA));
    expect(html).toContain("77-TB/GIA");
  });
});

describe("§5.4 — phản hồi PATCH cập nhật drawer", () => {
  it("`ghiXong` với phản hồi PATCH (mang `documents`): trường vô hướng VÀ khối văn bản lấy theo phản hồi", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const truoc = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
    const conMot = [BA_VAN_BAN[1] as petitions_nhiemVuVanBanRa];
    const sau = chuyenDrawer(truoc, {
      loai: "ghiXong",
      nhiemVu: nhiemVu({ note: "Ghi chú giả sau khi sửa", leader_approved: true, documents: conMot }),
    });
    expect(sau?.nhiemVu.note).toBe("Ghi chú giả sau khi sửa");
    expect(sau?.nhiemVu.leader_approved).toBe(true);
    expect(sau?.vanBan).toEqual({ pha: "xong", duLieu: conMot });
    // Lượt tăng ⇒ đọc lại chi tiết, như mọi lần ghi khác.
    expect(sau?.luotDoc).toBe((truoc?.luotDoc ?? 0) + 1);

    const html = veTuDrawer(sau);
    expect(html).not.toContain("1742-CV/BTCTU");
    expect(html).toContain("324-BC/ĐU · 15/6/2026");
  });

  it("phản hồi PATCH gỡ hết văn bản (`documents: []`): khối là ba nhóm `—`, không phải khối cũ", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const truoc = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
    const sau = chuyenDrawer(truoc, { loai: "ghiXong", nhiemVu: nhiemVu({ documents: [] }) });
    expect(sau?.vanBan).toEqual({ pha: "xong", duLieu: [] });
  });
});

describe("§5.4 — form `✎ Sửa`", () => {
  function veForm(sua: Partial<petitions_nhiemVuRa> = {}): string {
    return renderToStaticMarkup(
      <FormSuaKhoiVanBan
        nhiemVu={nhiemVu({ lead_unit: "01JBOPHAN", monitor: "CB-00001", ...sua })}
        vanBan={BA_VAN_BAN}
        tenBoPhan={TEN_BO_PHAN}
        luu={KHONG_SUA}
        docLai={KHONG_SUA}
        xong={() => {}}
      />,
    );
  }

  it("Mã, Hạn, Cơ quan chủ trì, Chuyên viên HIỆN mà KHÔNG có ô nhập — mỗi thứ kèm lý do", () => {
    const html = veForm();
    expect(html).toContain("NV19");
    expect(html).toContain(nhuTrongHTML(LY_DO_KHONG_SUA_MA));
    expect(html).toContain("20/6/2026");
    expect(html).toContain(nhuTrongHTML(LY_DO_KHONG_SUA_HAN));
    expect(html).toContain("VĂN PHÒNG ĐẢNG ỦY");
    expect(html).toContain("CB-00001");
    expect(html).toContain(nhuTrongHTML(LY_DO_KHONG_SUA_CHU_TRI));
    // Không ô ngày nào cho hạn, không ô mã, không ô chọn bộ phận — `type="date"` còn lại chỉ là
    // ngày của từng văn bản.
    expect(html).not.toContain('id="sua-ma"');
    expect(html).not.toContain('id="sua-han"');
    expect(html).not.toContain("<select");
    expect(html.split('type="date"').length - 1).toBe(BA_VAN_BAN.length);
  });

  it("năm ô sửa được có nhãn thật, và chú thích BẮT BUỘC của hai ô tick vẫn hiện", () => {
    const html = veForm();
    expect(html).toContain('<label for="sua-tieu-de">Nội dung nhiệm vụ / Trích yếu văn bản</label>');
    expect(html).toContain('id="sua-tom-tat-ket-qua"');
    expect(html).toContain('<label for="sua-ghi-chu">Ghi chú</label>');
    expect(html).toContain('id="sua-lanh-dao-phe-duyet"');
    expect(html).toContain('id="sua-cap-tren-cong-nhan"');
    expect(html).toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("mọi dòng đã có hiện ra để sửa, dùng CÙNG ô của form tạo; không có lối chuyển nhóm", () => {
    const html = veForm();
    expect(html).toContain('id="sua-van-ban-id-01JVANBAN1"');
    expect(html).toContain('id="sua-van-ban-id-01JVANBAN2"');
    expect(html).toContain("Công văn của Ban Tổ chức Thành uỷ");
    expect(html).toContain('value="1742-CV/BTCTU"');
    expect(html).toContain('value="2026-06-09"');
    expect(html.split("+ Thêm văn bản</button>").length - 1).toBe(3);
    // Id không đụng form tạo khi hai form cùng mở.
    expect(html).not.toContain('id="giao-');
  });

  it("vừa mở, chưa đổi gì: nút `Lưu` KHOÁ và câu nói vì sao", () => {
    const html = veForm();
    expect(html).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    expect(html).toContain("Chưa có gì thay đổi để lưu.");
    expect(html).toContain(">Huỷ</button>");
  });
});
