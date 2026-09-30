import { afterEach, describe, expect, it, vi } from "vitest";

import { DUONG_DAN_SAU_KHI_DOI, doiMatKhau, loiTaiCho } from "./doi-mat-khau";

/**
 * NĂM TÍNH CHẤT TỆP NÀY CANH, và mỗi tính chất đều là thứ một lần sửa MỘT DÒNG phá được mà không
 * phép kiểm nào khác thấy — `tsc` vẫn sạch, màn hình vẫn dựng, mọi ca khác vẫn xanh:
 *
 *   1. Hai ô mật khẩu mới lệch nhau thì CHẶN TẠI CLIENT và KHÔNG GỌI MẠNG. Vế chịu lực là vế thứ
 *      hai, nên nó được kiểm bằng cách ĐẾM số lần `fetch` bị gọi chứ không bằng câu thông báo.
 *   2. Ô "nhập lại" KHÔNG đi lên máy chủ: thân gửi đi có ĐÚNG hai khoá.
 *   3. Ô mật khẩu hiện tại là BẮT BUỘC. Ca này tồn tại để ĐỎ khi ai đó bỏ ô ấy đi cho gọn —
 *      đề xuất sẽ tới, vì ở màn bắt đổi lần đầu nó trông như một lần gõ thừa (câu mở #18).
 *   4. 204 thì đưa người dùng về màn đăng nhập. Không làm thế thì trang kế tiếp nhận 401 và
 *      trông như hệ thống hỏng.
 *   5. Máy chủ từ chối thì câu của máy chủ ra tới nơi gọi, NGUYÊN VĂN.
 *
 * KHÔNG CÓ MẬT KHẨU NÀO TRÔNG NHƯ THẬT TRONG TỆP NÀY. Các giá trị dưới đây được đặt tên đúng vai
 * của chúng trong ca kiểm và không phải là chuỗi ai đó có thể gõ vào đâu đó.
 */

// Đủ dài để đi qua ngưỡng 12 ký tự của máy chủ, và hiển nhiên là chuỗi mẫu chứ không phải một
// mật khẩu. Ba giá trị KHÁC HẲN NHAU: hai giá trị giống nhau thì một lần lẫn trường vẫn cho ra
// cùng một thân, và ca kiểm xanh mà không chứng minh gì.
const HIEN_TAI = "gia-tri-mau-hien-tai";
const MOI = "gia-tri-mau-moi-hoan-toan";
const LECH = "gia-tri-mau-go-nham";

/**
 * `fetch` giả, khai ĐỦ HAI THAM SỐ dù thân hàm không dùng tới chúng: chữ ký ấy là thứ làm
 * `goi.mock.calls[0]` mang kiểu `[string, RequestInit]`, nên ca kiểm đọc được đường dẫn và thân
 * đã gửi mà không phải ép kiểu qua `unknown` — một phép ép như thế sẽ nuốt luôn ngày chữ ký
 * `goiGhi` đổi.
 */
function mayChuGia(tra: () => Response) {
  return vi.fn(async (_duongDan: string, _tuyChon: RequestInit) => tra());
}

/**
 * Đối số của lần gọi đầu tiên — NÉM nếu chưa có lần gọi nào.
 *
 * Ném chứ không trả một giá trị dự phòng: một `?? {}` ở đây biến ca "chưa từng gọi mạng" thành
 * một ca đọc thân rỗng rồi so với hai khoá và ĐỎ vì lý do sai — hoặc tệ hơn, XANH.
 */
function doiSoLanDau(goi: ReturnType<typeof mayChuGia>): [string, RequestInit] {
  const doiSo = goi.mock.calls[0];
  if (doiSo === undefined) throw new Error("fetch chưa được gọi lần nào");
  return doiSo;
}

function than(goi: ReturnType<typeof mayChuGia>): Record<string, unknown> {
  return JSON.parse(String(doiSoLanDau(goi)[1].body)) as Record<string, unknown>;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("phép kiểm tại client", () => {
  it("hai ô mật khẩu mới lệch nhau thì KHÔNG có lời gọi mạng nào", async () => {
    const goi = vi.fn();
    vi.stubGlobal("fetch", goi);
    const diTiep = vi.fn();

    const ketCuc = await doiMatKhau(
      { hienTai: HIEN_TAI, moi: MOI, nhapLai: LECH },
      diTiep,
    );

    // VẾ CHỊU LỰC: không phải "có thông báo", mà là "KHÔNG gọi mạng". Một bản cài gửi lên rồi
    // đợi máy chủ từ chối cũng hiện ra một câu — và nó đã đưa một giá trị người dùng không định
    // đặt đi qua mạng, vào mọi tầng ghi log trên đường.
    expect(goi).toHaveBeenCalledTimes(0);
    expect(diTiep).toHaveBeenCalledTimes(0);
    expect(ketCuc.pha).toBe("loi");
  });

  it("ô mật khẩu HIỆN TẠI để trống thì bị chặn tại client, không gọi mạng", async () => {
    const goi = vi.fn();
    vi.stubGlobal("fetch", goi);
    const diTiep = vi.fn();

    // Đây đúng là hình dạng của màn BẮT ĐỔI LẦN ĐẦU khi ai đó bỏ ô "mật khẩu hiện tại" đi: hai ô
    // mới khớp nhau, hợp lệ về mọi mặt, và chỉ thiếu bằng chứng rằng người đang ngồi đây biết
    // mật khẩu hiện thời. Máy ở bộ phận một cửa là máy DÙNG CHUNG (#18), nên "trình duyệt này
    // đang giữ một phiên" và "người này biết mật khẩu" thường xuyên là hai người khác nhau.
    const ketCuc = await doiMatKhau({ hienTai: "", moi: MOI, nhapLai: MOI }, diTiep);

    expect(goi).toHaveBeenCalledTimes(0);
    expect(ketCuc).toEqual({ pha: "loi", thongBao: expect.any(String) });
  });

  it("mật khẩu mới ngắn hơn ngưỡng của máy chủ thì được nhắc trước", () => {
    // Ngưỡng thật nằm ở `service-identity/internal/domain/mat_khau.go` (`DaiMatKhauToiThieu`).
    // Ở đây chỉ kiểm rằng client CÓ nhắc; câu từ chối cuối cùng vẫn là của máy chủ.
    expect(loiTaiCho({ hienTai: HIEN_TAI, moi: "ngan", nhapLai: "ngan" })).not.toBeNull();
  });

  it("mật khẩu mới trùng mật khẩu hiện tại thì bị từ chối", () => {
    expect(loiTaiCho({ hienTai: HIEN_TAI, moi: HIEN_TAI, nhapLai: HIEN_TAI })).not.toBeNull();
  });

  it("ba ô hợp lệ thì không có gì để nói", () => {
    expect(loiTaiCho({ hienTai: HIEN_TAI, moi: MOI, nhapLai: MOI })).toBeNull();
  });
});

describe("lời gọi đi lên máy chủ", () => {
  it("thân gửi đi có ĐÚNG hai khoá — ô nhập lại ở lại trình duyệt", async () => {
    const goi = mayChuGia(() => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", goi);

    await doiMatKhau({ hienTai: HIEN_TAI, moi: MOI, nhapLai: MOI }, () => {});

    expect(goi).toHaveBeenCalledTimes(1);

    const gui = than(goi);
    // Đếm khoá, không chỉ kiểm hai khoá có mặt: `toContain` vẫn xanh khi một khoá thứ ba lọt
    // vào, và một bản sao mật khẩu dưới tên khác đi lên máy chủ chính là ca phải đỏ.
    expect(Object.keys(gui).sort()).toEqual(["current_password", "new_password"]);
    expect(gui.current_password).toBe(HIEN_TAI);
    expect(gui.new_password).toBe(MOI);
  });

  it("gọi đúng tuyến `current`, KHÔNG có định danh nào trên đường dẫn", async () => {
    const goi = mayChuGia(() => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", goi);

    await doiMatKhau({ hienTai: HIEN_TAI, moi: MOI, nhapLai: MOI }, () => {});

    // `current` là của máy chủ, lấy từ phiên. Một `id` ở đây là đổi một chuỗi trong URL để đổi
    // mật khẩu người khác.
    const doiSo = doiSoLanDau(goi);
    expect(doiSo[0]).toBe("/api/v1/staff/current/password");
    expect(doiSo[1].method).toBe("PUT");
  });

  it("204 thì đưa người dùng về màn đăng nhập", async () => {
    vi.stubGlobal("fetch", mayChuGia(() => new Response(null, { status: 204 })));
    const diTiep = vi.fn();

    const ketCuc = await doiMatKhau({ hienTai: HIEN_TAI, moi: MOI, nhapLai: MOI }, diTiep);

    // Máy chủ thu hồi MỌI phiên của người này, kể cả phiên vừa gọi. Ở lại ứng dụng nghĩa là
    // trang kế tiếp nhận 401, ngay sau khi người dùng vừa làm một việc đúng.
    expect(ketCuc).toEqual({ pha: "xong" });
    expect(diTiep).toHaveBeenCalledTimes(1);
    expect(diTiep).toHaveBeenCalledWith(DUONG_DAN_SAU_KHI_DOI);
    expect(DUONG_DAN_SAU_KHI_DOI).toBe("/dang-nhap");
  });

  it("máy chủ từ chối thì câu CỦA MÁY CHỦ đi ra nguyên văn, và không rời trang", async () => {
    // Đúng hình dạng `httpx.Error` của kho, với câu máy chủ viết cho ca mật khẩu hiện tại sai.
    const cuaMayChu = "Mật khẩu hiện tại không đúng.";
    vi.stubGlobal(
      "fetch",
      mayChuGia(
        () =>
          new Response(
            JSON.stringify({
              code: "invalid_credentials",
              message: cuaMayChu,
              trace_id: "01JTRACE",
            }),
            { status: 400, headers: { "Content-Type": "application/json" } },
          ),
      ),
    );
    const diTiep = vi.fn();

    const ketCuc = await doiMatKhau({ hienTai: HIEN_TAI, moi: MOI, nhapLai: MOI }, diTiep);

    expect(ketCuc).toEqual({ pha: "loi", thongBao: cuaMayChu });
    expect(diTiep).toHaveBeenCalledTimes(0);

    // Không viết lại, không thêm mã lỗi, không thêm `trace_id`, không thêm số hiệu HTTP.
    if (ketCuc.pha !== "loi") throw new Error("ca này phải là nhánh lỗi");
    expect(ketCuc.thongBao).not.toContain("01JTRACE");
    expect(ketCuc.thongBao).not.toContain("invalid_credentials");
    expect(ketCuc.thongBao).not.toContain("400");
  });
});
