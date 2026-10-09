import { readFileSync } from "node:fs";

import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import type { comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  BANNER_APP_PATHS,
  BANNER_ARTICLE_PATH_HINT,
  BANNER_PATH_NOT_TAPPABLE,
  bannerLinkTapWarning,
  ERR_LINK_TO_INVALID,
  isMiniAppBannerPath,
  validateTypeFields,
  FORM_TRONG,
} from "./nhan-noi-dung";
import { FormNoiDung } from "./so-noi-dung";

/**
 * Banner `link_to` suggestions (ADR 0067 §5): the in-app paths the citizen Mini App opens, offered as a
 * datalist, and a WARNING — never a refusal — for an in-app path it cannot open.
 */

describe("BANNER_APP_PATHS mirrors the Mini App's closed table", () => {
  it("same keys as `BANNER_ROUTES` in citizen-app TrangXa.tsx — a drift here is a banner nobody can tap", () => {
    const src = readFileSync(
      new URL("../../../../citizen-app/src/cong-dan/man/TrangXa.tsx", import.meta.url),
      "utf8",
    );
    const table = /const BANNER_ROUTES[^=]*=\s*\{([\s\S]*?)\n\};/.exec(src);
    expect(table, "BANNER_ROUTES not found — the table moved; update this test and BANNER_APP_PATHS").not.toBeNull();
    const keys = [...(table?.[1] ?? "").matchAll(/"(\/[^"]*)"\s*:/g)].map((m) => m[1]);
    expect(keys.length).toBeGreaterThan(0);
    expect(BANNER_APP_PATHS.map((p) => p.path)).toEqual(keys);
    // The article form is the one dynamic path `bannerScreen` adds.
    expect(src).toContain("/^\\/tin-tuc\\/([^/]+)$/");
  });
});

describe("isMiniAppBannerPath — mirror of `bannerScreen`", () => {
  it("every listed path, with or without one trailing slash, and /tin-tuc/<id>", () => {
    for (const { path } of BANNER_APP_PATHS) {
      expect(isMiniAppBannerPath(path), path).toBe(true);
      expect(isMiniAppBannerPath(`${path}/`), `${path}/`).toBe(true);
    }
    expect(isMiniAppBannerPath("/tin-tuc/01JND1")).toBe(true);
    expect(isMiniAppBannerPath("/tin-tuc/01JND1/")).toBe(true);
  });

  it("unknown paths, query / fragment, //host, deeper article paths, empty or bad %-escape ⇒ not openable", () => {
    for (const p of [
      "/lich-tiep-dan",
      "/tin-tuc?x=1",
      "/video#a",
      "//evil.example",
      "/tin-tuc/a/b",
      "/tin-tuc/%20",
      "/tin-tuc/%E0%A4%A",
      "tin-tuc",
    ]) {
      expect(isMiniAppBannerPath(p), p).toBe(false);
    }
  });
});

describe("bannerLinkTapWarning — warns, never blocks", () => {
  it("no warning for empty, https://, or a path the Mini App opens", () => {
    expect(bannerLinkTapWarning("")).toBeNull();
    expect(bannerLinkTapWarning("https://dichvucong.gov.vn")).toBeNull();
    expect(bannerLinkTapWarning("/su-kien")).toBeNull();
    expect(bannerLinkTapWarning(" /tin-tuc/01JND1 ")).toBeNull();
  });

  it("a valid in-app path the Mini App has no screen for ⇒ the warning, and Lưu is NOT held", () => {
    expect(bannerLinkTapWarning("/lich-tiep-dan")).toBe(BANNER_PATH_NOT_TAPPABLE);
    const gt = { ...FORM_TRONG, type: "banner", title: "Banner", cover_image_file_id: "01JCOVER1", link_to: "/lich-tiep-dan" };
    expect(validateTypeFields(gt)).toBeNull();
  });

  it("an INVALID value keeps its refusal and gets no second sentence", () => {
    expect(bannerLinkTapWarning("//evil.example")).toBeNull();
    const gt = { ...FORM_TRONG, type: "banner", title: "Banner", cover_image_file_id: "01JCOVER1", link_to: "//evil.example" };
    expect(validateTypeFields(gt)).toBe(ERR_LINK_TO_INVALID);
  });
});

describe("the banner box on the form", () => {
  const banner: comms_noiDungRa = {
    id: "01JBN1",
    type: "banner",
    category_id: "",
    title: "Ngày hội chuyển đổi số",
    summary: "",
    image_url: "",
    has_image: true,
    status: "dang-hien",
    source: "soan-tay",
    source_url: "",
    hand_edited: false,
    published_on: "2026-10-01",
    author_code: "CB-00123",
    created_at: "2026-10-01T02:00:00Z",
    updated_at: "2026-10-01T02:00:00Z",
    cover_image_file_id: "01JCOVER1",
    link_to: "/lich-tiep-dan",
    display_order: 1,
  } as comms_noiDungRa;

  function render(row: comms_noiDungRa): string {
    return renderToStaticMarkup(
      createElement(FormNoiDung, {
        tieuDeForm: "Sửa",
        moTa: "",
        giaTriDau: {
          ...FORM_TRONG,
          type: row.type,
          title: row.title,
          cover_image_file_id: row.cover_image_file_id ?? "",
          link_to: row.link_to ?? "",
          display_order: String(row.display_order ?? ""),
        },
        hang: row,
        danhMuc: [],
        dangGui: false,
        loi: null,
        huy: () => {},
        luu: () => {},
      }),
    );
  }

  it("offers every Mini App path in a datalist bound to the input", () => {
    const html = render(banner);
    expect(html).toContain('list="lien-ket-banner-duong-app"');
    expect(html).toContain('<datalist id="lien-ket-banner-duong-app">');
    for (const { path } of BANNER_APP_PATHS) expect(html).toContain(`value="${path}"`);
  });

  it("the prototype's label and placeholder only — no hint paragraph, no not-tappable warning (09/10/2026)", () => {
    const html = render(banner);
    expect(html).toContain("Bấm vào thì mở gì");
    expect(html).toContain('placeholder="Ví dụ: /tin-tuc — để trống thì ảnh chỉ để xem"');
    expect(html).not.toContain(BANNER_ARTICLE_PATH_HINT.replace(/</g, "&lt;").replace(/>/g, "&gt;"));
    expect(html).not.toContain('id="lien-ket-banner-goi-y"');
    expect(html).not.toContain('id="lien-ket-banner-canh-bao"');
    expect(html).not.toContain("Mini App chưa có màn nào cho đường này");
  });
});
