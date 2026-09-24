import { afterEach, describe, expect, it, vi } from "vitest";

import {
  capSoVanBanDi,
  chuyenVanBanDen,
  duongDanSoVanBanDen,
  duongDanSoVanBanDi,
  goVanBanDen,
  goVanBanDi,
  suaVanBanDen,
  suaVanBanDi,
  thamSoTheoHopDong,
  vaoSoVanBanDen,
  type SuaVanBanDenVao,
  type VaoSoVanBanDenVao,
} from "./van-ban";
import type {
  documents_get_incoming_documents,
  documents_get_outgoing_documents,
} from "./schema.gen";

/**
 * Chín tuyến của hai quyển sổ, và tệp này canh những thứ một lần sửa MỘT DÒNG phá được mà không
 * phép kiểm nào khác thấy — vì hậu quả của chúng không hiện ra trên màn hình, nó hiện ra ở chỗ một
 * SỐ VĂN BẢN bị tiêu mà không lấy lại được (luật 7, bất biến 3).
 */

/** Một `fetch` giả ghi lại lời gọi. `null` là thân rỗng — đúng hình dạng một câu trả lời 204. */
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

function loiGoi(gia: ReturnType<typeof ghiGia>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

function than(gia: ReturnType<typeof ghiGia>, n: number): Record<string, unknown> {
  return JSON.parse(String(loiGoi(gia, n).tuyChon.body)) as Record<string, unknown>;
}

const DONG_DEN = {
  id: "01JVBDEN0000000000000001",
  number: 7,
  year: 2026,
  received_date: "2026-09-22",
  reference_no: "1742-CV/BTCTU",
  document_date: "2026-09-18",
  issuing_body: "Ban Tổ chức Tỉnh uỷ",
  document_type: "cong-van",
  summary: "Về việc rà soát hồ sơ cán bộ",
  urgency: "khan",
  holding_unit: "",
  assignee: "",
  due_at: "2026-09-25T08:00:00Z",
  status: "moi-vao-so",
  created_by: "CB-00123",
  created_at: "2026-09-22T02:00:00Z",
  updated_at: "2026-09-22T02:00:00Z",
};

const DONG_DI = {
  id: "01JVBDI00000000000000001",
  number: 12,
  year: 2026,
  document_date: "2026-09-22",
  document_type: "cong-van",
  summary: "Trả lời đơn kiến nghị",
  recipient: "UBND huyện",
  signer: "Chủ tịch UBND xã",
  created_by: "CB-00123",
  created_at: "2026-09-22T02:00:00Z",
  updated_at: "2026-09-22T02:00:00Z",
};

const VAO_SO: VaoSoVanBanDenVao = {
  received_date: "2026-09-22",
  reference_no: "1742-CV/BTCTU",
  document_date: "2026-09-18",
  issuing_body: "Ban Tổ chức Tỉnh uỷ",
  document_type: "cong-van",
  summary: "Về việc rà soát hồ sơ cán bộ",
  urgency: "khan",
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Idempotency-Key — khoá do biểu mẫu giữ, KHÔNG sinh trong hàm", () => {
  it("vào sổ văn bản đến: lần bấm thứ hai dùng LẠI đúng khoá truyền vào", async () => {
    // CA ĐẮT NHẤT CỦA CẢ TỆP. Nếu hàm tự sinh khoá bên trong thì mỗi lần bấm lại sau một lỗi mạng
    // là một khoá MỚI — và lần gửi đầu CÓ THỂ đã tới máy chủ và đã cấp một số đến. Hai lần bấm là
    // HAI SỐ bị tiêu khỏi quyển sổ, vĩnh viễn, và quyển sổ có hai dòng cho một tờ văn bản.
    const gia = ghiGia(201, DONG_DEN);
    await vaoSoVanBanDen(VAO_SO, "khoa-cua-bieu-mau");
    await vaoSoVanBanDen(VAO_SO, "khoa-cua-bieu-mau");

    expect(gia).toHaveBeenCalledTimes(2);
    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("khoa-cua-bieu-mau");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("khoa-cua-bieu-mau");
    // VẾ CHỊU LỰC: hai khoá phải BẰNG NHAU. Một hàm sinh khoá tại chỗ vẫn đặt header ở cả hai lần
    // gọi, nên một phép kiểm chỉ hỏi "có header không" sẽ xanh trong khi lỗi vẫn còn nguyên.
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe(
      loiGoi(gia, 0).header.get("Idempotency-Key"),
    );
  });

  it("cấp số văn bản đi: lần bấm thứ hai dùng LẠI đúng khoá truyền vào", async () => {
    // Cùng lý do, một bậc nặng hơn: số đi đã được in lên giấy và đóng dấu gửi ra khỏi xã.
    const gia = ghiGia(201, DONG_DI);
    await capSoVanBanDi(
      { document_date: "2026-09-22", document_type: "cong-van", summary: "x", recipient: "y", signer: "" },
      "khoa-cua-bieu-mau-di",
    );
    await capSoVanBanDi(
      { document_date: "2026-09-22", document_type: "cong-van", summary: "x", recipient: "y", signer: "" },
      "khoa-cua-bieu-mau-di",
    );

    expect(gia).toHaveBeenCalledTimes(2);
    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("khoa-cua-bieu-mau-di");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("khoa-cua-bieu-mau-di");
  });

  it("bốn tuyến ghi còn lại KHÔNG mang Idempotency-Key — hợp đồng khai `khong-can`", async () => {
    // VẾ PHỦ ĐỊNH, và nó không phải chuyện thẩm mỹ: `POST .../routings` mà mang khoá chống trùng sẽ
    // GIẤU lần chuyển thứ hai thay vì ghi nó. Bảng lịch sử chỉ-thêm ghi HÀNH VI, và hai lần chuyển
    // là hai hành vi có thật (`routes.go`, `idem.KhongCan` của tuyến ấy).
    const gia = ghiGia(200, DONG_DEN);
    await suaVanBanDen(DONG_DEN.id, { summary: "y" });
    await chuyenVanBanDen(DONG_DEN.id, { to_unit: "bp", assignee: "", reason: "thuộc thẩm quyền" });

    const gia2 = ghiGia(200, DONG_DI);
    await suaVanBanDi(DONG_DI.id, { summary: "y" });

    const gia3 = ghiGia(204, null);
    await goVanBanDen(DONG_DEN.id, "nhập trùng");

    for (const [g, n] of [
      [gia, 0],
      [gia, 1],
      [gia2, 0],
      [gia3, 0],
    ] as const) {
      expect(loiGoi(g, n).header.get("Idempotency-Key")).toBeNull();
    }
  });
});

describe("thân gửi lên KHÔNG mang trường nào của máy chủ, dưới mọi cách viết", () => {
  /** Mọi cách viết của một trường "quá hạn" mà một màn hình có thể trót dựng ra ở client. */
  const CACH_VIET = /overdue|qua_?han|is_?late|tre_?han/i;

  it("vào sổ: đúng bảy khoá của hợp đồng, không một khoá lạ nào", async () => {
    const gia = ghiGia(201, DONG_DEN);

    // Một dòng vừa đọc được, kèm đúng những trường máy chủ TỪ CHỐI và một trường `overdue` bịa ra.
    // Đây là hình dạng của lỗi thật: ai đó truyền thẳng một đối tượng đang có vào hàm này.
    const ban = {
      ...VAO_SO,
      number: 99,
      status: "da-giai-quyet",
      due_at: "2026-01-01T00:00:00Z",
      overdue: true,
      qua_han: true,
      is_overdue: true,
      isLate: true,
    } as unknown as VaoSoVanBanDenVao;
    await vaoSoVanBanDen(ban, "k");

    const daGui = than(gia, 0);
    expect(Object.keys(daGui).sort()).toEqual([
      "document_date",
      "document_type",
      "issuing_body",
      "received_date",
      "reference_no",
      "summary",
      "urgency",
    ]);
    // VẾ CHỊU LỰC. Kiểu `Omit<…>` chặn lúc biên dịch, phép dựng từng trường chặn lúc chạy; ca này
    // canh vế thứ hai, vì một `...than` thêm vào ngày mai vẫn biên dịch sạch.
    for (const khoa of Object.keys(daGui)) expect(khoa).not.toMatch(CACH_VIET);
    expect(JSON.stringify(daGui)).not.toMatch(CACH_VIET);
    expect(daGui).not.toHaveProperty("number");
    expect(daGui).not.toHaveProperty("status");
    expect(daGui).not.toHaveProperty("due_at");
  });

  it("sửa văn bản đến: cùng bảy khoá, và vẫn không có số, trạng thái hay hạn", async () => {
    const gia = ghiGia(200, DONG_DEN);
    const ban = { summary: "sửa lại", number: 3, due_at: "x", overdue: false } as unknown as SuaVanBanDenVao;
    await suaVanBanDen(DONG_DEN.id, ban);

    const daGui = than(gia, 0);
    expect(daGui).not.toHaveProperty("number");
    expect(daGui).not.toHaveProperty("due_at");
    expect(JSON.stringify(daGui)).not.toMatch(CACH_VIET);
  });

  it("cấp số và sửa văn bản đi: đúng năm khoá, không có `number`", async () => {
    const gia = ghiGia(201, DONG_DI);
    await capSoVanBanDi(
      {
        document_date: "2026-09-22",
        document_type: "cong-van",
        summary: "x",
        recipient: "y",
        signer: "",
        number: 12,
      } as unknown as Parameters<typeof capSoVanBanDi>[0],
      "k",
    );

    expect(Object.keys(than(gia, 0)).sort()).toEqual([
      "document_date",
      "document_type",
      "recipient",
      "signer",
      "summary",
    ]);
    expect(than(gia, 0)).not.toHaveProperty("number");
  });

  it("chuyển xử lý: đúng ba khoá của hợp đồng", async () => {
    const gia = ghiGia(200, DONG_DEN);
    await chuyenVanBanDen(DONG_DEN.id, {
      to_unit: "bp",
      assignee: "CB-00123",
      reason: "thuộc thẩm quyền bộ phận Địa chính",
    });

    expect(Object.keys(than(gia, 0)).sort()).toEqual(["assignee", "reason", "to_unit"]);
  });
});

describe("gỡ khỏi sổ — lý do đi trong THÂN, và 204 không thân là THÀNH CÔNG", () => {
  it("gửi `reason` trong thân, không trong chuỗi truy vấn", async () => {
    // Chữ tự do về một hồ sơ nhà nước mà nằm trong URL là chữ nằm lại trong mọi nhật ký truy cập
    // và mọi bộ đệm trung gian (luật 3, cấm #4).
    const gia = ghiGia(204, null);
    await goVanBanDen(DONG_DEN.id, "nhập trùng với số 12/2026");

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(tuyChon.method).toBe("DELETE");
    expect(duongDan).toBe(`/api/v1/incoming-documents/${DONG_DEN.id}`);
    expect(duongDan).not.toContain("reason");
    expect(than(gia, 0)).toEqual({ reason: "nhập trùng với số 12/2026" });
  });

  it("204 KHÔNG THÂN là thành công, không phải 'không đọc được'", async () => {
    // Một hàm dùng chung luôn gọi `.json()` sẽ biến một lần gỡ thành công thành một lỗi — và cán
    // bộ sẽ bấm gỡ lần nữa, trên một dòng đã không còn.
    ghiGia(204, null);
    expect(await goVanBanDi(DONG_DI.id, "ghi nhầm")).toEqual({ ok: true, duLieu: null });
  });

  it("id vào đường dẫn ĐÃ MÃ HOÁ, không ghép thẳng", async () => {
    const gia = ghiGia(204, null);
    await goVanBanDen("a/b?c=d", "x");
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/incoming-documents/a%2Fb%3Fc%3Dd");
  });
});

describe("câu của máy chủ ra NGUYÊN VĂN, không rẽ nhánh theo `code`", () => {
  it("409 chưa cấu hình thời hạn: nguyên câu máy chủ viết, không thêm không bớt", async () => {
    // Đây là câu một xã MỚI chắc chắn gặp ở lần vào sổ đầu tiên: `sla` rỗng thì
    // `identity.ResolveDeadlines` từ chối, và `service-documents` biến nó thành 409
    // `sla_chua_cau_hinh`. Câu ấy chỉ thẳng màn hình sửa được, nên nó phải đi ra nguyên vẹn.
    const cauCuaMayChu =
      "Xã chưa cấu hình thời hạn xử lý cho văn bản đến, nên chưa vào sổ được. " +
      "Vào Cấu hình → Thời hạn xử lý để đặt số giờ xử lý, rồi vào sổ lại.";
    ghiGia(409, { code: "sla_chua_cau_hinh", message: cauCuaMayChu, trace_id: "tr-1" });

    const kq = await vaoSoVanBanDen(VAO_SO, "k");
    expect(kq).toEqual({ ok: false, thongBao: cauCuaMayChu });
    expect(JSON.stringify(kq)).not.toContain("409");
    expect(JSON.stringify(kq)).not.toContain("tr-1");
  });

  it("409 loại văn bản đã bị tắt: cũng nguyên văn, không diễn giải lại", async () => {
    const cauCuaMayChu =
      "Loại văn bản này không còn trong danh mục đang dùng của xã. Hãy chọn lại loại văn bản.";
    ghiGia(409, { code: "document_type_unknown", message: cauCuaMayChu, trace_id: "tr-2" });

    expect(await capSoVanBanDi(
      { document_date: "2026-09-22", document_type: "da-tat", summary: "x", recipient: "y", signer: "" },
      "k",
    )).toEqual({ ok: false, thongBao: cauCuaMayChu });
  });
});

describe("bộ lọc và phân trang — đúng tên tham số tuyến đọc", () => {
  it("không đặt gì thì KHÔNG có chuỗi truy vấn nào", () => {
    // Không tự điền mặc định: máy chủ có mặc định của nó (`limit=20`, số vào sổ giảm dần), và một
    // bản sao thứ hai của các mặc định ấy ở đây sẽ trôi khỏi bản của máy chủ.
    expect(duongDanSoVanBanDen()).toBe("/api/v1/incoming-documents");
    expect(duongDanSoVanBanDi()).toBe("/api/v1/outgoing-documents");
  });

  it("sổ đến: năm · trạng thái · loại · bộ phận · tìm · limit · cursor", () => {
    const d = duongDanSoVanBanDen({
      nam: 2026,
      trangThai: "dang-xu-ly",
      loaiVanBan: "cong-van",
      boPhanDangGiu: "01JBOPHAN",
      tim: "rà soát",
      limit: 20,
      cursor: "con-tro-cua-may-chu",
    });
    const truyVan = new URLSearchParams(d.split("?")[1]);

    expect(truyVan.get("year")).toBe("2026");
    expect(truyVan.get("status")).toBe("dang-xu-ly");
    expect(truyVan.get("document_type")).toBe("cong-van");
    expect(truyVan.get("holding_unit")).toBe("01JBOPHAN");
    expect(truyVan.get("q")).toBe("rà soát");
    expect(truyVan.get("limit")).toBe("20");
    expect(truyVan.get("cursor")).toBe("con-tro-cua-may-chu");
  });

  it("con trỏ rỗng KHÔNG đi vào URL — `cursor=` rỗng là 400 ngay lần mở màn hình đầu tiên", () => {
    expect(duongDanSoVanBanDen({ cursor: null })).toBe("/api/v1/incoming-documents");
    expect(duongDanSoVanBanDen({ cursor: "" })).toBe("/api/v1/incoming-documents");
  });

  it("bộ lọc rỗng KHÔNG đi vào URL — chuỗi rỗng nghĩa là 'không lọc'", () => {
    // Máy chủ TỪ CHỐI một `status` lạ thay vì bỏ qua (`van_ban_den.go:447`), nên gửi `status=` rỗng
    // là mời một lời từ chối cho một bộ lọc người dùng vừa bỏ chọn.
    expect(duongDanSoVanBanDen({ trangThai: "", loaiVanBan: "", boPhanDangGiu: "", tim: "" })).toBe(
      "/api/v1/incoming-documents",
    );
  });

  it("sổ đi KHÔNG gửi `status` và KHÔNG gửi `holding_unit`", () => {
    // VẾ PHỦ ĐỊNH: sổ đi không có trạng thái và không có bộ phận đang giữ (`van_ban_di.go:40`).
    // Gửi hai tham số ấy là 400, và tệ hơn: nó là dấu hiệu ai đó vừa chép quy trình của sổ đến sang.
    const d = duongDanSoVanBanDi({ nam: 2026, loaiVanBan: "cong-van", tim: "x" });
    expect(d).not.toContain("status");
    expect(d).not.toContain("holding_unit");
  });

  it("không chọn thứ tự thì KHÔNG gửi `sort` và `order` — máy chủ giữ mặc định số giảm dần", () => {
    // Mặc định là của máy chủ (`docstore.SapXepVanBanDen`). Gửi `sort=number&order=desc` ở đây là
    // giữ bản sao thứ hai của mặc định ấy, và nó trôi vào ngày máy chủ đổi.
    for (const d of [duongDanSoVanBanDen({ nam: 2026 }), duongDanSoVanBanDi({ nam: 2026 })]) {
      expect(d).not.toContain("sort");
      expect(d).not.toContain("order");
    }
  });

  it("chọn thứ tự: `sort` và `order` đi lên ĐÚNG giá trị của enum hợp đồng, ở cả hai sổ", () => {
    for (const d of [
      duongDanSoVanBanDen({ sort: "number", order: "asc" }),
      duongDanSoVanBanDi({ sort: "number", order: "asc" }),
    ]) {
      const truyVan = new URLSearchParams(d.split("?")[1]);
      expect(truyVan.getAll("sort")).toEqual(["number"]);
      expect(truyVan.getAll("order")).toEqual(["asc"]);
    }
  });

  it("tìm chữ: `q` được cắt khoảng trắng hai đầu, giữ nguyên chữ ở giữa, ở cả hai sổ", () => {
    for (const d of [
      duongDanSoVanBanDen({ tim: "  rà soát hồ sơ " }),
      duongDanSoVanBanDi({ tim: "  rà soát hồ sơ " }),
    ]) {
      expect(new URLSearchParams(d.split("?")[1]).getAll("q")).toEqual(["rà soát hồ sơ"]);
    }
  });

  it("ô tìm rỗng hoặc toàn dấu cách KHÔNG gửi `q`", () => {
    // Máy chủ không cắt `q` (`http/van_ban_den.go:471`): `q=%20` sẽ thành phép lọc `% %` — một lát
    // cắt cán bộ không hề yêu cầu.
    for (const tim of ["", "   "]) {
      expect(duongDanSoVanBanDen({ tim })).toBe("/api/v1/incoming-documents");
      expect(duongDanSoVanBanDi({ tim })).toBe("/api/v1/outgoing-documents");
    }
  });
});

describe("tên tham số bị ĐỐI CHIẾU với hợp đồng lúc biên dịch", () => {
  // Các dòng `@ts-expect-error` dưới đây LÀ phép kiểm, và `npm run typecheck` chạy nó: nếu một dòng
  // thôi lỗi — tức kiểu đã nới ra nhận một tên hay giá trị ngoài hợp đồng — thì chính chỉ thị ấy
  // thành lỗi "unused @ts-expect-error" và `tsc` đỏ. Vitest không kiểm kiểu; phần chạy chỉ để tệp
  // có một ca xanh mang tên điều nó canh.
  it("tên ngoài `truyVan` và giá trị ngoài enum không biên dịch được", () => {
    const q = new URLSearchParams();
    const datDen = thamSoTheoHopDong<documents_get_incoming_documents["truyVan"]>(q);
    const datDi = thamSoTheoHopDong<documents_get_outgoing_documents["truyVan"]>(q);

    // Một lần máy chủ đổi tên `holding_unit` là đúng hình dạng này ở dòng gửi nó.
    // @ts-expect-error — `holding_units` không phải tham số của tuyến sổ đến.
    datDen("holding_units", "x");
    // @ts-expect-error — sổ đi không có trạng thái; hợp đồng không khai `status` cho nó.
    datDi("status", "moi-vao-so");
    // @ts-expect-error — `received_date` không nằm trong danh sách trắng sắp xếp của máy chủ.
    datDen("sort", "received_date");
    // @ts-expect-error — cùng điều ấy ở tầng bộ lọc mà màn hình dùng.
    duongDanSoVanBanDen({ sort: "received_date" });

    datDen("holding_unit", "01JBOPHAN");
    expect(q.get("holding_unit")).toBe("01JBOPHAN");
  });
});

describe("không một tuyến nào tự khai xã, và không một tuyến nào đi tới một host", () => {
  it("chín đường dẫn đều TƯƠNG ĐỐI và không mang chữ `tenant`", async () => {
    // Luật 1 cấm #2: client tự khai xã là client tự cấp quyền. Và một host nung vào bundle là một
    // bản dựng chỉ đúng cho một xã (luật 1, bất biến 10).
    const gia = ghiGia(200, DONG_DEN);
    await vaoSoVanBanDen(VAO_SO, "k");
    await suaVanBanDen(DONG_DEN.id, { summary: "x" });
    await chuyenVanBanDen(DONG_DEN.id, { to_unit: "bp", assignee: "", reason: "r" });
    await capSoVanBanDi(
      { document_date: "2026-09-22", document_type: "cong-van", summary: "x", recipient: "y", signer: "" },
      "k",
    );
    await suaVanBanDi(DONG_DI.id, { summary: "x" });

    const gia2 = ghiGia(204, null);
    await goVanBanDen(DONG_DEN.id, "x");
    await goVanBanDi(DONG_DI.id, "x");

    const duong = [
      ...[0, 1, 2, 3, 4].map((n) => loiGoi(gia, n).duongDan),
      ...[0, 1].map((n) => loiGoi(gia2, n).duongDan),
      duongDanSoVanBanDen({ nam: 2026 }),
      duongDanSoVanBanDi({ nam: 2026 }),
    ];

    for (const d of duong) {
      expect(d.startsWith("/api/v1/")).toBe(true);
      expect(d).not.toMatch(/tenant/i);
      expect(d).not.toMatch(/^https?:/);
    }
  });
});
