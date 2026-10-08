import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhBaChonNguoiRa,
  petitions_loaiNhiemVuRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";

import { BANG_NHAN_MAC_DINH, quyenNhiemVu, TIEU_DE_KHOI_VAN_BAN } from "./nhan-nhiem-vu"; // vi-name-ok: existing exports, imported unchanged (rule 12 inv 3)
import { BangNhiemVu, ChiTietNhiemVu, FormGiaoViec } from "./so-nhiem-vu"; // vi-name-ok: existing components, imported unchanged (rule 12 inv 3)
import type { DanhMucNhiemVu } from "./so-nhiem-vu"; // vi-name-ok: existing type, imported unchanged (rule 12 inv 3)
import { showsDirectiveBlock, TASK_INFO_TITLE } from "./so-nhiem-vu";
import { NOT_SENT, serverTransitions } from "./task-transitions.fixture";

/**
 * The two task facts service-petitions now sends on every read (9f3a21c4) — `extension_count`,
 * `pending_extension` — and the catalogue's `requires_directive`, as the list, the drawer and the
 * create form draw them. Every value below is fake.
 */

const ASSIGNER = "CB-2026-GIALD2";

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Rà soát danh sách giả quý III",
    description: "",
    status: "dang-thuc-hien",
    allowed_transitions: serverTransitions("dang-thuc-hien"),
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: ASSIGNER,
    due_at: "2026-10-20T23:59:59+07:00",
    original_due_at: "2026-10-20T23:59:59+07:00",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    created_by: "CB-2026-VANTHU",
    created_at: "2026-06-01T02:00:00Z",
    updated_at: "2026-06-01T02:00:00Z",
    ...patch,
  };
}

function typeRow(code: string, requiresDirective: boolean, isDefault = false): petitions_loaiNhiemVuRa {
  return {
    id: `01J${code}`,
    code,
    label: code,
    is_default: isDefault,
    active: true,
    order: 1,
    source: "he-thong",
    tier: 3,
    color: null,
    requires_directive: requiresDirective,
  };
}

const catalogue = (types: petitions_loaiNhiemVuRa[]): DanhMucNhiemVu => ({
  loai: types,
  mucUuTien: [],
  khoi: [],
  boPhan: [],
});
const SHIPPED = catalogue([typeRow("theo-van-ban", true), typeRow("co-ban", false, true)]);
const NOW = new Date("2026-09-15T03:00:00Z");
const NOT_CALLED = (): Promise<KetQua<never>> => Promise.resolve({ ok: false, thongBao: "không gọi" });
const DIRECTORY: KetQua<identity_danhBaChonNguoiRa> = { ok: true, duLieu: { items: [] } };

function listHtml(rows: petitions_nhiemVuRa[]): string {
  return renderToStaticMarkup(
    <BangNhiemVu
      nhiemVu={rows}
      danhMuc={SHIPPED}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={new Map()}
      bayGio={NOW}
      maDangMo={null}
      moNhiemVu={() => {}}
      sapXep={{ cot: "created_at", chieu: "desc" }}
      doiSapXep={() => {}}
    />,
  );
}

function drawerHtml(patch: Partial<petitions_nhiemVuRa>, types: DanhMucNhiemVu = SHIPPED): string {
  return renderToStaticMarkup(
    <ChiTietNhiemVu
      nhiemVu={task(patch)}
      vanBan={{ pha: "xong", duLieu: [] }}
      danhMuc={types}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={new Map()}
      bayGio={NOW}
      maNguoiDangNhap="CB-2026-7K3M9Q"
      quyen={quyenNhiemVu([])}
      dangGui={false}
      dong={() => {}}
      doiTrangThai={NOT_SENT}
      xoa={NOT_SENT}
      guiDeNghiLuiHan={NOT_CALLED}
      quyetDinh={NOT_CALLED}
      suaKhoiVanBan={NOT_CALLED}
      docLaiChiTiet={NOT_CALLED}
      extensionRefreshKey="0"
      onExtensionDecided={() => {}}
      openTask={() => {}}
      addChild={null}
      reassign={NOT_CALLED}
    />,
  );
}

/** The `Hạn xử lý` cell of the fact grid, label to the end of its second line. */
function dueFact(html: string): string {
  const start = html.indexOf(">Hạn xử lý</p>");
  return html.slice(start, html.indexOf("</div>", start));
}

describe("list sub-line — `{nguồn} · đã gia hạn n lần` (prototype `TaskListTable.tsx:83-89`)", () => {
  it("n = 0: the source alone, no separator, no \"?\" marker", () => {
    const html = listHtml([task({ extension_count: 0 })]);
    expect(html).toContain('<span class="text-ink-muted block text-[11px]">Giao trực tiếp</span>');
    // The body only: the header keeps its own "?" for the columns the server cannot sort by.
    const body = html.slice(html.indexOf("<tbody>"));
    expect(body).not.toContain("đã gia hạn");
    expect(body).not.toContain("Số lần gia hạn");
    expect(body).not.toContain("data-pending-marker");
  });

  it("n > 0: `· đã gia hạn n lần` after the source, per row", () => {
    const html = listHtml([task({ code: "NV1", extension_count: 1 }), task({ code: "NV2", extension_count: 4 })]);
    expect(html).toContain('<span class="text-ink-muted block text-[11px]">Giao trực tiếp · đã gia hạn 1 lần</span>');
    expect(html).toContain("Giao trực tiếp · đã gia hạn 4 lần</span>");
  });

  it("a pending request adds nothing to the row — the prototype list draws no marker", () => {
    expect(listHtml([task({ pending_extension: true })])).toBe(listHtml([task({ pending_extension: false })]));
  });
});

describe("drawer — `Hạn xử lý` cell and the pending strip (prototype `TaskDetailDrawer.tsx:351-375`)", () => {
  it("n = 0: `Còn trong hạn` alone; no \"?\" marker", () => {
    const fact = dueFact(drawerHtml({ extension_count: 0 }));
    expect(fact).toContain("Còn trong hạn</p>");
    expect(fact).not.toContain("đã gia hạn");
    expect(fact).not.toContain("data-pending-marker");
  });

  it("n > 0: `Còn trong hạn · đã gia hạn n lần`; overdue: `Hạn {d} · đã gia hạn n lần`", () => {
    expect(dueFact(drawerHtml({ extension_count: 2 }))).toContain("Còn trong hạn · đã gia hạn 2 lần</p>");
    const late = dueFact(
      drawerHtml({
        extension_count: 1,
        due_at: "2026-09-01T23:59:59+07:00",
        original_due_at: "2026-08-20T23:59:59+07:00",
      }),
    );
    expect(late).toContain("Hạn 1/9/2026 · đã gia hạn 1 lần</p>");
  });

  it("`pending_extension` true: the strip names the recorded assigner; false: no strip", () => {
    const pending = drawerHtml({ pending_extension: true });
    expect(pending).toContain(`Chờ duyệt lùi hạn — đã gửi tới ${ASSIGNER}`);
    // The strip sits between the status strip and the fact grid.
    expect(pending.indexOf("Chờ duyệt lùi hạn")).toBeLessThan(pending.indexOf(">Hạn xử lý</p>"));
    expect(drawerHtml({ pending_extension: false })).not.toContain("Chờ duyệt lùi hạn");
  });
});

describe("`requires_directive` decides the directive block — not the type code", () => {
  it("shipped catalogue: `theo-van-ban` has the block, `co-ban` has not (behaviour unchanged)", () => {
    expect(showsDirectiveBlock(task({ type: "theo-van-ban" }), SHIPPED.loai)).toBe(true);
    expect(showsDirectiveBlock(task({ type: "co-ban" }), SHIPPED.loai)).toBe(false);
    expect(drawerHtml({ type: "theo-van-ban" })).toContain(`>${TIEU_DE_KHOI_VAN_BAN}</h3>`);
    expect(drawerHtml({ type: "co-ban" })).toContain(`>${TASK_INFO_TITLE}</h3>`);
  });

  it("a flagged type with ANOTHER code gets the block; an unflagged `theo-van-ban` does not", () => {
    const other = catalogue([typeRow("chi-dao", true), typeRow("theo-van-ban", false)]);
    expect(showsDirectiveBlock(task({ type: "chi-dao" }), other.loai)).toBe(true);
    expect(showsDirectiveBlock(task({ type: "theo-van-ban" }), other.loai)).toBe(false);
    expect(drawerHtml({ type: "chi-dao" }, other)).toContain(`>${TIEU_DE_KHOI_VAN_BAN}</h3>`);
    expect(drawerHtml({ type: "theo-van-ban" }, other)).toContain(`>${TASK_INFO_TITLE}</h3>`);
  });

  it("an approval tick or a result still opens the block, whatever the flag (prototype `showRegister`)", () => {
    expect(showsDirectiveBlock(task({ leader_approved: true }), [])).toBe(true);
    expect(showsDirectiveBlock(task({ result_summary: "Đã xong" }), [])).toBe(true);
  });

  it("create form: the flagged default type draws the three lists and `Nội dung nhiệm vụ…`, whatever its code", () => {
    const form = (types: DanhMucNhiemVu) =>
      renderToStaticMarkup(
        <FormGiaoViec
          danhMuc={types}
          danhBa={DIRECTORY}
          danhBaLanhDao={DIRECTORY}
          coDanhSachVanBan
          taskScreen
          dangGui={false}
          loi={null}
          huy={() => {}}
          giaoViec={() => {}}
        />,
      );
    const flagged = form(catalogue([typeRow("chi-dao", true, true), typeRow("co-ban", false)]));
    expect(flagged).toContain("Nội dung nhiệm vụ / Trích yếu văn bản");
    expect(flagged).toContain('id="giao-them-van-ban-cap-tren-giao"');
    const unflagged = form(catalogue([typeRow("theo-van-ban", false, true), typeRow("co-ban", false)]));
    expect(unflagged).not.toContain("Trích yếu văn bản");
    expect(unflagged).not.toContain("Thêm văn bản");
  });
});
