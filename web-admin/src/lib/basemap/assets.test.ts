import { describe, expect, it } from "vitest";

import { BASEMAP_FILE, basemapAsset } from "./assets";

describe("basemapAsset — the exact allow-list behind /basemap/*", () => {
  it("serves the archive, the three font stacks and the four sprite files", () => {
    expect(basemapAsset([BASEMAP_FILE])?.key).toBe("vn-mainland.pmtiles");
    expect(basemapAsset(["fonts", "Noto Sans Regular", "7680-7935.pbf"])?.key).toBe(
      "fonts/Noto%20Sans%20Regular/7680-7935.pbf",
    );
    expect(basemapAsset(["fonts", "Noto Sans Italic", "65280-65535.pbf"])).not.toBeNull();
    for (const f of ["light.json", "light.png", "light@2x.json", "light@2x.png"]) {
      expect(basemapAsset(["sprites", f]), f).not.toBeNull();
    }
  });

  it("sets its own content type — never what storage says", () => {
    expect(basemapAsset([BASEMAP_FILE])?.contentType).toBe("application/octet-stream");
    expect(basemapAsset(["fonts", "Noto Sans Medium", "0-255.pbf"])?.contentType).toBe("application/x-protobuf");
    expect(basemapAsset(["sprites", "light.png"])?.contentType).toBe("image/png");
    expect(basemapAsset(["sprites", "light.json"])?.contentType).toBe("application/json");
  });

  it("the archive must be revalidated: a cached range of the previous file would corrupt tiles", () => {
    expect(basemapAsset([BASEMAP_FILE])?.cacheControl).toContain("no-cache");
  });

  it("refuses traversal and anything not listed, by exact comparison", () => {
    const refused: string[][] = [
      [],
      [""],
      [".."],
      ["..", BASEMAP_FILE],
      ["fonts", "..", "..", "secret"],
      ["fonts", "../Noto Sans Regular", "0-255.pbf"],
      ["fonts", "Noto Sans Regular", "../0-255.pbf"],
      ["fonts", "Noto Sans Regular/../../x", "0-255.pbf"],
      ["sprites", "../light.json"],
      ["sprites", "light.json", ""],
      ["other.pmtiles"],
      [BASEMAP_FILE, ""],
      ["VN-MAINLAND.PMTILES"],
      ["fonts", "Arial Regular", "0-255.pbf"],
      ["fonts", "Noto Sans Regular", "1-256.pbf"],
      ["fonts", "Noto Sans Regular", "0-511.pbf"],
      ["fonts", "Noto Sans Regular", "65536-65791.pbf"],
      ["fonts", "Noto Sans Regular", "0-255.pbf.html"],
      ["sprites", "light.svg"],
      ["sprites", "dark.json"],
      ["private", "petition.jpg"],
    ];
    for (const s of refused) expect(basemapAsset(s), JSON.stringify(s)).toBeNull();
  });
});
