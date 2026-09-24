import { afterEach, describe, expect, it, vi } from "vitest";

import {
  BAY_DANH_MUC_GHI,
  suaMuc,
  themMuc,
  xoaMuc,
  type MoTaDanhMucGhi,
  type SuaMucVao,
  type ThemMucVao,
} from "./danh-muc";

/**
 * ĐƯỜNG GHI CỦA TAB "DANH MỤC" — tệp này canh đúng ba điều KHÔNG nhìn thấy được trên một màn
 * hình đang chạy tốt, và cả ba đều hỏng theo kiểu im lặng:
 *
 *   1. thân POST/PATCH không mang `source`, `tier`, `code`. Máy chủ trả 400 nếu thân NHẮC TỚI
 *      chúng, kể cả với giá trị đúng bằng giá trị nó vừa trả về
 *      (`service-documents/internal/http/loai_van_ban.go:247`, `:284`, `:288`);
 *   2. `Idempotency-Key` có mặt, và KHÔNG đổi giữa hai lần gửi của cùng một biểu mẫu;
 *   3. một lần từ chối theo tầng (409) ra tới người dùng bằng CÂU của máy chủ — không phải
 *      "409 Conflict", và không phải một câu client tự nghĩ ra.
 *
 * TÁCH KHỎI `danh-muc.test.ts` vì tệp ấy canh hai tuyến ĐỌC của màn danh bạ; hai bề mặt khác
 * nhau trong một tệp là một tệp mà người sửa một bên phải đọc cả bên kia.
 */

function ok200(than: unknown) {
  return new Response(JSON.stringify(than), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function taoRa(than: unknown) {
  return new Response(JSON.stringify(than), {
    status: 201,
    headers: { "Content-Type": "application/json" },
  });
}

function loiCuaMayChu(status: number, code: string, message: string) {
  return new Response(JSON.stringify({ code, message, trace_id: "tr-1" }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** Bắt `fetch`, giữ lại đường dẫn, phương thức, header và thân của từng lời gọi. */
function batFetchGhi(tra: () => Response) {
  const gia = vi.fn(async () => tra());
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof batFetchGhi>, i = 0) {
  const [duongDan, init] = gia.mock.calls[i] as unknown as [string, RequestInit];
  const header = new Headers(init.headers);
  const than =
    init.body === undefined || init.body === null
      ? {}
      : (JSON.parse(String(init.body)) as Record<string, unknown>);
  return { duongDan, phuongThuc: init.method, header, than, khoaThan: Object.keys(than) };
}

function moTa(khoa: string): MoTaDanhMucGhi {
  const mo = BAY_DANH_MUC_GHI.find((m) => m.khoa === khoa);
  if (mo === undefined) throw new Error(`không có nhóm ${khoa}`);
  return mo;
}

/** Một dòng đúng như máy chủ trả về — CÓ `source` và `tier`. Đây là nguồn của lối sai tự nhiên. */
function dongMayChuTraVe() {
  return {
    id: "01JHM7",
    code: "cong-van",
    label: "Công văn",
    active: true,
    is_default: false,
    order: 3,
    source: "he-thong",
    tier: 2,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("POST thêm mục", () => {
  it("gửi ĐÚNG những trường của biểu mẫu — không `source`, không `tier`", async () => {
    const gia = batFetchGhi(() => taoRa(dongMayChuTraVe()));

    await themMuc(moTa("loaiVanBan"), { code: "cong-van", label: "Công văn", order: 3 }, "k-1");

    const g = loiGoi(gia);
    expect(g.duongDan).toBe("/api/v1/document-types");
    expect(g.phuongThuc).toBe("POST");
    expect(g.khoaThan.sort()).toEqual(["code", "label", "order"]);
  });

  it("MỘT DÒNG ĐỌC TỪ MÁY CHỦ gửi ngược lên cũng không mang `source`/`tier` đi theo", async () => {
    // CA ĐÁNG GIÁ NHẤT CỦA TỆP NÀY. `Omit<...>` chặn lối này lúc biên dịch, nhưng kiểu không sống
    // tới lúc chạy: một `as` ở đâu đó, một thân JSON đọc từ chỗ khác, và `source` lại đi lên. Phép
    // ép kiểu dưới đây dựng đúng tình huống ấy — nếu `themMuc` trải đối tượng nguồn thay vì dựng
    // từng trường, ca này đỏ.
    const gia = batFetchGhi(() => taoRa(dongMayChuTraVe()));

    await themMuc(moTa("loaiVanBan"), dongMayChuTraVe() as unknown as ThemMucVao, "k-2");

    const g = loiGoi(gia);
    expect(g.khoaThan).not.toContain("source");
    expect(g.khoaThan).not.toContain("tier");
    expect(g.khoaThan).not.toContain("id");
    expect(g.khoaThan).not.toContain("active");
  });

  it("mang `Idempotency-Key` đúng khoá truyền vào, và lần gửi lại dùng LẠI khoá ấy", async () => {
    const gia = batFetchGhi(() => taoRa(dongMayChuTraVe()));

    await themMuc(moTa("loaiVanBan"), { code: "cong-van", label: "Công văn" }, "k-co-dinh");
    await themMuc(moTa("loaiVanBan"), { code: "cong-van", label: "Công văn" }, "k-co-dinh");

    // Hai lần gửi của CÙNG một biểu mẫu mang cùng một khoá: lần đầu có thể đã tới máy chủ, và một
    // khoá mới ở lần thử lại biến nó thành mục thứ hai trong danh mục.
    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBe("k-co-dinh");
    expect(loiGoi(gia, 1).header.get("Idempotency-Key")).toBe("k-co-dinh");
  });

  it("bảy nhóm gửi tới bảy đường dẫn của hợp đồng, không nhóm nào gửi nhầm chỗ", async () => {
    const gia = batFetchGhi(() => taoRa(dongMayChuTraVe()));

    for (const mo of BAY_DANH_MUC_GHI) {
      await themMuc(mo, { code: "a-b", label: "A B" }, "k");
    }

    expect(BAY_DANH_MUC_GHI.map((_, i) => loiGoi(gia, i).duongDan)).toEqual([
      "/api/v1/map-asset-types",
      "/api/v1/capital-plan-categories",
      "/api/v1/document-types",
      "/api/v1/residential-unit-types",
      "/api/v1/task-blocs",
      "/api/v1/task-types",
      "/api/v1/task-priorities",
    ]);
  });

  it("không có host nào bị nung vào bundle, và không có `tenant_id` ở bất kỳ đâu", async () => {
    const gia = batFetchGhi(() => taoRa(dongMayChuTraVe()));

    await themMuc(moTa("loaiVanBan"), { code: "a-b", label: "A B" }, "k");

    const g = loiGoi(gia);
    expect(g.duongDan).not.toMatch(/^https?:/);
    expect(`${g.duongDan} ${JSON.stringify(g.than)}`).not.toMatch(/tenant/i);
  });
});

describe("PATCH sửa mục", () => {
  it("không gửi `code` — mã đã cấp thì không đổi được (luật 7, bất biến 3)", async () => {
    const gia = batFetchGhi(() => ok200(dongMayChuTraVe()));

    await suaMuc(moTa("loaiVanBan"), "01JHM7", {
      ...(dongMayChuTraVe() as unknown as SuaMucVao),
      label: "Công văn đến",
    });

    const g = loiGoi(gia);
    expect(g.phuongThuc).toBe("PATCH");
    expect(g.duongDan).toBe("/api/v1/document-types/01JHM7");
    expect(g.khoaThan).not.toContain("code");
    expect(g.khoaThan).not.toContain("source");
    expect(g.khoaThan).not.toContain("tier");
    expect(g.than.label).toBe("Công văn đến");
  });

  it("nút `Tắt` gửi ĐÚNG MỘT trường `active` — trường không nhắc tới là trường không đổi", async () => {
    const gia = batFetchGhi(() => ok200(dongMayChuTraVe()));

    await suaMuc(moTa("mucUuTienNhiemVu"), "01JHM9", { active: false });

    const g = loiGoi(gia);
    expect(g.duongDan).toBe("/api/v1/task-priorities/01JHM9");
    expect(g.khoaThan).toEqual(["active"]);
    expect(g.than.active).toBe(false);
  });
});

describe("DELETE xoá mềm", () => {
  it("gửi lý do trong THÂN, không trong query", async () => {
    const gia = batFetchGhi(() => new Response(null, { status: 204 }));

    const kq = await xoaMuc(moTa("loaiVanBan"), "01JHM7", "Trùng với loại Công văn");

    const g = loiGoi(gia);
    expect(kq.ok).toBe(true);
    expect(g.phuongThuc).toBe("DELETE");
    // Một lý do nằm trong query string lọt vào mọi nhật ký truy cập và mọi bộ đệm proxy — đó là
    // chữ cán bộ gõ về một hồ sơ của cơ quan nhà nước.
    expect(g.duongDan).toBe("/api/v1/document-types/01JHM7");
    expect(g.than).toEqual({ reason: "Trùng với loại Công văn" });
  });

  it("id đi vào đường dẫn được mã hoá, không ghép thô", async () => {
    const gia = batFetchGhi(() => new Response(null, { status: 204 }));

    await xoaMuc(moTa("loaiVanBan"), "a/b?c", "lý do");

    expect(loiGoi(gia).duongDan).toBe("/api/v1/document-types/a%2Fb%3Fc");
  });
});

describe("409 — từ chối theo tầng", () => {
  it("hiện NGUYÊN câu của máy chủ, không hiện số hiệu HTTP, không tự viết lại", async () => {
    const cauMayChu = "Mục do hệ thống cấp thì không xoá được. Hãy tắt mục đó thay vì xoá.";
    batFetchGhi(() => loiCuaMayChu(409, "system_row", cauMayChu));

    const kq = await xoaMuc(moTa("loaiVanBan"), "01JHM7", "dọn danh mục");

    expect(kq.ok).toBe(false);
    if (kq.ok) return;
    expect(kq.thongBao).toBe(cauMayChu);
    // "409" hay "Conflict" lọt ra màn hình là một câu cán bộ không làm gì được với nó.
    expect(kq.thongBao).not.toMatch(/409|conflict/i);
    // `trace_id` là mốc tra log, không phải mã lỗi nghiệp vụ (`goi.ts`).
    expect(kq.thongBao).not.toContain("tr-1");
  });

  it("400 vì thân nhắc tới `source` cũng ra tới người dùng bằng câu của máy chủ", async () => {
    const cauMayChu =
      "danh_muc: `source` và tầng của mục do hệ thống quyết định, không nhận từ yêu cầu";
    batFetchGhi(() => loiCuaMayChu(400, "invalid_request", cauMayChu));

    const kq = await themMuc(moTa("loaiVanBan"), { code: "a", label: "A" }, "k");

    expect(kq.ok).toBe(false);
    if (kq.ok) return;
    expect(kq.thongBao).toBe(cauMayChu);
  });

  it("mạng hỏng KHÔNG bị đọc thành một trạng thái nghiệp vụ", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new Error("mạng hỏng");
      }),
    );

    const kq = await suaMuc(moTa("loaiVanBan"), "01JHM7", { active: false });

    expect(kq.ok).toBe(false);
    if (kq.ok) return;
    expect(kq.thongBao).toBe("Không kết nối được máy chủ. Vui lòng thử lại.");
  });
});

/**
 * NỬA CÒN LẠI CỦA QUY TẮC MÀ `DELETE xoá mềm` CANH NỬA ĐẦU — và nó vào đây vì cả kho KHÔNG canh
 * nó, đo bằng đột biến ngày 24/09/2026.
 *
 * Hai nửa ấy kéo ngược chiều nhau, nên bỏ một nửa là mời người sau sửa cho "gọn": 204 KHÔNG THÂN
 * phải là thành công (`.json()` sẽ hỏng), còn 201 CÓ mã đúng mà thân KHÔNG đọc được thì KHÔNG phải
 * thành công. Chỉ canh nửa đầu thì phép sửa hiển nhiên — nuốt lỗi phân giải rồi trả `ok: true` —
 * làm xanh mọi ca của cả 72 tệp, trong khi nó vừa giao cho màn hình một `duLieu` là `undefined`.
 *
 * ĐIỀU ẤY GIỜ ĐẮT HƠN TRƯỚC, VÀ ĐÓ LÀ LÝ DO CA NÀY KHÔNG CHỜ ĐƯỢC: phép đọc thân từng có mười một
 * bản rải khắp `lib/api/`, nay gom về `goi.ts` (`docThanKetQua`). Một dòng sai ở đó là sai ở MỌI
 * tuyến ghi của mọi phân hệ cùng một lúc.
 *
 * MỘT CA CHO CẢ MƯỜI MỘT CHỖ GỌI, KHÔNG MƯỜI MỘT CA: thân hàm chỉ còn một bản: chép ca này sang
 * từng tệp là dựng lại đúng thứ vừa gỡ đi. `docThanLoiGoi` không có ca riêng vì nó không có logic
 * riêng — nó `then` sang chính hàm này.
 */
describe("mã đúng mà THÂN không đọc được KHÔNG phải là thành công", () => {
  it("201 kèm thân không phải JSON ra một câu cho người dùng, không ra `ok: true`", async () => {
    // Hình dạng có thật: một proxy chen vào giữa và trả trang lỗi HTML của nó kèm mã của máy chủ.
    batFetchGhi(
      () => new Response("<html>502</html>", { status: 201, headers: { "Content-Type": "text/html" } }),
    );

    const kq = await themMuc(moTa("loaiVanBan"), { code: "cong-van", label: "Công văn" }, "k-1");

    expect(kq.ok).toBe(false);
    if (kq.ok) return;
    expect(kq.thongBao).toBe("Không kết nối được máy chủ. Vui lòng thử lại.");
  });
});

/**
 * HAI DANH MỤC CỦA IDENTITY — `Loại đơn vị dân cư` và `Khối nhiệm vụ` — đi qua ĐÚNG ba hàm ghi
 * chung ở trên, không qua bản thứ hai nào. Các ca dưới đây canh rằng ba điều của đầu tệp cũng
 * đúng với hai đường dẫn mới: thân dựng từng trường, `code` không đi lên trong PATCH, lý do nằm
 * trong thân DELETE (`service-identity/internal/http/danh_muc_ghi.go:9-16`).
 */
describe("hai danh mục identity: residential-unit-types và task-blocs", () => {
  const HAI_NHOM = [
    { khoa: "loaiDonViDanCu", goc: "/api/v1/residential-unit-types" },
    { khoa: "khoiNhiemVu", goc: "/api/v1/task-blocs" },
  ] as const;

  for (const n of HAI_NHOM) {
    it(`${n.khoa}: POST gửi đúng code · label · order · is_default, kèm Idempotency-Key — không source/tier`, async () => {
      const gia = batFetchGhi(() => taoRa(dongMayChuTraVe()));

      // Truyền vào một dòng đọc từ máy chủ (có `source`, `tier`, `id`, `active`) — lối sai tự
      // nhiên nhất. Thân gửi đi phải chỉ còn bốn trường của biểu mẫu.
      await themMuc(
        moTa(n.khoa),
        { ...(dongMayChuTraVe() as unknown as ThemMucVao), is_default: true },
        "k-id",
      );

      const g = loiGoi(gia);
      expect(g.duongDan).toBe(n.goc);
      expect(g.phuongThuc).toBe("POST");
      expect(g.header.get("Idempotency-Key")).toBe("k-id");
      expect(g.khoaThan.sort()).toEqual(["code", "is_default", "label", "order"]);
      expect(g.than).toEqual({ code: "cong-van", label: "Công văn", order: 3, is_default: true });
    });

    it(`${n.khoa}: PATCH không bao giờ mang code, source, tier`, async () => {
      const gia = batFetchGhi(() => ok200(dongMayChuTraVe()));

      await suaMuc(moTa(n.khoa), "01JHX1", {
        ...(dongMayChuTraVe() as unknown as SuaMucVao),
        label: "Khu phố",
      });

      const g = loiGoi(gia);
      expect(g.phuongThuc).toBe("PATCH");
      expect(g.duongDan).toBe(`${n.goc}/01JHX1`);
      expect(g.khoaThan).not.toContain("code");
      expect(g.khoaThan).not.toContain("source");
      expect(g.khoaThan).not.toContain("tier");
      expect(g.than.label).toBe("Khu phố");
    });

    it(`${n.khoa}: DELETE gửi lý do trong thân, tới đúng đường dẫn của mục`, async () => {
      const gia = batFetchGhi(() => new Response(null, { status: 204 }));

      const kq = await xoaMuc(moTa(n.khoa), "01JHX1", "Trùng với loại Thôn");

      const g = loiGoi(gia);
      expect(kq.ok).toBe(true);
      expect(g.phuongThuc).toBe("DELETE");
      expect(g.duongDan).toBe(`${n.goc}/01JHX1`);
      expect(g.than).toEqual({ reason: "Trùng với loại Thôn" });
    });
  }
});
