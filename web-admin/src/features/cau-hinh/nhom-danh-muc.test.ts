import { afterEach, describe, expect, it, vi } from "vitest";

import { docDanhMucNghiepVu } from "@/lib/api/danh-muc-nghiep-vu";

import { nhomDanhMuc } from "./nhom-danh-muc";

/**
 * Kiểm CẢ ĐƯỜNG: phản hồi HTTP của bảy tuyến danh mục → bảy nhóm mà màn hình dựng được.
 *
 * Cố ý KHÔNG gọi `nhomDanhMuc` với một đối tượng `BayDanhMuc` dựng sẵn: hai ca đáng lo nhất của
 * màn hình này — danh mục RỖNG và một dịch vụ trong năm dịch vụ đang hỏng — đến từ phản hồi thật
 * của máy chủ, không từ một giá trị ai đó gõ trong test. Cùng khuôn với `quyen-tab.test.ts`.
 */

const TUYEN = {
  loaiTaiNguyen: "/api/v1/map-asset-types",
  hangMuc: "/api/v1/capital-plan-categories",
  loaiVanBan: "/api/v1/document-types",
  loaiDonViDanCu: "/api/v1/residential-unit-types",
  khoiNhiemVu: "/api/v1/task-blocs",
  loaiNhiemVu: "/api/v1/task-types",
  mucUuTien: "/api/v1/task-priorities",
} as const;

type PhanHoiGia = { status: number; than: unknown };

/**
 * Dựng `fetch` trả lời theo ĐƯỜNG DẪN. Một tuyến không được dựng thì test HỎNG thay vì im lặng
 * trả rỗng — nếu không, ngày ai đó đổi đường dẫn của một tuyến, bài test này vẫn xanh và chỉ báo
 * rằng danh mục ấy "chưa có mục nào".
 */
function batFetchTheoTuyen(bang: Readonly<Record<string, PhanHoiGia>>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (duongDan: string) => {
      const pd = bang[duongDan];
      if (pd === undefined) throw new Error(`test chưa dựng phản hồi cho tuyến: ${duongDan}`);
      return new Response(JSON.stringify(pd.than), {
        status: pd.status,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );
}

/** Bảy tuyến cùng trả `{"items": []}` — ĐÚNG câu trả lời của mọi đơn vị hôm nay. */
function bayTuyenRong(): Record<string, PhanHoiGia> {
  const bang: Record<string, PhanHoiGia> = {};
  for (const duongDan of Object.values(TUYEN)) bang[duongDan] = { status: 200, than: { items: [] } };
  return bang;
}

function muc(code: string, label: string, them: { is_default?: boolean; active?: boolean } = {}) {
  return {
    id: `01J00000000000000000${code.slice(0, 2).toUpperCase().padEnd(2, "X")}`,
    code,
    label,
    is_default: them.is_default ?? false,
    active: them.active ?? true,
  };
}

function loi(message: string) {
  return { code: "internal", message, trace_id: "01JTRACE" };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("bảy nhóm danh mục", () => {
  it("đủ bảy nhóm, ĐÚNG THỨ TỰ bảng §5 của đặc tả — không theo vần, không theo tên dịch vụ", async () => {
    batFetchTheoTuyen(bayTuyenRong());
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    expect(nhom.map((n) => n.nhan)).toEqual([
      "Loại tài nguyên bản đồ",
      "Hạng mục kế hoạch vốn",
      "Loại văn bản",
      "Loại đơn vị dân cư",
      "Khối nhiệm vụ",
      "Loại nhiệm vụ",
      "Mức ưu tiên nhiệm vụ",
    ]);
  });

  it("ĐÚNG MỘT nhóm có thứ tự mang nghĩa thang bậc", async () => {
    // Nếu có ngày cờ này bật cho một nhóm thứ hai, màn hình sẽ nói với cán bộ rằng thứ tự của một
    // danh mục trình bày là thang bậc nghiệp vụ — một câu sai về dữ liệu của đơn vị.
    batFetchTheoTuyen(bayTuyenRong());
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    expect(nhom.filter((n) => n.thuTuLaThangBac).map((n) => n.nhan)).toEqual([
      "Mức ưu tiên nhiệm vụ",
    ]);
  });
});

describe("danh mục rỗng là ĐƯỜNG THÔNG THƯỜNG, không phải lỗi", () => {
  it("bảy tuyến trả `items: []` → bảy nhóm `chuaCoMuc`, không nhóm nào là lỗi", async () => {
    // ĐÂY LÀ CA CỦA MỌI ĐƠN VỊ HÔM NAY: migration cố ý không gieo mục nào và bước khởi tạo đơn vị
    // chưa tồn tại. Một màn hình đọc ca này thành lỗi — hay tệ hơn, thành một vòng quay không bao
    // giờ dừng — là một cuộc gọi hỗ trợ từ một cơ quan nhà nước về một hệ thống đang chạy đúng.
    batFetchTheoTuyen(bayTuyenRong());
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    expect(nhom).toHaveLength(7);
    expect(nhom.every((n) => n.trangThai.pha === "chuaCoMuc")).toBe(true);
  });

  it("`items: []` KHÔNG được đọc thành `khongDocDuoc`", async () => {
    batFetchTheoTuyen(bayTuyenRong());
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    expect(nhom.some((n) => n.trangThai.pha === "khongDocDuoc")).toBe(false);
  });
});

describe("một dịch vụ hỏng KHÔNG làm im lặng sáu nhóm còn lại", () => {
  it("500 ở một tuyến: nhóm ấy mang câu của máy chủ, sáu nhóm kia vẫn dựng", async () => {
    // Năm dịch vụ đứng sau bảy tuyến, nên một dịch vụ đang khởi động lại là chuyện có thật. Gộp
    // bảy kết quả làm một sẽ biến sự cố của một dịch vụ thành bảy danh mục cùng biến mất.
    batFetchTheoTuyen({
      ...bayTuyenRong(),
      [TUYEN.loaiVanBan]: { status: 500, than: loi("Đã xảy ra lỗi. Vui lòng thử lại.") },
    });
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    const vanBan = nhom.find((n) => n.nhan === "Loại văn bản");
    expect(vanBan?.trangThai).toEqual({
      pha: "khongDocDuoc",
      thongBao: "Đã xảy ra lỗi. Vui lòng thử lại.",
    });
    expect(nhom.filter((n) => n.trangThai.pha === "khongDocDuoc")).toHaveLength(1);
  });

  it("401 (phiên hết hạn) hiện ĐÚNG câu của máy chủ, không phải một câu tự đặt", async () => {
    const bang = bayTuyenRong();
    const het = {
      status: 401,
      than: loi("Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại."),
    };
    for (const duongDan of Object.values(TUYEN)) bang[duongDan] = het;

    batFetchTheoTuyen(bang);
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    for (const n of nhom) {
      expect(n.trangThai).toEqual({
        pha: "khongDocDuoc",
        thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
      });
    }
  });

  it("mạng hỏng: nhóm nào cũng nói không đọc được, không nhóm nào nói 'chưa có mục'", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    expect(nhom.every((n) => n.trangThai.pha === "khongDocDuoc")).toBe(true);
    expect(nhom.some((n) => n.trangThai.pha === "chuaCoMuc")).toBe(false);
  });
});

describe("thang mức ưu tiên: thứ tự của `items` LÀ dữ liệu", () => {
  it("giữ NGUYÊN thứ tự máy chủ trả về — không sắp theo vần, không sắp theo mã", async () => {
    // Máy chủ sắp theo `thu_tu` của chính đơn vị và CỐ Ý không nói đầu nào là đầu gấp nhất. Một
    // phép `sort` ở bất kỳ đâu trong chuỗi từ `docJSON` tới bảng không đổi cách trình bày — nó đổi
    // mức việc đơn vị coi là gấp nhất, và màn hình vẫn trông bình thường. Sắp theo vần sẽ cho ra
    // ["cao", "khan", "thuong"], nên bài test này đỏ ngay.
    batFetchTheoTuyen({
      ...bayTuyenRong(),
      [TUYEN.mucUuTien]: {
        status: 200,
        than: {
          items: [
            muc("khan", "Khẩn"),
            muc("cao", "Cao"),
            muc("thuong", "Thường", { is_default: true }),
          ],
        },
      },
    });

    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());
    const thang = nhom.find((n) => n.nhan === "Mức ưu tiên nhiệm vụ");
    if (thang?.trangThai.pha !== "coMuc") throw new Error("thang bậc phải dựng được");

    expect(thang.trangThai.muc.map((m) => m.code)).toEqual(["khan", "cao", "thuong"]);
  });
});

describe("mục đã tắt vẫn về tới màn hình", () => {
  it("`active: false` KHÔNG bị lọc bỏ — hồ sơ đã lập theo mã đó vẫn cần nhãn để hiển thị", async () => {
    batFetchTheoTuyen({
      ...bayTuyenRong(),
      [TUYEN.loaiVanBan]: {
        status: 200,
        than: {
          items: [
            muc("cong-van", "Công văn", { is_default: true }),
            muc("to-trinh", "Tờ trình", { active: false }),
          ],
        },
      },
    });

    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());
    const vanBan = nhom.find((n) => n.nhan === "Loại văn bản");
    if (vanBan?.trangThai.pha !== "coMuc") throw new Error("nhóm loại văn bản phải dựng được");

    expect(vanBan.trangThai.muc.map((m) => m.code)).toEqual(["cong-van", "to-trinh"]);
    expect(vanBan.trangThai.muc.map((m) => m.active)).toEqual([true, false]);
  });
});

describe("lớp client không quyết định quyền — máy chủ quyết định", () => {
  it("máy chủ trả 403 thì màn hình hiện đúng câu của máy chủ, không tự đoán 'không có quyền'", async () => {
    // Bảy tuyến này khai `any-authenticated` ở máy chủ, nên tab Danh mục KHÔNG có cổng quyền ở
    // giao diện (`tab-danh-muc.tsx`). Ngày khai báo tuyến đổi sang đòi một quyền, 403 phải hiện
    // ra nguyên văn câu của máy chủ chứ không biến thành một bảng rỗng.
    const bang = bayTuyenRong();
    bang[TUYEN.hangMuc] = {
      status: 403,
      than: loi("Bạn không có quyền thực hiện thao tác này."),
    };

    batFetchTheoTuyen(bang);
    const nhom = nhomDanhMuc(await docDanhMucNghiepVu());

    expect(nhom.find((n) => n.nhan === "Hạng mục kế hoạch vốn")?.trangThai).toEqual({
      pha: "khongDocDuoc",
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });
});
