import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { PhieuCuaToiTomTat } from "../api/hop-dong-phan-anh";

import { PetitionCard, shortPetitionCode } from "./PhanAnhAppXa";

/**
 * Customer sheet 12/10/2026 row 9 ("ID phản ánh quá dài → rút ngắn; ngày tạo ngang hàng ID, canh phải"); owner:
 * shorten on the LIST CARD only. The issued code is untouched (rule 4 #4); detail, post-submit and lookup keep it whole.
 */

describe("shortPetitionCode", () => {
  it("prefix + last group: PA-4K7M-92XR-BTVD → PA-…-BTVD", () => {
    expect(shortPetitionCode("PA-4K7M-92XR-BTVD")).toBe("PA-…-BTVD");
    expect(shortPetitionCode("PA-THU-0001")).toBe("PA-…-0001");
  });

  it("anything else is shown UNCHANGED — no groups, two groups, empty groups, empty", () => {
    for (const code of ["PA7K2QX9M4TD", "PA-1", "PA--BTVD", "-4K7M-BTVD", "PA-4K7M-", "", "PA-4K7M--BTVD"]) {
      expect(shortPetitionCode(code), code).toBe(code);
    }
  });
});

describe("the list card", () => {
  const ROW: PhieuCuaToiTomTat = {
    ma_tra_cuu: "PA-4K7M-92XR-BTVD",
    trang_thai: "da-tiep-nhan",
    linh_vuc: "moi-truong",
    nhan_linh_vuc: "Môi trường",
    trich_noi_dung: "Rác ở đầu ngõ",
    goc_dem_han: "2026-09-28T01:00:00Z",
    han_tiep_nhan: null,
    han_xu_ly_xong: null,
    rating: null,
    rated_at: null,
  };
  const card = renderToStaticMarkup(createElement(PetitionCard, { petition: ROW, onOpen: () => {} }));

  it("draws the short code, keeps the FULL one for a screen reader and the long-press title", () => {
    expect(card).toContain(
      '<strong class="xa-petition-card__code" title="#PA-4K7M-92XR-BTVD"><span aria-hidden="true">#PA-…-BTVD</span><span class="xa-sr-only">#PA-4K7M-92XR-BTVD</span></strong>',
    );
  });

  it("the day sits in the same head row, after the code (right-aligned by `.xa-petition-card__head`)", () => {
    expect(card).toMatch(/<span class="xa-petition-card__head"><strong[^>]*>[\s\S]*?<\/strong><span class="xa-phu">28\/9\/2026<\/span><\/span>/);
  });

  it("the stylesheet keeps the row on one line: nowrap, space-between, the day never shrinks", async () => {
    // Read from disk, not imported — the reason and the variable specifier are `accessibility.test.ts:8-19`'s.
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    const head = css.match(/\.xa-petition-card__head \{([^}]*)\}/)![1]!;
    expect(head).toContain("flex-wrap: nowrap;");
    expect(head).toContain("justify-content: space-between;");
    const day = css.match(/\.xa-petition-card__head \.xa-phu \{([^}]*)\}/)![1]!;
    expect(day).toContain("flex-shrink: 0;");
    expect(day).toContain("white-space: nowrap;");
  });
});
