import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type {
  page_Result_petitions_nhiemVuRa,
  petitions_nhiemVuRa,
  petitions_taskCountsOut,
} from "@/lib/api/schema.gen";

import {
  BANG_NHAN_MAC_DINH,
  CAU_LOC_TRANG_THAI_KHONG_CO_COT,
  CHUA_PHAN_CONG,
  COT_RONG,
  KANBAN_COUNTS_ERROR,
  MOI_TRANG_THAI,
  PHAN_CHUA_DUNG,
  TRANG_THAI_CHINH,
  childCountLabel,
  cotPhaiDoc,
  ghiChuKanbanReNhanh,
  kanbanColumnCount,
  kanbanPartialNote,
} from "./nhan-nhiem-vu";
import { BangKanban, TheNhiemVu, priorityStripClass, type CotKanban, type DanhMucNhiemVu } from "./so-nhiem-vu";
import type { TrangThaiTai } from "./so-nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)

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
    child_count: 0,
    allowed_transitions: [],
    updated_at: "2026-06-01T02:00:00Z",
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

/** All seven codes, as `GET /api/v1/task-counts` promises. */
function allCounts(byStatus: Partial<Record<string, number>> = {}): petitions_taskCountsOut {
  return { by_status: MOI_TRANG_THAI.map((status) => ({ status, count: byStatus[status] ?? 0 })) };
}

function veBang(
  cot: readonly CotKanban[],
  counts: TrangThaiTai<petitions_taskCountsOut> = { pha: "xong", duLieu: allCounts() },
): string {
  return renderToStaticMarkup(
    <BangKanban
      cot={cot}
      danhMuc={DANH_MUC}
      nhanTT={BANG_NHAN_MAC_DINH}
      bayGio={BAY_GIO}
      maDangMo={null}
      moNhiemVu={() => {}}
      counts={counts}
    />,
  );
}

describe("năm cột §4.1", () => {
  it("đủ năm cột chính, không có cột nào cho hai trạng thái rẽ nhánh", () => {
    const html = veBang(namCot());
    for (const ma of TRANG_THAI_CHINH) {
      expect(html).toContain(`id="cot-kanban-${ma}"`);
    }
    expect(html).not.toContain('id="cot-kanban-tam-dung"');
    expect(html).not.toContain('id="cot-kanban-chuyen-tiep"');
  });

  it("tên cột là nhãn của BẢNG NHÃN truyền vào — không còn nhãn Kanban riêng", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 24/09/2026 (#21): bài này từng canh "Chưa thực hiện" nung trong màn hình.
    // Nay chữ ấy là thứ xã tự đặt cho `moi-giao`; với bảng mặc định cột đầu là "Mới giao". Ca xã đã
    // đổi nhãn nằm ở `nhan-trang-thai-xa.test.tsx`.
    const html = veBang(namCot());
    expect(html).toContain("Mới giao");
    expect(html).not.toContain("Chưa thực hiện");
  });

  it("layout của bản mẫu: cột nằm thẳng trên trang, không trong khung trắng `.bang-cuon` (07/10/2026)", () => {
    // Inside `.bang-cuon`'s white frame the columns showed a white band between every two of them.
    const html = veBang(namCot());
    expect(html).not.toContain("bang-cuon");
    expect(html).toContain('<div class="bang-kanban xl:grid-cols-5">');
    expect(html).toMatch(/<section class="cot-kanban" aria-labelledby="cot-kanban-moi-giao">/);
    // The count is the prototype's small white tag, not the `.chip` pill whose unlayered padding won.
    expect(html).not.toMatch(/class="chip[^"]*">0<\/span>/);
    expect(html).toContain('class="kanban-count ml-auto rounded-[10px]');
    // Header = decorative dot + the commune's label; the dot is hidden from screen readers.
    expect(html).toMatch(
      /<h3 id="cot-kanban-dang-thuc-hien"[^>]*><span class="size-2 shrink-0 rounded-full bg-\[#2fb1f9\]" aria-hidden="true"><\/span>/,
    );
  });

  it("câu nói ra rằng Tạm dừng và Chuyển tiếp không hiện ở bảng này", () => {
    // Không có câu này thì một việc vừa sang `tam-dung` biến mất khỏi Kanban không dấu vết, và
    // người giao việc kết luận nhiệm vụ đã bị xoá.
    expect(veBang(namCot())).toContain(nhuTrongHTML(ghiChuKanbanReNhanh(BANG_NHAN_MAC_DINH)));
  });
});

describe("(#15) con số đầu cột — TỔNG THẬT từ `/task-counts`, không phải số thẻ đã tải", () => {
  it("`kanbanColumnCount` đọc đúng mã; mã vắng mặt là `null`, không phải 0", () => {
    const d = allCounts({ "cho-duyet": 57 });
    expect(kanbanColumnCount(d.by_status, "cho-duyet")).toBe(57);
    expect(kanbanColumnCount(d.by_status, "moi-giao")).toBe(0);
    expect(kanbanColumnCount([], "moi-giao")).toBeNull();
  });

  it("đầu cột hiện 57 dù chỉ 2 thẻ đã tải — và dưới cột nói ra phần còn lại", () => {
    const html = veBang(
      namCot({ "dang-thuc-hien": trang([nhiemVu(), nhiemVu({ code: "NV20" })], true) }),
      { pha: "xong", duLieu: allCounts({ "dang-thuc-hien": 57 }) },
    );
    expect(html).toMatch(/<span class="kanban-count[^"]*">57<\/span>/);
    expect(html).not.toContain(">2+<");
    expect(html).toContain(nhuTrongHTML(kanbanPartialNote(2, 57)));
    // The old "cards loaded, not a total" disclaimer is gone — it would now be false.
    expect(html).not.toContain("không phải tổng số việc của cột");
  });

  it("đang đọc số: chưa có chip số nào — không vẽ một số chưa biết", () => {
    const html = veBang(namCot(), { pha: "dangTai" });
    expect(html).not.toContain("kanban-count");
    expect(html).not.toContain(KANBAN_COUNTS_ERROR);
  });

  it("đọc số HỎNG: đầu cột `—`, câu máy chủ nguyên văn, và các thẻ VẪN hiện", () => {
    const cau = "không đủ quyền: thiếu task.read";
    const html = veBang(namCot({ "dang-thuc-hien": trang([nhiemVu()], true) }), {
      pha: "loi",
      thongBao: cau,
    });
    expect(html).toMatch(/<span class="kanban-count[^"]*">—<\/span>/);
    expect(html).not.toMatch(/<span class="kanban-count[^"]*">0<\/span>/);
    expect(html).toContain(`role="alert">${KANBAN_COUNTS_ERROR} ${nhuTrongHTML(cau)}</p>`);
    expect(html).toContain('aria-labelledby="the-nhiem-vu-NV19"');
    // `has_more` still says the column goes on, even without a total.
    expect(html).toContain(nhuTrongHTML(kanbanPartialNote(1, null)));
  });

  it("thứ tự cột vẫn theo bảng nhãn của xã (`/task-statuses`), không theo thứ tự trả về của số", () => {
    const d = allCounts();
    const html = veBang(namCot(), { pha: "xong", duLieu: { by_status: [...d.by_status].reverse() } });
    const vi = TRANG_THAI_CHINH.map((ma) => html.indexOf(`id="cot-kanban-${ma}"`));
    expect([...vi].sort((a, b) => a - b)).toEqual(vi);
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
        nhanTT={BANG_NHAN_MAC_DINH}
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
    expect(html).toMatch(/class="[^"]*\bnhan-lech\b[^"]*"/);
    expect(html).toContain("CB-2026-3H8N2W");
  });

  it("prototype card (06/10/2026): priority strip on top, then code → title → deadline → assignee", () => {
    const html = veThe({ child_count: 2 });
    const at = (s: string) => html.indexOf(s);
    // Rank 0 of the commune's scale is the top of the scale — the red strip; colour is never alone.
    expect(html).toMatch(/<div class="h-\[3px\] rounded-t-\[10px\] bg-[a-z0-9-]+" aria-hidden="true">/);
    expect(at("h-[3px]")).toBeLessThan(at(">NV19<"));
    expect(at(">NV19<")).toBeLessThan(at("Báo cáo tổng kết"));
    // Deadline and sub-task count share ONE meta line (equal card heights, 06/10/2026).
    expect(at("Báo cáo tổng kết")).toBeLessThan(at("Trễ 86 ngày"));
    expect(at("Trễ 86 ngày")).toBeLessThan(at("2 việc con"));
    expect(at("2 việc con")).toBeLessThan(at("CB-2026-3H8N2W"));
    // The title reserves and clamps to two lines; the full title stays on hover.
    expect(html).toContain("min-h-[2.75em]");
    expect(html).toContain('title="Báo cáo tổng kết');
    // No extension count: the list contract carries none (schema.gen.ts `petitions_nhiemVuRa`).
    expect(html).not.toContain("đã gia hạn");
  });

  it("`priorityStripClass` follows the commune's RANK, not a hard-coded code", () => {
    const scale = [{ code: "a" }, { code: "b" }, { code: "c" }];
    expect(priorityStripClass(scale, "a")).toBe("bg-danger-500");
    expect(priorityStripClass(scale, "b")).toBe("bg-warning-500");
    expect(priorityStripClass(scale, "c")).toBe("bg-brand-500");
    expect(priorityStripClass(scale, "")).toBe("bg-line-strong");
    expect(priorityStripClass(scale, "khong-co")).toBe("bg-line-strong");
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

  it("danh bạ có người ấy: thẻ hiện HỌ TÊN; mã không có trong danh bạ: thẻ hiện MÃ, không để trống", () => {
    const danhBa = new Map([
      [
        "CB-2026-3H8N2W",
        { code: "CB-2026-3H8N2W", full_name: "Huỳnh Văn Ba", position: "", department_id: "" },
      ],
    ]);
    const ve = (assignee: string) =>
      renderToStaticMarkup(
        <TheNhiemVu
          nhiemVu={nhiemVu({ assignee })}
          danhMuc={DANH_MUC}
          danhBa={danhBa}
          nhanTT={BANG_NHAN_MAC_DINH}
          bayGio={BAY_GIO}
          maDangMo={null}
          moNhiemVu={() => {}}
        />,
      );
    const coTen = ve("CB-2026-3H8N2W");
    expect(coTen).toContain("Huỳnh Văn Ba");
    expect(coTen).not.toContain("CB-2026-3H8N2W");
    expect(ve("CB-2019-NGHIHUU")).toContain(">CB-2019-NGHIHUU</span>");
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

  it("thẻ mở drawer — cả thân thẻ là MỘT nút (prototype), `aria-expanded` nói thẻ nào đang mở", () => {
    expect(veThe()).toMatch(/<button type="button" class="[^"]*" aria-expanded="false"><span class="ma-muc/);
    expect(veThe({}, "NV19")).toContain('aria-expanded="true"');
  });

  it("bảng CHỈ ĐỌC (không `move`): không `draggable`, không nút chuyển cột", () => {
    // ĐỔI CÓ CHỦ Ý 28/09/2026 (TASK-02 lượt web 1): kéo-thả nay CÓ, nhưng chỉ khi bên gọi trao
    // `move`. Vế chịu lực giữ nguyên: một thẻ `draggable` mà thả xuống không gọi tuyến nào là một
    // thao tác trông như đã đổi trạng thái và không đổi gì. Ca có `move`: `kanban-move.test.tsx`.
    const html = veBang(namCot({ "dang-thuc-hien": trang([nhiemVu()]) }));
    expect(html).not.toContain("Chuyển sang cột…");
    expect(html).not.toContain("draggable");
    expect(html).not.toContain("ondrop");
  });

  it("KHÔNG vẽ ô tick chọn hàng loạt — `Xoá đã chọn` không có tuyến nào", () => {
    const html = veBang(namCot({ "dang-thuc-hien": trang([nhiemVu()]) }));
    expect(html).not.toContain('type="checkbox"');
  });

  it("(#6) chip `{n} việc con` CHỈ khi `child_count > 0` — số do máy chủ đếm", () => {
    expect(childCountLabel(0)).toBeNull();
    expect(childCountLabel(1)).toBe("1 việc con");
    expect(childCountLabel(3)).toBe("3 việc con");
    expect(veThe({ child_count: 3 })).toMatch(/<\/svg>3 việc con<\/span>/);
    // Zero children: no chip at all — not "0 việc con".
    expect(veThe({ child_count: 0 })).not.toContain("việc con");
  });
});

describe("phần chưa dựng được của lượt này ra tới danh sách, không nằm trong chú thích mã", () => {
  it("kéo-thả ĐÃ DỰNG kèm lối bàn phím: mục cũ rời danh sách", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (TASK-02 lượt web 1): bài này từng canh mục "KÉO-THẢ … KHÔNG
    // dựng". Nay có kéo-thả VÀ nút `Chuyển sang cột…`; một mục còn nằm đó sau khi đã dựng là mục
    // đẩy người sau đi dựng lại thứ đã có.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.includes("KÉO-THẢ"))).toBeUndefined();
  });

  it("con số thật của cột ĐÃ DỰNG (#15) — mục cũ rời; chế độ xem thứ ba ĐÃ DỰNG (W6) — mục ấy rời", () => {
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("SỐ LƯỢNG THẬT"))).toBe(false);
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W6): ca này ghim mục `Chế độ xem Sổ theo dõi` CÓ MẶT với lý do
    // "tuyến sổ không trả `documents`". `include=documents` (90d12ff) và màn Sổ theo dõi nay có.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("Chế độ xem `Sổ theo dõi`"))).toBeUndefined();
  });
});
