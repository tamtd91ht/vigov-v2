import { readFileSync } from "node:fs";

import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_danhSachLichSuChuyenRa,
  documents_lichSuChuyenRa,
  documents_vanBanDenRa,
} from "@/lib/api/schema.gen";

import { BAN_CHUYEN_TRONG, LICH_SU_RONG, type BanChuyen } from "./nhan-van-ban";
import { NganVanBanDen } from "./ngan-van-ban-den";
import { guiChuyenVanBan } from "./thao-tac-van-ban";

/**
 * Ngăn chi tiết một văn bản đến. Canh những điều một lần sửa MỘT DÒNG phá được mà `tsc` vẫn sạch:
 *
 *   1. Dòng thời gian giữ ĐÚNG thứ tự máy chủ trả — không sắp lại ở client.
 *   2. Dòng thời gian rỗng NÓI RA thành chữ, không vẽ một danh sách trống.
 *   3. 404 hiện NGUYÊN câu máy chủ, không hiện số hiệu, không hiện khối chuyển.
 *   4. Quá hạn SUY RA từ `due_at` và đồng hồ truyền vào (luật 10, bất biến 3).
 *   5. Thiếu `document.route` thì KHÔNG có khối chuyển — vế từ chối, không chỉ vế cho phép.
 *   6. `reason` không đi vào console, bộ nhớ trình duyệt hay URL (luật 3).
 */

const TRA_LOAI: BangTraDanhMuc = { pha: "xong", ten: new Map([["cong-van", "Công văn"]]) };
const TRA_BO_PHAN: BangTraDanhMuc = {
  pha: "xong",
  ten: new Map([
    ["01JBOPHANMOTCUA", "BỘ PHẬN MỘT CỬA"],
    ["01JBOPHANVPDU", "VĂN PHÒNG ĐẢNG UỶ"],
  ]),
};

const HAN = "2026-09-25T08:00:00Z";
const TRUOC_HAN = new Date("2026-09-24T00:00:00Z");
const SAU_HAN = new Date("2026-09-30T00:00:00Z");

function vanBan(sua: Partial<documents_vanBanDenRa> = {}): documents_vanBanDenRa {
  return {
    id: "01JVBDEN0000000000000001",
    number: 7,
    year: 2026,
    received_date: "2026-09-22",
    reference_no: "1742-CV/BTCTU",
    document_date: "2026-09-18",
    issuing_body: "Ban Tổ chức Tỉnh uỷ",
    document_type: "cong-van",
    summary: "Về việc rà soát hồ sơ cán bộ",
    urgency: "khan",
    holding_unit: "01JBOPHANVPDU",
    assignee: "",
    due_at: HAN,
    status: "da-phan-cong",
    created_by: "CB-00123",
    created_at: "2026-09-22T02:00:00Z",
    updated_at: "2026-09-22T02:00:00Z",
    ...sua,
  };
}

function dongLichSu(sua: Partial<documents_lichSuChuyenRa>): documents_lichSuChuyenRa {
  return {
    id: "01JLS000000000000000000001",
    document_id: "01JVBDEN0000000000000001",
    routed_at: "2026-09-22T03:00:00Z",
    routed_by: "CB-00123",
    status: "da-phan-cong",
    from_unit: "",
    to_unit: "01JBOPHANMOTCUA",
    assignee: "",
    reason: "Lý do lần một",
    created_at: "2026-09-22T03:00:00Z",
    ...sua,
  };
}

function lichSu(items: documents_lichSuChuyenRa[]): KetQua<documents_danhSachLichSuChuyenRa> {
  return { ok: true, duLieu: { items } };
}

function ve({
  vb = { ok: true, duLieu: vanBan() } as KetQua<documents_vanBanDenRa> | null,
  ls = lichSu([]) as KetQua<documents_danhSachLichSuChuyenRa> | null,
  bayGio = TRUOC_HAN,
  coQuyenChuyen = true,
  ban = BAN_CHUYEN_TRONG as BanChuyen,
  loi = "",
} = {}) {
  return renderToStaticMarkup(
    <NganVanBanDen
      vb={vb}
      lichSu={ls}
      bayGio={bayGio}
      traLoai={TRA_LOAI}
      traBoPhan={TRA_BO_PHAN}
      coQuyenChuyen={coQuyenChuyen}
      ban={ban}
      datBan={() => {}}
      loi={loi}
      dangGui={false}
      cauDaXong=""
      tieuDiemChuyen={false}
      onGui={() => {}}
      onDong={() => {}}
    />,
  );
}

describe("phần đầu và các ô thông tin", () => {
  it("tiêu đề mang số/năm và ngày nhận; tiêu đề KHÔNG mang trích yếu", () => {
    const html = ve();

    expect(html).toMatch(
      /<h3 id="tieu-de-ngan-van-ban-den"[^>]*>Văn bản đến số 7\/2026 · nhận ngày 22\/09\/2026<\/h3>/,
    );
    expect(html).toContain('aria-labelledby="tieu-de-ngan-van-ban-den"');
    // Trích yếu hiện trong thân ngăn, nhưng không trong tiêu đề — tiêu đề đi vào cây trợ năng.
    expect(html).toContain("Về việc rà soát hồ sơ cán bộ");
    expect(html).not.toMatch(/<h3[^>]*>[^<]*rà soát/);
  });

  it("ba chip, cơ quan ban hành, số/ký hiệu + ngày, bộ phận đang giữ tra ra TÊN", () => {
    const html = ve();

    expect(html).toContain("Đã phân công");
    expect(html).toContain("Công văn");
    expect(html).toContain("Khẩn");
    expect(html).toContain("Ban Tổ chức Tỉnh uỷ");
    expect(html).toContain("1742-CV/BTCTU · 18/09/2026");
    expect(html).toContain("VĂN PHÒNG ĐẢNG UỶ");
    // Cán bộ hiện bằng MÃ — không có tuyến tra tên cho người chỉ có `document.read`.
    expect(html).toContain("CB-00123");
    expect(html).toContain("Để bộ phận tự phân công");
  });

  it("quá hạn SUY RA từ due_at: cùng dữ liệu, trước hạn không có, sau hạn có", () => {
    // Chỉ khác đồng hồ truyền vào — đó chính là hình dạng của một giá trị được SUY RA.
    expect(ve({ bayGio: TRUOC_HAN })).not.toContain("Quá hạn");
    expect(ve({ bayGio: SAU_HAN })).toContain("Quá hạn");
    expect(ve({ bayGio: SAU_HAN })).not.toMatch(/overdue|qua_han|is_?late/i);
  });
});

describe("dòng thời gian chuyển tiếp — chỉ đọc", () => {
  it("giữ NGUYÊN thứ tự máy chủ trả, kể cả khi mốc giờ không tăng dần", () => {
    // Hai dòng cố ý đặt mốc NGƯỢC với thứ tự trong mảng: một phép `sort` ở client sẽ đảo chúng, và
    // ca này đỏ. Thứ tự là việc của máy chủ — nó biết thứ tự các lần chuyển thật đã xảy ra.
    const html = ve({
      ls: lichSu([
        dongLichSu({ id: "a", routed_at: "2026-09-23T03:00:00Z", reason: "Lý do THỨ NHẤT" }),
        dongLichSu({
          id: "b",
          routed_at: "2026-09-22T01:00:00Z",
          from_unit: "01JBOPHANMOTCUA",
          to_unit: "01JBOPHANVPDU",
          assignee: "CB-00456",
          reason: "Lý do THỨ HAI",
        }),
      ]),
    });

    const mot = html.indexOf("Lý do THỨ NHẤT");
    const hai = html.indexOf("Lý do THỨ HAI");
    expect(mot).toBeGreaterThan(-1);
    expect(hai).toBeGreaterThan(mot);
  });

  it("tra bộ phận ra tên, lần chuyển đầu nói rõ chưa bộ phận nào giữ, cán bộ hiện bằng mã", () => {
    const html = ve({
      ls: lichSu([
        dongLichSu({}),
        dongLichSu({
          id: "b",
          from_unit: "01JBOPHANMOTCUA",
          to_unit: "01JBOPHANVPDU",
          routed_by: "CB-00999",
          assignee: "CB-00456",
        }),
      ]),
    });

    expect(html).toContain("Chưa bộ phận nào giữ → BỘ PHẬN MỘT CỬA");
    expect(html).toContain("BỘ PHẬN MỘT CỬA → VĂN PHÒNG ĐẢNG UỶ");
    expect(html).toContain("CB-00999");
    expect(html).toContain("Phụ trách: CB-00456");
    // Mốc giờ theo múi giờ Việt Nam đã ghim: 03:00Z là 10:00 giờ ta.
    expect(html).toMatch(/10:00 22\/09\/2026/);
  });

  it("rỗng: nói 'Chưa chuyển xử lý lần nào', KHÔNG vẽ một danh sách trống", () => {
    const html = ve({ ls: lichSu([]) });

    expect(html).toContain(LICH_SU_RONG);
    expect(LICH_SU_RONG).toBe("Chưa chuyển xử lý lần nào.");
    expect(html).not.toContain("<ol");
  });

  it("đang tải và tải hỏng là hai câu khác câu 'chưa chuyển lần nào'", () => {
    expect(ve({ ls: null })).not.toContain(LICH_SU_RONG);
    const hong = ve({ ls: { ok: false, thongBao: "Không đọc được dòng thời gian." } });
    expect(hong).toContain("Không đọc được dòng thời gian.");
    expect(hong).not.toContain(LICH_SU_RONG);
  });

  it("không một nút Sửa, Xoá hay thùng rác nào trên dòng thời gian", () => {
    // Bảng lịch sử chỉ-thêm ở tầng CSDL, hợp đồng không có tuyến sửa hay xoá (luật 7, cấm #5).
    const html = ve({ ls: lichSu([dongLichSu({})]) });

    expect(html).not.toContain("🗑");
    expect(html).not.toContain("nut-xoa");
    expect(html).not.toMatch(/>\s*Xoá\s*</);
    expect(html).not.toMatch(/>\s*Sửa\s*</);
  });
});

describe("văn bản không đọc được", () => {
  it("404: hiện NGUYÊN câu máy chủ, không số hiệu, không khối chuyển, không dòng thời gian", () => {
    // Một câu cho ba ca — không có, đã gỡ, thuộc xã khác. Màn hình không tách chúng ra.
    const cau = "Không tìm thấy văn bản đến này.";
    const html = ve({ vb: { ok: false, thongBao: cau }, ls: { ok: false, thongBao: cau } });

    expect(html).toContain(cau);
    expect(html).toContain('role="alert"');
    expect(html).not.toContain("404");
    expect(html).not.toContain("Chuyển và ghi vết");
    expect(html).not.toContain("Dòng thời gian chuyển tiếp");
    // Tiêu đề vẫn có, để `aria-labelledby` của ngăn không trỏ vào hư không.
    expect(html).toContain("Chi tiết văn bản đến");
  });

  it("đang tải: nói đang tải, chưa vẽ khối chuyển", () => {
    const html = ve({ vb: null, ls: null });

    expect(html).toContain("Đang tải văn bản");
    expect(html).not.toContain("Chuyển và ghi vết");
  });
});

describe("khối chuyển xử lý — cổng `document.route`", () => {
  it("THIẾU quyền: không có khối chuyển, nhưng thông tin và dòng thời gian vẫn hiện", () => {
    const html = ve({ coQuyenChuyen: false, ls: lichSu([dongLichSu({})]) });

    expect(html).not.toContain("Chuyển cho bộ phận khác");
    expect(html).not.toContain("Lý do chuyển");
    expect(html).not.toContain("Chuyển và ghi vết");
    expect(html).toContain("Dòng thời gian chuyển tiếp");
    expect(html).toContain("Lý do lần một");
  });

  it("CÓ quyền: đủ ba ô, câu 'không sửa được', và nút Chuyển và ghi vết", () => {
    const html = ve({ coQuyenChuyen: true });

    expect(html).toContain("Chuyển cho bộ phận khác");
    expect(html).toContain("Chuyển đến bộ phận");
    expect(html).toContain("Cán bộ xử lý (không bắt buộc)");
    expect(html).toContain("Lý do chuyển");
    expect(html).toMatch(/KHÔNG sửa được/);
    expect(html).toMatch(/hai lần chuyển/);
    expect(html).toContain("Chuyển và ghi vết");
  });

  it("câu từ chối của máy chủ ra nguyên văn trong khối", () => {
    const cau = "Văn bản đã kết thúc xử lý, không chuyển tiếp được.";
    expect(ve({ loi: cau })).toContain(cau);
  });
});

describe("lý do chuyển không rò ra ngoài (luật 3)", () => {
  const LY_DO = "Chuyển vì liên quan hộ ông Nguyễn Văn Demo";

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("lý do không nằm trong thuộc tính nào của HTML — chỉ trong value của ô đang gõ", () => {
    const html = ve({ ban: { denBoPhan: "", canBoXuLy: "", lyDo: LY_DO } });

    expect(html).toContain(`value="${LY_DO}"`);
    expect(html).not.toMatch(new RegExp(`(aria-label|title|id|href|data-[a-z-]+)="[^"]*${LY_DO}`));
  });

  it("một lần chuyển: lý do chỉ đi trong THÂN POST — không trong URL, console hay bộ nhớ trình duyệt", async () => {
    const goi = vi.fn(
      async () =>
        new Response(JSON.stringify(vanBan()), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", goi);
    const luu = { setItem: vi.fn(), getItem: vi.fn(), removeItem: vi.fn(), clear: vi.fn() };
    vi.stubGlobal("localStorage", luu);
    vi.stubGlobal("sessionStorage", luu);
    const ghi = (["log", "info", "warn", "error", "debug"] as const).map((m) =>
      vi.spyOn(console, m).mockImplementation(() => {}),
    );

    const kq = await guiChuyenVanBan("01JVBDEN0000000000000001", {
      denBoPhan: "01JBOPHANVPDU",
      canBoXuLy: "",
      lyDo: LY_DO,
    });

    expect(kq.ok).toBe(true);
    expect(goi).toHaveBeenCalledTimes(1);
    const [url, tuyChon] = goi.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/incoming-documents/01JVBDEN0000000000000001/routings");
    expect(decodeURIComponent(url)).not.toContain("Nguyễn");
    expect(String(tuyChon.body)).toContain(LY_DO);
    for (const g of ghi) expect(g).not.toHaveBeenCalled();
    expect(luu.setItem).not.toHaveBeenCalled();
  });

  it("mã nguồn ngăn và sổ không có đường ghi nào ra console, bộ nhớ trình duyệt hay URL", () => {
    // Vế tĩnh của ca trên: ca trên chỉ chạy một đường đi; ca này canh cả hai tệp, bỏ chú thích.
    for (const tep of ["./ngan-van-ban-den.tsx", "./so-van-ban-den.tsx"]) {
      const nguon = readFileSync(new URL(tep, import.meta.url), "utf8")
        .replace(/\{\/\*[\s\S]*?\*\/\}/g, "")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .replace(/^\s*\/\/.*$/gm, "");
      expect(nguon, tep).not.toMatch(/console\./);
      expect(nguon, tep).not.toMatch(/localStorage|sessionStorage|indexedDB/);
      expect(nguon, tep).not.toMatch(/history\.(push|replace)State|router\.(push|replace)|useSearchParams/);
    }
  });
});
