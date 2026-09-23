import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import type { comms_thongBaoRa } from "@/lib/api/schema.gen";

import {
  coChipTrangThai,
  mocThe,
  nangGhimLenDau,
  nhanBoDemXacNhan,
  nhanMoc,
  nhanTrangThai,
  nhanTrangThaiThu,
  tachMaNguoiNhan,
  trichNoiDung,
} from "./nhan-thong-bao";

/**
 * Phép định dạng và phép quyết định của màn Thông báo.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **BỘ ĐẾM XÁC NHẬN**, và nó là nhóm không ai nhìn thấy
 * trong lúc phát triển: dữ liệu mẫu bao giờ cũng có `ack_required: true`, nên "luôn vẽ" và "chỉ vẽ
 * khi bật cờ" cho ra cùng một màn hình. Điều phải đúng là chuyện ngược lại — một thông báo KHÔNG
 * bắt buộc xác nhận vẫn mang hai con số trên dây, và vẽ chúng ra là nói với cả xã rằng `0/8` người
 * đã xác nhận một thứ không ai được yêu cầu xác nhận.
 */

function thongBao(sua: Partial<comms_thongBaoRa> = {}): comms_thongBaoRa {
  return {
    id: "01JTB1",
    title: "Thông báo về việc triển khai hệ thống an ninh",
    body: "Đề nghị các bộ phận cử cán bộ dự buổi tập huấn.",
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

describe("mốc `HH:mm dd/MM/yyyy` (§3)", () => {
  it("in theo giờ Việt Nam, có đệm số 0, đúng khuôn đặc tả", () => {
    // 09:35Z = 16:35 giờ Việt Nam — đúng con số §3 và §10 in ra.
    expect(nhanMoc("2026-09-07T09:35:00Z")).toBe("16:35 07/09/2026");
  });

  it("GHIM múi giờ chứ không theo máy chạy — một mốc quanh nửa đêm không được nhảy ngày", () => {
    // 2026-09-07T17:30Z = 00:30 ngày 08/09 giờ Việt Nam. Không ghim múi giờ thì máy chạy ở UTC in
    // ra ngày 07, và giờ phát hành của một thông báo lệch hẳn một ngày.
    expect(nhanMoc("2026-09-07T17:30:00Z")).toBe("00:30 08/09/2026");
  });

  it("bộ định dạng ghim `timeZone` TRONG MÃ NGUỒN, không mượn đồng hồ của máy chạy", () => {
    /* ═════════════════════════════════════════════════════════════════════════════════════
     * BÀI KIỂM NÀY ĐỌC MÃ NGUỒN, VÀ NÓ Ở ĐÂY VÌ MỘT PHÉP ĐỘT BIẾN ĐÃ ĐO ĐƯỢC.
     *
     * Xoá dòng `timeZone: MUI_GIO` khỏi `DINH_DANG_MOC` làm bốn bài kiểm đỏ khi chạy với
     * `TZ=UTC` — và XANH HẾT trên chính máy viết mã, vì máy ấy đang ở +07 nên đầu ra trùng
     * khít. `vitest.config.mts` không ghim `TZ` và lượt này không được sửa tệp ấy, nên mọi
     * bài kiểm so theo ĐẦU RA đều xanh vì lý do sai ở đúng những máy hay dùng nhất: máy của
     * người Việt Nam viết mã cho các xã Việt Nam.
     *
     * "Bộ định dạng có được ghim hay không" là tính chất của MÃ NGUỒN, không phải của đầu ra
     * trên máy này. Nên nó được kiểm bằng cách đọc mã nguồn — cùng khuôn `ranh-gioi-nguon.
     * test.ts` đã dùng cho những ràng buộc chỉ lộ ra khi có xã thứ hai.
     * ═════════════════════════════════════════════════════════════════════════════════════ */
    const nguon = readFileSync(new URL("./nhan-thong-bao.ts", import.meta.url), "utf8");

    expect(nguon).toContain('const MUI_GIO = "Asia/Ho_Chi_Minh"');
    // Dòng ghim phải nằm NGAY TRONG lời gọi `Intl.DateTimeFormat`, không phải ở một chú thích
    // nào đó trong tệp.
    expect(nguon).toMatch(/new Intl\.DateTimeFormat\([^)]*timeZone:\s*MUI_GIO/s);
  });

  it("`null` thành dấu gạch, chuỗi không đọc được hiện NGUYÊN VĂN", () => {
    expect(nhanMoc(null)).toBe("—");
    expect(nhanMoc("")).toBe("—");
    // Không `Invalid Date`, và không dấu gạch: dấu gạch nói dối rằng máy chủ không gửi gì.
    expect(nhanMoc("hôm qua")).toBe("hôm qua");
  });

  it("thẻ dùng `issued_at`, và rơi về `created_at` khi thông báo còn là nháp", () => {
    expect(mocThe(thongBao())).toBe("16:35 07/09/2026");
    expect(mocThe(thongBao({ issued_at: null }))).toBe("16:34 07/09/2026");
  });
});

describe("BỘ ĐẾM XÁC NHẬN — chỉ vẽ khi bắt buộc xác nhận (§3)", () => {
  it("bật cờ thì `{x}/{y} đã xác nhận`", () => {
    expect(nhanBoDemXacNhan(thongBao())).toBe("2/12 đã xác nhận");
  });

  it("TẮT cờ thì KHÔNG có bộ đếm, dù hai con số vẫn đi trên dây", () => {
    // Máy chủ luôn gửi hai con số; "có nên vẽ hay không" có đúng một nguồn là cái cờ.
    expect(nhanBoDemXacNhan(thongBao({ ack_required: false, ack_count: 0 }))).toBeNull();
  });

  it("chưa ai xác nhận thì vẫn là `0/7`, không phải một ô trống", () => {
    expect(nhanBoDemXacNhan(thongBao({ ack_count: 0, recipient_count: 7 }))).toBe(
      "0/7 đã xác nhận",
    );
  });
});

describe("chip trạng thái thư (§3)", () => {
  it("`chua-gui` KHÔNG có chip — §3 không có nhãn nào cho giá trị ấy", () => {
    // Và hôm nay MỌI hàng đều là `chua-gui`: kho không có bộ gửi thư nào.
    expect(nhanTrangThaiThu("chua-gui")).toBeNull();
  });

  it("ba giá trị còn lại mang đúng chữ đặc tả", () => {
    expect(nhanTrangThaiThu("dang-gui")).toBe("Đang gửi thư…");
    expect(nhanTrangThaiThu("da-gui")).toBe("Đã gửi thư");
    expect(nhanTrangThaiThu("loi")).toBe("Gửi thư lỗi");
  });

  it("mã lạ không sinh ra chip", () => {
    expect(nhanTrangThaiThu("dang-thu-lai")).toBeNull();
  });
});

describe("trạng thái bản ghi (§6)", () => {
  it("ba mã có nhãn tiếng Việt, mã lạ hiện NGUYÊN VĂN", () => {
    expect(nhanTrangThai("nhap")).toBe("Nháp");
    expect(nhanTrangThai("da-phat-hanh")).toBe("Đã phát hành");
    expect(nhanTrangThai("da-go")).toBe("Đã gỡ");
    // Một trạng thái mới ở máy chủ mà màn hình vẽ thành `—` là một thông báo trông như chưa có
    // trạng thái.
    expect(nhanTrangThai("dang-duyet")).toBe("dang-duyet");
  });

  it("chỉ thông báo KHÁC `da-phat-hanh` mới có chip", () => {
    expect(coChipTrangThai("da-phat-hanh")).toBe(false);
    expect(coChipTrangThai("da-go")).toBe(true);
    expect(coChipTrangThai("nhap")).toBe(true);
  });
});

describe("GHIM — nâng trong TRANG, và thứ tự tương đối không đổi", () => {
  const A = thongBao({ id: "a", pinned: false });
  const B = thongBao({ id: "b", pinned: true });
  const C = thongBao({ id: "c", pinned: false });
  const D = thongBao({ id: "d", pinned: true });

  it("thẻ ghim lên đầu, hai nhóm giữ nguyên thứ tự máy chủ trả", () => {
    // Thứ tự máy chủ là `tao_luc` giảm dần. Đảo thứ tự BÊN TRONG một nhóm là đổi "mới nhất ở trên"
    // thành một thứ tự không ai đọc được.
    expect(nangGhimLenDau([A, B, C, D]).map((t) => t.id)).toEqual(["b", "d", "a", "c"]);
  });

  it("không thẻ nào ghim thì danh sách y nguyên", () => {
    expect(nangGhimLenDau([A, C]).map((t) => t.id)).toEqual(["a", "c"]);
  });

  it("không làm mất và không nhân đôi thẻ nào", () => {
    expect(nangGhimLenDau([A, B, C, D])).toHaveLength(4);
    expect(nangGhimLenDau([])).toHaveLength(0);
  });
});

describe("trích nội dung cho thẻ (§3)", () => {
  it("ngắn thì giữ nguyên, không thêm dấu ba chấm", () => {
    expect(trichNoiDung("Mời họp giao ban.")).toBe("Mời họp giao ban.");
  });

  it("dài thì cắt và đánh dấu đã cắt — toàn văn nằm ở phần chi tiết", () => {
    const ra = trichNoiDung("a".repeat(200));
    expect(ra.endsWith("…")).toBe(true);
    expect(ra.length).toBe(161);
  });

  it("xuống dòng thành dấu cách — không dính chữ cuối dòng vào chữ đầu dòng sau", () => {
    expect(trichNoiDung("Kính gửi các bộ phận\nĐề nghị cử cán bộ")).toBe(
      "Kính gửi các bộ phận Đề nghị cử cán bộ",
    );
  });
});

describe("tách mã người nhận (§5)", () => {
  it("mỗi dòng một mã, bỏ dòng trắng và khoảng trắng thừa", () => {
    expect(tachMaNguoiNhan("CB-2026-7K3M9Q\n\n  CB-2026-4H2N8P  \n")).toEqual([
      "CB-2026-7K3M9Q",
      "CB-2026-4H2N8P",
    ]);
  });

  it("ô trống cho danh sách RỖNG — biểu mẫu dựa vào đó để tắt nút phát hành", () => {
    expect(tachMaNguoiNhan("")).toEqual([]);
    expect(tachMaNguoiNhan("   \n  ")).toEqual([]);
  });

  it("KHÔNG kiểm khuôn mã — khuôn ấy là của `identity` và đổi được", () => {
    // Một biểu thức chính quy ở đây sẽ từ chối một mã thật vào ngày `identity` nới khuôn (luật 2).
    // Máy chủ vẫn từ chối thứ không phải mã: một họ tên có dấu cách, và `ChuanHoaMaCanBo` chặn nó.
    expect(tachMaNguoiNhan("MA-KIEU-MOI-2027")).toEqual(["MA-KIEU-MOI-2027"]);
  });
});
