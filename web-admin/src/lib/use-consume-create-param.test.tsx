// @vitest-environment jsdom
//
// jsdom for this file: what it pins — an effect that runs once, the address bar after
// `history.replaceState`, a remount — are effects and history, none of which a string render can show.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { searchWithoutCreate, useConsumeCreateParam } from "./use-consume-create-param";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let host: HTMLDivElement;
let root: Root;

beforeEach(() => {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
});

afterEach(() => {
  act(() => root.unmount());
  host.remove();
  window.history.replaceState(null, "", "/");
});

function Harness({ requested, allowed, open }: { requested: boolean; allowed: boolean | null; open: () => void }) {
  useConsumeCreateParam(requested, allowed, open);
  return null;
}

function draw(requested: boolean, allowed: boolean | null, open: () => void): void {
  act(() => root.render(<Harness requested={requested} allowed={allowed} open={open} />));
}

describe("searchWithoutCreate", () => {
  it("drops `create` only, every other parameter kept in order", () => {
    expect(searchWithoutCreate("?create=1")).toBe("");
    expect(searchWithoutCreate("?task=NV19&create=1&status=moi-giao")).toBe("?task=NV19&status=moi-giao");
    expect(searchWithoutCreate("?tab=noi-dung")).toBe("?tab=noi-dung");
  });
});

describe("useConsumeCreateParam", () => {
  it("allowed: opens ONCE and drops `create`, keeping the other parameters and the hash", () => {
    window.history.replaceState(null, "", "/nhiem-vu?task=NV19&create=1#hang-cho");
    const open = vi.fn();
    draw(true, true, open);
    expect(open).toHaveBeenCalledTimes(1);
    expect(window.location.pathname).toBe("/nhiem-vu");
    expect(window.location.search).toBe("?task=NV19");
    expect(window.location.hash).toBe("#hang-cho");
    // Re-renders do not open it again.
    draw(true, true, open);
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("undecided (`null`): waits, then opens when the gate opens", () => {
    window.history.replaceState(null, "", "/nhiem-vu?create=1");
    const open = vi.fn();
    draw(true, null, open);
    expect(open).not.toHaveBeenCalled();
    expect(window.location.search).toBe("?create=1");
    draw(true, true, open);
    expect(open).toHaveBeenCalledTimes(1);
    expect(window.location.search).toBe("");
  });

  it("DENIED: never opens, and the parameter is dropped all the same", () => {
    window.history.replaceState(null, "", "/nhiem-vu?create=1");
    const open = vi.fn();
    draw(true, false, open);
    expect(open).not.toHaveBeenCalled();
    expect(window.location.search).toBe("");
    // A later grant on the same mount does not reopen a link already decided.
    draw(true, true, open);
    expect(open).not.toHaveBeenCalled();
  });

  it("not requested: nothing opens, the address is left alone", () => {
    window.history.replaceState(null, "", "/nhiem-vu?create=1");
    const replace = vi.spyOn(window.history, "replaceState");
    const open = vi.fn();
    draw(false, true, open);
    expect(open).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
    replace.mockRestore();
  });

  it("a remount with the same server prop (tab switch, router cache) opens nothing once the address is clean", () => {
    window.history.replaceState(null, "", "/van-ban?create=1");
    const open = vi.fn();
    draw(true, true, open);
    expect(open).toHaveBeenCalledTimes(1);
    act(() => root.unmount());
    root = createRoot(host);
    draw(true, true, open);
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("the address must carry exactly `create=1`: a repeated value opens nothing", () => {
    window.history.replaceState(null, "", "/van-ban?create=1&create=1");
    const open = vi.fn();
    draw(true, true, open);
    expect(open).not.toHaveBeenCalled();
  });
});
