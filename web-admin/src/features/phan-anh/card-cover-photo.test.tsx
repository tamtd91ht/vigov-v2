// @vitest-environment jsdom
//
// jsdom: the fallback is a state change on the image's `error` event, which a static render cannot fire.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it } from "vitest";

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { ThePhieu } from "./so-phan-anh";

const URL_A = "https://store.example.test/t_01J/scene/1.jpg?sig=a";
const URL_B = "https://store.example.test/t_01J/scene/1.jpg?sig=b";

function petition(url: string): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-xu-ly",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ.",
    address: "Tổ 6, thôn Hà Lam",
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: "2026-09-09T09:20:00Z",
    resolve_due: "2026-09-10T09:20:00Z",
    classify_due: "2026-09-09T11:20:00Z",
    unit: "01JBOPHAN",
    assignee: "",
    result: "",
    public: false,
    cover_photo: {
      id: "01JPHOTO",
      content_type: "image/jpeg",
      size_bytes: 1024,
      created_at: "2026-09-09T07:20:00Z",
      url,
      url_expires_at: "2026-09-10T02:15:00Z",
    },
  };
}

const NOW = new Date("2026-09-10T02:00:00Z");

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function render(p: petitions_phieuPhanAnhRa): HTMLDivElement {
  host ??= document.body.appendChild(document.createElement("div"));
  root ??= createRoot(host);
  const r = root;
  act(() => r.render(<ThePhieu phieu={p} bayGio={NOW} mo={() => {}} />));
  return host;
}

describe("list card cover photo — expired or broken link", () => {
  it("onError swaps the <img> for the placeholder tile", () => {
    const el = render(petition(URL_A));
    const img = el.querySelector("img");
    expect(img?.getAttribute("src")).toBe(URL_A);

    act(() => {
      img!.dispatchEvent(new Event("error"));
    });

    expect(el.querySelector("img")).toBeNull();
    // The tile (first child of the card button) is now the aria-hidden ImageOff placeholder.
    const tile = el.querySelector("button > span:first-child");
    expect(tile?.getAttribute("aria-hidden")).toBe("true");
    expect(tile?.querySelector("svg")).not.toBeNull();
  });

  it("a re-read list carrying a fresh link draws the image again", () => {
    const el = render(petition(URL_A));
    act(() => {
      el.querySelector("img")!.dispatchEvent(new Event("error"));
    });
    expect(el.querySelector("img")).toBeNull();

    render(petition(URL_B));
    expect(el.querySelector("img")?.getAttribute("src")).toBe(URL_B);
  });
});
