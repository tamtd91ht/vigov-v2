import { readFileSync } from "node:fs";

import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { KetQua } from "@/lib/api/goi";
import type { comms_thongBaoRa, identity_danhBaChonNguoiRa } from "@/lib/api/schema.gen";

import {
  ACKNOWLEDGED_CHIP,
  CANH_BAO_CHUA_GUI_THU,
  CHIP_BAT_BUOC_XAC_NHAN,
  CHUA_CHON_THONG_BAO,
  DANG_TAI_DANH_BA,
  DANH_BA_NGUOI_NHAN_RONG,
  GHI_CHU_GHIM_TRONG_TRANG,
  NHAN_CHON_NGUOI_NHAN,
  NHAN_NUT_PHAT_HANH,
  PHAN_CHUA_DUNG,
  pendingPart,
  PINNED_LABEL,
  RECIPIENT_UNITS_CHIP,
  SAVE_DRAFT_LABEL,
  SO_RONG,
} from "./nhan-thong-bao";
import {
  ChiTietThongBao,
  DanhSachThongBao,
  docDanhBaNguoiNhan,
  FormSoanThongBao,
  TheThongBao,
} from "./so-thong-bao";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **BỘ PHẬN NHẬN**. Máy chủ trả 501 cho mọi thân mang
 * `org_unit_ids`, nên ô chọn bộ phận của §5 chỉ là MỘT chip vô hiệu kèm dấu "?" (ADR 0068 §14):
 * không tên bộ phận nào, không nút nào bấm được, và lý do nằm sau dấu "?". Bấm "?" thật sự không
 * gọi mạng: `announcement-placeholders.test.tsx` (jsdom).
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

function thongBao(sua: Partial<comms_thongBaoRa> = {}): comms_thongBaoRa {
  return {
    id: "01JTB1",
    title: "Thông báo về việc triển khai hệ thống an ninh",
    body: "Đề nghị các bộ phận cử cán bộ dự buổi tập huấn lúc 8h00.",
    status: "da-phat-hanh",
    pinned: false,
    ack_required: true,
    email_requested: true,
    email_status: "chua-gui",
    author_code: "CB-2026-7K3M9Q",
    recipient_count: 12,
    ack_count: 2,
    issued_at: "2026-09-07T09:35:00Z",
    created_at: "2026-09-07T09:34:00Z",
    ...sua,
  };
}

function veThe(tb: comms_thongBaoRa, dangChon = false): string {
  return renderToStaticMarkup(
    <TheThongBao thongBao={tb} dangChon={dangChon} chon={() => {}} />,
  );
}

describe("thẻ thông báo §3", () => {
  it("tiêu đề, trích nội dung, mốc phát hành và bộ đếm cùng ra một thẻ", () => {
    const html = veThe(thongBao());

    expect(html).toContain("Thông báo về việc triển khai hệ thống an ninh");
    expect(html).toContain("Đề nghị các bộ phận cử cán bộ");
    expect(html).toContain("16:35 07/09/2026");
    expect(html).toContain("2/12 đã xác nhận");
  });

  it("chip `Bắt buộc xác nhận` chỉ hiện khi bật cờ, và bộ đếm biến mất cùng nó", () => {
    expect(veThe(thongBao())).toContain(CHIP_BAT_BUOC_XAC_NHAN);

    const tat = veThe(thongBao({ ack_required: false }));
    expect(tat).not.toContain(CHIP_BAT_BUOC_XAC_NHAN);
    // `0/12 đã xác nhận` trên một thông báo không đòi xác nhận là nói với cả xã rằng mười hai
    // người đang nợ một việc không ai giao.
    expect(tat).not.toContain("đã xác nhận");
  });

  it("KHÔNG BAO GIỜ có chip thư hôm nay — `chua-gui` không có nhãn nào ở §3", () => {
    const html = veThe(thongBao());

    expect(html).not.toContain("Đang gửi thư");
    expect(html).not.toContain("Đã gửi thư");
    expect(html).not.toContain("Gửi thư lỗi");
  });

  it("thông báo `da-phat-hanh` KHÔNG mang chip trạng thái, `da-go` thì có", () => {
    // Một chip "Đã phát hành" trên mọi thẻ là một chip không nói gì.
    expect(veThe(thongBao())).not.toContain("Đã phát hành");
    expect(veThe(thongBao({ status: "da-go" }))).toContain("Đã gỡ");
  });

  it("thẻ ghim mang dấu ghim, thẻ thường thì không", () => {
    // ADR 0068 §2/§5: the `📌` glyph is a lucide `Pin` + the word `PINNED_LABEL` now — presentational
    // pin changed, the behaviour (marked iff `pinned`) is the same assertion.
    expect(veThe(thongBao({ pinned: true }))).toContain(PINNED_LABEL);
    expect(veThe(thongBao())).not.toContain(PINNED_LABEL);
  });

  it("thẻ đang chọn đánh dấu bằng `aria-current` — không mượn một lớp CSS của thứ khác", () => {
    expect(veThe(thongBao(), true)).toContain('aria-current="true"');
    expect(veThe(thongBao(), false)).not.toContain("aria-current");
  });
});

describe("danh sách thẻ §2", () => {
  it("sổ rỗng thì nói ra, không để trang trắng", () => {
    const html = renderToStaticMarkup(
      <DanhSachThongBao thongBao={[]} dangChon={null} chon={() => {}} />,
    );

    expect(html).toContain(SO_RONG);
  });

  it("nói thẳng rằng ghim chỉ nâng TRONG TRANG, không phải thứ tự cả sổ", () => {
    // `core/page` mang đúng một cột sắp xếp; thứ tự toàn sổ không diễn đạt được. Một cán bộ tin
    // rằng thẻ ghim luôn ở đầu quyển sổ sẽ đi tìm một thông báo ở chỗ nó không nằm.
    const html = renderToStaticMarkup(
      <DanhSachThongBao thongBao={[thongBao()]} dangChon={null} chon={() => {}} />,
    );

    expect(html).toContain(GHI_CHU_GHIM_TRONG_TRANG);
  });

  it("vẽ đúng thứ tự được truyền vào, không tự sắp xếp lại lần nữa", () => {
    const html = renderToStaticMarkup(
      <DanhSachThongBao
        thongBao={[
          thongBao({ id: "a", title: "Thông báo A" }),
          thongBao({ id: "b", title: "Thông báo B" }),
        ]}
        dangChon="b"
        chon={() => {}}
      />,
    );

    expect(html.indexOf("Thông báo A")).toBeLessThan(html.indexOf("Thông báo B"));
    // Đúng MỘT thẻ mang dấu đang chọn.
    expect(html.split('aria-current="true"').length - 1).toBe(1);
  });
});

describe("panel chi tiết §4", () => {
  it("chưa chọn gì thì nguyên văn câu của đặc tả", () => {
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={null} />);
    expect(html).toContain(CHUA_CHON_THONG_BAO);
  });

  it("hiện TOÀN VĂN nội dung, không phải bản trích của thẻ", () => {
    const dai = "Kính gửi các bộ phận. ".repeat(20);
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao({ body: dai })} />);

    expect(html).toContain(dai.trim());
    // Không có dấu ba chấm cắt bớt ở cột phải: đó là chỗ duy nhất đọc lại được toàn văn.
    expect(html).not.toContain("…");
  });

  it("người soạn là MÃ NGHIỆP VỤ, không phải id nội bộ và không phải họ tên", () => {
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao()} />);
    expect(html).toContain("CB-2026-7K3M9Q");
  });

  it("hai sự thật về thư đứng RIÊNG: đã yêu cầu, và đã xảy ra", () => {
    // Gộp chúng lại sẽ làm "chưa gửi được" không phân biệt được với "không ai yêu cầu gửi thư".
    const co = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao()} />);
    expect(co).toContain("Có yêu cầu gửi");
    expect(co).toContain("Chưa gửi");

    const khong = renderToStaticMarkup(
      <ChiTietThongBao thongBao={thongBao({ email_requested: false })} />,
    );
    expect(khong).toContain("Không yêu cầu gửi");
  });

  it("người nhận, bộ phận nhận, Gỡ và hai chip là CHỖ GIỮ có dấu '?', không phải danh sách rỗng", () => {
    const html = renderToStaticMarkup(<ChiTietThongBao thongBao={thongBao()} />);

    // `(12)` is the server's `recipient_count` — the heading is a true figure, only the list is missing.
    expect(html).toContain("Người nhận (12)");
    expect(html).toContain("Bộ phận nhận");
    for (const id of ["recipients", "byUnit", "withdraw", "acknowledge", "emailStatus"]) {
      expect(html, id).toContain(nhuTrongHTML(pendingMarkerLabel(pendingPart(id).ten)));
    }
    // The ONLY live buttons are the "?" markers; the Gỡ control itself is a disabled native button.
    const live = [...html.matchAll(/<button(?![^>]*disabled="")[^>]*>/g)].map((m) => m[0]);
    expect(live.length).toBeGreaterThan(0);
    for (const tag of live) expect(tag).toContain("data-pending-marker");
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*>(?:(?!<\/button>).)*Gỡ<\/button>/s);
  });

  it("chip `Xác nhận đã đọc` chỉ khi bắt buộc xác nhận; chip thư chỉ khi có yêu cầu gửi thư", () => {
    const tat = renderToStaticMarkup(
      <ChiTietThongBao thongBao={thongBao({ ack_required: false, email_requested: false })} />,
    );
    expect(tat).not.toContain(nhuTrongHTML(pendingMarkerLabel(pendingPart("acknowledge").ten)));
    expect(tat).not.toContain(nhuTrongHTML(pendingMarkerLabel(pendingPart("emailStatus").ten)));
  });
});

describe("biểu mẫu Soạn thông báo §5", () => {
  function veForm(
    loi: string | null = null,
    danhBa: KetQua<identity_danhBaChonNguoiRa> | null = { ok: true, duLieu: { items: [] } },
  ): string {
    return renderToStaticMarkup(
      <FormSoanThongBao
        dangGui={false}
        loi={loi}
        danhBa={danhBa}
        huy={() => {}}
        phatHanh={() => {}}
      />,
    );
  }

  it("có đủ các ô dựng được", () => {
    const html = veForm();

    expect(html).toContain("Tiêu đề *");
    expect(html).toContain("Nội dung *");
    expect(html).toContain("Gửi thêm đích danh *");
    expect(html).toContain("Ghim lên đầu danh sách");
    expect(html).toContain("Bắt buộc xác nhận đã đọc");
    expect(html).toContain("Gửi thư điện tử cho người nhận");
    expect(html).toContain(nhuTrongHTML(NHAN_NUT_PHAT_HANH));
  });

  it("ô chọn bộ phận chỉ là CHỖ GIỮ — một chip vô hiệu, không tên bộ phận nào, máy chủ trả 501", () => {
    const html = veForm();

    // Tên bộ phận là dữ liệu của xã: không một tên nào được gõ sẵn vào màn.
    expect(html).not.toContain("THƯỜNG TRỰC ĐẢNG UỶ");
    expect(html).not.toContain("org_unit");
    expect(html).toContain("Bộ phận nhận thông báo");
    expect(html).toMatch(new RegExp(`<button[^>]*disabled=""[^>]*>${RECIPIENT_UNITS_CHIP}</button>`));
    expect(html).toContain(nhuTrongHTML(pendingMarkerLabel(pendingPart("byUnit").ten)));
  });

  it("`Lưu nháp` là CHỖ GIỮ vô hiệu giữa Huỷ và Phát hành — không bao giờ là nút gửi", () => {
    // Một nút "lưu để sửa tiếp" mà phát hành luôn là một nút gửi đi cả xã.
    const html = veForm();
    const draft = html.match(new RegExp(`<button[^>]*>(?:(?!</button>).)*${SAVE_DRAFT_LABEL}</button>`, "s"))?.[0] ?? "";
    expect(draft).toContain('type="button"');
    expect(draft).toContain('disabled=""');
    expect(html.indexOf("Huỷ")).toBeLessThan(html.indexOf(SAVE_DRAFT_LABEL));
    expect(html.indexOf(SAVE_DRAFT_LABEL)).toBeLessThan(html.indexOf('type="submit"'));
  });

  it("nút Phát hành TẮT khi chưa gõ gì — ba trường, không phải hai", () => {
    // Hợp đồng chỉ đánh dấu `title` và `body` bắt buộc, nhưng máy chủ còn từ chối một thông báo
    // không có người nhận nào.
    // Read the ATTRIBUTE on the submit tag: since ADR 0068 every button carries the Tailwind class
    // `disabled:…`, so a bare `toContain("disabled")` would be green on any form.
    expect(veForm()).toMatch(/<button type="submit"[^>]*\sdisabled=""/);
  });

  it("ô tick gửi thư mặc định BẬT, kèm câu nói rõ chưa có thư nào đi", () => {
    const html = veForm();

    expect(html).toContain(nhuTrongHTML(CANH_BAO_CHUA_GUI_THU));
    // `checked` phải nằm trên đúng ô ấy, không phải trên một ô tick nào khác của biểu mẫu.
    expect(html).toContain('id="gui-thu-dien-tu"');
    expect(html).toMatch(/id="gui-thu-dien-tu"[^>]*checked/);
    expect(html).not.toMatch(/id="ghim-thong-bao"[^>]*checked/);
    expect(html).not.toMatch(/id="bat-buoc-xac-nhan"[^>]*checked/);
  });

  it("nói rõ ô người nhận nhận MÃ, không phải họ tên", () => {
    // Một ô text để cán bộ tự gõ sẽ được điền bằng họ tên nếu không ai nói khác, và giá trị ấy đi
    // thẳng vào cột mã của sổ lưu trữ.
    const html = veForm();

    expect(html).toContain("CB-2026-7K3M9Q");
    expect(html).toContain("không phải họ tên");
  });

  it("câu từ chối 501 của máy chủ hiện NGUYÊN VĂN", () => {
    const html = veForm(
      "Chưa gửi được thông báo theo bộ phận: hệ thống chưa lấy được danh sách cán bộ của một " +
        "bộ phận. Hãy chọn từng người ở mục \"Gửi thêm đích danh\".",
    );

    expect(html).toContain("Chưa gửi được thông báo theo bộ phận");
    expect(html).toContain(nhuTrongHTML('mục "Gửi thêm đích danh"'));
    expect(html).toContain('role="alert"');
  });
});

describe("phần chưa dựng — mô tả sau dấu '?' (ADR 0068 §14)", () => {
  it("không còn mục đã dựng: chuông thông báo, cổng quyền phía giao diện", () => {
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten).join(" | ");
    expect(ten).not.toMatch(/chuông/i);
    expect(ten).not.toContain("announcement.create");
  });

  it("mỗi mục có `id` riêng; `pendingPart` từ chối một `id` lạ thay vì mở mô tả rỗng", () => {
    const ids = PHAN_CHUA_DUNG.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(() => pendingPart("khong-co")).toThrow();
  });

  it("vì sao không có `Gửi cho tôi`: thiếu quyền đọc thông báo — không còn dẫn câu hỏi mở đã chốt (TB-05)", () => {
    const viSao = pendingPart("scopeMine").viSao;
    expect(viSao).toContain("quyền đọc thông báo");
    expect(viSao).not.toMatch(/#27|câu hỏi mở/);
  });

  it("chip xác nhận là HÀNH ĐỘNG `Xác nhận đã đọc`, không phải trạng thái `Đã xác nhận` (TB-03)", () => {
    expect(ACKNOWLEDGED_CHIP).toBe("Xác nhận đã đọc");
  });
});

describe("ô chọn người nhận — danh bạ chọn người, không phải sổ quản trị", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("đọc `GET /api/v1/staff-directory`, KHÔNG phải `/api/v1/staff`, và không mang `permission`", async () => {
    // `/api/v1/staff` đòi `admin.user` và trả số di động cá nhân. Người soạn thông báo không cầm
    // khoá ấy, và ô chọn không cần số ấy.
    const gia = vi.fn(
      async (_duongDan: string, _tuyChon?: RequestInit) =>
        new Response(JSON.stringify({ items: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", gia);

    await docDanhBaNguoiNhan();

    expect(gia).toHaveBeenCalledTimes(1);
    const duong = gia.mock.calls[0]?.[0];
    expect(duong).toBe("/api/v1/staff-directory");
    expect(duong).not.toContain("permission");
  });

  it("màn Thông báo không nhập client của sổ quản trị `lib/api/can-bo`", () => {
    // Canh ở mức nguồn: một lời gọi `layDanhSachCanBo` thêm vào sau này sẽ không đi qua
    // `docDanhBaNguoiNhan`, nên ca trên vẫn xanh trong khi màn đã quay lại đòi `admin.user`.
    const nguon = readFileSync(new URL("./so-thong-bao.tsx", import.meta.url), "utf8");
    expect(nguon).not.toMatch(/@\/lib\/api\/can-bo["']/);
    expect(nguon).toContain("@/lib/api/danh-ba-chon-nguoi");
  });

  const DANH_BA: KetQua<identity_danhBaChonNguoiRa> = {
    ok: true,
    duLieu: {
      items: [
        { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "01JBP" },
        { code: "CB-00124", full_name: "Lê Văn C", position: "", department_id: "" },
      ],
    },
  };

  function veForm(danhBa: KetQua<identity_danhBaChonNguoiRa> | null): string {
    return renderToStaticMarkup(
      <FormSoanThongBao dangGui={false} loi={null} danhBa={danhBa} huy={() => {}} phatHanh={() => {}} />,
    );
  }

  it("mỗi dòng mang họ tên, chức vụ và MÃ — giá trị gửi đi là mã nghiệp vụ", () => {
    const html = veForm(DANH_BA);
    expect(html).toContain(NHAN_CHON_NGUOI_NHAN);
    expect(html).toContain('value="CB-00123"');
    expect(html).toContain("Trần Thị B · Công chức — CB-00123");
    expect(html).toContain("Lê Văn C — CB-00124");
  });

  it("đang tải: nói ra, không vẽ ô chọn rỗng", () => {
    const html = veForm(null);
    expect(html).toContain(DANG_TAI_DANH_BA);
    expect(html).not.toContain('id="chon-nguoi-nhan-thong-bao"');
    // Ô gõ mã vẫn có: biểu mẫu không phụ thuộc danh bạ.
    expect(html).toContain('id="nguoi-nhan-thong-bao"');
  });

  it("tải hỏng: câu của máy chủ nguyên văn, và ô gõ mã vẫn dùng được", () => {
    const html = veForm({ ok: false, thongBao: "Phiên làm việc đã hết hạn." });
    expect(html).toContain("Phiên làm việc đã hết hạn.");
    expect(html).toContain('role="alert"');
    expect(html).toContain('id="nguoi-nhan-thong-bao"');
  });

  it("danh bạ rỗng: một câu, không phải ô chọn chỉ có mục trống", () => {
    const html = veForm({ ok: true, duLieu: { items: [] } });
    expect(html).toContain(nhuTrongHTML(DANH_BA_NGUOI_NHAN_RONG));
    expect(html).not.toContain('id="chon-nguoi-nhan-thong-bao"');
  });
});
