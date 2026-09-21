import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { layPhienHienTai } from "@/lib/api/phien";
import {
  quyetDinhTheoKhoa,
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_XEM_PHAN_ANH,
  type QuyetDinhHien,
} from "@/lib/quyen";

import { KhungQuyen } from "./cong-quyen";

/**
 * CA BỊ TỪ CHỐI, KHÔNG CHỈ CA ĐƯỢC PHÉP.
 *
 * Tài khoản của người viết mã luôn có đủ quyền, nên hai nhánh "thiếu quyền" và "không đọc được
 * quyền" là hai nhánh không ai nhìn thấy trong lúc dựng — và đúng chúng là hai nhánh sẽ chạy
 * trên máy của một cán bộ chuyên môn. Nhánh thứ hai còn xảy ra với MỌI tài khoản, vào đúng lúc
 * phiên hết hạn.
 */

function phanHoiPhien(quyen: readonly string[]) {
  return new Response(
    JSON.stringify({
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: quyen,
    }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

function batFetch(tra: Response) {
  vi.stubGlobal("fetch", vi.fn(async () => tra));
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const CAU_THIEU = "Tài khoản của bạn không có quyền xem theo dõi giải ngân (budget.read).";

function dung(quyetDinh: QuyetDinhHien | null): string {
  return renderToStaticMarkup(
    <KhungQuyen quyetDinh={quyetDinh} cauThieuQuyen={CAU_THIEU}>
      <p>NOI-DUNG-DUOC-BAO-VE</p>
    </KhungQuyen>,
  );
}

describe("cổng quyền theo khoá — quyết định", () => {
  it("CÓ `budget.read` thì hiện", async () => {
    batFetch(phanHoiPhien(["budget.read", "task.read"]));
    expect(quyetDinhTheoKhoa(await layPhienHienTai(), QUYEN_XEM_GIAI_NGAN)).toEqual({ hien: true });
  });

  it("KHÔNG có `budget.read` thì ẩn — ca bị từ chối", async () => {
    batFetch(phanHoiPhien(["task.read", "document.read", "feedback.read"]));
    expect(quyetDinhTheoKhoa(await layPhienHienTai(), QUYEN_XEM_GIAI_NGAN)).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("hai khoá KHÔNG suy ra nhau: `feedback.read` không mở màn giải ngân", async () => {
    // Bộ quyền của khách không phải tích Descartes — từng khoá được liệt kê một cái một chính vì
    // chúng khác nhau. Nếu có ngày ai đó viết một phép khớp tiền tố thì ca này đỏ.
    batFetch(phanHoiPhien(["feedback.read"]));
    const phien = await layPhienHienTai();
    expect(quyetDinhTheoKhoa(phien, QUYEN_XEM_PHAN_ANH)).toEqual({ hien: true });
    expect(quyetDinhTheoKhoa(phien, QUYEN_XEM_GIAI_NGAN)).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("chuỗi gần giống cũng không mở được màn", async () => {
    batFetch(phanHoiPhien(["budget.reads", "budget.read.all", "BUDGET.READ", "budget.update"]));
    expect(quyetDinhTheoKhoa(await layPhienHienTai(), QUYEN_XEM_GIAI_NGAN)).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("`feedback.restricted` KHÔNG thay được `feedback.read`", async () => {
    // Khoá hạn chế mở riêng lĩnh vực phản ánh về tác phong cán bộ; nó không phải quyền đọc
    // phản ánh nói chung, và máy chủ kiểm hai khoá ở hai chỗ khác nhau.
    batFetch(phanHoiPhien(["feedback.restricted"]));
    expect(quyetDinhTheoKhoa(await layPhienHienTai(), QUYEN_XEM_PHAN_ANH)).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("phiên hết hạn (401): ẩn, và mang theo đúng câu của máy chủ", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "unauthenticated",
          message: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
          trace_id: "01JTRACE",
        }),
        { status: 401, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(quyetDinhTheoKhoa(await layPhienHienTai(), QUYEN_XEM_PHAN_ANH)).toEqual({
      hien: false,
      vi: "khong-doc-duoc",
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });
  });

  it("mạng hỏng: ẩn — 'chưa rõ có quyền hay không' hành xử như 'không có'", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    expect(quyetDinhTheoKhoa(await layPhienHienTai(), QUYEN_XEM_GIAI_NGAN).hien).toBe(false);
  });
});

describe("cổng quyền theo khoá — cái RA TỚI TRANG", () => {
  it("thiếu quyền: KHÔNG dựng nội dung được bảo vệ, và câu giải thích có mặt nguyên văn", () => {
    const html = dung({ hien: false, vi: "khong-du-quyen" });

    expect(html).not.toContain("NOI-DUNG-DUOC-BAO-VE");
    expect(html).toContain(CAU_THIEU);
  });

  it("thiếu quyền KHÔNG được dựng như một lỗi", () => {
    // `role="alert"` cắt ngang người dùng trình đọc màn hình. Một tài khoản không có quyền là
    // trạng thái bình thường, không có gì hỏng.
    const html = dung({ hien: false, vi: "khong-du-quyen" });

    expect(html).toContain('class="trang-thai-rong"');
    expect(html).not.toContain('role="alert"');
  });

  it("thiếu quyền KHÔNG được dựng thành một ô trống", () => {
    // Thứ đang canh là "có chữ cho người đọc", không phải "có thẻ p": một `<p></p>` rỗng qua
    // được phép kiểm thẻ và trượt đúng thứ ca test này sinh ra để bắt.
    expect(dung({ hien: false, vi: "khong-du-quyen" })).not.toContain(
      'class="trang-thai-rong"></p>',
    );
  });

  it("không đọc được quyền: hiện ĐÚNG câu của máy chủ, và vẫn không dựng nội dung", () => {
    const html = dung({
      hien: false,
      vi: "khong-doc-duoc",
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });

    expect(html).not.toContain("NOI-DUNG-DUOC-BAO-VE");
    expect(html).toContain("Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.");
    // Hai trạng thái này phải phân biệt được: "không có quyền" và "chưa biết có quyền hay
    // không" dẫn người dùng đi hai đường khác hẳn nhau.
    expect(html).not.toContain(CAU_THIEU);
    expect(html).toContain('role="alert"');
  });

  it("chưa đọc xong: KHÔNG hiện câu thiếu quyền, và cũng chưa dựng nội dung", () => {
    const html = dung(null);

    expect(html).not.toContain(CAU_THIEU);
    expect(html).not.toContain("NOI-DUNG-DUOC-BAO-VE");
    expect(html).toContain('role="status"');
  });

  it("đủ quyền: nội dung ra tới trang", () => {
    expect(dung({ hien: true })).toContain("NOI-DUNG-DUOC-BAO-VE");
  });
});
