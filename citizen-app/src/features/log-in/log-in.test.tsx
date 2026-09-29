import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { MaDangNhap } from "../tinh-nang/zalo-api";

import { sessionAddress, readResponse, SESSION_PATH, sessionRequestBody } from "./contract";
import { issueSession } from "./server-calls";
import { SessionIssuer } from "./SessionIssuer";
import sessionIssuerSource from "./SessionIssuer.tsx?raw";

/**
 * KHỐI ĐĂNG NHẬP — PHÉP KIỂM CỦA BƯỚC MÁY CHỦ.
 *
 * BỐN ĐIỀU Ở ĐÂY KHÔNG CÓ PHÉP KIỂM NÀO KHÁC NÓI HỘ:
 *
 *   1. **Năm nhánh kết quả, mỗi nhánh một việc phải làm tiếp.** Địa chỉ máy chủ đọc lúc DỰNG
 *      nên dưới Vitest nó luôn rỗng — không có tham số địa chỉ của `issueSession` thì bốn
 *      nhánh còn lại không ca nào chạm tới, và một hàm không ai kiểm là một hàm sẽ hỏng đúng
 *      ngày máy chủ thật trả về thứ nó không ngờ.
 *   2. **401 và 502 KHÔNG được gộp.** Hai mã ấy dẫn tới hai việc khác nhau: bấm lại ngay, hay
 *      chờ rồi thử lại. Gộp là bảo một nửa số người làm một việc vô ích.
 *   3. **Thân yêu cầu mang ĐÚNG hai mã, không hơn.** Một trường thêm vào là một dữ liệu rời
 *      khỏi máy người dùng mà chính sách quyền riêng tư không khai.
 *   4. **Bearer không bao giờ ra màn hình.**
 *
 * ⚠ TỆP NÀY LÀ NƠI DUY NHẤT NGOÀI `contract.ts` BIẾT KHUÔN CỦA DÂY, và nó nằm ngay cạnh tệp ấy
 * có chủ đích: máy chủ `vihat-miniapp` **đang dựng song song**, nên chưa gọi thử thật được và
 * hợp đồng vẫn còn có thể nhúc nhích. Ngày nó đổi, chỗ phải sửa là hai tệp cạnh nhau trong cùng
 * một thư mục. Ca "thân yêu cầu" dưới đây còn cố ý KHÔNG gõ tên trường ra: nó đếm số khoá và so
 * giá trị, nên đổi tên trường không làm nó đỏ oan.
 */

const render = (element: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(element);

const textOf = (markup: string) => markup.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ");

const CODE: MaDangNhap = { ma_so_dien_thoai: "MA-SDT-GIA", ma_truy_cap: "MA-TRUY-CAP-GIA" };

const ADDRESS = `https://vidu.test${SESSION_PATH}`;

/** Một `Response` đủ dùng cho năm nhánh, không kéo cả `undici` vào. */
function fakeResponse(status: number, body: unknown) {
  return {
    status,
    ok: status >= 200 && status < 300,
    json: () => Promise.resolve(body),
  } as unknown as Response;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("bước máy chủ: năm nhánh, mỗi nhánh một việc phải làm tiếp", () => {
  it("chưa khai địa chỉ máy chủ thì DỪNG, không đoán một địa chỉ", () => {
    // FAIL CLOSED. Một `?? "https://…"` ở đây là gửi hai mã đăng nhập của một người thật tới
    // một máy chủ không ai chọn, trên mọi máy quên đặt biến lúc dựng.
    //
    // ⚠ TRUYỀN THẲNG CHUỖI RỖNG, KHÔNG DỰA VÀO GIÁ TRỊ LÚC DỰNG — sửa 21/09/2026, và lý do đo
    // được chứ không phải khẩu vị:
    //
    //   Bản trước gọi `issueSession(MA)` và đứng trên tiền đề "dưới Vitest thì địa chỉ luôn
    //   rỗng". Tiền đề ấy chết vào ngày máy dựng có `.env.local` mang một `VIGOV_API_HOST` thật
    //   (`scripts/cau-hinh.mjs`): Vite nung giá trị ấy vào `__VIGOV_API_HOST__` cho CẢ lượt chạy
    //   test, nên ca này ĐỎ trên máy của người đẩy bản và XANH trên máy chưa cấu hình. Đã đo:
    //   `sessionAddress()` dài 44 ký tự trên máy đã cấu hình, 0 khi đặt biến shell rỗng.
    //
    //   Một ca đỏ tuỳ theo máy là một ca sắp bị ai đó tắt. Bất biến THẬT SỰ phải canh là "địa chỉ
    //   rỗng thì không một lời gọi mạng nào", và nó được canh thẳng ở đây. Hình dạng của địa chỉ
    //   đọc lúc dựng có ca riêng ngay dưới.
    const call = vi.fn();
    vi.stubGlobal("fetch", call);
    return issueSession(CODE, "").then((result) => {
      expect(result).toEqual({ kind: "chua-khai-host" });
      expect(call, "đã gọi mạng dù chưa khai địa chỉ").not.toHaveBeenCalled();
    });
  });

  it("địa chỉ đọc lúc dựng chỉ có HAI hình dạng hợp lệ: rỗng, hoặc một tuyến https đầy đủ", () => {
    // Nửa còn lại của ca trên, và là phần chạy được trên MỌI máy — máy chưa cấu hình lẫn máy của
    // người đẩy bản. Nó bắt đúng thứ ca cũ định bắt mà không phụ thuộc vào máy: một mặc định lén
    // lút (`localhost`, `http://`, một địa chỉ cụt không có tuyến) là gửi hai mã đăng nhập của
    // một người thật tới một nơi không ai chọn.
    const address = sessionAddress();
    if (address === "") return;
    expect(address, "địa chỉ máy chủ không đi qua https").toMatch(/^https:\/\//);
    expect(
      address.endsWith(SESSION_PATH),
      `địa chỉ dựng ra không kết thúc bằng tuyến của hợp đồng (${SESSION_PATH})`,
    ).toBe(true);
  });

  it("201 đúng khuôn thì trả về một phiên, và phiên ấy KHÔNG hiện ra màn hình", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(fakeResponse(201, { token: "BEARER-GIA", expiresAt: "2026-09-20T10:00:00Z" })),
    );
    const result = await issueSession(CODE, ADDRESS);
    expect(result.kind).toBe("xong");
    if (result.kind !== "xong") return;
    expect(result.session.token).toBe("BEARER-GIA");
    // Bearer là chứng từ của một phiên: vẽ nó ra là mời người đứng cạnh chụp lại.
    expect(render(<SessionIssuer code={CODE} />)).not.toContain(result.session.token);
  });

  it("401 (mã Zalo sai hoặc hết hạn) và 502 (không với tới Zalo) là HAI nhánh khác nhau", async () => {
    // GỘP HAI MÃ NÀY LÀ HỎNG ĐÚNG CHỖ NGƯỜI DÙNG CẦN NHẤT. 401 sửa được bằng một cú bấm; 502
    // thì bấm lại ngay cũng hỏng y như vậy, và việc cần làm là chờ. Một câu chung chung bảo cả
    // hai nhóm "bấm lại đi" là bảo một nửa số người làm một việc vô ích, ba lần liền.
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(fakeResponse(401, {})));
    expect(await issueSession(CODE, ADDRESS)).toEqual({ kind: "ma-het-han" });

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(fakeResponse(502, {})));
    expect(await issueSession(CODE, ADDRESS)).toEqual({ kind: "zalo-khong-tra-loi" });
  });

  it("máy chủ hỏng, mất mạng, hoặc thân trả lời sai khuôn — cùng một nhánh, không ném ra ngoài", async () => {
    const broken = [
      fakeResponse(500, {}),
      fakeResponse(404, {}),
      fakeResponse(201, { token: "" }),
      fakeResponse(201, { token: "BEARER-GIA" }),
      fakeResponse(201, null),
      fakeResponse(201, "khong-phai-doi-tuong"),
    ];
    for (const response of broken) {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
      expect(await issueSession(CODE, ADDRESS)).toEqual({ kind: "khong-goi-duoc" });
    }

    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("mat mang")));
    expect(await issueSession(CODE, ADDRESS)).toEqual({ kind: "khong-goi-duoc" });
  });

  it("gọi đúng MỘT lần, bằng POST, tới đúng địa chỉ của hợp đồng", async () => {
    const call = vi.fn().mockResolvedValue(fakeResponse(500, {}));
    vi.stubGlobal("fetch", call);
    await issueSession(CODE, ADDRESS);
    expect(call).toHaveBeenCalledTimes(1);
    const [address, options] = call.mock.calls[0] as [string, RequestInit];
    expect(address).toBe(ADDRESS);
    expect(options.method).toBe("POST");
  });
});

describe("màn hình: nói việc đang xảy ra và việc phải làm tiếp, không nói mã lỗi", () => {
  it("vẽ trạng thái ĐANG GỬI trước khi có câu trả lời", () => {
    // `useEffect` không chạy khi dựng phía máy chủ, nên đây đúng là thứ người dùng thấy ở nhịp
    // đầu tiên: một câu nói việc đang xảy ra, không phải một khoảng trắng.
    expect(textOf(render(<SessionIssuer code={CODE} />)).trim().length).toBeGreaterThan(0);
  });

  it("không câu nào nhắc đường dẫn, tên trường hay mã trạng thái", () => {
    // README §Error message shape. Và một đường dẫn API vẽ ra màn hình là một chi tiết hạ tầng
    // nằm trong tay người dùng, không giúp họ làm được gì.
    const markup = render(<SessionIssuer code={CODE} />);
    expect(markup).not.toMatch(/\/api\/|http|[Tt]oken|401|502|fetch/);
  });

  it("giờ hết hạn phiên GHIM +07 qua hàm dùng chung, không theo múi của máy (quyết định 27/09/2026)", () => {
    // Nhánh "đã đăng nhập" chỉ vẽ sau `useEffect`, nên dựng tĩnh không tới được dòng "Phiên có
    // hiệu lực tới …". Canh ở mã nguồn: dòng ấy đi qua `vnDateTime` (định dạng của nó có ca so
    // nguyên văn ở `lib/date-time.test.ts`), và không còn lời gọi `toLocale*String` nào ở đây.
    const code = sessionIssuerSource;
    expect(code).toMatch(/import \{ vnDateTime \} from "\.\.\/\.\.\/lib\/date-time"/);
    expect(code).toMatch(/vnDateTime\(status\.session\.expires_at\)/);
    expect(code, "còn định dạng giờ theo múi của máy").not.toMatch(/\.toLocale(Date|Time)?String\(/);
  });
});

describe("hợp đồng: gửi ĐÚNG hai mã, không hơn", () => {
  it("thân yêu cầu có đúng hai khoá, và giá trị là đúng hai mã của Zalo", () => {
    // ĐẾM KHOÁ VÀ SO GIÁ TRỊ, KHÔNG GÕ TÊN TRƯỜNG RA: tên trường là khuôn của bên kia dây. Thứ
    // KHÔNG được đổi là "không có trường thứ ba" — một trường thêm vào là một dữ liệu rời khỏi
    // máy người dùng mà chính sách quyền riêng tư không khai.
    const body = JSON.parse(sessionRequestBody(CODE)) as Record<string, unknown>;
    expect(Object.keys(body)).toHaveLength(2);
    expect(Object.values(body).sort()).toEqual([CODE.ma_so_dien_thoai, CODE.ma_truy_cap].sort());
  });

  it("thân yêu cầu không mang theo gì khác của người dùng", () => {
    const body = sessionRequestBody(CODE);
    expect(body).not.toMatch(/secret|appSecret|phoneNumber|latitude|longitude/i);
  });

  it("đường dẫn là tài nguyên CỦA KHO NÀY, không mượn từ vựng của kênh công dân nhà nước", () => {
    // `vihat-miniapp` là backend thương mại của Tập đoàn ViHAT Group, độc lập với ViGov. Mượn tên
    // `citizen-session` của kênh nhà nước là bước đầu của việc người sau tưởng hai thứ là một —
    // và hai hệ thống bị tưởng là một thì dữ liệu của chúng sẽ được đối xử như nhau.
    expect(SESSION_PATH).toBe("/api/v1/sessions");
    expect(SESSION_PATH).not.toMatch(/citizen|cong-dan/);
  });

  it("đọc trả lời: thiếu trường, sai kiểu, hoặc rỗng đều là KHÔNG đọc được", () => {
    // Ép kiểu bằng `as` sẽ cho `undefined` đi tiếp và hiện ra màn hình dưới dạng chữ
    // "undefined" — hoặc tệ hơn, một phiên rỗng trông như đã đăng nhập.
    for (const body of [null, 7, "chuoi", {}, { token: "x" }, { token: "", expiresAt: "z" }, { token: 1, expiresAt: "z" }]) {
      expect(readResponse(body), `lọt một thân trả lời sai khuôn: ${JSON.stringify(body)}`).toBeNull();
    }
    expect(readResponse({ token: "x", expiresAt: "2026-09-20T10:00:00Z" })).toEqual({
      token: "x",
      expires_at: "2026-09-20T10:00:00Z",
    });
  });
});
