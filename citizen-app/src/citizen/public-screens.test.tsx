import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { communeNewsArticle, communeStaffDirectory, lookupCommuneByDomain, communeNews } from "./api/vigov-client";
import {
  splitParagraphs,
  readNewsArticle,
  readDirectory,
  readCommuneNewsPage,
  readCommune,
  DIRECTORY_PATH,
  COMMUNE_NEWS_PATH,
  COMMUNES_PATH,
} from "./api/public-contract";
import { type SessionOpenResult, openSessionAfterConfirmation, type SessionOpenRequest } from "./api/open-vigov-session";
import { setVigovSession, getVigovSession } from "./api/vigov-session";
import { dialTarget, afterDirectoryLoad, StaffCard, DirectoryBody } from "./screens/StaffDirectoryScreen";
import { CitizenChannel } from "./screens/CitizenChannel";
import { COMMUNE_NOT_LOGGED_IN, MY_REPORTS, DIRECTORY, SEND, COMMUNE_NEWS, LOOKUP, COMMUNE_CONFIRMATION } from "./screens/copy";
import {
  NewsArticle,
  startLoadingNews,
  afterArticleLoad,
  afterNewsLoad,
  ArticleBody,
  CommuneNewsBody,
  FIRST_NEWS,
} from "./screens/CommuneNewsScreen";
import { stepAfterSessionOpen, stepAfterCommuneLookup, CommuneConfirmationScreen } from "./screens/CommuneConfirmation";

/**
 * NỬA NHÀ NƯỚC NÓI CHUYỆN VỚI MÁY CHỦ — màn xác nhận xã, mở phiên qua hàm tiêm vào, tin tức và danh
 * bạ của xã. Dựng bằng `react-dom/server` và hàm thuần (không có DOM để bấm): mỗi bước của màn là
 * một hàm `stepAfter…`/`after…` được kiểm thẳng, và mỗi lời gọi mạng đi qua `fetch` giả.
 *
 * Tên miền, tên xã, họ tên và số điện thoại ở đây đều GIẢ. Số điện thoại là số giả đã thống nhất.
 */

const DOMAIN = "xa-vi-du.vigov.example";

type Call = { address: string; options: RequestInit };
let calls: Call[] = [];

function respond(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function stubFetch(...responses: Array<ReturnType<typeof respond> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (address: string, options: RequestInit) => {
    calls.push({ address, options });
    const p = responses[Math.min(i++, responses.length - 1)]!;
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, " ")
    .replace(/\s+/g, " ");

beforeEach(() => {
  calls = [];
  setVigovSession(null);
});

afterEach(() => {
  vi.unstubAllGlobals();
  setVigovSession(null);
});

// ---------------------------------------------------------------------------------------------

describe("tra xã theo tên miền — `GET identity /api/v1/communes?host=`", () => {
  it("GET đúng host của identity, `host` mã hoá, KHÔNG bearer, KHÔNG thân", async () => {
    stubFetch(respond(200, { items: [{ name: "Xã Thử Nghiệm", province: "Tỉnh Ví Dụ" }] }));
    const result = await lookupCommuneByDomain(DOMAIN);
    expect(result).toEqual({ kind: "xong", value: [{ ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" }] });
    expect(calls).toHaveLength(1);
    const g = calls[0]!;
    expect(g.address).toBe(`https://identity.api.vigov.vn${COMMUNES_PATH}?host=${DOMAIN}`);
    expect(g.options.method).toBe("GET");
    expect(g.options.body).toBeUndefined();
    expect(Object.keys(g.options.headers as Record<string, string>)).toEqual(["Accept"]);
  });

  it("KHÔNG gọi mạng khi tên miền rỗng hoặc sai khuôn", async () => {
    const fake_fetch = vi.fn();
    vi.stubGlobal("fetch", fake_fetch);
    for (const d of ["", "localhost", "a b.vn", "xa.vn/duong", "xa.vn?host=khac"]) {
      expect(await lookupCommuneByDomain(d), d).toEqual({ kind: "khong-hop-le" });
      expect(await communeStaffDirectory(d), d).toEqual({ kind: "khong-hop-le" });
      expect(await communeNews(d, ""), d).toEqual({ kind: "khong-hop-le" });
      expect(await communeNewsArticle(d, "t1"), d).toEqual({ kind: "khong-hop-le" });
    }
    expect(fake_fetch).not.toHaveBeenCalled();
  });

  it("mỗi mã trạng thái rơi vào đúng một nhánh", async () => {
    const CASES: Array<[ReturnType<typeof respond> | Error, string]> = [
      [respond(400, { error: "x" }), "khong-hop-le"],
      [respond(503, {}), "tam-ngung"],
      [respond(500, {}), "loi-may-chu"],
      [respond(401, {}), "loi-may-chu"],
      [respond(200, { items: [{ name: 1 }] }), "loi-may-chu"],
      [new Error("mat mang"), "loi-mang"],
    ];
    for (const [reply, kind] of CASES) {
      stubFetch(reply);
      expect((await lookupCommuneByDomain(DOMAIN)).kind).toBe(kind);
    }
  });

  it("parser: rỗng là rỗng, sai kiểu là `null`", () => {
    expect(readCommune({ items: [] })).toEqual([]);
    expect(readCommune({})).toBeNull();
    expect(readCommune({ items: [{ name: "a" }] })).toBeNull();
    expect(readCommune(null)).toBeNull();
  });
});

describe("màn xác nhận xã — tên từ máy chủ, fail closed ở mọi nhánh khác", () => {
  const COMMUNE = { ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" };

  it("máy chủ trả MỘT xã, nguồn qr → hỏi 'Làm việc với <tên>, <tỉnh>'", () => {
    const b = stepAfterCommuneLookup({ kind: "xong", value: [COMMUNE] }, "qr");
    expect(b).toEqual({ page: { kind: "hoi", commune: COMMUNE } });
    if (!("page" in b)) throw new Error("không tới bước hỏi");
    const text = textOf(
      renderToStaticMarkup(
        createElement(CommuneConfirmationScreen, { page: b.page, source: "qr", onConfirm: () => {}, onNotThis: () => {} }),
      ),
    );
    expect(text).toContain("làm việc với Xã Thử Nghiệm, Tỉnh Ví Dụ");
    expect(text).toContain("Bạn cần liên hệ với xã này?");
    expect(text).not.toContain("vigov.example");
  });

  it("rỗng · nhiều hơn một xã · tên rỗng · 400 → về giới thiệu với câu 'mã QR chưa dẫn tới xã nào'", () => {
    for (const result of [
      { kind: "xong", value: [] },
      { kind: "xong", value: [COMMUNE, { ten: "Xã Khác", tinh: "Tỉnh Khác" }] },
      { kind: "xong", value: [{ ten: "  ", tinh: "Tỉnh Ví Dụ" }] },
      { kind: "khong-hop-le" },
    ] as const) {
      expect(stepAfterCommuneLookup(result, "qr"), JSON.stringify(result)).toEqual({
        outcome: { kind: "ve-gioi-thieu", text: COMMUNE_CONFIRMATION.not_found },
      });
    }
  });

  it("503 · 500 · mất mạng → về giới thiệu với câu 'chưa kết nối được'", () => {
    for (const kind of ["tam-ngung", "loi-may-chu", "loi-mang", "chua-cau-hinh"] as const) {
      expect(stepAfterCommuneLookup({ kind }, "qr"), kind).toEqual({
        outcome: { kind: "ve-gioi-thieu", text: COMMUNE_CONFIRMATION.not_connected },
      });
    }
  });

  it("nguồn không tin được thì không hỏi, kể cả khi máy chủ trả một xã", () => {
    expect("outcome" in stepAfterCommuneLookup({ kind: "xong", value: [COMMUNE] }, "share")).toBe(true);
  });

  it("bước chờ: một câu `role=status`, không tên xã, không nút", () => {
    const html = renderToStaticMarkup(
      createElement(CommuneConfirmationScreen, {
        page: { kind: "dang-tra" },
        source: "qr",
        onConfirm: () => {},
        onNotThis: () => {},
      }),
    );
    expect(html).toContain('role="status"');
    expect(textOf(html)).toContain(COMMUNE_CONFIRMATION.looking_up);
    expect(html).not.toMatch(/<button\b/);
  });

  it("đang mở phiên: hai nút KHÔNG bấm được, câu chờ được đọc ra", () => {
    const html = renderToStaticMarkup(
      createElement(CommuneConfirmationScreen, { page: { kind: "dang-mo", commune: COMMUNE }, source: "qr", onConfirm: () => {}, onNotThis: () => {} }),
    );
    expect(html.match(/<button[^>]*disabled/g) ?? []).toHaveLength(2);
    expect(textOf(html)).toContain(COMMUNE_CONFIRMATION.opening);
  });

  it("các câu của lớp khám phá không mang mã lỗi, tên dịch vụ hay tên miền", () => {
    for (const text of Object.values(COMMUNE_CONFIRMATION)) {
      expect(text).not.toMatch(/\b[45]\d\d\b|error|identity|vigov|host|token/i);
      expect(text.length).toBeGreaterThan(20);
    }
  });
});

// ---------------------------------------------------------------------------------------------

describe("xác nhận → mở phiên qua hàm TIÊM VÀO", () => {
  it("gọi hàm tiêm với ĐÚNG tên miền và `communeConfirmed: true`, rồi phiên có trong bộ nhớ", async () => {
    const requests: SessionOpenRequest[] = [];
    const open = async (req: SessionOpenRequest): Promise<SessionOpenResult> => {
      requests.push(req);
      return { kind: "xong", token: "tok-vigov-thu", commune_name: "Xã Của Phiên", domain: null };
    };
    expect(getVigovSession()).toBeNull();
    const result = await openSessionAfterConfirmation(open, DOMAIN);
    expect(requests).toEqual([{ communeHostHint: DOMAIN, communeConfirmed: true }]);
    // Bearer KHÔNG đi ngược lên màn hình — chỉ tên xã (và tên miền chính của phiên, nếu có).
    expect(result).toEqual({ kind: "da-mo", commune_name: "Xã Của Phiên", domain: null });
    expect(getVigovSession()).toEqual({ token: "tok-vigov-thu", commune_name: "Xã Của Phiên" });
  });

  it("tên miền chính của phiên đi lên màn hình khi đúng khuôn; sai khuôn thì `null` — kiểm lại ở nửa này", async () => {
    const readWith = (domain: string | null) =>
      openSessionAfterConfirmation(async () => ({ kind: "xong", token: "tok", commune_name: "Xã Của Phiên", domain }), DOMAIN);
    expect(await readWith("xa-khac.vigov.example")).toEqual({
      kind: "da-mo",
      commune_name: "Xã Của Phiên",
      domain: "xa-khac.vigov.example",
    });
    for (const wrong of ["", "localhost", "https://xa.vn", "Xa.Vn", "xa.vn?host=khac"]) {
      expect(await readWith(wrong), wrong).toMatchObject({ kind: "da-mo", domain: null });
    }
    // Tên miền KHÔNG vào nguồn phiên: bearer và tên xã là tất cả những gì `vigov-session.ts` giữ.
    expect(getVigovSession()).toEqual({ token: "tok", commune_name: "Xã Của Phiên" });
  });

  it("tên xã của phiên thắng tên màn xác nhận đã hiện (ADR 0047 §Trả lời mục 4)", () => {
    expect(
      stepAfterSessionOpen({ ten: "Tên Đã Hiện", tinh: "T" }, { kind: "da-mo", commune_name: "Tên Của Phiên", domain: DOMAIN }),
    ).toEqual({
      outcome: { kind: "da-mo", commune_name: "Tên Của Phiên", domain: DOMAIN },
    });
  });

  it("cầu tắt · phiên không bearer · phiên không tên xã → KHÔNG có phiên, nhưng xã ĐÃ xác nhận: mở phần công khai", async () => {
    const commune = { ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" };
    for (const lookup of [
      { kind: "chua-mo" },
      { kind: "xong", token: "", commune_name: "Xã Của Phiên", domain: null },
      { kind: "xong", token: "tok", commune_name: " ", domain: DOMAIN },
    ] as const) {
      const result = await openSessionAfterConfirmation(async () => lookup, DOMAIN);
      expect(result, JSON.stringify(lookup)).toEqual({ kind: "chua-mo" });
      expect(getVigovSession()).toBeNull();
      // Tên và tỉnh là của `/communes` — thứ công dân vừa đọc và bấm xác nhận — không dựng từ tham số.
      expect(stepAfterSessionOpen(commune, result)).toEqual({ outcome: { kind: "xac-nhan-khong-phien", commune } });
    }
  });

  it("hàm tiêm ném lỗi hoặc báo thử lại → ở lại màn xác nhận với câu 'bấm lần nữa'", async () => {
    const result = await openSessionAfterConfirmation(async () => {
      throw new Error("x");
    }, DOMAIN);
    expect(result).toEqual({ kind: "thu-lai" });
    expect(getVigovSession()).toBeNull();
    const commune = { ten: "a", tinh: "b" };
    expect(stepAfterSessionOpen(commune, result)).toEqual({ page: { kind: "hoi", commune, error_text: COMMUNE_CONFIRMATION.retry } });
  });

  it("ngoài Zalo → không phiên, nhưng xã đã xác nhận: phần công khai vẫn mở", () => {
    const commune = { ten: "a", tinh: "b" };
    expect(stepAfterSessionOpen(commune, { kind: "ngoai-zalo" })).toEqual({ outcome: { kind: "xac-nhan-khong-phien", commune } });
  });
});

// ---------------------------------------------------------------------------------------------

describe("kênh công dân: hai lối vào công khai chỉ có khi đã biết tên miền xã", () => {
  it("không tên miền → không có 'Tin tức của xã', không có 'Danh bạ cán bộ xã'", () => {
    for (const domain of [undefined, null]) {
      const html = renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {}, domain }));
      expect(html).not.toContain(COMMUNE_NEWS.title);
      expect(html).not.toContain(DIRECTORY.title);
    }
  });

  it("có tên miền → hai nút, mỗi nút là một `cd-nut` 48px", () => {
    const html = renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {}, domain: DOMAIN }));
    expect(html).toContain(`<button type="button" class="cd-nut">${COMMUNE_NEWS.title}</button>`);
    expect(html).toContain(`<button type="button" class="cd-nut">${DIRECTORY.title}</button>`);
    expect(html).not.toContain(DOMAIN);
  });
});

describe("kênh công dân KHÔNG phiên — nói ra, không im lặng", () => {
  it("chưa có phiên → câu `role=status` TRƯỚC ba lối phản ánh; ba lối vẫn còn", () => {
    const html = renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {}, domain: DOMAIN }));
    const text = textOf(html);
    expect(html).toMatch(/<p class="cd-loi" role="status">/);
    expect(text).toContain(COMMUNE_NOT_LOGGED_IN.text);
    expect(text).toContain(COMMUNE_NOT_LOGGED_IN.remaining);
    // Câu đứng TRƯỚC nút đầu tiên: người dân đọc nó trước khi bấm.
    expect(html.indexOf(COMMUNE_NOT_LOGGED_IN.text)).toBeLessThan(html.indexOf(SEND.title));
    for (const label of [SEND.title, MY_REPORTS.title, LOOKUP.title]) {
      expect(html).toContain(`<button type="button" class="cd-nut">${label}</button>`);
    }
  });

  it("không tên miền → không hứa 'tin tức và danh bạ ở dưới'", () => {
    const text = textOf(renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {}, domain: null })));
    expect(text).toContain(COMMUNE_NOT_LOGGED_IN.text);
    expect(text).not.toContain(COMMUNE_NOT_LOGGED_IN.remaining);
  });

  it("CÓ phiên → không có câu 'chưa đăng nhập được'", () => {
    setVigovSession({ token: "tok", commune_name: "Xã Của Phiên" });
    const html = renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {}, domain: DOMAIN }));
    expect(textOf(html)).not.toContain(COMMUNE_NOT_LOGGED_IN.text);
    expect(html).toContain(COMMUNE_NEWS.title);
  });

  it("câu ấy nói việc làm tiếp, không mã lỗi, không tên dịch vụ", () => {
    for (const text of Object.values(COMMUNE_NOT_LOGGED_IN)) {
      expect(text).not.toMatch(/\b[45]\d\d\b|error|identity|vigov|host|token|phiên/i);
    }
    expect(COMMUNE_NOT_LOGGED_IN.text).toMatch(/Bộ phận tiếp nhận|gọi điện thoại/);
  });
});

// ---------------------------------------------------------------------------------------------

const NEWS_OUT = {
  id: "tin-01",
  title: "Lịch tiêm chủng tháng mười",
  summary: "Trạm y tế thông báo lịch tiêm.",
  published_on: "2026-09-27",
  category_name: "Y tế",
};

describe("tin của xã — lời gọi và đọc trang", () => {
  it("trang đầu: GET đúng host của comms, chỉ `host`; trang sau: thêm `cursor` NGUYÊN VĂN", async () => {
    stubFetch(respond(200, { items: [NEWS_OUT], next_cursor: "c+/=&1", has_more: true }));
    const result = await communeNews(DOMAIN, "");
    expect(result.kind).toBe("xong");
    expect(calls[0]!.address).toBe(`https://comms.api.vigov.vn${COMMUNE_NEWS_PATH}?host=${DOMAIN}`);
    expect(Object.keys(calls[0]!.options.headers as Record<string, string>)).toEqual(["Accept"]);

    await communeNews(DOMAIN, "c+/=&1");
    const url = new URL(calls[1]!.address);
    expect(url.searchParams.get("host")).toBe(DOMAIN);
    expect(url.searchParams.get("cursor")).toBe("c+/=&1");
    expect([...url.searchParams.keys()].sort()).toEqual(["cursor", "host"]);
  });

  it("chi tiết: `id` mã hoá trong đường dẫn, 404 là MỘT nhánh", async () => {
    stubFetch(respond(404, { error: "khong thay" }));
    expect(await communeNewsArticle(DOMAIN, "a/b")).toEqual({ kind: "khong-thay" });
    expect(calls[0]!.address).toBe(`https://comms.api.vigov.vn${COMMUNE_NEWS_PATH}/a%2Fb?host=${DOMAIN}`);
  });

  it("parser: `has_more` mà không có con trỏ là sai khuôn; một dòng hỏng làm hỏng cả trang", () => {
    expect(readCommuneNewsPage({ items: [NEWS_OUT], next_cursor: "", has_more: false })).toEqual({
      // `type: null` — this fixture has no `type` (an older server): absent is not malformed.
      entries: [{ id: "tin-01", title: NEWS_OUT.title, summary: NEWS_OUT.summary, published_on: "2026-09-27", category: "Y tế", type: null }],
      cursor: "",
      has_more: false,
    });
    expect(readCommuneNewsPage({ items: [NEWS_OUT], next_cursor: "", has_more: true })).toBeNull();
    expect(readCommuneNewsPage({ items: [NEWS_OUT, { ...NEWS_OUT, id: 3 }], next_cursor: "", has_more: false })).toBeNull();
    expect(readNewsArticle(NEWS_OUT)).toBeNull(); // chi tiết bắt buộc có `body`
    expect(readNewsArticle({ ...NEWS_OUT, body: "Đoạn một." })?.content).toBe("Đoạn một.");
  });

  it("'Xem thêm' NỐI trang sau vào cuối và bỏ tin trùng", () => {
    const t1 = { id: "1", title: "a", summary: "", published_on: "2026-09-27", category: "", type: null };
    const t2 = { ...t1, id: "2" };
    const after1 = afterNewsLoad(FIRST_NEWS, { kind: "xong", value: { entries: [t1], cursor: "c1", has_more: true } });
    const after2 = afterNewsLoad(startLoadingNews(after1), {
      kind: "xong",
      value: { entries: [t1, t2], cursor: "", has_more: false },
    });
    expect(after2.entries.map((t) => t.id)).toEqual(["1", "2"]);
    expect(after2.has_more).toBe(false);
    const html = renderToStaticMarkup(createElement(CommuneNewsBody, { list: after1, onOpen: () => {}, onLoad: () => {} }));
    expect(html).toContain(COMMUNE_NEWS.load_more_button);
    expect(textOf(html)).toContain("Ngày đăng: 27/09/2026");
  });

  it("lỗi giữ danh sách đã có; tên miền bị từ chối thì không mời Thử lại", () => {
    const found = afterNewsLoad(FIRST_NEWS, {
      kind: "xong",
      value: { entries: [{ id: "1", title: "a", summary: "", published_on: "", category: "", type: null }], cursor: "c", has_more: true },
    });
    const error = afterNewsLoad(startLoadingNews(found), { kind: "loi-mang" });
    expect(error.entries).toHaveLength(1);
    expect(renderToStaticMarkup(createElement(CommuneNewsBody, { list: error, onOpen: () => {}, onLoad: () => {} }))).toContain(
      COMMUNE_NEWS.retry_button,
    );
    const rejected = afterNewsLoad(FIRST_NEWS, { kind: "khong-hop-le" });
    const html = renderToStaticMarkup(createElement(CommuneNewsBody, { list: rejected, onOpen: () => {}, onLoad: () => {} }));
    expect(html).toContain(COMMUNE_NEWS.invalid);
    expect(html).not.toContain(COMMUNE_NEWS.retry_button);
  });
});

describe("tin của xã — thân tin là VĂN BẢN, không bao giờ là HTML", () => {
  const ARTICLE = {
    id: "tin-01",
    title: "<b>Tiêu đề</b>",
    summary: "",
    published_on: "2026-09-27",
    category: "Y tế",
    type: null,
    content: 'Đoạn một, dòng một.\nDòng hai.\n\n<script>alert("x")</script>\n\n\n\n<img src=x onerror=alert(1)>',
  };

  it("chia đoạn theo dòng trống, bỏ đoạn rỗng, giữ dòng đơn", () => {
    expect(splitParagraphs(ARTICLE.content)).toEqual([
      "Đoạn một, dòng một.\nDòng hai.",
      '<script>alert("x")</script>',
      "<img src=x onerror=alert(1)>",
    ]);
    expect(splitParagraphs("a\r\n\r\nb")).toEqual(["a", "b"]);
    expect(splitParagraphs("   \n\n  ")).toEqual([]);
  });

  it("`<script>` và `<img>` trong thân hiện thành CHỮ, không thành thẻ", () => {
    const html = renderToStaticMarkup(createElement(NewsArticle, { article: ARTICLE }));
    expect(html).not.toMatch(/<script\b/i);
    expect(html).not.toMatch(/<img\b/i);
    expect(html).not.toMatch(/<b>/);
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(html).toContain("&lt;b&gt;Tiêu đề&lt;/b&gt;");
    // Ba đoạn, ba `<p>` thân tin.
    expect(html.match(/class="cd-tin__doan"/g) ?? []).toHaveLength(3);
  });

  it("không tệp nào của nửa nhà nước dùng `dangerouslySetInnerHTML` hay `innerHTML`", () => {
    const RAW = import.meta.glob("./**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    // Bỏ chú thích: tệp màn tin GIẢI THÍCH bằng lời vì sao nó không dùng API ấy.
    const stripComments = (code: string) => code.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
    const production = Object.entries(RAW)
      .filter(([p]) => !p.includes(".test."))
      .map(([p, code]) => [p, stripComments(code)] as const);
    expect(production.length).toBeGreaterThan(10);
    expect(production.filter(([, code]) => /dangerouslySetInnerHTML|innerHTML/.test(code)).map(([p]) => p)).toEqual([]);
  });

  it("chi tiết: 404 nói việc cần làm; lỗi mạng mời Thử lại", () => {
    expect(textOf(renderToStaticMarkup(createElement(ArticleBody, { page: afterArticleLoad({ kind: "khong-thay" }), onLoad: () => {} })))).toContain(
      COMMUNE_NEWS.not_found,
    );
    const error = renderToStaticMarkup(createElement(ArticleBody, { page: afterArticleLoad({ kind: "loi-mang" }), onLoad: () => {} }));
    expect(error).toContain('role="alert"');
    expect(error).toContain(COMMUNE_NEWS.retry_button);
  });
});

// ---------------------------------------------------------------------------------------------

const STAFF_OUT = {
  full_name: "Nguyễn Văn Thử",
  position: "Công chức Văn phòng",
  department_name: "Văn phòng Ủy ban",
  phone: "0900.000 000",
  mobile: "0900000000",
  has_zalo: true,
};

describe("danh bạ cán bộ xã", () => {
  it("GET đúng host của identity, chỉ `host`, KHÔNG bearer", async () => {
    stubFetch(respond(200, { items: [STAFF_OUT] }));
    const result = await communeStaffDirectory(DOMAIN);
    expect(result.kind).toBe("xong");
    expect(calls[0]!.address).toBe(`https://identity.api.vigov.vn${DIRECTORY_PATH}?host=${DOMAIN}`);
    expect(Object.keys(calls[0]!.options.headers as Record<string, string>)).toEqual(["Accept"]);
  });

  it("hiện họ tên, chức vụ, bộ phận, và hai liên kết `tel:` đã làm sạch", () => {
    const staff = readDirectory({ items: [STAFF_OUT] })![0]!;
    const html = renderToStaticMarkup(createElement(StaffCard, { staff }));
    const text = textOf(html);
    expect(text).toContain("Nguyễn Văn Thử");
    expect(text).toContain(`${DIRECTORY.position}: Công chức Văn phòng`);
    expect(text).toContain(`${DIRECTORY.org_unit}: Văn phòng Ủy ban`);
    expect(html.match(/href="tel:0900000000"/g) ?? []).toHaveLength(2);
    expect(text).toContain(DIRECTORY.call("0900.000 000"));
    expect(text).toContain(DIRECTORY.has_zalo);
    expect(html).toContain('class="cd-goi"');
  });

  it("dấu Zalo CHỈ khi `has_zalo`; bộ phận rỗng không để một dòng trống; số rỗng không có liên kết", () => {
    const staff = readDirectory({ items: [{ ...STAFF_OUT, has_zalo: false, department_name: "", mobile: "" }] })![0]!;
    const html = renderToStaticMarkup(createElement(StaffCard, { staff }));
    expect(textOf(html)).not.toContain(DIRECTORY.has_zalo);
    expect(textOf(html)).not.toContain(DIRECTORY.org_unit);
    expect(textOf(html)).not.toContain(DIRECTORY.mobile);
    expect(html.match(/href="tel:/g) ?? []).toHaveLength(1);
  });

  it("`dialTarget`: giữ chữ số và một `+` đầu, không bao giờ ra một `tel:` mang ký tự lạ", () => {
    expect(dialTarget("0900.000 000")).toBe("tel:0900000000");
    expect(dialTarget("+84 900 000 000")).toBe("tel:+84900000000");
    expect(dialTarget("javascript:alert(1)")).toBeNull();
    expect(dialTarget("  ")).toBeNull();
    expect(dialTarget("09")).toBeNull();
  });

  it("parser: sai kiểu một trường là `null`", () => {
    expect(readDirectory({ items: [{ ...STAFF_OUT, has_zalo: "co" }] })).toBeNull();
    expect(readDirectory({ items: [] })).toEqual([]);
  });

  it("rỗng: một câu nói việc làm được; lỗi: câu + Thử lại; tên miền bị từ chối: không Thử lại", () => {
    const render = (page: Parameters<typeof DirectoryBody>[0]["page"]) =>
      renderToStaticMarkup(createElement(DirectoryBody, { page, onLoad: () => {} }));
    expect(render(afterDirectoryLoad({ kind: "xong", value: [] }))).toContain(DIRECTORY.empty);
    const network = render(afterDirectoryLoad({ kind: "loi-mang" }));
    expect(network).toContain(DIRECTORY.network_error);
    expect(network).toContain(DIRECTORY.retry_button);
    expect(render(afterDirectoryLoad({ kind: "tam-ngung" }))).toContain(DIRECTORY.server_error);
    const rejected = render(afterDirectoryLoad({ kind: "khong-hop-le" }));
    expect(rejected).toContain(DIRECTORY.invalid);
    expect(rejected).not.toContain(DIRECTORY.retry_button);
  });
});
