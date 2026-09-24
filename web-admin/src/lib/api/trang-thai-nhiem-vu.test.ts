import { afterEach, describe, expect, it, vi } from "vitest";

import { thanDatLaiMacDinh } from "@/features/cau-hinh/trang-thai-nhiem-vu";

import type { petitions_trangThaiNhiemVuRa } from "./schema.gen";
import { layTrangThaiNhiemVu, suaTrangThaiNhiemVu, type SuaTrangThaiVao } from "./trang-thai-nhiem-vu";

/**
 * Hai tuyến của nhóm `Trạng thái nhiệm vụ` (#21). Điều đắt nhất canh ở đây KHÔNG nhìn thấy được
 * trên màn hình: thân PATCH không bao giờ mang `code` hay `active`. Máy chủ trả 400 cho cả hai — kể
 * cả khi `code` đúng bằng mã trên đường dẫn — nên lối viết "đọc một dòng rồi gửi lại chính nó" là
 * một lần lưu hỏng mà người bấm không hiểu vì sao.
 */

function traVe(status: number, than: unknown) {
  return new Response(JSON.stringify(than), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function batFetch(tra: () => Response) {
  const gia = vi.fn(async () => tra());
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof batFetch>) {
  const [duongDan, init] = gia.mock.calls[0] as unknown as [string, RequestInit];
  const than =
    init.body === undefined || init.body === null
      ? {}
      : (JSON.parse(String(init.body)) as Record<string, unknown>);
  return { duongDan, phuongThuc: init.method, than };
}

/** Một dòng đúng như máy chủ trả — xã đã đổi nhãn `moi-giao`. */
const DONG: petitions_trangThaiNhiemVuRa = {
  code: "moi-giao",
  label: "Chưa thực hiện",
  order: 1,
  role: "chinh",
  default_label: "Mới giao",
  default_order: 1,
  customised: true,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("GET /api/v1/task-statuses", () => {
  it("đọc đúng đường dẫn tương đối, không tham số, không `tenant_id`", async () => {
    const gia = batFetch(() => traVe(200, { items: [DONG] }));
    const kq = await layTrangThaiNhiemVu();
    const { duongDan, phuongThuc } = loiGoi(gia);
    expect(duongDan).toBe("/api/v1/task-statuses");
    expect(phuongThuc).toBe("GET");
    expect(kq.ok && kq.duLieu.items[0]?.label).toBe("Chưa thực hiện");
  });

  it("hỏng: trả câu của máy chủ, không ném", async () => {
    batFetch(() => traVe(500, { code: "internal", message: "Đã xảy ra lỗi.", trace_id: "t" }));
    const kq = await layTrangThaiNhiemVu();
    expect(kq).toEqual({ ok: false, thongBao: "Đã xảy ra lỗi." });
  });
});

describe("PATCH /api/v1/task-statuses/{code}", () => {
  it("mã đi trên ĐƯỜNG DẪN (mã hoá), phương thức PATCH", async () => {
    const gia = batFetch(() => traVe(200, DONG));
    await suaTrangThaiNhiemVu("moi-giao", { label: "Chưa thực hiện" });
    const { duongDan, phuongThuc } = loiGoi(gia);
    expect(duongDan).toBe("/api/v1/task-statuses/moi-giao");
    expect(phuongThuc).toBe("PATCH");
  });

  it("thân CHỈ có `label`/`order` — kể cả khi bên gọi lỡ trải nguyên một dòng máy chủ vào", async () => {
    // VẾ CHỊU LỰC. Kiểu `SuaTrangThaiVao` đã `Omit` hai trường ấy; ép kiểu dưới đây mô phỏng đúng
    // lối sai lúc chạy mà kiểu không sống tới: truyền nguyên dòng đọc về (có `code`) cộng `active`.
    const gia = batFetch(() => traVe(200, DONG));
    const lanSai = { ...DONG, active: false } as unknown as SuaTrangThaiVao;
    await suaTrangThaiNhiemVu("moi-giao", lanSai);
    const { than } = loiGoi(gia);
    expect(Object.keys(than).sort()).toEqual(["label", "order"]);
    expect(than).not.toHaveProperty("code");
    expect(than).not.toHaveProperty("active");
  });

  it("trường không đổi thì VẮNG MẶT, không gửi `null` hay chuỗi rỗng", async () => {
    const gia = batFetch(() => traVe(200, DONG));
    await suaTrangThaiNhiemVu("moi-giao", { order: 3 });
    expect(loiGoi(gia).than).toEqual({ order: 3 });
  });

  it("`Đặt lại mặc định` gửi ĐÚNG `default_label` và `default_order` của dòng ấy", async () => {
    const gia = batFetch(() => traVe(200, { ...DONG, label: "Mới giao", customised: false }));
    await suaTrangThaiNhiemVu(DONG.code, thanDatLaiMacDinh({ ...DONG, order: 9 }));
    expect(loiGoi(gia).than).toEqual({ label: "Mới giao", order: 1 });
  });

  it("400 của máy chủ ra tới màn bằng CÂU của máy chủ", async () => {
    const cau = "Nhãn trạng thái không được để trống.";
    batFetch(() => traVe(400, { code: "invalid", message: cau, trace_id: "t" }));
    const kq = await suaTrangThaiNhiemVu("moi-giao", { label: "" });
    expect(kq).toEqual({ ok: false, thongBao: cau });
  });
});
