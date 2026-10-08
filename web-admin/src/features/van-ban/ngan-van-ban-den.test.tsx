import { readFileSync } from "node:fs";

import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { danhBaTheoMa, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
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
  danhBa = null as DanhBaTheoMa | null,
  coQuyenGhi = false,
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
      onGui={() => {}}
      onDong={() => {}}
      danhBa={danhBa}
      coQuyenGhi={coQuyenGhi}
    />,
  );
}

// VBD-07: the staff directory the register reads once. `CB-00999` is deliberately absent — an account
// no longer active must still read as its code.
const DANH_BA = danhBaTheoMa([
  { code: "CB-00123", full_name: "Trần Thị B", position: "Văn thư", department_id: "", email_masked: null },
  { code: "CB-00456", full_name: "Lê Văn C", position: "", department_id: "", email_masked: null },
]);

describe("phần đầu và các ô thông tin", () => {
  it("tiêu đề mang số/năm và ngày đến (câu chữ prototype); tiêu đề KHÔNG mang trích yếu", () => {
    const html = ve();

    expect(html).toMatch(/<h2 id="tieu-de-ngan-van-ban-den"[^>]*>Số đến 7\/2026 · đến ngày 22\/09\/2026<\/h2>/);
    // The prototype's 72rem drawer, passed to the shared panel as a size.
    expect(html).toContain("xl:w-[min(72rem,98vw)]");
    // The prototype's drawer is a dialog named by that heading (`LargeDialog`).
    expect(html).toMatch(/<dialog aria-labelledby="tieu-de-ngan-van-ban-den"/);
    // Trích yếu hiện ngay dưới, chữ lớn, nhưng KHÔNG trong tiêu đề — tiêu đề đi vào cây trợ năng.
    expect(html).toContain("Về việc rà soát hồ sơ cán bộ");
    expect(html).not.toMatch(/<h2[^>]*>[^<]*rà soát/);
  });

  it("trạng thái đang ghi, loại, cơ quan ban hành, số/ký hiệu + ngày, bộ phận đang giữ tra ra TÊN", () => {
    const html = ve();

    expect(html).toContain("Đã phân công");
    expect(html).toContain("Công văn");
    // Owner Q1: urgency is shown nowhere but the intake fold.
    expect(html).not.toContain("Khẩn");
    // Under the strip: the stored status only — no type or urgency chip.
    const line = /<p[^>]*aria-label="Trạng thái đang ghi"[^>]*>([\s\S]*?)<\/p>/.exec(html)?.[1] ?? "";
    expect(line).toContain("Đã phân công");
    expect(line).not.toContain("Công văn");
    expect(html).toContain("Ban Tổ chức Tỉnh uỷ");
    expect(html).toContain("1742-CV/BTCTU");
    expect(html).toContain("18/09/2026");
    expect(html).toContain("VĂN PHÒNG ĐẢNG UỶ");
    // No assignee: the prototype's sentence (`DocumentDetailDrawer.tsx:365-367`).
    expect(html).toContain("Chưa chỉ định người xử lý");
  });

  it("ô Hạn xử lý: dòng hai là “Hạn cuối {ngày}”, hoặc “Loại văn bản này chưa đặt hạn” khi không có hạn", () => {
    expect(ve()).toContain("Hạn cuối 25/09/2026");
    expect(ve({ vb: { ok: true, duLieu: vanBan({ due_at: "" }) } })).toContain("Loại văn bản này chưa đặt hạn");
  });

  it("văn bản ĐÃ KẾT THÚC không “Quá hạn” ở ngăn, kể cả sau hạn", () => {
    expect(ve({ bayGio: SAU_HAN, vb: { ok: true, duLieu: vanBan({ status: "da-giai-quyet" }) } })).not.toContain("Quá hạn");
    expect(ve({ bayGio: SAU_HAN })).toContain("Quá hạn");
  });

  it("quá hạn SUY RA từ due_at: cùng dữ liệu, trước hạn không có, sau hạn có", () => {
    // Chỉ khác đồng hồ truyền vào — đó chính là hình dạng của một giá trị được SUY RA.
    expect(ve({ bayGio: TRUOC_HAN })).not.toContain("Quá hạn");
    expect(ve({ bayGio: SAU_HAN })).toContain("Quá hạn");
    expect(ve({ bayGio: SAU_HAN })).not.toMatch(/overdue|qua_han|is_?late/i);
  });
});

describe("VBD-07 — cán bộ hiện họ tên, tra từ danh bạ đọc một lần", () => {
  it("ngăn KHÔNG còn ô “Người vào sổ” (không có trong sáu ô của prototype)", () => {
    expect(ve({ danhBa: DANH_BA })).not.toContain("Người vào sổ");
  });

  it("dòng thời gian: người chuyển hiện họ tên; mã ngoài danh bạ hiện nguyên mã; không còn dòng “Phụ trách”", () => {
    const html = ve({
      danhBa: DANH_BA,
      ls: lichSu([
        dongLichSu({ routed_by: "CB-00123", assignee: "CB-00456" }),
        dongLichSu({ id: "b", routed_by: "CB-00999", assignee: "CB-00999" }),
      ]),
    });
    expect(html).toContain("Trần Thị B (CB-00123)</strong>");
    // Not in the directory (e.g. an account no longer active): the bare code, never an empty cell.
    expect(html).toContain(">CB-00999</strong>");
    expect(html).not.toContain("Phụ trách");
  });

  it("chưa giao cán bộ / chưa bộ phận nào giữ: câu của prototype, có danh bạ hay không", () => {
    const html = ve({ danhBa: DANH_BA, vb: { ok: true, duLieu: vanBan({ holding_unit: "", assignee: "" }) } });
    expect(html).toContain("Chưa chỉ định người xử lý");
    expect(html).toContain("Chưa chuyển cho ai");
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

    // The prototype's line: from, an arrow (read as "đến"), to.
    const tu = (a: string, b: string) => new RegExp(`<span>${a}</span><svg[^>]*>[\\s\\S]*?</svg><span class="an-thi-giac">đến</span><span>${b}</span>`);
    // First routing: nobody held it — the prototype's "Văn thư".
    expect(html).toMatch(tu("Văn thư", "BỘ PHẬN MỘT CỬA"));
    expect(html).toMatch(tu("BỘ PHẬN MỘT CỬA", "VĂN PHÒNG ĐẢNG UỶ"));
    expect(html).toContain("CB-00999");
    // Mốc giờ theo múi giờ Việt Nam đã ghim: 03:00Z là 10:00 giờ ta.
    expect(html).toMatch(/10:00 22\/09\/2026/);
  });

  it("rỗng: nói 'Chưa chuyển xử lý lần nào', KHÔNG vẽ một danh sách trống", () => {
    const html = ve({ ls: lichSu([]) });

    expect(html).toContain(LICH_SU_RONG);
    expect(LICH_SU_RONG).toBe("Chưa chuyển cho bộ phận nào.");
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
    // Measured on the timeline section only, with write permission ON: the header's Sửa / Gỡ act
    // on the DOCUMENT, never on a routing line.
    const full = ve({ ls: lichSu([dongLichSu({})]), coQuyenGhi: true });
    const html = /<section aria-labelledby="tieu-de-dong-thoi-gian"[\s\S]*?<\/section>/.exec(full)?.[0] ?? "";
    expect(html).toContain("Lý do lần một");

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

    // The prototype's title, verbatim; no extra paragraph under the reason.
    expect(html).toContain(">Chuyển cho bộ phận khác, không đổi trạng thái</h3>");
    expect(html).toContain(">Chuyển đến</label>");
    expect(html).toContain("Người xử lý (không bắt buộc)");
    expect(html).toContain("Lý do chuyển");
    expect(html).not.toMatch(/KHÔNG sửa được/);
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
    //
    // 28/09/2026, SRS M7.2.2 ("Làm luôn bộ nhận"): trang `/van-ban` nay ĐỌC ba tham số lọc Tổng quan
    // (`metric`, `from`, `to`) từ đường dẫn. Lệnh cấm dưới đây KHÔNG nới, vì việc đọc ấy không nằm
    // trong hai tệp này: `app/van-ban/page.tsx` đọc `searchParams` ở MÁY CHỦ, đi qua đúng một hàm
    // thuần (`parseDrillDown`, `lib/drill-down.ts`) chỉ nhìn ba khoá ấy, rồi chuyển xuống bằng prop
    // `drillDown`. Ba khoá ấy không mang dữ liệu cá nhân — tên một số liệu và hai mốc thời gian — nên
    // điều ca này canh (lý do chuyển, trích yếu không bao giờ lên URL, luật 3 cấm #4) vẫn nguyên. Để
    // giữ nguyên hình dạng ấy, ca này còn cấm thêm hai tệp tự đọc thanh địa chỉ (`location`,
    // `searchParams`): quyển sổ đọc URL là bước đầu của quyển sổ GHI trạng thái của nó lên URL.
    for (const tep of ["./ngan-van-ban-den.tsx", "./so-van-ban-den.tsx"]) {
      const nguon = readFileSync(new URL(tep, import.meta.url), "utf8")
        .replace(/\{\/\*[\s\S]*?\*\/\}/g, "")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .replace(/^\s*\/\/.*$/gm, "");
      expect(nguon, tep).not.toMatch(/console\./);
      expect(nguon, tep).not.toMatch(/localStorage|sessionStorage|indexedDB/);
      expect(nguon, tep).not.toMatch(/history\.(push|replace)State|router\.(push|replace)|useSearchParams/);
      expect(nguon, tep).not.toMatch(/\blocation\b|searchParams/);
    }
  });
});

describe("dải trạng thái C2 (chỗ giữ “?”, ADR 0068 §14)", () => {
  it("đứng sau trích yếu, trước hàng chip; năm bước C2, bước đang đứng sáng, mọi nút vô hiệu, một “?”", () => {
    const html = ve();
    const hang = html.indexOf('aria-labelledby="nhan-chuyen-trang-thai"');

    expect(hang).toBeGreaterThan(html.indexOf("Về việc rà soát hồ sơ cán bộ"));
    expect(hang).toBeLessThan(html.indexOf('aria-label="Trạng thái đang ghi"'));
    // `da-phan-cong` stands on "Đã chuyển xử lý"; the four other steps are "chuyển sang".
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*aria-current="step"[^>]*aria-label="Đã chuyển xử lý, đang ở đây"/);
    const nut = html.match(/<button[^>]*aria-label="Chuyển sang [^"]*"[^>]*>/g) ?? [];
    expect(nut).toHaveLength(4);
    for (const n of nut) expect(n).toContain('disabled=""');
    expect(html).toContain("Chuyển trạng thái văn bản đến — tính năng đang phát triển");
  });

  it("mã không có bước C2 (chuyen-cap-tren, luu-khong-thu-ly): KHÔNG bước nào sáng; trạng thái vẫn ghi bằng chữ", () => {
    for (const status of ["chuyen-cap-tren", "luu-khong-thu-ly"]) {
      const html = ve({ vb: { ok: true, duLieu: vanBan({ status }) } });
      expect(html).not.toContain('aria-current="step"');
      expect((html.match(/aria-label="Chuyển sang /g) ?? []).length).toBe(5);
    }
    expect(ve({ vb: { ok: true, duLieu: vanBan({ status: "luu-khong-thu-ly" }) } })).toContain("Lưu, không thụ lý");
  });

  it("chưa đọc được văn bản thì không có hàng ấy", () => {
    expect(ve({ vb: null })).not.toContain("nhan-chuyen-trang-thai");
  });
});

describe("ngăn theo prototype (ADR 0068 lần 5)", () => {
  it("Sửa và Gỡ khỏi sổ ở đầu ngăn CHỈ khi có `document.create` — vế từ chối trước", () => {
    const denied = ve({ coQuyenGhi: false });
    expect(denied).not.toMatch(/>\s*Sửa\s*</);
    expect(denied).not.toContain("Gỡ khỏi sổ");

    const allowed = ve({ coQuyenGhi: true });
    expect(allowed).toMatch(/>\s*Sửa\s*</);
    expect(allowed).toContain("Gỡ khỏi sổ");
  });

  it("hàng ba ô dưới dải trạng thái: Hạn xử lý · Bộ phận đang giữ · Nguồn vào sổ (“?”)", () => {
    const html = ve();
    const a = html.indexOf(">Hạn xử lý<");
    const b = html.indexOf(">Bộ phận đang giữ<");
    const c = html.indexOf(">Nguồn vào sổ<");
    expect(a).toBeGreaterThan(-1);
    expect(b).toBeGreaterThan(a);
    expect(c).toBeGreaterThan(b);
    expect(html).toContain("Nguồn nhập văn bản đến — tính năng đang phát triển");
    expect(html).toContain("Ghi chú văn bản đến — tính năng đang phát triển");
  });

  it("“Chuyển thành nhiệm vụ” có mặt, vô hiệu, có “?” — không gọi tuyến nào", () => {
    const html = ve();
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*>[\s\S]{0,400}?Chuyển thành nhiệm vụ/);
    expect(html).toContain("Chuyển thành nhiệm vụ — tính năng đang phát triển");
  });

  it("dòng thời gian ở cột phải, sau thân trái", () => {
    const html = ve({ ls: lichSu([dongLichSu({})]) });
    expect(html.indexOf('aria-labelledby="tieu-de-dong-thoi-gian"')).toBeGreaterThan(html.indexOf("Trích yếu nội dung"));
    expect(html).toMatch(/<aside[^>]*md:w-\[24rem\]/);
  });

  it("có danh bạ: người xử lý chọn theo HỌ TÊN, giá trị gửi đi vẫn là MÃ cán bộ", () => {
    const html = ve({ danhBa: DANH_BA });
    expect(html).toMatch(/<select id="o-can-bo-xu-ly"/);
    expect(html).toContain('<option value="CB-00123">Trần Thị B — Văn thư</option>');
    expect(html).toContain("— Để bộ phận tự phân công —");
  });

  it("chưa có danh bạ: ô gõ mã như trước — chuyển không phải chờ danh bạ", () => {
    expect(ve({ danhBa: null })).toMatch(/<input id="o-can-bo-xu-ly"/);
  });
});
