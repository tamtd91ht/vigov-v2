import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";

import {
  cauDaTach,
  CHUA_DIEN_SAN,
  CHUA_TACH_NHIEM_VU,
  NGUON_GIAO_KHOA,
  NHAN_NUT_LUU,
  NHAN_NUT_TACH,
  NHAN_NUT_THEM_KET_LUAN,
  PHAN_CHUA_DUNG,
  PLACEHOLDER_KET_LUAN,
  SO_RONG,
} from "./nhan-bien-ban";
import {
  DanhSachBienBan,
  DongKetLuan,
  FormNhapBienBan,
  HangThemKetLuan,
  KhoiChuaDung,
  TheBienBan,
  type PhepTach,
} from "./so-bien-ban";

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

function veThe(bb: petitions_bienBanRa, tach: PhepTach = phepTach()): string {
  return renderToStaticMarkup(
    <TheBienBan
      bienBan={bb}
      lanGhiXong={0}
      dangGui={false}
      loiKetLuan={null}
      guiKetLuan={() => {}}
      tach={tach}
    />,
  );
}

function veDong(kl: petitions_ketLuanRa, tach: PhepTach = phepTach()): string {
  return renderToStaticMarkup(<DongKetLuan bienBanID="01JBB1" ketLuan={kl} tach={tach} />);
}

describe("thẻ biên bản §2", () => {
  it("header mang tiêu đề, dòng meta và badge của máy chủ", () => {
    const html = veThe(bienBan());

    expect(html).toContain("Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026");
    expect(html).toContain("5/8/2026 · 31/BB-UBND · Phòng họp UBND xã");
    expect(html).toContain("1 kết luận · 1/3 nhiệm vụ xong");
  });

  it("thiếu số hiệu và địa điểm thì dòng meta chỉ còn ngày — không có dấu chấm bơ vơ", () => {
    const html = veThe(bienBan({ reference_no: "", location: "" }));

    expect(html).toContain("5/8/2026");
    expect(html).not.toContain("· ·");
  });

  it("biên bản chưa có kết luận nào VẪN hiện, và nói ra trạng thái ấy (§7.3)", () => {
    const html = veThe(bienBan({ conclusions: [], task_count: 0, task_done_count: 0 }));

    expect(html).toContain("Biên bản này chưa ghi kết luận nào.");
    expect(html).toContain("0 kết luận · 0/0 nhiệm vụ xong");
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

  it("hộp mở thì nói ra ba điều: kết luận gốc, nguồn giao KHOÁ, và ô chưa điền sẵn", () => {
    const html = veDong(
      ketLuan({ id: "k7", ordinal: 3, content: "Giao Tài chính đối chiếu số liệu." }),
      phepTach({ moOKetLuan: "k7" }),
    );

    // Nội dung kết luận đứng ngay trên biểu mẫu — đó là chỗ cán bộ chép từ, vì ô "Nội dung nhiệm
    // vụ" chưa điền sẵn được.
    expect(html).toContain("Giao Tài chính đối chiếu số liệu.");
    expect(html).toContain(nhuTrongHTML(NGUON_GIAO_KHOA));
    expect(html).toContain(nhuTrongHTML(CHUA_DIEN_SAN));
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

describe("biểu mẫu nhập biên bản §4", () => {
  function veForm(loi: string | null = null): string {
    return renderToStaticMarkup(
      <FormNhapBienBan dangGui={false} loi={loi} huy={() => {}} luu={() => {}} />,
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
  });

  it("nói thẳng rằng số hiệu KHÔNG phải mã tra cứu", () => {
    // Số hiệu biên bản Việt Nam lặp lại theo năm (`31/BB-UBND` năm sau lại có). Một cán bộ tưởng
    // nó là mã sẽ đi tra cứu bằng nó và tìm ra hai biên bản.
    expect(veForm()).toContain("không phải mã tra cứu");
  });

  it("ngày họp là ô `type=\"date\"` — ngày lịch, không mốc thời gian", () => {
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

  it("KHÔNG có ô Chủ trì và KHÔNG có vùng tệp đính kèm — hai phần đã khai là chưa dựng", () => {
    const html = veForm();

    expect(html).not.toContain(">Chủ trì<");
    expect(html).not.toContain('type="file"');
  });
});
