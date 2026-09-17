import { afterEach, describe, expect, it, vi } from "vitest";

import { resolveTenant } from "./tenant-config";

/**
 * Suy ra xã từ `Host` là đường cách ly giữa hai cơ quan nhà nước. Mọi ca dưới đây đều là ca
 * KHÔNG nhìn thấy bằng mắt trên màn hình: một xã dự phòng, một 404 nuốt mất sự cố, hay một
 * `tenant_id` lọt vào yêu cầu đều chạy đúng ở xã đầu tiên và chỉ lộ ra ở xã thứ hai.
 */

const XA_A = "tanphu.example.gov.vn";

/** Thân 200 đúng hình dạng `identity.thongTinXa` của hợp đồng. */
function thongTinXa(than: { name: string; host: string; province: string }) {
  return new Response(JSON.stringify(than), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

/** Thân lỗi đúng hình dạng `httpx.Error`. */
function loiCuaMayChu(status: number, code: string, message: string) {
  return new Response(JSON.stringify({ code, message, trace_id: "" }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function batFetch(tra: Response) {
  const gia = vi.fn(async () => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof batFetch>) {
  const [dich, tuyChon] = gia.mock.calls[0] as unknown as [URL | string, RequestInit];
  return { dich: String(dich), tuyChon };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("resolveTenant — hình dạng lời gọi", () => {
  it("hỏi đúng tuyến công khai, trên chính host của yêu cầu", async () => {
    const gia = batFetch(
      thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "Thành phố Đà Nẵng" }),
    );
    await resolveTenant(XA_A);

    expect(loiGoi(gia).dich).toBe(`https://${XA_A}/api/v1/commune`);
  });

  it("không có `tenant_id` ở bất kỳ đâu — đường dẫn, truy vấn hay header", async () => {
    const gia = batFetch(thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" }));
    await resolveTenant(XA_A);

    const { dich, tuyChon } = loiGoi(gia);
    const tatCa = [dich, JSON.stringify(tuyChon.headers ?? {})].join(" ");
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã LÀ cái host đang hỏi.
    expect(tatCa).not.toMatch(/tenant|xa_id/i);
  });

  it("không gửi cookie và không đi theo chuyển hướng", async () => {
    const gia = batFetch(thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" }));
    await resolveTenant(XA_A);

    const { tuyChon } = loiGoi(gia);
    // Tuyến công khai: không có gì để uỷ quyền, nên không có cookie nào được chuyển tiếp.
    expect(tuyChon.credentials).toBeUndefined();
    // Một chuyển hướng ở đây là một đường để máy chủ đọc cấu hình của xã KHÁC rồi in ra dưới
    // host này.
    expect(tuyChon.redirect).toBe("error");
    expect(tuyChon.cache).toBe("no-store");
  });

  it("host viết hoa vẫn ra một origin duy nhất — biên cũng hạ chữ trước khi tra", async () => {
    const gia = batFetch(thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" }));
    await resolveTenant("TanPhu.Example.Gov.VN");

    expect(loiGoi(gia).dich).toBe(`https://${XA_A}/api/v1/commune`);
  });
});

describe("resolveTenant — 200", () => {
  it("`province` của hợp đồng về đúng `parentAuthority` của tầng web (ADR 0017)", async () => {
    batFetch(
      thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "Thành phố Đà Nẵng" }),
    );
    const xa = await resolveTenant(XA_A);

    expect(xa).toEqual({
      host: XA_A,
      displayName: "UBND xã Tân Phú",
      parentAuthority: "Thành phố Đà Nẵng",
    });
  });

  it("không mang `tenantId` và không mang `active` — hai trường hợp đồng CỐ Ý không trả", async () => {
    // Test này đỏ vào đúng ngày có người thêm lại một trong hai. `tenantId`: một định danh nội
    // bộ mà client cầm là một định danh client gửi ngược lên được (luật 1, cấm #2). `active`:
    // biên đã trả 404 cho xã ngừng hoạt động trước mọi handler, nên ở đây nó chỉ có thể là
    // `true` — một nhánh "xã đã sáp nhập" dựng trên nó sẽ không bao giờ chạy.
    batFetch(thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" }));
    const xa = await resolveTenant(XA_A);

    expect(Object.keys(xa ?? {}).sort()).toEqual(["displayName", "host", "parentAuthority"]);
  });

  it("tỉnh/thành rỗng đi nguyên vẹn thành chuỗi rỗng — không bịa, không mặc định", async () => {
    // "" nghĩa là xã chưa khai tỉnh/thành. Đoán một giá trị là để một cơ quan nhà nước tự nói
    // một điều không đúng về chính mình.
    batFetch(thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" }));
    const xa = await resolveTenant(XA_A);

    expect(xa?.parentAuthority).toBe("");
  });

  it("`host` lấy theo sổ đăng ký, không lấy lại chuỗi người gọi vừa gõ", async () => {
    batFetch(thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" }));
    const xa = await resolveTenant("bi-danh.example.gov.vn");

    expect(xa?.host).toBe(XA_A);
  });
});

describe("resolveTenant — không có xã nào", () => {
  it("404 trả `null` để bên gọi ra 404, không rơi về xã mặc định", async () => {
    batFetch(loiCuaMayChu(404, "tenant_not_found", "Không tìm thấy trang cho tên miền này."));
    expect(await resolveTenant("khong-ai-biet.example.gov.vn")).toBeNull();
  });

  it("xã đã ngừng hoạt động cũng là 404 ấy — hai ca, MỘT câu trả lời", async () => {
    // Biên trả cùng một 404 cho "chưa bao giờ có" và "đã ngừng hoạt động", để người gõ thử tên
    // miền không dò ra được xã nào từng tồn tại (`core/httpx/edge.go`). Phía web không dựng
    // lại sự phân biệt ấy.
    batFetch(loiCuaMayChu(404, "tenant_not_found", "Không tìm thấy trang cho tên miền này."));
    expect(await resolveTenant("da-sap-nhap.example.gov.vn")).toBeNull();
  });

  it("host không phải một tên miền thì từ chối NGAY, không gọi đi đâu cả", async () => {
    const gia = batFetch(thongTinXa({ name: "x", host: "x", province: "" }));

    for (const xau of ["", "   ", "xa.example.gov.vn/khac", "ai-do@noi-khac.example", "a b"]) {
      expect(await resolveTenant(xau)).toBeNull();
    }
    expect(gia).not.toHaveBeenCalled();
  });
});

describe("resolveTenant — máy chủ không trả lời được", () => {
  it("500 thì NÉM, không trả `null`: mất liên lạc không phải là 'xã này không tồn tại'", async () => {
    // Trả `null` ở đây sẽ dựng trang 404, tức là trong lúc sự cố thì hệ thống nói với người
    // vận hành rằng một xã có thật là không có thật.
    batFetch(loiCuaMayChu(500, "internal", "Đã xảy ra lỗi. Vui lòng thử lại."));
    await expect(resolveTenant(XA_A)).rejects.toThrow();
  });

  it("503 cũng vậy", async () => {
    batFetch(loiCuaMayChu(503, "unavailable", "Hệ thống đang bảo trì."));
    await expect(resolveTenant(XA_A)).rejects.toThrow();
  });

  it("mạng hỏng thì ném ra ngoài, không nuốt thành một xã rỗng", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    await expect(resolveTenant(XA_A)).rejects.toThrow();
  });
});
