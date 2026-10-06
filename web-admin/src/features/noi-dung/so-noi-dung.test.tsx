import { readFileSync } from "node:fs";

import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { comms_danhMucRa, comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  BANNER_COVER_NOTICE,
  CANH_BAO_HTML_THO,
  CATEGORY_HIDDEN,
  CATEGORY_HIDE_EXPLAINER,
  CATEGORY_SHOWN,
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
  MO_TA_THE_DANH_BA,
  MOI_DANH_MUC,
  MOI_LOAI_NHAN,
  NHAN_DA_SUA_TAY,
  NHAN_NUT_DANH_MUC,
  NHAN_NUT_SUA,
  NHAN_NUT_THEM,
  NHAN_O_DANG,
  PENDING_REVIEW_HINT,
  PHAN_CHUA_DUNG,
  THUMBNAIL_PART,
  portalCategoryLabel,
  PUBLISHED_STAFF_UNREAD,
  SO_RONG,
  STATUS_FILTER_OPTIONS,
  VIDEO_URL_HINT,
} from "./nhan-noi-dung";
import {
  COVER_HINT,
  COVER_PICK_BUTTON,
  COVER_PREVIEW_MISSING,
  COVER_REMOVE_BUTTON,
  COVER_REPLACE_BUTTON,
  COVER_WAIT_NOTE,
  COVER_WILL_DETACH,
  type CoverUploadState,
} from "./cover-image";
import { CategoryAdmin } from "./category-admin";
import { EDITOR_LOADING } from "./rich-text-editor";
import {
  BangNoiDung,
  CONTENT_TABPANEL_ID,
  contentTabId,
  FormDanhMuc,
  FormNoiDung,
  HangLocNoiDung,
  HeaderActions,
  ThanhTabLoai,
  TheDanhBaChinhQuyen,
  ThongTinChiDoc,
} from "./so-noi-dung";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **HTML KHÔNG ĐƯỢC DỰNG**: the body is never handed to the
 * page as a string. On the server render the editor has not mounted (it needs a DOM), so the stored body
 * must not appear AT ALL — escaped or not; the editor's own behaviour is `rich-text.test.ts` (jsdom).
 *
 * Nhóm thứ hai canh một sự VẮNG MẶT, thứ khó canh nhất vì không có gì để tìm trên trang: without
 * `content.update` there is no `✎`/`🗑` in the table, and KHÔNG có ô chọn trạng thái trong biểu mẫu.
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ `renderToStaticMarkup` thoát `&`, `<`, `>`, `"` và `'`, nên một phép `not.toContain` với chuỗi
 * thô sẽ XANH kể cả khi chữ ấy đang nằm chình ình trên trang — tức là canh đúng con số không. Đã
 * đo ở `features/thu-chi/bang-thu-chi.test.tsx`.
 *
 * NĂM KÝ TỰ, KHÔNG PHẢI HAI như bản ở màn Thông báo — và sự khác ấy do chính màn này gây ra: chữ
 * trên màn ở đây NHẮC TỚI HTML (`` `<script>` ``, `<p>`), nên `<` và `>` xuất hiện thật trong văn
 * bản chứ không chỉ trong dữ liệu thử. Đo 24/09/2026: thiếu hai ký tự ấy thì ca "TỪNG mục ra tới
 * trang" đỏ ngay ở mục đầu tiên — mục quan trọng nhất của khối.
 *
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
    <BangNoiDung ds={ds} danhMuc={dm} sua={() => {}} remove={() => {}} canEdit={canEdit} />,
  );
}

/** The table's body only — the header carries a "?" popover trigger of its own. */
const bodyOf = (html: string) => html.slice(html.indexOf("<tbody>"));

function veForm(gt = FORM_TRONG, h?: comms_noiDungRa, coverState?: CoverUploadState) {
  return renderToStaticMarkup(
    <FormNoiDung
      tieuDeForm="Thêm nội dung cho Mini App"
      moTa="mô tả"
      giaTriDau={gt}
      hang={h}
      danhMuc={[danhMuc()]}
      dangGui={false}
      loi={null}
      huy={() => {}}
      luu={() => {}}
      initialCoverState={coverState}
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
    // No tag opened from it: the danger this screen exists to avoid.
    expect(html).not.toContain("<script");
    expect(html).not.toContain("<img");
    // And not even escaped: the body lives only inside the editor, which mounts in the browser.
    expect(html).not.toContain("alert");
    expect(html).toContain(EDITOR_LOADING);
  });

  it("the one note about sanitising stands NEXT TO the box, not in a code comment", () => {
    const html = veForm();
    expect(html).toContain(nhuTrongHTML(CANH_BAO_HTML_THO));
    expect(html).toContain('id="than-bai-noi-dung-hint"');
    // The old sentences about raw HTML source are gone with the textarea.
    expect(html).not.toContain("MÃ NGUỒN HTML");
    expect(html).not.toContain("<textarea id=\"than-bai-noi-dung\"");
  });

  it("KHÔNG có ô xem trước nào — không `dangerouslySetInnerHTML`, không khối `preview`", () => {
    // Phép quét mã nguồn nằm ở `ranh-gioi-html.test.ts`; ca này canh cùng điều ấy ở đầu ra.
    const html = veForm({ ...FORM_TRONG, body: "<b>đậm</b>" });
    expect(html).not.toContain("<b>đậm</b>");
    expect(html).not.toContain("&lt;b&gt;");
  });

  it("tiêu đề mang dấu ngoặc nhọn cũng ra dạng đã thoát ở BẢNG", () => {
    // Bảng §6 không hiện thân bài, nhưng tiêu đề và tóm tắt cũng là chữ cán bộ gõ.
    const html = veBang([hang({ title: "<b>Tin nóng</b>", summary: "<i>ngay</i>" })]);
    expect(html).toContain("&lt;b&gt;Tin nóng&lt;/b&gt;");
    expect(html).not.toContain("<b>Tin nóng</b>");
  });

  it("liên kết bài gốc hiện dưới dạng CHỮ, không thành một `href` bấm được", () => {
    // Danh sách trắng lược đồ ở máy chủ chỉ chặn lúc GHI. Một hàng cũ trong CSDL không có gì bảo
    // đảm điều đó, và `javascript:…` trong một `href` là thực thi mã.
    const html = renderToStaticMarkup(
      <ThongTinChiDoc hang={hang({ source_url: "javascript:alert(1)" })} />,
    );
    expect(html).toContain("javascript:alert(1)");
    expect(html).not.toContain('href="javascript:alert(1)"');
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢNG §6
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("bảng nội dung §6", () => {
  it("sáu cột dữ liệu cùng ra một hàng", () => {
    const html = veBang([hang()]);

    expect(html).toContain("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số");
    expect(html).toContain("Hội nghị diễn ra sáng 14/9");
    expect(html).toContain("Tin tức");
    expect(html).toContain("Chuyển đổi số");
    // The `🔗` of the label is drawn as a lucide icon now (ADR 0068 §2); the words are unchanged.
    expect(html).toContain(">Có ảnh<");
    expect(html).toContain("lucide-image");
    expect(html).toContain("14/9/2026");
    expect(html).toContain("Đang hiện");
  });

  it("cột `Lượt xem` nằm sau `Ngày đăng`, trước `Trạng thái` (§6, owner 02/10/2026)", () => {
    const html = veBang([hang({ view_count: 42 })]);
    const headers = [...html.matchAll(/<th scope="col"[^>]*>([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(headers).toEqual([
      "Tiêu đề",
      "Loại",
      "Chuyên mục",
      "Tệp đính kèm",
      "Ngày đăng",
      "Lượt xem",
      "Trạng thái",
      ACTIONS_COLUMN_LABEL,
    ]);
    expect(html).toContain("lucide-eye");
    expect(html).toContain(">42</span>");
  });

  it("lượt xem in theo vi-VN: 1234 → `1.234`, và 0 hiện `0` (không phải dấu gạch)", () => {
    expect(veBang([hang({ view_count: 1234 })])).toContain(">1.234</span>");
    expect(veBang([hang({ view_count: 0 })])).toMatch(/lucide-eye[^]*?<\/svg>0<\/span>/);
  });

  it("cột `Lượt xem` không sắp xếp được — chỉ là chữ, không nút, không aria-sort", () => {
    const html = veBang([hang({ view_count: 42 })]);
    const th = /<th scope="col"[^>]*>Lượt xem<\/th>/.exec(html)?.[0] ?? "";
    expect(th).not.toBe("");
    expect(th).not.toContain("aria-sort");
    expect(th).not.toContain("<button");
  });

  it("the action column holds the pencil and the trash (lucide icons since ADR 0068)", () => {
    const html = veBang([hang()]);
    expect(html).toContain(NHAN_NUT_SUA);
    expect(html).toContain(`title="${CONTENT_DELETE_TITLE}"`);
    expect(html).toContain(`>${ACTIONS_COLUMN_LABEL}</th>`);
  });

  it("nút sửa mang nhãn nói rõ đang sửa bài nào — sáu hàng cùng một ký hiệu thì đọc không ra", () => {
    const html = veBang([hang()]);
    expect(html).toContain(nhuTrongHTML("Sửa: Xã tổ chức hội nghị tổng kết công tác chuyển đổi số"));
  });

  it("the delete button names its row too, and only OPENS a dialog", () => {
    const html = veBang([hang()]);
    expect(html).toContain(
      `aria-label="${nhuTrongHTML(contentDeleteAriaLabel("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số"))}"`,
    );
    // Counted in the BODY: the header's thumbnail "?" opens a popover of its own.
    expect(bodyOf(html).match(/aria-haspopup="dialog"/g)?.length).toBe(2);
  });

  it("bài chưa xếp danh mục hiện đúng chữ đặc tả, không hiện ô trống", () => {
    expect(veBang([hang({ category_id: "" })])).toContain(CHUA_XEP_DANH_MUC);
  });

  it("cờ §10.4 ra tới hàng — cán bộ phải thấy bài đã thoát khỏi lượt đồng bộ", () => {
    expect(veBang([hang({ hand_edited: true })])).toContain(NHAN_DA_SUA_TAY);
    expect(veBang([hang({ hand_edited: false })])).not.toContain(NHAN_DA_SUA_TAY);
  });

  it("chip `Chờ duyệt` và `Ẩn` hiện đúng chữ, không lẫn với `Đang hiện`", () => {
    expect(veBang([hang({ status: "cho-duyet" })])).toContain("Chờ duyệt");
    expect(veBang([hang({ status: "an" })])).toContain("Ẩn");
    expect(veBang([hang({ status: "an" })])).not.toContain("Đang hiện");
  });

  it("sổ rỗng thì nói ra, không để trang trắng", () => {
    expect(veBang([])).toContain(SO_RONG);
  });

  it("the summary is clamped by CSS to one line — NEVER cut by characters", () => {
    // 400 characters: well past the old 140-character cut, so a surviving slice would show here.
    const long = `${"Hội nghị tổng kết công tác chuyển đổi số của xã ".repeat(8)}KẾT-THÚC`;
    const html = veBang([hang({ summary: long })]);
    expect(html).toContain('<span class="dong-phu summary-one-line">');
    expect(html).toContain("KẾT-THÚC");
    expect(html).not.toContain("…");
  });
});

describe("`content.update` gates the table's write controls", () => {
  it("DENIED: no `✎`, no `🗑`, no action column — the data columns stay", () => {
    const html = veBang([hang()], [danhMuc()], false);
    expect(html).not.toContain(NHAN_NUT_SUA);
    expect(html).not.toContain(CONTENT_DELETE_TITLE);
    expect(html).not.toContain("Xoá");
    expect(html).not.toContain(`>${ACTIONS_COLUMN_LABEL}</th>`);
    expect(bodyOf(html)).not.toContain("aria-haspopup");
    // Eight header cells: the thumbnail "?" (prototype, not served), Tiêu đề, Loại, Chuyên mục, Tệp đính
    // kèm, Ngày đăng, Lượt xem, Trạng thái — and no Thao tác.
    expect(html.match(/<th scope="col"/g)?.length).toBe(8);
    expect(html).toContain("Xã tổ chức hội nghị tổng kết công tác chuyển đổi số");
  });

  it("ALLOWED: the pencil and the trash on every row, each saying it opens a dialog", () => {
    const html = veBang([hang({ id: "A" }), hang({ id: "B" })], [danhMuc()], true);
    expect(bodyOf(html).match(/aria-haspopup="dialog"/g)?.length).toBe(4);
    expect(html.match(new RegExp(`title="${CONTENT_DELETE_TITLE}"`, "g"))?.length).toBe(2);
    expect(html).toContain(`>${ACTIONS_COLUMN_LABEL}</th>`);
  });

  it("`+ Thêm nội dung`: absent without the key, present with it — the prototype's Plus icon + words", () => {
    expect(renderToStaticMarkup(<HeaderActions canEdit={false} addOpen={false} openAdd={() => {}} />)).toBe("");
    const html = renderToStaticMarkup(<HeaderActions canEdit addOpen={false} openAdd={() => {}} />);
    expect(html).toContain("lucide-plus");
    expect(html).toContain("Thêm nội dung");
    expect(html).not.toContain(nhuTrongHTML(NHAN_NUT_THEM));
    expect(html).toContain('aria-haspopup="dialog"');
  });

  it("the filter row draws what it is given at its end, and nothing when given nothing", () => {
    const row = (extra?: ReactNode) =>
      renderToStaticMarkup(
        <HangLocNoiDung
          danhMucID=""
          datDanhMucID={() => {}}
          tim=""
          datTim={() => {}}
          timNgay={() => {}}
          danhMuc={[]}
          status=""
          setStatus={() => {}}
          extra={extra}
        />,
      );
    expect(row()).not.toContain(NHAN_NUT_DANH_MUC);
    expect(row(<button type="button">{NHAN_NUT_DANH_MUC}</button>)).toContain(NHAN_NUT_DANH_MUC);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BIỂU MẪU §7
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("biểu mẫu nội dung §7", () => {
  it("bảy ô của đặc tả có mặt, và ô tích mang đúng chữ đặc tả", () => {
    const html = veForm();

    expect(html).toContain("Loại nội dung");
    expect(html).toContain("Danh mục");
    expect(html).toContain("Tiêu đề");
    expect(html).toContain("Tóm tắt");
    expect(html).toContain("Nội dung");
    expect(html).toContain("Ảnh đại diện");
    expect(html).toContain(NHAN_O_DANG);
  });

  it("KHÔNG có ô chọn trạng thái — máy chủ suy trạng thái từ ô tích", () => {
    // Một thân tự khai trạng thái là một bài đăng vượt qua bước duyệt mà §10.2 dành cho lượt đồng
    // bộ, hoặc một bài tự nhận `cho-duyet`.
    const html = veForm();
    expect(html).not.toContain('id="trang-thai-noi-dung"');
    expect(html).not.toContain("Chờ duyệt");
  });

  it("`Ảnh đại diện` is a file picker — `Chọn tệp từ máy`, JPG/PNG/WebP, no link box", () => {
    const html = veForm();
    expect(html).toMatch(/<input[^>]*id="anh-noi-dung"[^>]*type="file"[^>]*accept="\.jpg,\.jpeg,\.png,\.webp,image\/jpeg,image\/png,image\/webp"/);
    // The visible control is the input's LABEL — keyboard: the input itself stays focusable.
    expect(html).toContain(`<label for="anh-noi-dung" class="nut-phu">${COVER_PICK_BUTTON}`);
    expect(html).toContain(COVER_HINT);
    expect(html).not.toContain("heic");
    // The legacy link box is gone: no URL input carries the image any more.
    expect(html).not.toContain("Ảnh đại diện (liên kết)");
    expect(html).not.toMatch(/<input[^>]*id="anh-noi-dung"[^>]*type="url"/);
  });

  it("sáu loại của §5 đều có trong ô chọn", () => {
    const html = veForm();
    for (const nhan of ["Tin tức", "Sự kiện", "Thông báo", "Truyền thanh", "Video", "Banner"]) {
      expect(html).toContain(`>${nhan}</option>`);
    }
  });

  it("ô tích BẬT khi mở một bài đang hiện, TẮT khi mở một bài chờ duyệt", () => {
    expect(veForm(giaTriTuHang(hang({ status: "dang-hien", body: "" })))).toContain("checked");
    expect(veForm(giaTriTuHang(hang({ status: "cho-duyet", body: "" })))).not.toContain("checked");
  });

  it("nút Lưu TẮT khi tiêu đề rỗng — máy chủ vẫn là nơi từ chối thật", () => {
    // The submit button ITSELF: since ADR 0068 every button's classes carry `disabled:…` utilities, so a
    // bare `toContain("disabled")` would be green on any form.
    expect(veForm()).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    expect(veForm({ ...FORM_TRONG, title: "Có tiêu đề" })).toContain(">Lưu</button>");
  });

  it("khối chỉ đọc chỉ xuất hiện khi đang SỬA", () => {
    expect(veForm()).not.toContain("Cập nhật lúc");
    expect(veForm(FORM_TRONG, hang())).toContain("Cập nhật lúc");
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
    expect(html).toContain(nhuTrongHTML(EVENT_TIME_HINT));
    expect(html).not.toContain(VIDEO_BOX);
  });

  it("`Video` shows Liên kết video and no event box", () => {
    const html = veForm({ ...FORM_TRONG, type: "video" });
    expect(html).toContain(VIDEO_BOX);
    expect(html).toContain(">Liên kết video<");
    expect(html).toContain(nhuTrongHTML(VIDEO_URL_HINT));
    for (const box of EVENT_BOXES) expect(html).not.toContain(box);
  });

  it("the other four types show neither", () => {
    for (const type of ["tin-tuc", "thong-bao", "truyen-thanh", "banner"]) {
      const html = veForm({ ...FORM_TRONG, type });
      for (const box of [...EVENT_BOXES, VIDEO_BOX]) expect(html, type).not.toContain(box);
    }
  });

  it("`Banner` shows Liên kết khi bấm · Thứ tự hiển thị and the cover notice; no other type does", () => {
    const html = veForm({ ...FORM_TRONG, type: "banner" });
    expect(html).toContain('id="lien-ket-banner"');
    expect(html).toContain('id="thu-tu-banner"');
    expect(html).toContain(">Liên kết khi bấm<");
    expect(html).toContain(">Thứ tự hiển thị<");
    expect(html).toContain(nhuTrongHTML(BANNER_COVER_NOTICE));
    expect(html).toContain(nhuTrongHTML(LINK_TO_HINT));
    for (const type of ["tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video"]) {
      const other = veForm({ ...FORM_TRONG, type, link_to: "/x", display_order: "1" });
      expect(other, type).not.toContain('id="lien-ket-banner"');
      expect(other, type).not.toContain('id="thu-tu-banner"');
      expect(other, type).not.toContain(nhuTrongHTML(BANNER_COVER_NOTICE));
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
    // The submit button itself carries `disabled` (a `>Lưu</button>` match would pass either way).
    expect(html).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    // And with the window fixed, the same form lets Lưu through.
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
    const html = veForm({
      ...FORM_TRONG,
      type: "su-kien",
      title: "T",
      event_ends_local: "2026-10-05T07:00",
    });
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

describe("`Đăng lần đầu lúc` — published_at", () => {
  it("shows in the table row and in the read-only block when present", () => {
    const h = hang({ published_at: "2026-10-01T02:05:00Z" });
    expect(veBang([h])).toContain("Đăng lần đầu lúc 01/10/2026 09:05");
    expect(renderToStaticMarkup(<ThongTinChiDoc hang={h} />)).toContain(
      "Đăng lần đầu lúc 01/10/2026 09:05",
    );
  });

  it("absent for an item never published", () => {
    expect(veBang([hang()])).not.toContain("Đăng lần đầu");
    expect(renderToStaticMarkup(<ThongTinChiDoc hang={hang()} />)).not.toContain("Đăng lần đầu");
  });
});

describe("khối thông tin chỉ đọc", () => {
  it("mốc cập nhật in giờ Việt Nam kể cả khi máy chạy ở UTC", () => {
    const html = renderToStaticMarkup(<ThongTinChiDoc hang={hang()} />);
    expect(html).toContain("16:35 14/09/2026");
  });

  it("mã nghiệp vụ cán bộ, không phải id nội bộ", () => {
    expect(renderToStaticMarkup(<ThongTinChiDoc hang={hang()} />)).toContain("CB-2026-7K3M9Q");
  });

  it("có dòng `Lượt xem` chỉ đọc, và không có ô nhập nào", () => {
    const html = renderToStaticMarkup(<ThongTinChiDoc hang={hang({ view_count: 1234 })} />);
    expect(html).toContain("<dt>Lượt xem</dt><dd>1.234</dd>");
    expect(html).not.toContain("<input");
  });

  it("lượt xem bằng 0 hiện `0`", () => {
    const html = renderToStaticMarkup(<ThongTinChiDoc hang={hang({ view_count: 0 })} />);
    expect(html).toContain("<dt>Lượt xem</dt><dd>0</dd>");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TAB, BỘ LỌC, HAI THẺ ĐẦU MÀN, DANH MỤC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("sáu tab loại §5", () => {
  it("bảy nút: `Tất cả` cộng sáu loại", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="" datLoai={() => {}} />);
    expect(html).toContain(MOI_LOAI_NHAN);
    for (const nhan of ["Tin tức", "Sự kiện", "Thông báo", "Truyền thanh", "Video", "Banner"]) {
      expect(html).toContain(nhan);
    }
  });

  it("real tabs: a `tablist` of seven `tab`s, exactly one selected and the only one in the Tab order", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="video" datLoai={() => {}} />);
    expect(html).toContain('role="tablist"');
    expect(html).toContain('aria-label="Loại nội dung"');
    expect(html.match(/role="tab"/g)?.length).toBe(7);
    expect(html.match(/aria-selected="true"/g)?.length).toBe(1);
    expect(html.match(/tabindex="0"/g)?.length).toBe(1);
    expect(html.match(/tabindex="-1"/g)?.length).toBe(6);
    expect(html).toMatch(/id="content-type-tab-video"[^>]*aria-selected="true"[^>]*tabindex="0"/);
    // Every tab controls the one panel; no button is a toggle any more.
    expect(html.match(new RegExp(`aria-controls="${CONTENT_TABPANEL_ID}"`, "g"))?.length).toBe(7);
    expect(html).not.toContain("aria-pressed");
  });

  it("`Tất cả` is selected when no type is filtered, and its id is stable", () => {
    const html = renderToStaticMarkup(<ThanhTabLoai loai="" datLoai={() => {}} />);
    expect(html).toMatch(/id="content-type-tab-all"[^>]*aria-selected="true"/);
    expect(contentTabId("")).toBe("content-type-tab-all");
    expect(contentTabId("banner")).toBe("content-type-tab-banner");
  });
});

describe("hàng lọc §6", () => {
  it("ô tìm và ô chọn danh mục cùng ra một hàng", () => {
    const html = renderToStaticMarkup(
      <HangLocNoiDung
        danhMucID=""
        datDanhMucID={() => {}}
        tim=""
        datTim={() => {}}
        timNgay={() => {}}
        danhMuc={[danhMuc()]}
        status=""
        setStatus={() => {}}
      />,
    );

    expect(html).toContain("Tìm theo tiêu đề");
    expect(html).toContain(MOI_DANH_MUC);
    expect(html).toContain("Chuyển đổi số");
  });

  it("ô tìm nằm trong một `form` — gửi bằng submit, không gửi theo từng phím", () => {
    // Mỗi phím là một lời gọi mang chữ cán bộ đang gõ vào một URL, và một URL đi vào mọi log
    // truy cập (luật 3, cấm #4).
    const html = renderToStaticMarkup(
      <HangLocNoiDung
        danhMucID=""
        datDanhMucID={() => {}}
        tim=""
        datTim={() => {}}
        timNgay={() => {}}
        danhMuc={[]}
        status=""
        setStatus={() => {}}
      />,
    );
    expect(html).toContain('role="search"');
    expect(html).toContain('type="submit"');
  });
});

describe("§6 status filter", () => {
  function filterRow(status: string) {
    return renderToStaticMarkup(
      <HangLocNoiDung
        danhMucID=""
        datDanhMucID={() => {}}
        tim=""
        datTim={() => {}}
        timNgay={() => {}}
        danhMuc={[]}
        status={status}
        setStatus={() => {}}
      />,
    );
  }

  it("offers Tất cả / Đang hiện / Ẩn / Chờ duyệt with the server's three codes, Tất cả sending nothing", () => {
    const html = filterRow("");
    // The label is Field's now (label above, its own classes); still a real `<label for>` on the select.
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
    expect(filterRow("cho-duyet")).toMatch(/<option value="cho-duyet" selected="">Chờ duyệt<\/option>/);
  });
});

describe("portal category of a synced item (C2)", () => {
  it("the row shows it under the source line; a hand-written row and a synced one without it do not", () => {
    const synced = { ...hang({ id: "A", source: "dong-bo-cong", status: "cho-duyet" }), portal_category_name: "Tin địa phương" };
    const html = veBang([synced, hang({ id: "B" }), hang({ id: "C", source: "dong-bo-cong" })]);
    expect(html.split(portalCategoryLabel("Tin địa phương")).length - 1).toBe(1);
    expect(html.indexOf("Đồng bộ từ Cổng")).toBeLessThan(html.indexOf(portalCategoryLabel("Tin địa phương")));
    expect(html).not.toContain("Chuyên mục Cổng: <");
  });

  it("the detail's read-only block names it; absent when the server sent none", () => {
    const synced = { ...hang({ source: "dong-bo-cong", body: "<p>x</p>" }), portal_category_name: "Tin địa phương" };
    const html = renderToStaticMarkup(<ThongTinChiDoc hang={synced} />);
    expect(html).toContain("<dt>Chuyên mục Cổng</dt><dd>Tin địa phương</dd>");
    expect(renderToStaticMarkup(<ThongTinChiDoc hang={hang({ body: "" })} />)).not.toContain("Chuyên mục Cổng");
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

  it("a successful 0 is said — it is true", () => {
    expect(card({ ok: true, duLieu: 0 })).toContain("Đang hiện 0 cán bộ cho bà con.");
  });

  it("while loading: NO number at all", () => {
    const html = card(null);
    expect(html).not.toMatch(/\d+ cán bộ/);
    expect(html).toContain(MO_TA_THE_DANH_BA);
    expect(html).not.toContain(PUBLISHED_STAFF_UNREAD);
    expect(html).toContain('href="/mini-app?tab=danh-ba"');
  });

  it("after a failed read: NO number — never a 0 that lies — and the failure is said", () => {
    const html = card({ ok: false, thongBao: "Không kết nối được máy chủ. Vui lòng thử lại." });
    expect(html).not.toMatch(/\d+ cán bộ/);
    expect(html).toContain(MO_TA_THE_DANH_BA);
    expect(html).toContain(PUBLISHED_STAFF_UNREAD);
  });
});

describe("biểu mẫu danh mục tin", () => {
  it("the ADD form stays an add form — edit and delete live in `CategoryAdmin`", () => {
    const html = renderToStaticMarkup(
      <FormDanhMuc
        danhMuc={[danhMuc()]}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain("Thêm danh mục tin");
    expect(html).not.toContain("🗑");
    expect(html).not.toContain(">Xoá<");
    expect(html).not.toContain(">Sửa<");
  });

  it("nói trước hai điều máy chủ sẽ từ chối: khuôn slug, và slug đã cấp không cấp lại", () => {
    const html = renderToStaticMarkup(
      <FormDanhMuc danhMuc={[]} dangGui={false} loi={null} huy={() => {}} luu={() => {}} />,
    );
    expect(html).toContain("chữ thường a-z");
    expect(html).toContain("KHÔNG cấp lại");
  });
});

describe("`CategoryAdmin` — the tree with Sửa · Ẩn/Hiện · Xoá (ADR 0067 §3)", () => {
  const never = () => new Promise<never>(() => {});
  const ve = (ds: readonly comms_danhMucRa[]) =>
    renderToStaticMarkup(
      <CategoryAdmin categories={ds} update={never} remove={never} changed={() => {}} />,
    );

  it("every row has the three actions, named with the category", () => {
    const html = ve([danhMuc(), danhMuc({ id: "01JDM2", name: "An ninh", slug: "an-ninh" })]);
    expect(html).toContain(nhuTrongHTML("Sửa danh mục Chuyển đổi số"));
    expect(html).toContain(nhuTrongHTML("Xoá danh mục An ninh"));
    expect(html.match(/>Sửa</g)?.length).toBe(2);
    expect(html.match(/>Xoá</g)?.length).toBe(2);
    expect(html).toContain("Slug: chuyen-doi-so");
  });

  it("the hide toggle reads the row's state, and the owner's rule is said next to it", () => {
    const html = ve([danhMuc(), danhMuc({ id: "01JDM2", name: "Cũ", hidden: true })]);
    expect(html).toContain(CATEGORY_SHOWN);
    expect(html).toContain(CATEGORY_HIDDEN);
    expect(html).toContain(">Ẩn trên Mini App<");
    expect(html).toContain(">Hiện lại trên Mini App<");
    // Hiding removes the chip only — the articles still show (owner, 01/10/2026).
    expect(html).toContain(nhuTrongHTML(CATEGORY_HIDE_EXPLAINER));
    expect(CATEGORY_HIDE_EXPLAINER).toContain("vẫn hiện");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PHẦN CHƯA DỰNG ĐƯỢC — mỗi mục PHẢI ra HTML
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("phần chưa dựng được — một dấu '?' trên thẻ Đồng bộ Cổng, không còn khối gấp ở đầu màn (MA-02)", () => {
  const all = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" | ");

  it("màn không còn khối 'N phần của bản thiết kế chưa dựng được'", () => {
    const src = readFileSync(new URL("./so-noi-dung.tsx", import.meta.url), "utf8");
    expect(src).not.toContain("phần của bản thiết kế chưa dựng được");
    expect(src).not.toContain("KhoiChuaDung");
  });

  it("lý do viết cho cán bộ đọc: không số hiệu ADR, không ký hiệu mục đặc tả", () => {
    expect(all).not.toMatch(/ADR|§/);
    expect(all).toContain("đã chọn n/30");
  });

  it("thứ chặn THẬT được gọi tên — không phải thứ đã có", () => {
    // Built 02/10/2026: the §4 count, the §2 layout and the `content.update` gate; ADR 0067 §2 the
    // portal sync, §4 the broadcast audio. No entry may still say they are missing.
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
 * §7 `Ảnh đại diện` — the cover block in each state (no DOM: rendered to a string)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("cover block of §7", () => {
  const READY_FORM = { ...FORM_TRONG, title: "Có tiêu đề" };

  it("uploading: progress as `role=status`, Lưu held, the wait note said", () => {
    const html = veForm(READY_FORM, undefined, { kind: "uploading", id: "01JC", percent: 40 });
    expect(html).toContain('<p role="status">Đang tải lên 40%</p>');
    expect(html).toContain(COVER_WAIT_NOTE);
    expect(html).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    // The picker is frozen while one file moves.
    expect(html).toMatch(/<input[^>]*id="anh-noi-dung"[^>]*disabled=""/);
  });

  it("the server's 422 sentence is shown VERBATIM as an alert — text, not colour only", () => {
    const sentence = "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.";
    const html = veForm(READY_FORM, undefined, { kind: "refused", message: sentence });
    expect(html).toContain(`<p role="alert" class="thong-bao-loi">Bị từ chối: ${sentence}</p>`);
    // A refusal does not hold Lưu: the article can be saved without a cover.
    expect(html).not.toMatch(/<button type="submit"[^>]*disabled=""/);
  });

  it("retry state offers `Kiểm tra lại` (re-complete, never re-upload)", () => {
    const html = veForm(READY_FORM, undefined, { kind: "retry", id: "01JC", message: "Chưa quét được mã độc." });
    expect(html).toContain(">Kiểm tra lại</button>");
    expect(html).toContain("Chưa kiểm tra xong: Chưa quét được mã độc.");
  });

  it("edit form: the saved cover's preview comes ONLY from the server's `preview_url`", () => {
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
    expect(html).toContain('src="https://files.example.test/vigov-pub/t_01JXA/cover.jpg?X-Amz-Signature=S"');
    expect(html.match(/<img /g)?.length).toBe(1);
    // The legacy link is shown as text in the read-only block, never as a `src`.
    expect(html).not.toContain('src="https://legacy.example.vn/anh.jpg"');
    expect(html).toContain("https://legacy.example.vn/anh.jpg");
    expect(html).toContain(`>${COVER_REPLACE_BUTTON}`);
    expect(html).toContain(`>${COVER_REMOVE_BUTTON}</button>`);
  });

  it("a non-http(s) `preview_url` is never a `src`; a missing one says so", () => {
    const row = hang({
      body: "",
      cover_image_file_id: "01JCOVER0",
      cover_image: { file_id: "01JCOVER0", status: "ready", public: false, preview_url: "javascript:alert(1)" },
    });
    const html = veForm(giaTriTuHang(row), row);
    expect(html).not.toContain("<img");
    expect(html).toContain(COVER_PREVIEW_MISSING);
  });

  it("no cover on the article: no image, no `Gỡ ảnh`", () => {
    const html = veForm(READY_FORM);
    expect(html).not.toContain("<img");
    expect(html).not.toContain(`>${COVER_REMOVE_BUTTON}<`);
  });

  it("`Gỡ ảnh` pressed on a saved cover: the detach is said before Lưu", () => {
    const row = hang({ body: "", cover_image_file_id: "01JCOVER0" });
    const html = veForm({ ...giaTriTuHang(row), cover_image_file_id: "" }, row);
    expect(html).toContain(COVER_WILL_DETACH);
    expect(html).not.toContain("<img");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * Items from the portal sync (ADR 0067 §2) — visible, and publishable when waiting for approval
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("portal-synced items in the table and the edit form", () => {
  it("a synced row says it came from the portal; a hand-written one does not", () => {
    const html = veBang([
      hang({ id: "A", source: "dong-bo-cong", status: "cho-duyet" }),
      hang({ id: "B", source: "thu-cong" }),
    ]);
    expect(html.split("Đồng bộ từ Cổng").length - 1).toBe(1);
    expect(html).toContain(">Chờ duyệt<");
  });

  it("an item waiting for approval says how to publish it; a published one does not", () => {
    const waiting = hang({ source: "dong-bo-cong", status: "cho-duyet", body: "<p>x</p>" });
    const html = veForm(giaTriTuHang(waiting), waiting);
    expect(html).toContain(nhuTrongHTML(PENDING_REVIEW_HINT));
    // The tick is OFF for `cho-duyet`, so ticking it is what publishes (thanSua sends publish: true).
    expect(giaTriTuHang(waiting).publish).toBe(false);
    const live = hang({ status: "dang-hien", body: "<p>x</p>" });
    expect(veForm(giaTriTuHang(live), live)).not.toContain(nhuTrongHTML(PENDING_REVIEW_HINT));
  });
});

describe("bảng theo bản mẫu (ADR 0068 lần 5, 06/10/2026)", () => {
  it("the prototype's thumbnail column is a '?' header and '—' cells: the list route serves no image link", () => {
    const html = veBang([hang()]);
    expect(html).toContain(`aria-label="${pendingMarkerLabel(THUMBNAIL_PART)}"`);
    expect(bodyOf(html)).toContain("data-pending");
    expect(html).not.toContain("<img");
  });

  it("`Loại` is drawn under `Tất cả` only — each other tab IS one type, as in the prototype", () => {
    const all = renderToStaticMarkup(
      <BangNoiDung ds={[hang()]} danhMuc={[danhMuc()]} sua={() => {}} remove={() => {}} canEdit showType />,
    );
    const one = renderToStaticMarkup(
      <BangNoiDung ds={[hang()]} danhMuc={[danhMuc()]} sua={() => {}} remove={() => {}} canEdit showType={false} />,
    );
    expect(all).toContain(">Loại</th>");
    expect(one).not.toContain(">Loại</th>");
  });

  it("a long title stays one line, cut by CSS, the whole title on hover; the table scrolls inside its frame", () => {
    const long = "Tiêu đề rất dài ".repeat(20).trim();
    const html = veBang([hang({ title: long })]);
    expect(html).toContain(`title="${long}"`);
    expect(html).toMatch(/class="ten-can-bo block overflow-hidden text-ellipsis whitespace-nowrap"/);
    expect(html).toContain("max-w-[34rem]");
    expect(html).toContain("overflow-auto");
  });
});

