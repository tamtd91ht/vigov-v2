import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_deNghiLuiHanRa,
  petitions_nhiemVuRa,
  petitions_trangThaiNhiemVuRa,
} from "@/lib/api/schema.gen";

import {
  BANG_NHAN_MAC_DINH,
  CANH_BAO_NHAN_MAC_DINH,
  PHAN_CHUA_DUNG,
  TRANG_THAI_CHINH,
  docBangNhanTrangThai,
  ghiChuKanbanReNhanh,
  nhanHoanThanhTreHan,
  type BangNhanTrangThai,
} from "./nhan-nhiem-vu";
import {
  BangKanban,
  BangNhiemVu,
  CanhBaoNhanTrangThai,
  ChiTietNhiemVu,
  HangLoc,
  TheNhiemVu,
  type CotKanban,
  type DanhMucNhiemVu,
} from "./so-nhiem-vu";

/**
 * Màn Nhiệm vụ ĐỌC nhãn và thứ tự bảy trạng thái từ máy chủ (quyết định #21, 24/09/2026).
 *
 * LỖI CANH Ở ĐÂY LÀ LỖI IM LẶNG NHẤT CỦA LƯỢT NÀY: một chỗ nào đó của màn hình còn đọc nhãn mặc định
 * thay vì bảng của xã. Xã đã đổi "Mới giao" thành "Chưa thực hiện" ở tab Danh mục, lưu thành công,
 * và cột Kanban vẫn hiện "Mới giao" — không gì đỏ, vì chữ mặc định là một chữ trông hoàn toàn đúng.
 * Nên mọi bài dưới đây dùng một nhãn KHÔNG có trong bảng mặc định.
 */

function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/** Bảy dòng như máy chủ trả. Xã đổi nhãn `moi-giao`, `tam-dung`, và đưa `cho-duyet` lên trước. */
function bayDong(): petitions_trangThaiNhiemVuRa[] {
  const d = (
    code: string,
    label: string,
    order: number,
    role: string,
    default_label: string,
    default_order: number,
  ): petitions_trangThaiNhiemVuRa => ({
    code,
    label,
    order,
    role,
    default_label,
    default_order,
    customised: label !== default_label || order !== default_order,
  });
  return [
    d("moi-giao", "Việc mới về xã", 1, "chinh", "Mới giao", 1),
    d("da-tiep-nhan", "Đã tiếp nhận", 2, "chinh", "Đã tiếp nhận", 2),
    d("cho-duyet", "Chờ duyệt", 3, "chinh", "Chờ duyệt", 4),
    d("dang-thuc-hien", "Đang thực hiện", 3, "chinh", "Đang thực hiện", 3),
    d("hoan-thanh", "Hoàn thành", 5, "chinh", "Hoàn thành", 5),
    d("tam-dung", "Tạm hoãn", 6, "re-nhanh", "Tạm dừng", 6),
    d("chuyen-tiep", "Chuyển tiếp", 7, "re-nhanh", "Chuyển tiếp", 7),
  ];
}

const DOC_DUOC: KetQua<petitions_danhSachTrangThaiNhiemVuRa> = {
  ok: true,
  duLieu: { items: bayDong() },
};

function bangXa(): BangNhanTrangThai {
  const { bang, canhBao } = docBangNhanTrangThai(DOC_DUOC);
  expect(canhBao).toBeNull();
  return bang;
}

function nhiemVu(sua: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Rà soát hộ nghèo",
    description: "",
    status: "moi-giao",
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: "",
    lead_unit: "",
    monitor: "",
    due_at: null,
    original_due_at: null,
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

const DANH_MUC: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };
const BAY_GIO = new Date("2026-09-15T03:00:00Z");
const KHONG_GOI = (): Promise<KetQua<petitions_deNghiLuiHanRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi" });
const KHONG_SUA = (): Promise<KetQua<petitions_nhiemVuRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi" });

function namCot(): readonly CotKanban[] {
  return TRANG_THAI_CHINH.map((ma) => ({
    ma,
    tai: { pha: "xong" as const, duLieu: { items: [], next_cursor: "", has_more: false } },
  }));
}

function veKanban(nhanTT: BangNhanTrangThai): string {
  return renderToStaticMarkup(
    <BangKanban
      cot={namCot()}
      danhMuc={DANH_MUC}
      nhanTT={nhanTT}
      bayGio={BAY_GIO}
      maDangMo={null}
      moNhiemVu={() => {}}
    />,
  );
}

function veChiTiet(nhanTT: BangNhanTrangThai, status: string): string {
  return renderToStaticMarkup(
    <ChiTietNhiemVu
      nhiemVu={nhiemVu({ status })}
      vanBan={{ pha: "dangTai" }}
      danhMuc={DANH_MUC}
      nhanTT={nhanTT}
      tenBoPhan={new Map()}
      bayGio={BAY_GIO}
      maNguoiDangNhap=""
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

describe("docBangNhanTrangThai — ba nhánh, không nhánh nào lặng lẽ", () => {
  it("đọc được: nhãn và thứ tự LÀ của máy chủ, nguyên văn, không sắp lại", () => {
    const bang = bangXa();
    expect(bang.nhan["moi-giao"]).toBe("Việc mới về xã");
    expect(bang.thuTu).toEqual([
      "moi-giao",
      "da-tiep-nhan",
      "cho-duyet",
      "dang-thuc-hien",
      "hoan-thanh",
      "tam-dung",
      "chuyen-tiep",
    ]);
  });

  it("chưa đọc xong: đường lui, KHÔNG cảnh báo — cả màn còn đang tải", () => {
    expect(docBangNhanTrangThai(null)).toEqual({ bang: BANG_NHAN_MAC_DINH, canhBao: null });
  });

  it("đọc hỏng: đường lui KÈM cảnh báo và câu máy chủ nguyên văn", () => {
    const { bang, canhBao } = docBangNhanTrangThai({ ok: false, thongBao: "Máy chủ bận." });
    expect(bang).toBe(BANG_NHAN_MAC_DINH);
    expect(canhBao).toContain(CANH_BAO_NHAN_MAC_DINH);
    expect(canhBao).toContain("Máy chủ bận.");
  });

  it("máy chủ trả THIẾU một mã: đường lui kèm cảnh báo — không ghép nửa xã nửa mặc định", () => {
    const thieu = bayDong().filter((d) => d.code !== "hoan-thanh");
    const { bang, canhBao } = docBangNhanTrangThai({ ok: true, duLieu: { items: thieu } });
    expect(bang).toBe(BANG_NHAN_MAC_DINH);
    expect(canhBao).toBe(CANH_BAO_NHAN_MAC_DINH);
  });

  it("mã ngoài bảy bị bỏ qua — danh sách mã là đóng", () => {
    const them = [...bayDong(), { ...(bayDong()[0] as petitions_trangThaiNhiemVuRa), code: "da-ban-giao" }];
    const { bang, canhBao } = docBangNhanTrangThai({ ok: true, duLieu: { items: them } });
    expect(canhBao).toBeNull();
    expect(bang.thuTu).not.toContain("da-ban-giao");
  });

  it("câu cảnh báo RA TỚI TRANG khi đọc hỏng, và vắng mặt khi đọc được", () => {
    const hong = docBangNhanTrangThai({ ok: false, thongBao: "Máy chủ bận." });
    const html = renderToStaticMarkup(<CanhBaoNhanTrangThai canhBao={hong.canhBao} />);
    expect(html).toContain(nhuTrongHTML(CANH_BAO_NHAN_MAC_DINH));
    expect(html).toContain('role="status"');
    expect(renderToStaticMarkup(<CanhBaoNhanTrangThai canhBao={null} />)).toBe("");
  });
});

describe("màn Nhiệm vụ dùng NHÃN CỦA XÃ ở mọi chỗ hiện trạng thái", () => {
  it("tên cột Kanban là nhãn của xã", () => {
    const html = veKanban(bangXa());
    expect(html).toContain("Việc mới về xã");
    expect(html).not.toContain(">Mới giao");
  });

  it("thứ tự cột Kanban theo `order` của xã — năm cột chính, không thêm cột rẽ nhánh", () => {
    const html = veKanban(bangXa());
    expect(html.indexOf('id="cot-kanban-cho-duyet"')).toBeLessThan(
      html.indexOf('id="cot-kanban-dang-thuc-hien"'),
    );
    expect(html).not.toContain('id="cot-kanban-tam-dung"');
    // Với bảng mặc định: đúng thứ tự vòng đời.
    const macDinh = veKanban(BANG_NHAN_MAC_DINH);
    expect(macDinh.indexOf('id="cot-kanban-dang-thuc-hien"')).toBeLessThan(
      macDinh.indexOf('id="cot-kanban-cho-duyet"'),
    );
  });

  it("câu dưới Kanban gọi hai trạng thái rẽ nhánh bằng nhãn của xã", () => {
    const html = veKanban(bangXa());
    expect(ghiChuKanbanReNhanh(bangXa())).toContain("Tạm hoãn");
    expect(html).toContain(nhuTrongHTML(ghiChuKanbanReNhanh(bangXa())));
  });

  it("chip trạng thái ở bảng Danh sách", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu()]}
        danhMuc={DANH_MUC}
        nhanTT={bangXa()}
        tenBoPhan={new Map()}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain(">Việc mới về xã</span>");
  });

  it("ô lọc Trạng thái: nhãn của xã, theo thứ tự của xã", () => {
    const html = renderToStaticMarkup(
      <HangLoc
        loc={{}}
        tim=""
        datTim={() => {}}
        datLoc={() => {}}
        danhMuc={DANH_MUC}
        nhanTT={bangXa()}
      />,
    );
    expect(html).toContain('<option value="moi-giao">Việc mới về xã</option>');
    expect(html).toContain('<option value="tam-dung">Tạm hoãn</option>');
    // GIÁ TRỊ vẫn là MÃ — bộ lọc gửi mã lên máy chủ, không gửi nhãn.
    expect(html.indexOf('value="cho-duyet"')).toBeLessThan(html.indexOf('value="dang-thuc-hien"'));
  });

  it("dải bước và nút chuyển trạng thái trong drawer", () => {
    const html = veChiTiet(bangXa(), "moi-giao");
    expect(html).toContain(">Việc mới về xã</span>");
    expect(html).toContain("Chuyển sang Tạm hoãn");
    // DẢI BƯỚC GIỮ THỨ TỰ VÒNG ĐỜI §6, không theo `order`: `dang-thuc-hien` vẫn đứng trước
    // `cho-duyet` dù xã đã đưa `cho-duyet` lên trước ở Kanban.
    const buoc = html.slice(html.indexOf("<ol"), html.indexOf("</ol>"));
    expect(buoc.indexOf("Đang thực hiện")).toBeLessThan(buoc.indexOf("Chờ duyệt"));
  });
});

/**
 * BẢNG MÀ CẢ BẢY NHÃN ĐỀU ĐÃ ĐỔI. Các ca ở trên chỉ đổi hai nhãn (`moi-giao`, `tam-dung`), nên một
 * chỗ vẽ đọc nhãn MẶC ĐỊNH cho năm mã kia vẫn in ra đúng chữ mà ca mong đợi — xanh sai lý do. Ở
 * đây không một nhãn mặc định nào còn đúng, nên chữ mặc định xuất hiện ở bất kỳ đâu là một chỗ vẽ
 * đã bỏ qua bảng của xã.
 */
function bangDoiHet(): BangNhanTrangThai {
  const items = bayDong().map((d) => ({ ...d, label: `Xã đặt ${d.code}`, customised: true }));
  const { bang, canhBao } = docBangNhanTrangThai({ ok: true, duLieu: { items } });
  expect(canhBao).toBeNull();
  return bang;
}

/** Nhãn mặc định đứng một mình như chữ của một phần tử — `>Tạm dừng<`, hoặc sau "Chuyển sang ". */
function chuMacDinhLot(html: string): string[] {
  return Object.values(BANG_NHAN_MAC_DINH.nhan).filter(
    (nhan) => html.includes(`>${nhan}<`) || html.includes(`sang ${nhan}<`) || html.includes(`— ${nhan} `),
  );
}

describe("không chỗ vẽ nào còn đọc nhãn mặc định khi xã đã đổi CẢ BẢY", () => {
  it("hàng Rẽ nhánh trong drawer dùng nhãn xã — và chip sáng đúng trạng thái hiện tại", () => {
    // Ca "dải bước" ở trên chỉ nhìn `Chuyển sang Tạm hoãn` — chữ của NÚT. Hàng `Rẽ nhánh:` là chỗ
    // vẽ thứ hai của cùng mã ấy, và trước ca này nó có thể đọc bảng mặc định mà không gì đỏ.
    const html = veChiTiet(bangXa(), "tam-dung");
    expect(html).toContain('class="chip chip-hoat-dong">Tạm hoãn</span>');
    expect(html).not.toContain(">Tạm dừng<");
  });

  for (const ma of ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet", "hoan-thanh", "tam-dung", "chuyen-tiep"]) {
    it(`drawer ở trạng thái ${ma}: dải bước, hàng Rẽ nhánh, nút chuyển — không một nhãn mặc định`, () => {
      const html = veChiTiet(bangDoiHet(), ma);
      expect(chuMacDinhLot(html)).toEqual([]);
      expect(html).toContain(`class="chip chip-hoat-dong">Xã đặt ${ma}</span>`);
    });
  }

  it("Kanban: năm tiêu đề cột và câu rẽ nhánh — không một nhãn mặc định", () => {
    const html = veKanban(bangDoiHet());
    expect(chuMacDinhLot(html)).toEqual([]);
    for (const ma of TRANG_THAI_CHINH) expect(html).toContain(`Xã đặt ${ma}`);
    expect(html).toContain("Xã đặt tam-dung");
    expect(html).toContain("Xã đặt chuyen-tiep");
  });

  it("bảng Danh sách: chip của MỖI mã, không chỉ `moi-giao`", () => {
    const bang = bangDoiHet();
    const ma7 = bang.thuTu;
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={ma7.map((status, i) => nhiemVu({ code: `NV${i + 1}`, status }))}
        danhMuc={DANH_MUC}
        nhanTT={bang}
        tenBoPhan={new Map()}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(chuMacDinhLot(html)).toEqual([]);
    for (const ma of ma7) expect(html).toContain(`>Xã đặt ${ma}</span>`);
  });

  it("ô lọc: bảy lựa chọn đều là nhãn xã", () => {
    const html = renderToStaticMarkup(
      <HangLoc loc={{}} tim="" datTim={() => {}} datLoc={() => {}} danhMuc={DANH_MUC} nhanTT={bangDoiHet()} />,
    );
    expect(chuMacDinhLot(html)).toEqual([]);
    for (const ma of BANG_NHAN_MAC_DINH.thuTu) {
      expect(html).toContain(`<option value="${ma}">Xã đặt ${ma}</option>`);
    }
  });
});

/**
 * Chip `Hoàn thành trễ hạn` từng gõ cứng ở hai chỗ vẽ (thẻ Kanban, bảng Danh sách). Nó không lọt
 * lưới `chuMacDinhLot` vì chữ ấy không phải một nhãn đứng một mình — nên cần ca riêng.
 */
describe("chip hoàn thành trễ hạn theo nhãn `hoan-thanh` của xã", () => {
  const TRE = {
    status: "hoan-thanh",
    original_due_at: "2026-06-20T23:59:59+07:00",
    completed_at: "2026-08-25T02:00:00Z",
  } as const;

  it("xã đổi nhãn `hoan-thanh` → chip đổi theo", () => {
    expect(nhanHoanThanhTreHan(bangDoiHet())).toBe("Xã đặt hoan-thanh trễ hạn");
  });

  it("đọc nhãn hỏng → đường lui, đúng chữ cũ `Hoàn thành trễ hạn`", () => {
    const { bang } = docBangNhanTrangThai({ ok: false, thongBao: "Máy chủ bận." });
    expect(nhanHoanThanhTreHan(bang)).toBe("Hoàn thành trễ hạn");
  });

  it("thẻ Kanban và bảng Danh sách đều vẽ chữ theo nhãn xã", () => {
    const bang = bangDoiHet();
    const the = renderToStaticMarkup(
      <TheNhiemVu
        nhiemVu={nhiemVu(TRE)}
        danhMuc={DANH_MUC}
        nhanTT={bang}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    const ds = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu(TRE)]}
        danhMuc={DANH_MUC}
        nhanTT={bang}
        tenBoPhan={new Map()}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    for (const html of [the, ds]) {
      expect(html).toContain(">Xã đặt hoan-thanh trễ hạn</span>");
      expect(html).not.toContain("Hoàn thành trễ hạn");
    }
  });
});

describe("phần chưa dựng được — mục danh mục trạng thái đã dựng nên BIẾN KHỎI danh sách", () => {
  it("không còn mục nói hợp đồng thiếu tuyến trạng thái", () => {
    const moi = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(moi).not.toContain("Trạng thái nhiệm vụ` xã sửa được");
    expect(moi).not.toContain("không có tuyến phát ra danh mục trạng thái");
  });
});
