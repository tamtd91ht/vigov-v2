"use client";

import { Check, Download, KeyRound, TriangleAlert } from "lucide-react";

import { Button } from "@/components/ui/button";
import { CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { Notice } from "@/components/ui/notice";
import type { StaffImportCreatedRow } from "@/lib/api/staff-import";

import { CAU_VIEC_CAN_LAM, NUT_DAT_LAI_MAT_KHAU } from "./mat-khau-tam";

/**
 * The result of a staff import — the N temporary passwords of ADR 0059 §1, shown ONCE.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * THE SAME CONDITIONS AS `mat-khau-tam.tsx`, MULTIPLIED BY N. Read that header first; what holds for one
 * password holds here for each:
 *
 *   1. NO `console.*` anywhere near these values (rule 3, forbidden #1).
 *   2. NO `localStorage`, `sessionStorage`, module-level variable, URL, `title`, `aria-label` or file
 *      name carries them. They live in ONE place: the `result` of the import panel's attempt
 *      (`nextAttempt` in `excel-import-flow.ts`), which `closed` empties and the caller then unmounts.
 *   3. Readable text, monospaced (`.ma-muc`), no `type="password"`: they are read aloud or handed over.
 *   4. The panel never closes itself. Only `SAVED_CLOSE_BUTTON` — an act stated as done — closes it.
 *
 * THE ONE ADDITION IS THE CSV, AND IT IS THE USER'S CHOICE (ADR 0059 §1: "trả về MỘT LẦN … để quản trị
 * viên tải xuống và phát cho từng người"). It is built IN THE BROWSER from the rows already on the
 * screen — a `Blob`, an object URL revoked right after the click — so the passwords make no second trip
 * over the network and no server keeps a copy. The cost, stated on the screen as ADR 0059 states it:
 * the file is a list of credentials on the administrator's machine until it is deleted.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** One account issued by this import, with its password. */
export type IssuedCredential = {
  readonly code: string;
  readonly fullName: string;
  readonly login: string;
  readonly password: string;
};

/** The rows whose account was issued AND whose password came back. */
export function issuedCredentials(created: readonly StaffImportCreatedRow[]): readonly IssuedCredential[] {
  return created
    .filter((p) => p.account_issued && typeof p.temporary_password === "string" && p.temporary_password !== "")
    .map((p) => ({ code: p.code, fullName: p.full_name, login: p.login, password: p.temporary_password as string }));
}

/** Rows without a work address: a directory entry only, no account (ADR 0059 §1, email stored NULL). */
export function withoutAccount(created: readonly StaffImportCreatedRow[]): readonly StaffImportCreatedRow[] {
  return created.filter((p) => !p.account_issued);
}

/**
 * Rows the server says have an account but whose password did not arrive. Not expected on a first 201;
 * named anyway, because the alternative is a cell reading "undefined" to somebody holding a pen
 * (`CAU_PHAT_LAI_KHONG_CO_MAT_KHAU` in `mat-khau-tam.tsx` is the same trap on the single route).
 */
export function issuedWithoutPassword(created: readonly StaffImportCreatedRow[]): readonly StaffImportCreatedRow[] {
  return created.filter(
    (p) => p.account_issued && (typeof p.temporary_password !== "string" || p.temporary_password === ""),
  );
}

/* ---- the CSV --------------------------------------------------------------------------------- */

export const CSV_FILE_NAME = "mat-khau-tam-can-bo-nhap-tu-excel.csv";
export const CSV_HEADER = ["Mã cán bộ", "Họ và tên", "Tên đăng nhập", "Mật khẩu tạm"] as const;

/**
 * A text cell a spreadsheet would read as a FORMULA (`=`, `+`, `-`, `@`, tab, CR) gets a leading `'`.
 * A full name comes from a file somebody typed; opening the CSV must not run it.
 *
 * NOT APPLIED TO THE PASSWORD, on purpose: a prefix would change the credential and nobody would notice
 * until the sign-in failed. The server's alphabet never starts one with any of those characters — the
 * hyphens only sit BETWEEN groups (`service-identity/internal/domain/mat_khau.go:106`).
 */
function textCell(v: string): string {
  return /^[=+\-@\t\r]/.test(v) ? `'${v}` : v;
}

function quoted(v: string): string {
  return `"${v.replace(/"/g, '""')}"`;
}

/**
 * The file's text. CRLF and a UTF-8 BOM: without the BOM, Excel on Windows opens the file as ANSI and
 * every Vietnamese name is garbled — the one thing the person handing out slips must read.
 */
export function credentialsCsv(rows: readonly IssuedCredential[]): string {
  const lines = [CSV_HEADER.map(quoted).join(",")];
  for (const r of rows) {
    lines.push([textCell(r.code), textCell(r.fullName), textCell(r.login), r.password].map(quoted).join(","));
  }
  return `﻿${lines.join("\r\n")}\r\n`;
}

/** Build the file in the browser and hand it to the browser's download. Nothing is sent anywhere. */
export function downloadCredentialsCsv(rows: readonly IssuedCredential[]): void {
  const blob = new Blob([credentialsCsv(rows)], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = CSV_FILE_NAME;
  a.click();
  URL.revokeObjectURL(url);
}

/* ---- words ----------------------------------------------------------------------------------- */

export const CSV_BUTTON = "Tải danh sách (.csv)";
/** An act stated as done, not "Đóng" — the half-second before N unrecoverable values disappear. */
export const SAVED_CLOSE_BUTTON = "Tôi đã lưu, đóng";
export const PLAIN_CLOSE_BUTTON = "Đóng";

export const ONCE_WARNING =
  "Mật khẩu tạm chỉ hiện một lần. Hệ thống không lưu bản đọc được và không có cách nào hiện lại: hãy " +
  "ghi lại hoặc tải danh sách ngay bây giờ rồi giao tận tay từng người; người dùng phải đổi mật khẩu ở " +
  `lần đăng nhập đầu. Nếu để mất, bấm ${NUT_DAT_LAI_MAT_KHAU} ở dòng của từng người — lần đặt lại sinh ` +
  "một mật khẩu KHÁC.";

export const CSV_CAUTION =
  "Tệp tải xuống là danh sách thông tin đăng nhập, tạo ngay trên máy của bạn, không gửi đi đâu. Không " +
  "gửi tệp qua thư điện tử hay tin nhắn; xoá tệp sau khi đã giao mật khẩu cho từng người.";

export const REPLAY_SENTENCE =
  "Tệp này đã được nhập ở lần gửi trước. Hệ thống cố ý không lưu câu trả lời cũ, nên mật khẩu tạm của " +
  `lần ấy không hiện lại được. Với từng cán bộ vừa được cấp tài khoản, hãy bấm ${NUT_DAT_LAI_MAT_KHAU} ` +
  "ở dòng của người ấy — lần đặt lại sinh một mật khẩu mới.";

export const NO_EMAIL_REASON = "chưa có thư điện tử nên chưa cấp tài khoản";

export const MISSING_PASSWORD_SENTENCE =
  `Đã cấp tài khoản nhưng không nhận được mật khẩu tạm. Hãy bấm ${NUT_DAT_LAI_MAT_KHAU} ở dòng của ` +
  "người này.";

/* ---- the view -------------------------------------------------------------------------------- */

/**
 * `created === null` is a REPLAY (`{ code, replayed: true }`): no password, and the sentence says how to
 * get one. Pure rendering — the CSV button calls `downloadCredentialsCsv` on the rows it was given.
 *
 * `role="status"` sits on the panel's `importedSentence`, not on this block: a live region around the
 * table would read N passwords aloud through the speakers of a one-stop-shop office.
 */
export function StaffImportResult({
  created,
  onClose,
}: {
  created: readonly StaffImportCreatedRow[] | null;
  onClose: () => void;
}) {
  if (created === null) {
    return (
      <div className="flex flex-col gap-3">
        <Notice tone="neutral" icon={TriangleAlert}>
          {REPLAY_SENTENCE}
        </Notice>
        <p className="m-0 flex justify-end">
          <Button type="button" variant="secondary" onClick={onClose}>
            {PLAIN_CLOSE_BUTTON}
          </Button>
        </p>
      </div>
    );
  }

  const issued = issuedCredentials(created);
  const noAccount = withoutAccount(created);
  const missing = issuedWithoutPassword(created);

  return (
    // No hooks: rendered inside the import view, which a test renders under a mocked React.
    <section
      className="khoi-chi-tiet m-0 min-w-0 overflow-hidden rounded-card border border-brand-100 bg-surface p-0 shadow-sm"
      aria-labelledby="tieu-de-mat-khau-nhap"
    >
      <CardHeader className="dau-khoi-chi-tiet m-0">
        <CardTitle as="h3" id="tieu-de-mat-khau-nhap" className="flex items-center gap-2">
          <KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
          {issued.length > 0 ? "Mật khẩu tạm — chỉ hiện một lần" : "Tài khoản đăng nhập"}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-3 [&>*]:my-0">

      {issued.length > 0 && (
        <>
          <Notice tone="neutral" icon={TriangleAlert}>
            {ONCE_WARNING}
          </Notice>
          <TableScroll sticky aria-label="Tài khoản vừa cấp">
            <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
              <thead>
                <tr>
                  {CSV_HEADER.map((h) => (
                    <th key={h} scope="col">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {issued.map((c) => (
                  <tr key={c.code}>
                    <td className="ma-muc">{c.code}</td>
                    <td>{c.fullName}</td>
                    <td>{c.login}</td>
                    <td className="mat-khau-tam ma-muc">{c.password}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableScroll>
          <p className="ghi-chu">{CAU_VIEC_CAN_LAM}</p>
          <p className="ghi-chu">{CSV_CAUTION}</p>
        </>
      )}

      {missing.length > 0 && (
        <div className="thong-bao-loi [&_p]:m-0 [&_ul]:my-1" role="alert">
          <p>{MISSING_PASSWORD_SENTENCE}</p>
          <ul>
            {missing.map((p) => (
              <li key={p.code}>
                {p.full_name} ({p.code})
              </li>
            ))}
          </ul>
        </div>
      )}

      {noAccount.length > 0 && (
        <div className="[&_p]:m-0 [&_ul]:my-1">
          <p>Những người sau {NO_EMAIL_REASON}:</p>
          <ul>
            {noAccount.map((p) => (
              <li key={p.code}>
                {p.full_name} ({p.code})
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="cum-nut m-0 flex flex-wrap justify-end gap-2">
        {issued.length > 0 && (
          <Button
            type="button"
            variant="secondary"
            icon={<Download aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => downloadCredentialsCsv(issued)}
          >
            {CSV_BUTTON}
          </Button>
        )}
        <Button
          type="button"
          variant="primary"
          icon={<Check aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={onClose}
        >
          {issued.length > 0 ? SAVED_CLOSE_BUTTON : PLAIN_CLOSE_BUTTON}
        </Button>
      </div>
      </CardContent>
    </section>
  );
}
