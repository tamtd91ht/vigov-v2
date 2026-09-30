import { afterEach, describe, expect, it, vi } from "vitest";

import { capTaiKhoan, datLaiMatKhau, tuDoiMatKhau } from "./tai-khoan";

/**
 * BA TUYẾN CỦA THÔNG TIN ĐĂNG NHẬP, và tệp này canh những thứ một lần sửa MỘT DÒNG phá được mà
 * không phép kiểm nào khác thấy — vì hậu quả của chúng không hiện ra trên màn hình, nó hiện ra
 * ở chỗ một tài khoản bị chiếm hoặc một mật khẩu tạm mất vĩnh viễn.
 *
 * KHÔNG CÓ MỘT GIÁ TRỊ NÀO TRONG TỆP NÀY TRÔNG NHƯ MẬT KHẨU THẬT. `secret_scan` chặn, và nó
 * đúng: một chuỗi trông-như-thật trong test là một chuỗi có ngày bị chép sang chỗ khác.
 */

const MAT_KHAU_GIA = "MK-GIA-DE-TEST";

/** Một `fetch` giả ghi lại lời gọi. `header` cho ca kiểm dựng phản hồi mang `Idempotent-Replay`. */
function ghiGia(ma: number, than: unknown, header: Record<string, string> = {}) {
  const gia = vi.fn(
    async () =>
      new Response(than === null ? null : JSON.stringify(than), {
        status: ma,
        headers: than === null ? header : { "Content-Type": "application/json", ...header },
      }),
  );
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof ghiGia>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

const DONG_DANH_BA = {
  id: "01J000000000000000000001",
  code: "CB001",
  full_name: "Huỳnh Văn A",
  email: "demo@thangbinh.test",
  position: "Chuyên viên",
  department_id: "01J0000000000000000BOPHAN",
  role_id: "01J00000000000000000VAITRO",
  phone: "02350000000",
  mobile: "0900000000",
  has_account: true,
  active: true,
  last_login_at: null,
  created_at: "2026-09-22T08:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/v1/staff/{id}/account — cấp tài khoản", () => {
  it("KHÔNG gửi thân và KHÔNG khai `Content-Type` — hợp đồng khai `than: never`", async () => {
    // Câu mở #9: hệ thống SINH mật khẩu, quản trị viên không đặt. Một thân ở đây là chỗ để một
    // mật khẩu do client chọn đi lên — thứ #9 vừa từ chối.
    const gia = ghiGia(201, { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA });
    await capTaiKhoan(DONG_DANH_BA.id);

    const { tuyChon, header } = loiGoi(gia, 0);
    expect(tuyChon.method).toBe("POST");
    expect(tuyChon.body).toBeUndefined();
    expect(header.get("Content-Type")).toBeNull();
  });

  it("KHÔNG mang `Idempotency-Key` — máy chủ chặn bằng `AND NOT co_tai_khoan`, không bằng khoá", async () => {
    // VẾ CHỊU LỰC. Thêm một khoá chống trùng ở đây trông như cẩn thận hơn, mà thực ra YẾU hơn:
    // khoá theo biểu mẫu không chặn được hai quản trị viên khác nhau cùng bấm, còn điều kiện
    // trong WHERE thì chặn. Hợp đồng khai `idempotency: khong-can` đúng vì lý do ấy.
    const gia = ghiGia(201, { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA });
    await capTaiKhoan(DONG_DANH_BA.id);

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBeNull();
  });

  it("id vào đường dẫn ĐÃ MÃ HOÁ, không ghép thẳng", async () => {
    const gia = ghiGia(201, { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA });
    await capTaiKhoan("a/b?c=d");

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/staff/a%2Fb%3Fc%3Dd/account");
  });

  it("201 trả mật khẩu tạm; 409 trả NGUYÊN VĂN câu của máy chủ", async () => {
    const gia = ghiGia(201, { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA });
    const ok = await capTaiKhoan(DONG_DANH_BA.id);
    expect(ok).toEqual({
      ok: true,
      duLieu: { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA },
    });
    expect(gia).toHaveBeenCalledTimes(1);

    // 409 ở tuyến này có nghĩa cụ thể: người đó ĐÃ có tài khoản, việc cần làm là ĐẶT LẠI.
    ghiGia(409, { code: "conflict", message: "Cán bộ này đã có tài khoản đăng nhập." });
    expect(await capTaiKhoan(DONG_DANH_BA.id)).toEqual({
      ok: false,
      thongBao: "Cán bộ này đã có tài khoản đăng nhập.",
    });
  });
});

describe("PUT /api/v1/staff/{id}/password — quản trị viên đặt lại hộ (#17)", () => {
  it("dùng LẠI đúng khoá chống trùng truyền vào, không sinh khoá mới mỗi lần gọi", async () => {
    // ĐÂY LÀ CA ĐẮT NHẤT CỦA TUYẾN NÀY. Nếu hàm tự sinh khoá bên trong thì mỗi lần bấm lại sau
    // một lỗi mạng là một khoá MỚI, tức sinh một mật khẩu tạm KHÁC và vô hiệu cái trước — quản
    // trị viên vừa đọc một giá trị cho cán bộ qua điện thoại, bấm lại vì màn hình trông như chưa
    // xong, và giá trị vừa đọc hết hiệu lực mà không ai hiểu vì sao.
    const gia = ghiGia(200, { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA });
    await datLaiMatKhau(DONG_DANH_BA.id, "khoa-cua-bieu-mau");
    await datLaiMatKhau(DONG_DANH_BA.id, "khoa-cua-bieu-mau");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("khoa-cua-bieu-mau");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("khoa-cua-bieu-mau");
    expect(loiGoi(gia, 0).tuyChon.method).toBe("PUT");
    expect(loiGoi(gia, 0).tuyChon.body).toBeUndefined();
  });

  it("MỘT LẦN PHÁT LẠI CHỐNG TRÙNG KHÔNG PHẢI THÀNH CÔNG, dù HTTP là 200", async () => {
    // `core/idem` CỐ Ý không lưu thân của lần trả lời đầu (`idem.go:421-422`), nên bản phát lại
    // về với đúng mã 200 và thân `{"code":…,"replayed":true}` — không có mật khẩu nào.
    //
    // BA VẾ, VÀ VẾ THỨ BA LÀ VẾ CHỊU LỰC. `ok` phải là `false`; câu phải MỜI ĐẶT LẠI chứ không
    // nói "thất bại"; và câu KHÔNG được mời bấm lại — bấm lại với cùng khoá chỉ lấy về đúng bản
    // phát lại rỗng này thêm một lần nữa, một vòng lặp người dùng không tự thoát được.
    ghiGia(200, { code: "CB001", replayed: true }, { "Idempotent-Replay": "true" });
    const kq = await datLaiMatKhau(DONG_DANH_BA.id, "khoa-cua-bieu-mau");

    expect(kq.ok).toBe(false);
    if (kq.ok) throw new Error("không tới đây");
    expect(kq.thongBao).toMatch(/Đặt lại mật khẩu/);
    expect(kq.thongBao).toMatch(/KHÁC/);
    expect(kq.thongBao).not.toMatch(/bấm lại/i);
  });

  it("phát lại KHÔNG bao giờ lọt ra ngoài dưới dạng `ok: true` với mật khẩu rỗng", async () => {
    // Ca ĐỐI XỨNG của ca trên, và là ca giữ cho phép kiểm header không bị thay bằng một phép
    // đoán hình dạng thân. `.json()` của bản phát lại chạy trót lọt và TypeScript ép kiểu sạch,
    // nên không có `tsc` nào đỏ: bên gọi nhận `ok: true` với `staff` và `temporary_password` đều
    // `undefined`, rồi `kq.duLieu.staff.full_name` ném `TypeError` giữa `then` — không ai bắt,
    // màn hình đứng im, và mật khẩu thì đã mất từ lần gọi đầu.
    ghiGia(200, { code: "CB001", replayed: true }, { "Idempotent-Replay": "true" });
    const kq = await datLaiMatKhau(DONG_DANH_BA.id, "khoa-cua-bieu-mau");

    expect(kq).not.toHaveProperty("duLieu");
  });
});

describe("PUT /api/v1/staff/current/password — tự đổi", () => {
  it("gửi ĐÚNG HAI khoá; ô nhập lại của client KHÔNG đi lên", async () => {
    // Hợp đồng có hai trường. Ô "nhập lại" tồn tại vì một lần gõ nhầm mật khẩu mới là một tài
    // khoản không đăng nhập lại được — nó là phép kiểm của client, không phải dữ liệu của máy chủ.
    const gia = ghiGia(204, null);
    await tuDoiMatKhau({ current_password: "cu-gia", new_password: "moi-gia" });

    const { tuyChon } = loiGoi(gia, 0);
    expect(tuyChon.method).toBe("PUT");
    expect(JSON.parse(String(tuyChon.body))).toEqual({
      current_password: "cu-gia",
      new_password: "moi-gia",
    });
    expect(Object.keys(JSON.parse(String(tuyChon.body)))).toHaveLength(2);
  });

  it("KHÔNG có id nào trong đường dẫn — `current` lấy từ phiên", async () => {
    // Một tham số định danh trên tuyến đổi mật khẩu là đúng hình dạng luật 4 cấm #1, chỉ đổi chỗ
    // sang phía cán bộ: đổi một chuỗi trong URL là đổi mật khẩu người khác.
    const gia = ghiGia(204, null);
    await tuDoiMatKhau({ current_password: "cu-gia", new_password: "moi-gia" });

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/staff/current/password");
  });

  it("204 KHÔNG THÂN là THÀNH CÔNG, không phải 'không đọc được'", async () => {
    // Một hàm dùng chung luôn gọi `.json()` sẽ biến lần đổi mật khẩu thành công thành một lỗi —
    // và người dùng sẽ đổi lại bằng mật khẩu cũ, thứ vừa hết hiệu lực.
    ghiGia(204, null);
    expect(await tuDoiMatKhau({ current_password: "cu-gia", new_password: "moi-gia" })).toEqual({
      ok: true,
      duLieu: undefined,
    });
  });

  it("máy chủ từ chối thì câu của máy chủ ra NGUYÊN VĂN", async () => {
    ghiGia(400, { code: "bad_request", message: "Mật khẩu hiện tại không đúng." });
    expect(await tuDoiMatKhau({ current_password: "sai-gia", new_password: "moi-gia" })).toEqual({
      ok: false,
      thongBao: "Mật khẩu hiện tại không đúng.",
    });
  });
});

describe("không một tuyến nào tự khai xã, và không một tuyến nào đi tới một host", () => {
  it("ba đường dẫn đều TƯƠNG ĐỐI và không mang chữ `tenant`", async () => {
    // Luật 1 cấm #2: client tự khai xã là client tự cấp quyền. Và một host nung vào bundle là
    // một bản dựng chỉ đúng cho một xã (luật 1, bất biến 10).
    const gia = ghiGia(200, { staff: DONG_DANH_BA, temporary_password: MAT_KHAU_GIA });
    await capTaiKhoan(DONG_DANH_BA.id);
    await datLaiMatKhau(DONG_DANH_BA.id, "khoa-cua-bieu-mau");

    const gia2 = ghiGia(204, null);
    await tuDoiMatKhau({ current_password: "cu-gia", new_password: "moi-gia" });

    for (const d of [loiGoi(gia, 0).duongDan, loiGoi(gia, 1).duongDan, loiGoi(gia2, 0).duongDan]) {
      expect(d.startsWith("/api/v1/")).toBe(true);
      expect(d).not.toMatch(/tenant/i);
      expect(d).not.toMatch(/^https?:/);
    }
  });
});
