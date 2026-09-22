import { afterEach, describe, expect, it, vi } from "vitest";

import { layPhieuPhanAnh } from "./phieu-phan-anh";
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
