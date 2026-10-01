import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * WHAT THE SHELL HANDS THE COMMUNE'S OWN APP, PER BUILD — `App.tsx` `AppRieng` (owner 01/10/2026).
 *
 * The `--demo` build must NOT receive the location function: its exchange calls `getAccessToken` and
 * `getLocation`, and Zalo refuses both to an app it has not approved yet. Not injecting it is what removes
 * "Lấy vị trí hiện tại" from the send form (`CommuneSendScreen`: absent = no button). The normal build keeps
 * it. Measured on the REAL `AppRieng`, compiled once per value of `DEMO_BUILD` — the commune page component
 * is replaced only to read the props it is given.
 */

type Captured = { getSceneLocation?: unknown; openSession?: unknown };

async function propsGivenToCommunePage(demo: boolean): Promise<Captured> {
  vi.resetModules();
  vi.doMock("./lib/demo-build", async (importOriginal) => ({
    ...(await importOriginal<typeof import("./lib/demo-build")>()),
    DEMO_BUILD: demo,
  }));
  let captured: Captured | null = null;
  vi.doMock("./cong-dan", async (importOriginal) => ({
    ...(await importOriginal<typeof import("./cong-dan")>()),
    TrangXa: (props: Captured) => {
      captured = props;
      return null;
    },
  }));
  const { AppRieng } = await import("./App");
  renderToStaticMarkup(<AppRieng ten_mien="xa-thu.vigov.example" />);
  expect(captured, "AppRieng no longer renders the commune page — this measure measures nothing").not.toBeNull();
  return captured!;
}

afterEach(() => {
  vi.doUnmock("./lib/demo-build");
  vi.doUnmock("./cong-dan");
  vi.resetModules();
});

describe("AppRieng — the location control per build", () => {
  it("--demo build: no location function, so no “Lấy vị trí hiện tại”", async () => {
    const props = await propsGivenToCommunePage(true);
    expect(props.getSceneLocation).toBeUndefined();
    // The session opener is still there: only the location is withheld.
    expect(typeof props.openSession).toBe("function");
  });

  it("normal build: the location function is injected, unchanged", async () => {
    const props = await propsGivenToCommunePage(false);
    expect(typeof props.getSceneLocation).toBe("function");
    expect(typeof props.openSession).toBe("function");
  });
});
