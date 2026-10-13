// @vitest-environment jsdom
//
// jsdom for this file: the candidates are READ on mount, and the merge / unmerge acts are dialogs that
// are typed into and submitted. A markup string shows neither.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { petitions_duplicateCandidatesOut, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  DUPLICATES_TITLE,
  DUPLICATES_TRUNCATED,
  MERGED_CHILDREN_LABEL,
  MERGED_INTO_UNKNOWN,
  mergeActionLabel,
  MERGE_REASON_ID,
  PetitionMergeSection,
  SHOW_MAIN_PETITION,
  UNMERGE_ACTION,
  UNMERGE_CLOSED,
  UNMERGE_REASON_ID,
  type MergeApi,
} from "./petition-merge";
import { SECTION_CARD } from "./petition-ui";

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
    code: "PA-2",
    channel: "zalo-mini-app",
    status: "dang-xu-ly",
    field: "giao-thong",
    field_label: "Giao thông",
    content: "Ổ gà trước cổng trường.",
    address: "Thôn Hà Lam",
    lat: 15.5,
    lng: 108.2,
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-10-08T02:00:00Z",
    booked_at: "2026-10-08T02:00:00Z",
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

const CANDIDATE = petition({ code: "PA-1", content: "Đường thủng một hố lớn.", reporter_name: "Trần T. B." });

function api(
  list: KetQua<petitions_duplicateCandidatesOut> = {
    ok: true,
    duLieu: { items: [CANDIDATE], radius_meters: 50, window_days: 7, truncated: false },
  },
  change: Partial<MergeApi> = {},
): MergeApi {
  return {
    list: vi.fn(async () => list),
    merge: vi.fn(async () => ({ ok: true as const, duLieu: petition({ merged_into: "PA-1" }) })),
    unmerge: vi.fn(async () => ({ ok: true as const, duLieu: petition() })),
    ...change,
  };
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement | undefined {
  return [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent?.trim() === text);
}

function type(el: HTMLTextAreaElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set;
  act(() => {
    setter?.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

/** The section's `<h3>`, or null when nothing at all is drawn. */
function heading(el: ParentNode): HTMLHeadingElement | null {
  return el.querySelector<HTMLHeadingElement>("section > h3");
}

// Sheet web-admin-131026: "Cụm ... chỉ được hiển thị khi phiếu này trùng với phiếu khác". The prototype
// draws the section only when `duplicates.length > 0` (`FeedbackDetailDrawer.tsx:408`).
describe("the section is drawn ONLY when there is a duplicate or a link", () => {
  it("open petition, search found nothing: nothing drawn — no heading, no sentence", async () => {
    const a = api({ ok: true, duLieu: { items: [], radius_meters: 80, window_days: 5, truncated: false } });
    const el = render(<PetitionMergeSection petition={petition()} mayMerge api={a} />);
    await flush();
    expect(a.list).toHaveBeenCalledWith("PA-2");
    expect(el.innerHTML).toBe("");
  });

  it("while the search runs: nothing drawn", async () => {
    const a = api(undefined, { list: vi.fn(() => new Promise<never>(() => {})) });
    const el = render(<PetitionMergeSection petition={petition()} mayMerge api={a} />);
    await flush();
    expect(el.innerHTML).toBe("");
  });

  it("no location: nothing drawn, and the search is never called", async () => {
    const a = api();
    const el = render(<PetitionMergeSection petition={petition({ lat: null, lng: null })} mayMerge api={a} />);
    await flush();
    expect(el.innerHTML).toBe("");
    expect(a.list).not.toHaveBeenCalled();
  });

  it("a refused search: nothing drawn (the section only ever announces duplicates)", async () => {
    const el = render(
      <PetitionMergeSection petition={petition()} mayMerge api={api({ ok: false, thongBao: "Máy chủ bận." })} />,
    );
    await flush();
    expect(el.innerHTML).toBe("");
  });

  it.each([
    ["closed", { status: "da-dong" }],
    ["`can-bo`", { field: "can-bo" }],
  ])("%s, not linked: nothing drawn, no search", async (_, change) => {
    const a = api();
    const el = render(<PetitionMergeSection petition={petition(change)} mayMerge api={a} />);
    await flush();
    expect(el.innerHTML).toBe("");
    expect(a.list).not.toHaveBeenCalled();
  });

  it("candidates found: the heading, and the body inside the drawer's card (sheet: padding)", async () => {
    const el = render(<PetitionMergeSection petition={petition()} mayMerge={false} api={api()} />);
    await flush();
    expect(heading(el)?.textContent).toBe(DUPLICATES_TITLE);
    expect(heading(el)?.nextElementSibling?.className).toContain(SECTION_CARD);
  });

  it.each([
    ["merged (even closed)", { status: "da-dong", merged_at: "2026-10-08T03:00:00Z" }],
    ["a main petition (even closed)", { status: "da-dong", merged_petitions: ["PA-3"] }],
  ])("%s: drawn without any search result", async (_, change) => {
    const el = render(<PetitionMergeSection petition={petition(change)} mayMerge={false} api={api()} />);
    await flush();
    expect(heading(el)?.textContent).toBe(DUPLICATES_TITLE);
  });
});

describe("candidates", () => {
  it("lists each candidate: code as a link to it, field, masked sender, content", async () => {
    const a = api();
    const el = render(<PetitionMergeSection petition={petition()} mayMerge={false} api={a} />);
    await flush();
    expect(a.list).toHaveBeenCalledWith("PA-2");
    const link = el.querySelector<HTMLAnchorElement>('a[href="/phan-anh?id=PA-1"]');
    expect(link?.textContent).toBe("PA-1");
    expect(el.textContent).toContain("Giao thông");
    expect(el.textContent).toContain("Đường thủng một hố lớn.");
    expect(el.textContent).toContain("Trần T. B. · 09****0000");
  });

  it("DENIED: without `feedback.classify` there is no merge action", async () => {
    const el = render(<PetitionMergeSection petition={petition()} mayMerge={false} onChanged={() => {}} api={api()} />);
    await flush();
    expect(buttonByText(el, mergeActionLabel("PA-1"))).toBeUndefined();
    expect(el.querySelector("button")).toBeNull();
  });

  it("no write path (no `onChanged`): no merge action even with the key", async () => {
    const el = render(<PetitionMergeSection petition={petition()} mayMerge api={api()} />);
    await flush();
    expect(buttonByText(el, mergeActionLabel("PA-1"))).toBeUndefined();
  });

  it("clicking a code opens that petition in place", async () => {
    const onOpen = vi.fn();
    const el = render(<PetitionMergeSection petition={petition()} mayMerge={false} onOpen={onOpen} api={api()} />);
    await flush();
    act(() => el.querySelector<HTMLAnchorElement>('a[href="/phan-anh?id=PA-1"]')!.click());
    expect(onOpen).toHaveBeenCalledWith("PA-1");
  });

  it("truncated: a short note under the candidates", async () => {
    const many = render(
      <PetitionMergeSection
        petition={petition()}
        mayMerge
        api={api({ ok: true, duLieu: { items: [CANDIDATE], radius_meters: 50, window_days: 7, truncated: true } })}
      />,
    );
    await flush();
    expect(many.textContent).toContain(DUPLICATES_TRUNCATED);
  });
});

describe("merge dialog", () => {
  async function openDialog(a: MergeApi, onChanged = vi.fn()) {
    const el = render(<PetitionMergeSection petition={petition()} mayMerge onChanged={onChanged} api={a} />);
    await flush();
    act(() => buttonByText(el, mergeActionLabel("PA-1"))!.click());
    return el;
  }

  it("sends THIS petition into the candidate, with the optional reason; success hands the body back", async () => {
    const a = api();
    const onChanged = vi.fn();
    const el = await openDialog(a, onChanged);
    type(document.querySelector<HTMLTextAreaElement>(`#${MERGE_REASON_ID}`)!, "Cùng một ổ gà");
    act(() => el.querySelector<HTMLFormElement>("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
    await flush();
    expect(a.merge).toHaveBeenCalledWith("PA-2", "PA-1", "Cùng một ổ gà");
    expect(onChanged).toHaveBeenCalledWith(expect.objectContaining({ merged_into: "PA-1" }));
    expect(el.querySelector("form")).toBeNull();
  });

  it("409 `merge_deadline_before_origin`: the server's sentence stays IN the dialog", async () => {
    const cau =
      "Không gộp được: hạn xử lý của phiếu được gộp sớm hơn lúc phiếu chính được phản ánh. Hãy chọn phiếu được phản ánh trước làm phiếu chính.";
    const onChanged = vi.fn();
    const el = await openDialog(api(undefined, { merge: vi.fn(async () => ({ ok: false as const, thongBao: cau })) }), onChanged);
    act(() => el.querySelector<HTMLFormElement>("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
    await flush();
    expect(el.querySelector("form [role='alert']")?.textContent).toBe(cau);
    expect(onChanged).not.toHaveBeenCalled();
  });
});

describe("a merged petition", () => {
  const MERGED = petition({ merged_into: "PA-1", merged_at: "2026-10-08T03:00:00Z" });

  it("says where it went, with a link; no search", async () => {
    const a = api();
    const el = render(<PetitionMergeSection petition={MERGED} mayMerge={false} api={a} />);
    await flush();
    expect(el.textContent).toContain("Đã gộp vào phiếu PA-1");
    expect(el.querySelector('a[href="/phan-anh?id=PA-1"]')).not.toBeNull();
    expect(a.list).not.toHaveBeenCalled();
  });

  it("DENIED: without `feedback.classify` there is no unmerge", async () => {
    const el = render(<PetitionMergeSection petition={MERGED} mayMerge={false} onChanged={() => {}} api={api()} />);
    await flush();
    expect(buttonByText(el, UNMERGE_ACTION)).toBeUndefined();
  });

  it("unmerge: the reason is REQUIRED; the call carries it", async () => {
    const a = api();
    const onChanged = vi.fn();
    const el = render(<PetitionMergeSection petition={MERGED} mayMerge onChanged={onChanged} api={a} />);
    await flush();
    act(() => buttonByText(el, UNMERGE_ACTION)!.click());
    const submit = () => buttonByText(el, "Tách phiếu")!;
    expect(submit().disabled).toBe(true);
    const box = document.querySelector<HTMLTextAreaElement>(`#${UNMERGE_REASON_ID}`)!;
    type(box, "   ");
    expect(submit().disabled).toBe(true);
    type(box, "Không cùng vụ việc");
    expect(submit().disabled).toBe(false);
    act(() => submit().click());
    await flush();
    expect(a.unmerge).toHaveBeenCalledWith("PA-2", "Không cùng vụ việc");
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it("unmerge refused: the server's sentence in the dialog, the reason kept", async () => {
    const cau = "Chỉ tách được phiếu chưa đóng (phiếu đã đóng, không tiếp nhận hoặc chuyển cấp trên thì không tách).";
    const el = render(
      <PetitionMergeSection
        petition={MERGED}
        mayMerge
        onChanged={() => {}}
        api={api(undefined, { unmerge: vi.fn(async () => ({ ok: false as const, thongBao: cau })) })}
      />,
    );
    await flush();
    act(() => buttonByText(el, UNMERGE_ACTION)!.click());
    const box = document.querySelector<HTMLTextAreaElement>(`#${UNMERGE_REASON_ID}`)!;
    type(box, "Không cùng vụ việc");
    act(() => buttonByText(el, "Tách phiếu")!.click());
    await flush();
    expect(el.querySelector("form [role='alert']")?.textContent).toBe(cau);
    expect(box.value).toBe("Không cùng vụ việc");
  });

  it.each(["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"])("`%s`: unmerge disabled, with the reason", async (status) => {
    const el = render(<PetitionMergeSection petition={{ ...MERGED, status }} mayMerge onChanged={() => {}} api={api()} />);
    await flush();
    expect(buttonByText(el, UNMERGE_ACTION)?.disabled).toBe(true);
    expect(el.textContent).toContain(UNMERGE_CLOSED);
  });

  it("opened from a card (no `merged_into`): says it is merged, offers to read the main code", async () => {
    const onOpen = vi.fn();
    const el = render(
      <PetitionMergeSection petition={petition({ merged_at: "2026-10-08T03:00:00Z" })} mayMerge={false} onOpen={onOpen} api={api()} />,
    );
    await flush();
    expect(el.textContent).toContain(MERGED_INTO_UNKNOWN);
    act(() => buttonByText(el, SHOW_MAIN_PETITION)!.click());
    expect(onOpen).toHaveBeenCalledWith("PA-2");
  });
});

describe("a main petition", () => {
  it("lists the petitions merged into it, each a link", async () => {
    const el = render(
      <PetitionMergeSection petition={petition({ merged_petitions: ["PA-3", "PA-4"] })} mayMerge={false} api={api()} />,
    );
    await flush();
    expect(el.textContent).toContain(MERGED_CHILDREN_LABEL);
    expect(el.querySelector('a[href="/phan-anh?id=PA-3"]')).not.toBeNull();
    expect(el.querySelector('a[href="/phan-anh?id=PA-4"]')).not.toBeNull();
  });
});
