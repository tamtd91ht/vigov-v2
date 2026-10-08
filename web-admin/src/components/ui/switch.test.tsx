import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Switch } from "./switch";

describe("Switch", () => {
  it("is a native button with the ARIA switch role and its state in aria-checked", () => {
    const on = renderToStaticMarkup(<Switch checked aria-label="Bật" />);
    expect(on).toMatch(/^<button /);
    expect(on).toContain('type="button" role="switch" aria-checked="true"');
    expect(on).toContain("bg-primary");
    const off = renderToStaticMarkup(<Switch checked={false} aria-label="Tắt" />);
    expect(off).toContain('aria-checked="false"');
    expect(off).toContain("bg-input");
  });

  it("a caller cannot turn it into a submit button inside a form", () => {
    // `type` is fixed AFTER the spread: a switch inside a form must never submit it.
    const html = renderToStaticMarkup(<Switch checked={false} aria-label="x" {...({ type: "submit" } as object)} />);
    expect(html).toContain('type="button"');
    expect(html).not.toContain('type="submit"');
  });
});
