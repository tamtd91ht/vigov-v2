import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type {
  finance_bangDayDuRa,
  finance_chiSoNamRa,
  finance_cotRa,
  finance_dongRa,
  finance_tomTatRa,
} from "@/lib/api/schema.gen";

import {
  BangDayDu,
  DongKhoanMuc,
  FormGoKemLyDo,
  FormSuaDong,
  KhoiChuaDung,
  TheChiSoNam,
  TheTomTat,
} from "./bang-thu-chi";
import { LapBang } from "./lap-bang";
import {
  CANH_BAO_GO_BANG,
  CAU_KHONG_QUY_DOI,
  CAU_THIEU_QUYEN_XEM,
  nhanDatDongTong,
  nhanGoKhoanMuc,
  nhanThemCon,
  NHAN_SUA_TEN,
  PHAN_CHUA_DUNG,
} from "./nhan-thu-chi";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không — bổ cho các ca kiểm module thuần, vốn chỉ canh
 * quyết định bên trong hàm. Đã đo trên màn Danh mục: bôi trắng câu quan trọng nhất của một màn
 * hình vẫn để lại toàn bộ bài kiểm xanh và `tsc` sạch.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **BỊ TỪ CHỐI**, không phải nhóm đủ quyền: tài khoản của
 * người viết mã luôn có cả ba khoá, nên nhánh thiếu quyền là nhánh không ai nhìn thấy trong lúc
 * phát triển. Và chỗ dễ gắn cổng nhầm nhất được kiểm riêng: **GỠ** và **ĐÁNH DẤU DÒNG TỔNG** đứng
 * sau `budget.confirm`, KHÔNG sau `budget.update`.
 */

const COT: finance_cotRa[] = [
  { id: "C1", name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam" },
  { id: "C2", name: "Chi ngân sách", order: 2, type: "so", role: "chi-ngan-sach" },
  { id: "C3", name: "So sánh TH/DT (%)", order: 3, type: "phan_tram", formula: "col_2 / col_1 * 100" },
];

function dong(sua: Partial<finance_dongRa> = {}): finance_dongRa {
  return {
    id: "A",
    no: "A",
    name: "CHI NGÂN SÁCH NHÀ NƯỚC",
    order: 1,
    method: "manual",
    level: 0,
    is_headline: false,
    values: { C1: 5502660, C2: 3401673 },
    ...sua,
  };
}

const TOM_TAT: finance_tomTatRa = {
  headline_line_id: "A",
  cells: [
    { column_id: "C1", name: "Dự toán năm", role: "du-toan-nam", value: 3794740 },
    { column_id: "C2", name: "Chi ngân sách", role: "chi-ngan-sach", value: 3463459 },
  ],
  indicator: { name: "Chi đạt dự toán", basis_points: 9130 },
};

function bang(sua: Partial<finance_bangDayDuRa> = {}): finance_bangDayDuRa {
  return {
    sheet: {
      id: "01JBANG",
      code: "NS-2026-CHI-01",
      year: 2026,
      kind: "chi",
      revision: 1,
      title: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026",
      unit: "Triệu đồng",
      cumulative_to: "2026-08-25",
    },
    columns: COT,
    lines: [dong(), dong({ id: "I", no: "I", name: "Chi đầu tư phát triển", parent_id: "A" })],
    summary: TOM_TAT,
    ...sua,
  };
}

/**
 * Nhãn a11y như nó THẬT SỰ nằm trong chuỗi HTML.
 *
 * ⚠ HÀM NÀY ĐẾN TỪ MỘT LẦN XANH SAI ĐÃ ĐO, không từ sự cẩn thận: `nhanDatDongTong` sinh ra
 * `Đặt "X" làm con số tổng`, và `renderToStaticMarkup` thoát dấu ngoặc kép thành `&quot;` khi ghi
 * vào thuộc tính. Nên một phép so với chuỗi THÔ đi qua `not.toContain` sẽ XANH kể cả khi cái nút ấy
 * đang nằm chình ình trên trang — tức là ba bài kiểm cổng quyền quan trọng nhất của tệp này từng
 * canh đúng con số không. Mọi phép so nhãn a11y ở dưới đi qua hàm này.
 */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/** Bảng vẽ với một bộ quyền. `thaoTac` rỗng: tệp này canh cái GÌ hiện ra, không canh lời gọi. */
function veBang(coGhi: boolean, coXacNhan: boolean, duLieu = bang()): string {
  return renderToStaticMarkup(
    <BangDayDu
      duLieu={duLieu}
      thuGon={new Set()}
      datThuGon={() => {}}
      coGhi={coGhi}
      coXacNhan={coXacNhan}
      dangGui={false}
      dangSuaDong={null}
      moSua={() => {}}
      huySua={() => {}}
      luuSua={() => {}}
      moThem={() => {}}
      moGoDong={() => {}}
      datTong={() => {}}
      moGoBang={() => {}}
    />,
  );
}

describe("CỔNG QUYỀN — nhánh bị từ chối", () => {
  it("thiếu `budget.read`: cả màn không dựng, và câu từ chối gọi ĐÚNG TÊN khoá", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen quyetDinh={{ hien: false, vi: "khong-du-quyen" }} cauThieuQuyen={CAU_THIEU_QUYEN_XEM}>
        {veBangJSX()}
      </KhungQuyen>,
    );

    expect(html).toContain("budget.read");
    expect(html).toContain(CAU_THIEU_QUYEN_XEM);
    // Không một dòng khoản mục nào ra tới trang.
    expect(html).not.toContain("CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026");
  });

  it("KHÔNG đọc được quyền (phiên hết hạn) cũng là ẨN — 'chưa rõ' không được xử như 'có'", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen
        quyetDinh={{ hien: false, vi: "khong-doc-duoc", thongBao: "Phiên làm việc đã hết hạn." }}
        cauThieuQuyen={CAU_THIEU_QUYEN_XEM}
      >
        {veBangJSX()}
      </KhungQuyen>,
    );

    expect(html).toContain("Phiên làm việc đã hết hạn.");
    expect(html).not.toContain("CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026");
  });

  it("chỉ đọc (không `budget.update`, không `budget.confirm`): KHÔNG nút ghi nào ra trang", () => {
    const html = veBang(false, false);

    expect(html).not.toContain(nhuTrongHTML(nhanThemCon("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanGoKhoanMuc("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(NHAN_SUA_TEN));
    expect(html).not.toContain("Thêm khoản mục cấp cao nhất");
    expect(html).not.toContain("Gỡ bảng");
  });

  it("tài khoản chỉ đọc vẫn ĐỌC ĐƯỢC dòng nào đang là con số tổng", () => {
    // Đó là thông tin ai xem bảng cũng cần, kể cả người không đổi được nó — ẩn luôn ngôi sao sẽ
    // làm người đọc không biết ba con số tóm tắt phía trên lấy từ dòng nào.
    expect(veBang(false, false)).toContain("★");
  });
});

describe("CỔNG QUYỀN — chỗ dễ gắn nhầm nhất", () => {
  it("`budget.update` MỘT MÌNH KHÔNG mở nút GỠ và KHÔNG mở ngôi sao", () => {
    // Đây là bài kiểm đắt nhất của cả màn. Gỡ một khoản mục đổi ngay con số lãnh đạo đã đọc, và
    // đánh sao đổi DÒNG mà mọi ô tóm tắt cùng hai chỉ số đọc từ đó — cả hai đứng sau
    // `budget.confirm` ở máy chủ (`routes.go:909`, `:943`). Gắn chúng vào quyền nhập liệu là mở
    // một thao tác xác nhận cho người chỉ được nhập số, và không có gì trên màn hình nói ra.
    const html = veBang(true, false);

    expect(html).toContain(nhuTrongHTML(nhanThemCon("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).toContain(nhuTrongHTML(NHAN_SUA_TEN));
    expect(html).toContain("Thêm khoản mục cấp cao nhất");

    expect(html).not.toContain(nhuTrongHTML(nhanGoKhoanMuc("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain("Gỡ bảng");
  });

  it("`budget.confirm` mở GỠ và ngôi sao, với nhãn a11y nguyên văn đặc tả", () => {
    const html = veBang(true, true);

    expect(html).toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).toContain(nhuTrongHTML(nhanGoKhoanMuc("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).toContain("Gỡ bảng");
  });

  it("`budget.confirm` MỘT MÌNH không mở nút thêm và không mở sửa tên", () => {
    const html = veBang(false, true);

    expect(html).toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanThemCon("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain("Thêm khoản mục cấp cao nhất");
  });
});

describe("con số ra tới trang", () => {
  it("IN THẲNG SỐ MÁY CHỦ TRẢ, không quy đổi theo nhãn đơn vị tính", () => {
    const html = veBang(false, false);

    expect(html).toContain("5.502.660");
    // Nếu có ai chia cho 1e6 vì `unit` đọc là "Triệu đồng" thì con số này sẽ thành "5,5".
    expect(html).not.toContain(">5,5<");
    expect(html).toContain(CAU_KHONG_QUY_DOI);
    expect(html).toContain("Đơn vị tính: Triệu đồng");
  });

  it("ô phần trăm trên từng dòng hiện DẤU GẠCH, không phải một tỷ lệ đoán từ `formula`", () => {
    // `formula` trỏ tới cột bằng `col_4`, `col_2` — không ánh xạ được sang mã cột hợp đồng trả về,
    // và máy chủ cố ý không diễn giải chuỗi ấy. Một tỷ lệ đoán sai trông y hệt một tỷ lệ đúng.
    const html = renderToStaticMarkup(
      <table>
        <tbody>
          <DongKhoanMuc
            hien={{ dong: dong(), cap: 0, coCon: false, moRong: false }}
            cot={COT}
            dongTongId=""
            coGhi={false}
            coXacNhan={false}
            dangGui={false}
            moRongDoi={() => {}}
            moSua={() => {}}
            them={() => {}}
            go={() => {}}
            datTong={() => {}}
          />
        </tbody>
      </table>,
    );

    expect(html).toContain("—");
    expect(html).not.toContain("61,8%");
  });

  it("thẻ tóm tắt KHÔNG đoán dòng tổng: chưa ai đánh dấu thì hiện CÂU của máy chủ, không hiện 0", () => {
    // §5 quy tắc 5 nói "mặc định dòng đầu tiên", và nửa ấy là cạm bẫy: bảng thu có hai dòng cấp
    // cao lồng nhau, bảng chi có `Tổng số` đứng NGANG HÀNG A…E (ADR 0035 §A).
    const cau = "ngan_sach: bảng chưa có dòng nào được đánh dấu là dòng tổng";
    const html = renderToStaticMarkup(
      <TheTomTat
        bang={bang().sheet}
        tomTat={{ unavailable_reason: cau, cells: [], indicator: { name: "Chi đạt dự toán", basis_points: null } }}
        soKhoanMuc={59}
      />,
    );

    expect(html).toContain(cau);
    expect(html).not.toContain("0%");
  });
});

describe("thẻ ba chỉ số của năm", () => {
  function chiSo(sua: Partial<finance_chiSoNamRa> = {}): finance_chiSoNamRa {
    return {
      year: 2026,
      revenue_achievement: { name: "Thu đạt dự toán", basis_points: 10811 },
      expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
      balance: { amount: 853304 },
      revenue_totals: [
        { column_id: "T1", name: "Thu ngân sách NSNN", role: "thu-nsnn", value: 4316764 },
        { column_id: "T2", name: "Thu ngân sách Thu xã hưởng", role: "thu-xa-huong", value: 3300800 },
      ],
      ...sua,
    };
  }

  it("`basis_points` là phần vạn: 10811 ra trang thành 108,11%", () => {
    expect(renderToStaticMarkup(<TheChiSoNam chiSo={chiSo()} />)).toContain("108,11%");
  });

  it("CẢ HAI số thu hiện ra, mỗi số gọi đúng tên (ADR 0035 §A)", () => {
    // Xã cần một số để báo cáo thu ngân sách, và một số để biết mình còn được giữ bao nhiêu. Màn
    // hình chỉ hiện một trong hai là trả lời một câu hỏi không ai đặt.
    const html = renderToStaticMarkup(<TheChiSoNam chiSo={chiSo()} />);
    expect(html).toContain("Thu ngân sách NSNN");
    expect(html).toContain("Thu ngân sách Thu xã hưởng");
  });

  it("xã chưa có bảng: thẻ VẪN Ở LẠI, nói câu của máy chủ, và KHÔNG hiện 0", () => {
    const cau = "xã chưa có bảng ngân sách cho năm và loại này";
    const html = renderToStaticMarkup(
      <TheChiSoNam
        chiSo={chiSo({
          revenue_achievement: { name: "Thu đạt dự toán", basis_points: null, unavailable_reason: cau },
          balance: { amount: null, unavailable_reason: cau },
          revenue_totals: [],
        })}
      />,
    );

    expect(html).toContain("Chỉ số ngân sách năm 2026");
    expect(html).toContain(cau);
    expect(html).not.toContain("0,00%");
  });
});

describe("cây khoản mục và thanh công cụ", () => {
  it("thu gọn một dòng thì con của nó KHÔNG ra trang, và bộ đếm nói đúng", () => {
    const html = renderToStaticMarkup(
      <BangDayDu
        duLieu={bang()}
        thuGon={new Set(["A"])}
        datThuGon={() => {}}
        coGhi={false}
        coXacNhan={false}
        dangGui={false}
        dangSuaDong={null}
        moSua={() => {}}
        huySua={() => {}}
        luuSua={() => {}}
        moThem={() => {}}
        moGoDong={() => {}}
        datTong={() => {}}
        moGoBang={() => {}}
      />,
    );

    expect(html).not.toContain("Chi đầu tư phát triển");
    expect(html).toContain("đang hiện 1/2 khoản mục");
  });

  it("mở hết thì đủ hai dòng", () => {
    expect(veBang(false, false)).toContain("đang hiện 2/2 khoản mục");
  });
});

describe("biểu mẫu ghi", () => {
  it("hộp GỠ có Ô LÝ DO BẮT BUỘC, không chỉ một nút Đồng ý", () => {
    // §6 chỉ vẽ "hộp xác nhận". Máy chủ đòi `reason` trong thân (luật 7 bất biến 1), nên một hộp
    // không có ô lý do nhận 400 ở MỌI lần bấm.
    const html = renderToStaticMarkup(
      <FormGoKemLyDo
        tieuDe="Gỡ bảng X"
        canhBao={CANH_BAO_GO_BANG}
        dangGui={false}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain('name="reason"');
    expect(html).toContain("required");
    expect(html).toContain(CANH_BAO_GO_BANG);
  });

  it("dòng CÓ CON không có ô số nào để gõ, và nói ra vì sao", () => {
    const html = renderToStaticMarkup(
      <table>
        <tbody>
          <FormSuaDong
            dong={dong({ method: "children" })}
            cot={[COT[0]!, COT[1]!]}
            soCotBang={7}
            dangGui={false}
            huy={() => {}}
            luu={() => {}}
          />
        </tbody>
      </table>,
    );

    expect(html).not.toContain('name="gia:C1"');
    expect(html).toContain("tổng các con");
  });

  it("dòng KHÔNG có con mở đủ ô số của các cột `so`, và không mở ô cho cột phần trăm", () => {
    const html = renderToStaticMarkup(
      <table>
        <tbody>
          <FormSuaDong
            dong={dong()}
            cot={[COT[0]!, COT[1]!]}
            soCotBang={7}
            dangGui={false}
            huy={() => {}}
            luu={() => {}}
          />
        </tbody>
      </table>,
    );

    expect(html).toContain('name="gia:C1"');
    expect(html).toContain('name="gia:C2"');
    expect(html).not.toContain('name="gia:C3"');
  });
});

describe("lập bảng — thứ thay cho `⬆ Nạp từ Excel`", () => {
  it("KHÔNG vẽ ô chọn tệp nào, vì hợp đồng không có tuyến nhận tệp", () => {
    // Vẽ một vùng kéo thả `.xlsx` ở đây là hứa với cán bộ một chức năng không tồn tại — đúng điều
    // `dau-trang.tsx` đã từ chối làm với ô tìm kiếm.
    const html = renderToStaticMarkup(
      <LapBang nam={2026} loai="chi" dangGui={false} datDangGui={() => {}} xong={() => {}} />,
    );

    expect(html).toContain("Lập bảng chi ngân sách năm 2026");
    expect(html).not.toContain('type="file"');
    expect(html).not.toContain(".xlsx");
  });
});

describe("những phần đặc tả vẽ mà chưa dựng được", () => {
  it("cả năm phần ra TỚI MÀN HÌNH kèm lý do, không giấu trong chú thích mã", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);

    expect(PHAN_CHUA_DUNG).toHaveLength(5);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(p.ten);
    }
    expect(html).toContain("Nạp từ Excel");
    expect(html).toContain("Các đợt thu, chi");
  });
});

/** Nội dung dựng bên trong cổng quyền ở hai ca từ chối — tách ra để hai ca không chép lại nhau. */
function veBangJSX() {
  return (
    <BangDayDu
      duLieu={bang()}
      thuGon={new Set()}
      datThuGon={() => {}}
      coGhi
      coXacNhan
      dangGui={false}
      dangSuaDong={null}
      moSua={() => {}}
      huySua={() => {}}
      luuSua={() => {}}
      moThem={() => {}}
      moGoDong={() => {}}
      datTong={() => {}}
      moGoBang={() => {}}
    />
  );
}
