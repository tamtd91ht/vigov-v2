import { createServer, type IncomingHttpHeaders, type Server, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

// `server-only` throws outside a React Server build by design; in Node tests it is inert.
vi.mock("server-only", () => ({}));

import { resolveTenant } from "./tenant-config";

/**
 * Suy ra xã từ `Host` là đường cách ly giữa hai cơ quan nhà nước. Mọi ca dưới đây đều là ca
 * KHÔNG nhìn thấy bằng mắt trên màn hình: một xã dự phòng, một 404 nuốt mất sự cố, hay một
 * `tenant_id` lọt vào yêu cầu đều chạy đúng ở xã đầu tiên và chỉ lộ ra ở xã thứ hai.
 *
 * ĐỔI 25/09/2026: bộ kiểm cũ thay `globalThis.fetch` rồi đọc `mock.calls[0]` — mà `Headers`
 * GIỮ `host` khi đọc lại trong khi undici gửi host khác trên dây, nên ca "mang Host của xã" có
 * thể XANH trong khi sản xuất gửi sai (sổ `web-admin/goc-api-noi-bo`). Nay mọi ca đọc cái mà
 * MỘT MÁY CHỦ `node:http` THẬT nhận được, và hai ghim URL `https://<Host>/api/v1/…` cũ đã đổi
 * theo thiết kế: lời gọi đi tới gốc nội bộ `IDENTITY_HTTP_ADDR`, Host của xã nằm ở header.
 */

const XA_A = "tanphu.example.gov.vn";

type DaNhan = { method: string; url: string; headers: IncomingHttpHeaders };

let nhan: DaNhan[] = [];
let traLoi: (res: ServerResponse) => void = () => {};
let mayChu: Server;
let goc = "";

function dau(): DaNhan {
  const d = nhan[0];
  if (!d) throw new Error("máy giả chưa nhận lời gọi nào");
  return d;
}

beforeAll(async () => {
  mayChu = createServer((req, res) => {
    nhan.push({ method: req.method ?? "", url: req.url ?? "", headers: req.headers });
    req.resume();
    req.on("end", () => traLoi(res));
  });
  await new Promise<void>((ok) => mayChu.listen(0, "127.0.0.1", ok));
  goc = `http://127.0.0.1:${(mayChu.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise((ok) => mayChu.close(ok));
});

beforeEach(() => {
  process.env.IDENTITY_HTTP_ADDR = goc;
});

afterEach(() => {
  nhan = [];
  traLoi = () => {};
  delete process.env.IDENTITY_HTTP_ADDR;
});

/** Thân 200 đúng hình dạng `identity.thongTinXa` của hợp đồng. */
function thongTinXa(than: { name: string; host: string; province: string }) {
  traLoi = (res) => {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(than));
  };
}

/** Thân lỗi đúng hình dạng `httpx.Error`. */
function loiCuaMayChu(status: number, code: string, message: string) {
  traLoi = (res) => {
    res.writeHead(status, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ code, message, trace_id: "" }));
  };
}

describe("resolveTenant — hình dạng lời gọi, đọc trên dây", () => {
  it("hỏi đúng tuyến công khai ở gốc NỘI BỘ, với Host của xã trên dây", async () => {
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "Thành phố Đà Nẵng" });
    await resolveTenant(XA_A);

    expect(nhan).toHaveLength(1);
    expect(dau().method).toBe("GET");
    expect(dau().url).toBe("/api/v1/communes/current");
    // Kết nối tới 127.0.0.1, nhưng biên Go đọc `r.Host` — và đó phải là xã, không phải gốc nội bộ.
    expect(dau().headers.host).toBe(XA_A);
  });

  it("không có `tenant_id` ở bất kỳ đâu — đường dẫn, truy vấn hay header", async () => {
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" });
    await resolveTenant(XA_A);

    const tatCa = [dau().url, JSON.stringify(dau().headers)].join(" ");
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã LÀ cái host đang hỏi.
    expect(tatCa).not.toMatch(/tenant|xa_id/i);
  });

  it("không gửi cookie hay thông tin đăng nhập nào", async () => {
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" });
    await resolveTenant(XA_A);

    // Tuyến công khai: không có gì để uỷ quyền, nên không có cookie nào được chuyển tiếp.
    expect(dau().headers.cookie).toBeUndefined();
    expect(dau().headers.authorization).toBeUndefined();
  });

  it("host viết hoa vẫn ra một Host duy nhất — biên cũng hạ chữ trước khi tra", async () => {
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" });
    await resolveTenant("TanPhu.Example.Gov.VN");

    expect(dau().headers.host).toBe(XA_A);
  });
});

describe("resolveTenant — 200", () => {
  it("`province` của hợp đồng về đúng `parentAuthority` của tầng web (ADR 0017)", async () => {
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "Thành phố Đà Nẵng" });
    const xa = await resolveTenant(XA_A);

    expect(xa).toEqual({
      host: XA_A,
      displayName: "UBND xã Tân Phú",
      parentAuthority: "Thành phố Đà Nẵng",
    });
  });

  it("không mang `tenantId` và không mang `active` — hai trường hợp đồng CỐ Ý không trả", async () => {
    // Test này đỏ vào đúng ngày có người thêm lại một trong hai. `tenantId`: một định danh nội
    // bộ mà client cầm là một định danh client gửi ngược lên được (luật 1, cấm #2). `active`:
    // biên đã trả 404 cho xã ngừng hoạt động trước mọi handler, nên ở đây nó chỉ có thể là
    // `true` — một nhánh "xã đã sáp nhập" dựng trên nó sẽ không bao giờ chạy.
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" });
    const xa = await resolveTenant(XA_A);

    expect(Object.keys(xa ?? {}).sort()).toEqual(["displayName", "host", "parentAuthority"]);
  });

  it("tỉnh/thành rỗng đi nguyên vẹn thành chuỗi rỗng — không bịa, không mặc định", async () => {
    // "" nghĩa là xã chưa khai tỉnh/thành. Đoán một giá trị là để một cơ quan nhà nước tự nói
    // một điều không đúng về chính mình.
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" });
    const xa = await resolveTenant(XA_A);

    expect(xa?.parentAuthority).toBe("");
  });

  it("`host` lấy theo sổ đăng ký, không lấy lại chuỗi người gọi vừa gõ", async () => {
    thongTinXa({ name: "UBND xã Tân Phú", host: XA_A, province: "" });
    const xa = await resolveTenant("bi-danh.example.gov.vn");

    expect(xa?.host).toBe(XA_A);
  });
});

describe("resolveTenant — không có xã nào", () => {
  it("404 trả `null` để bên gọi ra 404, không rơi về xã mặc định", async () => {
    loiCuaMayChu(404, "tenant_not_found", "Không tìm thấy trang cho tên miền này.");
    expect(await resolveTenant("khong-ai-biet.example.gov.vn")).toBeNull();
  });

  it("xã đã ngừng hoạt động cũng là 404 ấy — hai ca, MỘT câu trả lời", async () => {
    // Biên trả cùng một 404 cho "chưa bao giờ có" và "đã ngừng hoạt động", để người gõ thử tên
    // miền không dò ra được xã nào từng tồn tại (`core/httpx/edge.go`). Phía web không dựng
    // lại sự phân biệt ấy.
    loiCuaMayChu(404, "tenant_not_found", "Không tìm thấy trang cho tên miền này.");
    expect(await resolveTenant("da-sap-nhap.example.gov.vn")).toBeNull();
  });

  it("host không phải một tên miền thì từ chối NGAY, không gọi đi đâu cả", async () => {
    thongTinXa({ name: "x", host: "x", province: "" });

    for (const xau of ["", "   ", "xa.example.gov.vn/khac", "ai-do@noi-khac.example", "a b"]) {
      expect(await resolveTenant(xau)).toBeNull();
    }
    expect(nhan).toHaveLength(0);
  });
});

describe("resolveTenant — máy chủ không trả lời được", () => {
  it("500 thì NÉM, không trả `null`: mất liên lạc không phải là 'xã này không tồn tại'", async () => {
    // Trả `null` ở đây sẽ dựng trang 404, tức là trong lúc sự cố thì hệ thống nói với người
    // vận hành rằng một xã có thật là không có thật.
    loiCuaMayChu(500, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.");
    await expect(resolveTenant(XA_A)).rejects.toThrow();
  });

  it("503 cũng vậy", async () => {
    loiCuaMayChu(503, "unavailable", "Hệ thống đang bảo trì.");
    await expect(resolveTenant(XA_A)).rejects.toThrow();
  });

  it("3xx thì NÉM và KHÔNG đi theo — đúng cú 307 đã làm /dang-nhap 500 trên sản xuất", async () => {
    // Một chuyển hướng được đi theo là một đường để máy chủ đọc cấu hình của xã KHÁC rồi in ra
    // dưới host này. Máy chủ giả chỉ nhận ĐÚNG MỘT lời gọi: không có lần thứ hai tới `Location`.
    traLoi = (res) => {
      res.writeHead(307, { Location: `${goc}/dang-nhap` });
      res.end();
    };
    await expect(resolveTenant(XA_A)).rejects.toThrow(/307/);
    expect(nhan).toHaveLength(1);
  });

  it("mạng hỏng thì ném ra ngoài, không nuốt thành một xã rỗng", async () => {
    const tam = createServer();
    await new Promise<void>((ok) => tam.listen(0, "127.0.0.1", ok));
    const cong = (tam.address() as AddressInfo).port;
    await new Promise((ok) => tam.close(ok));
    process.env.IDENTITY_HTTP_ADDR = `http://127.0.0.1:${cong}`;

    await expect(resolveTenant(XA_A)).rejects.toThrow();
  });

  it("IDENTITY_HTTP_ADDR hỏng thì ném, nêu tên biến", async () => {
    process.env.IDENTITY_HTTP_ADDR = "http://identity:8080/duong-dan";
    await expect(resolveTenant(XA_A)).rejects.toThrow(/IDENTITY_HTTP_ADDR/);
    expect(nhan).toHaveLength(0);
  });
});
