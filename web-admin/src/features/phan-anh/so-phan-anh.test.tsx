import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import type { ReactElement, ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_nhatKyPhieuRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";

import {
  CANH_BAO_RE_NHANH,
  composerTitle,
  congThaoTac,
  danhBaTheoMa,
  DE_BO_PHAN_PHAN_CONG,
  GOI_Y_GHI_NHAT_KY,
  LIST_EMPTY_HINT,
  LIST_EMPTY_TITLE,
  NHAC_DU_LIEU_CA_NHAN,
  NHAN_BO_PHAN_PHU_TRACH,
  NHAN_CHUYEN_CAP_TREN,
  NHAN_KHONG_TIEP_NHAN,
  NHAN_NUT_GHI_NHAT_KY,
  NHAN_O_CO_QUAN,
  NHAN_O_GHI_CHU_NOI_BO,
  NHAN_O_KET_QUA,
  NHAN_O_LY_DO,
  NHAT_KY_RONG,
  RESULT_PLACEHOLDER,
  STRIP_NEEDS_RESOLVE,
  STRIP_NO_PERMISSION,
  STRIP_NOT_A_STEP,
  STRIP_WAIT_CITIZEN,
  PHAM_VI_GIAO_CHO_TOI,
  PHAM_VI_TOAN_XA,
  PHAN_CHUA_DUNG,
  petitionPendingPart,
  SCOPE_RELATED_LABEL,
  SO_RONG,
  TIEU_DE_NHAT_KY,
  UNVERIFIED_CONTACT_LABEL,
  UNVERIFIED_CONTACT_NOTE,
  LOG_INTERNAL_NOTE,
} from "./nhan-phieu";
import { BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";
import {
  BieuMauReNhanh,
  ChiTietPhieu,
  DanhSachThe,
  HangLoc,
  ONhapGhiChuNoiBo,
  ThePhieu,
} from "./so-phan-anh";
import { BieuMauGhiNhatKy, DanhSachNhatKy, NhatKyPhieu } from "./nhat-ky-phieu";
import { PublicationBox } from "./citizen-report-blocks";

const THU_MUC = fileURLToPath(new URL(".", import.meta.url));

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **HAI CỔNG KHÁC NHAU**, và nó là nhóm không ai nhìn thấy
 * trong lúc phát triển: tài khoản người viết mã có cả bốn khoá, nên cả bốn nút luôn hiện. Điều
 * phải đúng là chuyện ngược lại — một trưởng thôn chỉ có `feedback.read` và đang giữ một phiếu
 * **tiến được trạng thái phiếu ấy** nhưng **KHÔNG đóng được nó**.
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

/**
 * The opening tag of the first submit button, WITHOUT its `class` attribute.
 *
 * Since the 02/10/2026 redesign the class string carries Tailwind's `disabled:` variants
 * (`disabled:opacity-60`), so `toContain("disabled")` on the whole tag would be green for an
 * ENABLED button too — the positive assertions would check nothing. Stripping the class leaves
 * the `disabled=""` attribute as the only way the word can appear.
 */
function submitTag(html: string): string {
  return (html.match(/<button type="submit"[^>]*>/)?.[0] ?? "").replace(/\sclass="[^"]*"/, "");
}

function phieu(sua: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-xu-ly",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.",
    address: "Tổ 6, thôn Hà Lam",
    // SỐ ĐÃ CHE SẴN Ở MÁY CHỦ. Số mẫu theo luật 3, bất biến 5.
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: "2026-09-09T09:20:00Z",
    resolve_due: "2026-09-10T09:20:00Z",
    classify_due: "2026-09-09T11:20:00Z",
    unit: "01JBOPHAN",
    assignee: "",
    result: "",
    public: false,
    ...sua,
  };
}

const BO_PHAN: identity_boPhanRa[] = [
  { id: "01JBOPHAN", code: "vp-dang-uy", name: "VĂN PHÒNG ĐẢNG ỦY", parent_id: "", order: 0, staff_count: 0 },
];
const TEN_BO_PHAN = new Map(BO_PHAN.map((b) => [b.id, b.name]));

const BAY_GIO = new Date("2026-09-10T02:00:00Z");

const DANH_BA: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: {
    items: [
      { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "01JBOPHAN", email_masked: null },
      { code: "CB-00200", full_name: "Lê Văn C", position: "Trưởng thôn", department_id: "01JKHAC", email_masked: null },
    ],
  },
};

function veChiTiet(
  cong: ReturnType<typeof congThaoTac>,
  p = phieu(),
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null = DANH_BA,
  initialStep: string | null = null,
): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      initialStep={initialStep}
      phieu={p}
      bayGio={BAY_GIO}
      cong={cong}
      tenBoPhan={TEN_BO_PHAN}
      boPhan={BO_PHAN}
      danhBa={danhBa}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
    />,
  );
}

/** `veChiTiet` with the session's permission keys — the blocks drawn behind a key need them. */
function veChiTietWith(cong: ReturnType<typeof congThaoTac>, p: petitions_phieuPhanAnhRa, permissions: readonly string[]): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={BAY_GIO}
      cong={cong}
      tenBoPhan={TEN_BO_PHAN}
      boPhan={BO_PHAN}
      danhBa={DANH_BA}
      dangGui={false}
      loiGhi={null}
      permissions={permissions}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
    />,
  );
}

/** Chỉ dấu KHÔNG THỂ NHẦM của biểu mẫu đóng phiếu: id của ô kết quả. */
const O_KET_QUA = 'id="ket-qua-xu-ly"';

type Chip = { tag: string; label: string; role: string };

/** Every chip of the status strip: its `<button>` tag, its word, its second line. */
function chips(html: string): Chip[] {
  return [
    ...html.matchAll(
      /<li class="flex min-w-\[6\.5rem\] flex-1">(<button[^>]*>)[\s\S]*?<\/svg>([^<]*)<\/span><span[^>]*>([^<]*)<\/span><\/button><\/li>/g,
    ),
  ].map((m) => ({ tag: m[1] ?? "", label: m[2] ?? "", role: m[3] ?? "" }));
}

function chip(html: string, label: string): Chip | undefined {
  return chips(html).find((c) => c.label === label);
}

/**
 * OWNER DECISION D1 (09/10/2026): the strip's chips are pressable ONLY for a move our server performs,
 * and pressing one opens THE EXISTING ACT for it. A static render cannot press: `initialStep` opens the
 * composer of that chip — and opens NOTHING when the chip is not pressable, which is what the denied
 * cases below rely on.
 */
describe("HAI CỔNG KHÁC NHAU — tiến trạng thái (luật nắm giữ) và Đóng phiếu (`feedback.resolve`)", () => {
  it("chỉ có LUẬT NẮM GIỮ (không khoá nào): bước kế tiếp BẤM ĐƯỢC; Đóng phiếu thì không", () => {
    // Đây là tài khoản trưởng thôn: xem được sổ, đang giữ một phiếu, không có khoá toàn xã.
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ status: "dang-xu-ly" }), DANH_BA, "da-xu-ly");
    // Chip kế tiếp CÓ — luật nắm giữ mở nó, và giao diện không được lấy mất.
    expect(chip(html, "Đã xử lý")?.role).toBe("chuyển sang");
    expect(html).toContain(nhuTrongHTML(composerTitle("da-xu-ly")));
    expect(html).toContain('id="ghi-chu-tien"');
    expect(html).toMatch(/>Xác nhận<\/button>/);
    expect(html).toMatch(/>Huỷ<\/button>/);
    // No close form on this status, whatever the key.
    expect(html).not.toContain(O_KET_QUA);
  });

  it("DENIED — `cho-dan-xac-nhan` without `feedback.resolve`: the `Đã đóng` chip is blocked, names the key, opens nothing", () => {
    const html = veChiTiet(congThaoTac(true, true, false), phieu({ status: "cho-dan-xac-nhan" }), DANH_BA, "da-dong");
    const dong = chip(html, "Đã đóng");
    expect(dong?.role).toBe("—");
    expect(dong?.tag).toContain('aria-disabled="true"');
    expect(dong?.tag).toContain(nhuTrongHTML(STRIP_NEEDS_RESOLVE));
    expect(html).not.toContain(O_KET_QUA);
    expect(html).not.toContain(nhuTrongHTML(NHAN_O_KET_QUA));
    // No move is open to this account here: the prototype's one sentence.
    expect(html).toContain(STRIP_NO_PERMISSION);
  });

  it("có `feedback.resolve`: the `Đã đóng` chip opens the close act, with the result the citizen reads", () => {
    const html = veChiTiet(congThaoTac(false, false, true), phieu({ status: "cho-dan-xac-nhan" }), DANH_BA, "da-dong");
    expect(chip(html, "Đã đóng")?.role).toBe("chuyển sang");
    expect(html).toContain(O_KET_QUA);
    expect(html).toContain(nhuTrongHTML(NHAN_O_KET_QUA));
    expect(html).toContain(nhuTrongHTML(RESULT_PLACEHOLDER));
    expect(html).not.toContain(STRIP_NO_PERMISSION);
  });

  it("bước tiến KHÔNG bị gắn sau `feedback.resolve` — bốn bộ quyền, bốn lần vẫn bấm được", () => {
    // VẾ CHỊU LỰC. Bài này đỏ đúng vào ngày ai đó "gộp cho gọn" hai cổng làm một — thao tác trông
    // hợp lý, không làm hỏng màn hình của người viết mã, và lấy mất khả năng xử lý việc của mọi
    // trưởng thôn trong xã.
    for (const cong of [
      congThaoTac(false, false, false),
      congThaoTac(true, false, false),
      congThaoTac(false, true, false),
      congThaoTac(true, true, true),
    ]) {
      expect(chip(veChiTiet(cong), "Đã xử lý")?.role).toBe("chuyển sang");
    }
  });

  it("phiếu đã đóng: không chip nào bấm được — không phải vì quyền", () => {
    const html = veChiTiet(congThaoTac(true, true, true), phieu({ status: "da-dong" }));
    expect(chips(html).filter((c) => c.role === "chuyển sang")).toEqual([]);
    expect(chip(html, "Đã đóng")?.role).toBe("đang ở đây");
    expect(html).not.toContain(STRIP_NO_PERMISSION);
  });

  it("the strip: icon + word on every chip, the current one filled with ITS status colour", () => {
    const html = veChiTiet(congThaoTac(true, true, true), phieu({ status: "dang-xu-ly" }));
    expect(chips(html).map((c) => c.label)).toEqual([
      "Đã tiếp nhận",
      "Đang phân loại",
      "Đã chuyển xử lý",
      "Đang xử lý",
      "Đã xử lý",
      "Chờ dân xác nhận",
      "Đã đóng",
    ]);
    const current = chip(html, "Đang xử lý");
    expect(current?.tag).toContain('aria-current="step"');
    expect(current?.tag).toContain("bg-teal text-white");
    // No branch row when no branch is the petition's or open to it.
    expect(html).not.toContain("Rẽ nhánh:");
  });
});

describe("CỔNG QUYỀN — phân loại và chuyển xử lý", () => {
  it("DENIED — thiếu `feedback.classify`: chip `Đang phân loại` blocked with the key, no field select", () => {
    const html = veChiTiet(congThaoTac(false, true, true), phieu({ status: "da-tiep-nhan" }), DANH_BA, "dang-phan-loai");
    expect(html).not.toContain('id="chon-linh-vuc"');
    expect(chip(html, "Đang phân loại")?.tag).toContain("feedback.classify");
    expect(html).toContain(STRIP_NO_PERMISSION);
  });

  it("có `feedback.classify` ở `da-tiep-nhan`: the chip opens the classify act", () => {
    const html = veChiTiet(congThaoTac(true, false, false), phieu({ status: "da-tiep-nhan" }), DANH_BA, "dang-phan-loai");
    expect(html).toContain('id="chon-linh-vuc"');
    expect(html).toContain('id="ghi-chu-phan-loai"');
  });

  it("DENIED — thiếu `feedback.assign`: no hand-over section, NO officer select", () => {
    const html = veChiTiet(congThaoTac(true, false, true));
    expect(html).not.toContain('id="chon-bo-phan"');
    expect(html).not.toContain('id="chon-can-bo"');
    expect(html).not.toContain(nhuTrongHTML(DE_BO_PHAN_PHAN_CONG));
    expect(html).not.toContain("Chuyển xử lý, không đổi trạng thái");
  });

  it("có `feedback.classify` nhưng phiếu đã qua bước phân loại: không vẽ ô chọn", () => {
    // Máy chủ chốt lĩnh vực bằng câu UPDATE mang `trang_thai = 'da-tiep-nhan'`, nên lần thứ hai là
    // 409. The chip is not a move from here.
    const html = veChiTiet(congThaoTac(true, true, true), phieu({ status: "dang-xu-ly" }), DANH_BA, "dang-phan-loai");
    expect(html).not.toContain('id="chon-linh-vuc"');
    expect(chip(html, "Đang phân loại")?.tag).toContain(nhuTrongHTML(STRIP_NOT_A_STEP));
  });

  it("thiếu `feedback.read`: cả màn không dựng", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen
        quyetDinh={{ hien: false, vi: "khong-du-quyen" }}
        cauThieuQuyen="Tài khoản của bạn không có quyền xem phản ánh của người dân (feedback.read)."
      >
        <ThePhieu
          phieu={phieu()}
          bayGio={BAY_GIO}
          dangMo={false}
          mo={() => {}}
        />
      </KhungQuyen>,
    );
    expect(html).toContain("feedback.read");
    expect(html).not.toContain("PA-2026-0021");
  });
});

describe("ô chọn cán bộ xử lý (§8.5) và ô `Đang giao cho` (§8.3)", () => {
  it("có `feedback.assign`: ô chọn cán bộ có mặt, mặc định `— Để bộ phận phân công —`", () => {
    const html = veChiTiet(congThaoTac(false, true, false));
    expect(html).toContain('id="chon-can-bo"');
    expect(html).toContain(nhuTrongHTML(DE_BO_PHAN_PHAN_CONG));
    // Chưa chọn bộ phận: ô chọn cán bộ khoá lại và không liệt kê ai — người được liệt kê là người
    // của bộ phận được chọn.
    expect(html).not.toContain('value="CB-00123"');
    expect(html).not.toContain("Lê Văn C");
  });

  it("ô `Đang giao cho` hiện HỌ TÊN tra từ danh bạ, không hiện mã", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ assignee: "CB-00123" }));
    expect(html).toContain("Trần Thị B");
    expect(html).not.toContain("CB-00123");
  });

  it("người giữ phiếu không còn trong danh bạ: hiện mã kèm câu trung tính, không `undefined`", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ assignee: "CB-00999" }));
    expect(html).toContain("CB-00999");
    expect(html).toContain("không có trong danh bạ");
    expect(html).not.toContain("undefined");
  });

  it("danh bạ tải hỏng: vẫn hiện mã người giữ phiếu, và nói ra câu lỗi ở khối chuyển xử lý", () => {
    const hong: KetQua<identity_danhBaChonNguoiRa> = {
      ok: false,
      thongBao: "Đã xảy ra lỗi. Vui lòng thử lại.",
    };
    const html = veChiTiet(congThaoTac(false, true, false), phieu({ assignee: "CB-00123" }), hong);
    expect(html).toContain("CB-00123");
    expect(html).toContain("Không tải được danh bạ cán bộ");
    // Chuyển cho bộ phận vẫn làm được — lỗi danh bạ không chặn đường ấy.
    expect(html).toContain('id="chon-bo-phan"');
  });
});

describe("hai tab phạm vi (§4)", () => {
  function veHangLoc(phamVi?: "all" | "mine"): string {
    return renderToStaticMarkup(
      <HangLoc
        loc={phamVi === undefined ? {} : { phamVi }}
        tim=""
        datTim={() => {}}
        datLoc={() => {}}
        thon={[]}
      />,
    );
  }

  it("có `Toàn xã` và `Giao cho tôi`; `Liên quan đến tôi` chỉ là CHỖ GIỮ vô hiệu có dấu '?'", () => {
    const html = veHangLoc();
    expect(html).toContain(PHAM_VI_TOAN_XA);
    expect(html).toContain(PHAM_VI_GIAO_CHO_TOI);
    // The server answers 400 to `scope=related`: the third tab is a disabled native button, never live.
    const related = html.match(new RegExp(`<button[^>]*>${SCOPE_RELATED_LABEL}</button>`))?.[0] ?? "";
    expect(related).toContain('disabled=""');
    expect(related).toContain('aria-pressed="false"');
    expect(html).toContain(`aria-label="${nhuTrongHTML(pendingMarkerLabel(petitionPendingPart("scopeRelated").ten))}"`);
  });

  it("đúng MỘT tab được đánh dấu, theo bộ lọc đang chọn", () => {
    for (const phamVi of [undefined, "mine"] as const) {
      const html = veHangLoc(phamVi);
      expect(html.match(/aria-pressed="true"/g)?.length).toBe(1);
      const nutDangChon = html.match(/aria-pressed="true"[^>]*>([^<]*)</)?.[1];
      expect(nutDangChon).toBe(phamVi === "mine" ? PHAM_VI_GIAO_CHO_TOI : PHAM_VI_TOAN_XA);
    }
  });
});

describe("dữ liệu cá nhân — màn hình hiện đúng thứ máy chủ gửi", () => {
  it("số điện thoại ra màn hình đúng dạng đã che, không ghép lại chữ số nào", () => {
    const html = renderToStaticMarkup(
      <ThePhieu
        phieu={phieu()}
        bayGio={BAY_GIO}
        dangMo={false}
        mo={() => {}}
      />,
    );
    expect(html).toContain("09****0000");
    // Không có phép "làm đẹp" nào biến dấu sao thành chữ số.
    expect(html).not.toMatch(/09\d{8}/);
  });

  it("phiếu ẩn danh: KHÔNG có tên, KHÔNG có số, và không bù vào bằng gì cả", () => {
    const html = renderToStaticMarkup(
      <ThePhieu
        phieu={phieu({ anonymous: true, reporter_name: "", reporter_phone: "" })}
        bayGio={BAY_GIO}
        dangMo={false}
        mo={() => {}}
      />,
    );
    // The card's words are the prototype's: "Gửi ẩn danh" (the drawer says "Người gửi ẩn danh").
    expect(html).toContain("Gửi ẩn danh");
    expect(html).not.toContain("Nguyễn");
    expect(html).not.toContain("09****0000");
  });
});

describe("vị trí — khối chi tiết (D2: only with coordinates)", () => {
  const ALL_GATES = congThaoTac(true, true, true);

  it("coordinates: the `Vị trí` section — the map's box, the address · hamlet, the coordinates as text", () => {
    const html = veChiTiet(
      ALL_GATES,
      phieu({ lat: 21.028511, lng: 105.804817, residential_unit_id: "01JTHON1", residential_unit_name: "Thôn Hà Lam" }),
    );
    expect(html).toContain(">Vị trí</h3>");
    // No basemap flag passed (default false, fail closed): ONE sentence in the map's place, no map.
    expect(html).toContain(BASEMAP_MISSING_SENTENCE);
    expect(html).not.toContain('data-testid="petition-map"');
    expect(html).toContain("Tổ 6, thôn Hà Lam · Thôn Hà Lam");
    expect(html).toContain("21.028511, 105.804817");
    expect(html).toContain("Toạ độ do người dân gửi kèm từ ứng dụng");
  });

  it("no coordinates: no `Vị trí` section at all (prototype `FeedbackDetailDrawer.tsx:394`)", () => {
    const html = veChiTiet(ALL_GATES, phieu());
    expect(html).not.toContain(">Vị trí</h3>");
    expect(html).not.toContain(BASEMAP_MISSING_SENTENCE);
  });

  it("anonymous: location shown, reporter hidden", () => {
    const html = veChiTiet(
      ALL_GATES,
      phieu({ anonymous: true, reporter_name: "", reporter_phone: "", lat: 21.028511, lng: 105.804817 }),
    );
    expect(html).toContain("Người gửi ẩn danh");
    expect(html).not.toContain("Nguyễn");
    expect(html).not.toContain("09****0000");
    expect(html).toContain("21.028511, 105.804817");
  });

  it("no map frame and no link carrying the coordinates; the address is escaped (rule 13)", () => {
    const html = veChiTiet(
      ALL_GATES,
      phieu({ address: "<img src=x onerror=alert(1)>", lat: 21.028511, lng: 105.804817 }),
    );
    expect(html).not.toContain("<iframe");
    expect(html).not.toMatch(/href="[^"]*21\.028511/);
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img");
  });
});

describe("lĩnh vực hạn chế — màn hình KHÔNG nói ra rằng có phiếu bị giấu", () => {
  it("sổ rỗng thì hiện câu trạng thái rỗng, không một chữ nào về phiếu bị ẩn", () => {
    // Máy chủ loại hẳn phiếu `can-bo` khỏi trang VÀ khỏi con trỏ khi tài khoản thiếu
    // `feedback.restricted`. Một câu kiểu "có n phiếu bị ẩn" ở đây là nói cho một đồng nghiệp của
    // người bị phản ánh biết rằng phiếu ấy tồn tại (luật 4, cấm #2).
    const html = renderToStaticMarkup(
      <DanhSachThe
        phieu={[]}
        bayGio={BAY_GIO}
        maDangMo={null}
        moPhieu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(SO_RONG));
    expect(html).not.toMatch(/bị ẩn|hạn chế|feedback\.restricted/);
  });
});

describe("phần chưa dựng — mô tả sau dấu '?' (ADR 0068 §14)", () => {
  it("no entry claims a built part is missing: both photo halves, the intake modal, the log", () => {
    const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");
    // Both photo halves are built (02/10/2026; the "after" row of ADR 0047 replaces G8), and the close
    // gate is enforced by the server — no entry may still claim it is not.
    expect(all).not.toContain("ADR 0047, G8");
    expect(all).not.toContain("bat_buoc_anh_nghiem_thu");
    expect(all).not.toContain("anh_phan_anh");
    expect(all).not.toContain("POST /api/v1/citizen-reports` không tồn tại");
    // Nhật ký xử lý ĐÃ dựng (26/09/2026) — nó không còn là "phần chưa dựng được".
    expect(all).not.toContain("nhat_ky_phan_anh");
  });

  it("every entry has its own `id`; an unknown one throws instead of opening an empty description", () => {
    const ids = PHAN_CHUA_DUNG.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(() => petitionPendingPart("khong-co")).toThrow();
  });
});

/**
 * HAI NHÁNH RẼ (`Không tiếp nhận`, `Chuyển cấp trên`) — ai thấy, ở đâu, và biểu mẫu nói gì.
 *
 * Canh cả CA BỊ TỪ CHỐI, không chỉ ca được phép: tài khoản người viết mã có mọi khoá, nên một nút
 * lọt ra ngoài cổng là thứ không ai thấy trong lúc phát triển.
 */
describe("hai nhánh rẽ — chỉ ở `dang-phan-loai`, chỉ với `feedback.classify`", () => {
  it("có `feedback.classify` và phiếu ở `dang-phan-loai`: hai chip rẽ nhánh bấm được; each opens its act", () => {
    const html = veChiTiet(congThaoTac(true, false, false), phieu({ status: "dang-phan-loai" }));
    expect(html).toContain("Rẽ nhánh:");
    expect(chip(html, NHAN_KHONG_TIEP_NHAN)?.role).toBe("chuyển sang");
    expect(chip(html, NHAN_CHUYEN_CAP_TREN)?.role).toBe("chuyển sang");
    const reject = veChiTiet(congThaoTac(true, false, false), phieu({ status: "dang-phan-loai" }), DANH_BA, "khong-tiep-nhan");
    expect(reject).toContain('id="ly-do-khong-tiep-nhan"');
    expect(reject).not.toContain('id="co-quan-khong-tiep-nhan"');
    const refer = veChiTiet(congThaoTac(true, false, false), phieu({ status: "dang-phan-loai" }), DANH_BA, "chuyen-cap-tren");
    expect(refer).toContain('id="ly-do-chuyen-cap-tren"');
    expect(refer).toContain('id="co-quan-chuyen-cap-tren"');
  });

  it("DENIED — THIẾU `feedback.classify`: both branch chips blocked with the key; no form opens", () => {
    const html = veChiTiet(congThaoTac(false, true, true), phieu({ status: "dang-phan-loai" }), DANH_BA, "khong-tiep-nhan");
    for (const label of [NHAN_KHONG_TIEP_NHAN, NHAN_CHUYEN_CAP_TREN]) {
      expect(chip(html, label)?.role, label).toBe("—");
      expect(chip(html, label)?.tag, label).toContain("feedback.classify");
    }
    expect(html).not.toContain('id="ly-do-khong-tiep-nhan"');
  });

  it("có khoá nhưng phiếu ở trạng thái khác: no branch chip is pressable — the server would answer 409", () => {
    for (const status of [
      "da-tiep-nhan",
      "da-chuyen-xu-ly",
      "dang-xu-ly",
      "da-xu-ly",
      "cho-dan-xac-nhan",
      "da-dong",
      "khong-tiep-nhan",
      "chuyen-cap-tren",
    ]) {
      const html = veChiTiet(congThaoTac(true, true, true), phieu({ status }), DANH_BA, "khong-tiep-nhan");
      expect(chip(html, NHAN_KHONG_TIEP_NHAN)?.role, status).not.toBe("chuyển sang");
      expect(chip(html, NHAN_CHUYEN_CAP_TREN)?.role, status).not.toBe("chuyển sang");
      expect(html, status).not.toContain('id="ly-do-khong-tiep-nhan"');
    }
  });
});

describe("biểu mẫu nhánh rẽ", () => {
  function veBieuMau(
    loai: "khong-tiep-nhan" | "chuyen-cap-tren",
    lyDo = "",
    coQuan = "",
  ): string {
    return renderToStaticMarkup(
      <BieuMauReNhanh
        loai={loai}
        dangGui={false}
        gui={() => {}}
        huy={() => {}}
        lyDoBanDau={lyDo}
        coQuanBanDau={coQuan}
      />,
    );
  }

  /** Nút gửi là nút `type="submit"`; lấy riêng thẻ ấy (bỏ `class`) để đọc thuộc tính `disabled`. */
  function nutGui(html: string): string {
    return submitTag(html);
  }

  it("nhãn ô lý do (prototype, verbatim), cảnh báo không hoàn tác + người dân được báo, Huỷ / Xác nhận", () => {
    const html = veBieuMau("khong-tiep-nhan");
    expect(html).toContain(nhuTrongHTML(NHAN_O_LY_DO));
    expect(NHAN_O_LY_DO).toBe("Lý do (bắt buộc, trả lời cho người dân)");
    expect(html).toMatch(/>Xác nhận<\/button>/);
    expect(html).toMatch(/>Huỷ<\/button>/);
    // Inline in the composer now, not a confirm dialog.
    expect(html.startsWith("<form")).toBe(true);
    expect(html).toContain(nhuTrongHTML(CANH_BAO_RE_NHANH));
    expect(CANH_BAO_RE_NHANH).toContain("không hoàn tác");
    expect(CANH_BAO_RE_NHANH).toContain("Người dân được thông báo");
  });

  it("`Không tiếp nhận` KHÔNG có ô cơ quan; `Chuyển cấp trên` CÓ", () => {
    expect(veBieuMau("khong-tiep-nhan")).not.toContain(nhuTrongHTML(NHAN_O_CO_QUAN));
    const html = veBieuMau("chuyen-cap-tren");
    expect(html).toContain(nhuTrongHTML(NHAN_O_CO_QUAN));
  });

  it("bộ đếm ký tự trực tiếp: 9 ký tự chữ Việt thì khoá nút, 10 thì mở", () => {
    const chin = "Ngập ước!"; // 9 điểm mã
    const muoi = "Ngập nước!"; // 10 điểm mã
    expect(nutGui(veBieuMau("khong-tiep-nhan", chin))).toContain("disabled");
    const html = veBieuMau("khong-tiep-nhan", muoi);
    expect(nutGui(html)).not.toContain("disabled");
    expect(html).toMatch(/10(<!-- -->)?\/(<!-- -->)?2000(<!-- -->)? ký tự/);
  });

  it("2000 ký tự chữ Việt thì mở, 2001 thì khoá", () => {
    expect(nutGui(veBieuMau("khong-tiep-nhan", "ệ".repeat(2000)))).not.toContain("disabled");
    expect(nutGui(veBieuMau("khong-tiep-nhan", "ệ".repeat(2001)))).toContain("disabled");
  });

  it("`Chuyển cấp trên` thiếu cơ quan tiếp nhận thì khoá nút, dù lý do hợp lệ", () => {
    const lyDo = "Vượt thẩm quyền của xã, thuộc ngành điện.";
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, ""))).toContain("disabled");
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, "   "))).toContain("disabled");
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, "Công ty điện lực"))).not.toContain(
      "disabled",
    );
    expect(nutGui(veBieuMau("chuyen-cap-tren", lyDo, "đ".repeat(201)))).toContain("disabled");
  });
});

describe("chi tiết phiếu ở nhánh rẽ — lý do, cơ quan, thời điểm", () => {
  const LY_DO = "Nội dung không thuộc địa bàn xã quản lý.";
  const CO_QUAN = "Công ty điện lực";

  it("`khong-tiep-nhan`: hiện lý do và thời điểm theo giờ Việt Nam, KHÔNG hiện ô cơ quan", () => {
    const html = veChiTiet(
      congThaoTac(true, true, true),
      phieu({
        status: "khong-tiep-nhan",
        reason: LY_DO,
        branch_ended_at: "2026-09-10T03:05:00Z",
      }),
    );
    // The prototype's tangerine box, labelled "Lý do {trạng thái viết thường}".
    expect(html).toContain(">Lý do không tiếp nhận</p>");
    expect(html).toContain(LY_DO);
    // 03:05Z = 10:05 giờ Việt Nam.
    expect(html).toContain("10:05");
    expect(html).not.toContain("<dt>Cơ quan tiếp nhận</dt>");
  });

  it("`chuyen-cap-tren`: hiện lý do, cơ quan tiếp nhận và thời điểm", () => {
    const html = veChiTiet(
      congThaoTac(false, false, false),
      phieu({
        status: "chuyen-cap-tren",
        reason: LY_DO,
        receiving_body: CO_QUAN,
        branch_ended_at: "2026-09-10T03:05:00Z",
      }),
    );
    expect(html).toContain(LY_DO);
    expect(html).toContain("<dt>Cơ quan tiếp nhận</dt>");
    expect(html).toContain(CO_QUAN);
    expect(html).toContain("10:05");
  });

  it("trạng thái khác: KHÔNG có ô lý do hay cơ quan, dù phản hồi lỡ mang theo", () => {
    for (const status of ["dang-phan-loai", "dang-xu-ly", "da-dong"]) {
      const html = veChiTiet(
        congThaoTac(true, true, true),
        phieu({ status, reason: LY_DO, receiving_body: CO_QUAN }),
      );
      expect(html, status).not.toMatch(/>Lý do (không tiếp nhận|chuyển cấp trên)<\/p>/);
      expect(html, status).not.toContain("<dt>Cơ quan tiếp nhận</dt>");
      expect(html, status).not.toContain(LY_DO);
    }
  });

  it("thanh bước: ô rẽ nhánh đúng trạng thái là `đang ở đây`, luồng chính không ô nào", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ status: "chuyen-cap-tren" }));
    expect(html.match(/đang ở đây/g)?.length).toBe(1);
    expect(html).toMatch(/Chuyển cấp trên<\/span><span[^>]*>đang ở đây<\/span>/);
  });
});

describe("Đóng phiếu — hai điểm đóng (the `Đã đóng` chip)", () => {
  const close = (cong: ReturnType<typeof congThaoTac>, p: petitions_phieuPhanAnhRa) =>
    veChiTiet(cong, p, DANH_BA, "da-dong");

  it("`cho-dan-xac-nhan`: có biểu mẫu đóng, kênh nào cũng vậy", () => {
    expect(close(congThaoTac(false, false, true), phieu({ status: "cho-dan-xac-nhan" }))).toContain(O_KET_QUA);
  });

  it("`da-xu-ly` kênh `can-bo-nhap-ho` (không có công dân để xác nhận): có biểu mẫu đóng", () => {
    const html = close(congThaoTac(false, false, true), phieu({ status: "da-xu-ly", channel: "can-bo-nhap-ho" }));
    expect(html).toContain(O_KET_QUA);
  });

  it("DENIED — `da-xu-ly` kênh công dân: KHÔNG có biểu mẫu đóng — phải qua bước chờ dân xác nhận", () => {
    for (const channel of ["zalo-mini-app", "zalo-oa", "web-xa"]) {
      const html = close(congThaoTac(false, false, true), phieu({ status: "da-xu-ly", channel }));
      expect(html, channel).not.toContain(O_KET_QUA);
      expect(chip(html, "Đã đóng")?.tag, channel).toContain(nhuTrongHTML(STRIP_WAIT_CITIZEN));
    }
  });

  it("DENIED — `da-xu-ly` nhập hộ nhưng THIẾU `feedback.resolve`: không biểu mẫu, the chip names the key", () => {
    const html = close(congThaoTac(true, true, false), phieu({ status: "da-xu-ly", channel: "can-bo-nhap-ho" }));
    expect(html).not.toContain(O_KET_QUA);
    expect(chip(html, "Đã đóng")?.tag).toContain("feedback.resolve");
  });

  it("các trạng thái khác: không có biểu mẫu đóng", () => {
    for (const status of ["da-tiep-nhan", "dang-phan-loai", "dang-xu-ly", "da-dong", "khong-tiep-nhan"]) {
      const html = close(congThaoTac(true, true, true), phieu({ status, channel: "can-bo-nhap-ho" }));
      expect(html, status).not.toContain(O_KET_QUA);
    }
  });
});

describe("lý do nhánh rẽ không rời khỏi thân POST", () => {
  it("mã nguồn màn hình không đụng tới bộ nhớ trình duyệt hay console", () => {
    // Lý do là chữ cán bộ gõ về việc của một công dân (luật 3). Nó chỉ được đi vào thân POST.
    for (const tep of ["so-phan-anh.tsx", "nhan-phieu.ts", "nhat-ky-phieu.tsx"]) {
      const nguon = readFileSync(join(THU_MUC, tep), "utf-8");
      expect(nguon, tep).not.toMatch(/localStorage|sessionStorage|indexedDB|document\.cookie/);
      expect(nguon, tep).not.toMatch(/console\./);
    }
  });
});

/**
 * NHẬT KÝ XỬ LÝ (§8.7). Canh cả CA BỊ TỪ CHỐI: tài khoản người viết mã có mọi khoá, nên một nút
 * `Ghi nhật ký` lọt ra khi chưa rõ quyền là thứ không ai thấy trong lúc phát triển.
 */
describe("nhật ký xử lý — khối, nút ghi, các dòng", () => {
  const TB = new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG ỦY"]]);
  const DB = DANH_BA.ok ? danhBaTheoMa(DANH_BA.duLieu.items) : null;
  const NUT_GHI = `>${NHAN_NUT_GHI_NHAT_KY}</button>`;

  function dong(sua: Partial<petitions_nhatKyPhieuRa> = {}): petitions_nhatKyPhieuRa {
    return {
      id: "01JDONG1",
      at: "2026-09-26T03:05:00Z",
      actor_code: "CB-00200",
      action: "ghi-chu",
      status: "dang-xu-ly",
      unit: "",
      assignee: "",
      note: "",
      attachments: [],
      ...sua,
    };
  }

  it("có `feedback.read`: khối nhật ký, the open box and `Ghi nhật ký`; title has no icon", () => {
    const html = renderToStaticMarkup(
      <NhatKyPhieu maTraCuu="PA-2026-0021" tenBoPhan={TB} danhBa={DB} coNutGhi={true} />,
    );
    expect(html).toMatch(new RegExp(`<h3[^>]*>${TIEU_DE_NHAT_KY}</h3>`));
    expect(html).not.toContain("lucide-history");
    expect(html).toContain(NUT_GHI);
    expect(html).toContain(nhuTrongHTML(GOI_Y_GHI_NHAT_KY));
    // The log is ALWAYS internal (ADR 0041 §Sửa đổi 09/10/2026): a plain statement, no pill, no "?".
    expect(html).toContain(LOG_INTERNAL_NOTE);
    expect(html).not.toContain(nhuTrongHTML(pendingMarkerLabel("Nội bộ")));
  });

  it("CA BỊ TỪ CHỐI — không có quyền (hoặc phiên chưa rõ): KHÔNG có nút ghi, vẫn đọc được", () => {
    const html = renderToStaticMarkup(
      <NhatKyPhieu maTraCuu="PA-2026-0021" tenBoPhan={TB} danhBa={DB} coNutGhi={false} />,
    );
    expect(html).toContain(TIEU_DE_NHAT_KY);
    expect(html).not.toContain(NUT_GHI);
    expect(html).not.toContain(nhuTrongHTML(GOI_Y_GHI_NHAT_KY));
  });

  it("chi tiết phiếu: mặc định KHÔNG có nút ghi (fail closed); có khi được truyền quyền", () => {
    expect(veChiTiet(congThaoTac(true, true, true))).not.toContain(NUT_GHI);
    const html = renderToStaticMarkup(
      <ChiTietPhieu
        phieu={phieu()}
        bayGio={BAY_GIO}
        cong={congThaoTac(false, false, false)}
        tenBoPhan={TEN_BO_PHAN}
        boPhan={BO_PHAN}
        danhBa={DANH_BA}
        dangGui={false}
        loiGhi={null}
        coGhiNhatKy={true}
        dong={() => {}}
        phanLoai={() => {}}
        chuyenXuLy={() => {}}
        tienTrangThai={() => {}}
        dongPhieuLai={() => {}}
        khongTiepNhan={() => {}}
        chuyenCapTren={() => {}}
      />,
    );
    expect(html).toContain(NUT_GHI);
  });

  it("rỗng: the prototype's sentence (`FeedbackActivityPanel.tsx:237-239`)", () => {
    const html = renderToStaticMarkup(<DanhSachNhatKy dong={[]} tenBoPhan={TB} danhBa={DB} />);
    expect(html).toContain(NHAT_KY_RONG);
    expect(NHAT_KY_RONG).toBe("Chưa có ghi chép nào.");
  });

  it("một dòng: initials avatar, HỌ TÊN kèm MÃ, giờ Việt Nam, the act, the status pill", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy dong={[dong({ action: "chuyen-trang-thai" })]} tenBoPhan={TB} danhBa={DB} />,
    );
    // 03:05Z = 10:05 giờ Việt Nam, date first like the prototype's `formatDateTime`.
    expect(html).toContain("26/09/2026 10:05");
    expect(html).toContain("Chuyển trạng thái");
    expect(html).toMatch(/bg-brand\/12[^"]*"[^>]*>Đang xử lý</);
    // PA-06: họ tên tra từ danh bạ màn hình đã đọc một lần, MÃ vẫn ở trên dòng (luật 6, bất biến 8).
    expect(html).toContain("<b class=\"text-navy text-[12.5px]\">Lê Văn C (CB-00200)</b>");
    // The avatar: initials of the name, never the code.
    expect(html).toMatch(/aria-hidden="true"[^>]*>VC<\/span>/);
    // Dòng không phải `phan-cong` thì không có dòng bộ phận.
    expect(html).not.toContain("Bộ phận: ");
  });

  it("status pill ONLY when the status changed from the row before (older); a note keeps none", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy
        dong={[
          dong({ id: "3", action: "ghi-chu", status: "dang-xu-ly", note: "Đã gọi điện." }),
          dong({ id: "2", action: "chuyen-trang-thai", status: "dang-xu-ly" }),
          dong({ id: "1", action: "phan-cong", status: "da-chuyen-xu-ly" }),
        ]}
        tenBoPhan={TB}
        danhBa={DB}
      />,
    );
    // Three rows, two status changes: the note on an unchanged status carries no pill.
    expect(html.match(/inline-block rounded-full bg-brand\/12/g)?.length).toBe(2);
  });

  it("dòng `phan-cong`: “Bộ phận: X” and “Phụ trách: Y” lines (prototype `:282-299`)", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy
        dong={[dong({ action: "phan-cong", unit: "01JBOPHAN", assignee: "CB-00123" })]}
        tenBoPhan={TB}
        danhBa={DB}
      />,
    );
    expect(html).toMatch(/Bộ phận: (<!-- -->)?<\/span><b[^>]*>VĂN PHÒNG ĐẢNG ỦY<\/b>/);
    expect(html).toMatch(/Phụ trách: (<!-- -->)?<\/span><b[^>]*>Trần Thị B<\/b>/);
    expect(html).toContain("Chuyển xử lý");
    expect(NHAN_BO_PHAN_PHU_TRACH).toBe("Bộ phận / Phụ trách");
  });

  it("ghi chú: chữ được THOÁT (không HTML nào chạy), xuống dòng giữ bằng lớp CSS", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy
        dong={[dong({ note: "Dòng 1\nDòng 2 <img src=x onerror=alert(1)>" })]}
        tenBoPhan={TB}
        danhBa={DB}
      />,
    );
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img");
    expect(html).toContain("Dòng 1\nDòng 2");
    expect(html).toContain('class="ghi-chu-nhat-ky"');
  });

  it("mã nguồn nhật ký KHÔNG dùng `dangerouslySetInnerHTML`", () => {
    const nguon = readFileSync(join(THU_MUC, "nhat-ky-phieu.tsx"), "utf-8");
    // Chữ ấy được nhắc trong chú thích để nói vì sao không dùng; canh THUỘC TÍNH JSX.
    expect(nguon).not.toMatch(/dangerouslySetInnerHTML\s*=/);
  });

  it("biểu mẫu ghi (always open): labelled 3-row box, the hint, the reminder, `Ghi nhật ký` with Send", () => {
    const html = renderToStaticMarkup(
      <BieuMauGhiNhatKy
        id="bm"
        noiDung=""
        datNoiDung={() => {}}
        dangGui={false}
        loi={null}
        gui={() => {}}
      />,
    );
    expect(html).toContain('for="bm-noi-dung"');
    expect(html).toMatch(/<textarea[^>]*id="bm-noi-dung"[^>]*rows="3"|<textarea[^>]*rows="3"[^>]*id="bm-noi-dung"/);
    expect(html).toContain(nhuTrongHTML(GOI_Y_GHI_NHAT_KY));
    expect(html).toContain(nhuTrongHTML(NHAC_DU_LIEU_CA_NHAN));
    expect(html).toContain("lucide-send");
    expect(html).toMatch(/>Ghi nhật ký<\/button>/);
    // No `Huỷ`: the box is not a toggled form any more (prototype `:159-231`).
    expect(html).not.toContain(">Huỷ<");
    // Trống thì nút lưu khoá.
    expect(submitTag(html)).toContain("disabled");
  });

  it("biểu mẫu ghi: over 2000 characters, the server's limit is said and the button locks", () => {
    const html = renderToStaticMarkup(
      <BieuMauGhiNhatKy id="bm" noiDung={"ệ".repeat(2001)} datNoiDung={() => {}} dangGui={false} loi={null} gui={() => {}} />,
    );
    expect(html).toContain("không được quá 2000 ký tự");
    expect(submitTag(html)).toContain("disabled");
  });

  it("biểu mẫu ghi: câu 403/400 của máy chủ ra NGUYÊN VĂN", () => {
    const cau = "Bạn không được phân công phiếu này.";
    const html = renderToStaticMarkup(
      <BieuMauGhiNhatKy
        id="bm"
        noiDung="Đã gọi điện."
        datNoiDung={() => {}}
        dangGui={false}
        loi={cau}
        gui={() => {}}
      />,
    );
    expect(html).toContain(cau);
    expect(submitTag(html)).not.toContain("disabled");
  });
});

describe("`Nội dung cập nhật` trên sáu thao tác — ô nhập", () => {
  it("ô có nhãn gắn đúng id (prototype words), and the line under it says it is NOT sent to the citizen", () => {
    const html = renderToStaticMarkup(<ONhapGhiChuNoiBo id="gc" giaTri="" datGiaTri={() => {}} />);
    expect(html).toContain('for="gc"');
    expect(html).toContain('id="gc"');
    expect(NHAN_O_GHI_CHU_NOI_BO).toBe("Nội dung cập nhật");
    expect(html).toContain(nhuTrongHTML(NHAN_O_GHI_CHU_NOI_BO));
    expect(html).toContain("Đã làm gì, ai làm, còn vướng gì… (không bắt buộc)");
    expect(html).toContain("không gửi người dân");
  });

  it("có mặt ở phân loại, chuyển xử lý, tiến trạng thái, đóng phiếu — each in its chip's composer", () => {
    const ALL = congThaoTac(true, true, true);
    expect(veChiTiet(ALL, phieu({ status: "da-tiep-nhan" }), DANH_BA, "dang-phan-loai")).toContain(
      'id="ghi-chu-phan-loai"',
    );
    expect(veChiTiet(ALL, phieu({ status: "dang-phan-loai" }), DANH_BA, "da-chuyen-xu-ly")).toContain(
      'id="ghi-chu-phan-cong"',
    );
    expect(veChiTiet(ALL, phieu({ status: "da-chuyen-xu-ly" }), DANH_BA, "dang-xu-ly")).toContain('id="ghi-chu-tien"');
    expect(
      veChiTiet(congThaoTac(false, false, true), phieu({ status: "cho-dan-xac-nhan" }), DANH_BA, "da-dong"),
    ).toContain('id="ghi-chu-dong"');
  });

  it("có mặt ở hai nhánh rẽ, TÁCH khỏi ô lý do người dân đọc", () => {
    for (const loai of ["khong-tiep-nhan", "chuyen-cap-tren"] as const) {
      const html = renderToStaticMarkup(
        <BieuMauReNhanh loai={loai} dangGui={false} gui={() => {}} huy={() => {}} />,
      );
      expect(html, loai).toContain(`id="ghi-chu-${loai}"`);
      expect(html, loai).toContain(`id="ly-do-${loai}"`);
    }
  });

  it("ghi chú quá 2000 ký tự khoá nút gửi của nhánh rẽ, dù lý do hợp lệ", () => {
    const html = renderToStaticMarkup(
      <BieuMauReNhanh
        loai="khong-tiep-nhan"
        dangGui={false}
        gui={() => {}}
        huy={() => {}}
        lyDoBanDau="Vượt thẩm quyền của xã."
        ghiChuBanDau={"ệ".repeat(2001)}
      />,
    );
    expect(submitTag(html)).toContain("disabled");
  });
});

describe("Đóng phiếu — `has_citizen` thắng kênh khi có mặt", () => {
  it("kênh công dân nhưng `has_citizen: false`: có biểu mẫu đóng ở `da-xu-ly`", () => {
    const html = veChiTiet(
      congThaoTac(false, false, true),
      phieu({ status: "da-xu-ly", channel: "zalo-mini-app", has_citizen: false }),
      DANH_BA,
      "da-dong",
    );
    expect(html).toContain(O_KET_QUA);
  });

  it("kênh nhập hộ nhưng `has_citizen: true`: KHÔNG có biểu mẫu đóng ở `da-xu-ly`", () => {
    const html = veChiTiet(
      congThaoTac(false, false, true),
      phieu({ status: "da-xu-ly", channel: "can-bo-nhap-ho", has_citizen: true }),
      DANH_BA,
      "da-dong",
    );
    expect(html).not.toContain(O_KET_QUA);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * RATING, REOPENING, PUBLIC-PAGE MODERATION (ADR 0050 points 2 and 8)
 *
 * The denied case is tested as hard as the allowed one: the developer's account holds every key, so
 * a button leaking past its gate is invisible during development.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const NUT_CONG_KHAI = "Cho hiện công khai</button>";
const NUT_AN = "Ẩn khỏi trang công khai</button>";

function veChiTietVoiCongKhai(
  cong: ReturnType<typeof congThaoTac>,
  p: petitions_phieuPhanAnhRa,
  loiGhi: string | null = null,
): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={BAY_GIO}
      cong={cong}
      tenBoPhan={TEN_BO_PHAN}
      boPhan={BO_PHAN}
      danhBa={DANH_BA}
      dangGui={false}
      loiGhi={loiGhi}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
      setPublication={() => {}}
    />,
  );
}

describe("drawer — moderation buttons (§8.3), gated by `feedback.assign`", () => {
  it("WITH `feedback.assign`, pending: both buttons, the state label and its hint", () => {
    const html = veChiTietVoiCongKhai(
      congThaoTac(false, true, false),
      phieu({ publication_status: "cho-duyet" }),
    );
    expect(html).toContain(NUT_CONG_KHAI);
    expect(html).toContain(NUT_AN);
    expect(html).toContain("Chưa cho hiện công khai");
    expect(html).toContain("Người gửi vẫn tra cứu được phiếu của mình");
  });

  it("DENIED — without `feedback.assign`: NO button, and the sentence names the key", () => {
    for (const cong of [congThaoTac(false, false, false), congThaoTac(true, false, true)]) {
      const html = veChiTietVoiCongKhai(cong, phieu({ publication_status: "cho-duyet" }));
      expect(html).not.toContain(NUT_CONG_KHAI);
      expect(html).not.toContain(NUT_AN);
      expect(html).toContain("không đổi được việc hiển thị phiếu này trên trang công khai");
      expect(html).toContain("feedback.assign");
    }
  });

  it("public: only `Ẩn`; hidden: only `Cho hiện`", () => {
    const cong = veChiTietVoiCongKhai(
      congThaoTac(false, true, false),
      phieu({ publication_status: "cong-khai", public: true }),
    );
    expect(cong).toContain("Đang hiện công khai");
    expect(cong).not.toContain(NUT_CONG_KHAI);
    expect(cong).toContain(NUT_AN);

    const an = veChiTietVoiCongKhai(congThaoTac(false, true, false), phieu({ publication_status: "an" }));
    expect(an).toContain("Không cho hiện công khai");
    expect(an).toContain(NUT_CONG_KHAI);
    expect(an).not.toContain(NUT_AN);
  });

  it("`can-bo` petition: NEVER `Cho hiện công khai`, even with the key", () => {
    const html = veChiTietVoiCongKhai(
      congThaoTac(true, true, true),
      phieu({ field: "can-bo", field_label: "", publication_status: "an" }),
    );
    expect(html).not.toContain(NUT_CONG_KHAI);
    // The box's pre-action hint (no publish button is offered), worded as the message's default.
    expect(html).toContain("Phản ánh về thái độ, tác phong cán bộ không được hiển thị công khai.");
  });

  it("no write path passed (default): read-only, whatever the permissions", () => {
    // `veChiTiet` passes no `setPublication`.
    const html = veChiTiet(congThaoTac(true, true, true), phieu({ publication_status: "cho-duyet" }));
    expect(html).not.toContain(NUT_CONG_KHAI);
    expect(html).not.toContain(NUT_AN);
    expect(html).toContain("Chưa cho hiện công khai");
  });

  it("409 `never_public`: the server's sentence shows in the drawer's alert line, verbatim", () => {
    // A commune-reworded `feedback.never_public` — NOT the default — so the test proves the line
    // shows what the server sent, not a sentence the web keeps.
    const cau = "Xã không công khai phản ánh liên quan đến cán bộ, công chức.";
    const html = veChiTietVoiCongKhai(congThaoTac(false, true, false), phieu(), cau);
    expect(html).toContain(`<p class="thong-bao-loi" role="alert">${cau}</p>`);
  });
});

type ThuocTinh = Record<string, unknown> & { children?: ReactNode };

/**
 * Every inline element of a tree, depth first (components are not expanded). Besides `children` it
 * walks the two slot props of `FilterBar` (`primary`, `more`), where the filter row's controls are
 * passed since the 02/10/2026 redesign — otherwise a control moved into a slot would vanish from the
 * walk and the test would fail for a layout reason, not a behaviour one.
 */
function moiPhanTu(nut: ReactNode, ra: ReactElement<ThuocTinh>[] = []): ReactElement<ThuocTinh>[] {
  if (Array.isArray(nut)) {
    for (const con of nut) moiPhanTu(con as ReactNode, ra);
    return ra;
  }
  if (nut !== null && typeof nut === "object" && "props" in nut) {
    const pt = nut as ReactElement<ThuocTinh>;
    ra.push(pt);
    moiPhanTu(pt.props.children, ra);
    moiPhanTu(pt.props.primary as ReactNode, ra);
    moiPhanTu(pt.props.more as ReactNode, ra);
  }
  return ra;
}

function textOf(nut: ReactNode): string {
  if (typeof nut === "string" || typeof nut === "number") return String(nut);
  if (Array.isArray(nut)) return nut.map(textOf).join("");
  if (nut !== null && typeof nut === "object" && "props" in nut) {
    return textOf((nut as ReactElement<ThuocTinh>).props.children);
  }
  return "";
}

describe("moderation buttons send the right target", () => {
  it("`Cho hiện` → `cong-khai`, `Ẩn` → `an`", () => {
    const setPublication = vi.fn();
    // PublicationBox has no hooks, so it can be called as a function and its buttons driven directly.
    const tree = PublicationBox({
      petition: phieu({ publication_status: "cho-duyet" }),
      mayModerate: true,
      setPublication,
    });
    const buttons = moiPhanTu(tree).filter((e) => e.type === "button");
    const byText = (t: string) => buttons.find((b) => textOf(b.props.children).includes(t));
    (byText("Cho hiện công khai")?.props.onClick as () => void)();
    (byText("Ẩn khỏi trang công khai")?.props.onClick as () => void)();
    expect(setPublication.mock.calls).toEqual([["cong-khai"], ["an"]]);
  });

  it("buttons are disabled while a write is in flight", () => {
    const html = renderToStaticMarkup(
      <PublicationBox
        petition={phieu({ publication_status: "cho-duyet" })}
        mayModerate={true}
        busy={true}
        setPublication={() => {}}
      />,
    );
    // The ATTRIBUTE, not the word: the class string carries `disabled:` variants since 02/10/2026.
    expect(html.match(/<button[^>]*\sdisabled=""/g)?.length).toBe(2);
  });
});

describe("drawer — rating block and reopen line", () => {
  it("not rated: NO rating section (prototype `FeedbackDetailDrawer.tsx:441`), no reopen line", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ reopen_count: 0 }));
    expect(html).not.toContain("Đánh giá của người dân");
    expect(html).not.toContain("Đã mở lại");
  });

  it("1 star, reopened twice: danger box, star icons, comment, red sentence, reopen line under the deadlines", () => {
    const html = veChiTiet(
      congThaoTac(false, false, false),
      phieu({
        rating: 1,
        rating_comment: "Chưa ai đến xem.",
        rated_at: "2026-09-28T03:05:00Z",
        reopen_count: 2,
      }),
    );
    expect(html).toContain("Đánh giá của người dân");
    expect(html).toContain("border-danger/25 bg-danger/8");
    expect(html.match(/lucide-star /g)?.length).toBe(5);
    expect(html.match(/fill-tangerine text-tangerine/g)?.length).toBe(1);
    expect(html).toContain('aria-label="1/5 sao"');
    expect(html).toContain(">1/5</span>");
    expect(html).toContain("“Chưa ai đến xem.”");
    expect(html).toMatch(/text-danger">Đánh giá thấp — phiếu đã tự mở lại để xử lý tiếp\.<\/p>/);
    expect(html).toContain('<p class="nhan-lech">Đã mở lại 2 lần do người dân chấm điểm thấp</p>');
    // The reopen line sits in the `Hạn xử lý xong` cell, after the deadline.
    expect(html.indexOf("Hạn xử lý xong")).toBeLessThan(html.indexOf("Đã mở lại 2 lần"));
    expect(html.indexOf("Đã mở lại 2 lần")).toBeLessThan(html.indexOf("Đang giao cho"));
  });

  it("5 stars: leaf box, five filled stars, no red sentence", () => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ rating: 5, reopen_count: 1 }));
    expect(html).toContain("border-leaf/25 bg-leaf/8");
    expect(html.match(/fill-tangerine text-tangerine/g)?.length).toBe(5);
    expect(html).not.toContain("Đánh giá thấp");
    // An earlier reopening is still on record and still said.
    expect(html).toContain("Đã mở lại 1 lần");
  });

  it("card corner: stars once rated, nothing before", () => {
    const the = (p: petitions_phieuPhanAnhRa) =>
      renderToStaticMarkup(
        <ThePhieu phieu={p} bayGio={BAY_GIO} dangMo={false} mo={() => {}} />,
      );
    expect(the(phieu({ rating: 2 }))).toContain('aria-label="2/5 sao"');
    expect(the(phieu({ rating: 2 }))).toContain("★★☆☆☆");
    expect(the(phieu())).not.toContain("★");
  });
});

describe("filter `Bị đánh giá thấp` (§4) → `ratingMax: 2`", () => {
  function hang(loc: Parameters<typeof HangLoc>[0]["loc"], datLoc = vi.fn()) {
    return {
      datLoc,
      tree: HangLoc({ loc, tim: "", datTim: () => {}, datLoc, thon: [] }),
    };
  }

  it("the box exists, labelled, unticked by default; ticked when the filter is on", () => {
    expect(renderToStaticMarkup(hang({}).tree)).toMatch(
      /<input id="loc-danh-gia-thap" type="checkbox"( class="[^"]*")?\/> (<!-- -->)?Bị đánh giá thấp/,
    );
    expect(renderToStaticMarkup(hang({ ratingMax: 2 }).tree)).toMatch(
      /id="loc-danh-gia-thap" type="checkbox"[^>]*checked=""/,
    );
  });

  it("ticking sets `ratingMax: 2` and keeps the other filters; unticking removes it", () => {
    const { tree, datLoc } = hang({ trangThai: "da-dong", chiTreHan: true });
    const o = moiPhanTu(tree).find((e) => e.props.id === "loc-danh-gia-thap");
    (o?.props.onChange as (e: { target: { checked: boolean } }) => void)({ target: { checked: true } });
    expect(datLoc).toHaveBeenLastCalledWith({ trangThai: "da-dong", chiTreHan: true, ratingMax: 2 });

    const tat = hang({ ratingMax: 2 });
    const o2 = moiPhanTu(tat.tree).find((e) => e.props.id === "loc-danh-gia-thap");
    (o2?.props.onChange as (e: { target: { checked: boolean } }) => void)({ target: { checked: false } });
    expect(tat.datLoc).toHaveBeenLastCalledWith({ ratingMax: undefined });
  });
});

describe("log — the two citizen-rating rows", () => {
  const TB = new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG ỦY"]]);
  // A directory that WOULD match if the marker were looked up — it must not be.
  const DB = danhBaTheoMa([
    { code: "cong-dan", full_name: "Không được hiện", position: "", department_id: "", email_masked: null },
  ]);

  function dongNhatKy(sua: Partial<petitions_nhatKyPhieuRa>): petitions_nhatKyPhieuRa {
    return {
      id: "01JDONG9",
      at: "2026-09-28T03:05:00Z",
      actor_code: "cong-dan",
      action: "danh-gia",
      status: "cho-dan-xac-nhan",
      unit: "",
      assignee: "",
      note: "Người dân đánh giá 4 sao",
      attachments: [],
      ...sua,
    };
  }

  it("`danh-gia`: labelled, actor reads “Người dân”, never the directory", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy dong={[dongNhatKy({})]} tenBoPhan={TB} danhBa={DB} />,
    );
    expect(html).toMatch(/>Người dân đánh giá<\/span>/);
    expect(html).toMatch(/<b[^>]*>Người dân<\/b>/);
    expect(html).not.toContain("Không được hiện");
    expect(html).not.toContain(">cong-dan<");
    expect(html).toContain("Người dân đánh giá 4 sao");
  });

  it("`mo-lai-theo-danh-gia`: labelled, with the server's note", () => {
    const html = renderToStaticMarkup(
      <DanhSachNhatKy
        dong={[
          dongNhatKy({
            action: "mo-lai-theo-danh-gia",
            status: "dang-xu-ly",
            note: "Người dân đánh giá 2 sao — phiếu được mở lại",
          }),
        ]}
        tenBoPhan={TB}
        danhBa={DB}
      />,
    );
    expect(html).toMatch(/>Mở lại do đánh giá thấp<\/span>/);
    expect(html).toContain("Người dân đánh giá 2 sao — phiếu được mở lại");
    expect(html).not.toContain("chưa có nhãn");
  });
});

/**
 * PA-03 (tester report 05/10/2026): a staff-booked petition sits in `da-tiep-nhan` with its field
 * chosen at intake, and the server refuses `Chuyển xử lý` from there with 409 (ADR 0027). The drawer
 * must (a) open the classify select on that field, (b) not offer a form whose only answer is a refusal,
 * (c) say an assignment refusal next to the assign button, not at the top of the card.
 */
describe("PA-03 — phân loại trước, chuyển xử lý sau", () => {
  function veVoi(
    p: petitions_phieuPhanAnhRa,
    cong: ReturnType<typeof congThaoTac>,
    assignRefusal: string | null = null,
    initialStep: string | null = null,
  ) {
    return renderToStaticMarkup(
      <ChiTietPhieu
        initialStep={initialStep}
        phieu={p}
        bayGio={BAY_GIO}
        cong={cong}
        tenBoPhan={TEN_BO_PHAN}
        boPhan={BO_PHAN}
        danhBa={DANH_BA}
        dangGui={false}
        loiGhi={null}
        dong={() => {}}
        phanLoai={() => {}}
        chuyenXuLy={() => {}}
        tienTrangThai={() => {}}
        dongPhieuLai={() => {}}
        khongTiepNhan={() => {}}
        chuyenCapTren={() => {}}
        assignRefusal={assignRefusal}
      />,
    );
  }

  /** The `<option>` the classify select marks as selected, read from the static markup. */
  function linhVucDangChon(html: string): string | null {
    const o = html.match(/<select id="chon-linh-vuc"[^>]*>([\s\S]*?)<\/select>/)?.[1] ?? "";
    return o.match(/<option value="([^"]*)" selected="">/)?.[1] ?? null;
  }

  it("(a) ô Lĩnh vực mở sẵn lĩnh vực đã chọn lúc nhập hộ — nút Xác nhận bấm được ngay", () => {
    const html = veVoi(
      phieu({ status: "da-tiep-nhan", channel: "can-bo-nhap-ho", field: "giao-thong" }),
      congThaoTac(true, false, false),
      null,
      "dang-phan-loai",
    );
    expect(linhVucDangChon(html)).toBe("giao-thong");
    expect(submitTag(html)).not.toContain("disabled");
  });

  it("(a) phiếu chưa có lĩnh vực: ô để trống, nút khoá", () => {
    const html = veVoi(phieu({ status: "da-tiep-nhan", field: "" }), congThaoTac(true, false, false), null, "dang-phan-loai");
    expect(linhVucDangChon(html)).toBe("");
    expect(submitTag(html)).toContain("disabled");
  });

  it("(b) CA BỊ TỪ CHỐI — `da-tiep-nhan`, có `feedback.assign`: `Đã chuyển xử lý` is not a move from here", () => {
    const html = veVoi(phieu({ status: "da-tiep-nhan" }), congThaoTac(false, true, false), null, "da-chuyen-xu-ly");
    expect(html).not.toContain('id="chon-bo-phan"');
    expect(chip(html, "Đã chuyển xử lý")?.tag).toContain(nhuTrongHTML(STRIP_NOT_A_STEP));
  });

  it("(b) đã phân loại (`dang-phan-loai`): the chip opens the assign act", () => {
    const html = veVoi(phieu({ status: "dang-phan-loai" }), congThaoTac(false, true, false), null, "da-chuyen-xu-ly");
    expect(chip(html, "Đã chuyển xử lý")?.role).toBe("chuyển sang");
    expect(html).toContain('id="chon-bo-phan"');
    expect(html).toContain(nhuTrongHTML(composerTitle("da-chuyen-xu-ly")));
  });

  it("(b) DENIED — thiếu `feedback.assign` at `dang-phan-loai`: the chip names the key, no picker", () => {
    const html = veVoi(phieu({ status: "dang-phan-loai" }), congThaoTac(false, false, false), null, "da-chuyen-xu-ly");
    expect(chip(html, "Đã chuyển xử lý")?.tag).toContain("feedback.assign");
    expect(html).not.toContain('id="chon-bo-phan"');
  });

  it("(c) câu từ chối chuyển xử lý nằm TRONG biểu mẫu chuyển xử lý, ngay trước nút", () => {
    const cau = "Phiếu phải được phân loại trước khi chuyển xử lý.";
    const html = veVoi(phieu({ status: "dang-phan-loai" }), congThaoTac(false, true, false), cau, "da-chuyen-xu-ly");
    const form = html.match(/<form[^>]*>(?:(?!<\/form>)[\s\S])*id="chon-bo-phan"[\s\S]*?<\/form>/)?.[0] ?? "";
    expect(form).toContain(cau);
    expect(form.indexOf(cau)).toBeLessThan(form.indexOf(">Xác nhận</button>"));
  });

  it("(c) the same in the hand-over section (later statuses), before `Chuyển xử lý`", () => {
    const cau = "Bộ phận này không nhận phiếu lĩnh vực ấy.";
    const html = veVoi(phieu({ status: "dang-xu-ly" }), congThaoTac(false, true, false), cau);
    const form = html.match(/<form[^>]*>(?:(?!<\/form>)[\s\S])*id="chon-bo-phan"[\s\S]*?<\/form>/)?.[0] ?? "";
    expect(form).toContain(cau);
    expect(form.indexOf(cau)).toBeLessThan(form.indexOf(">Chuyển xử lý</button>"));
  });

  it("(c) không có lời từ chối: biểu mẫu chuyển xử lý không có dòng báo lỗi", () => {
    const html = veVoi(phieu({ status: "dang-phan-loai" }), congThaoTac(false, true, false), null, "da-chuyen-xu-ly");
    const form = html.match(/<form[^>]*>(?:(?!<\/form>)[\s\S])*id="chon-bo-phan"[\s\S]*?<\/form>/)?.[0] ?? "";
    expect(form).not.toBe("");
    expect(form).not.toContain('role="alert"');
  });

  it("PA-07: nhãn hạn phân loại viết bằng lời hành chính", () => {
    const html = veVoi(phieu(), congThaoTac(false, false, false));
    expect(html).toContain("Hạn phân loại");
    expect(html).not.toContain("Trần phân loại");
  });
});

/**
 * ADR 0068 lần 5 — the prototype's composition: cards in the list, the drawer as a right-hand dialog
 * with its three fact cells, and the "?" where the prototype shows what the backend cannot give.
 */
describe("prototype composition — list cards", () => {
  const card = (p: petitions_phieuPhanAnhRa) =>
    renderToStaticMarkup(<ThePhieu phieu={p} bayGio={BAY_GIO} dangMo={false} mo={() => {}} />);

  it("a card is ONE button that opens the drawer dialog; code, field, content, place, sender", () => {
    const html = card(phieu());
    expect(html.startsWith('<button type="button" aria-haspopup="dialog"')).toBe(true);
    expect(html).toContain("PA-2026-0021");
    expect(html).toContain("Rác thải – Vệ sinh môi trường");
    expect(html).toContain("Tổ 6, thôn Hà Lam");
    expect(html).toContain("Nguyễn V. A. · 09****0000");
  });

  it("the tile is a neutral placeholder: NO image is loaded, no photo link is signed", () => {
    const html = card(phieu());
    expect(html).not.toContain("<img");
    expect(html).not.toMatch(/src="/);
  });

  it("no address: the prototype's fallback words; no duplicate count is ever invented", () => {
    const html = card(phieu({ address: "" }));
    expect(html).toContain("Chưa rõ vị trí");
    expect(html).not.toContain("phiếu trùng");
  });

  it("the hamlet recorded on the petition takes the place line (hamlet ?? address, ADR 0088 §1)", () => {
    const html = card(phieu({ residential_unit_id: "01JTHON1", residential_unit_name: "Thôn Bình An" }));
    expect(html).toContain("Thôn Bình An");
    expect(html).not.toContain("Tổ 6, thôn Hà Lam");
  });

  it("past the resolve deadline: marked, with the clock icon and the words — never colour alone", () => {
    const late = card(phieu({ resolve_due: "2026-09-09T08:00:00Z" }));
    expect(late).toContain("data-tre-han");
    expect(late).toContain("lucide-alarm-clock");
    expect(card(phieu())).not.toContain("data-tre-han");
  });

  it("not rated: the channel sits in the corner instead of stars; `web-xa` reads “Web của xã”", () => {
    expect(card(phieu())).toContain("Zalo Mini App");
    expect(card(phieu({ channel: "web-xa" }))).toContain("Web của xã");
  });

  it("the prototype's card (`FeedbackCard`, rows 19-28): shadow, code chip, brand field chip, no open ring", () => {
    const html = card(phieu());
    const tag = html.match(/<button[^>]*>/)?.[0] ?? "";
    expect(tag).toContain("shadow-card");
    expect(tag).not.toContain("ring-2");
    expect(html).toMatch(/bg-\[#F7FAFC\][^"]*text-\[10\.5px\][^"]*"[^>]*>PA-2026-0021</);
    expect(html).toMatch(/bg-brand\/12[^"]*text-brand"[^>]*>Rác thải/);
    expect(html).toContain("line-clamp-2 text-[12.8px]");
    // Status chip in the prototype's colours, icon + word (ADR 0068 lần 6 #7).
    expect(html).toMatch(/bg-teal\/12 text-teal[^"]*"><svg[\s\S]*?<\/svg>Đang xử lý<\/span>/);
  });

  it("stars: danger at 1–2, tangerine from 3", () => {
    expect(card(phieu({ rating: 2 }))).toMatch(/text-\[11\.5px\] text-danger"[^>]*>★★☆☆☆/);
    expect(card(phieu({ rating: 4 }))).toMatch(/text-\[11\.5px\] text-tangerine"[^>]*>★★★★☆/);
  });

  it("empty register: the prototype's two lines in a white card", () => {
    const html = renderToStaticMarkup(
      <DanhSachThe
        phieu={[]}
        bayGio={BAY_GIO}
        maDangMo={null}
        moPhieu={() => {}}
        empty={undefined}
      />,
    );
    // No filter sentence passed → the list's own (filter) sentence; the screen passes the two lines.
    expect(html).toContain("shadow-card");
    expect(LIST_EMPTY_TITLE).toBe("Chưa có phản ánh nào");
    expect(LIST_EMPTY_HINT).toBe("Phiếu gửi từ Zalo Mini App sẽ hiện ở đây ngay khi người dân bấm gửi.");
  });
});

describe("the filter row (rows 9-14)", () => {
  const html = renderToStaticMarkup(
    <HangLoc loc={{}} tim="" datTim={() => {}} datLoc={() => {}} thon={[]} />,
  );

  it("scope: the prototype's bordered group, active = navy, each choice's hint as its title", () => {
    expect(html).toMatch(/<button type="button" title="Tất cả hồ sơ trong xã" class="[^"]*bg-navy text-white/);
    expect(html).toContain('title="Đích danh tôi là người xử lý"');
    expect(html).toContain('title="Tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ"');
    expect(html.match(/class="h-9 [^"]*text-\[12\.5px\] font-semibold/g)?.length).toBe(3);
  });

  it("search: 256px box, the prototype's placeholder; selects and box are 36px / 12.5px", () => {
    expect(html).toContain('placeholder="Tìm theo nội dung, mã phiếu, địa chỉ…"');
    expect(html).toContain("w-64");
    expect(html.match(/\[&amp;_select\]:h-9!/g)?.length).toBe(4);
  });

  it("the prototype's row only: NO unit and NO intake-channel filter (customer bug sheet row 56)", () => {
    expect(html).not.toContain('id="loc-bo-phan"');
    expect(html).not.toContain('id="loc-kenh"');
    expect(html.match(/<select /g)?.length).toBe(3);
  });

  it("checkboxes: 12.5px words, a 14px brand box", () => {
    expect(html.match(/class="accent-brand size-3\.5"/g)?.length).toBe(2);
    expect(html).toContain("Chỉ phiếu trễ hạn");
    expect(html).toContain("Bị đánh giá thấp");
  });
});

describe("prototype composition — the drawer", () => {
  const ALL = congThaoTac(true, true, true);

  it("is a dialog named by code · channel · time — never by the citizen's words; the field is the title", () => {
    const html = veChiTiet(ALL);
    expect(html).toMatch(/<dialog[^>]*aria-labelledby="tieu-de-chi-tiet-phieu"/);
    const name = html.match(/<p id="tieu-de-chi-tiet-phieu"[^>]*>([\s\S]*?)<\/p>/)?.[1] ?? "";
    expect(name).toContain("PA-2026-0021");
    // Date first, Vietnam time (07:21Z = 14:21).
    expect(name).toContain("09/09/2026 14:21");
    expect(name).not.toContain("Rác tồn đọng");
    expect(html).toMatch(/<h2 class="[^"]*text-\[15px\][^"]*">Rác thải – Vệ sinh môi trường<\/h2>/);
  });

  it("the prototype's drawer frame: 72rem, never past 98vw, navy veil, left-cast shadow", () => {
    const tag = veChiTiet(ALL).match(/<dialog[^>]*>/)?.[0] ?? "";
    expect(tag).toContain("md:w-[72rem]");
    expect(tag).toContain("max-w-[98vw]");
    expect(tag).toContain("backdrop:bg-navy/30");
    expect(tag).toContain("md:shadow-[-10px_0_36px_rgba(16,43,67,0.18)]");
  });

  it("header: the sender in 11.5px; neither half sent → “Không rõ người gửi”", () => {
    expect(veChiTiet(ALL, phieu({ reporter_name: "", reporter_phone: "" }))).toContain("Không rõ người gửi");
  });

  it("facts: 10.5px labels; an unassigned petition says so in the prototype's words", () => {
    const html = veChiTiet(ALL, phieu({ unit: "", assignee: "" }));
    expect(html).toContain("Chưa giao bộ phận nào");
    expect(html).toContain("Chưa chỉ định cán bộ xử lý");
    expect(html.match(/text-\[10\.5px\] font-bold tracking-wide text-ink-muted uppercase/g)?.length).toBe(3);
    // The publication state is words only — no glyph before it.
    expect(html).not.toContain("lucide-eye");
  });

  it("three fact cells in the prototype's order, both clocks in the first", () => {
    const html = veChiTiet(ALL);
    const order = ["Hạn xử lý xong", "Hạn tiếp nhận", "Hạn phân loại", "Đang giao cho", "Hiển thị với người dân"];
    const at = order.map((w) => html.indexOf(w));
    expect(at.every((i) => i >= 0)).toBe(true);
    expect([...at].sort((a, b) => a - b)).toEqual(at);
  });

  it("sections in the prototype's order: content → location → duplicates → rating → hand-over → log", () => {
    const html = veChiTietWith(ALL, phieu({ lat: 21.028511, lng: 105.804817, rating: 4 }), ["feedback.read"]);
    const order = [
      "Nội dung phản ánh",
      ">Vị trí</h3>",
      ">Có thể trùng với phiếu khác</h3>",
      "Đánh giá của người dân",
      "Chuyển xử lý, không đổi trạng thái",
      TIEU_DE_NHAT_KY,
    ].map((w) => html.indexOf(w.startsWith(">") ? w : nhuTrongHTML(w)));
    expect(order.every((i) => i >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    // Flat sections, no card and no icon title (row 52-60).
    expect(html).not.toContain("Xử lý phiếu");
    expect(html).not.toContain("lucide-list-checks");
  });

  it("the duplicates block: no '?' any more; drawn only behind `feedback.read`", () => {
    expect(veChiTiet(ALL)).not.toContain("Có thể trùng với phiếu khác");
    const html = veChiTietWith(ALL, phieu(), ["feedback.read"]);
    expect(html).toContain("Có thể trùng với phiếu khác");
    expect(html).not.toContain("data-pending-marker");
  });
});

describe("self-declared contact — ADR 0080 decision 5", () => {
  // The server's explicit field, and ONLY it, draws the label: `has_citizen: false` on a Mini App
  // petition is also what a staff intake looks like, so it must not be enough on its own.
  const UNVERIFIED = phieu({ contact_unverified: true, has_citizen: false });

  function card(p: petitions_phieuPhanAnhRa): string {
    return renderToStaticMarkup(<ThePhieu phieu={p} bayGio={BAY_GIO} dangMo={false} mo={() => {}} />);
  }

  it("list row: label beside the masked sender when contact_unverified is true", () => {
    const html = card(UNVERIFIED);
    expect(html).toContain("Nguyễn V. A. · 09****0000");
    expect(html).toContain(UNVERIFIED_CONTACT_LABEL);
  });

  it.each([false, undefined, null])("list row: no label when contact_unverified is %s", (v) => {
    expect(card(phieu({ contact_unverified: v }))).not.toContain(UNVERIFIED_CONTACT_LABEL);
  });

  it("list row: has_citizen false alone draws nothing (never derived)", () => {
    expect(card(phieu({ has_citizen: false }))).not.toContain(UNVERIFIED_CONTACT_LABEL);
  });

  it("drawer: label and the visible one-line note", () => {
    const html = veChiTiet(congThaoTac(false, false, false), UNVERIFIED);
    expect(html).toContain(UNVERIFIED_CONTACT_LABEL);
    expect(html).toContain(`>${UNVERIFIED_CONTACT_NOTE}</p>`);
  });

  it.each([false, undefined, null])("drawer: neither label nor note when contact_unverified is %s", (v) => {
    const html = veChiTiet(congThaoTac(false, false, false), phieu({ contact_unverified: v, has_citizen: false }));
    expect(html).not.toContain(UNVERIFIED_CONTACT_LABEL);
    expect(html).not.toContain(UNVERIFIED_CONTACT_NOTE);
  });
});
