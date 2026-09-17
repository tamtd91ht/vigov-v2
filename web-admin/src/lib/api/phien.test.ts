import { afterEach, describe, expect, it, vi } from "vitest";

import { dangNhap, dangXuat } from "./phien";

/** Thân phản hồi lỗi đúng hình dạng `httpx.Error` của máy chủ. */
function loiCuaMayChu(status: number, than: { code: string; message: string }) {
  return new Response(JSON.stringify({ ...than, trace_id: "" }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function batFetch(tra: Response) {
  const gia = vi.fn(async () => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

/** Đối số của lời gọi fetch gần nhất. */
function loiGoi(gia: ReturnType<typeof batFetch>) {
  const [duongDan, tuyChon] = gia.mock.calls[0] as unknown as [string, RequestInit];
  return { duongDan, tuyChon };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("dangNhap — một thông báo duy nhất", () => {
  // Đây là test giữ đúng thứ máy chủ cố ý giấu. Nếu có ngày ai đó ánh xạ `code` sang câu chữ
  // riêng cho từng ca, test này đỏ — và nó phải đỏ: phân biệt "email sai" với "mật khẩu sai"
  // là để người ngoài dò ra thư công vụ nào có thật trên tên miền của xã.
  const CAU_CHUNG = "Email hoặc mật khẩu không đúng.";

  it("sai email trả đúng câu máy chủ viết", async () => {
    batFetch(loiCuaMayChu(401, { code: "invalid_credentials", message: CAU_CHUNG }));
    const kq = await dangNhap({ email: "khong-ton-tai@thangbinh.vigov.vn", password: "x" });
    expect(kq).toEqual({ ok: false, thongBao: CAU_CHUNG });
  });

  it("sai mật khẩu trả đúng câu ấy, không khác một chữ", async () => {
    batFetch(loiCuaMayChu(401, { code: "invalid_credentials", message: CAU_CHUNG }));
    const kq = await dangNhap({ email: "can.bo@thangbinh.vigov.vn", password: "sai" });
    expect(kq).toEqual({ ok: false, thongBao: CAU_CHUNG });
  });

  it("bỏ trống cũng vậy — máy chủ trả lời ca này, giao diện không tự đoán", async () => {
    batFetch(loiCuaMayChu(401, { code: "invalid_credentials", message: CAU_CHUNG }));
    const kq = await dangNhap({ email: "", password: "" });
    expect(kq).toEqual({ ok: false, thongBao: CAU_CHUNG });
  });

  it("không đọc `code` để đổi chữ: cùng message thì cùng kết quả dù code khác", async () => {
    batFetch(loiCuaMayChu(401, { code: "mot_ma_khac", message: CAU_CHUNG }));
    const kq = await dangNhap({ email: "a@b.vn", password: "x" });
    expect(kq).toEqual({ ok: false, thongBao: CAU_CHUNG });
  });

  it("thân lỗi không đọc được thì vẫn chỉ một câu, không lộ chi tiết kỹ thuật", async () => {
    batFetch(new Response("<html>502</html>", { status: 502 }));
    const kq = await dangNhap({ email: "a@b.vn", password: "x" });
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("Không kết nối được máy chủ. Vui lòng thử lại.");
  });
});

describe("dangNhap — hình dạng yêu cầu", () => {
  const OK_201 = () =>
    new Response(
      JSON.stringify({
        sid: "01J000000000000000000SID",
        expires_at: "2026-09-17T12:00:00Z",
        staff: { code: "CB001", full_name: "Nguyễn Văn A", position: "Chủ tịch UBND xã" },
      }),
      { status: 201, headers: { "Content-Type": "application/json" } },
    );

  it("thân yêu cầu có ĐÚNG hai trường của hợp đồng — không có tenant_id", async () => {
    const gia = batFetch(OK_201());
    await dangNhap({ email: "can.bo@thangbinh.vigov.vn", password: "mat-khau" });

    const { tuyChon } = loiGoi(gia);
    const than = JSON.parse(String(tuyChon.body)) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual(["email", "password"]);
  });

  it("không có chữ 'tenant' ở bất kỳ đâu trong yêu cầu — thân, đường dẫn hay header", async () => {
    const gia = batFetch(OK_201());
    await dangNhap({ email: "can.bo@thangbinh.vigov.vn", password: "mat-khau" });

    const { duongDan, tuyChon } = loiGoi(gia);
    const tatCa = [
      duongDan,
      String(tuyChon.body),
      JSON.stringify(tuyChon.headers ?? {}),
    ].join(" ");
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã suy từ Host, ở máy chủ.
    expect(tatCa).not.toMatch(/tenant|commune|\bxa_id\b/i);
  });

  it("đường dẫn là tương đối — không host nào bị nung vào bundle", async () => {
    const gia = batFetch(OK_201());
    await dangNhap({ email: "a@b.vn", password: "x" });

    const { duongDan } = loiGoi(gia);
    expect(duongDan).toBe("/api/v1/sessions");
    expect(duongDan).not.toMatch(/^https?:/);
  });

  it("cookie đi kèm bằng credentials same-origin, không phải include", async () => {
    const gia = batFetch(OK_201());
    await dangNhap({ email: "a@b.vn", password: "x" });

    expect(loiGoi(gia).tuyChon.credentials).toBe("same-origin");
  });

  it("trả về sid và khối cán bộ đúng kiểu sinh từ hợp đồng", async () => {
    batFetch(OK_201());
    const kq = await dangNhap({ email: "a@b.vn", password: "x" });

    expect(kq.ok).toBe(true);
    if (kq.ok) {
      expect(kq.duLieu.sid).toBe("01J000000000000000000SID");
      expect(kq.duLieu.staff.position).toBe("Chủ tịch UBND xã");
    }
  });

  it("mạng hỏng thì không ném ra ngoài — biểu mẫu còn nguyên dữ liệu người dùng đang nhập", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    const kq = await dangNhap({ email: "a@b.vn", password: "x" });
    expect(kq.ok).toBe(false);
  });
});

describe("dangXuat", () => {
  it("gọi đúng DELETE /api/v1/sessions/{sid}, không thân, không tenant_id", async () => {
    const gia = batFetch(new Response(null, { status: 204 }));
    const kq = await dangXuat("01J000000000000000000SID");

    const { duongDan, tuyChon } = loiGoi(gia);
    expect(duongDan).toBe("/api/v1/sessions/01J000000000000000000SID");
    expect(tuyChon.method).toBe("DELETE");
    expect(tuyChon.body).toBeUndefined();
    expect(kq.ok).toBe(true);
  });

  it("sid được mã hoá vào đường dẫn, không ghép thẳng", async () => {
    const gia = batFetch(new Response(null, { status: 204 }));
    await dangXuat("a/b?c=d");
    expect(loiGoi(gia).duongDan).toBe("/api/v1/sessions/a%2Fb%3Fc%3Dd");
  });

  it("401 và 404 coi như đã xong — phiên không còn hiệu lực là đúng điều vừa yêu cầu", async () => {
    for (const ma of [401, 404]) {
      batFetch(loiCuaMayChu(ma, { code: "x", message: "y" }));
      expect((await dangXuat("SID")).ok).toBe(true);
      vi.unstubAllGlobals();
    }
  });

  it("500 thì báo lỗi, không giả vờ đã đăng xuất", async () => {
    batFetch(loiCuaMayChu(500, { code: "internal", message: "Đã xảy ra lỗi. Vui lòng thử lại." }));
    const kq = await dangXuat("SID");
    expect(kq).toEqual({ ok: false, thongBao: "Đã xảy ra lỗi. Vui lòng thử lại." });
  });
});
