import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";

import {
  CHO_NUT_TACH,
  CHUA_TACH_NHIEM_VU,
  NHAN_NUT_LUU,
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

function veThe(bb: petitions_bienBanRa): string {
  return renderToStaticMarkup(
    <TheBienBan
      bienBan={bb}
      lanGhiXong={0}
      dangGui={false}
      loiKetLuan={null}
      guiKetLuan={() => {}}
    />,
  );
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
    const html = renderToStaticMarkup(<DongKetLuan ketLuan={ketLuan({ ordinal: 4 })} />);

    expect(html).toContain('<span class="chip">4</span>');
    expect(html).not.toContain('<span class="chip">1</span>');
  });

  it("dòng phụ nói `Chưa tách thành nhiệm vụ nào` khi chưa có nhiệm vụ nào", () => {
    const html = renderToStaticMarkup(<DongKetLuan ketLuan={ketLuan({ task_count: 0 })} />);
    expect(html).toContain(CHUA_TACH_NHIEM_VU);
  });

  it("có nhiệm vụ rồi thì là hai con số, không còn câu `Chưa tách…`", () => {
    const html = renderToStaticMarkup(
      <DongKetLuan ketLuan={ketLuan({ task_count: 1, task_done_count: 1 })} />,
    );

    expect(html).toContain("1/1 nhiệm vụ đã hoàn thành");
    expect(html).not.toContain(CHUA_TACH_NHIEM_VU);
  });
});

describe("nút `Tách thành nhiệm vụ` — KHÔNG dựng lượt này, và màn nói ra điều đó", () => {
  it("chỗ ấy là một dòng chữ giải thích, KHÔNG phải một nút", () => {
    const html = renderToStaticMarkup(<DongKetLuan ketLuan={ketLuan()} />);

    expect(html).toContain(nhuTrongHTML(CHO_NUT_TACH));
    // Không một `<button>` nào trong một dòng kết luận: một nút mờ nói "bạn không có quyền", và
    // đó là một câu khác hẳn với "màn hình chưa dựng".
    expect(html).not.toContain("<button");
  });

  it("lý do nằm trong khối chưa dựng được ở đầu màn, không chỉ trong chú thích mã", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);

    expect(html).toContain("Tách thành nhiệm vụ");
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
