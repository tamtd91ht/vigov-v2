import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type {
  identity_capQuyenRa,
  identity_nhomQuyenRa,
  identity_vaiTroCotRa,
} from "@/lib/api/schema.gen";

import { BangMaTran, type ChinhSuaMaTran } from "./ma-tran-phan-quyen";
import { dungBangDaCap } from "./ma-tran-quyen";
import { LY_DO_KHONG_TU_SUA } from "./nhan-ma-tran";
import { banSuaMoi, batDauLuu, batTatO, bangHienThi, ketThucLuu, type BanSua } from "./sua-phan-quyen";

/**
 * CA KẾT XUẤT CHO MA TRẬN PHÂN QUYỀN — thứ `ma-tran-quyen.test.ts` KHÔNG thấy được.
 *
 * Tệp kia khẳng định module nặn dữ liệu đúng: nhóm nào, khoá nào, ô nào đã cấp. Nó không thể thấy
 * phần JSX có DÙNG dữ liệu ấy hay không. Đo ngày 22/09/2026: thay `n.permissions.map` bằng một
 * mảng khoá viết cứng thì mọi ca kiểm đang có VẪN XANH — không tệp test nào chạm component.
 *
 * Đó đúng hình dạng "phép kiểm xanh sai lý do": một bảng phân quyền in ra danh sách của lập
 * trình viên thay vì danh sách của xã, và không có gì đỏ. Trên màn hình phân quyền của một cơ
 * quan nhà nước, hai danh sách ấy khác nhau nghĩa là một quyền thật không hiện ra để ai đó gỡ,
 * hoặc một quyền không tồn tại hiện ra để ai đó tick.
 *
 * KẾT XUẤT `BangMaTran` TRỰC TIẾP, không qua `MaTranPhanQuyen`: component cha đọc API trong
 * `useEffect`, mà `renderToStaticMarkup` không chạy effect — kết xuất cha chỉ cho ra trạng thái
 * đang tải, tức một ca xanh mà không chạm một hàng nào.
 */

// VẬT MẪU ĐẦY ĐỦ THEO HỢP ĐỒNG, không phải một object rút gọn cho vừa ca kiểm. `tsc` đã bắt bản
// rút gọn đầu tiên của tôi, và đó là hợp đồng làm đúng việc: một vật mẫu thiếu trường là một ca
// kiểm chạy trên hình dạng dữ liệu máy chủ không bao giờ gửi.
const VAI_TRO: readonly identity_vaiTroCotRa[] = [
  {
    id: "vt-1",
    code: "chuyen-vien",
    name: "Chuyên viên",
    is_leader: false,
    staff_count: 3,
    active_account_count: 2,
  },
  {
    id: "vt-2",
    code: "lanh-dao",
    name: "Lãnh đạo",
    is_leader: true,
    staff_count: 1,
    active_account_count: 1,
  },
];

/** Hai bộ dữ liệu KHÁC HẲN NHAU, để "màn hiện đúng thứ được đưa vào" là một khẳng định thật. */
const NHOM_A: readonly identity_nhomQuyenRa[] = [
  {
    name: "Quản trị",
    permissions: [
      { code: "admin.user", label: "Quản lý tài khoản" },
      { code: "admin.role", label: "Phân quyền" },
    ],
  },
];

const NHOM_B: readonly identity_nhomQuyenRa[] = [
  {
    name: "Phản ánh",
    permissions: [
      { code: "feedback.classify", label: "Phân loại phản ánh" },
      { code: "feedback.unmask", label: "Xem đầy đủ người gửi" },
    ],
  },
];

function ve(nhom: readonly identity_nhomQuyenRa[], daCap: readonly identity_capQuyenRa[] = []) {
  return renderToStaticMarkup(
    <BangMaTran nhom={nhom} vaiTro={VAI_TRO} daCap={dungBangDaCap(daCap)} />,
  );
}

describe("BangMaTran kết xuất từ dữ liệu, không từ hằng trong mã", () => {
  it("hiện đúng khoá của phản hồi thứ nhất và KHÔNG hiện khoá của phản hồi thứ hai", () => {
    const html = ve(NHOM_A);

    expect(html).toContain("admin.user");
    expect(html).toContain("admin.role");
    expect(html).toContain("Quản trị");

    // VẾ PHỦ ĐỊNH LÀ VẾ CHỊU LỰC. Một danh sách viết cứng chứa CẢ HAI bộ sẽ qua được vế khẳng
    // định ở trên; chỉ vế này bắt được nó.
    expect(html).not.toContain("feedback.classify");
    expect(html).not.toContain("feedback.unmask");
  });

  it("đưa vào bộ khác thì hiện bộ khác — cùng một component, không sửa gì", () => {
    const html = ve(NHOM_B);

    expect(html).toContain("feedback.classify");
    expect(html).toContain("feedback.unmask");
    expect(html).toContain("Phản ánh");
    expect(html).not.toContain("admin.user");
  });

  it("khoá hiện NGUYÊN CHUỖI PHẲNG, không bị tách rồi ghép lại", () => {
    // Luật 5 bất biến 3b: một quyền là MỘT khoá `<nhóm>.<việc>`, đúng chuỗi bảng `quyen` lưu và
    // đúng chuỗi máy chủ kiểm. Tách thành (nhóm, việc) rồi ghép lại trên màn hình là mở đường cho
    // một màn hình sau suy ra `feedback.assign` từ `feedback` + `assign` — một khoá không ai gieo.
    const html = ve(NHOM_B);

    expect(html).toContain(">feedback.classify<");
    // Không có mảnh nào đứng một mình: nếu chuỗi bị tách, `classify` sẽ xuất hiện trong một thẻ
    // riêng và dấu hiệu là một thẻ đóng-mở ngay trước nó.
    expect(html).not.toContain(">classify<");
    expect(html).not.toContain(">feedback<");
  });

  it("bảng rỗng vẫn kết xuất được, và không bịa ra hàng nào", () => {
    // Một xã chưa cấu hình quyền nào là trạng thái THẬT (bảng `quyen` gieo theo xã), không phải
    // lỗi. Màn hình phải chịu được nó mà không vẽ hàng mẫu.
    const html = ve([]);

    expect(html).not.toContain("admin.user");
    expect(html).not.toContain("feedback.classify");
    // Đầu cột vai trò vẫn còn — cột là của xã, hàng là của bộ khoá.
    expect(html).toContain("Chuyên viên");
  });

  it("ô đã cấp và ô chưa cấp kết xuất khác nhau", () => {
    const chuaCap = ve(NHOM_A);
    const daCap = ve(NHOM_A, [{ role_id: "vt-1", permission: "admin.user" }]);

    // Không khẳng định ký tự cụ thể — đó là việc của `nhan-ma-tran.ts`. Khẳng định thứ duy nhất
    // quan trọng ở tầng này: trạng thái đã cấp CÓ đi vào HTML, chứ không bị nuốt.
    expect(daCap).not.toBe(chuaCap);
  });
});

/**
 * CHẾ ĐỘ SỬA — ô bấm, nút `Lưu`/`Huỷ` đầu cột, câu lỗi của máy chủ ngay dưới cột ấy.
 *
 * `renderToStaticMarkup` không bấm được, nên trạng thái đưa vào đã được dựng sẵn bằng đúng các hàm
 * màn hình dùng (`sua-phan-quyen.ts`); ca ở đây khẳng định phần JSX ĐỌC trạng thái ấy.
 */
function veSua(b: BanSua, maVaiTroCuaToi: string | null = null) {
  const chinhSua: ChinhSuaMaTran = {
    banSua: b,
    maVaiTroCuaToi,
    batTat: () => {},
    luu: () => {},
    huy: () => {},
  };
  return renderToStaticMarkup(
    <BangMaTran nhom={NHOM_A} vaiTro={VAI_TRO} daCap={bangHienThi(b)} chinhSua={chinhSua} />,
  );
}

/** Thẻ `<button>` mang đúng tên đọc được ấy — để khẳng định `disabled` của CHÍNH nút ấy. */
function nut(html: string, ten: string): string {
  const m = html.match(new RegExp(`<button[^>]*aria-label="${ten}"[^>]*>`));
  if (m === null) throw new Error(`không thấy nút "${ten}"`);
  return m[0];
}

/**
 * The toggle `<button aria-pressed>` with exactly that name (the prototype's cell, ADR 0068 lần 5) —
 * attribute order is React's, so the tag is cut out. `nut` finds the column-head buttons the same way.
 */
function o(html: string, ten: string): string {
  const m = html.match(new RegExp(`<button[^>]*aria-label="${ten}"[^>]*>`));
  if (m === null) throw new Error(`không thấy ô "${ten}"`);
  return m[0];
}

const GOC_A = dungBangDaCap([{ role_id: "vt-2", permission: "admin.user" }]);

describe("BangMaTran ở chế độ sửa", () => {
  it("KHÔNG có phần sửa (tài khoản thiếu `admin.role`) thì không một ô bấm, không một nút Lưu", () => {
    const html = ve(NHOM_A, [{ role_id: "vt-2", permission: "admin.user" }]);
    expect(html).not.toContain("aria-pressed");
    expect(html).not.toContain("<button");
  });

  it("có phần sửa thì mỗi ô là một nút bật/tắt mang tên \"{tên vai trò} — {nhãn quyền}\" (prototype :231)", () => {
    const html = veSua(banSuaMoi(GOC_A));
    expect(html).toContain("aria-pressed");
    expect(html).toContain('aria-label="Chuyên viên — Quản lý tài khoản"');
    expect(html).toContain('title="Chuyên viên — Quản lý tài khoản"');
    expect(html).toContain('aria-label="Lãnh đạo — Phân quyền"');
    // Ô đã cấp thì đang bật.
    expect(o(html, "Lãnh đạo — Quản lý tài khoản")).toContain('aria-pressed="true"');
    expect(o(html, "Chuyên viên — Quản lý tài khoản")).toContain('aria-pressed="false"');
  });

  it("`Lưu` · `Huỷ` chỉ có ở cột ĐÃ SỬA (prototype) — cột chưa đụng tới không có nút nào", () => {
    const html = veSua(batTatO(banSuaMoi(GOC_A), "vt-1", "admin.role"));
    expect(nut(html, "Lưu phân quyền của vai trò Chuyên viên")).not.toContain('disabled=""');
    expect(html).toContain('aria-label="Huỷ thay đổi chưa lưu của vai trò Chuyên viên"');
    expect(html).not.toContain('aria-label="Lưu phân quyền của vai trò Lãnh đạo"');
    expect(html).not.toContain('aria-label="Huỷ thay đổi chưa lưu của vai trò Lãnh đạo"');
    // The changed cell is marked (P23 `bg-tangerine/12`), so the officer sees what will be saved.
    expect(html).toMatch(
      /<td class="border-line border-b px-3 py-2\.5 text-center bg-tangerine\/12"><button[^>]*aria-label="Chuyên viên — Phân quyền"/,
    );
    // An unchanged cell carries no tint.
    expect(html).toMatch(/<td class="border-line border-b px-3 py-2\.5 text-center"><button[^>]*aria-label="Lãnh đạo — Phân quyền"/);
  });

  it("chưa sửa gì thì không cột nào có nút Lưu", () => {
    expect(veSua(banSuaMoi(GOC_A))).not.toContain("Lưu phân quyền của vai trò");
  });

  it("câu từ chối của máy chủ hiện NGUYÊN VĂN, role=alert, và phần đã tick vẫn còn trên bảng", () => {
    const cau =
      "Xã phải luôn còn ít nhất một cán bộ đang hoạt động giữ quyền admin.user. Hãy cấp quyền này cho một vai trò khác có cán bộ đang hoạt động trước, rồi lưu lại.";
    let b = batTatO(banSuaMoi(GOC_A), "vt-2", "admin.user");
    b = ketThucLuu(batDauLuu(b, "vt-2"), "vt-2", { ok: false, thongBao: cau });
    const html = veSua(b);

    expect(html).toContain(`role="alert">${cau}<`);
    // Ô vừa gỡ vẫn đang gỡ (không bị trả về bản máy chủ), và nút Lưu vẫn bật để thử lại bằng tay.
    expect(o(html, "Lãnh đạo — Quản lý tài khoản")).toContain('aria-pressed="false"');
    expect(nut(html, "Lưu phân quyền của vai trò Lãnh đạo")).not.toContain('disabled=""');
  });

  it("cột của CHÍNH vai trò mình bị khoá kèm lý do (#14); cột khác vẫn bấm được", () => {
    const html = veSua(banSuaMoi(GOC_A), "lanh-dao");
    expect(html).toContain(LY_DO_KHONG_TU_SUA);
    expect(o(html, "Lãnh đạo — Phân quyền")).toContain('disabled=""');
    expect(o(html, "Chuyên viên — Phân quyền")).not.toContain('disabled=""');
    // Its cells cannot change, so it never offers a Save.
    expect(html).not.toContain('aria-label="Lưu phân quyền của vai trò Lãnh đạo"');
  });

  it("không biết vai trò của mình thì KHÔNG khoá cột nào — không đoán, máy chủ vẫn trả 403", () => {
    const html = veSua(banSuaMoi(GOC_A), null);
    expect(html).not.toContain(LY_DO_KHONG_TU_SUA);
    expect(html).not.toMatch(/<button[^>]*aria-pressed[^>]*disabled=""/);
    expect(html).not.toMatch(/<button[^>]*disabled=""[^>]*aria-pressed/);
  });
});

/**
 * PROTOTYPE FIDELITY — card B (08/10/2026), `vigov-require/apps/admin/src/components/admin/
 * RolePermissionMatrix.tsx`. One case per MISMATCH row it closes; the row number is in the name.
 * Class strings are pinned because the legacy `.bang-phan-quyen` block they replace is deleted: a
 * utility missing here is a property nothing sets any more.
 */
describe("BangMaTran follows the prototype (card B)", () => {
  it("P8/P9: frame scrolls horizontally only — no max-height, no sticky header row", () => {
    const html = ve(NHOM_A);
    expect(html).toMatch(/<div class="border-line overflow-x-auto rounded-\[10px\] border"/);
    expect(html).toContain('<table class="w-full border-collapse text-[12.5px]">');
    expect(html).not.toMatch(/max-h-|75vh|bang-cuon|bang-phan-quyen/);
    expect(html).not.toMatch(/sticky top-0/);
  });

  it("P10: corner cell 'Quyền' sticks left, canvas fill, no fixed width", () => {
    const html = ve(NHOM_A);
    expect(html).toMatch(
      /<th scope="col" class="bg-canvas border-line text-ink-muted sticky left-0 z-10 border-b px-4 py-3 text-left font-semibold">Quyền<\/th>/,
    );
  });

  it("P11–P13: role head — name, then ONE count '{n} cán bộ'", () => {
    const html = ve(NHOM_A);
    expect(html).toContain(
      'class="bg-canvas border-line min-w-30 border-b px-3 py-3 text-center align-bottom"',
    );
    expect(html).toContain('<div class="text-navy font-semibold">Chuyên viên</div>');
    expect(html).toContain('<div class="text-ink-muted mt-1 text-[10.5px] font-normal">3 cán bộ</div>');
    expect(html).not.toContain("trong số đó");
  });

  it("P14: leader badge is violet 'Lãnh đạo' with NO icon, after the count", () => {
    const html = ve(NHOM_A);
    expect(html).toMatch(
      /<span class="[^"]*bg-violet\/12 text-violet border-violet\/25 mt-1\.5 text-\[9\.5px\][^"]*">Lãnh đạo<\/span>/,
    );
    expect(html).not.toContain("lucide-crown");
    // Count first, badge second (prototype :147-154).
    const head = html.slice(html.indexOf(">1 cán bộ<"));
    expect(head.indexOf("bg-violet/12")).toBeGreaterThan(0);
    // Non-leader column has no badge: exactly one.
    expect(html.match(/bg-violet\/12/g)).toHaveLength(1);
  });

  it("P18: group row — one full-width cell in the prototype's classes, label from the server", () => {
    const html = ve(NHOM_A);
    expect(html).toMatch(
      /<th[^>]*colSpan="3"[^>]*class="bg-canvas\/70 border-line text-navy border-y px-4 py-2 text-left text-\[11\.5px\] font-bold tracking-wide uppercase"[^>]*>Quản trị<\/th>/,
    );
    expect(html).not.toContain("nhan-nhom");
  });

  it("P21/P22: permission cell — label + code, sticky left, white, wraps (no truncate)", () => {
    const html = ve(NHOM_A);
    expect(html).toContain('<tr class="hover:bg-canvas/60">');
    expect(html).toMatch(
      /<th scope="row" class="border-line sticky left-0 z-10 border-b bg-white px-4 py-2\.5 text-left font-normal"><div class="text-navy font-medium">Quản lý tài khoản<\/div><code class="text-ink-muted text-\[10\.5px\]">admin\.user<\/code><\/th>/,
    );
    expect(html).not.toContain("truncate");
  });

  it("P24/P25: read-only cell is the icon alone — leaf Check / muted Minus, centred", () => {
    const html = ve(NHOM_A, [{ role_id: "vt-1", permission: "admin.user" }]);
    expect(html).not.toContain("<button");
    expect(html).toMatch(/<svg[^>]*class="lucide lucide-check[^"]*text-leaf mx-auto size-4"/);
    expect(html).toMatch(/<svg[^>]*class="lucide lucide-minus[^"]*text-ink-muted\/40 mx-auto size-4"/);
  });

  it("P24: edit-mode toggle is the prototype's 28px square button", () => {
    const html = veSua(banSuaMoi(GOC_A));
    expect(o(html, "Chuyên viên — Quản lý tài khoản")).toContain(
      'class="hover:bg-canvas mx-auto grid size-7 place-items-center rounded-md',
    );
  });

  it("P15: Lưu / Huỷ are the prototype's 28px buttons, in a centred row", () => {
    const html = veSua(batTatO(banSuaMoi(GOC_A), "vt-1", "admin.role"));
    expect(html).toContain('<div class="mt-2 flex justify-center gap-1">');
    expect(nut(html, "Lưu phân quyền của vai trò Chuyên viên")).toMatch(/h-7 px-2 text-\[11px\]/);
    expect(nut(html, "Huỷ thay đổi chưa lưu của vai trò Chuyên viên")).toMatch(/h-7 px-2 text-\[11px\]/);
  });

  it("P15/P16: while ONE column saves, every column's Lưu/Huỷ is disabled; the saving one spins", () => {
    let b = batTatO(banSuaMoi(GOC_A), "vt-1", "admin.role");
    b = batTatO(b, "vt-2", "admin.role");
    b = batDauLuu(b, "vt-1");
    const html = veSua(b);

    const saveSaving = nut(html, "Lưu phân quyền của vai trò Chuyên viên");
    expect(saveSaving).toContain('disabled=""');
    expect(nut(html, "Huỷ thay đổi chưa lưu của vai trò Chuyên viên")).toContain('disabled=""');
    // The OTHER dirty column is frozen too (prototype `savingRoleId !== null`).
    expect(nut(html, "Lưu phân quyền của vai trò Lãnh đạo")).toContain('disabled=""');
    expect(nut(html, "Huỷ thay đổi chưa lưu của vai trò Lãnh đạo")).toContain('disabled=""');
    // Loader2 inside the saving column's Lưu only; the word stays "Lưu".
    expect(html.match(/lucide-loader-circle[^"]*size-3 animate-spin/g)).toHaveLength(1);
    expect(html).not.toContain("Đang lưu…");
  });
});
