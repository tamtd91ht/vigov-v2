/**
 * "KẾT NỐI VỚI ViHAT" — the one card holding every ask of the phone number (owner 12/10/2026, Zalo resubmission).
 *
 * What is pinned, and why:
 *   · each action's purpose sentence comes BEFORE its button in the markup — the button is the only way to reach
 *     Zalo's dialog, so "before the button" is "before the dialog" (Mini App policy 3.3.4);
 *   · a held session replaces the login ask with one sentence, and that sentence carries no number;
 *   · the login anchor survives the connected state ("Đăng nhập lại" elsewhere scrolls to it).
 *
 * Rendered with `react-dom/server`: no effect and no handler runs, so nothing here reaches Zalo or a server.
 */
import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NhaCungCapPhien } from "../dang-nhap/kho-phien"; // vi-name-ok: existing export
import type { Phien } from "../dang-nhap/hop-dong"; // vi-name-ok: existing type
import { noiDung } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export
import { CHAT_SPECIALIST, CONNECT_VIHAT, SMS_OFFERS } from "../yeu-cau/noi-dung";

import { ConnectWithVihat } from "./ConnectWithVihat";
import { ContactScreen } from "./ContactScreen";
import { MOC_DANG_NHAP } from "./dieu-huong";

const render = (node: ReactElement) => renderToStaticMarkup(node);
const textOf = (markup: string) => markup.replace(/<[^>]+>/g, " ").replace(/&amp;/g, "&").replace(/\s+/g, " ");

const SESSION: Phien = { token: "phieu-thu", het_han: "2026-10-14T03:00:00Z" }; // vi-name-ok: existing type

const loggedOut = render(<ConnectWithVihat />);
const loggedIn = render(
  <NhaCungCapPhien phien_ban_dau={SESSION}>
    <ConnectWithVihat />
  </NhaCungCapPhien>,
);

/** Position of a button whose whole label is `label`, or -1. */
const buttonAt = (markup: string, label: string) => markup.search(new RegExp(`<button[^>]*>${label}</button>`));

const ACTIONS = [
  { name: "login", purpose: noiDung("dang-nhap").vi_sao, button: noiDung("dang-nhap").nut },
  { name: "chat", purpose: CHAT_SPECIALIST.why, button: CHAT_SPECIALIST.button },
  { name: "sms", purpose: SMS_OFFERS.consent, button: SMS_OFFERS.subscribe },
] as const;

describe("one card, three separate actions, each saying its purpose before its button", () => {
  it("one h2 for the card, one h3 per action — three actions, not three cards", () => {
    expect(textOf(loggedOut)).toContain(CONNECT_VIHAT.title);
    expect(loggedOut.match(/<h2\b/g) ?? []).toHaveLength(1);
    expect(loggedOut.match(/<h3\b/g) ?? []).toHaveLength(3);
    for (const title of [noiDung("dang-nhap").tieu_de, CHAT_SPECIALIST.title, SMS_OFFERS.title]) {
      expect(textOf(loggedOut), title).toContain(title);
    }
  });

  for (const action of ACTIONS) {
    it(`${action.name}: the purpose is on screen before the button that opens Zalo's dialog`, () => {
      const purpose = loggedOut.indexOf(action.purpose);
      const button = buttonAt(loggedOut, action.button);
      expect(purpose, "purpose sentence missing").toBeGreaterThan(-1);
      expect(button, "button missing").toBeGreaterThan(-1);
      expect(purpose, "the button comes before its purpose").toBeLessThan(button);
    });
  }

  it("the actions come in the order of the card: login, chat, SMS — each purpose after the previous button", () => {
    const login = buttonAt(loggedOut, ACTIONS[0].button);
    const chatPurpose = loggedOut.indexOf(ACTIONS[1].purpose);
    const chat = buttonAt(loggedOut, ACTIONS[1].button);
    const smsPurpose = loggedOut.indexOf(ACTIONS[2].purpose);
    expect(login).toBeLessThan(chatPurpose);
    expect(chat).toBeLessThan(smsPurpose);
  });

  it("no input of any kind — every action is one tap", () => {
    expect(loggedOut).not.toMatch(/<(form|input|textarea|select)[\s/>]/);
  });
});

describe("a held session: the card says it is connected instead of asking again", () => {
  it("logged out: the login button, no connected sentence", () => {
    expect(buttonAt(loggedOut, noiDung("dang-nhap").nut)).toBeGreaterThan(-1);
    expect(textOf(loggedOut)).not.toContain(CONNECT_VIHAT.connected);
  });

  it("logged in: the connected sentence replaces the login ask; chat and SMS stay", () => {
    const t = textOf(loggedIn);
    expect(t).toContain(CONNECT_VIHAT.connected);
    expect(buttonAt(loggedIn, noiDung("dang-nhap").nut), "the login ask is still shown").toBe(-1);
    expect(t, "the login purpose is still shown").not.toContain(noiDung("dang-nhap").vi_sao);
    expect(buttonAt(loggedIn, CHAT_SPECIALIST.button)).toBeGreaterThan(-1);
    expect(t).toContain(SMS_OFFERS.consent);
  });

  it("the connected sentence carries no number, masked or not — the app never receives it", () => {
    expect(CONNECT_VIHAT.connected).not.toMatch(/\d|\*/);
    expect(textOf(loggedIn)).not.toMatch(/\d{2,}[*•x]+\d*|\*{2,}/);
  });

  it("a way to log in again stays (an expired session is not cleared by anything) and it is a plain button", () => {
    expect(buttonAt(loggedIn, CONNECT_VIHAT.relogin)).toBeGreaterThan(-1);
  });

  it("the login anchor exists in both states, so 'Đăng nhập lại' on other screens lands on it", () => {
    expect(loggedOut).toContain(`id="${MOC_DANG_NHAP}"`);
    expect(loggedIn).toContain(`id="${MOC_DANG_NHAP}"`);
    for (const markup of [loggedOut, loggedIn]) expect(markup.split(`id="${MOC_DANG_NHAP}"`)).toHaveLength(2);
  });
});

describe("Liên hệ carries the card once, and no second ask of the number", () => {
  it("each action appears exactly once on the screen", () => {
    const contact = render(<ContactScreen />);
    for (const action of ACTIONS) {
      expect(contact.split(action.purpose), action.name).toHaveLength(2);
    }
    expect(contact.split(`>${CONNECT_VIHAT.title}<`)).toHaveLength(2);
  });
});
