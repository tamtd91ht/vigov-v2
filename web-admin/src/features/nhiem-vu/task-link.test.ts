import { describe, expect, it } from "vitest";

import { parseOpenTask, taskDetailHref } from "./task-link";

describe("task-link — /nhiem-vu?task=<code>", () => {
  it("builds the link with the code encoded", () => {
    expect(taskDetailHref("NV19")).toBe("/nhiem-vu?task=NV19");
    expect(taskDetailHref("NV 1/2")).toBe("/nhiem-vu?task=NV%201%2F2");
  });

  it("reads one trimmed code back", () => {
    expect(parseOpenTask({ task: " NV19 " })).toBe("NV19");
  });

  it("refuses absent, empty, repeated and over-long values — never guesses one", () => {
    expect(parseOpenTask({})).toBeNull();
    expect(parseOpenTask({ task: "  " })).toBeNull();
    expect(parseOpenTask({ task: ["NV1", "NV2"] })).toBeNull();
    expect(parseOpenTask({ task: "N".repeat(101) })).toBeNull();
  });
});
