import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhoiThuongHieu } from "@/app/dang-nhap/khoi-thuong-hieu";
import { CauHinhXaProvider } from "@/components/cau-hinh-xa";

import { FormDangNhap } from "./form-dang-nhap";

/**
 * The login redesign (spec §8.4) is visual only. These pin what it must not have moved: the two
 * fields and their names, the single shared error region, the submit button, the absence of
 * `required` (one server sentence for every failure — see the block comment in the form), and the
 * commune name printed verbatim from runtime configuration.
 */
describe("FormDangNhap after the redesign", () => {
  const html = renderToStaticMarkup(<FormDangNhap tiepTuc={null} />);

  it("keeps the same two fields, names and autocomplete hints", () => {
    expect(html.match(/<input /g)).toHaveLength(2);
    const inputs = html.match(/<input [^>]*\/>/g) ?? [];
    for (const a of ['name="email"', 'type="email"', 'autoComplete="username"']) expect(inputs[0]).toContain(a);
    for (const a of ['name="password"', 'type="password"', 'autoComplete="current-password"']) expect(inputs[1]).toContain(a);
    expect(html).toContain(">Thư điện tử công vụ</label>");
    expect(html).toContain(">Mật khẩu</label>");
  });

  it("keeps noValidate, no `required`, one alert region, one submit button", () => {
    expect(html).toContain("noValidate");
    expect(html).not.toContain("required");
    expect(html.match(/role="alert"/g)).toHaveLength(1);
    expect(html.match(/<button /g)).toHaveLength(1);
    expect(html).toMatch(/<button type="submit" class="nut-chinh"[^>]*>.*Đăng nhập<\/button>/);
  });

  it("decorative icons are hidden from assistive tech", () => {
    const svgs = html.match(/<svg[^>]*>/g) ?? [];
    expect(svgs.length).toBeGreaterThanOrEqual(4);
    for (const s of svgs) expect(s).toContain('aria-hidden="true"');
  });
});

describe("KhoiThuongHieu", () => {
  const render = (parentAuthority: string, logoUrl = "") =>
    renderToStaticMarkup(
      <CauHinhXaProvider giaTri={{ displayName: "UBND xã Tân Phú", parentAuthority, logoUrl, webAdminBannerUrl: "" }}>
        <KhoiThuongHieu />
      </CauHinhXaProvider>,
    );

  it("no logo uploaded: the star tile, no image (ADR 0069 #7 — never a picture the commune did not issue)", () => {
    const html = render("Tỉnh Đồng Nai");
    expect(html).not.toContain("<img");
    expect(html).toContain('<div class="logo-vigov" aria-hidden="true">');
  });

  it("logo uploaded: the commune's image in the tile, decorative (the name is printed beside it)", () => {
    const url = "https://media.example.test/vigov-public/t_01JXA/logo-512.png";
    const html = render("Tỉnh Đồng Nai", url);
    expect(html).toContain('<div class="logo-vigov has-logo" aria-hidden="true">');
    expect(html.match(/<img /g)).toHaveLength(1);
    expect(html).toContain(`src="${url}"`);
    expect(html).toContain('alt=""');
    expect(html).toContain('class="commune-logo"');
  });

  it("prints the commune's name verbatim, once, where the product name used to be", () => {
    const html = render("Tỉnh Đồng Nai");
    expect(html).toContain('<p class="ten-san-pham">UBND xã Tân Phú</p>');
    expect(html).toContain('<p class="phu-de-san-pham">Hệ thống điều hành số</p>');
    expect(html.match(/UBND xã Tân Phú/g)).toHaveLength(1);
    expect(html).toContain('<p class="co-quan-cap-tren">Tỉnh Đồng Nai</p>');
    // The module tiles are illustration, not navigation: an anonymous visitor reaches no module.
    expect(html).not.toContain("<a ");
  });

  it("shows no product name a staff member can read (ADR 0068 §13)", () => {
    // Text only: the `logo-vigov` CSS class is an identifier and is meant to stay.
    const text = render("Tỉnh Đồng Nai").replace(/<[^>]*>/g, " ");
    expect(text).not.toMatch(/vigov/i);
  });

  it("drops the parent-authority block when the commune declares none", () => {
    const html = render("");
    expect(html).not.toContain("co-quan-cap-tren");
    expect(html).not.toContain("login-commune");
  });
});
