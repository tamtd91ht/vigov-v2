import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQuaGui } from "../../api/goi-may-chu"; // vi-name-ok: existing type
import { docDanhSach, DISPLAY_NAME_MAX_RUNES, thanYeuCau, type YeuCauDaGui, type YeuCauMoi } from "../../api/hop-dong-yeu-cau"; // vi-name-ok: existing exports
import { ContactScreen } from "../company-intro/ContactScreen";
import { HomeScreen } from "../company-intro/HomeScreen";
import { NHAN_NUT_CHAT } from "../company-intro/NutChatOA"; // vi-name-ok: existing export
import { type SessionSteps, ensureSession } from "../dang-nhap/ensure-session";
import type { Phien } from "../dang-nhap/hop-dong"; // vi-name-ok: existing type
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export
import { KHAI_BAO_LOI_GOI, type KetQuaXin } from "../tinh-nang/zalo-api"; // vi-name-ok: existing exports
import { TIEU_DE_KHOI_TIN } from "../../content/tin-tuc"; // vi-name-ok: existing export

import { ChatWithSpecialistView } from "./ChatWithSpecialist";
import { CHAT_SPECIALIST, SMS_OFFERS, TU_VAN } from "./noi-dung"; // vi-name-ok: existing export TU_VAN
import {
  chatWithSpecialist,
  deriveSmsSubscription,
  quickRequest,
  type QuickRequestSteps,
  sendSmsChoice,
  type SmsOutcome,
} from "./quick-request";
import { SmsOffersView } from "./SmsOffers";
import { NHAN_LOAI } from "./trang-thai"; // vi-name-ok: existing export

const render = (node: ReactElement) => renderToStaticMarkup(node);
const textOf = (markup: string) => markup.replace(/<[^>]+>/g, " ").replace(/&amp;/g, "&").replace(/\s+/g, " ");

const SESSION: Phien = { token: "phieu-thu", het_han: "2026-10-14T03:00:00Z" }; // vi-name-ok: existing type
const CODES = { ma_so_dien_thoai: "ma-so", ma_truy_cap: "ma-truy-cap" };
const NAME = "Nguyễn Văn Thử";

/* ============================================================================================
   BODY SHAPE — the name rides on `chat` only; identity never rides at all
   ============================================================================================ */

describe("request body: displayName on chat only, never an identity field", () => {
  const body = (yc: YeuCauMoi) => JSON.parse(thanYeuCau(yc)) as Record<string, unknown>; // vi-name-ok: existing type

  it("a chat carries the name, trimmed", () => {
    const b = body(quickRequest("chat", "", `  ${NAME}  `));
    expect(b["kind"]).toBe("chat");
    expect(b["displayName"]).toBe(NAME);
  });

  it("sms_promo, sms_optout, consult and callback NEVER carry it — even when a caller sets it", () => {
    for (const kind of ["sms_promo", "sms_optout", "consult", "callback"] as const) {
      expect(body(quickRequest(kind, "", NAME)), kind).not.toHaveProperty("displayName");
    }
  });

  it("an empty or blank name is left out rather than sent empty", () => {
    expect(body(quickRequest("chat", "", ""))).not.toHaveProperty("displayName");
    expect(body(quickRequest("chat", "", "   "))).not.toHaveProperty("displayName");
  });

  it("a name past the server's cap is cut to it in RUNES, not sent whole for a 400", () => {
    const long = "ệ".repeat(DISPLAY_NAME_MAX_RUNES + 20);
    const sent = body(quickRequest("chat", "", long))["displayName"] as string;
    expect(Array.from(sent).length).toBe(DISPLAY_NAME_MAX_RUNES);
  });

  it("no body of any one-tap kind carries the phone, a user id or a token", () => {
    for (const kind of ["chat", "sms_promo", "sms_optout"] as const) {
      const raw = thanYeuCau(quickRequest(kind, "", NAME)).toLowerCase();
      for (const bad of ["phone", "userid", "user_id", "nguoi_dung", "token", "0900000000"]) {
        expect(raw, `${kind} carries "${bad}"`).not.toContain(bad);
      }
    }
  });

  it("the one-tap bodies send no interests, scale or note — only the kind and the campaign code", () => {
    const b = body(quickRequest("sms_promo", "vp-quay"));
    expect(b).toEqual({ kind: "sms_promo", interests: [], scale: "", note: "", source: "vp-quay" });
  });

  it("the list keeps rows of the new kinds, and each has a Vietnamese label", () => {
    const rows = docDanhSach({
      items: ["chat", "sms_promo", "sms_optout"].map((kind, i) => ({
        requestId: `R${i}`,
        kind,
        status: "moi",
        createdAt: "2026-10-07T03:00:00Z",
      })),
    });
    expect(rows.map((r) => r.loai)).toEqual(["chat", "sms_promo", "sms_optout"]);
    expect(NHAN_LOAI.chat).toBe("Chat với chuyên viên");
    expect(NHAN_LOAI.sms_promo).toBe("Đăng ký nhận ưu đãi SMS");
    expect(NHAN_LOAI.sms_optout).toBe("Huỷ nhận ưu đãi SMS");
  });
});

/* ============================================================================================
   ENSURE SESSION — the existing one-tap login, reused
   ============================================================================================ */

function sessionSteps(codes: KetQuaXin<typeof CODES>, issued: Awaited<ReturnType<SessionSteps["issue"]>>) {
  const calls = { codes: 0, issue: 0 };
  const steps: SessionSteps = {
    requestCodes: () => {
      calls.codes++;
      return Promise.resolve(codes);
    },
    issue: () => {
      calls.issue++;
      return Promise.resolve(issued);
    },
  };
  return { steps, calls };
}

describe("ensureSession: one tap, four branches", () => {
  it("a session already held is used as is — Zalo is not asked again", async () => {
    const { steps, calls } = sessionSteps({ kieu: "tu-choi" }, { kieu: "khong-goi-duoc" });
    expect(await ensureSession(SESSION, steps)).toEqual({ kind: "ready", session: SESSION, issued: false });
    expect(calls).toEqual({ codes: 0, issue: 0 });
  });

  it("refusing the phone is a normal answer, and nothing reaches the server", async () => {
    const { steps, calls } = sessionSteps({ kieu: "tu-choi" }, { kieu: "xong", phien: SESSION });
    expect(await ensureSession(null, steps)).toEqual({ kind: "refused" });
    expect(calls.issue).toBe(0);
  });

  it("outside Zalo, and an EMPTY phone code, send nothing", async () => {
    const outside = sessionSteps({ kieu: "ngoai-zalo" }, { kieu: "xong", phien: SESSION });
    expect(await ensureSession(null, outside.steps)).toEqual({ kind: "outside-zalo" });
    const empty = sessionSteps({ kieu: "xong", du_lieu: { ...CODES, ma_so_dien_thoai: "" } }, { kieu: "xong", phien: SESSION });
    expect(await ensureSession(null, empty.steps)).toEqual({ kind: "failed" });
    expect(outside.calls.issue + empty.calls.issue).toBe(0);
  });

  it("a server that issues no session is `failed`; one that does is `ready`, marked as newly issued", async () => {
    expect(await ensureSession(null, sessionSteps({ kieu: "xong", du_lieu: CODES }, { kieu: "ma-het-han" }).steps)).toEqual({
      kind: "failed",
    });
    expect(await ensureSession(null, sessionSteps({ kieu: "xong", du_lieu: CODES }, { kieu: "xong", phien: SESSION }).steps)).toEqual({
      kind: "ready",
      session: SESSION,
      issued: true,
    });
  });
});

/* ============================================================================================
   THE TWO FLOWS
   ============================================================================================ */

type Recorded = { sent: { bearer: string; body: Record<string, unknown> }[]; opened: number; sessions: (Phien | null)[] }; // vi-name-ok: existing type

function flowSteps(opts: {
  session: Awaited<ReturnType<QuickRequestSteps["ensureSession"]>>;
  name?: KetQuaXin<string>;
  send?: KetQuaGui; // vi-name-ok: existing type
  opens?: boolean;
}): { steps: QuickRequestSteps; rec: Recorded } {
  const rec: Recorded = { sent: [], opened: 0, sessions: [] };
  const steps: QuickRequestSteps = {
    ensureSession: () => Promise.resolve(opts.session),
    requestName: () => Promise.resolve(opts.name ?? { kieu: "xong", du_lieu: NAME }),
    send: (bearer, request) => {
      rec.sent.push({ bearer, body: JSON.parse(thanYeuCau(request)) as Record<string, unknown> });
      return Promise.resolve(opts.send ?? { kieu: "xong", ma_yeu_cau: "YC-1" });
    },
    openChat: () => {
      rec.opened++;
      return Promise.resolve(opts.opens ?? true);
    },
  };
  return { steps, rec };
}

const onSessionOf = (rec: Recorded) => (p: Phien | null) => void rec.sessions.push(p); // vi-name-ok: existing type

describe("Chat với chuyên viên: session → name → POST chat → open the chat", () => {
  it("the whole path: the body is a chat carrying the name, sent with the session's bearer, then the chat opens", async () => {
    const { steps, rec } = flowSteps({ session: { kind: "ready", session: SESSION, issued: true } });
    expect(await chatWithSpecialist(null, "", onSessionOf(rec), steps)).toEqual({ kind: "opened", delivered: true });
    expect(rec.sent).toHaveLength(1);
    expect(rec.sent[0]!.bearer).toBe(SESSION.token);
    expect(rec.sent[0]!.body["kind"]).toBe("chat");
    expect(rec.sent[0]!.body["displayName"]).toBe(NAME);
    expect(rec.opened).toBe(1);
    // A freshly issued session goes to the in-memory holder, like the login block does.
    expect(rec.sessions).toEqual([SESSION]);
  });

  it("refusing the phone: nothing sent, the chat NOT opened from this button", async () => {
    const { steps, rec } = flowSteps({ session: { kind: "refused" } });
    expect(await chatWithSpecialist(null, "", onSessionOf(rec), steps)).toEqual({ kind: "refused" });
    expect(rec.sent).toHaveLength(0);
    expect(rec.opened).toBe(0);
  });

  it("refusing the NAME is fine: the request goes without it and the chat opens", async () => {
    const { steps, rec } = flowSteps({ session: { kind: "ready", session: SESSION, issued: false }, name: { kieu: "tu-choi" } });
    expect(await chatWithSpecialist(SESSION, "", onSessionOf(rec), steps)).toEqual({ kind: "opened", delivered: true });
    expect(rec.sent[0]!.body).not.toHaveProperty("displayName");
    expect(rec.sessions, "an existing session is not re-set").toEqual([]);
  });

  it("the POST failing still opens the chat — the chat is what the citizen came for", async () => {
    const { steps, rec } = flowSteps({ session: { kind: "ready", session: SESSION, issued: false }, send: { kieu: "khong-goi-duoc" } });
    expect(await chatWithSpecialist(SESSION, "", onSessionOf(rec), steps)).toEqual({ kind: "opened", delivered: false });
    expect(rec.opened).toBe(1);
  });

  it("an expired session (401) is forgotten, so the next tap logs in again", async () => {
    const { steps, rec } = flowSteps({ session: { kind: "ready", session: SESSION, issued: false }, send: { kieu: "chua-dang-nhap" } });
    expect(await chatWithSpecialist(SESSION, "", onSessionOf(rec), steps)).toEqual({ kind: "opened", delivered: false });
    expect(rec.sessions).toEqual([null]);
  });

  it("a session that could not be issued (not a refusal) still opens the chat, sending nothing", async () => {
    const { steps, rec } = flowSteps({ session: { kind: "failed" } });
    expect(await chatWithSpecialist(null, "", onSessionOf(rec), steps)).toEqual({ kind: "opened", delivered: false });
    expect(rec.sent).toHaveLength(0);
  });

  it("outside Zalo, and a window the platform did not open, are their own branches", async () => {
    expect(await chatWithSpecialist(null, "", () => {}, flowSteps({ session: { kind: "outside-zalo" } }).steps)).toEqual({
      kind: "outside-zalo",
    });
    expect(
      await chatWithSpecialist(SESSION, "", () => {}, flowSteps({ session: { kind: "ready", session: SESSION, issued: false }, opens: false }).steps),
    ).toEqual({ kind: "not-opened", delivered: true });
  });
});

describe("Nhận ưu đãi qua SMS: session → POST sms_promo / sms_optout", () => {
  it("subscribe and unsubscribe send their kind and NO name — the name is never asked here", async () => {
    for (const kind of ["sms_promo", "sms_optout"] as const) {
      const { steps, rec } = flowSteps({ session: { kind: "ready", session: SESSION, issued: false } });
      let asked = false;
      steps.requestName = () => {
        asked = true;
        return Promise.resolve({ kieu: "xong", du_lieu: NAME });
      };
      const out = await sendSmsChoice(kind, SESSION, "", onSessionOf(rec), steps);
      expect(out).toEqual({ kind: "sent", result: { kieu: "xong", ma_yeu_cau: "YC-1" } });
      expect(rec.sent[0]!.body["kind"]).toBe(kind);
      expect(rec.sent[0]!.body).not.toHaveProperty("displayName");
      expect(asked, `${kind} asked Zalo for the name`).toBe(false);
    }
  });

  it("refused, outside Zalo, and a failed login send nothing", async () => {
    for (const [session, kind] of [
      [{ kind: "refused" }, "refused"],
      [{ kind: "outside-zalo" }, "outside-zalo"],
      [{ kind: "failed" }, "session-failed"],
    ] as const) {
      const { steps, rec } = flowSteps({ session });
      expect((await sendSmsChoice("sms_promo", null, "", onSessionOf(rec), steps)).kind).toBe(kind);
      expect(rec.sent).toHaveLength(0);
    }
  });
});

/* ============================================================================================
   SUBSCRIPTION — derived from the citizen's own list, the latest act wins
   ============================================================================================ */

describe("deriveSmsSubscription", () => {
  const row = (loai: YeuCauDaGui["loai"], tao_luc: string, ma = tao_luc): YeuCauDaGui => ({ ma, loai, trang_thai: "moi", tao_luc }); // vi-name-ok: existing type

  it("no SMS row at all — not subscribed (other kinds do not count)", () => {
    expect(deriveSmsSubscription([])).toBe("not-subscribed");
    expect(deriveSmsSubscription([row("consult", "2026-10-07T01:00:00Z"), row("chat", "2026-10-07T02:00:00Z")])).toBe(
      "not-subscribed",
    );
  });

  it("the latest act decides, whatever order the list comes in", () => {
    const promo = row("sms_promo", "2026-10-07T01:00:00Z");
    const optout = row("sms_optout", "2026-10-07T02:00:00Z");
    const again = row("sms_promo", "2026-10-07T03:00:00Z");
    expect(deriveSmsSubscription([promo])).toBe("subscribed");
    expect(deriveSmsSubscription([optout, promo])).toBe("not-subscribed");
    expect(deriveSmsSubscription([promo, optout])).toBe("not-subscribed");
    expect(deriveSmsSubscription([again, optout, promo])).toBe("subscribed");
    expect(deriveSmsSubscription([promo, optout, again])).toBe("subscribed");
  });

  it("an exact tie goes to the opt-out, and an unreadable time is skipped, not guessed", () => {
    expect(deriveSmsSubscription([row("sms_promo", "2026-10-07T01:00:00Z", "a"), row("sms_optout", "2026-10-07T01:00:00Z", "b")])).toBe(
      "not-subscribed",
    );
    expect(deriveSmsSubscription([row("sms_optout", "khong-phai-ngay"), row("sms_promo", "2026-10-07T01:00:00Z")])).toBe("subscribed");
  });
});

/* ============================================================================================
   RENDERING — every state, without Zalo or a server
   ============================================================================================ */

describe("the chat block renders every state in words", () => {
  const view = (state: Parameters<typeof ChatWithSpecialistView>[0]["state"]) =>
    render(<ChatWithSpecialistView state={state} onPress={() => {}} />);

  it("idle: why the number and name are wanted, the button, and the way to chat sharing nothing", () => {
    const markup = view({ kind: "idle" });
    const t = textOf(markup);
    expect(t).toContain(CHAT_SPECIALIST.title);
    expect(t).toContain(CHAT_SPECIALIST.why);
    expect(t).toContain(CHAT_SPECIALIST.button);
    // The floating button's real label, not a retyped copy that drifts.
    expect(CHAT_SPECIALIST.share_nothing).toContain(`“${NHAN_NUT_CHAT}”`);
    expect(CHAT_SPECIALIST.refused).toContain(`“${NHAN_NUT_CHAT}”`);
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
    for (const svg of markup.match(/<svg[\s>][^>]*>/g) ?? []) expect(svg).toContain('aria-hidden="true"');
    expect(markup).toMatch(/<h2[^>]*id="chat-specialist"/);
  });

  it("busy is said on the button in words, and the button cannot be pressed twice", () => {
    const markup = view({ kind: "busy" });
    expect(textOf(markup)).toContain(CHAT_SPECIALIST.busy);
    expect(markup).toMatch(/<button[^>]*disabled=""/);
  });

  it("each outcome has its own sentence, none an error code", () => {
    const cases: [Parameters<typeof view>[0], string][] = [
      [{ kind: "refused" }, CHAT_SPECIALIST.refused],
      [{ kind: "outside-zalo" }, LOI_MO_NGOAI],
      [{ kind: "opened", delivered: true }, CHAT_SPECIALIST.opened_delivered],
      [{ kind: "opened", delivered: false }, CHAT_SPECIALIST.opened_undelivered],
      [{ kind: "not-opened", delivered: false }, CHAT_SPECIALIST.not_opened],
    ];
    for (const [state, sentence] of cases) {
      const t = textOf(view(state));
      expect(t, state.kind).toContain(sentence);
      expect(t).not.toMatch(/\b(400|401|429|503|undefined|null)\b/);
    }
  });
});

describe("the SMS block: consent, the stop button never out of reach, state in words", () => {
  const view = (subscription: "unknown" | "subscribed" | "not-subscribed", action: Parameters<typeof SmsOffersView>[0]["action"] = { kind: "idle" }) =>
    render(<SmsOffersView subscription={subscription} action={action} onChoose={() => {}} />);

  it("the consent names the number, the content (ViHAT offers) and how to stop", () => {
    expect(SMS_OFFERS.consent).toContain("số điện thoại Zalo của bạn");
    expect(SMS_OFFERS.consent).toContain("ưu đãi và chương trình khuyến mại của ViHAT Group");
    expect(SMS_OFFERS.consent).toContain(`“${SMS_OFFERS.unsubscribe}” ngay tại đây, bất cứ lúc nào`);
    expect(textOf(view("unknown"))).toContain(SMS_OFFERS.consent);
  });

  // Button labels, read from the buttons themselves: the consent sentence quotes both labels, so a text search
  // over the whole block would find them whatever buttons are drawn.
  const buttons = (markup: string) => [...markup.matchAll(/<button[^>]*>([\s\S]*?)<\/button>/g)].map((m) => textOf(m[1]!).trim());

  it("unknown (no session yet): BOTH buttons — a subscriber must always find the stop button", () => {
    expect(buttons(view("unknown"))).toEqual([SMS_OFFERS.subscribe, SMS_OFFERS.unsubscribe]);
  });

  it("subscribed: says so in words, offers only the stop button", () => {
    const markup = view("subscribed");
    expect(textOf(markup)).toContain(SMS_OFFERS.status_subscribed);
    expect(buttons(markup)).toEqual([SMS_OFFERS.unsubscribe]);
  });

  it("not subscribed (the server's list says so): only the subscribe button", () => {
    const markup = view("not-subscribed");
    expect(textOf(markup)).not.toContain(SMS_OFFERS.status_subscribed);
    expect(buttons(markup)).toEqual([SMS_OFFERS.subscribe]);
  });

  it("each outcome has its sentence; the server's own refusal sentence is shown verbatim", () => {
    const done = (choice: "sms_promo" | "sms_optout", outcome: SmsOutcome) =>
      textOf(view("unknown", { kind: "done", choice, outcome }));
    expect(done("sms_promo", { kind: "sent", result: { kieu: "xong", ma_yeu_cau: "Y" } })).toContain(SMS_OFFERS.done_subscribe);
    expect(done("sms_optout", { kind: "sent", result: { kieu: "xong", ma_yeu_cau: "Y" } })).toContain(SMS_OFFERS.done_unsubscribe);
    expect(done("sms_promo", { kind: "refused" })).toContain(SMS_OFFERS.refused_subscribe);
    expect(done("sms_optout", { kind: "refused" })).toContain(SMS_OFFERS.refused_unsubscribe);
    expect(done("sms_promo", { kind: "outside-zalo" })).toContain(LOI_MO_NGOAI);
    expect(done("sms_promo", { kind: "session-failed" })).toContain(SMS_OFFERS.session_failed);
    expect(done("sms_promo", { kind: "sent", result: { kieu: "chua-dang-nhap" } })).toContain(SMS_OFFERS.expired);
    expect(done("sms_promo", { kind: "sent", result: { kieu: "khong-goi-duoc" } })).toContain(SMS_OFFERS.network);
    expect(done("sms_promo", { kind: "sent", result: { kieu: "tu-choi", cau: "Câu của máy chủ." } })).toContain("Câu của máy chủ.");
    expect(done("sms_promo", { kind: "sent", result: { kieu: "tu-choi", cau: "" } })).toContain(TU_VAN.cau_lui_tu_choi);
  });

  it("busy: words on the pressed button, both buttons held", () => {
    const markup = view("unknown", { kind: "busy", choice: "sms_optout" });
    expect(textOf(markup)).toContain(SMS_OFFERS.busy);
    expect((markup.match(/disabled=""/g) ?? []).length).toBe(2);
  });
});

describe("where the blocks sit, and what the declarations say about them", () => {
  it("Liên hệ carries the chat block; the floating button is a separate thing it does not replace", () => {
    expect(textOf(render(<ContactScreen />))).toContain(CHAT_SPECIALIST.title);
  });

  it("the home screen carries the SMS block, right after 'Tin ViHAT'", () => {
    const t = textOf(render(<HomeScreen />));
    const news = t.indexOf(TIEU_DE_KHOI_TIN);
    const sms = t.indexOf(SMS_OFFERS.title);
    expect(news).toBeGreaterThan(-1);
    expect(sms).toBeGreaterThan(news);
  });

  it("the platform-call declarations name both features where the call runs behind them", () => {
    const row = (api: string) => KHAI_BAO_LOI_GOI.find((k) => k.api === api)!;
    for (const api of ["getPhoneNumber", "getAccessToken"]) {
      expect(row(api).tinh_nang, api).toContain("Chat với chuyên viên");
      expect(row(api).tinh_nang, api).toContain("Nhận ưu đãi qua SMS");
      expect(row(api).man, api).toContain("Trang chủ");
      expect(row(api).de_lam_gi, api).toContain("Chat với chuyên viên");
    }
    const name = row("getUserInfo");
    expect(name.nua).toBe("ca-hai");
    expect(name.tinh_nang).toContain("Chat với chuyên viên");
    expect(name.roi_khoi_may, "the name now leaves the phone — the row must say so").toContain("máy chủ của Tập đoàn ViHAT Group");
    expect(name.commune_app?.roi_khoi_may, "the commune app still sends the name nowhere on its own").toBe("");
  });

  it("no new sentence says 'demo' or 'trải nghiệm'", () => {
    const all = [...Object.values(CHAT_SPECIALIST), ...Object.values(SMS_OFFERS)].join("\n");
    expect(all).not.toMatch(/\bdemo\b|trải nghiệm|trình diễn/i);
  });
});
