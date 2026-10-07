import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { BANG_NHAN_MAC_DINH, SO_RONG } from "./nhan-nhiem-vu";
import { BangNhiemVu, SoNhiemVu } from "./so-nhiem-vu";
import { TaskRowsSkeleton, TaskStatusBadge, toggleButtonClass } from "./task-ui";

/**
 * The Nhiệm vụ restyle (ADR 0068, spec §8.3/§8b) is PRESENTATION. What is pinned here is what the
 * restyle must not have moved: the gate on the header actions, the words the screen already had,
 * and the fact that an icon never replaces a word.
 */

const PAGE = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

describe("page header — `Nhập từ Excel` · `Giao việc mới` stay behind `task.create`", () => {
  it("DENIED (no session read ⇒ every gate closed): the title is there, neither action is", () => {
    const html = renderToStaticMarkup(
      <PhienProvider>
        <SoNhiemVu />
      </PhienProvider>,
    );
    expect(html).toMatch(/<h1[^>]*>Quản lý nhiệm vụ<\/h1>/);
    expect(html).not.toContain("Giao việc mới");
    expect(html).not.toContain("Nhập từ Excel");
  });

  it("ALLOWED (source): both buttons are the `actions` of the header only when `quyen.giaoViec`", () => {
    expect(PAGE).toMatch(/actions=\{\s*quyen\.giaoViec \? \(/);
    // 06/10/2026 (prototype): `Nhập từ Excel` outline FIRST, `+ Giao việc mới` solid second; each
    // opens a dialog, neither toggles.
    const actions = PAGE.slice(PAGE.indexOf("actions={"), PAGE.indexOf("<section"));
    expect(actions.indexOf('variant="outline"')).toBeLessThan(actions.indexOf('variant="primary"'));
    expect(actions.indexOf("{IMPORT_OPEN_BUTTON}")).toBeLessThan(actions.indexOf("Giao việc mới"));
    expect(actions).toContain("onClick={() => setImportOpen(true)}");
    expect(actions).toContain("onClick={openCreate}");
    expect(PAGE).toContain("const openCreate = () => {\n    datMoFormTao(true);");
  });

  it("the section keeps its accessible name; only the visual heading moved to the `<h1>`", () => {
    const html = renderToStaticMarkup(
      <PhienProvider>
        <SoNhiemVu />
      </PhienProvider>,
    );
    expect(html).toContain('aria-labelledby="tieu-de-so-nhiem-vu"');
    expect(html).toContain('<h2 id="tieu-de-so-nhiem-vu" class="an-thi-giac">Sổ nhiệm vụ của xã</h2>');
  });
});

describe("status pill — icon by CODE, word by the commune", () => {
  it("draws the label it is given, verbatim, after a decorative icon", () => {
    const html = renderToStaticMarkup(<TaskStatusBadge status="tam-dung">Tạm hoãn</TaskStatusBadge>);
    expect(html).toMatch(/<svg[^>]*lucide-circle-pause[^>]*aria-hidden="true"/);
    expect(html).toContain(">Tạm hoãn</span>");
  });

  it("an unknown code still shows its label (neutral pill, no specific icon)", () => {
    const html = renderToStaticMarkup(<TaskStatusBadge status="ma-la">Mã lạ</TaskStatusBadge>);
    expect(html).toContain(">Mã lạ</span>");
  });

  it("red is not a status colour: overdue is derived in the `Hạn` column, never a pill tone", () => {
    for (const ma of BANG_NHAN_MAC_DINH.thuTu) {
      const html = renderToStaticMarkup(<TaskStatusBadge status={ma}>x</TaskStatusBadge>);
      expect(html).not.toContain("danger");
    }
  });
});

describe("empty list (spec §8b) — the screen picks the sentence; the table never guesses", () => {
  const props = {
    nhiemVu: [],
    danhMuc: { loai: [], mucUuTien: [], khoi: [], boPhan: [] },
    nhanTT: BANG_NHAN_MAC_DINH,
    tenBoPhan: new Map<string, string>(),
    bayGio: new Date("2026-10-02T03:00:00Z"),
    maDangMo: null,
    moNhiemVu: () => {},
    sapXep: { cot: "code", chieu: "asc" },
    doiSapXep: () => {},
  } as const;

  it("without `empty`: the filter sentence `SO_RONG`, as before", () => {
    expect(renderToStaticMarkup(<BangNhiemVu {...props} />)).toContain(SO_RONG);
  });

  it("with `empty`: exactly what the screen passed", () => {
    const html = renderToStaticMarkup(<BangNhiemVu {...props} empty={<p>riêng</p>} />);
    expect(html).toContain("<p>riêng</p>");
    expect(html).not.toContain(SO_RONG);
  });

  it("the screen says `Chưa có nhiệm vụ nào` only with no filter and no drill-down", () => {
    // `Xem thêm` appends to the first page (owner 07/10/2026 #5): there is no "later page" to exclude.
    expect(PAGE).toContain("const noFilter =\n    !drillDownActive &&\n    Object.entries(loc)");
    expect(PAGE).toContain('k === "sapXep" || k === "chieu" || v === undefined');
  });
});

describe("decorative pieces", () => {
  it("the skeleton is hidden from assistive tech — the `role=status` sentence announces the load", () => {
    expect(renderToStaticMarkup(<TaskRowsSkeleton />)).toMatch(/^<div aria-hidden="true"/);
    expect(PAGE).toContain('<p className="an-thi-giac" role="status">');
  });

  it("toggle look differs by weight and surface, not by colour alone", () => {
    expect(toggleButtonClass(true)).toContain("font-semibold");
    expect(toggleButtonClass(false)).not.toContain("font-semibold");
  });
});
