import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { banTuCanBo, thanSua } from "@/components/danh-ba/nhan-ghi-danh-ba";

import { BangLienHe } from "./bang-lien-he";
import { StaffContactForm } from "./staff-contact-form";
import {
  CHIP_CHUA_HIEN,
  CHIP_DANG_HIEN,
  COT_MINI_APP,
  NUT_RUT_MINI_APP,
  NUT_THEM_MINI_APP,
} from "./cong-khai";
import { NHAN_XOA_DONG } from "./xoa-dong";
import { CAU_THIEU_QUYEN, NUT_SUA_THONG_TIN, SELECT_PAGE_LABEL, selectRowLabel } from "./nhan-danh-ba";

/**
 * KIỂM CÁI RA TỚI TRANG, KHÔNG CHỈ KIỂM QUYẾT ĐỊNH.
 *
 * `vitest.config.mts` ghi lại vì sao cần cả hai loại: một quyết định đúng nằm trong module thuần
 * mà không component nào đưa ra trang là một quyết định không tồn tại với người dùng. Ở đây thứ
 * phải ra tới trang gồm hai nhóm — số liên hệ của cán bộ (ca được phép) và KHÔNG GÌ CẢ (ca bị từ
 * chối vì thiếu `admin.user`).
 *
 * SỐ ĐIỆN THOẠI TRONG TỆP NÀY LÀ SỐ GIẢ thuộc dải đã thống nhất `0900000xxx` (luật 3, bất biến 5),
 * và tên người cũng là tên giả — đúng bộ dữ liệu `docs/ui-ux/12-danh-ba-can-bo.md §10` đã thay.
 */

function canBo(ghiDe: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Bí thư Đảng ủy",
    department_id: "01J0000000000000000000BP01",
    role_id: "",
    phone: "02350000001",
    mobile: "0900000001",
    has_account: true,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...ghiDe,
  };
}

const TRA_XONG = {
  pha: "xong",
  ten: new Map([["01J0000000000000000000BP01", "THƯỜNG TRỰC ĐẢNG UỶ"]]),
} as const;

function dung(danhSach: readonly identity_canBoTomTat[]): string {
  return renderToStaticMarkup(
    <BangLienHe danhSach={danhSach} traBoPhan={TRA_XONG} onSua={() => undefined} />,
  );
}

/** Như `dung`, nhưng phiên CÓ `content.update` — hai nút Mini App được vẽ. */
function dungCK(danhSach: readonly identity_canBoTomTat[]): string {
  return renderToStaticMarkup(
    <BangLienHe
      danhSach={danhSach}
      traBoPhan={TRA_XONG}
      onSua={() => undefined}
      congKhai={{ onThem: () => undefined, onRut: () => undefined }}
    />,
  );
}

describe("bảng danh bạ — cái ra tới trang", () => {
  it("dựng đủ bốn cột dữ liệu của một dòng", () => {
    const html = dung([canBo()]);

    expect(html).toContain("Nguyễn Văn A");
    expect(html).toContain("nva@demo.invalid");
    expect(html).toContain("Bí thư Đảng ủy");
    expect(html).toContain("THƯỜNG TRỰC ĐẢNG UỶ");
    expect(html).toContain("0900000001");
  });

  it("số di động cá nhân hiện ĐẦY ĐỦ ở màn nội bộ — câu mở #11", () => {
    // #11 chốt 22/09/2026: KHÔNG che trong nội bộ xã. Cán bộ cùng xã cần gọi nhau để làm việc; che
    // thì họ truyền số qua kênh riêng và hệ thống mất cả vết lẫn quyền kiểm soát. Quyết định ấy
    // CHỈ nói về màn hình nội bộ — bản xuất Excel và mọi đường ra ngoài cơ quan vẫn che.
    const html = dung([canBo()]);

    expect(html).toContain("0900000001");
    // Nếu có ngày ai đó "che cho an toàn", ca này đỏ và buộc người sửa đọc lại #11 trước.
    expect(html).not.toContain("****");
    expect(html).not.toContain("•••");
  });

  it("cột `Máy bàn cơ quan` KHÔNG vẽ (bảng lỗi dòng 26) — và số máy bàn không bị gộp vào cột di động (#16)", () => {
    // Chỉ là hiển thị: trường vẫn ở dữ liệu và API (câu mở #16 giữ trường). Máy bàn là thông tin công
    // vụ, di động là dữ liệu cá nhân theo Nghị định 13 — ẩn cột không được biến thành gộp hai số
    // dưới một cái tên.
    const html = dung([canBo()]);

    expect(html).not.toContain("Máy bàn cơ quan");
    expect(html).not.toContain("02350000001");
    expect(html).toContain("Di động cá nhân");
  });

  it("không truyền `selection` (không `content.update`) → KHÔNG có cột chọn (ca bị từ chối)", () => {
    const html = dungCK([canBo(), canBo({ id: "b", full_name: "Trần Thị B", published: true })]);
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toContain("Khoá tài khoản");
  });

  it("có `selection` → cột chọn w-10: ô đầu bảng chọn cả trang, mỗi dòng một ô gọi tên người", () => {
    const html = renderToStaticMarkup(
      <BangLienHe
        danhSach={[canBo(), canBo({ id: "b", full_name: "Trần Thị B" })]}
        traBoPhan={TRA_XONG}
        onSua={() => undefined}
        selection={{ selectedIds: new Set(["b"]), onToggle: () => undefined, onTogglePage: () => undefined }}
      />,
    );
    expect(html).toContain('<th scope="col" class="w-10">');
    expect(html).toContain(`aria-label="${SELECT_PAGE_LABEL}"`);
    expect(html).toMatch(new RegExp(`<input type="checkbox"[^>]*aria-label="${selectRowLabel("Nguyễn Văn A")}"`));
    const b = new RegExp(`<input[^>]*aria-label="${selectRowLabel("Trần Thị B")}"[^>]*>`).exec(html)?.[0] ?? "";
    expect(b).toContain('checked=""');
    // Not every row ticked → the page box is not ticked.
    expect(new RegExp(`<input[^>]*aria-label="${SELECT_PAGE_LABEL}"[^>]*>`).exec(html)?.[0]).not.toContain("checked");
  });

  it("ô chọn chỉ gọi handler của màn — chọn KHÔNG công khai ai (#12)", () => {
    const calls: string[] = [];
    type El = { type: unknown; props: Record<string, unknown> };
    const boxes: El[] = [];
    const walk = (n: unknown) => {
      if (Array.isArray(n)) return n.forEach(walk);
      if (typeof n !== "object" || n === null || !("props" in n)) return;
      const el = n as El;
      if (el.props.type === "checkbox") boxes.push(el);
      walk(el.props.children);
    };
    walk(
      BangLienHe({
        danhSach: [canBo()],
        traBoPhan: TRA_XONG,
        onSua: () => undefined,
        congKhai: { onThem: () => calls.push("publish"), onRut: () => calls.push("withdraw") },
        selection: {
          selectedIds: new Set(),
          onToggle: (cb, on) => calls.push(`row:${cb.id}:${on}`),
          onTogglePage: (on) => calls.push(`page:${on}`),
        },
      }),
    );
    expect(boxes).toHaveLength(2);
    (boxes[0]!.props.onChange as (e: unknown) => void)({ target: { checked: true } });
    (boxes[1]!.props.onChange as (e: unknown) => void)({ target: { checked: true } });
    expect(calls).toEqual(["page:true", `row:${canBo().id}:true`]);
  });

  it("không còn cột Ảnh đại diện (bản mẫu không có) và không còn thẻ dưới 768px — một bảng ở mọi bề rộng", () => {
    const html = dung([canBo()]);
    expect(html).not.toContain("Ảnh đại diện");
    expect(html).not.toContain("data-pending");
    expect(html).not.toContain("<ul");
  });

  it("tên là nút mở hộp sửa, thư điện tử nằm dưới tên", () => {
    const calls: string[] = [];
    type El = { type: unknown; props: Record<string, unknown> };
    const found: El[] = [];
    const walk = (n: unknown) => {
      if (Array.isArray(n)) return n.forEach(walk);
      if (typeof n !== "object" || n === null || !("props" in n)) return;
      const el = n as El;
      if (el.type === "button" && el.props.children === "Nguyễn Văn A") found.push(el);
      walk(el.props.children);
    };
    walk(BangLienHe({ danhSach: [canBo()], traBoPhan: TRA_XONG, onSua: (cb) => calls.push(cb.id) }));
    expect(found).toHaveLength(1);
    (found[0]!.props.onClick as () => void)();
    expect(calls).toEqual([canBo().id]);
    expect(dung([canBo()])).toMatch(/Nguyễn Văn A<\/button><div class="text-ink-muted text-\[11px\] font-normal">nva@demo.invalid<\/div>/);
  });

  it("hộp sửa không vẽ ô Máy bàn cơ quan (dòng 26), nhưng số máy bàn đang lưu vẫn đi nguyên lên máy chủ", () => {
    const goc = canBo();
    const draft = banTuCanBo(goc);
    const html = renderToStaticMarkup(
      <StaffContactForm
        editing={goc}
        draft={draft}
        setDraft={() => undefined}
        showOnMiniApp={false}
        setShowOnMiniApp={() => undefined}
        canPublish={false}
        units={[]}
        errors={{}}
        serverError=""
        sending={false}
        onSubmit={() => undefined}
        onCancel={() => undefined}
      />,
    );
    expect(html).not.toContain("Máy bàn cơ quan");
    expect(html).not.toContain('id="staff-office-phone"');
    expect(html).toContain('id="staff-mobile"');
    // Display only: the stored landline is sent back unchanged — hiding the box never blanks the field.
    expect(thanSua(draft, goc).office_phone).toBe("02350000001");
  });

  it("khối / đơn vị là chữ thường, không huy hiệu", () => {
    expect(dung([canBo()])).toContain('<td class="text-ink-muted text-[12px]">THƯỜNG TRỰC ĐẢNG UỶ</td>');
  });

  it("nút sửa mang TÊN NGƯỜI trong aria-label, không chỉ một nhãn chung", () => {
    // Hai mươi dòng cho ra hai mươi nút đọc lên giống hệt nhau là danh sách mà người dùng trình
    // đọc màn hình không chọn đúng được dòng nào — và chọn nhầm dòng ở đây là sửa hồ sơ của một
    // cán bộ khác. Chuỗi `15-phu-luc §8` yêu cầu giữ vẫn nằm nguyên trong nhãn.
    const html = dung([canBo(), canBo({ id: "b", full_name: "Trần Thị B" })]);

    expect(html).toContain(`aria-label="${NUT_SUA_THONG_TIN}: Nguyễn Văn A"`);
    expect(html).toContain(`aria-label="${NUT_SUA_THONG_TIN}: Trần Thị B"`);
  });

  it("bộ phận không tra được thì NÓI RA, không để ô trống", () => {
    // Một ô trống trông y hệt "chưa phân bộ phận", mà hai thứ ấy cần hai hành động khác nhau: một
    // là việc phân công nhân sự, một là một dòng dữ liệu lệch chỉ người quản trị sửa được.
    const html = dung([canBo({ department_id: "01J0000000000000000000XXXX" })]);

    expect(html).toContain("Không tra được trong danh mục");
  });

  it("chưa phân bộ phận là một câu riêng, không lẫn với ca trên", () => {
    expect(dung([canBo({ department_id: "" })])).toContain("Chưa phân bộ phận");
  });
});

describe("cột Trên Mini App, dòng phụ Có Zalo, và hai nút theo dòng", () => {
  it("cột 'Trên Mini App' với hai chip, nguyên văn đặc tả §4", () => {
    const html = dung([canBo(), canBo({ id: "b", full_name: "Trần Thị B", published: true })]);
    expect(html).toContain(`<th scope="col" class="w-36 text-center">${COT_MINI_APP}</th>`);
    expect(html).toContain(CHIP_CHUA_HIEN);
    expect(html).toContain(CHIP_DANG_HIEN);
  });

  it("người đang hiện KHÔNG còn dòng 'Đồng ý ghi lúc …' (bản mẫu không có); dòng nền bg-leaf/4", () => {
    const html = dung([canBo({ published: true, consent_recorded_at: "2026-09-24T07:05:00Z" }), canBo({ id: "b" })]);
    expect(html).not.toContain("Đồng ý ghi lúc");
    expect(html.match(/<tr class="bg-leaf\/4">/g)).toHaveLength(1);
  });

  it("'Có Zalo' là dòng phụ dưới số di động, chỉ khi `has_zalo`", () => {
    expect(dung([canBo({ has_zalo: true })])).toMatch(
      /0900000001<div class="text-brand text-\[11px\]">Có Zalo<\/div>/,
    );
    expect(dung([canBo()])).not.toContain("Có Zalo");
  });

  it("KHÔNG có `content.update` → không một nút Mini App nào (ca bị từ chối)", () => {
    const html = dung([canBo(), canBo({ id: "b", published: true })]);
    expect(html).not.toContain(NUT_THEM_MINI_APP);
    expect(html).not.toContain(NUT_RUT_MINI_APP);
    // Chip và cột vẫn hiện: xem trạng thái không cần khoá ghi.
    expect(html).toContain(CHIP_DANG_HIEN);
  });

  it("CÓ `content.update` → mỗi dòng ĐÚNG MỘT nút, theo trạng thái của chính dòng ấy", () => {
    const html = dungCK([
      canBo(),
      canBo({ id: "b", full_name: "Trần Thị B", published: true }),
    ]);
    expect(html).toContain(`aria-label="${NUT_THEM_MINI_APP}: Nguyễn Văn A"`);
    expect(html).toContain(`aria-label="${NUT_RUT_MINI_APP}: Trần Thị B"`);
    expect(html).not.toContain(`aria-label="${NUT_RUT_MINI_APP}: Nguyễn Văn A"`);
    expect(html).not.toContain(`aria-label="${NUT_THEM_MINI_APP}: Trần Thị B"`);
  });
});

describe("nút 🗑 xoá dòng nhập trùng", () => {
  function dungXoa(danhSach: readonly identity_canBoTomTat[]): string {
    return renderToStaticMarkup(
      <BangLienHe
        danhSach={danhSach}
        traBoPhan={TRA_XONG}
        onSua={() => undefined}
        onXoa={() => undefined}
      />,
    );
  }

  it("KHÔNG có `admin.user.delete` (không truyền `onXoa`) → không một nút 🗑 nào (ca bị từ chối)", () => {
    // The trash icon carries no text of its own: the label below IS the button (ADR 0068).
    const html = dung([canBo(), canBo({ id: "b", full_name: "Trần Thị B" })]);
    expect(html).not.toContain(NHAN_XOA_DONG);
  });

  it("CÓ khoá → mỗi dòng một nút 🗑, nhãn trợ năng 'Xoá khỏi danh bạ: <tên>'", () => {
    const html = dungXoa([canBo(), canBo({ id: "b", full_name: "Trần Thị B" })]);
    expect(html).toContain(`aria-label="${NHAN_XOA_DONG}: Nguyễn Văn A"`);
    expect(html).toContain(`aria-label="${NHAN_XOA_DONG}: Trần Thị B"`);
  });

  it("dòng CÓ tài khoản VẪN có nút — hộp mở ra để nói vì sao không xoá được", () => {
    const html = dungXoa([canBo({ has_account: true })]);
    expect(html).toContain(`aria-label="${NHAN_XOA_DONG}: Nguyễn Văn A"`);
    const nut = /<button[^>]*aria-label="Xoá khỏi danh bạ: Nguyễn Văn A"[^>]*>/.exec(html)?.[0] ?? "";
    expect(nut).not.toBe("");
    expect(nut).not.toMatch(/\sdisabled=""/);
  });
});

/**
 * THE ROW BUTTONS — the prototype's three outline icon buttons (06/10/2026, ADR 0068 lần 5): Mini App
 * (add OR withdraw), edit, delete, each present only with its key. The name button carries no
 * `aria-label` (its text is its name), so the label list below is the icon buttons alone.
 */
describe("nút trên dòng của bảng — theo đúng khoá của phiên", () => {
  const tableOf = (html: string) => html.slice(html.indexOf("<table"), html.indexOf("</table>"));
  const labels = (html: string) =>
    [...tableOf(html).matchAll(/<button[^>]*aria-label="([^"]*)"/g)].map((m) => m[1]).filter((l) => !l?.includes("tính năng đang phát triển"));

  it("không `content.update`, không `admin.user.delete` → chỉ nút sửa (ca bị từ chối)", () => {
    expect(labels(dung([canBo()]))).toEqual([`${NUT_SUA_THONG_TIN}: Nguyễn Văn A`]);
  });

  it("đủ khoá → Mini App theo trạng thái dòng, rồi sửa, rồi xoá — đúng thứ tự bản mẫu", () => {
    const html = renderToStaticMarkup(
      <BangLienHe
        danhSach={[canBo(), canBo({ id: "b", full_name: "Trần Thị B", published: true })]}
        traBoPhan={TRA_XONG}
        onSua={() => undefined}
        congKhai={{ onThem: () => undefined, onRut: () => undefined }}
        onXoa={() => undefined}
      />,
    );
    expect(labels(html)).toEqual([
      `${NUT_THEM_MINI_APP}: Nguyễn Văn A`,
      `${NUT_SUA_THONG_TIN}: Nguyễn Văn A`,
      `${NHAN_XOA_DONG}: Nguyễn Văn A`,
      `${NUT_RUT_MINI_APP}: Trần Thị B`,
      `${NUT_SUA_THONG_TIN}: Trần Thị B`,
      `${NHAN_XOA_DONG}: Trần Thị B`,
    ]);
    // Every icon-only button carries its words as the tooltip too.
    expect(tableOf(html)).toContain(`title="${NUT_THEM_MINI_APP}: Nguyễn Văn A"`);
  });

  it("the publish button only OPENS the consent dialog — it calls the handler, never a route", () => {
    // `BangLienHe` has no hooks: walk the unrendered tree for the desktop publish button and press it.
    const calls: string[] = [];
    type El = { type: unknown; props: Record<string, unknown> };
    const found: El[] = [];
    const walk = (n: unknown) => {
      if (Array.isArray(n)) return n.forEach(walk);
      if (typeof n !== "object" || n === null || !("props" in n)) return;
      const el = n as El;
      if (el.props.label === `${NUT_THEM_MINI_APP}: Nguyễn Văn A`) found.push(el);
      walk(el.props.children);
    };
    walk(
      BangLienHe({
        danhSach: [canBo()],
        traBoPhan: TRA_XONG,
        onSua: () => undefined,
        congKhai: { onThem: (cb) => calls.push(`them:${cb.id}`), onRut: () => undefined },
      }),
    );
    expect(found).toHaveLength(1);
    (found[0]!.props.onClick as () => void)();
    expect(calls).toEqual([`them:${canBo().id}`]);
  });
});

describe("bảng danh bạ — CA BỊ TỪ CHỐI vì thiếu `admin.user`", () => {
  /**
   * Nhánh này không ai nhìn thấy trong lúc dựng: tài khoản người viết luôn có đủ quyền. Nó là
   * nhánh sẽ chạy trên máy của một cán bộ chuyên môn, và là nhánh phải đúng — vì thứ nó giữ lại
   * là số di động cá nhân của toàn bộ cán bộ trong xã.
   */
  function dungCong(quyetDinh: Parameters<typeof KhungQuyen>[0]["quyetDinh"]): string {
    return renderToStaticMarkup(
      <KhungQuyen quyetDinh={quyetDinh} cauThieuQuyen={CAU_THIEU_QUYEN}>
        <BangLienHe danhSach={[canBo()]} traBoPhan={TRA_XONG} onSua={() => undefined} />
      </KhungQuyen>,
    );
  }

  it("thiếu quyền: KHÔNG một dòng danh bạ nào ra tới trang", () => {
    const html = dungCong({ hien: false, vi: "khong-du-quyen" });

    expect(html).not.toContain("Nguyễn Văn A");
    expect(html).not.toContain("0900000001");
    expect(html).not.toContain("nva@demo.invalid");
    expect(html).toContain(CAU_THIEU_QUYEN);
  });

  it("không đọc được quyền: cũng không dựng gì — 'chưa rõ' hành xử như 'không có'", () => {
    // Fail closed. Trên đường cách ly không có giá trị mặc định nào (luật 1, cấm #1).
    const html = dungCong({
      hien: false,
      vi: "khong-doc-duoc",
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });

    expect(html).not.toContain("0900000001");
    expect(html).toContain("Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.");
  });

  it("chưa đọc xong quyền: chưa dựng, và chưa nói là thiếu quyền", () => {
    const html = dungCong(null);

    expect(html).not.toContain("0900000001");
    expect(html).not.toContain(CAU_THIEU_QUYEN);
  });

  it("đủ quyền: danh bạ ra tới trang", () => {
    expect(dungCong({ hien: true })).toContain("0900000001");
  });
});
