import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { danhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type {
  identity_canBoChonNguoiRa,
  petitions_bienBanRa,
  petitions_ketLuanRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";

import {
  CANH_BAO_BI_MAT,
  CAU_XAC_NHAN_KY,
  cauDaTach,
  CHUA_TACH_NHIEM_VU,
  NHAN_NUT_BO_DAU,
  NHAN_NUT_BO_SUNG,
  NHAN_NUT_DANH_DAU,
  NHAN_NUT_GHI_THONG_BAO,
  NHAN_NUT_KY,
  NHAN_NUT_LUU_SUA,
  NHAN_NUT_SUA_BIEN_BAN,
  NHAN_NUT_XAC_NHAN_KY,
  NHAN_NUT_XOA_BIEN_BAN,
  VI_SAO_BIEN_BAN_CON_NHIEM_VU,
  VI_SAO_KET_LUAN_KHOA,
  NGUON_GIAO_KHOA,
  NHAN_NUT_LUU,
  NHAN_NUT_TACH,
  NHAN_NUT_THEM_KET_LUAN,
  PHAN_CHUA_DUNG,
  PLACEHOLDER_KET_LUAN,
  SO_RONG,
} from "./nhan-bien-ban";
import {
  ChiTietBienBan,
  DanhSachBienBan,
  DongKetLuan,
  FormNhapBienBan,
  HangThemKetLuan,
  KhoiChuaDung,
  TheBienBan,
  type CheBieuMau,
  type PhepTach,
  type PhepVongDoi,
} from "./so-bien-ban";
import { BANG_NHAN_MAC_DINH } from "@/features/nhiem-vu/nhan-nhiem-vu";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **SỐ THỨ TỰ KẾT LUẬN**, và nó là nhóm không ai nhìn
 * thấy trong lúc phát triển: dữ liệu mẫu bao giờ cũng liên tục 1-2-3, nên `{i + 1}` và
 * `{kl.ordinal}` cho ra cùng một trang. Điều phải đúng là chuyện chỉ xảy ra sau một lần xoá mềm —
 * biên bản có các số ①③④, và màn hình phải vẽ đúng ba con số ấy, kèm một khoảng trống ở ②.
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

function ketLuan(sua: Partial<petitions_ketLuanRa> = {}): petitions_ketLuanRa {
  return {
    id: "01JKL1",
    ordinal: 1,
    content: "Giao bộ phận Địa chính rà soát tiến độ tuyến đường Hà Lam – Bình Trị.",
    task_count: 0,
    task_done_count: 0,
    status: "chua-giao",
    no_task: false,
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

function bienBan(sua: Partial<petitions_bienBanRa> = {}): petitions_bienBanRa {
  return {
    id: "01JBB1",
    title: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
    held_on: "2026-08-05",
    reference_no: "31/BB-UBND",
    location: "Phòng họp UBND xã",
    chaired_by: "",
    conclusions: [ketLuan()],
    task_count: 3,
    task_done_count: 1,
    status: "du-thao",
    conclusion_count: 1,
    conclusion_done_count: 0,
    created_by: "CB-2026-7K3M9Q",
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

/**
 * Luồng Tách ở trạng thái nghỉ: không hộp nào mở, không lỗi, không câu báo xong.
 *
 * Danh mục để RỖNG cố ý — nó không phải thứ nhóm nào dưới đây canh, và một danh mục giả đầy đủ chỉ
 * làm các ca khó đọc hơn.
 */
function phepTach(sua: Partial<PhepTach> = {}): PhepTach {
  return {
    danhMuc: { loai: [], mucUuTien: [], khoi: [], boPhan: [] },
    dangGui: false,
    moOKetLuan: null,
    loi: null,
    daXong: null,
    mo: () => {},
    dong: () => {},
    gui: () => {},
    ...sua,
  };
}

/**
 * Vòng đời ở trạng thái nghỉ: KHÔNG có quyền ký, không hộp nào mở, không lỗi, danh bạ chưa đọc.
 * Mặc định KHÔNG quyền ký cố ý — ca "thấy nút Ký" phải tự khai quyền, nên không ca nào xanh nhờ
 * một mặc định rộng tay.
 */
function vongDoi(sua: Partial<PhepVongDoi> = {}): PhepVongDoi {
  return {
    dangGui: false,
    coQuyenKy: false,
    danhBa: null,
    nhanTT: BANG_NHAN_MAC_DINH,
    hop: null,
    loi: null,
    chiTiet: null,
    nhiemVuKL: new Map(),
    moHop: () => {},
    dongHop: () => {},
    dongChiTiet: () => {},
    moSua: () => {},
    moBoSung: () => {},
    ky: () => {},
    ghiThongBao: () => {},
    xoa: () => {},
    suaKL: () => {},
    goKL: () => {},
    danhDau: () => {},
    boDau: () => {},
    batNhiemVuKL: () => {},
    ...sua,
  };
}

function veThe(
  bb: petitions_bienBanRa,
  tach: PhepTach = phepTach(),
  vd: PhepVongDoi = vongDoi(),
): string {
  return renderToStaticMarkup(
    <TheBienBan
      bienBan={bb}
      lanGhiXong={0}
      dangGui={false}
      loiKetLuan={null}
      guiKetLuan={() => {}}
      tach={tach}
      vongDoi={vd}
    />,
  );
}

function veDong(
  kl: petitions_ketLuanRa,
  tach: PhepTach = phepTach(),
  vd: PhepVongDoi = vongDoi(),
  bb: petitions_bienBanRa = bienBan(),
): string {
  return renderToStaticMarkup(
    <DongKetLuan bienBan={bb} ketLuan={kl} tach={tach} vongDoi={vd} />,
  );
}

describe("thẻ biên bản §2", () => {
  it("header mang tiêu đề, dòng meta và badge của máy chủ", () => {
    const html = veThe(bienBan());

    expect(html).toContain("Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026");
    expect(html).toContain("5/8/2026 · 31/BB-UBND · Phòng họp UBND xã");
    // CON SỐ CHÍNH là kết luận hoàn thành, số nhiệm vụ là phụ — cả hai đọc từ máy chủ.
    expect(html).toContain("<strong>0/1 kết luận hoàn thành</strong>");
    expect(html).toContain("1/3 nhiệm vụ xong");
  });

  it("thiếu số hiệu và địa điểm thì dòng meta chỉ còn ngày — không có dấu chấm bơ vơ", () => {
    const html = veThe(bienBan({ reference_no: "", location: "" }));

    expect(html).toContain("5/8/2026");
    expect(html).not.toContain("· ·");
  });

  it("biên bản chưa có kết luận nào VẪN hiện, và nói ra trạng thái ấy (§7.3)", () => {
    const html = veThe(
      bienBan({ conclusions: [], task_count: 0, task_done_count: 0, conclusion_count: 0 }),
    );

    expect(html).toContain("Biên bản này chưa ghi kết luận nào.");
    expect(html).toContain("0/0 kết luận hoàn thành");
    expect(html).toContain("0/0 nhiệm vụ xong");
    // Hàng thêm kết luận vẫn có: "nhập nháp trước, bổ sung sau" là cả điểm của trạng thái này.
    expect(html).toContain(PLACEHOLDER_KET_LUAN);
  });

  it("hàng thêm kết luận luôn ở cuối thẻ", () => {
    expect(veThe(bienBan())).toContain(nhuTrongHTML(NHAN_NUT_THEM_KET_LUAN));
  });

  it("câu 403 của máy chủ ra thẳng màn hình, nguyên văn", () => {
    const html = renderToStaticMarkup(
      <TheBienBan
        bienBan={bienBan()}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan="Bạn không có quyền tạo nhiệm vụ."
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );

    expect(html).toContain("Bạn không có quyền tạo nhiệm vụ.");
    expect(html).toContain('role="alert"');
  });
});

describe("SỐ THỨ TỰ KẾT LUẬN — nối tiếp số đã cấp, không đếm lại theo vị trí", () => {
  const CO_KHOANG_TRONG = bienBan({
    conclusions: [
      ketLuan({ id: "k1", ordinal: 1, content: "Kết luận thứ nhất." }),
      // ② đã bị gỡ. Số 2 KHÔNG BAO GIỜ được cấp lại (luật 7, bất biến 3).
      ketLuan({ id: "k3", ordinal: 3, content: "Kết luận thứ ba." }),
      ketLuan({ id: "k4", ordinal: 4, content: "Kết luận thứ tư." }),
    ],
  });

  it("vẽ ①③④ đúng như máy chủ trả, KHÔNG vẽ ①②③", () => {
    const html = veThe(CO_KHOANG_TRONG);

    // Số nằm trong một `<span class="chip">`, nên so cả thẻ bao quanh: một phép `toContain("3")`
    // trần sẽ xanh nhờ bất kỳ con số nào khác trên thẻ — kể cả `1/3 nhiệm vụ xong`.
    expect(html).toContain('<span class="chip">1</span>');
    expect(html).toContain('<span class="chip">3</span>');
    expect(html).toContain('<span class="chip">4</span>');
    expect(html).not.toContain('<span class="chip">2</span>');
  });

  it("nội dung đi CÙNG đúng con số của nó, không lệch một dòng", () => {
    const html = veDong(ketLuan({ ordinal: 4 }));

    expect(html).toContain('<span class="chip">4</span>');
    expect(html).not.toContain('<span class="chip">1</span>');
  });

  it("dòng phụ nói `Chưa tách thành nhiệm vụ nào` khi chưa có nhiệm vụ nào", () => {
    expect(veDong(ketLuan({ task_count: 0 }))).toContain(CHUA_TACH_NHIEM_VU);
  });

  it("có nhiệm vụ rồi thì là hai con số, không còn câu `Chưa tách…`", () => {
    const html = veDong(ketLuan({ task_count: 1, task_done_count: 1 }));

    expect(html).toContain("1/1 nhiệm vụ đã hoàn thành");
    expect(html).not.toContain(CHUA_TACH_NHIEM_VU);
  });

  /* ⚠ CA QUAN TRỌNG NHẤT CỦA CẢ TỆP. Con số trong tên đọc được của nút và con số đi trên đường dẫn
   * `{stt}` cùng đọc `ketLuan.ordinal`; lời gọi thì không kiểm được bằng `renderToStaticMarkup`,
   * nên tên nút là chỗ DUY NHẤT con số ấy quan sát được ở đây. Vẽ sai nó là dấu hiệu gửi sai nó. */
  it("TÊN ĐỌC ĐƯỢC CỦA NÚT TÁCH mang số ĐÃ CẤP, không mang vị trí trong mảng", () => {
    const html = veThe(CO_KHOANG_TRONG);

    expect(html).toContain('aria-label="Tách thành nhiệm vụ — kết luận số 1"');
    expect(html).toContain('aria-label="Tách thành nhiệm vụ — kết luận số 3"');
    expect(html).toContain('aria-label="Tách thành nhiệm vụ — kết luận số 4"');
    // Kết luận ở VỊ TRÍ THỨ HAI mang số 3. Một nút mang "số 2" nghĩa là màn đang đếm lại theo vị
    // trí — và con số ấy đi thẳng lên đường dẫn, tách nhầm kết luận.
    expect(html).not.toContain('aria-label="Tách thành nhiệm vụ — kết luận số 2"');
  });
});

/**
 * §3 — luồng Tách.
 *
 * NHÓM NÀY THAY CHO NHÓM CŨ "chỗ ấy là một dòng chữ, KHÔNG phải một nút". Ca ấy đúng cho tới
 * 24/09/2026: biểu mẫu "Giao việc mới" chưa có nên chỗ này là một dòng chữ nói rõ màn chưa dựng.
 * Biểu mẫu nay đã có và được dùng lại nguyên bản, nên hành vi đổi và ca kiểm đổi theo — KHÔNG gỡ
 * đi: mỗi điều ca cũ canh đều có một ca mới canh điều tương ứng ở hành vi mới.
 */
describe("nút `✂ Tách thành nhiệm vụ` §3", () => {
  it("chỗ ấy NAY LÀ MỘT NÚT THẬT, mang đúng nhãn đặc tả vẽ", () => {
    const html = veDong(ketLuan());

    expect(html).toContain("<button");
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_TACH));
  });

  it("hộp Giao việc chỉ hiện khi chính kết luận NÀY đang mở", () => {
    const dong = ketLuan({ id: "k7" });

    // Chưa mở: không có biểu mẫu nào trên dòng.
    expect(veDong(dong)).not.toContain("Giao việc mới");
    // Mở ở một kết luận KHÁC: dòng này vẫn không có biểu mẫu. Một hộp một lúc trên cả màn.
    expect(veDong(dong, phepTach({ moOKetLuan: "k9" }))).not.toContain("Giao việc mới");
    // Mở ở chính nó.
    expect(veDong(dong, phepTach({ moOKetLuan: "k7" }))).toContain("Giao việc mới");
  });

  it("hộp mở thì dùng LẠI biểu mẫu Giao việc của `02-nhiem-vu.md` §7, không dựng bản thứ hai", () => {
    const html = veDong(ketLuan({ id: "k7" }), phepTach({ moOKetLuan: "k7" }));

    // Ba ô chỉ có ở biểu mẫu ấy. Một bản chép tay ở `features/bien-ban/` sẽ trôi khỏi bản gốc, và
    // ngày một bên thêm một trường thì bên kia vẫn xanh (luật 9, cấm #2).
    expect(html).toContain("giao-tieu-de");
    expect(html).toContain("giao-tu-sinh-ma");
    expect(html).toContain("giao-han");
  });

  it("hộp mở thì kết luận gốc Ở LẠI dù ô đã điền sẵn, và nguồn giao là câu KHOÁ", () => {
    const html = veDong(
      ketLuan({ id: "k7", ordinal: 3, content: "Giao Tài chính đối chiếu số liệu." }),
      phepTach({ moOKetLuan: "k7" }),
    );

    // KẾT LUẬN GỐC PHẢI CÒN, và lý do đã ĐỔI kể từ 24/09/2026. Trước: ô "Nội dung nhiệm vụ" chưa
    // điền sẵn được nên đây là chỗ chép từ. Nay `tieuDeCoSan` đã điền sẵn — nhưng ô ấy là thứ cán
    // bộ SẼ SỬA thành một câu giao việc đọc được, nên câu gốc phải còn để đối chiếu. Xoá nó đi thì
    // sau lần sửa đầu tiên không còn chỗ nào trên màn nói kết luận ban đầu viết gì.
    expect(html).toContain("Giao Tài chính đối chiếu số liệu.");
    expect(html).toContain(nhuTrongHTML(NGUON_GIAO_KHOA));
    // Ô ĐÃ ĐIỀN SẴN: nội dung kết luận có mặt TRONG chính thuộc tính `value` của ô nhập, không chỉ
    // ở câu nhắc phía trên. So cả `id="giao-tieu-de"` để phép so không xanh nhờ câu nhắc ấy —
    // cùng chuỗi, hai chỗ, và chỉ một trong hai là thứ ca này canh.
    // Cắt từ `id="giao-tieu-de"` tới dấu đóng thẻ rồi mới so, thay vì ghim nguyên một chuỗi thẻ:
    // thứ tự thuộc tính do React quyết và nó đổi được mà không ai đụng vào màn này — một ca ghim
    // thứ tự sẽ đỏ vì lý do sai, và lần đỏ vì lý do sai đầu tiên là lần người ta bắt đầu bỏ qua nó.
    const o = html.slice(html.indexOf('id="giao-tieu-de"'));
    expect(o.slice(0, o.indexOf("/>"))).toContain(
      'value="Giao Tài chính đối chiếu số liệu."',
    );
  });

  it("câu từ chối của máy chủ hiện NGUYÊN VĂN, và chỉ trên dòng kết luận bị từ chối", () => {
    const tach = phepTach({
      moOKetLuan: "k7",
      loi: { ketLuanID: "k7", thongBao: "Bạn không có quyền tạo nhiệm vụ." },
    });

    expect(veDong(ketLuan({ id: "k7" }), tach)).toContain("Bạn không có quyền tạo nhiệm vụ.");
    // Kết luận khác: câu ấy không nói về nó.
    expect(veDong(ketLuan({ id: "k9" }), tach)).not.toContain(
      "Bạn không có quyền tạo nhiệm vụ.",
    );
  });

  it("tách xong thì báo SỐ SỔ máy chủ vừa cấp, trên đúng dòng kết luận ấy", () => {
    const tach = phepTach({ daXong: { ketLuanID: "k7", maNhiemVu: "NV12" } });

    expect(veDong(ketLuan({ id: "k7" }), tach)).toContain(cauDaTach("NV12"));
    expect(veDong(ketLuan({ id: "k9" }), tach)).not.toContain("NV12");
  });

  it("đang gửi thì nút khoá — bấm đóng giữa chừng là huỷ khoá chống trùng đang bay", () => {
    expect(veDong(ketLuan(), phepTach({ dangGui: true }))).toContain("disabled");
  });

  it("phần §3 CÒN THIẾU nằm trong khối chưa dựng được ở đầu màn, không chỉ trong chú thích mã", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);

    // Thứ còn thiếu là ĐIỀN SẴN, không còn là cả cái nút.
    expect(html).toContain("ĐIỀN SẴN");
    expect(html).toContain(String(PHAN_CHUA_DUNG.length));
    // Cổng quyền client cũng phải được nói ra — nó là thứ người đọc màn sẽ đi tìm.
    expect(html).toContain("task.create");
  });
});

describe("danh sách thẻ", () => {
  it("sổ rỗng thì nói ra, không để trang trắng", () => {
    const html = renderToStaticMarkup(
      <DanhSachBienBan
        bienBan={[]}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan={null}
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );

    expect(html).toContain(SO_RONG);
  });

  it("câu lỗi của MỘT thẻ không hiện trên thẻ khác", () => {
    // Lỗi mang theo id của biên bản đã từ chối. Hiện nó ở mọi thẻ sẽ nói với cán bộ rằng cả
    // quyển sổ vừa hỏng.
    const html = renderToStaticMarkup(
      <DanhSachBienBan
        bienBan={[bienBan(), bienBan({ id: "01JBB2", title: "Giao ban tháng 9" })]}
        lanGhiXong={0}
        dangGui={false}
        loiKetLuan={{ bienBanID: "01JBB2", thongBao: "Thiếu nội dung kết luận." }}
        guiKetLuan={() => {}}
        tach={phepTach()}
        vongDoi={vongDoi()}
      />,
    );

    expect(html.split("Thiếu nội dung kết luận.").length - 1).toBe(1);
  });
});

describe("hàng thêm kết luận", () => {
  it("nút gửi TẮT khi ô còn trống — một kết luận rỗng là một dòng không ai thi hành được", () => {
    const html = renderToStaticMarkup(
      <HangThemKetLuan bienBanID="01JBB1" dangGui={false} gui={() => {}} />,
    );

    expect(html).toContain("disabled");
    expect(html).toContain(PLACEHOLDER_KET_LUAN);
  });

  it("id của ô nhập mang id biên bản — hai thẻ trên một trang là hai `label` khác nhau", () => {
    const html = renderToStaticMarkup(
      <HangThemKetLuan bienBanID="01JBB9" dangGui={false} gui={() => {}} />,
    );

    expect(html).toContain("them-ket-luan-01JBB9");
  });
});

const DANH_BA: identity_canBoChonNguoiRa[] = [
  { code: "CB-2026-7K3M9Q", full_name: "Nguyễn Văn An", position: "Chủ tịch UBND", department_id: "" },
  { code: "CB-2026-1A2B3C", full_name: "Trần Thị Bình", position: "Văn phòng", department_id: "" },
];

describe("biểu mẫu nhập biên bản §4", () => {
  function veForm(
    loi: string | null = null,
    che: CheBieuMau = { loai: "tao", boSungCho: null },
  ): string {
    return renderToStaticMarkup(
      <FormNhapBienBan
        che={che}
        danhBa={DANH_BA}
        loiDanhBa={null}
        dangGui={false}
        loi={loi}
        huy={() => {}}
        luu={() => {}}
        sua={() => {}}
      />,
    );
  }

  it("có đủ các ô dựng được, và chỉ hai ô là bắt buộc", () => {
    const html = veForm();

    expect(html).toContain("Tên cuộc họp *");
    expect(html).toContain("Ngày họp *");
    expect(html).toContain("Số hiệu biên bản");
    expect(html).toContain("Địa điểm");
    expect(html).toContain("Thành phần tham dự");
    expect(html).toContain("Nội dung biên bản");
    expect(html).toContain("Các kết luận");
    // Số hiệu KHÔNG mang dấu sao: nó tuỳ chọn, và nó không phải mã định danh.
    expect(html).not.toContain("Số hiệu biên bản *");
    expect(html).not.toContain("Chủ trì *");
    expect(html).not.toContain("Thư ký *");
  });

  it("nói thẳng rằng số hiệu KHÔNG phải mã tra cứu", () => {
    expect(veForm()).toContain("không phải mã tra cứu");
  });

  it('ngày họp là ô `type="date"` — ngày lịch, không mốc thời gian', () => {
    expect(veForm()).toContain('type="date"');
  });

  it("nút Lưu TẮT khi chưa gõ tên và ngày", () => {
    const html = veForm();
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_LUU));
    expect(html).toContain("disabled");
  });

  it("câu từ chối của máy chủ hiện nguyên văn", () => {
    const html = veForm("biên bản họp: thiếu ngày họp");

    expect(html).toContain("biên bản họp: thiếu ngày họp");
    expect(html).toContain('role="alert"');
  });

  it("cảnh báo bí mật nhà nước có mặt — đã quyết: không có cờ “mật” nào", () => {
    const html = veForm();
    expect(html).toContain(CANH_BAO_BI_MAT);
    expect(html).not.toMatch(/>\s*Mật\s*</);
  });

  it("Chủ trì và Thư ký là ô CHỌN từ danh bạ: giá trị là MÃ, chữ hiện là họ tên · chức vụ", () => {
    const html = veForm();
    expect(html).toContain(">Chủ trì<");
    expect(html).toContain(">Thư ký<");
    // Giá trị gửi đi là MÃ nghiệp vụ; họ tên chỉ là chữ hiện.
    expect(html).toContain('value="CB-2026-7K3M9Q"');
    expect(html).toContain("Nguyễn Văn An · Chủ tịch UBND");
    expect(html).not.toContain('value="Nguyễn Văn An"');
  });

  it("thành phần: ô chọn cán bộ VÀ ô chữ tự do — khách mời không có tài khoản vẫn ghi được", () => {
    const html = veForm();
    expect(html).toContain('id="chon-thanh-phan"');
    expect(html).toContain('id="thanh-phan-khac"');
  });

  it("KHÔNG có vùng tệp đính kèm — phần đã khai là chưa dựng", () => {
    expect(veForm()).not.toContain('type="file"');
  });

  it("biểu mẫu BỔ SUNG nói nó bổ sung cho biên bản đã ký nào", () => {
    const goc = bienBan({ status: "da-ky", title: "Giao ban tháng 7" });
    const html = veForm(null, { loai: "tao", boSungCho: goc });
    expect(html).toContain("Lập biên bản bổ sung");
    expect(html).toContain("Giao ban tháng 7");
  });

  it("biểu mẫu SỬA điền sẵn, KHÔNG có ô “Các kết luận”, nút Lưu tắt khi chưa đổi gì", () => {
    const ban = bienBan({
      chaired_by: "CB-2026-7K3M9Q",
      minutes_taker: "CB-2026-1A2B3C",
      content: "Toàn văn đã lưu.",
      attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"],
    });
    const html = veForm(null, { loai: "sua", ban });
    expect(html).toContain("Sửa biên bản (dự thảo)");
    expect(html).toContain("Toàn văn đã lưu.");
    expect(html).toContain("Đại diện thôn Hà Lam");
    expect(html).not.toContain('id="cac-ket-luan"');
    const nut = html.slice(html.lastIndexOf("<button"), html.lastIndexOf("</button>"));
    expect(nut).toContain("disabled");
    expect(nut).toContain(NHAN_NUT_LUU_SUA);
  });
});

describe("vòng đời trên thẻ — dự thảo và đã ký", () => {
  const DA_KY = bienBan({
    status: "da-ky",
    signed_by: "CB-2026-7K3M9Q",
    signed_at: "2026-08-06T03:00:00Z",
  });

  it("chip trạng thái: “Dự thảo” và “Đã ký”", () => {
    expect(veThe(bienBan())).toContain(">Dự thảo<");
    expect(veThe(DA_KY)).toContain(">Đã ký<");
  });

  it("dự thảo: Sửa · Gỡ · Thêm kết luận có; Ghi Thông báo · Lập bổ sung KHÔNG", () => {
    const html = veThe(bienBan({ task_count: 0 }));
    expect(html).toContain(NHAN_NUT_SUA_BIEN_BAN);
    expect(html).toContain(NHAN_NUT_XOA_BIEN_BAN);
    expect(html).toContain(PLACEHOLDER_KET_LUAN);
    expect(html).not.toContain(NHAN_NUT_GHI_THONG_BAO);
    expect(html).not.toContain(NHAN_NUT_BO_SUNG);
  });

  it("đã ký: KHÔNG Sửa · Gỡ · Ký · Thêm kết luận; CÓ Lập bổ sung và Ghi Thông báo (khi chưa có)", () => {
    const html = veThe(DA_KY, phepTach(), vongDoi({ coQuyenKy: true }));
    expect(html).not.toContain(NHAN_NUT_SUA_BIEN_BAN);
    expect(html).not.toContain(NHAN_NUT_XOA_BIEN_BAN);
    expect(html).not.toContain(`>${NHAN_NUT_KY}<`);
    expect(html).not.toContain(PLACEHOLDER_KET_LUAN);
    expect(html).toContain(NHAN_NUT_BO_SUNG);
    expect(html).toContain(NHAN_NUT_GHI_THONG_BAO);
  });

  it("đã ký VÀ đã có Thông báo: nút Ghi Thông báo biến mất — máy chủ chỉ nhận một lần", () => {
    const html = veThe(
      bienBan({
        status: "da-ky",
        notice: { reference_no: "12/TB-UBND", issued_on: "2026-08-07" },
      }),
    );
    expect(html).not.toContain(NHAN_NUT_GHI_THONG_BAO);
  });

  it("nút Ký CHỈ hiện khi phiên có `task.approve` — ca BỊ TỪ CHỐI trước", () => {
    expect(veThe(bienBan(), phepTach(), vongDoi({ coQuyenKy: false }))).not.toContain(
      `>${NHAN_NUT_KY}<`,
    );
    expect(veThe(bienBan(), phepTach(), vongDoi({ coQuyenKy: true }))).toContain(
      `>${NHAN_NUT_KY}<`,
    );
  });

  it("hộp Ký nói rõ hệ quả và có hai ô Thông báo tuỳ chọn", () => {
    const html = veThe(
      bienBan(),
      phepTach(),
      vongDoi({ coQuyenKy: true, hop: { dich: "01JBB1", loai: "ky" } }),
    );
    expect(html).toContain(CAU_XAC_NHAN_KY);
    expect(html).toContain(NHAN_NUT_XAC_NHAN_KY);
    expect(html).toContain("Số, ký hiệu Thông báo kết luận");
  });

  it("Gỡ biên bản TẮT kèm lý do khi còn nhiệm vụ trỏ về", () => {
    const html = veThe(bienBan({ task_count: 2 }));
    expect(html).toContain(VI_SAO_BIEN_BAN_CON_NHIEM_VU);
    const dau = html.lastIndexOf("<button", html.indexOf('aria-describedby="ly-do-xoa-01JBB1"'));
    const nut = html.slice(dau, html.indexOf("</button>", dau));
    expect(nut).toContain("disabled");
    expect(nut).toContain(NHAN_NUT_XOA_BIEN_BAN);
  });

  it("hộp Gỡ biên bản đòi lý do — nút tắt khi chưa gõ", () => {
    const html = veThe(
      bienBan({ task_count: 0 }),
      phepTach(),
      vongDoi({ hop: { dich: "01JBB1", loai: "xoa" } }),
    );
    expect(html).toContain("Lý do gỡ biên bản *");
    expect(html).toContain('<button type="submit" class="nut-xoa" disabled="">');
  });

  it("câu lỗi vòng đời của MỘT biên bản chỉ hiện trên thẻ ấy", () => {
    const vd = vongDoi({ loi: { dich: "01JBB2", thongBao: "biên bản họp đã ký — …" } });
    expect(veThe(bienBan(), phepTach(), vd)).not.toContain("biên bản họp đã ký — …");
    expect(veThe(bienBan({ id: "01JBB2" }), phepTach(), vd)).toContain("biên bản họp đã ký — …");
  });

  it("biên bản bổ sung có liên kết về biên bản gốc", () => {
    expect(veThe(bienBan({ supplements_id: "01JBBGOC" }))).toContain('href="#bien-ban-01JBBGOC"');
  });

  it("“Xem biên bản” là neo tới chính thẻ — không mang dữ liệu nào ngoài id", () => {
    expect(veThe(bienBan())).toContain('href="#bien-ban-01JBB1"');
  });
});

describe("Xem biên bản — toàn văn, người ký, Thông báo, bổ sung hai chiều", () => {
  const CHI_TIET = bienBan({
    status: "da-ky",
    chaired_by: "CB-2026-7K3M9Q",
    minutes_taker: "CB-2026-1A2B3C",
    signed_by: "CB-2026-7K3M9Q",
    signed_at: "2026-08-06T03:00:00Z",
    notice: { reference_no: "12/TB-UBND", issued_on: "2026-08-07" },
    supplements_id: "01JBBGOC",
    supplemented_by: ["01JBBBS1"],
    content: "Dòng một.\nDòng hai.",
    attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"],
  });

  function veChiTiet(bb: petitions_bienBanRa = CHI_TIET): string {
    return renderToStaticMarkup(
      <ChiTietBienBan
        tai={{ pha: "xong", duLieu: bb }}
        danhBa={danhBaTheoMa(DANH_BA)}
        dong={() => {}}
      />,
    );
  }

  it("hiện chủ trì, thư ký bằng HỌ TÊN tra từ danh bạ", () => {
    const html = veChiTiet();
    expect(html).toContain("Nguyễn Văn An · Chủ tịch UBND");
    expect(html).toContain("Trần Thị Bình · Văn phòng");
  });

  it("thành phần: dòng chữ tự do ra nguyên văn", () => {
    expect(veChiTiet()).toContain("Đại diện thôn Hà Lam");
  });

  it("toàn văn giữ xuống dòng", () => {
    expect(veChiTiet()).toContain("Dòng một.<br/>");
  });

  it("Thông báo kết luận: số và NGÀY LỊCH d/M/yyyy", () => {
    expect(veChiTiet()).toContain("Số 12/TB-UBND, ngày 7/8/2026");
  });

  it("người ký và ngày ký", () => {
    expect(veChiTiet()).toContain("Nguyễn Văn An · Chủ tịch UBND · ngày 6/8/2026");
  });

  it("bổ sung HAI CHIỀU: về biên bản gốc và tới biên bản bổ sung", () => {
    const html = veChiTiet();
    expect(html).toContain('href="#bien-ban-01JBBGOC"');
    expect(html).toContain('href="#bien-ban-01JBBBS1"');
  });

  it("chủ trì không còn trong danh bạ vẫn hiện MÃ kèm câu trung tính, không một ô trống", () => {
    expect(veChiTiet(bienBan({ chaired_by: "CB-DA-NGHI" }))).toContain(
      "CB-DA-NGHI (không có trong danh bạ cán bộ đang hoạt động)",
    );
  });

  it("đọc hỏng thì câu máy chủ ra nguyên văn", () => {
    const html = renderToStaticMarkup(
      <ChiTietBienBan
        tai={{ pha: "loi", thongBao: "Không tìm thấy biên bản." }}
        danhBa={null}
        dong={() => {}}
      />,
    );
    expect(html).toContain("Không tìm thấy biên bản.");
  });
});

describe("dòng kết luận — chip trạng thái từ MÁY CHỦ và các nút theo trạng thái", () => {
  it("chip đọc đúng mã máy chủ; quá hạn là chip ĐỎ", () => {
    expect(veDong(ketLuan({ status: "chua-giao" }))).toContain(">Chưa giao<");
    expect(veDong(ketLuan({ status: "dang-thuc-hien", task_count: 1 }))).toContain(
      ">Đang thực hiện<",
    );
    expect(
      veDong(ketLuan({ status: "hoan-thanh", task_count: 1, task_done_count: 1 })),
    ).toContain(">Hoàn thành<");
    expect(veDong(ketLuan({ status: "qua-han", task_count: 1 }))).toContain(
      '<span class="chip chip-cham">Quá hạn</span>',
    );
  });

  it("KHÔNG tự suy trạng thái từ bộ đếm: 1/1 xong mà máy chủ nói “qua-han” thì vẽ Quá hạn", () => {
    const html = veDong(ketLuan({ status: "qua-han", task_count: 1, task_done_count: 1 }));
    expect(html).toContain(">Quá hạn<");
    expect(html).not.toContain(">Hoàn thành<");
  });

  it("đánh dấu “không phát sinh”: chip nói đúng lý do, nút Tách ẨN, còn Bỏ dấu", () => {
    const html = veDong(ketLuan({ no_task: true, status: "hoan-thanh" }));
    expect(html).toContain(">Không phát sinh nhiệm vụ<");
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_TACH));
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_BO_DAU));
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
  });

  it("dự thảo, chưa có nhiệm vụ: Sửa · Gỡ bấm được, Đánh dấu có", () => {
    const html = veDong(ketLuan({ ordinal: 3, task_count: 0 }));
    expect(html).toContain('aria-label="Sửa kết luận số 3"');
    expect(html).toContain('aria-label="Gỡ kết luận số 3"');
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
    expect(html).not.toContain(VI_SAO_KET_LUAN_KHOA);
  });

  it("dự thảo, ĐÃ có nhiệm vụ: Sửa · Gỡ TẮT kèm lý do, Đánh dấu ẨN", () => {
    const html = veDong(ketLuan({ ordinal: 3, task_count: 2, status: "dang-thuc-hien" }));
    for (const nhan of ["Sửa kết luận số 3", "Gỡ kết luận số 3"]) {
      const dau = html.lastIndexOf("<button", html.indexOf(`aria-label="${nhan}"`));
      const nut = html.slice(dau, html.indexOf("</button>", dau));
      expect(nut).toContain("disabled");
    }
    expect(html).toContain(VI_SAO_KET_LUAN_KHOA);
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
  });

  it("biên bản ĐÃ KÝ: Sửa · Gỡ · Đánh dấu · Bỏ dấu ẨN, còn Tách", () => {
    const daKy = bienBan({ status: "da-ky" });
    const html = veDong(ketLuan({ ordinal: 3 }), phepTach(), vongDoi(), daKy);
    expect(html).not.toContain("Sửa kết luận số 3");
    expect(html).not.toContain("Gỡ kết luận số 3");
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_DANH_DAU));
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_TACH));
  });

  it("có nhiệm vụ thì có nút mở danh sách; mở ra thì mã · tên · trạng thái · hạn", () => {
    const kl = ketLuan({ id: "k3", ordinal: 3, task_count: 1, status: "dang-thuc-hien" });
    expect(veDong(kl)).toContain("Xem 1 nhiệm vụ đã tách");
    const nv = {
      code: "NV12",
      title: "Đối chiếu số liệu giải ngân",
      status: "dang-thuc-hien",
      due_at: "2026-08-20T16:59:59Z",
    } as petitions_nhiemVuRa;
    const html = veDong(
      kl,
      phepTach(),
      vongDoi({ nhiemVuKL: new Map([["k3", { pha: "xong", duLieu: [nv] }]]) }),
    );
    expect(html).toContain("NV12");
    expect(html).toContain("Đối chiếu số liệu giải ngân");
    expect(html).toContain("Đang thực hiện");
    expect(html).toContain("Hạn 20/8/2026");
  });

  it("câu lỗi vòng đời của MỘT kết luận chỉ hiện trên dòng ấy", () => {
    const vd = vongDoi({ loi: { dich: "k7", thongBao: "kết luận đã được tách — …" } });
    expect(veDong(ketLuan({ id: "k7" }), phepTach(), vd)).toContain("kết luận đã được tách");
    expect(veDong(ketLuan({ id: "k9" }), phepTach(), vd)).not.toContain("kết luận đã được tách");
  });
});
