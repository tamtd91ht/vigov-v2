import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { OFFICES } from "../../content/company-profile";
import type { CurrentLocationResult } from "../dang-nhap/current-location";

import { NearbyOfficesView } from "./LienHeTinhNang";
import {
  createNearbyFlow,
  distanceKm,
  failureNote,
  formatDistanceKm,
  locateNearbyOffices,
  type NearbyState,
  sortByDistance,
} from "./nearby-offices";
import { NEARBY_OFFICES } from "./noi-dung";

/**
 * "TÌM VĂN PHÒNG GẦN TÔI" (owner 12/10/2026). What no other test says for this block:
 *   1. the distance is right — haversine against pairs whose distance is known independently;
 *   2. Zalo is asked only after the app's own "Cho phép", and "Không" asks nothing;
 *   3. every failure keeps the published order and says what to do next, never a code;
 *   4. the person's coordinates never leave `locateNearbyOffices` — the result carries distances only.
 */

const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, "&")
    .replace(/&#x27;/g, "'")
    .replace(/\s+/g, " ");

const HQ = OFFICES.findIndex((o) => o.name === "Trụ sở chính");
const HANOI = OFFICES.findIndex((o) => o.name === "Chi nhánh Hà Nội");
const CAMBODIA = OFFICES.findIndex((o) => o.name === "Chi nhánh Cambodia");

/** A fake exchange answer — a made-up point, not anybody's location. */
const located = (lat: number, lng: number): CurrentLocationResult => ({
  kieu: "xong",
  location: { latitude: lat, longitude: lng },
});

describe("great-circle distance, on the phone", () => {
  it("HCM head office ↔ Hà Nội branch ≈ 1140 km (±15)", () => {
    const km = distanceKm(OFFICES[HQ]!, OFFICES[HANOI]!);
    expect(km).toBeGreaterThan(1125);
    expect(km).toBeLessThan(1155);
  });

  it("one degree of longitude on the equator ≈ 111.2 km; the same point is 0 km; symmetric", () => {
    expect(distanceKm({ lat: 0, lng: 0 }, { lat: 0, lng: 1 })).toBeCloseTo(111.2, 1);
    expect(distanceKm(OFFICES[HQ]!, OFFICES[HQ]!)).toBe(0);
    expect(distanceKm(OFFICES[HQ]!, OFFICES[CAMBODIA]!)).toBeCloseTo(distanceKm(OFFICES[CAMBODIA]!, OFFICES[HQ]!), 9);
  });

  it("HCM head office ↔ Phnom Penh branch is a few hundred km, not a thousand", () => {
    const km = distanceKm(OFFICES[HQ]!, OFFICES[CAMBODIA]!);
    expect(km).toBeGreaterThan(180);
    expect(km).toBeLessThan(230);
  });
});

describe("rounding — what 'Cách khoảng X km' prints", () => {
  it.each([
    [0.04, "0,0"],
    [3.44, "3,4"],
    [3.46, "3,5"],
    [9.94, "9,9"],
    [9.96, "10"], // decided on the printed value
    [12.5, "13"],
    [212.4, "212"],
    [1140.4, "1.140"],
    [12345.6, "12.346"],
  ])("%s km → %s", (km, printed) => {
    expect(formatDistanceKm(km)).toBe(printed);
  });

  it("the sentence carries the number and the unit", () => {
    expect(NEARBY_OFFICES.distance(formatDistanceKm(3.44))).toBe("Cách khoảng 3,4 km");
  });
});

describe("sort order", () => {
  it("from Hà Nội: the Hà Nội branch, then Phnom Penh, then the HCM head office", () => {
    const rows = sortByDistance({ lat: 21.0285, lng: 105.8542 });
    expect(rows.map((r) => r.index)).toEqual([HANOI, CAMBODIA, HQ]);
    expect(rows.map((r) => r.km)).toEqual([...rows.map((r) => r.km)].sort((a, b) => a - b));
  });

  it("from central HCM: the head office first, a few km away", () => {
    const rows = sortByDistance({ lat: 10.7769, lng: 106.7009 });
    expect(rows[0]!.index).toBe(HQ);
    expect(rows[0]!.km).toBeLessThan(10);
    expect(rows.map((r) => r.index)).toEqual([HQ, CAMBODIA, HANOI]);
  });

  it("ties keep the published order", () => {
    const twin = { name: "A", address: "a", lat: 1, lng: 1 };
    const rows = sortByDistance({ lat: 0, lng: 0 }, [twin, { ...twin, name: "B" }]);
    expect(rows.map((r) => r.index)).toEqual([0, 1]);
  });
});

describe("the person's coordinates are not kept", () => {
  it("the result holds only { index, km } rows — no latitude, no longitude, no copy of the input", async () => {
    const result = await locateNearbyOffices(async () => located(10.123456, 106.654321));
    expect(result.step).toBe("sorted");
    const json = JSON.stringify(result);
    expect(json).not.toContain("10.123456");
    expect(json).not.toContain("106.654321");
    expect(json).not.toMatch(/lat|lng|latitude|longitude/);
    if (result.step !== "sorted") throw new Error("unreachable");
    for (const row of result.rows) expect(Object.keys(row).sort()).toEqual(["index", "km"]);
  });

  it("the flow's states never carry them either", async () => {
    const states: NearbyState[] = [];
    const flow = createNearbyFlow((s) => states.push(s), () => locateNearbyOffices(async () => located(10.123456, 106.654321)));
    flow.find();
    await flow.allow();
    expect(JSON.stringify(states)).not.toMatch(/10\.123456|106\.654321|lat|lng/);
  });
});

describe("Zalo only after the app's own 'Cho phép'", () => {
  it("the first tap shows the question and asks Zalo nothing; 'Cho phép' asks once", async () => {
    const locate = vi.fn(async () => ({ step: "sorted" as const, rows: [] }));
    const states: NearbyState[] = [];
    const flow = createNearbyFlow((s) => states.push(s), locate);
    flow.find();
    expect(states).toEqual([{ step: "asking" }]);
    expect(locate).not.toHaveBeenCalled();
    await flow.allow();
    expect(locate).toHaveBeenCalledTimes(1);
    expect(states.map((s) => s.step)).toEqual(["asking", "locating", "sorted"]);
  });

  it("'Không' asks nothing and keeps the published order, with one sentence of what to do", () => {
    const locate = vi.fn(async () => ({ step: "sorted" as const, rows: [] }));
    const states: NearbyState[] = [];
    const flow = createNearbyFlow((s) => states.push(s), locate);
    flow.find();
    flow.deny();
    expect(locate).not.toHaveBeenCalled();
    expect(states.at(-1)).toEqual({ step: "default", note: NEARBY_OFFICES.denied });
    // …and "Không" is not remembered as a yes: the next tap asks again.
    flow.find();
    expect(states.at(-1)).toEqual({ step: "asking" });
  });

  it("once allowed in this screen opening, a later tap goes straight to Zalo", async () => {
    const locate = vi.fn(async () => ({ step: "sorted" as const, rows: [] }));
    const states: NearbyState[] = [];
    const flow = createNearbyFlow((s) => states.push(s), locate);
    flow.find();
    await flow.allow();
    flow.find();
    await vi.waitFor(() => expect(locate).toHaveBeenCalledTimes(2));
    expect(states.filter((s) => s.step === "asking")).toHaveLength(1);
  });

  it("the question shows the purpose and both answers, in place of the find button", () => {
    const text = textOf(
      renderToStaticMarkup(
        <NearbyOfficesView
          state={{ step: "asking" }}
          onFind={() => {}}
          onAllow={() => {}}
          onDeny={() => {}}
          onChiDuong={() => {}}
          cannotOpen={false}
        />,
      ),
    );
    expect(text).toContain(NEARBY_OFFICES.purpose);
    expect(text).toContain(NEARBY_OFFICES.allow);
    expect(text).toContain(NEARBY_OFFICES.deny);
    expect(text).not.toContain(NEARBY_OFFICES.find);
    // The offices are still there while the question is open.
    for (const o of OFFICES) expect(text).toContain(o.name);
  });
});

describe("failures keep the order and say what to do next", () => {
  const KINDS: Array<[Exclude<CurrentLocationResult["kieu"], "xong">, string]> = [
    ["tu-choi", NEARBY_OFFICES.zalo_refused],
    ["ngoai-zalo", NEARBY_OFFICES.outside_zalo],
    ["qua-nhieu-lan", NEARBY_OFFICES.too_many],
    ["tam-ngung", NEARBY_OFFICES.paused],
    ["chua-khai-host", NEARBY_OFFICES.paused],
    ["zalo-khong-tra-loi", NEARBY_OFFICES.retry],
    ["yeu-cau-hong", NEARBY_OFFICES.retry],
    ["khong-goi-duoc", NEARBY_OFFICES.retry],
    ["khong-lay-duoc-ma", NEARBY_OFFICES.retry],
  ];

  it.each(KINDS)("%s → its sentence, as a default-order state", async (kind, sentence) => {
    expect(failureNote(kind)).toBe(sentence);
    const result = await locateNearbyOffices(async () => ({ kieu: kind }) as CurrentLocationResult);
    expect(result).toEqual({ step: "default", note: sentence });
  });

  it("a rate limit says to wait, not to press again at once", () => {
    expect(NEARBY_OFFICES.too_many).toMatch(/chờ vài phút/);
  });

  it("a thrown exchange is a retry sentence, never a stuck 'Đang tìm…'", async () => {
    expect(await locateNearbyOffices(async () => Promise.reject(new Error("x")))).toEqual({
      step: "default",
      note: NEARBY_OFFICES.retry,
    });
  });

  it("every failure sentence names a next step and no code", () => {
    for (const [, sentence] of KINDS) {
      expect(sentence).toMatch(/bấm|chờ|mở/i);
      expect(sentence).not.toMatch(/\d{3}|[Ee]rror|code|SDK/);
    }
  });

  it("a default state renders the offices in the published order, with its one sentence, no distance", () => {
    const markup = renderToStaticMarkup(
      <NearbyOfficesView
        state={{ step: "default", note: NEARBY_OFFICES.too_many }}
        onFind={() => {}}
        onAllow={() => {}}
        onDeny={() => {}}
        onChiDuong={() => {}}
        cannotOpen={false}
      />,
    );
    const text = textOf(markup);
    expect(text).toContain(NEARBY_OFFICES.too_many);
    expect(text).not.toContain("Cách khoảng");
    const positions = OFFICES.map((o) => text.indexOf(o.name));
    expect(positions).toEqual([...positions].sort((a, b) => a - b));
  });
});

describe("the sorted list", () => {
  it("nearest first, each with its distance; the find button stays for another try", () => {
    const rows = sortByDistance({ lat: 21.0285, lng: 105.8542 });
    const text = textOf(
      renderToStaticMarkup(
        <NearbyOfficesView
          state={{ step: "sorted", rows }}
          onFind={() => {}}
          onAllow={() => {}}
          onDeny={() => {}}
          onChiDuong={() => {}}
          cannotOpen={false}
        />,
      ),
    );
    expect(text.indexOf(OFFICES[HANOI]!.name)).toBeLessThan(text.indexOf(OFFICES[CAMBODIA]!.name));
    expect(text.indexOf(OFFICES[CAMBODIA]!.name)).toBeLessThan(text.indexOf(OFFICES[HQ]!.name));
    for (const r of rows) expect(text).toContain(NEARBY_OFFICES.distance(formatDistanceKm(r.km)));
    expect(text).toContain(NEARBY_OFFICES.sorted);
    expect(text).toContain(NEARBY_OFFICES.find);
  });

  it("while locating, the button is disabled and says so in words", () => {
    const markup = renderToStaticMarkup(
      <NearbyOfficesView
        state={{ step: "locating" }}
        onFind={() => {}}
        onAllow={() => {}}
        onDeny={() => {}}
        onChiDuong={() => {}}
        cannotOpen={false}
      />,
    );
    expect(markup).toContain("disabled");
    expect(markup).toContain('aria-busy="true"');
    expect(textOf(markup)).toContain(NEARBY_OFFICES.locating);
  });
});
