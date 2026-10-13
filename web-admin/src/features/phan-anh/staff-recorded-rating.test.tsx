// @vitest-environment jsdom
//
// jsdom for this file: the form is clicked, typed into and submitted. The drawer-level gate is checked on
// a static render of `ChiTietPhieu`, which jsdom serves too.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { congThaoTac } from "./nhan-phieu";
import { SECTION_CARD } from "./petition-ui";
import { ChiTietPhieu } from "./so-phan-anh";
import {
  STAFF_RATING_CITIZEN_RATED,
  STAFF_RATING_HINT,
  STAFF_RATING_SUBMIT,
  STAFF_RATING_TITLE,
  staffRatingByline,
  StaffRecordedRatingSection,
  type StaffRatingApi,
} from "./staff-recorded-rating";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

async function flush(times = 6) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function render(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function petition(change: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "da-xu-ly",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ.",
    address: "Tổ 6, thôn Hà Lam",
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-10-09T07:20:00Z",
    booked_at: "2026-10-09T07:21:00Z",
    acknowledge_due: null,
    resolve_due: null,
    classify_due: null,
    unit: "",
    assignee: "",
    result: "",
    public: false,
    ...change,
  };
}

const RECORD = {
  stars: 4,
  channel: "dien-thoai",
  note: "Dân gọi điện báo đã sạch.",
  source: "staff_recorded",
  recorded_by: "CB-00123",
  recorded_at: "2026-10-11T03:00:00Z",
};

function drawer(permissions: readonly string[], withWritePath = true, p = petition()): string {
  return renderToStaticMarkup(
    <ChiTietPhieu
      phieu={p}
      bayGio={new Date("2026-10-11T04:00:00Z")}
      cong={congThaoTac(false, false, permissions.includes("feedback.resolve"))}
      tenBoPhan={new Map()}
      boPhan={[]}
      danhBa={null}
      dangGui={false}
      loiGhi={null}
      permissions={permissions}
      onStaffRatingRecorded={withWritePath ? () => {} : undefined}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
    />,
  );
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement | undefined {
  return [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent?.trim() === text);
}

function click(el: HTMLElement | null | undefined): void {
  act(() => el?.click());
}

describe("the drawer's gate — `feedback.resolve` (UX; the route checks the same key)", () => {
  it("DENIED: an account with `feedback.read` only sees no block and no form", () => {
    const html = drawer(["feedback.read"]);
    expect(html).not.toContain(STAFF_RATING_TITLE);
    expect(html).not.toContain(STAFF_RATING_SUBMIT);
  });

  it("DENIED: even an existing staff record is not drawn without the key", () => {
    const html = drawer(["feedback.read"], true, petition({ staff_recorded_rating: RECORD }));
    expect(html).not.toContain(STAFF_RATING_TITLE);
    expect(html).not.toContain("CB-00123");
  });

  it("no write path → not drawn, whatever the keys", () => {
    expect(drawer(["feedback.read", "feedback.resolve"], false)).not.toContain(STAFF_RATING_TITLE);
  });

  it("ALLOWED: with `feedback.resolve` the block and its form are there, no '?' placeholder", () => {
    const html = drawer(["feedback.read", "feedback.resolve"]);
    expect(html).toContain(STAFF_RATING_TITLE);
    expect(html).toContain(STAFF_RATING_SUBMIT);
    expect(html).not.toContain("Ghi nhận đánh giá của người dân — tính năng đang phát triển");
  });
});

describe("StaffRecordedRatingSection", () => {
  // Sheet web-admin-131026: the form sat flush against the drawer edge; it is in the section card now.
  it("the form sits inside the section card, as the other drawer sections do", () => {
    const el = render(<StaffRecordedRatingSection petition={petition()} onRecorded={() => {}} />);
    expect(el.querySelector("section > form")?.className).toContain(SECTION_CARD);
  });

  it("never promises that a low rating reopens the petition", () => {
    const el = render(<StaffRecordedRatingSection petition={petition()} onRecorded={() => {}} />);
    expect(el.textContent).toContain(STAFF_RATING_HINT);
    expect(el.textContent).toContain("không tự mở lại phiếu");
    expect(el.textContent).not.toContain("sẽ tự mở lại");
  });

  it("submits stars, channel and the trimmed note, then hands the 200 body up", async () => {
    const recorded = petition({ staff_recorded_rating: RECORD });
    const api: StaffRatingApi = { record: vi.fn(async () => ({ ok: true as const, duLieu: recorded })) };
    const onRecorded = vi.fn();
    const el = render(<StaffRecordedRatingSection petition={petition()} onRecorded={onRecorded} api={api} />);

    const submit = buttonByText(el, STAFF_RATING_SUBMIT);
    expect(submit?.disabled).toBe(true); // no stars, no channel yet — the channel is never defaulted
    click(buttonByText(el, "4 ★"));
    expect(submit?.disabled).toBe(true);
    click(el.querySelector<HTMLInputElement>('input[value="dien-thoai"]'));
    expect(submit?.disabled).toBe(false);

    act(() => submit?.form?.requestSubmit());
    await flush();
    expect(api.record).toHaveBeenCalledWith("PA-2026-0021", 4, "dien-thoai", "");
    expect(onRecorded).toHaveBeenCalledWith(recorded);
  });

  it.each([
    ["citizen_rated", "Người dân đã tự đánh giá phiếu này nên không ghi nhận đánh giá thay được nữa."],
    ["staff_rating_exists", "Phiếu này đã có đánh giá do cán bộ ghi nhận."],
  ])("409 %s: the server's sentence, verbatim, and nothing handed up", async (_code, sentence) => {
    const refused: KetQua<petitions_phieuPhanAnhRa> = { ok: false, thongBao: sentence };
    const api: StaffRatingApi = { record: vi.fn(async () => refused) };
    const onRecorded = vi.fn();
    const el = render(<StaffRecordedRatingSection petition={petition()} onRecorded={onRecorded} api={api} />);
    click(buttonByText(el, "2 ★"));
    click(el.querySelector<HTMLInputElement>('input[value="tai-quay"]'));
    act(() => buttonByText(el, STAFF_RATING_SUBMIT)?.form?.requestSubmit());
    await flush();
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(onRecorded).not.toHaveBeenCalled();
  });

  it("the citizen rated: their rating, and NO form", () => {
    const el = render(<StaffRecordedRatingSection petition={petition({ rating: 5 })} onRecorded={() => {}} />);
    expect(el.querySelector("form")).toBeNull();
    expect(el.textContent).toContain("5/5");
    expect(el.textContent).toContain(STAFF_RATING_CITIZEN_RATED);
  });

  it("a staff record exists: read-only, with channel and staff code", () => {
    const el = render(
      <StaffRecordedRatingSection petition={petition({ staff_recorded_rating: RECORD })} onRecorded={() => {}} />,
    );
    expect(el.querySelector("form")).toBeNull();
    expect(el.textContent).toContain("Cán bộ ghi nhận qua điện thoại — CB-00123");
    expect(el.textContent).toContain("4/5");
    expect(el.textContent).toContain(RECORD.note);
  });

  it("byline: an unknown channel is shown as sent, never guessed", () => {
    expect(staffRatingByline("tai-quay", "CB-1")).toBe("Cán bộ ghi nhận qua quầy tiếp nhận — CB-1");
    expect(staffRatingByline("khac", "CB-1")).toBe("Cán bộ ghi nhận qua khac — CB-1");
  });
});
