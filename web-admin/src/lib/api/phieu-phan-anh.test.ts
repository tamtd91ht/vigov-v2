import { afterEach, describe, expect, it, vi } from "vitest";

import {
  chuyenXuLyPhieu,
  dongPhieu,
  duongDanSoPhanAnh,
  layPhieuPhanAnh,
  phanLoaiPhieu,
  tienTrangThaiPhieu,
} from "./phieu-phan-anh";
import type { petitions_phieuCuaToiRa, petitions_phieuPhanAnhRa } from "./schema.gen";

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)`
  // không tham số làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const KHONG_TIM_THAY = new Response(
  JSON.stringify({
    code: "not_found",
    message: "Không tìm thấy phiếu phản ánh.",
    trace_id: "01JTRACE",
  }),
  { status: 404, headers: { "Content-Type": "application/json" } },
);

describe("tra phiếu theo mã tra cứu", () => {
  it("mã hoá mã vào đường dẫn thay vì ghép thẳng", async () => {
    const gia = batFetch(
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await layPhieuPhanAnh("PA/2026 0021");
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA%2F2026%200021");
  });

  it("đường dẫn tương đối, không host, không `tenant_id`", async () => {
    const gia = batFetch(
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await layPhieuPhanAnh("PA-2026-0021");
    const duong = String(gia.mock.calls[0]?.[0]);
    expect(duong.startsWith("/api/")).toBe(true);
    expect(duong).not.toMatch(/tenant/i);
  });

  it("BỐN TÌNH HUỐNG, MỘT CÂU TRẢ LỜI — và giao diện không dựng lại sự phân biệt ấy", async () => {
    // Mã không tồn tại · mã của xã khác · phiếu đã xoá mềm · phiếu thuộc lĩnh vực hạn chế mà tài
    // khoản thiếu `feedback.restricted`. Máy chủ trả CÙNG một 404 cho cả bốn: phân biệt được
    // chúng là nói cho người đang thử mã biết họ gần tới đâu, và ở ca thứ tư là nói cho một đồng
    // nghiệp biết có người vừa phản ánh về họ (luật 4, cấm #2).
    //
    // Bài test này đỏ ngay khi ai đó thêm một nhánh rẽ theo `code` để "nói rõ hơn cho cán bộ".
    batFetch(KHONG_TIM_THAY.clone());
    const maBia = await layPhieuPhanAnh("PA-2026-9999");

    batFetch(KHONG_TIM_THAY.clone());
    const linhVucHanChe = await layPhieuPhanAnh("PA-2026-0021");

    expect(maBia).toEqual(linhVucHanChe);
    expect(maBia).toEqual({ ok: false, thongBao: "Không tìm thấy phiếu phản ánh." });
  });

  it("403 thiếu `feedback.read`: câu của máy chủ đi thẳng ra màn hình", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "forbidden",
          message: "Bạn không có quyền thực hiện thao tác này.",
          trace_id: "01JTRACE",
        }),
        { status: 403, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(await layPhieuPhanAnh("PA-2026-0021")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });

  it("200: trả về NGUYÊN phản hồi đã che sẵn của máy chủ, không ghép lại gì", async () => {
    // Hai trường người gửi về ở dạng đã che. Không có phép biến đổi nào ở tầng gọi API — nếu có
    // ngày ai đó thêm một hàm "làm đẹp số điện thoại" thì bài test này đỏ.
    const than = {
      code: "PA-2026-0021",
      channel: "zalo-mini-app",
      status: "dang-phan-loai",
      field: "",
      field_label: "",
      content: "Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.",
      address: "Tổ 6, thôn Hà Lam",
      reporter_name: "Nguyễn V. A.",
      reporter_phone: "09****5678",
      anonymous: false,
      clock_from: "2026-09-09T07:20:00Z",
      booked_at: "2026-09-09T07:21:00Z",
      acknowledge_due: "2026-09-09T09:20:00Z",
      resolve_due: null,
      public: false,
    };

    batFetch(
      new Response(JSON.stringify(than), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const kq = await layPhieuPhanAnh("PA-2026-0021");
    expect(kq).toEqual({ ok: true, duLieu: than });
  });

});

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TÊN THAM SỐ TRUY VẤN — VÀ ĐÂY LÀ CHỖ DUY NHẤT CANH CHÚNG.
 *
 * Hợp đồng nay đã khai các tham số của `GET /api/v1/citizen-reports`, nhưng
 * `themLocVaoTruyVan` vẫn ghép tên bằng chuỗi trần, nên `tsc` không canh giúp một chữ nào ở đây: gõ
 * `hamlet_id` thay vì `hamlet` thì máy chủ bỏ qua bộ lọc và trả về CẢ QUYỂN SỔ, còn màn hình trông
 * hoàn toàn bình thường — cán bộ tin mình đang xem một thôn.
 *
 * Mỗi `expect` dưới đây vì thế là một chuỗi ĐỌC LẠI TỪ HANDLER, không phải từ trí nhớ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
describe("đường dẫn đọc sổ phản ánh", () => {
  it("không lọc gì thì KHÔNG có dấu hỏi thừa", () => {
    expect(duongDanSoPhanAnh()).toBe("/api/v1/citizen-reports");
  });

  it("bảy bộ lọc đi ra bảy tên tham số máy chủ thật sự đọc", () => {
    const duong = duongDanSoPhanAnh({
      trangThai: "dang-xu-ly",
      kenh: "zalo-mini-app",
      linhVuc: "rac-thai",
      thonID: "01JTHON",
      boPhanID: "01JBOPHAN",
      tim: "rác đầu ngõ",
      chiTreHan: true,
    });
    const truyVan = new URLSearchParams(duong.slice(duong.indexOf("?") + 1));

    expect(truyVan.get("status")).toBe("dang-xu-ly");
    expect(truyVan.get("channel")).toBe("zalo-mini-app");
    expect(truyVan.get("field")).toBe("rac-thai");
    expect(truyVan.get("hamlet")).toBe("01JTHON");
    expect(truyVan.get("unit")).toBe("01JBOPHAN");
    expect(truyVan.get("q")).toBe("rác đầu ngõ");
    expect(truyVan.get("late")).toBe("true");
  });

  it("ô `Chỉ phiếu trễ hạn` bỏ tích: tham số VẮNG MẶT HẲN, không phải `late=false`", () => {
    // Máy chủ chỉ nhận đúng chuỗi `true` và trả **400** cho mọi giá trị khác
    // (`errLocTreHanKhongHopLe`) — có chủ ý, vì một ô đã tích mà bị bỏ qua lặng lẽ sẽ hiện cả sổ.
    expect(duongDanSoPhanAnh({ chiTreHan: false })).toBe("/api/v1/citizen-reports");
  });

  it("trang đầu KHÔNG gửi `cursor` rỗng", () => {
    // `cursor=` rỗng là 400 "con trỏ không hợp lệ" — đúng vào lần mở màn hình đầu tiên.
    const duong = duongDanSoPhanAnh({ limit: 20, cursor: null });
    expect(duong).toBe("/api/v1/citizen-reports?limit=20");
  });

  it("trang sau mang đúng con trỏ máy chủ phát ra", () => {
    const duong = duongDanSoPhanAnh({ limit: 20, cursor: "eyJrIjoi" });
    expect(new URLSearchParams(duong.slice(duong.indexOf("?") + 1)).get("cursor")).toBe("eyJrIjoi");
  });

  it("tab `Giao cho tôi` gửi `scope=mine`, và KHÔNG kèm danh tính nào", () => {
    const duong = duongDanSoPhanAnh({ phamVi: "mine", trangThai: "dang-xu-ly" });
    const truyVan = new URLSearchParams(duong.slice(duong.indexOf("?") + 1));
    expect(truyVan.get("scope")).toBe("mine");
    // Các bộ lọc khác đi cùng, không bị tab nuốt mất.
    expect(truyVan.get("status")).toBe("dang-xu-ly");
    // Máy chủ lấy mã cán bộ từ PHIÊN. Một `assignee=` do client gửi là client tự khai mình là ai.
    expect(truyVan.has("assignee")).toBe(false);
  });

  it("tab `Toàn xã` (vắng hoặc `all`): KHÔNG gửi `scope` — mặc định của máy chủ", () => {
    expect(duongDanSoPhanAnh({})).toBe("/api/v1/citizen-reports");
    expect(duongDanSoPhanAnh({ phamVi: "all" })).toBe("/api/v1/citizen-reports");
  });

  it("`scope=related` không bao giờ được gửi — máy chủ trả 400 cho nó", () => {
    // Kiểu `phamVi` không cho viết `"related"` (`tsc` đỏ). Bài chạy này canh chiều còn lại: không
    // bộ lọc nào khác lọt ra thành `related`.
    for (const phamVi of [undefined, "all", "mine"] as const) {
      expect(duongDanSoPhanAnh({ phamVi, chiTreHan: true })).not.toContain("related");
    }
  });

  it("không một chỗ nào mang `tenant_id`", () => {
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã suy từ `Host` ở rìa ngoài cùng.
    const duong = duongDanSoPhanAnh({ trangThai: "da-dong", boPhanID: "01JBOPHAN" });
    expect(duong).not.toMatch(/tenant/i);
    expect(duong.startsWith("/api/")).toBe(true);
  });
});

describe("bốn thao tác ghi", () => {
  function batGhi() {
    const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) =>
      new Response(JSON.stringify({ code: "PA-2026-0021" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", gia);
    return gia;
  }

  it("phân loại: POST …/classification, đúng MỘT trường `field`", () => {
    const gia = batGhi();
    return phanLoaiPhieu("PA-2026-0021", "rac-thai").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/classification");
      const tuyChon = gia.mock.calls[0]?.[1];
      expect(tuyChon?.method).toBe("POST");
      // KHÔNG có `due_at` và KHÔNG có `status`: hạn là cam kết của xã tính theo giờ làm việc của
      // chính xã, và hành vi này chỉ đưa phiếu sang `dang-phan-loai`.
      expect(JSON.parse(String(tuyChon?.body))).toEqual({ field: "rac-thai" });
    });
  });

  it("chuyển xử lý không chọn cán bộ: thân KHÔNG có `assignee` rỗng", () => {
    const gia = batGhi();
    return chuyenXuLyPhieu("PA-2026-0021", "01JBOPHAN").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/assignment");
      expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({ unit: "01JBOPHAN" });
    });
  });

  it("chuyển xử lý có chọn cán bộ: `assignee` là MÃ CÁN BỘ, không phải id nội bộ", () => {
    // Luật nắm giữ so `can_bo_xu_ly_id` với `Principal.Ma`. Một ULID ở đây là một phiếu "đã giao"
    // mà người được giao không bao giờ tiến được.
    const gia = batGhi();
    return chuyenXuLyPhieu("PA-2026-0021", "01JBOPHAN", "CB-00123").then(() => {
      const than = JSON.parse(String(gia.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
      expect(than).toEqual({ unit: "01JBOPHAN", assignee: "CB-00123" });
      expect(Object.keys(than)).not.toContain("id");
      expect(Object.keys(than)).not.toContain("assignee_id");
    });
  });

  it("chuyển xử lý với mã cán bộ rỗng: như không chọn ai, không gửi `assignee` rỗng", () => {
    const gia = batGhi();
    return chuyenXuLyPhieu("PA-2026-0021", "01JBOPHAN", "").then(() => {
      expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({ unit: "01JBOPHAN" });
    });
  });

  it("tiến trạng thái: POST …/status, KHÔNG THÂN và KHÔNG `Content-Type`", () => {
    // Một trạng thái đích đi trên dây là một client nhảy được bước. Gửi `{}` kèm `Content-Type` là
    // tuyên bố có một thân — thứ mời người sau điền vào đó đúng cái trường ấy.
    const gia = batGhi();
    return tienTrangThaiPhieu("PA-2026-0021").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/status");
      const tuyChon = gia.mock.calls[0]?.[1];
      expect(tuyChon?.body).toBeUndefined();
      expect(tuyChon?.headers).toEqual({});
    });
  });

  it("đóng phiếu: POST …/closure kèm kết quả người dân đọc được", () => {
    const gia = batGhi();
    return dongPhieu("PA-2026-0021", "Đã dọn xong điểm tập kết rác chiều 10/9.").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/closure");
      expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({
        result: "Đã dọn xong điểm tập kết rác chiều 10/9.",
      });
    });
  });

  it("mã tra cứu đi vào ĐƯỜNG DẪN thì được mã hoá, ở cả bốn tuyến", () => {
    const gia = batGhi();
    return dongPhieu("PA/2026 0021", "xong").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe(
        "/api/v1/citizen-reports/PA%2F2026%200021/closure",
      );
    });
  });

  it("409 của máy chủ ra thẳng màn hình, nguyên văn", () => {
    // Ba tuyến trả 409 với đúng quy tắc nghiệp vụ đã từ chối. Viết lại câu ấy ở client là dựng bản
    // sao thứ hai của một quy tắc rồi để nó trôi.
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            code: "sla_chua_cau_hinh",
            message:
              "Xã chưa cấu hình thời hạn xử lý cho lĩnh vực này, nên chưa phân loại được. " +
              "Vào Cấu hình → Thời hạn xử lý để đặt số giờ, rồi phân loại lại.",
            trace_id: "01JTRACE",
          }),
          { status: 409, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    return phanLoaiPhieu("PA-2026-0021", "an-ninh").then((kq) => {
      expect(kq.ok).toBe(false);
      expect(kq.ok === false && kq.thongBao).toContain("Cấu hình → Thời hạn xử lý");
    });
  });
});

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHÉP CANH CHIỀU NGƯỢC: hợp đồng KHÔNG được mọc thêm ghi chú nội bộ hay lịch sử luân chuyển.
 *
 * Phép canh này đọc KIỂU SINH RA TỪ HỢP ĐỒNG, không đọc một thân JSON do chính bài test viết ra.
 * Một ca test gửi vào 15 trường rồi đếm lại 15 trường không canh gì cả — nó chỉ chứng minh
 * `JSON.parse(JSON.stringify(x))` giữ nguyên khoá, và nó vẫn xanh nguyên vào ngày tuyến trả về
 * thêm `ghi_chu_noi_bo`.
 *
 * Ở đây thì ngược lại: thêm MỘT trường vào `petitions.phieuPhanAnhRa` là `tsc` đỏ ngay tại dòng
 * `_duKhoaPhieu` bên dưới — nên không trường nào vào được hợp đồng mà không có người đọc nó là gì.
 *
 * ⚠ TIỀN ĐỀ CŨ CỦA KHỐI NÀY ĐÃ SAI, SỬA 23/09/2026. Câu cũ viết *"tuyến này là tuyến tra theo mã
 * người dân cầm trên tay"*, và nó đúng vào ngày được viết. Nay KHÔNG còn: cả sáu tuyến trả về
 * `phieuPhanAnhRa` đều đòi `feedback.*` (đo trên `kb/20-contracts/openapi.json`), tức đây là hình
 * dạng CỦA CÁN BỘ. Công dân đọc một hình dạng KHÁC HẲN — `petitions.phieuCuaToiRa` trên
 * `my-citizen-reports`, đúng cách luật 4 bất biến 5 đòi hai bề mặt không dùng chung handler.
 *
 * HỆ QUẢ: phép canh "không được mọc trường nội bộ" chuyển sang hình dạng CÔNG DÂN, chỗ luật 4 cấm
 * #5 thật sự áp. Giữ nó ở hình dạng cán bộ là canh sai chỗ, và nó vừa chứng minh điều đó bằng cách
 * đỏ lên trước một trường HOÀN TOÀN HỢP LỆ: `assignee` — người được phân công — là thứ màn hình xử
 * lý của cán bộ tồn tại để hiện ra.
 *
 * Ở hình dạng cán bộ thì phép canh còn lại vẫn nguyên giá trị và KHÔNG được nới: mỗi trường mới
 * phải được thêm vào danh sách dưới đây BẰNG TAY, tức phải có người đọc nó là gì trước khi nó ra
 * tới màn hình.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
type KhoaPhieu = keyof petitions_phieuPhanAnhRa;

const KHOA_PHIEU_MONG_DOI = [
  "code",
  "channel",
  "status",
  "field",
  "field_label",
  "content",
  "address",
  "reporter_name",
  "reporter_phone",
  "anonymous",
  "clock_from",
  "booked_at",
  "acknowledge_due",
  "resolve_due",
  "public",
  // NĂM TRƯỜNG CỦA ĐƯỜNG XỬ LÝ PHÍA CÁN BỘ, thêm 23/09/2026 — và chúng vào đây SAU KHI đã đối
  // chiếu từng tuyến trả về hình dạng này: cả sáu đều đòi `feedback.*` (`citizen-reports` và bốn
  // tuyến ghi của nó). Tuyến công dân `my-citizen-reports` trả một hình dạng KHÁC, nên không
  // trường nào dưới đây tới được màn hình người dân — đó là điều luật 4 cấm #5 đòi, và là lý do
  // danh sách này tồn tại thay vì một dòng `Partial<>`.
  //
  // `classify_due` là hạn THỨ BA của một phiếu (trần bắt buộc phân loại, ADR 0035 §C), cạnh
  // `acknowledge_due` và `resolve_due`. `unit` · `assignee` · `result` là bộ phận đang giữ,
  // người được phân công, và kết quả đóng phiếu — thứ công dân ĐƯỢC đọc ở dạng đã lọc qua tuyến
  // riêng của họ, không phải qua hình dạng này.
  "classify_due",
  "unit",
  "assignee",
  "result",
  // BA TRƯỜNG CỦA HAI NHÁNH RẼ, thêm 25/09/2026 (27aed64): lý do không tiếp nhận / chuyển cấp
  // trên, cơ quan nhận, lúc rẽ nhánh. Cả ba là thứ công dân ĐƯỢC đọc (tuyến riêng của họ cũng trả
  // `reason`/`receiving_body`), không phải ghi chú nội bộ; chỉ có mặt khi phiếu ở hai nhánh ấy.
  "reason",
  "receiving_body",
  "branch_ended_at",
] as const satisfies readonly KhoaPhieu[];

/** Hợp đồng mọc thêm một trường mà danh sách trên không có → đỏ ngay tại đây. */
type DuKhoaPhieu =
  Exclude<KhoaPhieu, (typeof KHOA_PHIEU_MONG_DOI)[number]> extends never ? true : never;
const _duKhoaPhieu: DuKhoaPhieu = true;
void _duKhoaPhieu;

describe("phạm vi của phiếu trả về cho cán bộ", () => {
  it("không trường nào trong hợp đồng mang tên của ghi chú nội bộ hay lịch sử luân chuyển", () => {
    // Phép kiểm chạy được, đứng cạnh phép kiểm ở mức KIỂU bên trên. Nó bắt chiều còn lại: một
    // trường mới được thêm vào CẢ hợp đồng LẪN danh sách trên mà không ai đọc lại nó là gì.
    for (const khoa of KHOA_PHIEU_MONG_DOI) {
      expect(khoa).not.toMatch(/noi_bo|internal|note|history|luan_chuyen|nhat_ky/i);
    }
  });
});

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHÉP CANH CỦA LUẬT 4 — ĐẶT ĐÚNG CHỖ: hình dạng CÔNG DÂN đọc.
 *
 * `petitions.phieuCuaToiRa` là thứ `GET /api/v1/my-citizen-reports/{maTraCuu}` trả cho chính
 * người đã gửi phiếu. Luật 4 cấm #5 áp ở ĐÂY, không ở hình dạng của cán bộ: ghi chú nội bộ, lịch
 * sử luân chuyển và người được phân công nằm ngoài phạm vi công dân.
 *
 * SO SÁNH HAI HÌNH DẠNG LÀ ĐIỀU ĐÁNG CANH NHẤT, chứ không phải đếm trường: hai bề mặt dùng chung
 * một hình dạng là đúng thứ luật 4 bất biến 5 cấm, và nó xảy ra bằng một lần ai đó "tái dùng cho
 * gọn". Nếu hai kiểu này có ngày trùng nhau thì `tsc` không đỏ — nên phép kiểm dưới đây khẳng
 * định bốn trường CHỈ CÓ ở phía cán bộ vẫn vắng mặt ở phía công dân.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
type KhoaPhieuCuaToi = keyof petitions_phieuCuaToiRa;

describe("phạm vi của phiếu trả về cho CÔNG DÂN", () => {
  it("không trường nào mang tên ghi chú nội bộ, lịch sử luân chuyển hay người phân công", () => {
    const khoa: readonly KhoaPhieuCuaToi[] = [
      "code", "channel", "status", "field", "field_label", "content", "address",
      "reporter_name", "reporter_phone", "anonymous", "clock_from",
      "acknowledge_due", "resolve_due", "result",
    ];
    for (const k of khoa) {
      expect(k).not.toMatch(/noi_bo|internal|note|history|luan_chuyen|assignee|nhat_ky|unit/i);
    }
  });

  it("BỐN trường chỉ-của-cán-bộ KHÔNG có mặt ở hình dạng công dân", () => {
    // VẾ CHỊU LỰC của cả tệp này. `tsc` đỏ khi một trong bốn tên dưới đây trở thành khoá hợp lệ
    // của `phieuCuaToiRa` — tức đúng ngày ai đó cho hai bề mặt dùng chung một hình dạng, hoặc
    // thêm thẳng một trường nội bộ vào hình dạng công dân.
    //
    // `classify_due` nằm trong bốn tên ấy có chủ ý: trần bắt buộc phân loại là một con số NỘI BỘ
    // để xã tự chấm mình, không phải một lời hứa đã nói với người dân. Hai hạn đã hứa —
    // `acknowledge_due` và `resolve_due` — thì CÓ mặt, và đó là điểm khác nhau.
    type ChiCuaCanBo = "unit" | "assignee" | "classify_due" | "public";
    type KhongDuocLo = Extract<KhoaPhieuCuaToi, ChiCuaCanBo> extends never ? true : never;
    const _khongLo: KhongDuocLo = true;
    void _khongLo;
    expect(_khongLo).toBe(true);
  });
});
