import type { ReactElement, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, type Mock, vi } from "vitest";

/**
 * FIXES B · E · F · J · GP (14/10/2026) — what each button DOES, not only what it shows.
 *
 * The suite renders on the server and has no DOM to click. These cases call the components as plain functions
 * with `useState` / `useEffect` replaced by spies, walk the returned element tree, and call the `onClick` a press
 * would call. Child components are not expanded — each case reads one component's own decisions.
 */

const calls = vi.hoisted(() => ({
  log: [] as string[],
  stateValue: undefined as unknown,
  setState: undefined as unknown as Mock,
  effects: [] as Array<() => void | (() => void)>,
}));

vi.mock("react", async (importOriginal) => {
  const real = await importOriginal<typeof import("react")>();
  return {
    ...real,
    useState: (initial: unknown) => {
      const value =
        calls.stateValue !== undefined
          ? calls.stateValue
          : typeof initial === "function"
            ? (initial as () => unknown)()
            : initial;
      return [value, calls.setState];
    },
    useEffect: (effect: () => void | (() => void)) => {
      calls.effects.push(effect);
    },
  };
});

vi.mock("../../lib/cuon-toi", () => ({
  cuonToiMoc: () => calls.log.push("cuonToiMoc"),
  scrollMainToTop: () => calls.log.push("scrollMainToTop"),
}));

import { KhungApp } from "../../App";
import { CONTACT, SOLUTIONS } from "../../content/company-profile";

import { ChiTietGiaiPhap } from "./ChiTietGiaiPhap";
import { ContactScreen } from "./ContactScreen";
import { MOC_QUAN_LY_QUYEN, MOC_TIM_VAN_PHONG } from "./dieu-huong";
import { HomeScreen } from "./HomeScreen";
import { MUC_MENU_NHANH } from "./MenuNhanh";
import { CHAT_FAILURE_CLOSE_LABEL, CHAT_FAILURE_VISIBLE_MS, NutChatOA } from "./NutChatOA";
import { QuanLyQuyenScreen } from "./QuanLyQuyenScreen";
import { SolutionsScreen } from "./SolutionsScreen";

type AnyElement = ReactElement<Record<string, unknown> & { children?: ReactNode }>;

/** Every element in a returned tree, depth first. Function components are NOT called. */
function elements(node: ReactNode): AnyElement[] {
  if (node === null || node === undefined || typeof node === "boolean") return [];
  if (typeof node === "string" || typeof node === "number") return [];
  if (Array.isArray(node)) return node.flatMap(elements);
  const el = node as AnyElement;
  return [el, ...elements(el.props.children)];
}

/** The plain text inside an element, children only (what a screen reader names a button by). */
function textIn(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textIn).join(" ");
  return textIn((node as AnyElement).props.children);
}

const buttons = (tree: ReactNode) => elements(tree).filter((el) => el.type === "button");
const press = (el: AnyElement) => (el.props["onClick"] as () => void)();

beforeEach(() => {
  calls.log = [];
  calls.stateValue = undefined;
  calls.effects = [];
  calls.setState = vi.fn((next: unknown) => calls.log.push(`setState:${String(next)}`));
});

afterEach(() => {
  vi.useRealTimers();
});

describe("GP — Giải pháp nổi bật on the home screen open that solution's detail page", () => {
  it("one real <button> per solution, named by its product and headline, going to Giải pháp with the solution id", () => {
    const onDi = vi.fn();
    const cards = buttons(HomeScreen({ onDi })).filter((b) => b.props["className"] === "gp-noi-bat__nut");
    expect(cards, "a featured solution is not tappable").toHaveLength(SOLUTIONS.length);

    cards.forEach((card, i) => {
      const solution = SOLUTIONS[i]!;
      expect(card.props["type"]).toBe("button");
      const name = textIn(card.props.children);
      expect(name).toContain(solution.headline);
      if (solution.product) expect(name).toContain(solution.product);
      press(card);
      expect(onDi).toHaveBeenLastCalledWith({ man: "solutions", moc: solution.id });
    });
  });

  it("SolutionsScreen opens ChiTietGiaiPhap of the solution `moc` names — the same page its own cards open", () => {
    for (const solution of SOLUTIONS) {
      const tree = SolutionsScreen({ moc: solution.id }) as AnyElement;
      expect(tree.type, `moc ${solution.id} did not open the detail page`).toBe(ChiTietGiaiPhap);
      expect((tree.props["giai_phap"] as { id: string }).id).toBe(solution.id);
    }
  });

  it("any other `moc` (or none) keeps the list — a stray anchor never opens a detail page", () => {
    for (const moc of [undefined, "", MOC_TIM_VAN_PHONG, "khong-co"]) {
      const tree = SolutionsScreen({ moc }) as AnyElement;
      expect(tree.type, `moc ${String(moc)} opened a detail page`).not.toBe(ChiTietGiaiPhap);
    }
  });
});

describe("B — opening and closing a sub-screen puts .app-main back at the top first", () => {
  it("Giải pháp: a card scrolls to the top, then opens its detail", () => {
    const card = buttons(SolutionsScreen({})).find((b) => b.props["className"] === "solution--mo")!;
    press(card);
    expect(calls.log).toEqual(["scrollMainToTop", `setState:${SOLUTIONS[0]!.id}`]);
  });

  it("Giải pháp: '‹ Tất cả giải pháp' scrolls to the top, then shows the list", () => {
    const detail = SolutionsScreen({ moc: SOLUTIONS[0]!.id }) as AnyElement;
    (detail.props["onQuayLai"] as () => void)();
    expect(calls.log).toEqual(["scrollMainToTop", "setState:null"]);
  });

  it("Liên hệ: Quản lý quyền and the policy row both scroll to the top before opening", () => {
    const rows = buttons(ContactScreen({})).filter((b) => b.props["className"] === "action action--mo");
    expect(rows.length, "Liên hệ lost its sub-screen rows").toBe(2);
    for (const row of rows) {
      calls.log = [];
      press(row);
      expect(calls.log[0], `"${textIn(row.props.children)}" opened without scrolling to the top`).toBe("scrollMainToTop");
      expect(calls.log).toHaveLength(2);
    }
  });

  it("Liên hệ: going back from Quản lý quyền, and on to the policy from it, scroll to the top too", () => {
    const permissions = ContactScreen({ moc: MOC_QUAN_LY_QUYEN }) as AnyElement;
    expect(permissions.type).toBe(QuanLyQuyenScreen);
    (permissions.props["onQuayLai"] as () => void)();
    expect(calls.log).toEqual(["scrollMainToTop", "setState:contact"]);
    calls.log = [];
    (permissions.props["onOpenPolicy"] as () => void)();
    expect(calls.log).toEqual(["scrollMainToTop", "setState:policy-from-permissions"]);
  });
});

describe("E · J — the quick menu says what each tile does", () => {
  it("the dialling tile is 'Hotline', and no tile is called 'Tư vấn' (that name is the form's)", () => {
    const hotline = MUC_MENU_NHANH.find((m) => m.kieu === "goi")!;
    expect(hotline.nhan).toBe("Hotline");
    expect(hotline.kieu === "goi" && hotline.so).toBe(CONTACT.hotlineDialable);
    expect(MUC_MENU_NHANH.map((m) => m.nhan)).not.toContain("Tư vấn");
  });

  it("'Văn phòng gần tôi' lands on the office block of Liên hệ", () => {
    const offices = MUC_MENU_NHANH.find((m) => m.nhan === "Văn phòng gần tôi");
    expect(offices, "the offices tile lost its label").toBeDefined();
    expect(offices!.kieu === "man" && offices!.di).toEqual({ man: "contact", moc: MOC_TIM_VAN_PHONG });
  });
});

describe("F — the 'Chat Zalo' failure card can be closed, closes itself, and does not follow to the next screen", () => {
  function failureCard() {
    calls.stateValue = true;
    return NutChatOA();
  }

  it("has a 'Đóng' button sized by --tap-min (48px inside .app) that clears the card", () => {
    const close = buttons(failureCard()).find((b) => textIn(b.props.children) === CHAT_FAILURE_CLOSE_LABEL);
    expect(close, "the failure card has no close button").toBeDefined();
    expect(String(close!.props["className"])).toContain("tn__nut-phu");
    press(close!);
    expect(calls.setState).toHaveBeenCalledWith(false);
  });

  it(`clears itself after ${CHAT_FAILURE_VISIBLE_MS} ms, and the timer is cancelled if the card goes first`, () => {
    vi.useFakeTimers();
    failureCard();
    expect(calls.effects).toHaveLength(1);
    const cleanup = calls.effects[0]!();
    vi.advanceTimersByTime(CHAT_FAILURE_VISIBLE_MS - 1);
    expect(calls.setState).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(calls.setState).toHaveBeenCalledWith(false);

    calls.setState.mockClear();
    const cleanupAgain = calls.effects[0]!();
    (cleanupAgain as () => void)();
    vi.advanceTimersByTime(CHAT_FAILURE_VISIBLE_MS);
    expect(calls.setState, "a cancelled timer still fired").not.toHaveBeenCalled();
    expect(typeof cleanup).toBe("function");
  });

  it("no card → no timer is started", () => {
    vi.useFakeTimers();
    calls.stateValue = false;
    NutChatOA();
    expect(calls.effects[0]!()).toBeUndefined();
  });

  it("the shell keys the chat button by screen, so changing screen drops the card", () => {
    const shell = KhungApp({ man: "contact", onChonMan: () => {}, xaDaChon: null, children: null });
    const chat = elements(shell).find((el) => el.type === NutChatOA);
    expect(chat, "the shell no longer draws the chat button").toBeDefined();
    expect(chat!.key).toBe("contact");
  });
});
