import { afterEach, describe, expect, it, vi } from "vitest";

// `server-only` throws outside a React Server build by design; in Node tests it is inert.
vi.mock("server-only", () => ({}));

import { MAP_STYLE_URL_VAR, mapStyleUrl } from "./map-style";

afterEach(() => vi.restoreAllMocks());

describe("mapStyleUrl — server-side, no default, no fallback host (ADR 0072 H1)", () => {
  it("the variable name is the documented one, never NEXT_PUBLIC_", () => {
    expect(MAP_STYLE_URL_VAR).toBe("MAP_STYLE_URL");
  });

  it("set → that URL", () => {
    expect(mapStyleUrl({ MAP_STYLE_URL: " https://tiles.openfreemap.org/styles/liberty " })).toBe(
      "https://tiles.openfreemap.org/styles/liberty",
    );
  });

  it("unset or blank → null (no basemap), never another host", () => {
    expect(mapStyleUrl({})).toBeNull();
    expect(mapStyleUrl({ MAP_STYLE_URL: "  " })).toBeNull();
  });

  it("http, credentials or garbage → null, and the log names the VARIABLE, never the value", () => {
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(mapStyleUrl({ MAP_STYLE_URL: "http://tiles.example.test/style" })).toBeNull();
    expect(mapStyleUrl({ MAP_STYLE_URL: "https://user:secret@tiles.example.test/style" })).toBeNull();
    expect(mapStyleUrl({ MAP_STYLE_URL: "not a url" })).toBeNull();
    const logged = log.mock.calls.flat().join(" ");
    expect(logged).toContain("MAP_STYLE_URL");
    expect(logged).not.toContain("secret");
    expect(logged).not.toContain("tiles.example.test");
  });
});
