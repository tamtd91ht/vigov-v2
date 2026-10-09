import { readFileSync } from "node:fs";

import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { comms_danhMucRa, comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  BANNER_COVER_NOTICE,
  CANH_BAO_HTML_THO,
  CATEGORY_ROOT_GROUP,
  CHUA_XEP_DANH_MUC,
  ERR_BANNER_COVER_REQUIRED,
  ERR_EVENT_END_BEFORE_START,
  ERR_EVENT_END_WITHOUT_START,
  ERR_VIDEO_URL_INVALID,
  EVENT_TIME_HINT,
  ACTIONS_COLUMN_LABEL,
  CONTENT_DELETE_TITLE,
  contentDeleteAriaLabel,
  FORM_TRONG,
  giaTriTuHang,
  LINK_TO_HINT,
  MO_TA_FORM_THEM,
  MO_TA_THE_DANH_BA,
  MOI_DANH_MUC,
  NHAN_NUT_DANH_MUC,
  NHAN_NUT_SUA,
  NHAN_NUT_THEM,
  NHAN_O_DANG,
  PENDING_REVIEW_HINT,
  PHAN_CHUA_DUNG,
  THAN_BAI_RONG,
  THUMBNAIL_PART,
  TITLE_REQUIRED,
  TOTAL_ITEMS_PART,
  TOTAL_PAGES_PART,
  portalCategoryLabel,
  PUBLISHED_STAFF_UNREAD,
  SO_RONG,
  STATUS_FILTER_OPTIONS,
  UPLOADING_FILES_LABEL,
} from "./nhan-noi-dung";
import {
  COVER_BANNER_HINT,
  COVER_HINT,
  COVER_PICK_BUTTON,
  COVER_PREVIEW_MISSING,
  COVER_REMOVE_BUTTON,
  COVER_REPLACE_BUTTON,
  COVER_WILL_DETACH,
  type CoverUploadState,
} from "./cover-image";
import { CategoryAdmin } from "./category-admin";
import { EDITOR_LOADING } from "./rich-text-editor";
import {
  BangNoiDung,
  CONTENT_TABPANEL_ID,
  ContentPager,
  contentTabId,
  FormDanhMuc,
  FormNoiDung,
  HangLocNoiDung,
  HeaderActions,
  ThanhTabLoai,
  TheDanhBaChinhQuyen,
} from "./so-noi-dung";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không — the prototype's `ContentWorkspace` / `ContentItemForm`
 * (ADR 0068 lần 5) and the owner's decisions of 09/10/2026 (D1 `Đăng`/`Gỡ`, D3 extras removed).
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **HTML KHÔNG ĐƯỢC DỰNG**: the body is never handed to the
 * page as a string. On the server render the editor has not mounted (it needs a DOM), so the stored body
 * must not appear AT ALL — escaped or not; the editor's own behaviour is `rich-text.test.ts` (jsdom).
 *
 * Nhóm thứ hai canh một sự VẮNG MẶT, thứ khó canh nhất vì không có gì để tìm trên trang: without
 * `content.update` there is no `✎`/`Đăng`/`Gỡ`/`🗑` in the table, and KHÔNG có ô chọn trạng thái trong biểu mẫu.
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ `renderToStaticMarkup` thoát `&`, `<`, `>`, `"` và `'`, nên một phép `not.toContain` với chuỗi
 * thô sẽ XANH kể cả khi chữ ấy đang nằm chình ình trên trang — tức là canh đúng con số không.
 * `&` PHẢI THAY TRƯỚC, nếu không `&lt;` vừa sinh ra sẽ bị thay tiếp thành `&amp;lt;`.
 */
function nhuTrongHTML(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;");
}

function danhMuc(sua: Partial<comms_danhMucRa> = {}): comms_danhMucRa {
  return {
    id: "01JDM1",
    name: "Chuyển đổi số",
    slug: "chuyen-doi-so",
    parent_id: "",
    order: 0,
    created_at: "2026-09-01T02:00:00Z",
    hidden: false,
    ...sua,
  };
}

function hang(sua: Partial<comms_noiDungRa> = {}): comms_noiDungRa {
  return {
    id: "01JND1",
    type: "tin-tuc",
    category_id: "01JDM1",
    title: "Xã tổ chức hội nghị tổng kết công tác chuyển đổi số",
    summary: "Hội nghị diễn ra sáng 14/9 tại hội trường UBND xã.",
    image_url: "https://cdn.example.vn/anh/hoi-nghi.jpg",
    has_image: true,
    published_on: "2026-09-14",
    view_count: 0,
    status: "dang-hien",
    source: "thu-cong",
    source_url: "",
    source_ref: "",
    hand_edited: false,
    author_code: "CB-2026-7K3M9Q",
    created_at: "2026-09-14T02:00:00Z",
    updated_at: "2026-09-14T09:35:00Z",
    ...sua,
  };
}

function veBang(
  ds: readonly comms_noiDungRa[],
  dm: readonly comms_danhMucRa[] = [danhMuc()],
  canEdit = true,
) {
  return renderToStaticMarkup(
    <BangNoiDung ds={ds} danhMuc={dm} sua={() => {}} remove={() => {}} togglePublish={() => {}} canEdit={canEdit} />,
  );
}

/** The table's body only — the header carries a "?" popover trigger of its own. */
const bodyOf = (html: string) => html.slice(html.indexOf("<tbody>"));

function veForm(
  gt = FORM_TRONG,
  h?: comms_noiDungRa,
  coverState?: CoverUploadState,
  dm: readonly comms_danhMucRa[] = [danhMuc()],
  titleTouched = false,
) {
  return renderToStaticMarkup(
    <FormNoiDung
      tieuDeForm="Thêm nội dung cho Mini App"
      moTa={MO_TA_FORM_THEM}
      giaTriDau={gt}
      hang={h}
      danhMuc={dm}
      dangGui={false}
      loi={null}
      huy={() => {}}
      luu={() => {}}
      initialCoverState={coverState}
      initialTitleTouched={titleTouched}
    />,
  );
}

function filterRow(over: Partial<Parameters<typeof HangLocNoiDung>[0]> = {}) {
  return renderToStaticMarkup(
    <HangLocNoiDung
      danhMucID=""
      datDanhMucID={() => {}}
      tim=""
      datTim={() => {}}
      danhMuc={[]}
      status=""
      setStatus={() => {}}
      {...over}
    />,
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HTML KHÔNG ĐƯỢC DỰNG — nhóm quan trọng nhất của màn
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("thân bài ra tới trang dưới dạng DỮ LIỆU, không phải mã", () => {
  const DOC = '<script>alert("xin chao")</script><img src=x onerror=alert(1)>';

  it("a stored body never reaches the server-rendered page — neither as markup nor as text", () => {
    const html = veForm({ ...FORM_TRONG, body: DOC });
    expect(html).not.toContain("<script");
    expect(html).not.toContain("<img");
    expect(html).not.toContain("alert");
    expect(html).toContain(EDITOR_LOADING);
  });

  it("no note under the body box (prototype parity 09/10/2026): neither the cleaner's list nor the empty-body line", () => {
    const html = veForm();
    expect(html).not.toContain(nhuTrongHTML(CANH_BAO_HTML_THO));
    expect(html).not.toContain('id="than-bai-noi-dung-hint"');
    expect(html).not.toContain(THAN_BAI_RONG);
    expect(html).not.toContain("MÃ NGUỒN HTML");
    expect(html).not.toContain("<textarea id=\"than-bai-noi-dung\"");
  });

  it("KHÔNG có ô xem trước nào — không `dangerouslySetInnerHTML`, không khối `preview`", () => {
    const html = veForm({ ...FORM_TRONG, body: "<b>đậm</b>" });
    expect(html).not.toContain("<b>đậm</b>");
    expect(html).not.toContain("&lt;b&gt;");
  });

  it("tiêu đề mang dấu ngoặc nhọn cũng ra dạng đã thoát ở BẢNG", () => {
    const html = veBang([hang({ title: "<b>Tin nóng</b>", summary: "<i>ngay</i>" })]);
    expect(html).toContain("&lt;b&gt;Tin nóng&lt;/b&gt;");
    expect(html).not.toContain("<b>Tin nóng</b>");
  });

  it("D3: the edit form has no read-only block — the original link is not on the page at all", () => {
    const row = hang({ source_url: "javascript:alert(1)", body: "" });
    const html = veForm(giaTriTuHang(row), row);
    expect(html).not.toContain("javascript:alert(1)");
    expect(html).not.toContain("Cập nhật lúc");
    expect(html).not.toContain("Người soạn");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢNG §6 — the prototype's columns
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("bảng nội dung §6", () => {
  it("the data of a row, without a `Loại` column (each tab IS one type)", () => {
    const html = veBang([hang()]);
    expect(html).toContain("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số");
    expect(html).toContain("Hội nghị diễn ra sáng 14/9");
    expect(html).toContain("Chuyển đổi số");
    expect(html).toContain(">Có ảnh</span>");
    expect(html).toContain("lucide-paperclip");
    expect(html).toContain("14/9/2026");
    expect(html).toContain("Đang hiện");
    expect(html).not.toContain(">Loại</th>");
  });

  it("header: the thumbnail '?' with no words, then the prototype's six titles, then an unnamed actions column", () => {
    const html = veBang([hang({ view_count: 42 })]);
    const headers = [...html.matchAll(/<th scope="col"[^>]*>([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(headers).toEqual(["Tiêu đề", "Chuyên mục", "Tệp đính kèm", "Ngày đăng", "Lượt xem", "Trạng thái"]);
    // The thumbnail header: a "?" and its name for screen readers only, no visible "Ảnh".
    const thumb = /<th scope="col" class="w-16"[^>]*>[^]*?<\/th>/.exec(html)?.[0] ?? "";
    expect(thumb).toContain(`aria-label="${pendingMarkerLabel(THUMBNAIL_PART)}"`);
    expect(thumb).toContain(`<span class="an-thi-giac">${THUMBNAIL_PART}</span>`);
    expect(thumb).not.toContain(">Ảnh<");
    // The actions column is named for screen readers only.
    expect(html).toContain(`<th scope="col" class="w-32"><span class="an-thi-giac">${ACTIONS_COLUMN_LABEL}</span></th>`);
    // Widths and alignments of the prototype.
    expect(html).toContain('<th scope="col" class="w-48">Chuyên mục</th>');
    expect(html).toContain('<th scope="col" class="w-32">Ngày đăng</th>');
    expect(html).toContain('<th scope="col" class="w-24 text-right">Lượt xem</th>');
    expect(html).toContain('<th scope="col" class="w-28 text-center">Trạng thái</th>');
    expect(html).toContain("lucide-eye");
    expect(html).toContain(">42</span>");
  });

  it("lượt xem in theo vi-VN: 1234 → `1.234`, và 0 hiện `0` (không phải dấu gạch)", () => {
    expect(veBang([hang({ view_count: 1234 })])).toContain(">1.234</span>");
    expect(veBang([hang({ view_count: 0 })])).toMatch(/lucide-eye[^]*?<\/svg>0<\/span>/);
  });

  it("the title opens the edit dialog (a button), the summary is one line under it", () => {
    const html = veBang([hang()]);
    expect(html).toMatch(
      /<button type="button" title="Xã tổ chức hội nghị[^"]*" aria-haspopup="dialog" class="[^"]*block w-full[^"]*truncate[^"]*font-semibold text-navy hover:underline"/,
    );
    expect(html).toContain('<p class="m-0 mt-0.5 truncate text-[11.5px] text-ink-muted">Hội nghị diễn ra sáng 14/9');
  });

  it("the action column: pencil · Đăng/Gỡ · trash, each naming its row", () => {
    const html = veBang([hang()]);
    expect(html).toContain(nhuTrongHTML("Sửa: Xã tổ chức hội nghị tổng kết công tác chuyển đổi số"));
    expect(html).toContain(NHAN_NUT_SUA);
    expect(html).toContain(`title="${CONTENT_DELETE_TITLE}"`);
    expect(html).toContain(
      `aria-label="${nhuTrongHTML(contentDeleteAriaLabel("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số"))}"`,
    );
    expect(html).toContain('aria-label="Gỡ: Xã tổ chức hội nghị tổng kết công tác chuyển đổi số"');
    const pencil = html.indexOf("lucide-pencil");
    const toggle = html.indexOf(">Gỡ</button>");
    const trash = html.indexOf("lucide-trash");
    expect(pencil).toBeGreaterThan(0);
    expect(toggle).toBeGreaterThan(pencil);
    expect(trash).toBeGreaterThan(toggle);
  });

  it("`Gỡ` on a showing row, `Đăng` on a hidden or a pending one (D1)", () => {
    expect(veBang([hang({ status: "dang-hien" })])).toContain(">Gỡ</button>");
    expect(veBang([hang({ status: "an" })])).toContain(">Đăng</button>");
    expect(veBang([hang({ status: "cho-duyet" })])).toContain(">Đăng</button>");
    expect(veBang([hang({ status: "an" })])).not.toContain(">Gỡ</button>");
  });

  it("the toggle in flight is held, the others are not", () => {
    const html = renderToStaticMarkup(
      <BangNoiDung
        ds={[hang({ id: "A", title: "A" }), hang({ id: "B", title: "B" })]}
        danhMuc={[]}
        sua={() => {}}
        remove={() => {}}
        togglePublish={() => {}}
        publishingId="A"
        canEdit
      />,
    );
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*aria-busy="true"[^>]*aria-label="Gỡ: A"/);
    expect(html).not.toMatch(/<button[^>]*disabled=""[^>]*aria-label="Gỡ: B"/);
  });

  it("the delete button only OPENS a dialog; the title and the pencil open the edit dialog", () => {
    // Counted in the BODY: the header's thumbnail "?" opens a popover of its own.
    expect(bodyOf(veBang([hang()])).match(/aria-haspopup="dialog"/g)?.length).toBe(3);
  });

  it("an item filed nowhere shows the prototype's dash; a filed one `Cha › Con`", () => {
    const ds = [danhMuc({ id: "P", name: "Tin hoạt động" }), danhMuc({ id: "C", name: "Chuyển đổi số", parent_id: "P" })];
    const body = bodyOf(veBang([hang({ category_id: "" })], ds));
    expect(body).toMatch(/<span class="block max-w-\[14rem\] truncate text-ink-muted" title="—">—<\/span>/);
    expect(body).not.toContain(CHUA_XEP_DANH_MUC);
    expect(veBang([hang({ category_id: "C" })], ds)).toContain(">Tin hoạt động › Chuyển đổi số</span>");
  });

  it("`Tệp đính kèm` per type: banner image, broadcast file, video link — missing ones in amber with a warning", () => {
    const banner = veBang([hang({ type: "banner", has_image: false })]);
    expect(banner).toContain("text-tangerine");
    expect(banner).toContain("lucide-triangle-alert");
    expect(banner).toContain(">Thiếu ảnh</span>");
    expect(veBang([hang({ type: "banner", has_image: true })])).toContain(">Đã có ảnh</span>");
    expect(veBang([hang({ type: "truyen-thanh", audio_file_id: "01JA" })])).toMatch(/text-leaf[^]*Đã có tệp/);
    expect(veBang([hang({ type: "truyen-thanh" })])).toContain(">Thiếu tệp</span>");
    expect(veBang([hang({ type: "video", video_url: "https://youtu.be/x" })])).toContain(">Đã có tệp</span>");
    expect(bodyOf(veBang([hang({ has_image: false })]))).toContain('<span class="text-ink-muted">—</span>');
  });

  it("D3: no sub-line `Đồng bộ từ Cổng`, `Đã sửa tay…`, `Đăng lần đầu lúc…` — the portal category line stays", () => {
    const synced = {
      ...hang({ source: "dong-bo-cong", hand_edited: true, published_at: "2026-10-01T02:05:00Z" }),
      portal_category_name: "Tin địa phương",
    };
    const html = veBang([synced]);
    expect(html).not.toContain("Đồng bộ từ Cổng");
    expect(html).not.toContain("Đã sửa tay");
    expect(html).not.toContain("Đăng lần đầu");
    expect(html).toContain(portalCategoryLabel("Tin địa phương"));
    expect(veBang([hang({ id: "B" })])).not.toContain("Chuyên mục Cổng:");
  });

  it("chip `Chờ duyệt` và `Ẩn` hiện đúng chữ, không lẫn với `Đang hiện` — three statuses kept (D1)", () => {
    expect(veBang([hang({ status: "cho-duyet" })])).toContain("Chờ duyệt");
    expect(veBang([hang({ status: "an" })])).toContain("Ẩn");
    expect(veBang([hang({ status: "an" })])).not.toContain("Đang hiện");
  });

  it("an empty page says the prototype's sentence in ONE row spanning the table", () => {
    const html = veBang([]);
    expect(html).toContain(`<td colSpan="8" class="py-12 text-center text-ink-muted">${SO_RONG}</td>`);
    expect(veBang([], [], false)).toContain(`<td colSpan="7" class="py-12 text-center text-ink-muted">${SO_RONG}</td>`);
  });

  it("a long title stays one line, cut by CSS, the whole title on hover; the frame scrolls sideways only", () => {
    const long = "Tiêu đề rất dài ".repeat(20).trim();
    const html = veBang([hang({ title: long, summary: `${"Hội nghị ".repeat(60)}KẾT-THÚC` })]);
    expect(html).toContain(`title="${long}"`);
    expect(html).toContain("max-w-[34rem]");
    // The PAGE scrolls (prototype `ContentWorkspace.tsx:288-289`): no box height, no sticky header.
    const frame = /<div class="(bang-cuon[^"]*)">/.exec(html)![1]!;
    expect(frame).toContain("overflow-x-auto");
    expect(frame).not.toContain("max-h-");
    expect(frame.split(" ")).not.toContain("overflow-auto");
    expect(frame).not.toContain("sticky");
    // The summary is never cut by characters: the whole text is in the page.
    expect(html).toContain("KẾT-THÚC");
    expect(html).not.toContain("…");
  });

  it("the prototype's thumbnail column is '—' cells: the list route serves no image link", () => {
    const html = veBang([hang()]);
    expect(bodyOf(html)).toContain("data-pending");
    expect(html).not.toContain("<img");
  });
});

describe("`content.update` gates the table's write controls", () => {
  it("DENIED: no `✎`, no `Đăng`/`Gỡ`, no `🗑`, no action column, the title is plain text", () => {
    const html = veBang([hang()], [danhMuc()], false);
    expect(html).not.toContain(NHAN_NUT_SUA);
    expect(html).not.toContain(CONTENT_DELETE_TITLE);
    expect(html).not.toContain("Xoá");
    expect(html).not.toContain(">Gỡ</button>");
    expect(html).not.toContain(">Đăng</button>");
    expect(html).not.toContain(ACTIONS_COLUMN_LABEL);
    expect(bodyOf(html)).not.toContain("aria-haspopup");
    expect(bodyOf(html)).not.toContain("<button");
    // Seven header cells: the thumbnail "?" and the six data columns — no actions.
    expect(html.match(/<th scope="col"/g)?.length).toBe(7);
    expect(html).toContain("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số");
  });

  it("ALLOWED: title, pencil and trash open dialogs on every row; one toggle per row", () => {
    const html = veBang([hang({ id: "A" }), hang({ id: "B", status: "an" })], [danhMuc()], true);
    expect(bodyOf(html).match(/aria-haspopup="dialog"/g)?.length).toBe(6);
    expect(html.match(new RegExp(`title="${CONTENT_DELETE_TITLE}"`, "g"))?.length).toBe(2);
    expect(html.match(/>Gỡ<\/button>|>Đăng<\/button>/g)?.length).toBe(2);
  });

  it("`Thêm nội dung`: absent without the key, present with it — the prototype's Plus icon + words, pushed right", () => {
    expect(renderToStaticMarkup(<HeaderActions canEdit={false} addOpen={false} openAdd={() => {}} />)).toBe("");
    const html = renderToStaticMarkup(<HeaderActions canEdit addOpen={false} openAdd={() => {}} />);
    expect(html).toContain("lucide-plus");
    expect(html).toContain("Thêm nội dung");
    expect(html).toContain("ml-auto");
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_THEM));
    expect(html).toContain('aria-haspopup="dialog"');
  });

  it("the filter row draws what it is given at its end, and nothing when given nothing", () => {
    const row = (extra?: ReactNode) => filterRow({ extra });
    expect(row()).not.toContain(NHAN_NUT_DANH_MUC);
    expect(row(<button type="button">{NHAN_NUT_DANH_MUC}</button>)).toContain(NHAN_NUT_DANH_MUC);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PAGER — inside the frame, 25 rows, the total a "?"
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("pager", () => {
  const pager = (pageIndex: number, rows: number, hasNext: boolean) =>
    renderToStaticMarkup(
      <ContentPager pageIndex={pageIndex} rows={rows} hasNext={hasNext} previous={() => {}} next={() => {}} />,
    );

  it("hidden on a single page — the first page with nothing after it", () => {
    expect(pager(0, 7, false)).toBe("");
  });

  it("`a–b trên ? mục` · `Trước` · `Trang x/?` · `Sau`, each unknown number a '?' with its reason", () => {
    const html = pager(1, 25, true);
    expect(html).toContain("26–50 trên");
    expect(html).toContain(`aria-label="${pendingMarkerLabel(TOTAL_ITEMS_PART)}"`);
    expect(html).toContain("Trang 2/");
    expect(html).toContain(`aria-label="${pendingMarkerLabel(TOTAL_PAGES_PART)}"`);
    expect(html).toContain("Trước");
    expect(html).toContain("Sau");
    expect(html).toContain("lucide-chevron-left");
    expect(html).toContain("lucide-chevron-right");
    expect(html).toContain("border-t border-line px-4 py-2.5");
  });

  it("`Trước` off on the first page, `Sau` off on the last", () => {
    expect(pager(0, 25, true)).toMatch(/<button[^>]*disabled=""[^>]*>.*Trước<\/button>/);
    expect(pager(2, 3, false)).toMatch(/<button[^>]*disabled=""[^>]*>Sau/);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BIỂU MẪU §7 — the prototype's `ContentItemForm`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("biểu mẫu nội dung §7", () => {
  it("the boxes of a news item, in the prototype's order, and the sentence right under the title", () => {
    const html = veForm();
    const order = ["Loại nội dung", "Danh mục", "Tiêu đề", "Tóm tắt", "Nội dung", "Ảnh đại diện", NHAN_O_DANG].map((w) =>
      html.indexOf(`>${w}`),
    );
    for (const i of order) expect(i).toBeGreaterThan(0);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    expect(html).toMatch(/<h3 id="tieu-de-form-noi-dung"[^>]*>Thêm nội dung cho Mini App<\/h3><p[^>]*>/);
    expect(html).toContain(nhuTrongHTML(MO_TA_FORM_THEM));
    // No section headings any more (owner 09/10/2026: the prototype draws none).
    expect(html).not.toContain("Thông tin chung");
    expect(html).not.toContain("Ảnh, âm thanh và đính kèm");
    expect(html).not.toContain("<h4");
  });

  it("one scrolling column, Huỷ then Lưu at its end", () => {
    const html = veForm();
    expect(html).toContain("max-h-[70vh]");
    expect(html.indexOf(">Huỷ</button>")).toBeLessThan(html.indexOf(">Lưu</button>"));
  });

  it("KHÔNG có ô chọn trạng thái — máy chủ suy trạng thái từ ô tích", () => {
    const html = veForm();
    expect(html).not.toContain('id="trang-thai-noi-dung"');
    expect(html).not.toContain("Chờ duyệt");
  });

  it("the type select carries the prototype's longer labels", () => {
    const html = veForm();
    for (const nhan of ["Tin tức", "Sự kiện", "Thông báo", "Bản tin truyền thanh", "Video", "Banner trang chủ"]) {
      expect(html).toContain(`>${nhan}</option>`);
    }
  });

  it("`Loại nội dung` is locked when editing, free when adding", () => {
    expect(veForm()).not.toMatch(/<select id="loai-noi-dung"[^>]*disabled=""/);
    const row = hang({ body: "" });
    expect(veForm(giaTriTuHang(row), row)).toMatch(/<select id="loai-noi-dung"[^>]*disabled=""/);
  });

  it("the category select offers non-hidden categories only — but keeps the item's own hidden one", () => {
    const ds = [danhMuc({ id: "V", name: "Tin xã" }), danhMuc({ id: "H", name: "Mục cũ", hidden: true })];
    const create = veForm(FORM_TRONG, undefined, undefined, ds);
    expect(create).toContain('<option value="V">Tin xã</option>');
    expect(create).not.toContain('value="H"');
    const row = hang({ body: "", category_id: "H" });
    const edit = veForm(giaTriTuHang(row), row, undefined, ds);
    // Selected, not misreported as `— Chưa xếp danh mục —`.
    expect(edit).toContain('<option value="H" selected="">Mục cũ</option>');
    expect(edit).toContain('<option value="V">Tin xã</option>');
  });

  it("the category select reads `Cha › Con`, after `— Chưa xếp danh mục —`", () => {
    const ds = [danhMuc({ id: "P", name: "Tin hoạt động" }), danhMuc({ id: "C", name: "Chuyển đổi số", parent_id: "P" })];
    const html = veForm(FORM_TRONG, undefined, undefined, ds);
    expect(html).toMatch(new RegExp(`<select id="danh-muc-noi-dung"><option value=""[^>]*>${CHUA_XEP_DANH_MUC}</option>`));
    expect(html).toContain('<option value="C">Tin hoạt động › Chuyển đổi số</option>');
  });

  it("`Đăng lên Mini App` is a plain tick box and words — no bordered box", () => {
    const html = veForm();
    expect(html).toMatch(/<label for="dang-len-mini-app" class="flex cursor-pointer items-center gap-2.5 text-\[12.5px\] text-navy">/);
    expect(html).not.toContain("border-brand-500 bg-brand-50");
  });

  it("ô tích BẬT khi mở một bài đang hiện, TẮT khi mở một bài chờ duyệt", () => {
    expect(veForm(giaTriTuHang(hang({ status: "dang-hien", body: "" })))).toContain("checked");
    expect(veForm(giaTriTuHang(hang({ status: "cho-duyet", body: "" })))).not.toContain("checked");
  });

  it("saving: the word stays `Lưu` beside a spinner — no `Đang lưu…` (prototype `ContentItemForm.tsx:370-375`)", () => {
    const html = renderToStaticMarkup(
      <FormNoiDung
        tieuDeForm="T"
        moTa=""
        giaTriDau={{ ...FORM_TRONG, title: "Có tiêu đề" }}
        danhMuc={[]}
        dangGui
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );
    const submit = html.slice(html.indexOf('<button type="submit"'));
    expect(submit).toMatch(/^<button type="submit"[^>]*disabled=""[^>]*aria-busy="true"/);
    expect(submit).toContain("animate-spin");
    expect(submit).toMatch(/<\/svg>Lưu<\/button>/);
    expect(html).not.toContain("Đang lưu…");
    expect(html).not.toContain(UPLOADING_FILES_LABEL);
  });

  it("uploading a held file after the save: `Đang tải tệp…`", () => {
    const html = renderToStaticMarkup(
      <FormNoiDung
        tieuDeForm="T"
        moTa=""
        giaTriDau={{ ...FORM_TRONG, title: "Có tiêu đề", type: "truyen-thanh" }}
        danhMuc={[]}
        dangGui
        uploading
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );
    expect(html).toMatch(new RegExp(`<button type="submit"[^>]*disabled=""[^>]*>[^]*?${UPLOADING_FILES_LABEL}</button>`));
  });

  it("Lưu is PRESSABLE with an empty title; pressed, the title says `Vui lòng nhập tiêu đề` inline", () => {
    expect(veForm()).toMatch(/<button type="submit"[^>]*>Lưu<\/button>/);
    expect(veForm()).not.toMatch(/<button type="submit"[^>]*disabled=""/);
    expect(veForm()).not.toContain(TITLE_REQUIRED);
    const touched = veForm(FORM_TRONG, undefined, undefined, undefined, true);
    expect(touched).toContain(`<p role="alert" class="m-0 text-[12px] font-medium text-danger">${TITLE_REQUIRED}</p>`);
    expect(veForm({ ...FORM_TRONG, title: "Có tiêu đề" }, undefined, undefined, undefined, true)).not.toContain(
      TITLE_REQUIRED,
    );
  });

  it("D3: no read-only block on the edit form", () => {
    expect(veForm()).not.toContain("Cập nhật lúc");
    expect(veForm(FORM_TRONG, hang())).not.toContain("Cập nhật lúc");
    expect(veForm(FORM_TRONG, hang())).not.toContain("<dl");
  });
});

describe("per-type boxes of §7 :131 (ADR 0047 §6)", () => {
  const EVENT_BOXES = ['id="bat-dau-su-kien"', 'id="ket-thuc-su-kien"', 'id="dia-diem-su-kien"'];
  const VIDEO_BOX = 'id="lien-ket-video"';

  it("`Sự kiện` shows Bắt đầu · Kết thúc · Địa điểm and no video box", () => {
    const html = veForm({ ...FORM_TRONG, type: "su-kien" });
    for (const box of EVENT_BOXES) expect(html).toContain(box);
    expect(html).toContain(">Bắt đầu<");
    expect(html).toContain(">Kết thúc<");
    expect(html).toContain(">Địa điểm<");
    expect(html.match(/type="datetime-local"/g)?.length).toBe(2);
    // No GMT+7 / clock hint under the two boxes (prototype parity 09/10/2026).
    expect(html).not.toContain(nhuTrongHTML(EVENT_TIME_HINT));
    expect(html).not.toContain("GMT+7");
    expect(html).not.toContain(VIDEO_BOX);
  });

  it("`Video` shows Liên kết video and no event box", () => {
    const html = veForm({ ...FORM_TRONG, type: "video" });
    expect(html).toContain(VIDEO_BOX);
    expect(html).toContain(">Liên kết video<");
    // No hint under the box (prototype parity: `ContentItemForm.tsx` has none).
    expect(html).not.toContain("Chỉ nhận địa chỉ bắt đầu bằng http://");
    expect(html).not.toContain("mở video ở ngoài ứng dụng");
    for (const box of EVENT_BOXES) expect(html).not.toContain(box);
  });

  it("the other four types show neither", () => {
    for (const type of ["tin-tuc", "thong-bao", "truyen-thanh", "banner"]) {
      const html = veForm({ ...FORM_TRONG, type });
      for (const box of [...EVENT_BOXES, VIDEO_BOX]) expect(html, type).not.toContain(box);
    }
  });

  it("`Banner`: `Thứ tự chạy` beside the type, `Bấm vào thì mở gì` under the title, no summary, no body, no category", () => {
    const html = veForm({ ...FORM_TRONG, type: "banner" });
    expect(html).toContain('id="lien-ket-banner"');
    expect(html).toMatch(/<input id="thu-tu-banner"[^>]*placeholder="Số nhỏ hiện trước"/);
    expect(html).toContain(">Thứ tự chạy<");
    expect(html).toContain(">Bấm vào thì mở gì<");
    expect(html).toContain('placeholder="Ví dụ: /tin-tuc — để trống thì ảnh chỉ để xem"');
    expect(html.indexOf(">Thứ tự chạy<")).toBeLessThan(html.indexOf(">Tiêu đề"));
    expect(html.indexOf(">Tiêu đề")).toBeLessThan(html.indexOf(">Bấm vào thì mở gì<"));
    expect(html).not.toContain('id="tom-tat-noi-dung"');
    expect(html).not.toContain(EDITOR_LOADING);
    expect(html).not.toContain('id="danh-muc-noi-dung"');
    // No notice box and no link hint (prototype parity 09/10/2026); the required-image rule stays inline.
    expect(html).not.toContain(nhuTrongHTML(BANNER_COVER_NOTICE));
    expect(html).not.toContain(nhuTrongHTML(LINK_TO_HINT));
    for (const type of ["tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video"]) {
      const other = veForm({ ...FORM_TRONG, type, link_to: "/x", display_order: "1" });
      expect(other, type).not.toContain('id="lien-ket-banner"');
      expect(other, type).not.toContain('id="thu-tu-banner"');
      expect(other, type).not.toContain(nhuTrongHTML(BANNER_COVER_NOTICE));
      expect(other, type).toContain('id="danh-muc-noi-dung"');
    }
  });

  it("a banner with no cover says so and holds Lưu; with a cover Lưu is free", () => {
    const html = veForm({ ...FORM_TRONG, type: "banner", title: "Banner" });
    expect(html).toContain(ERR_BANNER_COVER_REQUIRED);
    expect(html).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    const ok = veForm({ ...FORM_TRONG, type: "banner", title: "Banner", cover_image_file_id: "01JC" });
    expect(ok).not.toContain(ERR_BANNER_COVER_REQUIRED);
    expect(ok).not.toMatch(/<button type="submit"[^>]*disabled=""/);
  });

  it("the cover picker is untouched by the type — still there for `Sự kiện` and `Video`", () => {
    expect(veForm({ ...FORM_TRONG, type: "su-kien" })).toContain('id="anh-noi-dung"');
    expect(veForm({ ...FORM_TRONG, type: "video" })).toContain('id="anh-noi-dung"');
  });

  it("an edit form opens with the stored instants in Vietnam time", () => {
    const html = veForm(
      giaTriTuHang(hang({ type: "su-kien", body: "", event_starts_at: "2026-10-05T01:00:00Z" })),
    );
    expect(html).toContain('value="2026-10-05T08:00"');
  });

  it("a validation problem is shown next to the boxes, in Vietnamese, and Lưu is off", () => {
    const html = veForm({
      ...FORM_TRONG,
      type: "su-kien",
      title: "Có tiêu đề",
      event_starts_local: "2026-10-05T08:00",
      event_ends_local: "2026-10-05T07:00",
    });
    expect(html).toContain(ERR_EVENT_END_BEFORE_START);
    expect(html).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    const ok = veForm({
      ...FORM_TRONG,
      type: "su-kien",
      title: "Có tiêu đề",
      event_starts_local: "2026-10-05T08:00",
      event_ends_local: "2026-10-05T09:00",
    });
    expect(ok).not.toMatch(/<button type="submit"[^>]*disabled=""/);
  });

  it("an end without a start is named", () => {
    const html = veForm({ ...FORM_TRONG, type: "su-kien", title: "T", event_ends_local: "2026-10-05T07:00" });
    expect(html).toContain(ERR_EVENT_END_WITHOUT_START);
  });

  it("a non-http(s) video link is named", () => {
    const html = veForm({ ...FORM_TRONG, type: "video", title: "T", video_url: "ftp://a.vn/x" });
    expect(html).toContain(ERR_VIDEO_URL_INVALID);
  });

  it("the server's 400 sentence is shown as it came", () => {
    const loi = "noi_dung_mini_app: `video_url` chỉ dùng cho loại video";
    const html = renderToStaticMarkup(
      <FormNoiDung
        tieuDeForm="x"
        moTa="x"
        giaTriDau={{ ...FORM_TRONG, title: "T" }}
        danhMuc={[]}
        dangGui={false}
        loi={loi}
        huy={() => {}}
        luu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(loi));
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TAB, BỘ LỌC, THẺ DANH BẠ, DANH MỤC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("sáu tab loại §5 — exactly the prototype's six, no `Tất cả`", () => {
  it("six tabs in the prototype's order and words", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="tin-tuc" datLoai={() => {}} />);
    const labels = [...html.matchAll(/role="tab"[^>]*>(?:<svg[^]*?<\/svg>)?([^<]*)<\/button>/g)].map((m) => m[1]);
    expect(labels).toEqual(["Tin tức", "Sự kiện", "Thông báo", "Truyền thanh", "Video", "Banner"]);
    expect(html).not.toContain("Tất cả");
    expect(html).toContain("lucide-radio");
    expect(html).toContain("lucide-images");
  });

  it("real tabs: a `tablist` of six `tab`s, exactly one selected and the only one in the Tab order", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="video" datLoai={() => {}} />);
    expect(html).toContain('role="tablist"');
    expect(html).toContain('aria-label="Loại nội dung"');
    expect(html.match(/role="tab"/g)?.length).toBe(6);
    expect(html.match(/aria-selected="true"/g)?.length).toBe(1);
    expect(html.match(/tabindex="0"/g)?.length).toBe(1);
    expect(html.match(/tabindex="-1"/g)?.length).toBe(5);
    expect(html).toMatch(/id="content-type-tab-video"[^>]*aria-selected="true"[^>]*tabindex="0"/);
    expect(html.match(new RegExp(`aria-controls="${CONTENT_TABPANEL_ID}"`, "g"))?.length).toBe(6);
    expect(html).not.toContain("aria-pressed");
    expect(contentTabId("banner")).toBe("content-type-tab-banner");
  });

  it("the prototype's pill bar: canvas fill, the open tab white and raised", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="tin-tuc" datLoai={() => {}} />);
    expect(html).toMatch(/role="tablist"/);
    expect(html).toContain("rounded-[10px] border border-line bg-canvas p-1");
    expect(html).toMatch(/id="content-type-tab-tin-tuc"[^>]*class="[^"]*bg-white text-navy shadow-card"/);
  });
});

describe("hàng lọc §6", () => {
  it("search filters as you type: no `Tìm` button, no submit, the prototype's box", () => {
    const html = filterRow();
    expect(html).toContain('role="search"');
    expect(html).not.toContain('type="submit"');
    expect(html).not.toContain(">Tìm<");
    expect(html).toContain('placeholder="Tìm theo tiêu đề…"');
    expect(html).toContain("lucide-search");
    expect(html).toMatch(/<input id="tim-noi-dung"[^>]*class="[^"]*h-9[^"]*pl-9[^"]*text-\[12.5px\]/);
    expect(html).toContain("w-72");
  });

  it("the category select: only once the commune has a visible category; `Tất cả danh mục` first; `Cha › Con`; hidden ones left out", () => {
    expect(filterRow()).not.toContain('id="loc-danh-muc"');
    expect(filterRow({ danhMuc: [danhMuc({ hidden: true })] })).not.toContain('id="loc-danh-muc"');
    const ds = [
      danhMuc({ id: "P", name: "Tin hoạt động" }),
      danhMuc({ id: "C", name: "Chuyển đổi số", parent_id: "P" }),
      danhMuc({ id: "H", name: "Mục đã ẩn", hidden: true }),
    ];
    const html = filterRow({ danhMuc: ds });
    const options = [...(/<select id="loc-danh-muc"[^>]*>([^]*?)<\/select>/.exec(html)?.[1] ?? "").matchAll(/>([^<]*)<\/option>/g)].map(
      (m) => m[1],
    );
    expect(options).toEqual([MOI_DANH_MUC, "Tin hoạt động", "Tin hoạt động › Chuyển đổi số"]);
  });

  it("a category still the active filter keeps its select, even if hidden since", () => {
    expect(filterRow({ danhMucID: "X" })).toContain('id="loc-danh-muc"');
  });
});

describe("§6 status filter (kept, owner D1)", () => {
  it("offers Tất cả / Đang hiện / Ẩn / Chờ duyệt with the server's three codes, Tất cả sending nothing", () => {
    const html = filterRow();
    expect(html).toMatch(/<label for="loc-trang-thai"[^>]*>Trạng thái<\/label>/);
    const options = [...html.matchAll(/<option value="([^"]*)"[^>]*>([^<]*)<\/option>/g)]
      .map((m) => [m[1], m[2]])
      .filter(([v]) => ["", "dang-hien", "an", "cho-duyet"].includes(v ?? "x"));
    expect(options.slice(-4)).toEqual([
      ["", "Tất cả"],
      ["dang-hien", "Đang hiện"],
      ["an", "Ẩn"],
      ["cho-duyet", "Chờ duyệt"],
    ]);
    expect(STATUS_FILTER_OPTIONS.map((o) => o.value)).toEqual(["", "dang-hien", "an", "cho-duyet"]);
  });

  it("the chosen status is the select's value", () => {
    expect(filterRow({ status: "cho-duyet" })).toMatch(/<option value="cho-duyet" selected="">Chờ duyệt<\/option>/);
  });
});

describe("thẻ Danh bạ chính quyền §4", () => {
  const card = (count: Parameters<typeof TheDanhBaChinhQuyen>[0]["publishedCount"]) =>
    renderToStaticMarkup(<TheDanhBaChinhQuyen publishedCount={count} />);

  it("after a successful read: §4's sentence with the server's number, and the link", () => {
    const html = card({ ok: true, duLieu: 26 });
    expect(html).toContain("Đang hiện 26 cán bộ cho bà con. Chọn thêm hoặc bớt ở ngăn Danh bạ cán bộ.");
    expect(html).toContain('href="/mini-app?tab=danh-ba"');
  });

  it("the prototype's look: white card, brand icon, `Mở →` in brand", () => {
    const html = card({ ok: true, duLieu: 1 });
    expect(html).toContain("mb-5 flex min-w-0 items-center gap-3 rounded-[10px] border border-line bg-white px-4 py-3");
    expect(html).toContain("hover:border-brand/40");
    expect(html).toMatch(/lucide-book-user[^"]*size-5 shrink-0 text-brand"/);
    expect(html).toMatch(/<span class="shrink-0 text-\[12.5px\] font-semibold text-brand">Mở/);
  });

  it("a successful 0 is said — it is true", () => {
    expect(card({ ok: true, duLieu: 0 })).toContain("Đang hiện 0 cán bộ cho bà con.");
  });

  it("while loading: NO number at all", () => {
    const html = card(null);
    expect(html).not.toMatch(/\d+ cán bộ/);
    expect(html).toContain(MO_TA_THE_DANH_BA);
    expect(html).not.toContain(PUBLISHED_STAFF_UNREAD);
  });

  it("after a failed read: NO number — never a 0 that lies — and the failure is said", () => {
    const html = card({ ok: false, thongBao: "Không kết nối được máy chủ. Vui lòng thử lại." });
    expect(html).not.toMatch(/\d+ cán bộ/);
    expect(html).toContain(MO_TA_THE_DANH_BA);
    expect(html).toContain(PUBLISHED_STAFF_UNREAD);
  });
});

describe("biểu mẫu danh mục tin — the prototype's add row (`CategoryManagerDialog.tsx:127-166`)", () => {
  const ve = (ds: readonly comms_danhMucRa[], loi: string | null = null) =>
    renderToStaticMarkup(<FormDanhMuc danhMuc={ds} dangGui={false} loi={loi} luu={() => {}} />);

  it("one row: `Thêm danh mục` (placeholder Nông nghiệp) · `Thuộc mục (nếu có)` · `Slug` · `+ Thêm` — no Huỷ/Lưu, no order", () => {
    const html = ve([danhMuc()]);
    const labels = [...html.matchAll(/<label for="([^"]+)"[^>]*>([^<]*)/g)].map((m) => [m[1], m[2]]);
    expect(labels).toEqual([
      ["ten-danh-muc", "Thêm danh mục"],
      ["cha-danh-muc", "Thuộc mục (nếu có)"],
      ["slug-danh-muc", "Slug"],
    ]);
    expect(html).toContain('placeholder="Nông nghiệp"');
    expect(html).toMatch(/<button[^>]*type="submit"[^>]*>[^]*?lucide-plus[^]*?Thêm<\/button>/);
    expect(html).not.toContain(">Huỷ<");
    expect(html).not.toContain(">Lưu<");
    expect(html).not.toContain("Thứ tự hiển thị");
    expect(html).not.toContain("Thêm danh mục tin");
    expect(html).toMatch(/<div class="flex flex-wrap items-end gap-2">/);
  });

  it("nói trước hai điều máy chủ sẽ từ chối: khuôn slug, và slug đã cấp không cấp lại", () => {
    const html = ve([]);
    expect(html).toContain("chữ thường a-z");
    expect(html).toContain("KHÔNG cấp lại");
  });

  it("the server's sentence is shown verbatim under the row", () => {
    expect(ve([], "Slug đã có trong xã.")).toContain('role="alert">Slug đã có trong xã.</p>');
  });
});

describe("`CategoryAdmin` — grouped rows with rename · Tắt/Bật · Xoá (ADR 0067 §3)", () => {
  const never = () => new Promise<never>(() => {});
  const ve = (ds: readonly comms_danhMucRa[]) =>
    renderToStaticMarkup(<CategoryAdmin categories={ds} update={never} remove={never} changed={() => {}} />);

  it("every row: its name in an editable box, `Tắt`/`Bật` and the bin, named with the category", () => {
    const html = ve([danhMuc(), danhMuc({ id: "01JDM2", name: "An ninh", slug: "an-ninh", hidden: true })]);
    expect(html).toContain(CATEGORY_ROOT_GROUP);
    expect(html).toContain('value="Chuyển đổi số"');
    expect(html).toContain(nhuTrongHTML("Xoá danh mục An ninh"));
    expect(html).toContain(">Tắt</button>");
    expect(html).toContain(">Bật</button>");
    expect(html).not.toContain(">Sửa<");
    expect(html).not.toContain("Slug:");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PHẦN CHƯA DỰNG ĐƯỢC — mỗi mục PHẢI ra HTML
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("phần chưa dựng được — dấu '?' tại chỗ, không còn khối gấp ở đầu màn (MA-02)", () => {
  const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");

  it("màn không còn khối 'N phần của bản thiết kế chưa dựng được'", () => {
    const src = readFileSync(new URL("./so-noi-dung.tsx", import.meta.url), "utf8");
    expect(src).not.toContain("phần của bản thiết kế chưa dựng được");
    expect(src).not.toContain("KhoiChuaDung");
  });

  it("lý do viết cho cán bộ đọc: không số hiệu ADR, không ký hiệu mục đặc tả", () => {
    expect(all).not.toMatch(/ADR|§/);
    expect(all).toContain("đã chọn n/tổng số chuyên mục");
    expect(all).toContain("tối đa 30 chuyên mục");
  });

  it("thứ chặn THẬT được gọi tên — không phải thứ đã có", () => {
    for (const built of [
      "Đang hiện 26 cán bộ",
      "aria-pressed",
      "bộ lập lịch",
      "adapter HTTP",
      "Lọc riêng các bài `Chờ duyệt`",
      "tệp âm thanh",
      "Chọn tệp từ máy",
      "core/crypto` chưa tồn tại",
      "/api/cong/mini-app",
    ]) {
      expect(all).not.toContain(built);
    }
  });
});

describe("nhãn nút chính của màn", () => {
  it("đúng chữ đặc tả, kể cả dấu cộng", () => {
    expect(NHAN_NUT_THEM).toBe("+ Thêm nội dung");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §7 `Ảnh đại diện` / `Ảnh banner` — the prototype's dashed slot, the 3-step flow kept
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("cover block of §7", () => {
  const READY_FORM = { ...FORM_TRONG, title: "Có tiêu đề" };

  it("a dashed slot that IS the picker: Upload icon, `Chọn tệp từ máy`, the hint; JPG/PNG/WebP only", () => {
    const html = veForm();
    expect(html).toMatch(/<input id="anh-noi-dung"[^>]*type="file"[^>]*accept="\.jpg,\.jpeg,\.png,\.webp,image\/jpeg,image\/png,image\/webp"/);
    // The input first (focusable), its label right after: the visible slot.
    expect(html).toMatch(/<input id="anh-noi-dung"[^>]*class="peer an-thi-giac"[^>]*\/><label for="anh-noi-dung" class="[^"]*border-dashed[^"]*peer-focus-visible:ring-3/);
    expect(html).toContain("lucide-upload");
    expect(html).toContain(`>${COVER_PICK_BUTTON}</span>`);
    expect(html).toContain(COVER_HINT);
    expect(html).toContain(">Ảnh đại diện</span>");
    expect(html).not.toContain("heic");
    expect(html).not.toMatch(/<input[^>]*id="anh-noi-dung"[^>]*type="url"/);
  });

  it("a banner's cover is `Ảnh banner` with its landscape hint", () => {
    const html = veForm({ ...FORM_TRONG, type: "banner" });
    expect(html).toContain(">Ảnh banner</span>");
    expect(html).toContain(COVER_BANNER_HINT);
    expect(html).not.toContain(">Ảnh đại diện</span>");
  });

  it("uploading: progress as `role=status`, Lưu held and saying `Đang tải tệp…`, the picker frozen", () => {
    const html = veForm(READY_FORM, undefined, { kind: "uploading", id: "01JC", percent: 40 });
    expect(html).toMatch(/<p role="status"[^>]*>Đang tải lên 40%<\/p>/);
    expect(html).toMatch(new RegExp(`<button type="submit"[^>]*disabled=""[^>]*>[^]*?${UPLOADING_FILES_LABEL}</button>`));
    expect(html).toMatch(/<input[^>]*id="anh-noi-dung"[^>]*disabled=""/);
  });

  it("the server's 422 sentence is shown VERBATIM as an alert — text, not colour only", () => {
    const sentence = "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.";
    const html = veForm(READY_FORM, undefined, { kind: "refused", message: sentence });
    expect(html).toContain(`<p role="alert" class="thong-bao-loi">Bị từ chối: ${sentence}</p>`);
    expect(html).not.toMatch(/<button type="submit"[^>]*disabled=""/);
  });

  it("retry state offers `Kiểm tra lại` (re-complete, never re-upload)", () => {
    const html = veForm(READY_FORM, undefined, { kind: "retry", id: "01JC", message: "Chưa quét được mã độc." });
    expect(html).toContain(">Kiểm tra lại</button>");
    expect(html).toContain("Chưa kiểm tra xong: Chưa quét được mã độc.");
  });

  it("edit form: the slot says a file is there — no preview, no saved-cover line, no `Gỡ ảnh`", () => {
    const row = hang({
      body: "",
      image_url: "https://legacy.example.vn/anh.jpg",
      cover_image_file_id: "01JCOVER0",
      cover_image: {
        file_id: "01JCOVER0",
        status: "ready",
        public: true,
        preview_url: "https://files.example.test/vigov-pub/t_01JXA/cover.jpg?X-Amz-Signature=S",
      },
    });
    const html = veForm(giaTriTuHang(row), row);
    expect(html).not.toContain("<img");
    expect(html).toContain(`>${COVER_REPLACE_BUTTON}</span>`);
    expect(COVER_REPLACE_BUTTON).toBe("Đã có tệp — chọn tệp mới để thay");
    expect(html).not.toContain(COVER_REMOVE_BUTTON);
    expect(html).not.toContain(COVER_PREVIEW_MISSING);
    expect(html).not.toContain("Ảnh bìa hiện tại");
  });

  it("no cover on the article: no image, no `Gỡ ảnh`", () => {
    const html = veForm(READY_FORM);
    expect(html).not.toContain("<img");
    expect(html).not.toContain(`>${COVER_REMOVE_BUTTON}<`);
  });

  it("no `will be detached` line either", () => {
    const row = hang({ body: "", cover_image_file_id: "01JCOVER0" });
    const html = veForm({ ...giaTriTuHang(row), cover_image_file_id: "" }, row);
    expect(html).not.toContain(COVER_WILL_DETACH);
    expect(html).toContain(`>${COVER_PICK_BUTTON}</span>`);
  });
});

describe("an item waiting for approval", () => {
  it("no `Chờ duyệt` notice in the edit form (prototype parity 09/10/2026); the tick starts off", () => {
    const waiting = hang({ source: "dong-bo-cong", status: "cho-duyet", body: "<p>x</p>" });
    const html = veForm(giaTriTuHang(waiting), waiting);
    expect(html).not.toContain(nhuTrongHTML(PENDING_REVIEW_HINT));
    expect(giaTriTuHang(waiting).publish).toBe(false);
  });
});
