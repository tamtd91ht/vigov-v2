// @vitest-environment jsdom
//
// jsdom for this file: it MOUNTS both pages' hook halves and reads the requests they send. The
// spec's one hard rule for /bao-cao — "/tong-quan và /bao-cao không được lệch số" (13 §10) — is a
// statement about what is ASKED, so it is tested on the wire, not on a pure function alone.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DashboardPage } from "@/features/dashboard/overview";
import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { ReportPage } from "./report-overview";

const COMMUNE = { displayName: "Xã Kiểm Thử", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" };

const PERMISSIONS = ["report.read", "task.read", "document.read", "feedback.read", "budget.read"];

vi.mock("@/lib/api/phien", () => ({
  layPhienHienTai: () => Promise.resolve({ ok: true, duLieu: { permissions: PERMISSIONS } }),
}));
vi.mock("@/features/mat-khau/bat-doi-mat-khau", () => ({ duongDanBatDoiMatKhau: () => null }));

const AT = new Date("2026-09-28T09:43:00Z"); // 16:43 in Viet Nam

let calls: string[] = [];
let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(AT);
  // Never answers: only what is ASKED matters here, and nothing renders a figure.
  vi.stubGlobal(
    "fetch",
    vi.fn((url: unknown) => {
      calls.push(String(url));
      return new Promise(() => {});
    }),
  );
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

async function mount(page: React.ReactNode): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  await act(async () => {
    // The commune configuration every page receives from the server (`app/tong-quan/page.tsx`);
    // Tổng quan reads it for its export files and refuses to render without it.
    r.render(
      <CauHinhXaProvider giaTri={COMMUNE}>
        <PhienProvider>{page}</PhienProvider>
      </CauHinhXaProvider>,
    );
  });
  await act(async () => {
    await Promise.resolve();
  });
  return host;
}

const SUMMARIES = ["/api/v1/task-summary?", "/api/v1/incoming-document-summary?", "/api/v1/citizen-report-summary?"];

const summaryCalls = () => calls.filter((c) => SUMMARIES.some((s) => c.startsWith(s))).sort();

function button(el: HTMLElement, text: string): HTMLButtonElement {
  const found = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === text);
  if (found === undefined) throw new Error(`no button "${text}"`);
  return found;
}

function setValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

describe("/bao-cao asks the servers what /tong-quan asks (spec 13 §10)", () => {
  it("default period: the identical six summary query strings, current and previous", async () => {
    await mount(<DashboardPage />);
    const overview = summaryCalls();
    expect(overview).toHaveLength(6);

    act(() => root?.unmount());
    host?.remove();
    calls = [];

    await mount(<ReportPage />);
    expect(summaryCalls()).toEqual(overview);
  });

  it("/bao-cao reads no overdue queue (it has no 'Cần xử lý ngay'), and reads the unit table once", async () => {
    await mount(<ReportPage />);
    expect(calls.some((c) => c.includes("overdue"))).toBe(false);
    const unit = calls.filter((c) => c.startsWith("/api/v1/task-unit-summary?"));
    expect(unit).toHaveLength(1);
    const q = new URLSearchParams(unit[0]!.split("?")[1]);
    expect(q.get("from")).toBe("2026-09-01T00:00:00+07:00");
    expect(q.get("to")).toBe("2026-09-28T16:43:01+07:00");
    expect(calls).toContain("/api/v1/org-units");
  });
});

describe("Tuỳ chọn — a refused range is never sent", () => {
  it("from after to: inline sentence, no request; a valid range asks [D1, D2+1)", async () => {
    const el = await mount(<ReportPage />);
    act(() => button(el, "Tuỳ chọn").click());
    const from = el.querySelector<HTMLInputElement>("#bao-cao-tu-ngay")!;
    const to = el.querySelector<HTMLInputElement>("#bao-cao-den-ngay")!;
    expect(from.value).toBe("2026-09-01");
    expect(to.value).toBe("2026-09-28");

    calls = [];
    act(() => {
      setValue(from, "2026-09-17");
      setValue(to, "2026-09-01");
    });
    act(() => button(el, "Xem").click());
    expect(el.querySelector('[role="alert"]')?.textContent).toBe("Từ ngày phải trước hoặc trùng Đến ngày.");
    expect(calls).toEqual([]);

    act(() => {
      setValue(from, "2026-09-01");
      setValue(to, "2026-09-17");
    });
    await act(async () => {
      button(el, "Xem").click();
    });
    const task = calls.filter((c) => c.startsWith("/api/v1/task-summary?")).map((c) => new URLSearchParams(c.split("?")[1]));
    expect(task.map((q) => [q.get("from"), q.get("to")]).sort()).toEqual([
      ["2026-08-15T00:00:00+07:00", "2026-09-01T00:00:00+07:00"],
      ["2026-09-01T00:00:00+07:00", "2026-09-18T00:00:00+07:00"],
    ]);
    expect(el.textContent).toContain("Kỳ tuỳ chọn: 1/9/2026 – 17/9/2026 (17 ngày)");
  });
});
