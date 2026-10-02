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
  it("prints the commune's name verbatim, from configuration, never upper-cased in data", () => {
    const html = renderToStaticMarkup(
      <CauHinhXaProvider giaTri={{ displayName: "UBND xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai" }}>
        <KhoiThuongHieu />
      </CauHinhXaProvider>,
    );
    expect(html).toContain('<p class="ten-xa">UBND xã Tân Phú</p>');
    expect(html).toContain('<p class="co-quan-cap-tren">Tỉnh Đồng Nai</p>');
    // The module tiles are illustration, not navigation: an anonymous visitor reaches no module.
    expect(html).not.toContain("<a ");
  });
});
