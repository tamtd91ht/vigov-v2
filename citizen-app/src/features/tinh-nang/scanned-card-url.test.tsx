import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { docMaQR, isOpenableWebLink } from "./danh-thiep";
import { TheDanhThiep } from "./ManDanhThiep";
import { DANH_THIEP } from "./noi-dung";

/**
 * F2 (security review 02/10/2026): a vCard `URL` field gets an "open" button only when it is an http(s)
 * absolute URL — the bare-link rule. Anything else is shown as text and never reaches the opener.
 */
const card = (url: string) => {
  const parsed = docMaQR(`BEGIN:VCARD\nFN:Trần Bình\nURL:${url}\nEND:VCARD`);
  if (parsed.loai !== "danh-thiep") throw new Error("sample vCard is broken");
  return parsed;
};

const buttons = (markup: string) => markup.split(DANH_THIEP.nut_mo_lien_ket).length - 1;

describe("vCard URL — only http(s) is offered to the opener", () => {
  const REFUSED = ["javascript:alert(1)", "intent://scan/#Intent;scheme=x;end", "data:text/html,hi", "/trang", "www.vidu.vn"];

  for (const url of REFUSED) {
    it(`${JSON.stringify(url)}: shown as text, no open button`, () => {
      expect(isOpenableWebLink(url)).toBe(false);
      const markup = renderToStaticMarkup(<TheDanhThiep thiep={card(url)} onGoi={() => {}} onMoLienKet={() => {}} />);
      expect(markup).toContain(DANH_THIEP.nhan_trang_web);
      expect(buttons(markup), "a non-http(s) vCard URL got an open button").toBe(0);
    });
  }

  for (const url of ["https://vidu.vn/a", "http://vidu.vn", "HTTPS://VIDU.VN"]) {
    it(`${url}: still has its open button`, () => {
      expect(isOpenableWebLink(url)).toBe(true);
      const markup = renderToStaticMarkup(<TheDanhThiep thiep={card(url)} onGoi={() => {}} onMoLienKet={() => {}} />);
      expect(markup).toContain(url);
      expect(buttons(markup)).toBe(1);
    });
  }

  it("the rule is the bare-link rule: a bare QR is a link exactly when a vCard URL is openable", () => {
    for (const url of [...REFUSED, "https://vidu.vn/a", "http://vidu.vn"]) {
      expect(docMaQR(url).loai === "lien-ket", url).toBe(isOpenableWebLink(url));
    }
  });
});
