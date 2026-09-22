import { afterEach, describe, expect, it, vi } from "vitest";

import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  TRANG_DAU,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";

import {
  datKhoaCanBo,
  doiVaiTroCanBo,
  duongDanDanhSachCanBo,
  layChiTietCanBo,
  layDanhSachCanBo,
  suaCanBo,
  themCanBo,
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
