import { renderToStaticMarkup } from "react-dom/server";
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { commitImport, previewImport } from "@/lib/api/excel-import";
import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";

import { EMPTY_ATTEMPT, nextAttempt } from "./excel-import-flow";
import type { ImportAttempt } from "./excel-import-flow";
import { ExcelImportPanel, ExcelImportView } from "./excel-import-panel";
import { STAFF_IMPORT_TARGET } from "./excel-import-targets";
import {
  CSV_BUTTON,
  CSV_FILE_NAME,
  NO_EMAIL_REASON,
  ONCE_WARNING,
  REPLAY_SENTENCE,
  SAVED_CLOSE_BUTTON,
  credentialsCsv,
  downloadCredentialsCsv,
  issuedCredentials,
} from "./staff-import-result";

/**
 * The staff import (ADR 0059 §1): the preview draws the masked mobile AS GIVEN; the 201's temporary
 * passwords are shown ONCE and are gone after close; the CSV is built in the browser and sent nowhere;
 * a replay says the passwords cannot be shown again; a 403 `role_permission_required` shows the
 * server's sentence.
 *
 * THE PANEL IS DRIVEN FOR REAL through a minimal hook runner (the shape of
 * `danh-ba-can-bo.tim.test.tsx`): not React, but enough for "what is still in the panel after close".
 *
 * Fake data only: the agreed number `0900000000`, masked as `core/privacy.MaskPhone` would; `.invalid`
 * addresses; passwords that are obviously not real.
 */

const H = vi.hoisted(() => ({
  current: null as null | { slots: { value: unknown }[]; i: number },
}));

vi.mock("react", async (original) => {
  const R = await original<typeof import("react")>();
  const slot = () => {
    const c = H.current;
    if (c === null) throw new Error("hook called outside a render");
    return { c, i: c.i++ };
  };
  return {
    ...R,
    useState: (init: unknown) => {
      const { c, i } = slot();
      if (c.slots[i] === undefined) c.slots[i] = { value: typeof init === "function" ? (init as () => unknown)() : init };
      const s = c.slots[i] as { value: unknown };
      return [s.value, (v: unknown) => (s.value = typeof v === "function" ? (v as (o: unknown) => unknown)(s.value) : v)];
    },
    useReducer: (reducer: (s: unknown, e: unknown) => unknown, init: unknown) => {
      const { c, i } = slot();
      if (c.slots[i] === undefined) c.slots[i] = { value: init };
      const s = c.slots[i] as { value: unknown };
      return [s.value, (e: unknown) => (s.value = reducer(s.value, e))];
    },
  };
});

type ViewProps = Parameters<typeof ExcelImportView<StaffImportPlannedRow, StaffImportCreatedRow>>[0];

function runPanel(props: Parameters<typeof ExcelImportPanel<StaffImportPlannedRow, StaffImportCreatedRow>>[0]) {
  const state = { slots: [] as { value: unknown }[], i: 0 };
  let tree: ReactElement<ViewProps> | null = null;
  const render = () => {
    state.i = 0;
    H.current = state;
    try {
      tree = ExcelImportPanel(props) as ReactElement<ViewProps>;
    } finally {
      H.current = null;
    }
    return tree.props;
  };
  return { render, html: () => renderToStaticMarkup(tree!) };
}

const tick = () => new Promise((r) => setTimeout(r, 0));

/* ---- fixtures ---------------------------------------------------------------------------------- */

const MASKED = "09****0000";

const PLANNED: StaffImportPlannedRow[] = [
  {
    row: 2,
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Chuyên viên",
    org_unit_code: "van-phong",
    org_unit_name: "Văn phòng",
    role_code: "",
    role_name: "",
    office_phone: "02350000001",
    mobile: MASKED,
    issues_account: true,
  },
  {
    row: 3,
    full_name: "Trần Thị B",
    email: "",
    position: "",
    org_unit_code: "",
    org_unit_name: "",
    role_code: "",
    role_name: "",
    office_phone: "",
    mobile: "",
    issues_account: false,
  },
];

const PASSWORD = "TEST-PASS-WORD-0001";

const CREATED: StaffImportCreatedRow[] = [
  {
    row: 2,
    id: "01J00000000000000000000001",
    code: "CB-7K3M9Q",
    full_name: "Nguyễn Văn A",
    login: "nva@demo.invalid",
    org_unit_code: "van-phong",
    role_code: "",
    account_issued: true,
    temporary_password: PASSWORD,
  },
  {
    row: 3,
    id: "01J00000000000000000000002",
    code: "CB-8R2T4V",
    full_name: "Trần Thị B",
    login: "",
    org_unit_code: "",
    role_code: "",
    account_issued: false,
  },
];

const ROLE_SENTENCE =
  "Tệp có cột Vai trò không trống — gán vai trò cần thêm quyền Phân quyền (admin.role). Hãy để trống cột Vai trò, hoặc nhờ người có quyền này nhập tệp.";

function staffView(
  preview: KetQua<ImportPreview<StaffImportPlannedRow>> | null,
  result: ImportResult<StaffImportCreatedRow> | null = null,
) {
  return renderToStaticMarkup(
    <ExcelImportView
      target={STAFF_IMPORT_TARGET}
      fileChosen
      preview={preview}
      result={result}
      busy=""
      templateError=""
      onDownloadTemplate={() => {}}
      onChooseFile={() => {}}
      onPreview={() => {}}
      onImport={() => {}}
      onClose={() => {}}
    />,
  );
}

let consoleSpies: ReturnType<typeof vi.spyOn>[] = [];

beforeEach(() => {
  consoleSpies = (["log", "info", "warn", "error", "debug"] as const).map((m) =>
    vi.spyOn(console, m).mockImplementation(() => {}),
  );
});

afterEach(() => {
  // NEVER LOG THE PASSWORDS (rule 3, forbidden #1): nothing in any case of this file writes to the console.
  for (const s of consoleSpies) expect(s).not.toHaveBeenCalled();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

/* ---- the routes and the preview ---------------------------------------------------------------- */

describe("staff import — routes and preview", () => {
  it("reads the three staff routes; the preview's rows are `people`", () => {
    expect(STAFF_IMPORT_TARGET.routes).toEqual({
      template: "/api/v1/staff/import-template",
      previews: "/api/v1/staff/import-previews",
      imports: "/api/v1/staff/imports",
    });
    expect(STAFF_IMPORT_TARGET.rowsField).toBe("people");
    expect(STAFF_IMPORT_TARGET.templateFileName).toBe("mau-nhap-can-bo.xlsx");
  });

  it("the explanation says, before the file is filled, that Vai trò needs Phân quyền and passwords show once", () => {
    const html = staffView(null);
    expect(html).toContain("Tệp có cột Vai trò không trống thì tài khoản của bạn cần thêm quyền Phân quyền.");
    expect(html).toContain("hiện ĐÚNG MỘT LẦN");
  });

  it("valid preview: the MASKED mobile is drawn exactly as the server sent it; a row without email says why", () => {
    const html = staffView({ ok: true, duLieu: { valid: true, rows: PLANNED, errors: [] } });
    expect(html).toContain("Sẽ tạo 2 cán bộ");
    expect(html).toContain(`<td>${MASKED}</td>`);
    // Not re-masked, not unmasked: the agreed fake number never appears whole.
    expect(html).not.toContain("0900000000");
    expect(html).toContain("<td>Văn phòng</td><td>Chưa gán vai trò</td><td>02350000001</td>");
    expect(html).toContain(`<td>Không cấp — ${NO_EMAIL_REASON}</td>`);
    expect(html).toContain("<td>Chưa phân bộ phận</td>");
    expect(html).toContain(">Nhập các cán bộ này</button>");
  });

  it("403 role_permission_required on the PREVIEW: the server's sentence, as it came", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ code: "role_permission_required", message: ROLE_SENTENCE, trace_id: "t" }), {
          status: 403,
        }),
      ),
    );
    const p = await previewImport<StaffImportPlannedRow>(
      STAFF_IMPORT_TARGET.routes,
      STAFF_IMPORT_TARGET.rowsField,
      new Blob(["x"]),
      "can-bo.xlsx",
    );
    expect(p).toEqual({ ok: false, thongBao: ROLE_SENTENCE });
    expect(staffView(p)).toContain(`role="alert">${ROLE_SENTENCE}</p>`);
  });

  it("403 role_permission_required on the IMPORT: the server's sentence, no password panel", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ code: "role_permission_required", message: ROLE_SENTENCE, trace_id: "t" }), {
          status: 403,
        }),
      ),
    );
    const r = await commitImport<StaffImportCreatedRow>(STAFF_IMPORT_TARGET.routes, new Blob(["x"]), "can-bo.xlsx", "k-1");
    expect(r).toEqual({ ok: false, message: ROLE_SENTENCE, errors: [] });
    const html = staffView({ ok: true, duLieu: { valid: true, rows: PLANNED, errors: [] } }, r);
    expect(html).toContain(ROLE_SENTENCE);
    expect(html).not.toContain("Mật khẩu tạm");
  });
});

/* ---- the passwords: shown once, cleared on close ----------------------------------------------- */

describe("staff import — the temporary passwords", () => {
  it("201: code · name · login · password for the issued row, the once-warning, CSV and 'Tôi đã lưu, đóng'", () => {
    const html = staffView(null, { ok: true, created: CREATED });
    expect(html).toContain("Đã nhập 2 cán bộ. Danh sách đã được tải lại.");
    expect(html).toContain(ONCE_WARNING);
    expect(html).toContain(
      `<td class="ma-muc">CB-7K3M9Q</td><td>Nguyễn Văn A</td><td>nva@demo.invalid</td><td class="mat-khau-tam ma-muc">${PASSWORD}</td>`,
    );
    expect(html).toContain(`>${CSV_BUTTON}</button>`);
    expect(html).toContain(`>${SAVED_CLOSE_BUTTON}</button>`);
    // The panel's reflex "Đóng" is NOT drawn: only the explicit act closes a panel holding passwords.
    expect(html).not.toContain(">Đóng</button>");
    // The row without an address: named, with the reason, and no password cell.
    expect(html).toContain(NO_EMAIL_REASON);
    expect(html).toContain("Trần Thị B (CB-8R2T4V)");
    expect(html.match(/mat-khau-tam ma-muc/g)).toHaveLength(1);
    // The value is never in an attribute (rule 3, forbidden #4).
    expect(html).not.toMatch(new RegExp(`="[^"]*${PASSWORD}`));
  });

  it("the panel, driven for real: preview → import shows the password ONCE; close empties the attempt", async () => {
    const fetchMock = vi.fn(async (path: string) =>
      path.endsWith("/import-previews")
        ? new Response(JSON.stringify({ valid: true, people: PLANNED, errors: [] }), { status: 200 })
        : new Response(JSON.stringify({ batch_id: "01JBATCH", created: CREATED }), { status: 201 }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const onImported = vi.fn();
    const onClose = vi.fn();
    const m = runPanel({ target: STAFF_IMPORT_TARGET, onImported, onClose });

    m.render().onChooseFile(new File(["x"], "can-bo.xlsx"));
    m.render().onPreview();
    await tick();
    m.render().onImport();
    await tick();
    const shown = m.render();
    expect(shown.result).toEqual({ ok: true, created: CREATED });
    expect(m.html()).toContain(PASSWORD);
    expect(onImported).toHaveBeenCalledTimes(1);

    // A list re-read (the caller's `onImported`) re-renders the panel; the passwords stay.
    expect(m.html()).toContain(PASSWORD);

    shown.onClose();
    const after = m.render();
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(after.result).toBeNull();
    expect(after.preview).toBeNull();
    expect(after.fileChosen).toBe(false);
    expect(m.html()).not.toContain(PASSWORD);
    // Two requests, the preview and the import — the passwords never went back to the network.
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("`nextAttempt`: `closed` and a new file both drop the 201; a new preview drops the key", () => {
    const withPasswords: ImportAttempt<StaffImportPlannedRow, StaffImportCreatedRow> = {
      file: { blob: new Blob(["x"]), name: "can-bo.xlsx" },
      preview: { ok: true, duLieu: { valid: true, rows: PLANNED, errors: [] } },
      key: null,
      result: { ok: true, created: CREATED },
    };
    expect(nextAttempt(withPasswords, { type: "closed" })).toEqual(EMPTY_ATTEMPT);
    expect(nextAttempt(withPasswords, { type: "chosen", file: null }).result).toBeNull();
    expect(nextAttempt(withPasswords, { type: "previewStarted" }).result).toBeNull();
    const failed = nextAttempt(
      { ...withPasswords, result: null },
      { type: "imported", key: "k-1", result: { ok: false, message: "x", errors: [] } },
    );
    expect(failed.key).toBe("k-1");
    expect(nextAttempt(failed, { type: "previewed", preview: withPasswords.preview! }).key).toBeNull();
  });

  it("REPLAY (201 { code, replayed: true }): no password, the sentence says reset per person", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ code: "01JBATCH", replayed: true }), {
          status: 201,
          headers: { "Idempotent-Replay": "true" },
        }),
      ),
    );
    const r = await commitImport<StaffImportCreatedRow>(STAFF_IMPORT_TARGET.routes, new Blob(["x"]), "can-bo.xlsx", "k-1");
    expect(r).toEqual({ ok: true, created: null });
    const html = staffView(null, r);
    expect(html).toContain(REPLAY_SENTENCE);
    expect(html).toContain("Đặt lại mật khẩu");
    expect(html).not.toContain(CSV_BUTTON);
    expect(html).not.toContain("mat-khau-tam");
  });

  it("an issued account whose password did not arrive is named, never drawn as 'undefined'", () => {
    const odd: StaffImportCreatedRow[] = [{ ...CREATED[0]!, temporary_password: undefined }];
    const html = staffView(null, { ok: true, created: odd });
    expect(html).toContain("không nhận được mật khẩu tạm");
    expect(html).not.toContain("undefined");
    expect(issuedCredentials(odd)).toEqual([]);
  });
});

/* ---- the CSV ----------------------------------------------------------------------------------- */

describe("staff import — the CSV is built in the browser", () => {
  const rows = issuedCredentials(CREATED);

  it("BOM, header, CRLF, every cell quoted; only issued rows", () => {
    expect(credentialsCsv(rows)).toBe(
      '﻿"Mã cán bộ","Họ và tên","Tên đăng nhập","Mật khẩu tạm"\r\n' +
        `"CB-7K3M9Q","Nguyễn Văn A","nva@demo.invalid","${PASSWORD}"\r\n`,
    );
  });

  it("a name that a spreadsheet would run as a formula is neutralised; the password is never altered", () => {
    const csv = credentialsCsv([{ code: "CB-1", fullName: '=HYPERLINK("x")', login: "a@demo.invalid", password: "-AB" }]);
    expect(csv).toContain(`"'=HYPERLINK(""x"")"`);
    expect(csv).toContain('"-AB"');
  });

  it("download: a Blob with the CSV, a fixed file name, the URL revoked — no fetch, no storage", async () => {
    const fetchMock = vi.fn();
    const setItem = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("localStorage", { setItem });
    vi.stubGlobal("sessionStorage", { setItem });
    const anchor = { href: "", download: "", click: vi.fn() };
    vi.stubGlobal("document", { createElement: vi.fn(() => anchor) });
    const captured: { blob?: Blob } = {};
    const create = vi.spyOn(URL, "createObjectURL").mockImplementation((b) => {
      captured.blob = b as Blob;
      return "blob:local/1";
    });
    const revoke = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});

    downloadCredentialsCsv(rows);

    expect(create).toHaveBeenCalledTimes(1);
    // Bytes, not `.text()`: decoding strips the BOM, and the BOM is what makes Excel read UTF-8.
    const bytes = new Uint8Array(await captured.blob!.arrayBuffer());
    expect([...bytes.slice(0, 3)]).toEqual([0xef, 0xbb, 0xbf]);
    expect(new TextDecoder("utf-8", { ignoreBOM: true }).decode(bytes)).toBe(credentialsCsv(rows));
    expect(captured.blob!.type).toBe("text/csv;charset=utf-8");
    expect(anchor.download).toBe(CSV_FILE_NAME);
    expect(anchor.href).toBe("blob:local/1");
    expect(anchor.click).toHaveBeenCalledTimes(1);
    expect(revoke).toHaveBeenCalledWith("blob:local/1");
    expect(fetchMock).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
    // The file name carries no person and no value.
    expect(CSV_FILE_NAME).not.toMatch(/CB-|@|TEST/);
  });
});
