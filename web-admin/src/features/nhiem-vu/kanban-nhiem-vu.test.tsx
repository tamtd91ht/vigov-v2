import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import {
  CAU_LOC_TRANG_THAI_KHONG_CO_COT,
  CHUA_PHAN_CONG,
  COT_RONG,
  GHI_CHU_DEM_COT,
  GHI_CHU_KANBAN_RE_NHANH,
  PHAN_CHUA_DUNG,
  TRANG_THAI_CHINH,
  cotPhaiDoc,
  nhanCotKanban,
  nhanDemCot,
} from "./nhan-nhiem-vu";
import { BangKanban, TheNhiemVu, type CotKanban, type DanhMucNhiemVu } from "./so-nhiem-vu";

/**
 * Bảng Kanban §4.1.
 *
 * NHÓM CHỊU LỰC Ở TỆP NÀY LÀ NHÓM "CỘT RỖNG VÌ CÁI GÌ". Ba lý do khác nhau cho ra ba màn hình phải
 * khác nhau — xã không có việc nào · bộ lọc cắt mất cột · lượt đọc cột ấy hỏng — và cả ba đều vẽ ra
 * một khoảng trắng nếu viết cẩu thả. Một cột im lặng đọc lên là "không còn việc nào", và đó là câu
 * một trưởng bộ phận tin rồi báo lên lãnh đạo.
 */

/** Chuỗi như nó THẬT SỰ nằm trong HTML — `renderToStaticMarkup` thoát `"` và `&`. */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

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
    assigner: "CB-2026-7K3M9Q",
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

const DANH_MUC: DanhMucNhiemVu = {
  loai: [],
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
  khoi: [],
  boPhan: [],
};

/** 15/09/2026 — gần ba tháng sau hạn 20/6, đúng bối cảnh "trễ" của đặc tả. */
const BAY_GIO = new Date("2026-09-15T03:00:00Z");

function trang(
  items: readonly petitions_nhiemVuRa[],
  conNua = false,
): page_Result_petitions_nhiemVuRa {
  return {
    items: [...items],
    next_cursor: conNua ? "CON-TRO-TRANG-SAU" : "",
    has_more: conNua,
  };
}

/** Năm cột đã đọc xong, mỗi cột nhận đúng những thẻ truyền vào. */
function namCot(
  theoCot: Partial<Record<string, page_Result_petitions_nhiemVuRa>> = {},
): readonly CotKanban[] {
  return TRANG_THAI_CHINH.map((ma) => ({
    ma,
    tai: { pha: "xong" as const, duLieu: theoCot[ma] ?? trang([]) },
  }));
}

function veBang(cot: readonly CotKanban[]): string {
  return renderToStaticMarkup(
    <BangKanban
      cot={cot}
      danhMuc={DANH_MUC}
      bayGio={BAY_GIO}
      maDangMo={null}
      moNhiemVu={() => {}}
    />,
  );
}

describe("năm cột §4.1 — và nhãn cột KHÁC nhãn §6 ở đúng một ô", () => {
  it("đủ năm cột chính, không có cột nào cho hai trạng thái rẽ nhánh", () => {
    const html = veBang(namCot());
    for (const ma of TRANG_THAI_CHINH) {
      expect(html).toContain(`id="cot-kanban-${ma}"`);
    }
    expect(html).not.toContain('id="cot-kanban-tam-dung"');
    expect(html).not.toContain('id="cot-kanban-chuyen-tiep"');
  });

  it("cột đầu tiên tên `Chưa thực hiện`, KHÔNG phải `Mới giao`", () => {
    // §6 ghi thẳng sự lệch ấy, và §4.1 lẫn §12 đều đếm "Chưa thực hiện". Dùng một nhãn cho cả hai
    // chỗ là làm sai một trong hai màn — mà cả hai vẫn chạy, nên không có gì đỏ ngoài bài này.
    const html = veBang(namCot());
    expect(html).toContain("Chưa thực hiện");
    expect(html).not.toContain("Mới giao");
    expect(nhanCotKanban("moi-giao")).toBe("Chưa thực hiện");
  });

  it("câu nói ra rằng Tạm dừng và Chuyển tiếp không hiện ở bảng này", () => {
    // Không có câu này thì một việc vừa sang `tam-dung` biến mất khỏi Kanban không dấu vết, và
    // người giao việc kết luận nhiệm vụ đã bị xoá.
    expect(veBang(namCot())).toContain(nhuTrongHTML(GHI_CHU_KANBAN_RE_NHANH));
  });
});

describe("con số đầu cột — số THẺ ĐÃ TẢI, không phải tổng số việc", () => {
  it("`has_more` biến con số thành `20+`", () => {
    // `page.Result` không mang tổng số. Một con số trần đọc ra là "cột này có 20 việc" trong khi
    // sự thật là "ít nhất 20", và con số ấy đi thẳng vào một câu báo cáo với lãnh đạo.
    expect(nhanDemCot(20, true)).toBe("20+");
    expect(nhanDemCot(7, false)).toBe("7");
    expect(nhanDemCot(0, false)).toBe("0");
  });

  it("dấu + ra tới trang, và câu giải thích đứng cùng bảng", () => {
    const html = veBang(
      namCot({ "dang-thuc-hien": trang([nhiemVu(), nhiemVu({ code: "NV20" })], true) }),
    );
    expect(html).toContain(">2+<");
    expect(html).toContain(nhuTrongHTML(GHI_CHU_DEM_COT));
  });
});

describe("ba lý do khiến một cột rỗng — ba màn hình khác nhau", () => {
  it("xã không có việc nào ở cột ấy: câu `Không có nhiệm vụ`, không phải khoảng trắng", () => {
    const html = veBang(namCot());
    expect(html).toContain(nhuTrongHTML(COT_RONG));
  });

  it("lượt đọc một cột HỎNG: câu nguyên văn của máy chủ hiện ở đúng cột ấy", () => {
    // Nuốt câu ấy thành một khoảng trắng là biến "không đọc được" thành "không có việc nào" — hai
    // câu trả lời ngược nhau, và cái sai là cái yên tâm hơn.
    const cau = "không đủ quyền: thiếu task.read";
    const cot: readonly CotKanban[] = TRANG_THAI_CHINH.map((ma) => ({
      ma,
      tai:
        ma === "cho-duyet"
          ? { pha: "loi" as const, thongBao: cau }
          : { pha: "xong" as const, duLieu: trang([]) },
    }));
    const html = veBang(cot);
    expect(html).toContain(nhuTrongHTML(cau));
    expect(html).toContain('role="alert"');
    // Bốn cột kia VẪN LÀ SỔ: một cột hỏng không được kéo cả bảng thành một trang lỗi.
    expect(html).toContain(nhuTrongHTML(COT_RONG));
  });

  it("bộ lọc Trạng thái chọn một trạng thái rẽ nhánh: nói ra, không vẽ năm cột rỗng", () => {
    // Năm cột rỗng ở đây đọc lên là "xã không có việc nào", đúng điều ngược lại với sự thật.
    const html = veBang([]);
    expect(html).toContain(nhuTrongHTML(CAU_LOC_TRANG_THAI_KHONG_CO_COT));
    expect(html).not.toContain("cot-kanban-");
    expect(html).not.toContain(nhuTrongHTML(COT_RONG));
  });

  it("`cotPhaiDoc` — bộ lọc Trạng thái quyết định cột nào còn phải đọc", () => {
    expect(cotPhaiDoc(undefined)).toEqual(TRANG_THAI_CHINH);
    expect(cotPhaiDoc("")).toEqual(TRANG_THAI_CHINH);
    expect(cotPhaiDoc("cho-duyet")).toEqual(["cho-duyet"]);
    // Hai trạng thái rẽ nhánh và một mã lạ đều KHÔNG có cột — mảng rỗng là câu trả lời đúng.
    expect(cotPhaiDoc("tam-dung")).toEqual([]);
    expect(cotPhaiDoc("chuyen-tiep")).toEqual([]);
    expect(cotPhaiDoc("mot-ma-la")).toEqual([]);
  });
});

describe("thẻ nhiệm vụ §4.1", () => {
  function veThe(sua: Partial<petitions_nhiemVuRa> = {}, maDangMo: string | null = null): string {
    return renderToStaticMarkup(
      <TheNhiemVu
        nhiemVu={nhiemVu(sua)}
        danhMuc={DANH_MUC}
        bayGio={BAY_GIO}
        maDangMo={maDangMo}
        moNhiemVu={() => {}}
      />,
    );
  }

  it("mã, tiêu đề, hạn trễ tô lệch, và người thực hiện", () => {
    const html = veThe();
    expect(html).toContain("NV19");
    expect(html).toContain("Trễ 86 ngày");
    expect(html).toContain('class="nhan-lech"');
    expect(html).toContain("CB-2026-3H8N2W");
  });

  it("không có hạn thì `Hạn —`, không phải một ô trống và không phải `Trễ 0 ngày`", () => {
    const html = veThe({ due_at: null, original_due_at: null });
    expect(html).toContain("Hạn —");
    expect(html).not.toContain("Trễ");
    expect(html).not.toContain('class="nhan-lech"');
  });

  it("MỨC ƯU TIÊN HIỆN THÀNH CHỮ — đặc tả mã hoá nó bằng màu viền, màu một mình là con số không", () => {
    // a11y: màu không bao giờ là tín hiệu duy nhất. Khi lớp CSS viền trái được thêm, nó chồng lên
    // chữ này chứ không thay chữ này.
    expect(veThe()).toContain("Cao");
  });

  it("chưa phân công là một TRẠNG THÁI THẬT, không phải dấu gạch", () => {
    expect(veThe({ assignee: "" })).toContain(nhuTrongHTML(CHUA_PHAN_CONG));
  });

  it("chip `Hoàn thành trễ hạn` so với HẠN BAN ĐẦU, không với hạn hiện tại", () => {
    // So với `due_at` thì một lần lùi hạn được duyệt tự xoá dấu vết của chính nó khỏi báo cáo.
    const html = veThe({
      status: "hoan-thanh",
      due_at: "2026-08-30T23:59:59+07:00",
      original_due_at: "2026-06-20T23:59:59+07:00",
      completed_at: "2026-08-25T02:00:00Z",
    });
    expect(html).toContain("Hoàn thành trễ hạn");
  });

  it("thẻ mở drawer — đó là lối đổi trạng thái của Kanban", () => {
    expect(veThe()).toContain("Mở NV19");
    expect(veThe({}, "NV19")).toContain("Đang mở");
  });

  it("KHÔNG kéo-thả trang trí: không thuộc tính `draggable` nào trên thẻ", () => {
    // VẾ CHỊU LỰC. Một thẻ `draggable` mà thả xuống không gọi tuyến nào là một thao tác trông như
    // đã đổi trạng thái và không đổi gì — cán bộ tin việc đã chuyển, máy chủ không biết gì cả.
    const html = veBang(namCot({ "dang-thuc-hien": trang([nhiemVu()]) }));
    expect(html).not.toContain("draggable");
    expect(html).not.toContain("ondrop");
  });

  it("KHÔNG vẽ ô tick chọn hàng loạt — `Xoá đã chọn` không có tuyến nào", () => {
    const html = veBang(namCot({ "dang-thuc-hien": trang([nhiemVu()]) }));
    expect(html).not.toContain('type="checkbox"');
  });

  it("KHÔNG vẽ chip `{n} việc con` — phản hồi không mang số ấy", () => {
    const html = veBang(namCot({ "dang-thuc-hien": trang([nhiemVu()]) }));
    expect(html).not.toContain("việc con");
  });
});

describe("phần chưa dựng được của lượt này ra tới danh sách, không nằm trong chú thích mã", () => {
  it("kéo-thả được khai là KHÔNG dựng, và lý do là lối bàn phím", () => {
    const keoTha = PHAN_CHUA_DUNG.find((p) => p.ten.includes("KÉO-THẢ"));
    expect(keoTha).toBeDefined();
    expect(keoTha?.viSao).toContain("bàn phím");
    // Và nó chỉ đúng chỗ đổi trạng thái thay thế, cùng tuyến.
    expect(keoTha?.viSao).toContain("/status");
  });

  it("con số thật của cột và chế độ xem thứ ba đều được khai", () => {
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("SỐ LƯỢNG THẬT"))).toBe(true);
    const soTheoDoi = PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("Chế độ xem `Sổ theo dõi`"));
    expect(soTheoDoi).toBeDefined();
    // SỬA CÓ CHỦ Ý 24/09/2026: lý do cũ ("bảng `nhiem_vu_van_ban` chưa tồn tại") đã sai từ khi
    // bảng có. Bài này nay canh LÝ DO THẬT — tuyến sổ không trả `documents` — chứ không chỉ canh
    // rằng mục ấy có mặt, vì một mục có mặt với lý do sai vẫn xanh ở phép kiểm cũ.
    expect(soTheoDoi?.viSao).toContain("`GET /api/v1/tasks`");
    expect(soTheoDoi?.viSao).not.toContain("chưa tồn tại");
  });
});
