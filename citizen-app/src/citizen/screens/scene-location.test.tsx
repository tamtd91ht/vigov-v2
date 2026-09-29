import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { isSceneLocation, OPTIONAL_SCENE_FIELDS, type NewReport, submitReportBody, ACCEPTED_FIELDS } from "../api/citizen-report-contract";

import { EntryStep, ConfirmStep, EMPTY_REPORT } from "./SubmitReportScreen";
import { SEND, SEND_LOCATION_WORDS } from "./copy";
import { COMMUNE_LOCATION_WORDS, CommuneSendScreen } from "./CommuneAppReports";
import {
  formatCoordinates,
  type GetSceneLocation,
  locateOnce,
  SceneLocationControl,
  type SceneLocationFailure,
} from "./scene-location";

/**
 * "LẤY VỊ TRÍ HIỆN TẠI" — the state half: what the screens show, what one tap keeps, and what the intake
 * body carries. Rendered with `react-dom/server` like the other screen tests (no DOM, no clicks): the tap
 * itself is `locateOnce`, tested directly; the React glue (`useSceneLocation`) only wires it to state.
 */

const FORM: NewReport = {
  content: "Ổ gà lớn trước cổng chợ",
  address: "Đầu ngõ thôn Hà Lam",
  full_name: "Nguyễn Văn A",
  phone: "0900000000",
  anonymous: false,
};
const HERE = { lat: 15.571234, lng: 108.476543 };
const FAILURES: readonly SceneLocationFailure[] = ["tu-choi", "ngoai-zalo", "qua-nhieu-lan", "thu-lai", "tam-ngung"];
const FIVE = [...ACCEPTED_FIELDS].sort();

const keysOf = (body: string) => Object.keys(JSON.parse(body) as Record<string, unknown>).sort();

describe("intake body — the five keys, plus lat/lng only as a pair", () => {
  it("no location: EXACTLY the five keys, as before", () => {
    expect(keysOf(submitReportBody(FORM))).toEqual(FIVE);
    expect(keysOf(submitReportBody({ ...FORM, scene_location: null }))).toEqual(FIVE);
  });

  it("a location: the five keys plus both optional keys, as JSON numbers, unchanged", () => {
    const body = JSON.parse(submitReportBody({ ...FORM, scene_location: HERE })) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual([...ACCEPTED_FIELDS, ...OPTIONAL_SCENE_FIELDS].sort());
    expect(body["lat"]).toBe(HERE.lat);
    expect(body["lng"]).toBe(HERE.lng);
  });

  it("a partial or invalid pair is NEVER sent — neither key goes", () => {
    for (const bad of [
      { lat: 15.5 },
      { lng: 108.2 },
      { lat: Number.NaN, lng: 108.2 },
      { lat: 15.5, lng: Number.POSITIVE_INFINITY },
      { lat: 90.5, lng: 108.2 },
      { lat: 15.5, lng: -180.5 },
      { lat: "15.5", lng: "108.2" },
    ]) {
      const body = submitReportBody({ ...FORM, scene_location: bad as never });
      expect(keysOf(body), JSON.stringify(bad)).toEqual(FIVE);
    }
  });

  it("anonymous: name and phone emptied as always; the location the citizen tapped for still goes", () => {
    const body = JSON.parse(submitReportBody({ ...FORM, anonymous: true, scene_location: HERE })) as Record<string, unknown>;
    expect(body["reporter_name"]).toBe("");
    expect(body["reporter_phone"]).toBe("");
    expect([body["lat"], body["lng"]]).toEqual([HERE.lat, HERE.lng]);
  });

  it("the range check matches the server's bounds, edges included", () => {
    expect(isSceneLocation({ lat: -90, lng: -180 })).toBe(true);
    expect(isSceneLocation({ lat: 90, lng: 180 })).toBe(true);
    expect(isSceneLocation({ lat: 0, lng: 0 })).toBe(true);
    expect(isSceneLocation(null)).toBe(false);
    expect(isSceneLocation({ lat: 1 })).toBe(false);
  });
});

describe("one tap (`locateOnce`)", () => {
  it("success keeps exactly the pair", async () => {
    expect(await locateOnce(async () => ({ kind: "xong", location: HERE }))).toEqual({ location: HERE, failure: null });
  });

  it("each failure is kept as its branch, with no location", async () => {
    for (const f of FAILURES) {
      expect(await locateOnce(async () => ({ kind: f }))).toEqual({ location: null, failure: f });
    }
  });

  it("a rejection, or a 'success' carrying an invalid pair, becomes `thu-lai` — never a half location", async () => {
    expect(await locateOnce(() => Promise.reject(new Error("x")))).toEqual({ location: null, failure: "thu-lai" });
    const bad: GetSceneLocation = async () => ({ kind: "xong", location: { lat: 15.5, lng: Number.NaN } });
    expect(await locateOnce(bad)).toEqual({ location: null, failure: "thu-lai" });
  });
});

describe("what the citizen sees — live form (shared app)", () => {
  type InputLocation = Parameters<typeof EntryStep>[0]["location"];
  const input = (form: NewReport, location: InputLocation) =>
    renderToStaticMarkup(createElement(EntryStep, { report: form, error: null, onChange: () => {}, onNext: () => {}, location }));
  const idle = { locating: false, failure: null, onLocate: () => {} };

  it("no injected function: no button at all", () => {
    expect(input(FORM, null)).not.toContain(SEND_LOCATION_WORDS.button);
  });

  it("before a tap: the button and why, under the address box, which stays empty and editable", () => {
    const html = input(EMPTY_REPORT, idle);
    expect(html).toContain(`<button type="button" class="cd-nut-phu">${SEND_LOCATION_WORDS.button}</button>`);
    expect(html).toContain(SEND_LOCATION_WORDS.why);
    expect(html.indexOf('id="cd-dia-chi"')).toBeGreaterThan(-1);
    expect(html.indexOf('id="cd-dia-chi"')).toBeLessThan(html.indexOf(SEND_LOCATION_WORDS.button));
  });

  it("after success: 'Đã lấy vị trí hiện tại (lat, lng)' with five decimals; the typed address is untouched", () => {
    const html = input({ ...FORM, scene_location: HERE }, idle);
    expect(formatCoordinates(HERE)).toBe("15.57123, 108.47654");
    expect(html).toContain(SEND_LOCATION_WORDS.found("15.57123, 108.47654"));
    expect(html).toContain("Đã lấy vị trí hiện tại");
    expect(html).toContain(SEND_LOCATION_WORDS.button_again);
    // Never a guessed address: the box holds exactly what the citizen typed.
    expect(html).toContain(`value="${FORM.address}"`);
  });

  it("while locating: the button says so and is disabled", () => {
    const html = input(FORM, { ...idle, locating: true });
    expect(html).toContain(SEND_LOCATION_WORDS.locating);
    expect(html).toMatch(/<button type="button" class="cd-nut-phu" disabled="">/);
  });

  it("each failure shows the client's own sentence, which says what to do next and never a code", () => {
    for (const f of FAILURES) {
      const html = input(FORM, { ...idle, failure: f });
      const sentence = SEND_LOCATION_WORDS.failures[f];
      expect(html, f).toContain(sentence);
      // Every branch points at the address box — the way that always works.
      expect(sentence, f).toMatch(/ghi rõ nơi xảy ra/);
      expect(sentence, f).not.toMatch(/\b(4\d\d|5\d\d)\b|rate_limited|zalo_location_unavailable|unavailable/);
    }
    // The server's own sentences are not the client's (vigov-client.ts rule).
    expect(Object.values(SEND_LOCATION_WORDS.failures)).not.toContain(
      "Chưa lấy được vị trí từ Zalo. Vui lòng thử lại, hoặc tự nhập địa chỉ.",
    );
  });

  it("the confirmation step names the commune AND says the location goes with it", () => {
    const html = renderToStaticMarkup(
      createElement(ConfirmStep, { commune_name: "Xã Thử Nghiệm", onSend: () => {}, onEdit: () => {}, location: HERE }),
    );
    expect(html).toContain("Xã Thử Nghiệm");
    expect(html).toContain(SEND.confirm_location("15.57123, 108.47654"));
    const without = renderToStaticMarkup(
      createElement(ConfirmStep, { commune_name: "Xã Thử Nghiệm", onSend: () => {}, onEdit: () => {} }),
    );
    expect(without).not.toContain("Kèm vị trí hiện tại");
  });
});

describe("what the citizen sees — send form of the commune's own app", () => {
  it("the control speaks 'bà con' and shows the real coordinates", () => {
    const html = renderToStaticMarkup(
      createElement(SceneLocationControl, {
        words: COMMUNE_LOCATION_WORDS,
        look: "commune",
        locating: false,
        location: HERE,
        failure: null,
        onLocate: () => {},
      }),
    );
    expect(html).toContain(COMMUNE_LOCATION_WORDS.found("15.57123, 108.47654"));
    expect(html).toContain("Bà con");
    expect(html).toContain('class="xa-nut xa-nut--phu"');
    // The old token-only sentence is gone.
    expect(html).not.toContain("Đã nhận mã vị trí");
  });

  it("every failure has a 'bà con' sentence pointing at the address box", () => {
    for (const f of FAILURES) {
      expect(COMMUNE_LOCATION_WORDS.failures[f], f).toMatch(/ghi rõ nơi xảy ra/);
    }
  });

  it("rendering the send screen never taps: no exchange runs on its own", () => {
    const get = vi.fn<GetSceneLocation>();
    const html = renderToStaticMarkup(
      createElement(CommuneSendScreen, {
        commune_name: "Xã Thử Nghiệm",
        full_name: null,
        onBack: () => {},
        onSessionLost: () => {},
        onSent: () => {},
        onOpenReport: () => {},
        getSceneLocation: get,
      }),
    );
    expect(html.length).toBeGreaterThan(0);
    expect(get).not.toHaveBeenCalled();
  });
});
