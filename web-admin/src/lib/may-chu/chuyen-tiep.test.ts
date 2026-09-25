import { createServer, type IncomingHttpHeaders, type Server, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";

// `server-only` throws outside a React Server build by design; in Node tests it is inert.
vi.mock("server-only", () => ({}));

import { chuSoHuu, chuyenTiep } from "./chuyen-tiep";
import { gocDichVu } from "./goc-dich-vu";

/**
 * EVERY CASE HERE READS WHAT A REAL SOCKET RECEIVED. The ledger (`web-admin/goc-api-noi-bo`)
 * measured that a `Headers` object keeps `host` when read back while undici puts a different
 * one on the wire — so a test asserting on a mocked call's headers goes green over the exact
 * defect this gateway exists to avoid. The fake upstream below is a real `node:http` server.
 */

const XA = "tanphu.example.gov.vn";
const BIEN = [
  "IDENTITY_HTTP_ADDR",
  "DOCUMENTS_HTTP_ADDR",
  "PETITIONS_HTTP_ADDR",
  "FINANCE_HTTP_ADDR",
  "COMMS_HTTP_ADDR",
] as const;

type DaNhan = { dichVu: string; method: string; url: string; headers: IncomingHttpHeaders; than: Buffer };

let nhan: DaNhan[] = [];
let traLoi: (res: ServerResponse, da: DaNhan) => void = (res) => res.end("{}");
const mayChu: Server[] = [];

function dau(): DaNhan {
  const d = nhan[0];
  if (!d) throw new Error("máy giả chưa nhận lời gọi nào");
  return d;
}

/** One fake upstream per service, each tagging what it received with its own name. */
async function dungMayGia(dichVu: string): Promise<string> {
  const s = createServer((req, res) => {
    const khoi: Buffer[] = [];
    req.on("data", (k: Buffer) => khoi.push(k));
    req.on("end", () => {
      const da = {
        dichVu,
        method: req.method ?? "",
        url: req.url ?? "",
        headers: req.headers,
        than: Buffer.concat(khoi),
      };
      nhan.push(da);
      traLoi(res, da);
    });
  });
  await new Promise<void>((ok) => s.listen(0, "127.0.0.1", ok));
  mayChu.push(s);
  return `http://127.0.0.1:${(s.address() as AddressInfo).port}`;
}

const GOC: Record<string, string> = {};

beforeAll(async () => {
  for (const [bien, dv] of [
    ["IDENTITY_HTTP_ADDR", "identity"],
    ["DOCUMENTS_HTTP_ADDR", "documents"],
    ["PETITIONS_HTTP_ADDR", "petitions"],
    ["FINANCE_HTTP_ADDR", "finance"],
    ["COMMS_HTTP_ADDR", "comms"],
  ] as const) {
    GOC[bien] = await dungMayGia(dv);
  }
});

afterAll(async () => {
  await Promise.all(mayChu.map((s) => new Promise((ok) => s.close(ok))));
});

function datBien() {
  for (const b of BIEN) process.env[b] = GOC[b];
}

afterEach(() => {
  nhan = [];
  traLoi = (res) => res.end("{}");
  for (const b of BIEN) delete process.env[b];
  vi.restoreAllMocks();
});

function yeuCau(duong: string, init: RequestInit & { headers?: Record<string, string> } = {}) {
  return new Request(`http://${XA}${duong}`, {
    ...init,
    headers: { host: XA, ...init.headers },
  });
}

describe("chuyenTiep — Host trên dây", () => {
  it("dịch vụ nhận ĐÚNG host của xã, trong khi kết nối tới 127.0.0.1", async () => {
    datBien();
    const phanHoi = await chuyenTiep(yeuCau("/api/v1/staff?page=2&q=a%20b"));

    expect(phanHoi.status).toBe(200);
    expect(nhan).toHaveLength(1);
    expect(dau().headers.host).toBe(XA);
    // Path and query go through unchanged — encoding included.
    expect(dau().url).toBe("/api/v1/staff?page=2&q=a%20b");
    expect(dau().method).toBe("GET");
  });

  it("không có Host thì 404 và KHÔNG gọi đi đâu cả", async () => {
    datBien();
    const phanHoi = await chuyenTiep(
      new Request(`http://${XA}/api/v1/staff`, { headers: { host: "" } }),
    );

    expect(phanHoi.status).toBe(404);
    expect((await phanHoi.json()).code).toBe("tenant_not_found");
    expect(nhan).toHaveLength(0);
  });
});

describe("chuyenTiep — header đi lên", () => {
  it("mọi header x-tenant* bị bỏ, không phân biệt hoa thường (luật 1, cấm #2)", async () => {
    datBien();
    await chuyenTiep(
      yeuCau("/api/v1/roles", {
        headers: { "X-Tenant-Id": "01HXYZ", "x-tenant": "a", "X-TENANT-HOST": "khac.example" },
      }),
    );

    expect(Object.keys(dau().headers).filter((h) => h.startsWith("x-tenant"))).toEqual([]);
  });

  it("forwarded và x-forwarded-host bị bỏ — Host là lời khai duy nhất về xã", async () => {
    datBien();
    await chuyenTiep(
      yeuCau("/api/v1/roles", {
        headers: { forwarded: "host=khac.example", "x-forwarded-host": "khac.example" },
      }),
    );

    expect(dau().headers.forwarded).toBeUndefined();
    expect(dau().headers["x-forwarded-host"]).toBeUndefined();
  });

  it("header hop-by-hop và header được khai trong Connection bị bỏ; cookie đi nguyên", async () => {
    datBien();
    await chuyenTiep(
      yeuCau("/api/v1/roles", {
        headers: {
          connection: "x-rieng-mot-chang",
          "x-rieng-mot-chang": "1",
          "keep-alive": "timeout=5",
          "proxy-authorization": "Basic xxx",
          te: "trailers",
          upgrade: "h2c",
          cookie: "vigov_session=abc",
          "x-giu-lai": "1",
        },
      }),
    );

    const h = dau().headers;
    expect(h["x-rieng-mot-chang"]).toBeUndefined();
    expect(h["keep-alive"]).toBeUndefined();
    expect(h["proxy-authorization"]).toBeUndefined();
    expect(h.te).toBeUndefined();
    expect(h.upgrade).toBeUndefined();
    // Go authenticates every call itself — the session cookie must reach it.
    expect(h.cookie).toBe("vigov_session=abc");
    expect(h["x-giu-lai"]).toBe("1");
  });

  it("X-Forwarded-For đi NGUYÊN — route handler không thấy socket, xem chú thích ở chuyen-tiep.ts", async () => {
    // Next fills XFF from the socket peer ONLY when it is absent (base-server.js:612), so what
    // reaches the handler is already "the peer" or "the chain a proxy sent". Appending anything
    // here would need the socket, which Next does not hand to route handlers.
    datBien();
    await chuyenTiep(yeuCau("/api/v1/roles", { headers: { "x-forwarded-for": "203.0.113.7, 10.0.0.5" } }));

    expect(dau().headers["x-forwarded-for"]).toBe("203.0.113.7, 10.0.0.5");
  });
});

describe("chuyenTiep — chọn dịch vụ theo ĐOẠN đường dẫn", () => {
  it("/api/v1/roles và /api/v1/role-permissions về đúng chủ, dù cùng chung tiền tố ký tự", async () => {
    datBien();
    await chuyenTiep(yeuCau("/api/v1/roles/7"));
    await chuyenTiep(yeuCau("/api/v1/role-permissions"));
    await chuyenTiep(yeuCau("/api/v1/tasks"));
    await chuyenTiep(yeuCau("/api/v1/incoming-documents/abc"));

    expect(nhan.map((n) => n.dichVu)).toEqual(["identity", "identity", "petitions", "documents"]);
  });

  it("chuSoHuu không khớp theo ký tự: /api/v1/rolesX không phải của ai", () => {
    expect(chuSoHuu("/api/v1/roles")).toBe("identity");
    expect(chuSoHuu("/api/v1/rolesX")).toBeNull();
    expect(chuSoHuu("/api/v1/task-types/1")).toBe("petitions");
    expect(chuSoHuu("/api/v1/tasks-khac")).toBeNull();
  });

  it("tiền tố không ai sở hữu → 404 dạng httpx.Error, KHÔNG gọi dịch vụ mặc định nào", async () => {
    datBien();
    const phanHoi = await chuyenTiep(yeuCau("/api/v1/khong-ai-so-huu"));

    expect(phanHoi.status).toBe(404);
    expect(await phanHoi.json()).toEqual({
      code: "not_found",
      message: "Không tìm thấy đường dẫn này.",
      trace_id: "",
    });
    expect(nhan).toHaveLength(0);
  });
});

describe("chuyenTiep — thân yêu cầu và phản hồi", () => {
  it("thân > 1MB gửi dạng luồng tới nơi nguyên vẹn từng byte", async () => {
    datBien();
    const tong = 3 * 1024 * 1024 + 17;
    const goc = Buffer.alloc(tong);
    for (let i = 0; i < tong; i++) goc[i] = (i * 31) % 251;

    // Delivered as a STREAM in 64 KiB chunks, so the forwarder cannot rely on a known length.
    let viTri = 0;
    const luong = new ReadableStream<Uint8Array>({
      pull(c) {
        if (viTri >= tong) return c.close();
        const het = Math.min(viTri + 65536, tong);
        c.enqueue(new Uint8Array(goc.subarray(viTri, het)));
        viTri = het;
      },
    });
    const phanHoi = await chuyenTiep(
      new Request(`http://${XA}/api/v1/incoming-documents/x/attachments`, {
        method: "POST",
        headers: { host: XA, "content-type": "application/octet-stream" },
        body: luong,
        duplex: "half",
      } as RequestInit),
    );

    expect(phanHoi.status).toBe(200);
    expect(dau().than.length).toBe(tong);
    expect(dau().than.equals(goc)).toBe(true);
    expect(dau().headers["content-type"]).toBe("application/octet-stream");
  });

  it("nhiều Set-Cookie giữ riêng từng cái; status và header phản hồi đi nguyên", async () => {
    datBien();
    traLoi = (res) => {
      res.statusCode = 201;
      res.setHeader("Set-Cookie", [
        "vigov_session=a; Path=/; HttpOnly; Secure; SameSite=Lax; Expires=Wed, 21 Oct 2026 07:28:00 GMT",
        "vigov_csrf=b; Path=/; Secure",
      ]);
      res.setHeader("X-Vigov-Thu", "1");
      res.setHeader("Content-Type", "application/json");
      res.end('{"ok":true}');
    };
    const phanHoi = await chuyenTiep(yeuCau("/api/v1/sessions", { method: "POST", body: "{}" }));

    expect(phanHoi.status).toBe(201);
    expect(phanHoi.headers.getSetCookie()).toEqual([
      "vigov_session=a; Path=/; HttpOnly; Secure; SameSite=Lax; Expires=Wed, 21 Oct 2026 07:28:00 GMT",
      "vigov_csrf=b; Path=/; Secure",
    ]);
    expect(phanHoi.headers.get("x-vigov-thu")).toBe("1");
    expect(await phanHoi.json()).toEqual({ ok: true });
    expect(dau().than.toString()).toBe("{}");
  });

  it("header hop-by-hop của phản hồi bị bỏ", async () => {
    datBien();
    traLoi = (res) => {
      res.setHeader("Connection", "keep-alive, x-chang-sau");
      res.setHeader("X-Chang-Sau", "1");
      res.setHeader("Keep-Alive", "timeout=5");
      res.end("{}");
    };
    const phanHoi = await chuyenTiep(yeuCau("/api/v1/roles"));

    expect(phanHoi.headers.get("x-chang-sau")).toBeNull();
    expect(phanHoi.headers.get("keep-alive")).toBeNull();
    expect(phanHoi.headers.get("connection")).toBeNull();
  });

  it("204 trả về không thân", async () => {
    datBien();
    traLoi = (res) => {
      res.statusCode = 204;
      res.end();
    };
    const phanHoi = await chuyenTiep(yeuCau("/api/v1/roles/1", { method: "DELETE" }));

    expect(phanHoi.status).toBe(204);
    expect(phanHoi.body).toBeNull();
  });
});

describe("chuyenTiep — dịch vụ không trả lời", () => {
  it("không kết nối được → 502 dạng httpx.Error, không lộ địa chỉ nội bộ", async () => {
    // A port nobody listens on: bind, read the port, close.
    const tam = createServer();
    await new Promise<void>((ok) => tam.listen(0, "127.0.0.1", ok));
    const cong = (tam.address() as AddressInfo).port;
    await new Promise((ok) => tam.close(ok));
    process.env.IDENTITY_HTTP_ADDR = `http://127.0.0.1:${cong}`;
    vi.spyOn(console, "info").mockImplementation(() => {});

    const phanHoi = await chuyenTiep(yeuCau("/api/v1/roles"));
    const chu = await phanHoi.text();

    expect(phanHoi.status).toBe(502);
    expect(JSON.parse(chu).code).toBe("bad_gateway");
    expect(chu).not.toContain("127.0.0.1");
    expect(chu).not.toContain(String(cong));
  });

  it("dòng nhật ký chỉ có phương thức, dịch vụ, mã trạng thái, thời gian — không đường dẫn, không header", async () => {
    datBien();
    const ghi = vi.spyOn(console, "info").mockImplementation(() => {});
    await chuyenTiep(
      yeuCau("/api/v1/staff?phone=0900000000", { headers: { cookie: "vigov_session=bimat" } }),
    );

    expect(ghi).toHaveBeenCalledTimes(1);
    const dong = String(ghi.mock.calls[0]?.[0]);
    expect(Object.keys(JSON.parse(dong)).sort()).toEqual(["method", "ms", "msg", "service", "status"]);
    expect(dong).not.toContain("0900000000");
    expect(dong).not.toContain("bimat");
  });
});

describe("gocDichVu — biến môi trường", () => {
  it("chưa đặt hoặc rỗng → tên Service k8s, cổng 8080", () => {
    process.env.FINANCE_HTTP_ADDR = "   ";
    expect(gocDichVu("finance").href).toBe("http://finance:8080/");
    expect(gocDichVu("comms").href).toBe("http://comms:8080/");
  });

  it("giá trị hỏng thì NÉM, nêu TÊN biến và KHÔNG nêu giá trị", () => {
    for (const hong of [
      "khong-phai-url",
      "ftp://identity:21",
      "http://nguoi:matkhau@identity:8080",
      "http://identity:8080/api",
      "http://identity:8080/?a=1",
    ]) {
      process.env.IDENTITY_HTTP_ADDR = hong;
      let loi: unknown;
      try {
        gocDichVu("identity");
      } catch (e) {
        loi = e;
      }
      expect(loi, hong).toBeInstanceOf(Error);
      expect((loi as Error).message).toContain("IDENTITY_HTTP_ADDR");
      expect((loi as Error).message).not.toContain(hong);
      expect((loi as Error).message).not.toContain("matkhau");
    }
  });

  it("chuyenTiep với biến hỏng: lỗi nêu tên biến và không có lời gọi nào đi ra", async () => {
    datBien();
    process.env.PETITIONS_HTTP_ADDR = "http://nguoi:matkhau@127.0.0.1:1";
    await expect(chuyenTiep(yeuCau("/api/v1/tasks"))).rejects.toThrow(/PETITIONS_HTTP_ADDR/);
    expect(nhan).toHaveLength(0);
  });
});
