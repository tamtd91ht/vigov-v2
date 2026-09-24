import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NAM_DANH_MUC_GHI, type MoTaDanhMucGhi, type MucDanhMucGhi } from "@/lib/api/danh-muc";
import type { MucDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";

import {
  GHI_CHU_NHOM_CHI_XEM,
  NUT_BAT_LAI,
  NUT_SUA,
  NUT_TAT,
  NUT_THEM,
  NUT_XOA,
  O_LY_DO_XOA,
  giaiThichKhongThaoTac,
  nhanNhomRong,
  nhanSoMuc,
} from "@/features/cau-hinh/nhan-danh-muc";
import type { NhomDanhMuc } from "@/features/cau-hinh/nhom-danh-muc";
import { BieuMauGhi, NhomMuc, type ThaoTacNhom } from "@/features/cau-hinh/tab-danh-muc";
import { TANG_DON_VI, TANG_HE_THONG, TANG_RE_NHANH } from "@/features/cau-hinh/tang-danh-muc";

/**
 * VÌ SAO TỆP NÀY TỒN TẠI, và nó không phải chuyện phủ thêm cho đủ.
 *
 * Mọi ca test khác của màn Danh mục canh một QUYẾT ĐỊNH trong module thuần: nhóm nào rỗng, câu
 * nào hiện ra, tầng nào cho phép thao tác gì. Không ca nào canh việc quyết định ấy CÓ RA TỚI
 * TRANG hay không.
 *
 * Đã đo, không phải lo xa: bôi trắng `{nhanNhomRong(...)}` trong `tab-danh-muc.tsx` — câu quan
 * trọng nhất trên màn hình — từng làm cả bộ test vẫn xanh và `tsc` vẫn sạch. Nay điều tương tự
 * đúng với NÚT: `thaoTacTheoTang` có thể trả về đúng, mà JSX vẫn vẽ ra một nút `Xoá` ở dòng của
 * hệ thống. Ba ca "tầng ... vẽ những nút nào" dưới đây canh đúng chỗ ấy.
 *
 * KHÔNG THÊM PHỤ THUỘC NÀO. `react-dom` đã nằm trong `dependencies`; `renderToStaticMarkup` chạy
 * trong Node thuần và trả về một chuỗi. Những thứ THẬT SỰ cần trình duyệt (sự kiện, tiêu điểm,
 * bố cục) vẫn nằm ngoài phạm vi — phép kiểm của chúng là `kiemLyDoXoa` và `thaoTacTheoTang`.
 */

const KHONG_LAM_GI: ThaoTacNhom = {
  them: () => {},
  sua: () => {},
  xoa: () => {},
  datTrangThai: () => {},
};

function duongGhiLoaiVanBan(): MoTaDanhMucGhi {
  const mo = NAM_DANH_MUC_GHI.find((m) => m.khoa === "loaiVanBan");
  if (mo === undefined) throw new Error("hợp đồng không còn tuyến ghi cho Loại văn bản");
  return mo;
}

function muc(tier: number, active = true, label = "Công văn"): MucDanhMucGhi {
  return {
    id: `01JH-${tier}-${String(active)}`,
    code: "cong-van",
    label,
    active,
    is_default: false,
    order: 7,
    source: tier === TANG_DON_VI ? "don-vi" : "he-thong",
    tier,
  };
}

function nhomRong(ghi: MoTaDanhMucGhi | null = duongGhiLoaiVanBan()): NhomDanhMuc {
  return {
    khoa: "loaiVanBan",
    nhan: "Loại văn bản",
    thuTuLaThangBac: false,
    ghi,
    trangThai: { pha: "chuaCoMuc" },
  };
}

function nhomCoMuc(
  ds: readonly MucDanhMuc[],
  ghi: MoTaDanhMucGhi | null = duongGhiLoaiVanBan(),
): NhomDanhMuc {
  return {
    khoa: "loaiVanBan",
    nhan: "Loại văn bản",
    thuTuLaThangBac: false,
    ghi,
    trangThai: { pha: "coMuc", muc: ds },
  };
}

function ve(nhom: NhomDanhMuc, coQuyenGhi = true): string {
  return renderToStaticMarkup(
    <NhomMuc nhom={nhom} coQuyenGhi={coQuyenGhi} thaoTac={KHONG_LAM_GI} form={null} />,
  );
}

describe("NhomMuc kết xuất ra trang", () => {
  it("nhóm rỗng: câu báo rỗng PHẢI có mặt trong markup, nguyên văn", () => {
    // Nguyên văn, không phải một mảnh: một câu bị cắt còn tệ hơn câu vắng mặt, vì nó vẫn đọc
    // trôi chảy mà thiếu đúng nửa nói phải làm gì tiếp theo.
    expect(ve(nhomRong())).toContain(nhanNhomRong("Loại văn bản", "themDuoc"));
  });

  it("nhóm rỗng KHÔNG được dựng thành ô trống", () => {
    const html = ve(nhomRong());

    // Thứ đang canh là "có chữ cho người đọc", không phải "có thẻ p". Một <p></p> rỗng qua được
    // phép kiểm thẻ và trượt đúng thứ ca test này sinh ra để bắt.
    expect(html).toContain('class="trang-thai-rong"');
    expect(html).not.toContain('class="trang-thai-rong"></p>');
  });

  it("nhóm rỗng KHÔNG được dựng như một lỗi", () => {
    const html = ve(nhomRong());

    // `role="alert"` cắt ngang người dùng trình đọc màn hình. Một danh mục chưa có mục nào là
    // trạng thái BÌNH THƯỜNG của nhiều đơn vị hôm nay — không có gì để báo động.
    expect(html).not.toContain('role="alert"');
    expect(html).not.toContain("thong-bao-loi");
  });

  it("nhóm rỗng nói BA câu khác nhau theo ba lý do khác nhau", () => {
    // Gộp "chưa có tuyến ghi" với "thiếu quyền" là nói sai với một trong hai người đọc: một người
    // cần đi xin quyền, người kia xin quyền cũng không có gì mở ra.
    expect(ve(nhomRong(), true)).toContain(nhanNhomRong("Loại văn bản", "themDuoc"));
    expect(ve(nhomRong(), false)).toContain(nhanNhomRong("Loại văn bản", "thieuQuyen"));
    expect(ve(nhomRong(null))).toContain(nhanNhomRong("Loại văn bản", "khongCoTuyen"));
  });

  it("nhóm có mục: hiện số đếm và mã, và KHÔNG hiện câu báo rỗng", () => {
    const html = ve(nhomCoMuc([muc(TANG_DON_VI), muc(TANG_HE_THONG, false, "Tờ trình")]));

    expect(html).toContain(nhanSoMuc(2));
    expect(html).toContain("cong-van");
    expect(html).toContain("Tờ trình");
    expect(html).not.toContain(nhanNhomRong("Loại văn bản", "themDuoc"));
  });

  it("nhóm đọc không được: hiện ĐÚNG thông báo của máy chủ, và không hiện câu báo rỗng", () => {
    const thongBao = "Đã xảy ra lỗi. Vui lòng thử lại.";
    const html = ve({ ...nhomRong(), trangThai: { pha: "khongDocDuoc", thongBao } });

    expect(html).toContain(thongBao);
    // Hai trạng thái này phải phân biệt được trên màn hình: "chưa có gì" và "không đọc được" dẫn
    // người dùng đi hai đường khác hẳn nhau — một bên chờ onboard, một bên gọi hỗ trợ.
    expect(html).not.toContain(nhanNhomRong("Loại văn bản", "themDuoc"));
    expect(html).toContain('role="alert"');
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BA TẦNG HIỆN RA THÀNH NÚT. Ba ca dưới đây là lý do chính tệp này tồn tại.
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */

describe("nút của một dòng, theo tầng", () => {
  it("tầng 1 — đơn vị tự thêm: đủ Sửa, Tắt, Xoá", () => {
    const html = ve(nhomCoMuc([muc(TANG_DON_VI)]));

    expect(html).toContain(NUT_SUA);
    expect(html).toContain(NUT_TAT);
    expect(html).toContain(NUT_XOA);
  });

  it("tầng 2 — phần mềm cấp: CÓ Tắt, KHÔNG có Xoá", () => {
    const html = ve(nhomCoMuc([muc(TANG_HE_THONG)]));

    expect(html).toContain(NUT_SUA);
    expect(html).toContain(NUT_TAT);
    expect(html).not.toContain(NUT_XOA);
    // Và nói ra vì sao thiếu nút, ngay trong ô hành động của chính dòng ấy.
    expect(html).toContain(giaiThichKhongThaoTac(TANG_HE_THONG));
  });

  it("tầng 3 — mã nguồn rẽ nhánh theo mã: KHÔNG Tắt, KHÔNG Xoá, chỉ còn Sửa", () => {
    // ĐÂY LÀ CA ĐẮT NHẤT CỦA MÀN HÌNH NÀY. Vẽ ra một nút `Xoá` hay `Tắt` ở đây là mời cán bộ bấm
    // vào một 409 — và người bấm sẽ kết luận phần mềm hỏng, chứ không kết luận quy tắc ba tầng
    // đang làm đúng việc của nó. Cái CHẶN thật là trigger CSDL; ca này canh cái VẼ.
    const html = ve(nhomCoMuc([muc(TANG_RE_NHANH)]));

    expect(html).toContain(NUT_SUA);
    expect(html).not.toContain(NUT_TAT);
    expect(html).not.toContain(NUT_XOA);
    expect(html).toContain(giaiThichKhongThaoTac(TANG_RE_NHANH));
  });

  it("mục đã tắt có nút Bật lại ở MỌI tầng, kể cả tầng 3", () => {
    // Trigger chỉ từ chối chiều bật → tắt. Giấu nốt nút bật thì một dòng tầng 3 lỡ tắt không còn
    // đường nào quay lại (`tang-danh-muc.ts`, `choBatLai`).
    const html = ve(nhomCoMuc([muc(TANG_RE_NHANH, false)]));

    expect(html).toContain(NUT_BAT_LAI);
    expect(html).not.toContain(NUT_XOA);
  });

  it("KHÔNG CÓ QUYỀN `admin.lookup`: không một nút ghi nào được vẽ — kể cả ở dòng tầng 1", () => {
    // CA BỊ TỪ CHỐI, không phải ca được phép. Ẩn nút chỉ là tiện dụng — máy chủ vẫn kiểm
    // `RequirePermission` trên từng yêu cầu (luật 5, cấm #1) — nhưng nếu lớp tiện dụng này sai
    // thì cán bộ không có quyền sẽ bấm và nhận 403 ở mọi dòng.
    const html = ve(nhomCoMuc([muc(TANG_DON_VI)]), false);

    expect(html).not.toContain(NUT_THEM);
    expect(html).not.toContain(NUT_SUA);
    expect(html).not.toContain(NUT_TAT);
    expect(html).not.toContain(NUT_XOA);
    // Bảng thì VẪN HIỆN: tuyến đọc khai `any-authenticated`, nên ẩn cả bảng là giao diện từ chối
    // điều máy chủ đang phục vụ bình thường.
    expect(html).toContain("cong-van");
  });
});

describe("cột Nguồn và cột Thứ tự", () => {
  it("hiện `Hệ thống` / `Đơn vị` bằng chữ người đọc được", () => {
    const html = ve(nhomCoMuc([muc(TANG_DON_VI), muc(TANG_HE_THONG)]));

    expect(html).toContain("Nguồn");
    expect(html).toContain("Đơn vị");
    expect(html).toContain("Hệ thống");
  });

  it("Thứ tự hiện `order` CỦA HỢP ĐỒNG, không phải vị trí trong mảng", () => {
    // Biểu mẫu sửa đổi chính con số này. Một cột hiện vị trí trong mảng sẽ nói "1" ngay sau khi
    // cán bộ vừa đặt thứ tự 7 — và họ sẽ đặt lại lần nữa.
    const html = ve(nhomCoMuc([muc(TANG_DON_VI)]));

    expect(html).toContain("<td>7</td>");
  });

  it("nhóm KHÔNG có đường ghi: không cột Nguồn, không cột hành động, và nói rõ là chỉ xem", () => {
    // Read-only is decided by the group having NO write descriptor (`null` below), not by the row
    // shape: since 7aa0127 the identity catalogues emit `order`/`source`/`tier` like every other
    // group, so this row is write-shaped and the guarantee must hold anyway. This is the one place
    // the read-only guarantee is asserted (`tang-danh-muc.test.ts` points here).
    const chiDoc: MucDanhMuc = {
      id: "01JH-x",
      code: "thon",
      label: "Thôn",
      active: true,
      is_default: false,
      order: 1,
      source: "don-vi",
      tier: 1,
    };
    const html = ve(nhomCoMuc([chiDoc], null));

    expect(html).toContain(GHI_CHU_NHOM_CHI_XEM);
    expect(html).not.toContain("Nguồn");
    expect(html).not.toContain(NUT_SUA);
    expect(html).not.toContain(NUT_THEM);
  });
});

describe("biểu mẫu xoá", () => {
  const ghi = duongGhiLoaiVanBan();
  const banTrong = { ma: "", nhan: "", thuTu: "", lyDo: "", macDinh: false };

  function veForm(loiTaiCho: string) {
    return renderToStaticMarkup(
      <BieuMauGhi
        dangMo={{ kieu: "xoa", ghi, nhanNhom: "Loại văn bản", muc: muc(TANG_DON_VI) }}
        ban={banTrong}
        datBan={() => {}}
        loiTaiCho={loiTaiCho}
        loiMayChu=""
        dangGui={false}
        onGui={() => {}}
        onHuy={() => {}}
      />,
    );
  }

  it("có ô lý do, và ô ấy bắt buộc", () => {
    const html = veForm("");

    expect(html).toContain(O_LY_DO_XOA);
    expect(html).toContain("required");
  });

  it("lý do rỗng: câu từ chối hiện ra, có `role=alert` và ô được đánh dấu sai", () => {
    // Phép quyết định nằm ở `kiemLyDoXoa` (có bài test riêng). Ca này canh việc quyết định ấy RA
    // TỚI TRANG: một câu lỗi tính đúng mà không dựng ra là một biểu mẫu im lặng không gửi gì.
    const html = veForm("Vui lòng nhập lý do xoá.");

    expect(html).toContain("Vui lòng nhập lý do xoá.");
    expect(html).toContain('role="alert"');
    expect(html).toContain('aria-invalid="true"');
  });

  it("nói trước rằng mã vẫn bị giữ chỗ — 'xoá' ở đây không phải xoá", () => {
    expect(veForm("")).toContain("không dùng lại được");
  });

  it("KHÔNG có ô nhập mã: mã đã cấp thì không đổi được", () => {
    // Một ô `Mã` trong biểu mẫu sửa/xoá là một ô hứa điều máy chủ sẽ từ chối bằng 400.
    expect(veForm("")).not.toContain('id="o-ma-muc"');
  });
});
