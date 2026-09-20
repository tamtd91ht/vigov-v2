import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { MaDangNhap } from "../tinh-nang/zalo-api";

import { docTraLoi, DUONG_DAN_PHIEN, thanYeuCau } from "./hop-dong";
import { phatHanhPhien } from "./goi-may-chu";
import { PhatHanhPhien } from "./PhatHanhPhien";

/**
 * KHỐI ĐĂNG NHẬP — PHÉP KIỂM CỦA BƯỚC MÁY CHỦ.
 *
 * BỐN ĐIỀU Ở ĐÂY KHÔNG CÓ PHÉP KIỂM NÀO KHÁC NÓI HỘ:
 *
 *   1. **Năm nhánh kết quả, mỗi nhánh một việc phải làm tiếp.** Địa chỉ máy chủ đọc lúc DỰNG
 *      nên dưới Vitest nó luôn rỗng — không có tham số địa chỉ của `phatHanhPhien` thì bốn
 *      nhánh còn lại không ca nào chạm tới, và một hàm không ai kiểm là một hàm sẽ hỏng đúng
 *      ngày máy chủ thật trả về thứ nó không ngờ.
 *   2. **401 và 502 KHÔNG được gộp.** Hai mã ấy dẫn tới hai việc khác nhau: bấm lại ngay, hay
 *      chờ rồi thử lại. Gộp là bảo một nửa số người làm một việc vô ích.
 *   3. **Thân yêu cầu mang ĐÚNG hai mã, không hơn.** Một trường thêm vào là một dữ liệu rời
 *      khỏi máy người dùng mà chính sách quyền riêng tư không khai.
 *   4. **Bearer không bao giờ ra màn hình.**
 *
 * ⚠ TỆP NÀY LÀ NƠI DUY NHẤT NGOÀI `hop-dong.ts` BIẾT KHUÔN CỦA DÂY, và nó nằm ngay cạnh tệp ấy
 * có chủ đích: máy chủ `vihat-miniapp` **đang dựng song song**, nên chưa gọi thử thật được và
 * hợp đồng vẫn còn có thể nhúc nhích. Ngày nó đổi, chỗ phải sửa là hai tệp cạnh nhau trong cùng
 * một thư mục. Ca "thân yêu cầu" dưới đây còn cố ý KHÔNG gõ tên trường ra: nó đếm số khoá và so
 * giá trị, nên đổi tên trường không làm nó đỏ oan.
 */

const ve = (phan_tu: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(phan_tu);

const textOf = (markup: string) => markup.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ");

const MA: MaDangNhap = { ma_so_dien_thoai: "MA-SDT-GIA", ma_truy_cap: "MA-TRUY-CAP-GIA" };

const DIA_CHI = `https://vidu.test${DUONG_DAN_PHIEN}`;

/** Một `Response` đủ dùng cho năm nhánh, không kéo cả `undici` vào. */
function traLoiGia(status: number, than: unknown) {
  return {
    status,
    ok: status >= 200 && status < 300,
    json: () => Promise.resolve(than),
  } as unknown as Response;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("bước máy chủ: năm nhánh, mỗi nhánh một việc phải làm tiếp", () => {
  it("chưa khai địa chỉ máy chủ thì DỪNG, không đoán một địa chỉ", () => {
    // FAIL CLOSED. Một `?? "https://…"` ở đây là gửi hai mã đăng nhập của một người thật tới
    // một máy chủ không ai chọn, trên mọi máy quên đặt biến lúc dựng.
    const goi = vi.fn();
    vi.stubGlobal("fetch", goi);
    return phatHanhPhien(MA).then((ket_qua) => {
      expect(ket_qua).toEqual({ kieu: "chua-khai-host" });
      expect(goi, "đã gọi mạng dù chưa khai địa chỉ").not.toHaveBeenCalled();
    });
  });

  it("201 đúng khuôn thì trả về một phiên, và phiên ấy KHÔNG hiện ra màn hình", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(traLoiGia(201, { token: "BEARER-GIA", expiresAt: "2026-09-20T10:00:00Z" })),
    );
    const ket_qua = await phatHanhPhien(MA, DIA_CHI);
    expect(ket_qua.kieu).toBe("xong");
    if (ket_qua.kieu !== "xong") return;
    expect(ket_qua.phien.token).toBe("BEARER-GIA");
    // Bearer là chứng từ của một phiên: vẽ nó ra là mời người đứng cạnh chụp lại.
    expect(ve(<PhatHanhPhien ma={MA} />)).not.toContain(ket_qua.phien.token);
  });

  it("401 (mã Zalo sai hoặc hết hạn) và 502 (không với tới Zalo) là HAI nhánh khác nhau", async () => {
    // GỘP HAI MÃ NÀY LÀ HỎNG ĐÚNG CHỖ NGƯỜI DÙNG CẦN NHẤT. 401 sửa được bằng một cú bấm; 502
    // thì bấm lại ngay cũng hỏng y như vậy, và việc cần làm là chờ. Một câu chung chung bảo cả
    // hai nhóm "bấm lại đi" là bảo một nửa số người làm một việc vô ích, ba lần liền.
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(traLoiGia(401, {})));
    expect(await phatHanhPhien(MA, DIA_CHI)).toEqual({ kieu: "ma-het-han" });

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(traLoiGia(502, {})));
    expect(await phatHanhPhien(MA, DIA_CHI)).toEqual({ kieu: "zalo-khong-tra-loi" });
  });

  it("máy chủ hỏng, mất mạng, hoặc thân trả lời sai khuôn — cùng một nhánh, không ném ra ngoài", async () => {
    const hong = [
      traLoiGia(500, {}),
      traLoiGia(404, {}),
      traLoiGia(201, { token: "" }),
      traLoiGia(201, { token: "BEARER-GIA" }),
      traLoiGia(201, null),
      traLoiGia(201, "khong-phai-doi-tuong"),
    ];
    for (const tra_loi of hong) {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(tra_loi));
      expect(await phatHanhPhien(MA, DIA_CHI)).toEqual({ kieu: "khong-goi-duoc" });
    }

    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("mat mang")));
    expect(await phatHanhPhien(MA, DIA_CHI)).toEqual({ kieu: "khong-goi-duoc" });
  });

  it("gọi đúng MỘT lần, bằng POST, tới đúng địa chỉ của hợp đồng", async () => {
    const goi = vi.fn().mockResolvedValue(traLoiGia(500, {}));
    vi.stubGlobal("fetch", goi);
    await phatHanhPhien(MA, DIA_CHI);
    expect(goi).toHaveBeenCalledTimes(1);
    const [dia_chi, tuy_chon] = goi.mock.calls[0] as [string, RequestInit];
    expect(dia_chi).toBe(DIA_CHI);
    expect(tuy_chon.method).toBe("POST");
  });
});

describe("màn hình: nói việc đang xảy ra và việc phải làm tiếp, không nói mã lỗi", () => {
  it("vẽ trạng thái ĐANG GỬI trước khi có câu trả lời", () => {
    // `useEffect` không chạy khi dựng phía máy chủ, nên đây đúng là thứ người dùng thấy ở nhịp
    // đầu tiên: một câu nói việc đang xảy ra, không phải một khoảng trắng.
    expect(textOf(ve(<PhatHanhPhien ma={MA} />)).trim().length).toBeGreaterThan(0);
  });

  it("không câu nào nhắc đường dẫn, tên trường hay mã trạng thái", () => {
    // README §Error message shape. Và một đường dẫn API vẽ ra màn hình là một chi tiết hạ tầng
    // nằm trong tay người dùng, không giúp họ làm được gì.
    const ma_nguon = ve(<PhatHanhPhien ma={MA} />);
    expect(ma_nguon).not.toMatch(/\/api\/|http|[Tt]oken|401|502|fetch/);
  });
});

describe("hợp đồng: gửi ĐÚNG hai mã, không hơn", () => {
  it("thân yêu cầu có đúng hai khoá, và giá trị là đúng hai mã của Zalo", () => {
    // ĐẾM KHOÁ VÀ SO GIÁ TRỊ, KHÔNG GÕ TÊN TRƯỜNG RA: tên trường là khuôn của bên kia dây. Thứ
    // KHÔNG được đổi là "không có trường thứ ba" — một trường thêm vào là một dữ liệu rời khỏi
    // máy người dùng mà chính sách quyền riêng tư không khai.
    const than = JSON.parse(thanYeuCau(MA)) as Record<string, unknown>;
    expect(Object.keys(than)).toHaveLength(2);
    expect(Object.values(than).sort()).toEqual([MA.ma_so_dien_thoai, MA.ma_truy_cap].sort());
  });

  it("thân yêu cầu không mang theo gì khác của người dùng", () => {
    const than = thanYeuCau(MA);
    expect(than).not.toMatch(/secret|appSecret|phoneNumber|latitude|longitude/i);
  });

  it("đường dẫn là tài nguyên CỦA KHO NÀY, không mượn từ vựng của kênh công dân nhà nước", () => {
    // `vihat-miniapp` là backend thương mại của VihatSoftware, độc lập với ViGov. Mượn tên
    // `citizen-session` của kênh nhà nước là bước đầu của việc người sau tưởng hai thứ là một —
    // và hai hệ thống bị tưởng là một thì dữ liệu của chúng sẽ được đối xử như nhau.
    expect(DUONG_DAN_PHIEN).toBe("/api/v1/sessions");
    expect(DUONG_DAN_PHIEN).not.toMatch(/citizen|cong-dan/);
  });

  it("đọc trả lời: thiếu trường, sai kiểu, hoặc rỗng đều là KHÔNG đọc được", () => {
    // Ép kiểu bằng `as` sẽ cho `undefined` đi tiếp và hiện ra màn hình dưới dạng chữ
    // "undefined" — hoặc tệ hơn, một phiên rỗng trông như đã đăng nhập.
    for (const than of [null, 7, "chuoi", {}, { token: "x" }, { token: "", expiresAt: "z" }, { token: 1, expiresAt: "z" }]) {
      expect(docTraLoi(than), `lọt một thân trả lời sai khuôn: ${JSON.stringify(than)}`).toBeNull();
    }
    expect(docTraLoi({ token: "x", expiresAt: "2026-09-20T10:00:00Z" })).toEqual({
      token: "x",
      het_han: "2026-09-20T10:00:00Z",
    });
  });
});
