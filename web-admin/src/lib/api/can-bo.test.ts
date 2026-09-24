import { afterEach, describe, expect, it, vi } from "vitest";

import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  TRANG_DAU,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";

import {
  TU_KHOA_TIM_TOI_DA,
  chuanHoaTuKhoaTim,
  datCongKhaiCanBo,
  datKhoaCanBo,
  docTrangDanhBa,
  doiVaiTroCanBo,
  duongDanDanhSachCanBo,
  layChiTietCanBo,
  layDanhSachCanBo,
  suaCanBo,
  thanCongKhai,
  thanTimCanBo,
  themCanBo,
  timCanBo,
  type TuKhoaHopLe,
} from "./can-bo";
import type { identity_canBoTomTat } from "./schema.gen";

/**
 * VÌ SAO CÓ MỘT MÁY CHỦ GIẢ CHỨ KHÔNG PHẢI MỘT `fetch` TRẢ SẴN MỘT TRANG:
 *
 *   Thứ đáng kiểm ở đây là **đi qua nhiều trang mà không lặp, không sót** — một tính chất chỉ
 *   xuất hiện khi có một thứ ở đầu kia thật sự đọc con trỏ và trả trang kế tiếp. Một `fetch`
 *   trả sẵn `{items, next_cursor}` cố định thì test xanh kể cả khi client gửi sai con trỏ, gửi
 *   `offset`, hay bỏ luôn con trỏ — tức là test cái mock, không test cái client.
 *
 *   Máy chủ giả dưới đây vì vậy **từ chối** đúng những thứ máy chủ thật từ chối: `offset` (tham
 *   số không tồn tại), khoá sắp xếp ngoài danh sách trắng, và `cursor=` rỗng. Cả ba trả 400
 *   đúng hình dạng `httpx.Error`, nên một lần client dựng sai URL là một test đỏ.
 */

/**
 * Dữ liệu mẫu.
 *
 * HAI SỐ, KHÔNG CHE — và câu chú thích cũ ở đây ("đã che đúng như máy chủ trả — `09****0000`") nay
 * SAI. Câu mở #11 được khách chốt 22/09/2026: cán bộ cùng một xã KHÔNG bị che số của nhau, vì họ
 * phải gọi nhau để làm việc; che thì họ truyền số qua kênh riêng và cơ quan mất cả vết lẫn quyền
 * kiểm soát. Máy chủ nay trả số nguyên vẹn trên bề mặt nội bộ
 * (`service-identity/internal/http/can_bo.go`, `soRaManHinhNoiBo`).
 *
 * HAI TRƯỜNG RIÊNG vì hai địa vị pháp lý khác nhau (#16): `phone` là máy bàn cơ quan — thông tin
 * công vụ; `mobile` là di động cá nhân — dữ liệu cá nhân theo Nghị định 13. Vật mẫu dùng hai đầu
 * số KHÁC HẲN NHAU, để một lần lẫn cột là một ca đỏ chứ không phải một ca xanh trùng giá trị.
 *
 * Số ở đây là số giả đã thoả thuận của kho (luật 3, bất biến 5), không phải số của người thật.
 */
function canBo(n: number, dangNhapGanNhat: string | null): identity_canBoTomTat {
  const so = String(n).padStart(3, "0");
  return {
    id: `01J00000000000000000000${so}`,
    code: `CB${so}`,
    full_name: `Huỳnh Văn ${n}`,
    email: `demo${n}@thangbinh.test`,
    position: "Chuyên viên chuyên môn",
    department_id: "01J0000000000000000BOPHAN",
    role_id: "01J00000000000000000VAITRO",
    phone: "02350000000",
    mobile: "0900000000",
    has_account: true,
    active: true,
    last_login_at: dangNhapGanNhat,
    created_at: `2026-09-${String(n).padStart(2, "0")}T04:10:38Z`,
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
  };
}

const BAY_NGUOI = [1, 2, 3, 4, 5, 6, 7].map((n) => canBo(n, null));

function loi400(ma: string, thongBao: string) {
  return new Response(JSON.stringify({ code: ma, message: thongBao, trace_id: "01JTRACE" }), {
    status: 400,
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Máy chủ giả: đọc theo mốc trên `code`, đúng như `service-identity` làm.
 *
 * Con trỏ ở đây là `code` của dòng cuối vừa phát ra, và nó **mờ đục với client**: không chỗ nào
 * trong mã ứng dụng dựng hay đọc giá trị ấy.
 */
function mayChuGia(tatCa: readonly identity_canBoTomTat[] = BAY_NGUOI) {
  const gia = vi.fn(async (duongDan: string) => {
    const url = new URL(duongDan, "https://mot-xa.test");

    const motNguoi = /^\/api\/v1\/staff\/(.+)$/.exec(url.pathname);
    if (motNguoi !== null) {
      const id = decodeURIComponent(motNguoi[1] ?? "");
      const cb = tatCa.find((x) => x.id === id);
      if (cb === undefined) {
        return new Response(
          JSON.stringify({
            code: "staff_not_found",
            message: "Không tìm thấy cán bộ.",
            trace_id: "01JTRACE",
          }),
          { status: 404, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response(JSON.stringify(cb), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }

    if (url.pathname !== "/api/v1/staff") return new Response(null, { status: 404 });

    if (url.searchParams.has("offset") || url.searchParams.has("page")) {
      return loi400("invalid_limit", "Số bản ghi mỗi trang không hợp lệ.");
    }

    const sort = url.searchParams.get("sort") ?? "code";
    if (sort !== "code" && sort !== "created_at") {
      return loi400("invalid_sort", "Tiêu chí sắp xếp không hợp lệ.");
    }

    const order = url.searchParams.get("order") ?? "asc";
    const limit = Number(url.searchParams.get("limit") ?? "20");

    if (url.searchParams.has("cursor") && url.searchParams.get("cursor") === "") {
      return loi400("invalid_cursor", "Con trỏ phân trang không hợp lệ. Vui lòng tải lại danh sách.");
    }

    const xepTheo = [...tatCa].sort((a, b) =>
      order === "asc" ? a.code.localeCompare(b.code) : b.code.localeCompare(a.code),
    );
    const conTro = url.searchParams.get("cursor");
    const batDau =
      conTro === null ? 0 : xepTheo.findIndex((x) => x.code === conTro) + 1;
    if (conTro !== null && batDau === 0) {
      return loi400("invalid_cursor", "Con trỏ phân trang không hợp lệ. Vui lòng tải lại danh sách.");
    }

    const items = xepTheo.slice(batDau, batDau + limit);
    const conNua = batDau + limit < xepTheo.length;
    const cuoi = items[items.length - 1];
    return new Response(
      JSON.stringify({
        items,
        next_cursor: conNua && cuoi !== undefined ? cuoi.code : "",
        has_more: conNua,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("dựng URL truy vấn", () => {
  it("không tham số nào thì không có dấu hỏi — để máy chủ áp mặc định của nó", () => {
    expect(duongDanDanhSachCanBo()).toBe("/api/v1/staff");
  });

  it("limit · sort · order · cursor vào đúng tên tham số của hợp đồng", () => {
    expect(duongDanDanhSachCanBo({ limit: 3, sort: "created_at", order: "desc", cursor: "MOC" })).toBe(
      "/api/v1/staff?limit=3&sort=created_at&order=desc&cursor=MOC",
    );
  });

  it("con trỏ rỗng hoặc null KHÔNG xuất hiện — `cursor=` rỗng bị máy chủ trả 400", () => {
    expect(duongDanDanhSachCanBo({ cursor: null })).toBe("/api/v1/staff");
    expect(duongDanDanhSachCanBo({ cursor: "" })).toBe("/api/v1/staff");
  });

  it("con trỏ được mã hoá, không ghép thẳng vào chuỗi truy vấn", () => {
    expect(duongDanDanhSachCanBo({ cursor: "a b&c=d" })).toBe("/api/v1/staff?cursor=a+b%26c%3Dd");
  });

  it("đường dẫn tương đối, không host nào bị nung vào bundle", () => {
    expect(duongDanDanhSachCanBo({ limit: 20 })).not.toMatch(/^https?:/);
  });

  it("không có `offset`, `page` hay `total` ở bất kỳ tham số nào", () => {
    // Máy chủ đọc theo mốc và cố ý không đếm tổng số. Một `offset` bịa ra chỉ hỏng lúc chạy, ở
    // trang thứ hai.
    const duong = duongDanDanhSachCanBo({ limit: 50, sort: "code", order: "asc", cursor: "X" });
    expect(duong).not.toMatch(/offset|[?&]page=|total/);
  });

  it("không chữ `tenant` nào trong URL — client tự khai xã là client tự cấp quyền", () => {
    const duong = duongDanDanhSachCanBo({ limit: 20, sort: "code", cursor: "X" });
    expect(duong).not.toMatch(/tenant|commune|\bxa_id\b/i);
  });
});

describe("hình dạng lời gọi", () => {
  it("gọi GET, cookie đi kèm bằng credentials same-origin", async () => {
    const gia = mayChuGia();
    await layDanhSachCanBo({ limit: 3 });

    const [duongDan, tuyChon] = gia.mock.calls[0] as unknown as [string, RequestInit];
    expect(duongDan).toBe("/api/v1/staff?limit=3");
    expect(tuyChon.method).toBe("GET");
    expect(tuyChon.credentials).toBe("same-origin");
    expect(tuyChon.cache).toBe("no-store");
  });

  it("máy chủ giả từ chối `offset` — chốt rằng client không bao giờ gửi tham số ấy", async () => {
    mayChuGia();
    // Gọi thẳng qua fetch để chứng minh máy chủ giả có răng: nếu nó nhận `offset` thì bài test
    // "đi nhiều trang" ở dưới sẽ xanh cả khi client phân trang bằng offset.
    const phanHoi = await fetch("/api/v1/staff?offset=20");
    expect(phanHoi.status).toBe(400);
  });
});

describe("danh sách rỗng và danh sách lỗi là hai chuyện khác nhau", () => {
  it("xã mới onboard: items rỗng, has_more false, và đó là một kết quả THÀNH CÔNG", async () => {
    mayChuGia([]);
    const ketQua = await layDanhSachCanBo();

    expect(ketQua.ok).toBe(true);
    if (ketQua.ok) {
      expect(ketQua.duLieu.items).toEqual([]);
      expect(ketQua.duLieu.has_more).toBe(false);
      expect(ketQua.duLieu.next_cursor).toBe("");
    }
  });

  it("403 trả về `message` của máy chủ, không rẽ nhánh theo `code`, không lộ `trace_id`", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              code: "forbidden",
              message: "Bạn không có quyền thực hiện thao tác này.",
              trace_id: "01JTRACE0000000000000000",
            }),
            { status: 403, headers: { "Content-Type": "application/json" } },
          ),
      ),
    );

    const ketQua = await layDanhSachCanBo();
    expect(ketQua).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
    if (!ketQua.ok) expect(ketQua.thongBao).not.toMatch(/01JTRACE/);
  });

  it("mạng hỏng thì một câu chung, không chi tiết kỹ thuật", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    const ketQua = await layDanhSachCanBo();
    expect(ketQua).toEqual({ ok: false, thongBao: "Không kết nối được máy chủ. Vui lòng thử lại." });
  });
});

describe("đi qua nhiều trang bằng con trỏ — không lặp, không sót", () => {
  /** Đọc một trang theo đúng cách màn hình đọc: chỉ sort/order/cursor, không offset. */
  async function doc(nganXep: NganXepConTro) {
    const ketQua = await layDanhSachCanBo({ limit: 3, sort: "code", cursor: nganXep.hienTai });
    if (!ketQua.ok) throw new Error(`đọc trang thất bại: ${ketQua.thongBao}`);
    return ketQua.duLieu;
  }

  it("7 người, mỗi trang 3: đi xuôi hết danh sách, đúng thứ tự, không dòng nào hai lần", async () => {
    mayChuGia();

    let nganXep = TRANG_DAU;
    const daThay: string[] = [];
    const soTrang: string[][] = [];

    for (let i = 0; i < 10; i++) {
      const trang = await doc(nganXep);
      soTrang.push(trang.items.map((x) => x.code));
      daThay.push(...trang.items.map((x) => x.code));
      if (!trang.has_more) break;
      nganXep = sangTrangSau(nganXep, trang.next_cursor);
    }

    expect(soTrang).toEqual([["CB001", "CB002", "CB003"], ["CB004", "CB005", "CB006"], ["CB007"]]);
    expect(daThay).toEqual(BAY_NGUOI.map((x) => x.code));
    expect(new Set(daThay).size).toBe(daThay.length);
  });

  it("đi xuôi rồi lùi: mỗi trang lùi ra đúng trang đã thấy lúc đi xuôi", async () => {
    mayChuGia();

    let nganXep = TRANG_DAU;
    const xuoi: string[][] = [];
    const xepLuu: NganXepConTro[] = [];

    for (let i = 0; i < 10; i++) {
      xepLuu.push(nganXep);
      const trang = await doc(nganXep);
      xuoi.push(trang.items.map((x) => x.code));
      if (!trang.has_more) break;
      nganXep = sangTrangSau(nganXep, trang.next_cursor);
    }

    // Ở trang cuối thì lùi được; ở trang đầu thì không — nút "Trang trước" phải mờ đi.
    expect(coTrangTruoc(nganXep)).toBe(true);

    const lui: string[][] = [];
    while (coTrangTruoc(nganXep)) {
      nganXep = veTrangTruoc(nganXep);
      const trang = await doc(nganXep);
      lui.push(trang.items.map((x) => x.code));
    }

    expect(nganXep).toEqual(TRANG_DAU);
    expect(coTrangTruoc(nganXep)).toBe(false);
    expect(lui).toEqual([...xuoi].reverse().slice(1));
    expect(xepLuu[0]).toEqual(TRANG_DAU);
  });

  it("sắp xếp giảm dần đi hết đúng danh sách theo chiều ngược", async () => {
    mayChuGia();

    let nganXep = TRANG_DAU;
    const daThay: string[] = [];
    for (let i = 0; i < 10; i++) {
      const ketQua = await layDanhSachCanBo({
        limit: 2,
        sort: "code",
        order: "desc",
        cursor: nganXep.hienTai,
      });
      if (!ketQua.ok) throw new Error(ketQua.thongBao);
      daThay.push(...ketQua.duLieu.items.map((x) => x.code));
      if (!ketQua.duLieu.has_more) break;
      nganXep = sangTrangSau(nganXep, ketQua.duLieu.next_cursor);
    }

    expect(daThay).toEqual([...BAY_NGUOI.map((x) => x.code)].reverse());
  });
});

describe("chi tiết một cán bộ", () => {
  it("id vào đường dẫn đã mã hoá, phản hồi giữ nguyên `last_login_at: null`", async () => {
    const gia = mayChuGia();
    const cb = BAY_NGUOI[0];
    if (cb === undefined) throw new Error("dữ liệu mẫu rỗng");

    const ketQua = await layChiTietCanBo(cb.id);
    expect((gia.mock.calls[0] as unknown as [string])[0]).toBe(`/api/v1/staff/${cb.id}`);
    expect(ketQua.ok).toBe(true);
    if (ketQua.ok) expect(ketQua.duLieu.last_login_at).toBeNull();
  });

  it("id lạ trả 404 kèm câu của máy chủ — không đoán thêm 'của xã khác' hay 'đã xoá'", async () => {
    mayChuGia();
    const ketQua = await layChiTietCanBo("01JKHONGTONTAI0000000000");
    expect(ketQua).toEqual({ ok: false, thongBao: "Không tìm thấy cán bộ." });
  });

  it("id có ký tự lạ được mã hoá, không ghép thẳng vào đường dẫn", async () => {
    const gia = mayChuGia();
    await layChiTietCanBo("a/b?c=d");
    expect((gia.mock.calls[0] as unknown as [string])[0]).toBe("/api/v1/staff/a%2Fb%3Fc%3Dd");
  });
});

/* ---- năm tuyến ghi -------------------------------------------------------------------------- */

/** Một `fetch` giả ghi lại lời gọi và trả về mã + thân do ca kiểm chọn. */
function ghiGia(ma: number, than: unknown) {
  const gia = vi.fn(
    async () =>
      new Response(than === null ? null : JSON.stringify(than), {
        status: ma,
        headers: than === null ? {} : { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", gia);
  return gia;
}

/** Tham số của lời gọi thứ `n`, đã ép kiểu một lần cho cả tệp. */
function loiGoi(gia: ReturnType<typeof ghiGia>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

const THAN_THEM = {
  full_name: "Huỳnh Văn A",
  position: "Chuyên viên",
  email: "demo@thangbinh.test",
  org_unit_id: "01J0000000000000000BOPHAN",
  office_phone: "02350000000",
  mobile: "0900000000",
};

describe("POST /api/v1/staff — thêm một dòng danh bạ", () => {
  it("gửi đúng sáu trường của hợp đồng, và KHÔNG có `code`", async () => {
    const gia = ghiGia(201, canBo(1, null));
    await themCanBo(THAN_THEM, "k-co-dinh");

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/staff");
    expect(tuyChon.method).toBe("POST");
    expect(JSON.parse(String(tuyChon.body))).toEqual(THAN_THEM);
  });

  it("mang `Idempotency-Key` đúng khoá truyền vào, và lần gửi lại dùng LẠI khoá ấy", async () => {
    // Sinh khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng cái
    // khoá chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã tạo người.
    const gia = ghiGia(201, canBo(1, null));
    await themCanBo(THAN_THEM, "k-co-dinh");
    await themCanBo(THAN_THEM, "k-co-dinh");

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-co-dinh");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-co-dinh");
  });

  it("một trường lạ truyền vào KHÔNG đi lên máy chủ", async () => {
    // Thân dựng từng trường chứ không `...than`: một phép trải là đường để `code`, `role_id` hay
    // `active` đi lên vào ngày ai đó truyền vào một dòng vừa đọc được từ máy chủ.
    const gia = ghiGia(201, canBo(1, null));
    await themCanBo({ ...THAN_THEM, code: "CB999", role_id: "X" } as never, "k");

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("code");
    expect(than).not.toHaveProperty("role_id");
  });

  it("201 trả về dòng vừa tạo; 409 trả về câu của máy chủ, không rẽ nhánh theo `code`", async () => {
    ghiGia(201, canBo(1, null));
    const tao = await themCanBo(THAN_THEM, "k");
    expect(tao.ok).toBe(true);
    if (tao.ok) expect(tao.duLieu.code).toBe("CB001");

    ghiGia(409, {
      code: "email_taken",
      message: "Thư điện tử này đã được dùng cho một cán bộ khác trong xã.",
      trace_id: "01JTRACE",
    });
    const trung = await themCanBo(THAN_THEM, "k");
    expect(trung).toEqual({
      ok: false,
      thongBao: "Thư điện tử này đã được dùng cho một cán bộ khác trong xã.",
    });
  });

  it("200 KHÔNG được coi là thành công — hợp đồng nói 201", async () => {
    // Một máy chủ hay proxy trả 200 kèm thân khác là chuyện có thật, và coi nó là tạo thành công
    // là báo với cán bộ rằng đã thêm người trong khi chưa có ai được thêm.
    ghiGia(200, canBo(1, null));
    const kq = await themCanBo(THAN_THEM, "k");
    expect(kq.ok).toBe(false);
  });
});

describe("PATCH /api/v1/staff/{id} — sửa hồ sơ", () => {
  it("id vào đường dẫn đã mã hoá, thân mang đúng sáu trường", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await suaCanBo("a/b", {
      full_name: "Huỳnh Văn B",
      position: null,
      email: null,
      org_unit_id: null,
      office_phone: null,
      mobile: "",
    });

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/staff/a%2Fb");
    expect(tuyChon.method).toBe("PATCH");
    // `null` là "không đổi", `""` là "xoá trắng ô này" — hai thứ khác nhau, cả hai phải đi lên
    // nguyên vẹn. `JSON.stringify` giữ `null` và bỏ `undefined`, nên ca này cũng chốt rằng không
    // có chỗ nào đổi `null` thành `undefined` trên đường đi.
    expect(JSON.parse(String(tuyChon.body))).toEqual({
      full_name: "Huỳnh Văn B",
      position: null,
      email: null,
      org_unit_id: null,
      office_phone: null,
      mobile: "",
    });
  });
});

describe("PATCH /api/v1/staff/{id} — `has_zalo` tuỳ chọn", () => {
  const SAU_TRUONG = {
    full_name: "Huỳnh Văn B",
    position: null,
    email: null,
    org_unit_id: null,
    office_phone: null,
    mobile: null,
  };

  it("không đặt `has_zalo` → thân KHÔNG có khoá ấy (vắng = không đổi)", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await suaCanBo("01JABC", SAU_TRUONG);
    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).not.toHaveProperty("has_zalo");
  });

  it("đặt `has_zalo: false` → đi lên `false`, không bị coi là vắng", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await suaCanBo("01JABC", { ...SAU_TRUONG, has_zalo: false });
    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).toHaveProperty("has_zalo", false);
  });

  it("đặt `has_zalo: true` → đi lên `true`", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await suaCanBo("01JABC", { ...SAU_TRUONG, has_zalo: true });
    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).toHaveProperty("has_zalo", true);
  });
});

describe("PUT /api/v1/staff/{id}/publication — công khai một người (#12)", () => {
  it("PUT, id mã hoá vào đường dẫn, thân đúng ba trường", async () => {
    const gia = ghiGia(200, { ...canBo(1, null), published: true });
    await datCongKhaiCanBo("a/b", { congKhai: true, daXacNhanDongY: true, thuTu: 5 });

    const { duongDan, tuyChon, header } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/staff/a%2Fb/publication");
    expect(tuyChon.method).toBe("PUT");
    expect(header.get("Idempotency-Key")).toBeNull();
    expect(JSON.parse(String(tuyChon.body))).toEqual({
      published: true,
      consent_confirmed: true,
      display_order: 5,
    });
  });

  it("`display_order: null` VẪN có mặt trong JSON gửi đi — vắng là xoá, null cũng là xoá, nhưng phải có chủ ý", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await datCongKhaiCanBo("01JABC", { congKhai: false, daXacNhanDongY: false, thuTu: null });
    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual(["consent_confirmed", "display_order", "published"]);
  });

  it("công khai mà CHƯA xác nhận → `consent_confirmed: false`, không bao giờ tự thành true", () => {
    expect(
      thanCongKhai({ congKhai: true, daXacNhanDongY: false, thuTu: null }).consent_confirmed,
    ).toBe(false);
  });

  it("200 trả về dòng sau khi đổi, kèm `consent_recorded_at`", async () => {
    ghiGia(200, { ...canBo(1, null), published: true, consent_recorded_at: "2026-09-24T07:05:00Z" });
    const kq = await datCongKhaiCanBo("01JABC", { congKhai: true, daXacNhanDongY: true, thuTu: null });
    expect(kq.ok && kq.duLieu.consent_recorded_at).toBe("2026-09-24T07:05:00Z");
  });

  it("400 consent_required · 400 invalid_request · 404 staff_not_found — câu máy chủ NGUYÊN VĂN", async () => {
    for (const [ma, code, cau] of [
      [400, "consent_required", "Chưa xác nhận đã hỏi ý và được chính người này đồng ý."],
      [400, "invalid_request", "Thứ tự hiển thị không hợp lệ."],
      [404, "staff_not_found", "Không tìm thấy cán bộ."],
    ] as const) {
      ghiGia(ma, { code, message: cau, trace_id: "01JTRACE" });
      const kq = await datCongKhaiCanBo("01JABC", { congKhai: true, daXacNhanDongY: true, thuTu: null });
      expect(kq).toEqual({ ok: false, thongBao: cau });
    }
  });
});

describe("POST/DELETE /api/v1/staff/{id}/lockout — khoá và mở khoá", () => {
  it("khoá là POST, mở khoá là DELETE, cùng một đường dẫn", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await datKhoaCanBo("01JABC", true);
    await datKhoaCanBo("01JABC", false);

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/staff/01JABC/lockout");
    expect(loiGoi(gia, 0).tuyChon.method).toBe("POST");
    expect(loiGoi(gia, 1).duongDan).toBe("/api/v1/staff/01JABC/lockout");
    expect(loiGoi(gia, 1).tuyChon.method).toBe("DELETE");
  });

  it("KHÔNG gửi thân và KHÔNG khai `Content-Type` — hai tuyến này không nhận thân nào", async () => {
    // Lược đồ không có cột nào giữ lý do khoá, nên một ô lý do sẽ rơi vào vết kiểm toán mà không
    // màn hình nào đọc lại được. Gửi `{}` kèm `Content-Type` là tuyên bố có một thân — thứ sẽ mời
    // người sau điền vào.
    const gia = ghiGia(200, canBo(1, null));
    await datKhoaCanBo("01JABC", true);

    const { tuyChon, header } = loiGoi(gia, 0);
    expect(tuyChon.body).toBeUndefined();
    expect(header.get("Content-Type")).toBeNull();
  });

  it("#13 — 409 'người quản trị cuối cùng' về tới giao diện NGUYÊN VĂN", async () => {
    const cau =
      "Xã phải luôn còn ít nhất một người quản trị. Hãy cấp quyền quản trị cho một cán bộ " +
      "khác trước, rồi thực hiện lại thao tác này.";
    ghiGia(409, { code: "last_admin", message: cau, trace_id: "01JTRACE" });

    const kq = await datKhoaCanBo("01JABC", true);
    expect(kq).toEqual({ ok: false, thongBao: cau });
    // Không lộ mã lỗi, không lộ mốc tra log.
    if (!kq.ok) expect(kq.thongBao).not.toMatch(/last_admin|01JTRACE|409/);
  });
});

describe("PUT /api/v1/staff/{id}/role — đổi vai trò", () => {
  it("gửi `role_id` trong thân, PUT chứ không PATCH", async () => {
    const gia = ghiGia(200, canBo(1, null));
    await doiVaiTroCanBo("01JABC", "01JVAITRO");

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/staff/01JABC/role");
    expect(tuyChon.method).toBe("PUT");
    expect(JSON.parse(String(tuyChon.body))).toEqual({ role_id: "01JVAITRO" });
  });

  it("gỡ vai trò VẪN gửi yêu cầu, với `role_id` rỗng", async () => {
    // `""` là một đích đến hợp lệ ("không giữ vai trò nào"). Một nhánh "rỗng thì thôi không gửi"
    // làm việc gỡ vai trò lặng lẽ không xảy ra, mà màn hình vẫn báo đã lưu.
    const gia = ghiGia(200, canBo(1, null));
    await doiVaiTroCanBo("01JABC", "");

    expect(gia).toHaveBeenCalledTimes(1);
    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).toEqual({ role_id: "" });
  });

  it("#14 — hai lần từ chối khác nhau cho ra HAI câu khác nhau", async () => {
    const tuThaoTac =
      "Không thao tác được lên chính tài khoản của mình. " +
      "Hãy nhờ một người quản trị khác của xã thực hiện.";
    ghiGia(403, { code: "self_target_forbidden", message: tuThaoTac, trace_id: "01JTRACE" });
    expect(await doiVaiTroCanBo("01JABC", "01JVAITRO")).toEqual({
      ok: false,
      thongBao: tuThaoTac,
    });

    const vuotQuyen =
      "Vai trò này mang quyền mà tài khoản của bạn không có, nên bạn không gán được: " +
      "admin.user. Hãy nhờ người có đủ quyền thực hiện.";
    ghiGia(403, { code: "permission_escalation", message: vuotQuyen, trace_id: "01JTRACE" });
    expect(await doiVaiTroCanBo("01JABC", "01JVAITRO")).toEqual({
      ok: false,
      thongBao: vuotQuyen,
    });
  });
});

describe("mọi tuyến ghi: cookie đi kèm, không host, không `tenant_id`", () => {
  it("credentials same-origin, cache no-store, đường dẫn tương đối", async () => {
    const gia = ghiGia(201, canBo(1, null));
    await themCanBo(THAN_THEM, "k");

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(tuyChon.credentials).toBe("same-origin");
    expect(tuyChon.cache).toBe("no-store");
    expect(duongDan).not.toMatch(/^https?:/);
  });

  it("không chữ `tenant` nào trong đường dẫn hay trong thân", async () => {
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã đến từ `Host` của yêu cầu.
    const gia = ghiGia(201, canBo(1, null));
    await themCanBo(THAN_THEM, "k");

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).not.toMatch(/tenant|commune|\bxa_id\b/i);
    expect(String(tuyChon.body)).not.toMatch(/tenant|commune|\bxa_id\b/i);
  });

  it("mạng hỏng thì một câu chung, không chi tiết kỹ thuật", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    expect(await suaCanBo("01JABC", thanSuaRong())).toEqual({
      ok: false,
      thongBao: "Không kết nối được máy chủ. Vui lòng thử lại.",
    });
  });
});

/** Thân PATCH không đổi gì — sáu `null`. Dùng ở ca mạng hỏng, nơi thân không phải thứ đang kiểm. */
function thanSuaRong() {
  return {
    full_name: null,
    position: null,
    email: null,
    org_unit_id: null,
    office_phone: null,
    mobile: null,
  };
}

/* ---- bộ lọc trên URL và tìm theo chữ trong thân --------------------------------------------- */

describe("GET /api/v1/staff — hai bộ lọc `unit` và `published` trên URL", () => {
  it("`unit` và `published` vào đúng tên tham số của hợp đồng", () => {
    const d = duongDanDanhSachCanBo({ boPhan: "01J0000000000000000BOPHAN", congKhai: true });
    const q = new URLSearchParams(d.split("?")[1]);
    expect(d.startsWith("/api/v1/staff?")).toBe(true);
    expect(q.get("unit")).toBe("01J0000000000000000BOPHAN");
    expect(q.get("published")).toBe("true");
  });

  it("`congKhai: false` là một bộ lọc THẬT — gửi `published=false`, không bỏ qua", () => {
    // Gộp `false` với "không lọc" thì ô "Chưa hiện" trả về cả xã.
    expect(duongDanDanhSachCanBo({ congKhai: false })).toBe("/api/v1/staff?published=false");
  });

  it("`null`, vắng, bộ phận rỗng hay toàn khoảng trắng thì KHÔNG có tham số nào", () => {
    expect(duongDanDanhSachCanBo({ congKhai: null, boPhan: "" })).toBe("/api/v1/staff");
    expect(duongDanDanhSachCanBo({ boPhan: "   " })).toBe("/api/v1/staff");
  });

  it("kết hợp với con trỏ: bộ lọc đi cùng trang sau, con trỏ vẫn đứng cuối", () => {
    expect(duongDanDanhSachCanBo({ boPhan: "BP", congKhai: false, cursor: "MOC" })).toBe(
      "/api/v1/staff?unit=BP&published=false&cursor=MOC",
    );
  });

  it("không có đường nào đưa chữ tìm vào URL của tuyến danh sách", () => {
    // @ts-expect-error — `ThamSoTrang` không có trường chữ tìm; thêm vào là `tsc` đỏ ở đây.
    const d = duongDanDanhSachCanBo({ q: "Huỳnh Văn" });
    expect(d).toBe("/api/v1/staff");
  });
});

describe("chuẩn hoá chữ tìm — giống máy chủ, đếm KÝ TỰ", () => {
  it("cắt hai đầu, gộp khoảng trắng bên trong", () => {
    expect(chuanHoaTuKhoaTim("  Huỳnh   Văn \t 1 ")).toEqual({ loai: "hopLe", tu: "Huỳnh Văn 1" });
  });

  it("rỗng hoặc toàn khoảng trắng là `rong` — bỏ tìm, không phải lỗi", () => {
    expect(chuanHoaTuKhoaTim("")).toEqual({ loai: "rong" });
    expect(chuanHoaTuKhoaTim("   \n\t ")).toEqual({ loai: "rong" });
  });

  it("đúng 200 chữ có dấu được nhận — dù đã là 600 byte", () => {
    const tu = "ễ".repeat(TU_KHOA_TIM_TOI_DA);
    expect(new TextEncoder().encode(tu).length).toBe(600);
    expect(chuanHoaTuKhoaTim(tu)).toEqual({ loai: "hopLe", tu });
  });

  it("201 ký tự bị từ chối", () => {
    expect(chuanHoaTuKhoaTim("ễ".repeat(TU_KHOA_TIM_TOI_DA + 1))).toEqual({ loai: "quaDai" });
    expect(chuanHoaTuKhoaTim("a".repeat(TU_KHOA_TIM_TOI_DA + 1))).toEqual({ loai: "quaDai" });
  });

  it("đếm điểm mã, không đếm đơn vị UTF-16 — 200 ký tự ngoài mặt phẳng cơ bản vẫn nhận", () => {
    // "𝐀" là MỘT rune ở máy chủ nhưng `.length` bằng 2. Đếm bằng `.length` là từ chối ở 100 ký tự.
    const tu = "𝐀".repeat(TU_KHOA_TIM_TOI_DA);
    expect(tu.length).toBe(400);
    expect(chuanHoaTuKhoaTim(tu).loai).toBe("hopLe");
    expect(chuanHoaTuKhoaTim(tu + "𝐀").loai).toBe("quaDai");
  });

  it("khoảng trắng thừa không bị tính vào giới hạn — đếm SAU khi gộp, như máy chủ", () => {
    const tu = `  ${"a".repeat(100)}     ${"b".repeat(99)}  `;
    expect(chuanHoaTuKhoaTim(tu).loai).toBe("hopLe");
  });
});

/** Chữ tìm dùng xuyên suốt: có dấu, có khoảng trắng, có ký tự phải mã hoá khi lên URL. */
const CHU_TIM = "Huỳnh Văn & 0900000000";

function hopLe(tho: string): TuKhoaHopLe {
  const tu = chuanHoaTuKhoaTim(tho);
  if (tu.loai !== "hopLe") throw new Error("chữ tìm mẫu phải hợp lệ");
  return tu;
}

/**
 * Mọi hình dạng chữ tìm có thể mang khi lọt lên một URL: nguyên văn, `encodeURIComponent`, và kiểu
 * `URLSearchParams` (dấu cách thành `+`) — cả cụm lẫn từng từ dài. Chỉ kiểm nguyên văn thì một URL
 * đã mã hoá vẫn qua.
 */
function hinhDangTrenUrl(tu: string): string[] {
  const tuDai = tu.split(" ").filter((p) => p.length > 3);
  return [
    tu,
    encodeURIComponent(tu),
    new URLSearchParams({ q: tu }).toString().slice(2),
    ...tuDai.flatMap((p) => [p, encodeURIComponent(p)]),
  ];
}

/**
 * Máy chủ giả cho CẢ HAI tuyến đọc: GET lọc theo URL, POST tìm theo thân. Đọc con trỏ thật, nên đi
 * nhiều trang của một lần tìm chỉ xanh khi con trỏ thật sự đi trong thân. Tuyến tìm trả 405 nếu
 * URL mang bất kỳ chuỗi truy vấn nào.
 */
function mayChuTimGia(tatCa: readonly identity_canBoTomTat[]) {
  function trang(loc: readonly identity_canBoTomTat[], conTro: string, limit: number) {
    const batDau = conTro === "" ? 0 : loc.findIndex((x) => x.code === conTro) + 1;
    if (conTro !== "" && batDau === 0) {
      return loi400("invalid_cursor", "Con trỏ phân trang không hợp lệ. Vui lòng tải lại danh sách.");
    }
    const items = loc.slice(batDau, batDau + limit);
    const conNua = batDau + limit < loc.length;
    return new Response(
      JSON.stringify({
        items,
        next_cursor: conNua ? (items[items.length - 1]?.code ?? "") : "",
        has_more: conNua,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }

  const gia = vi.fn(async (duongDan: string, tuyChon?: RequestInit) => {
    const url = new URL(duongDan, "https://mot-xa.test");
    if (url.pathname === "/api/v1/staff/searches") {
      if (tuyChon?.method !== "POST" || url.search !== "") return new Response(null, { status: 405 });
      const than = JSON.parse(String(tuyChon.body)) as Record<string, unknown>;
      const q = String(than.q ?? "");
      if (q === "") return loi400("invalid_request", "Hãy nhập từ khoá tìm kiếm.");
      const khop = tatCa.filter(
        (x) =>
          (x.full_name.includes(q) || x.mobile.includes(q)) &&
          (than.unit === "" || x.department_id === than.unit) &&
          (than.published === null || x.published === than.published),
      );
      return trang(khop, String(than.cursor ?? ""), 2);
    }
    if (url.pathname === "/api/v1/staff" && (tuyChon?.method ?? "GET") === "GET") {
      const unit = url.searchParams.get("unit");
      const pub = url.searchParams.get("published");
      const khop = tatCa.filter(
        (x) =>
          (unit === null || x.department_id === unit) &&
          (pub === null || String(x.published) === pub),
      );
      return trang(khop, url.searchParams.get("cursor") ?? "", 2);
    }
    return new Response(null, { status: 404 });
  });
  vi.stubGlobal("fetch", gia);
  return gia;
}

/** Năm người tên "Huỳnh Văn …", hai khối, ba người đầu đã hiện trên Mini App. */
const NAM_NGUOI: identity_canBoTomTat[] = [1, 2, 3, 4, 5].map((n) => ({
  ...canBo(n, null),
  full_name: `Huỳnh Văn ${n}`,
  department_id: n % 2 === 0 ? "BP-CHAN" : "BP-LE",
  published: n <= 3,
}));

describe("POST /api/v1/staff/searches — chữ tìm đi trong THÂN", () => {
  it("thân mang đúng năm trường của hợp đồng: q · unit · published · limit · cursor", async () => {
    const gia = ghiGia(200, { items: [], next_cursor: "", has_more: false });
    await timCanBo(hopLe(`  ${CHU_TIM}  `), { boPhan: "BP-LE", congKhai: false, cursor: "MOC" });

    const { duongDan, tuyChon, header } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/staff/searches");
    expect(tuyChon.method).toBe("POST");
    expect(header.get("Content-Type")).toBe("application/json");
    expect(header.get("Idempotency-Key")).toBeNull();
    expect(JSON.parse(String(tuyChon.body))).toEqual({
      q: CHU_TIM,
      unit: "BP-LE",
      published: false,
      limit: null,
      cursor: "MOC",
    });
  });

  it("không bộ lọc, trang đầu: giá trị rỗng đúng nghĩa hợp đồng, không bỏ trường", () => {
    expect(thanTimCanBo(hopLe("abc"))).toEqual({
      q: "abc",
      unit: "",
      published: null,
      limit: null,
      cursor: "",
    });
  });

  it("`published: true` đi lên là giá trị đúng/sai, không phải chuỗi", () => {
    expect(thanTimCanBo(hopLe("abc"), { congKhai: true }).published).toBe(true);
  });

  it("chỉ nhận chữ ĐÃ chuẩn hoá — một chuỗi thô hay một kết quả `quaDai` không qua được `tsc`", () => {
    // @ts-expect-error — nhận `TuKhoaHopLe`, không nhận chuỗi.
    expect(() => thanTimCanBo("abc")).not.toThrow();
    // @ts-expect-error — một chữ quá dài không bao giờ tới được lời gọi mạng.
    expect(() => thanTimCanBo({ loai: "quaDai" })).not.toThrow();
  });

  it("một trường lạ không đi lên máy chủ — thân dựng từng trường", async () => {
    const gia = ghiGia(200, { items: [], next_cursor: "", has_more: false });
    await timCanBo(hopLe("abc"), { boPhan: "BP", sort: "code", tenant_id: "X" } as never);
    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual(["cursor", "limit", "published", "q", "unit"]);
  });

  it("400 của máy chủ về tới giao diện nguyên văn", async () => {
    ghiGia(400, {
      code: "invalid_request",
      message: "Từ khoá tìm kiếm quá dài (tối đa 200 ký tự).",
      trace_id: "01JTRACE",
    });
    expect(await timCanBo(hopLe("abc"))).toEqual({
      ok: false,
      thongBao: "Từ khoá tìm kiếm quá dài (tối đa 200 ký tự).",
    });
  });
});

describe("docTrangDanhBa — rẽ GET hay POST, và chữ tìm KHÔNG BAO GIỜ lên URL", () => {
  it("không chữ tìm → GET mang `unit`/`published`, không thân", async () => {
    const gia = mayChuTimGia(NAM_NGUOI);
    const kq = await docTrangDanhBa(null, { boPhan: "BP-LE", congKhai: true, cursor: null });

    const [duongDan, tuyChon] = gia.mock.calls[0] as unknown as [string, RequestInit];
    expect(duongDan).toBe("/api/v1/staff?unit=BP-LE&published=true");
    expect(tuyChon.method).toBe("GET");
    expect(tuyChon.body).toBeUndefined();
    expect(kq.ok && kq.duLieu.items.map((x) => x.code)).toEqual(["CB001", "CB003"]);
  });

  it("có chữ tìm → POST, bộ lọc cùng đi trong thân, AND với chữ", async () => {
    const gia = mayChuTimGia(NAM_NGUOI);
    const kq = await docTrangDanhBa(hopLe("Huỳnh Văn"), { boPhan: "BP-LE", congKhai: false });

    const [duongDan, tuyChon] = gia.mock.calls[0] as unknown as [string, RequestInit];
    expect(duongDan).toBe("/api/v1/staff/searches");
    expect(JSON.parse(String(tuyChon.body))).toMatchObject({
      q: "Huỳnh Văn",
      unit: "BP-LE",
      published: false,
    });
    expect(kq.ok && kq.duLieu.items.map((x) => x.code)).toEqual(["CB005"]);
  });

  it("đi hết mọi trang của một lần tìm: con trỏ trong THÂN, URL đứng yên, không lặp không sót", async () => {
    const gia = mayChuTimGia(NAM_NGUOI);
    const tu = hopLe("Huỳnh Văn");

    let nganXep = TRANG_DAU;
    const daThay: string[] = [];
    for (let i = 0; i < 10; i++) {
      const kq = await docTrangDanhBa(tu, { cursor: nganXep.hienTai });
      if (!kq.ok) throw new Error(kq.thongBao);
      daThay.push(...kq.duLieu.items.map((x) => x.code));
      if (!kq.duLieu.has_more) break;
      nganXep = sangTrangSau(nganXep, kq.duLieu.next_cursor);
    }

    expect(daThay).toEqual(NAM_NGUOI.map((x) => x.code));
    const cacLoiGoi = gia.mock.calls as unknown as [string, RequestInit][];
    expect(new Set(cacLoiGoi.map(([url]) => url))).toEqual(new Set(["/api/v1/staff/searches"]));
    const conTro = cacLoiGoi.map(([, tc]) => (JSON.parse(String(tc.body)) as { cursor: string }).cursor);
    expect(conTro).toEqual(["", "CB002", "CB004"]);
  });

  it("KHÔNG URL NÀO — tìm, lọc, sang trang, tìm số, bỏ tìm — chứa chữ tìm", async () => {
    // Chữ tìm thường là họ tên hoặc số điện thoại: trên URL nó vào log truy cập, log proxy, và lịch
    // sử trình duyệt của máy dùng chung (luật 3, cấm #4). Ca này đi đủ mọi đường màn hình đi.
    const gia = mayChuTimGia(NAM_NGUOI);
    const tu = hopLe("Huỳnh Văn");

    const p1 = await docTrangDanhBa(tu, {});
    expect(p1.ok && p1.duLieu.has_more).toBe(true);
    await docTrangDanhBa(tu, { cursor: p1.ok ? p1.duLieu.next_cursor : null });
    await docTrangDanhBa(tu, { boPhan: "BP-LE", congKhai: true });
    await docTrangDanhBa(hopLe(CHU_TIM), { congKhai: false });
    await docTrangDanhBa(hopLe("0900000000"), {});
    await docTrangDanhBa(null, { boPhan: "BP-CHAN" });

    const cacLoiGoi = gia.mock.calls as unknown as [string, RequestInit][];
    expect(cacLoiGoi.length).toBe(6);
    for (const [url] of cacLoiGoi) {
      for (const hinh of [...hinhDangTrenUrl(CHU_TIM), ...hinhDangTrenUrl("Huỳnh Văn")]) {
        expect(url).not.toContain(hinh);
      }
      expect(url).not.toMatch(/[?&]q=/);
    }
    // Và máy chủ giả (trả 405 cho tuyến tìm có chuỗi truy vấn) đã trả lời thành công cả sáu.
    for (const r of gia.mock.results) {
      expect(((await r.value) as Response).status).toBe(200);
    }
  });
});
