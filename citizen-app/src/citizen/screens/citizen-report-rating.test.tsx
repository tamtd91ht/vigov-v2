/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE CITIZEN'S STAR RATING OVER THE NETWORK — `POST /api/v1/my-citizen-reports/{code}/rating`
 * (ADR 0050 point 2; server contract `service-petitions/internal/http/petition_rating.go`, 7359484).
 *
 * A FAKE SESSION FOR THIS FILE ONLY, exactly as `api/vigov-client.test.tsx` does: product code has one session
 * source and it returns `null` (`citizen.test.tsx` pins that on the real code).
 */
const state = vi.hoisted(() => ({
  session: { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" } as { token: string; commune_name: string } | null,
  host: "https://vigov.vidu.vn",
}));

vi.mock("../api/vigov-session", () => ({ getVigovSession: () => state.session }));
vi.mock("../api/vigov-address", () => ({
  vigovAddress: (_service: "petitions", d: string) => (state.host === "" ? "" : `${state.host}${d}`),
}));

import { rateReport, lookupReport } from "../api/vigov-client";
import {
  readReport,
  readMyReportsPage,
  MY_REPORTS_PATH,
  isRateable,
  type MyReport,
  RATING_COMMENT_MAX_LEN,
  RATING_FIELDS,
  ratingAddress,
  ratingBody,
  ACCEPTED_FIELDS,
} from "../api/citizen-report-contract";
import { createSendAttempt } from "../api/send-attempt";

import * as TEXTS from "./copy";
import { PHONE_VERIFICATION_TASK, RATING, RATING_ERROR, STATUS, COMMUNE_APP_REPORTS } from "./copy";
import { CommuneReportBody } from "./CommuneAppReports";
import { attemptFor, type RatingErrorBranch, ratingOutcome, RatingPanel, showsRating } from "./CitizenReportRating";
import { STAR_LABELS, StarPicker } from "./star-picker";
import { LookupResult } from "./ReportLookupScreen";

type Call = { url: string; init: RequestInit };
let calls: Call[] = [];

function reply(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function stubFetch(...replies: Array<ReturnType<typeof reply> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
    calls.push({ url, init });
    const r = replies[Math.min(i++, replies.length - 1)]!;
    return r instanceof Error ? Promise.reject(r) : Promise.resolve(r);
  });
}

const headers = (c: Call) => c.init.headers as Record<string, string>;

const CODE = "PA7K2QX9M4TD";
const RESOLVED = {
  code: CODE,
  channel: "zalo-mini-app",
  status: "da-xu-ly",
  field: "giao-thong",
  field_label: "Giao thông",
  content: "Ổ gà lớn trước cổng chợ",
  address: "Đầu ngõ thôn Hà Lam",
  reporter_name: "Nguyễn V. A.",
  reporter_phone: "09****0000",
  anonymous: false,
  clock_from: "2026-09-24T01:30:00Z",
  acknowledge_due: "2026-09-24T03:30:00Z",
  resolve_due: "2026-09-26T09:00:00Z",
  result: "Đã vá ổ gà ngày 25/09.",
};
const RATED_AT = "2026-09-28T02:00:00Z";

/** A petition as the client holds it, from a server body. */
function reportWith(over: Record<string, unknown> = {}): MyReport {
  const p = readReport({ ...RESOLVED, ...over });
  if (p === null) throw new Error("fixture is malformed");
  return p;
}

beforeEach(() => {
  calls = [];
  state.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
  state.host = "https://vigov.vidu.vn";
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("rating — the request", () => {
  it("POST to {code}/rating, Bearer from the session source, Idempotency-Key, and exactly four headers", async () => {
    stubFetch(reply(200, { ...RESOLVED, rating: 4, rated_at: RATED_AT }));
    const result = await rateReport(`  ${CODE} `, createSendAttempt(ratingBody(4, "")));
    expect(result.kind).toBe("xong");
    expect(calls).toHaveLength(1);
    const c = calls[0]!;
    expect(c.url).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}/${CODE}/rating`);
    expect(c.init.method).toBe("POST");
    expect(headers(c)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    expect(headers(c)["Idempotency-Key"]).toMatch(/^[0-9a-f]{32}$/);
    // No header names a commune or a citizen (rule 1 forbidden #2, rule 4 forbidden #1).
    expect(Object.keys(headers(c)).sort()).toEqual(["Accept", "Authorization", "Content-Type", "Idempotency-Key"]);
    expect(c.url).not.toMatch(/\?/);
  });

  it("the code is encoded once, on the path — a typed `/` cannot change the route", () => {
    expect(ratingAddress("a/../b")).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}/a%2F..%2Fb/rating`);
  });

  it("body: only `stars` and `comment`; a blank comment is left out; the comment is trimmed", () => {
    expect(RATING_FIELDS).toEqual(["stars", "comment"]);
    const withComment = JSON.parse(ratingBody(5, "  sạch sẽ rồi  ")) as Record<string, unknown>;
    expect(withComment).toEqual({ stars: 5, comment: "sạch sẽ rồi" });
    expect(JSON.parse(ratingBody(3, "   \n "))).toEqual({ stars: 3 });
    for (const k of Object.keys(withComment)) expect(RATING_FIELDS as readonly string[]).toContain(k);
    for (const banned of ["code", "status", "tenant", "tenant_id", "xa", "citizen_id", "phone", "reopen"]) {
      expect(withComment, banned).not.toHaveProperty(banned);
    }
    // The intake body is a different contract and still has exactly its five keys.
    expect(ACCEPTED_FIELDS).toHaveLength(5);
    expect(RATING_COMMENT_MAX_LEN).toBe(1000);
  });

  it("SEND AGAIN with the same act: same key, same body. A new act: a new key", async () => {
    stubFetch(new Error("mất mạng"), reply(200, { ...RESOLVED, rating: 4, rated_at: RATED_AT }));
    const act = attemptFor(null, 4, "tốt");
    if (act === null || act === "no-key") throw new Error("expected an act");
    expect(await rateReport(CODE, act)).toEqual({ kind: "loi-mang" });
    // "Gửi lại" — the screen passes the act it holds.
    expect(attemptFor(act, 4, "tốt")).toBe(act);
    expect((await rateReport(CODE, act)).kind).toBe("xong");
    expect(headers(calls[0]!)["Idempotency-Key"]).toBe(headers(calls[1]!)["Idempotency-Key"]);
    expect(calls[0]!.init.body).toBe(calls[1]!.init.body);

    const fresh = attemptFor(null, 4, "tốt");
    if (fresh === null || fresh === "no-key") throw new Error("expected an act");
    expect(fresh.key).not.toBe(act.key);
  });

  it("no act for stars outside 1..5; no CSPRNG is a refusal, never a keyless send", () => {
    for (const s of [0, 6, 2.5, Number.NaN]) expect(attemptFor(null, s, ""), String(s)).toBeNull();
    vi.stubGlobal("crypto", undefined);
    expect(attemptFor(null, 4, "")).toBe("no-key");
  });

  it("each status lands in exactly one branch; 503 is not in this contract", async () => {
    const cases: ReadonlyArray<[number, string]> = [
      [400, "khong-hop-le"],
      [401, "het-phien"],
      [404, "khong-thay"],
      [409, "dang-xu-ly-truoc"],
      [503, "loi-may-chu"],
      [500, "loi-may-chu"],
    ];
    for (const [status, kind] of cases) {
      stubFetch(reply(status, { code: "petition_state", message: "Trạng thái phản ánh vừa thay đổi.", trace_id: "" }));
      expect((await rateReport(CODE, createSendAttempt(ratingBody(4, "")))).kind, `status ${status}`).toBe(kind);
    }
    stubFetch(reply(403, { code: "chua_xac_thuc_so", message: "x", trace_id: "" }));
    expect((await rateReport(CODE, createSendAttempt(ratingBody(4, "")))).kind).toBe("can-xac-thuc-so");
    stubFetch(reply(200, { ...RESOLVED, rating: 9, rated_at: RATED_AT }));
    expect((await rateReport(CODE, createSendAttempt(ratingBody(4, "")))).kind, "200 malformed").toBe("loi-may-chu");
  });

  it("no session, no address, or no code: stops BEFORE fetch", async () => {
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    state.session = null;
    expect(await rateReport(CODE, createSendAttempt(ratingBody(4, "")))).toEqual({ kind: "chua-co-phien" });
    state.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
    state.host = "";
    expect(await rateReport(CODE, createSendAttempt(ratingBody(4, "")))).toEqual({ kind: "chua-cau-hinh" });
    state.host = "https://vigov.vidu.vn";
    expect(await rateReport("  ", createSendAttempt(ratingBody(4, "")))).toEqual({ kind: "khong-thay" });
    expect(spy).not.toHaveBeenCalled();
  });

  it("sending and reading a rating with a comment writes nothing to the console (rule 3)", async () => {
    const spies = (["log", "info", "warn", "error", "debug"] as const).map((k) => vi.spyOn(console, k));
    stubFetch(reply(200, { ...RESOLVED, status: "dang-xu-ly", rating: 1, rated_at: RATED_AT }));
    await rateReport(CODE, createSendAttempt(ratingBody(1, "chưa dọn hết, còn mùi")));
    for (const s of spies) {
      expect(s).not.toHaveBeenCalled();
      s.mockRestore();
    }
  });
});

describe("rating — reading `rating` / `rated_at`", () => {
  it("absent is 'not rated'; present is read on both the petition and the list item", () => {
    expect(reportWith()).toMatchObject({ rating: null, rated_at: null });
    expect(reportWith({ rating: 4, rated_at: RATED_AT })).toMatchObject({ rating: 4, rated_at: RATED_AT });
    const item = {
      code: CODE,
      status: "da-xu-ly",
      field: "",
      field_label: "",
      content_excerpt: "Ổ gà…",
      clock_from: RESOLVED.clock_from,
      acknowledge_due: null,
      resolve_due: null,
    };
    const page = readMyReportsPage({
      items: [item, { ...item, code: "PB2", rating: 2, rated_at: RATED_AT }],
      next_cursor: "",
      has_more: false,
    });
    expect(page?.entries[0]).toMatchObject({ rating: null, rated_at: null });
    expect(page?.entries[1]).toMatchObject({ rating: 2, rated_at: RATED_AT });
    expect(
      readMyReportsPage({ items: [{ ...item, rating: 0, rated_at: RATED_AT }], next_cursor: "", has_more: false }),
    ).toBeNull();
  });

  it("malformed: out of range, not an integer, not a number, or one field without the other", () => {
    for (const bad of [
      { rating: 0, rated_at: RATED_AT },
      { rating: 6, rated_at: RATED_AT },
      { rating: 2.5, rated_at: RATED_AT },
      { rating: "3", rated_at: RATED_AT },
      { rating: null, rated_at: RATED_AT },
      { rating: 3 },
      { rated_at: RATED_AT },
      { rating: 3, rated_at: 5 },
      { rating: 3, rated_at: "" },
    ]) {
      expect(readReport({ ...RESOLVED, ...bad }), JSON.stringify(bad)).toBeNull();
    }
  });

  it("rateable is exactly the server's two statuses — fail closed on anything else", () => {
    const yes = Object.keys(STATUS).filter(isRateable).sort();
    expect(yes).toEqual(["cho-dan-xac-nhan", "da-xu-ly"]);
    for (const s of ["trang-thai-moi", "", "toString", "DA-XU-LY"]) expect(isRateable(s), s).toBe(false);
  });
});

describe("rating — what the screen does with each result", () => {
  it("each call result maps to one outcome", () => {
    const p = reportWith({ rating: 4, rated_at: RATED_AT });
    expect(ratingOutcome({ kind: "xong", report: p })).toEqual({ kind: "done", report: p });
    expect(ratingOutcome({ kind: "can-xac-thuc-so" })).toEqual({ kind: "phone" });
    type Kind = Parameters<typeof ratingOutcome>[0]["kind"];
    const table: ReadonlyArray<[Kind, RatingErrorBranch]> = [
      ["chua-co-phien", "session-expired"],
      ["het-phien", "session-expired"],
      ["khong-thay", "not-found"],
      ["dang-xu-ly-truoc", "state-changed"],
      ["khong-hop-le", "invalid"],
      ["loi-mang", "network"],
      ["loi-may-chu", "server-fault"],
      ["kenh-chua-mo", "server-fault"],
      ["chua-cau-hinh", "server-fault"],
    ];
    for (const [kind, branch] of table) {
      expect(ratingOutcome({ kind } as Parameters<typeof ratingOutcome>[0]), kind).toEqual({ kind: "error", branch });
    }
  });
});

describe("rating — the block on the petition", () => {
  const noop = () => {};
  type PanelProps = Parameters<typeof RatingPanel>[0];
  const render = (over: Partial<PanelProps> & { report:MyReport }) =>
    renderToStaticMarkup(
      createElement(RatingPanel, {
        open: true,
        stars: 0,
        comment: "",
        sending: false,
        error: null,
        notice: null,
        onPick: noop,
        onComment: noop,
        onSubmit: noop,
        onRetry: noop,
        onReload: noop,
        onRateAgain: noop,
        ...over,
      }),
    );

  it("rateable and not rated: five radios, a comment box, the submit button disabled until a star", () => {
    for (const status of ["da-xu-ly", "cho-dan-xac-nhan"]) {
      const html = render({ report:reportWith({ status }) });
      expect(html, status).toContain(RATING.title);
      expect(html, status).toContain('role="radiogroup"');
      expect(html.match(/role="radio"/g), status).toHaveLength(5);
      expect(html, status).toContain("<textarea");
      expect(html, status).toContain('maxLength="1000"');
      expect(html, status).toContain(`<button type="button" class="cd-nut" disabled="">${RATING.submit}</button>`);
      expect(html, status).toContain(COMMUNE_APP_REPORTS.tap_star);
    }
    const picked = render({ report:reportWith(), stars: 4 });
    expect(picked).toContain(`<button type="button" class="cd-nut">${RATING.submit}</button>`);
    expect(picked).toContain(STAR_LABELS[3]);
  });

  it("not rateable and not rated: no block at all — every other status", () => {
    for (const status of Object.keys(STATUS).filter((s) => !isRateable(s))) {
      expect(showsRating(reportWith({ status })), status).toBe(false);
      expect(render({ report:reportWith({ status }) }), status).toBe("");
    }
  });

  it("rated, then reopened by the server: 'Bạn đã đánh giá n sao', read-only, no form, no button", () => {
    const html = render({ report:reportWith({ status: "dang-xu-ly", rating: 2, rated_at: RATED_AT }) });
    expect(html).toContain(RATING.your_rating);
    expect(html).toContain(RATING.rated(2));
    expect(RATING.rated(2)).toBe("Bạn đã đánh giá 2 sao.");
    expect(html).toContain('role="img"');
    expect(html).not.toContain("<textarea");
    expect(html).not.toContain(RATING.submit);
    expect(html).not.toContain(RATING.rate_again);
  });

  it("rated and still rateable, picker closed: the rating in words and 'Đánh giá lại'", () => {
    const html = render({ report:reportWith({ rating: 4, rated_at: RATED_AT }), open: false, notice: "sent" });
    expect(html).toContain(RATING.rated(4));
    expect(html).toContain(`<button type="button" class="cd-nut-phu">${RATING.rate_again}</button>`);
    expect(html).not.toContain("<textarea");
    expect(html).toContain(`<div role="status"><p class="cd-cau">${RATING.sent}</p></div>`);
  });

  it("after a reopening: the sentence says what happened, and nothing about why", () => {
    const html = render({
      report: reportWith({ status: "dang-xu-ly", rating: 1, rated_at: RATED_AT }),
      open: false,
      notice: "reopened",
    });
    expect(html).toContain(RATING.sent);
    expect(html).toContain(RATING.reopened);
  });

  it("errors: retryable shows 'Gửi lại' INSTEAD of submit; a 409 offers 'Tải lại phiếu'; each says what to do", () => {
    const base = { report:reportWith(), stars: 4 };
    for (const branch of Object.keys(RATING_ERROR) as RatingErrorBranch[]) {
      const html = render({ ...base, error: branch });
      const e = RATING_ERROR[branch];
      expect(html, branch).toContain(`<p class="cd-loi" role="alert">${e.text}</p>`);
      expect(html.includes(`>${RATING.retry}</button>`), branch).toBe(e.can_retry);
      expect(html.includes(`>${RATING.submit}</button>`), branch).toBe(!e.can_retry);
      expect(html.includes(`>${RATING.reload}</button>`), branch).toBe(e.can_reload);
      // Never an error code, never an HTTP status (agent non-negotiable #7).
      expect(e.text, branch).not.toMatch(/\b[45]\d\d\b|petition_state|invalid_request|error/i);
    }
    expect(RATING_ERROR["state-changed"].can_reload).toBe(true);
    expect(render({ ...base, sending: true })).toContain(RATING.sending);
  });

  it("the phone-verification panel has its own task sentence", () => {
    expect(PHONE_VERIFICATION_TASK.rate).toBe("đánh giá chưa được gửi");
  });

  it("the lookup result carries the block only when the screen wires it", () => {
    const result = { kind: "xong", report: reportWith() } as const;
    const withHooks = renderToStaticMarkup(createElement(LookupResult, { result, rating: { onRated: noop, onReload: noop } }));
    expect(withHooks).toContain(RATING.title);
    // The card comes first: the commune's result, then the rating of it.
    expect(withHooks.indexOf("Đã vá ổ gà ngày 25/09.")).toBeLessThan(withHooks.indexOf(RATING.title));
    expect(renderToStaticMarkup(createElement(LookupResult, { result }))).not.toContain(RATING.title);
  });
});

describe("rating — never states the reopen threshold (ADR 0050)", () => {
  /** Every string a value can show, functions called with a sample. */
  const collect = (v: unknown): string[] =>
    typeof v === "string"
      ? [v]
      : typeof v === "function"
        ? collect(v(...Array<number>(v.length).fill(2)))
        : v && typeof v === "object"
          ? Object.values(v).flatMap(collect)
          : [];

  it("no sentence of RATING / RATING_ERROR / the star labels names a threshold or a reopening rule", () => {
    // "mở lại" alone is not the probe: "đóng ứng dụng, mở lại" (reopen the APP) is a legitimate next step.
    const THRESHOLD = /(phiếu|phản ánh)[^.]*mở lại|mở lại (phiếu|phản ánh)|ngưỡng|\d\s*[–-]\s*\d\s*sao|trở xuống|dưới \d sao/i;
    const sentences = [...collect(RATING), ...collect(RATING_ERROR), ...STAR_LABELS];
    expect(sentences.length).toBeGreaterThan(15);
    expect(sentences.filter((s) => THRESHOLD.test(s))).toEqual([]);
    // The probe is alive.
    expect(THRESHOLD.test("Chấm 1–2 sao thì phiếu được mở lại")).toBe(true);
    expect(THRESHOLD.test("Từ 2 sao trở xuống, xã xử lý lại")).toBe(true);
    expect(THRESHOLD.test("Đánh giá thấp sẽ mở lại phản ánh")).toBe(true);
    expect(THRESHOLD.test("Hãy đóng ứng dụng, mở lại rồi đánh giá lại.")).toBe(false);
    // The collector reads real values.
    expect(collect(TEXTS).length).toBeGreaterThan(100);
  });

  it("the live block never reads the experience app's threshold or compares stars with one", () => {
    const src = Object.values(
      import.meta.glob(["./CitizenReportRating.tsx", "./star-picker.tsx"], {
        query: "?raw",
        import: "default",
        eager: true,
      }) as Record<string, string>,
    );
    expect(src).toHaveLength(2);
    for (const code of src) {
      expect(code).not.toMatch(/SAO_MO_LAI|RatingReopenThreshold|stars\s*<=\s*\d|rating\s*<=\s*\d/);
      // Rule 3 + rule 13: no console, no raw HTML.
      expect(code).not.toMatch(/\bconsole\s*\.|dangerouslySetInnerHTML|innerHTML\s*=/);
    }
  });
});

describe("the shared star row — and the commune app rates through the same block", () => {
  it("StarPicker: interactive is a radiogroup, read-only is one image with the level in words", () => {
    const live = renderToStaticMarkup(createElement(StarPicker, { stars: 3, onPick: () => {} }));
    expect(live).toContain(`aria-label="${COMMUNE_APP_REPORTS.rate_stars}"`);
    expect(live.match(/xa-sao__nut--on/g)).toHaveLength(3);
    expect(live).toContain('aria-checked="true"');
    const ro = renderToStaticMarkup(createElement(StarPicker, { stars: 4 }));
    expect(ro).toContain(`role="img" aria-label="${COMMUNE_APP_REPORTS.rated_stars(4)}"`);
    expect(ro.match(/disabled=""/g)).toHaveLength(5);
    expect(ro).toContain(STAR_LABELS[3]);
  });

  it("the commune app's petition detail renders the REAL rating block (`CitizenReportRating`), not a local copy", () => {
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    const report = readReport(RESOLVED)!;
    const open = renderToStaticMarkup(
      createElement(CommuneReportBody, { report, onRated: () => {}, onReload: () => {} }),
    );
    expect(open).toContain(RATING.title);
    expect(open).toContain('role="radiogroup"');
    expect(open).toContain(RATING.submit);
    expect(open).toContain(`maxLength="${RATING_COMMENT_MAX_LEN}"`);
    // Rendering sends nothing: a rating leaves the phone only on "Gửi đánh giá".
    expect(spy).not.toHaveBeenCalled();
    const rated = readReport({ ...RESOLVED, rating: 5, rated_at: RATED_AT })!;
    const ro = renderToStaticMarkup(createElement(CommuneReportBody, { report:rated, onRated: () => {}, onReload: () => {} }));
    expect(ro).toContain(RATING.rated(5));
  });

  it("the lookup of the real channel still reads a rated petition", async () => {
    stubFetch(reply(200, { ...RESOLVED, rating: 3, rated_at: RATED_AT }));
    const result = await lookupReport(CODE);
    expect(result.kind === "xong" && result.report.rating).toBe(3);
  });
});
