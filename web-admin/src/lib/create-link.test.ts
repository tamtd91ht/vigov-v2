import { describe, expect, it } from "vitest";

import { OPEN_CREATE_PARAM, createHref, parseOpenCreate } from "./create-link";

describe("parseOpenCreate — `?create=1` and nothing else", () => {
  it("`1` asks for the dialog", () => {
    expect(parseOpenCreate({ create: "1" })).toBe(true);
  });

  it("any other value, a repeated parameter, or none: no dialog (fail closed)", () => {
    expect(parseOpenCreate({ create: "0" })).toBe(false);
    expect(parseOpenCreate({ create: "true" })).toBe(false);
    expect(parseOpenCreate({ create: "" })).toBe(false);
    expect(parseOpenCreate({ create: ["1", "1"] })).toBe(false);
    expect(parseOpenCreate({ create: undefined })).toBe(false);
    expect(parseOpenCreate({})).toBe(false);
  });

  it("other parameters are not its business", () => {
    expect(parseOpenCreate({ task: "NV19", create: "1" })).toBe(true);
    expect(parseOpenCreate({ task: "NV19" })).toBe(false);
  });
});

describe("createHref", () => {
  it("adds `?create=1` to a bare route", () => {
    expect(createHref("/nhiem-vu")).toBe("/nhiem-vu?create=1");
    expect(OPEN_CREATE_PARAM).toBe("create");
  });

  it("appends with `&` to a route that already carries a query", () => {
    expect(createHref("/mini-app?tab=noi-dung")).toBe("/mini-app?tab=noi-dung&create=1");
  });
});
