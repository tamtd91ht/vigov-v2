"use client";

import { Pencil, Plus, Sprout, Trash2, TriangleAlert } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import {
  gieoNgayNghiLeMacDinh,
  gieoTuanMacDinh,
  suaCaLamViec,
  suaNgayLamBu,
  suaNgayNghiLe,
  themCaLamViec,
  themNgayLamBu,
  themNgayNghiLe,
  xoaCaLamViec,
  xoaNgayLamBu,
  xoaNgayNghiLe,
} from "@/lib/api/lich-ghi";
import { layLichLamViec, layNgayLamBu, layNgayNghiLe } from "@/lib/api/lich-lam-viec";
import type {
  identity_caLamBuRa,
  identity_caLamViecRa,
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
  identity_ngayNghiLeRa,
} from "@/lib/api/schema.gen";
import { danhSachNam, namTheoDongHoMay } from "@/lib/nam";

import { khoiCanhBao, tinhTrangBang } from "./chua-cau-hinh";
import type { KhoiCanhBao } from "./chua-cau-hinh";
import {
  ConfigField,
  ConfigFormRow,
  ConfigLoading,
  ConfigTable,
  EmptyRow,
  RowActions,
  SMALL_BUTTON_CLASS,
  formInputCls,
  formSelectCls,
} from "./config-ui";
import { DAN_LICH_LAM_VIEC, nhanCa, nhanGio, nhanNgay, tenThu } from "./nhan-lich-lam-viec";
import {
  CANH_BAO_XOA_CA,
  CANH_BAO_XOA_NGAY_LAM_BU,
  CANH_BAO_XOA_NGAY_NGHI,
  CON_THIEU_NGAY_LE,
  DA_LUU_LICH,
  DA_XOA_LICH,
  GHI_CHU_SAU_KHI_GIEO,
  GIAI_THICH_CA,
  GIAI_THICH_LY_DO_XOA,
  KHOI_CHUA_KHAI_HAU_QUA,
  KHOI_CHUA_KHAI_THIEU_LICH_TUAN,
  KHOI_CHUA_KHAI_THIEU_THOI_HAN,
  KHOI_CHUA_KHAI_TIEU_DE,
  LOI_THIEU_LY_DO,
  NUT_GIEO_NGAY_LE,
  NUT_GIEO_THOI_HAN,
  NUT_GIEO_TUAN,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA,
  NUT_THEM_CA,
  NUT_THEM_NGAY_LAM_BU,
  NUT_THEM_NGAY_NGHI,
  NUT_XAC_NHAN_XOA,
  NUT_XOA,
  O_GHI_CHU_CA,
  O_GIO_BAT_DAU,
  O_GIO_KET_THUC,
  O_LY_DO_XOA,
  O_NGAY,
  O_TEN_NGAY_LAM_BU,
  O_TEN_NGAY_NGHI,
  O_THU,
  cauGieoNgayLe,
  cauGieoTuan,
} from "./nhan-thoi-han";
import { quyetDinhGhiThoiHan } from "./quyen-tab";

/**
 * Tab "Lịch làm việc" — the commune's three calendar tables: working hours of the week, public
 * holidays, swap working days (ADR 0007). Owner decision 08/10/2026 (ADR 0079 D1/D2): they left
 * "Thời hạn xử lý", which is now gated on `admin.sla` like the prototype, for a tab of their own with
 * NO gate. The code below MOVED from `tab-thoi-han-xu-ly.tsx`; its behaviour is unchanged, its look is
 * the shared table pattern of spec `02-khung-trang.md` (grey form row above a framed table).
 *
 * THE GATE WRAPS THE WRITES, NOT THE TABLES. The three reads are `any-authenticated` on the server,
 * so every account sees them; hiding them would be the UI refusing what the server serves (rule 5,
 * forbidden #1). The eleven write routes declare `RequirePermission("admin.sla")` and check it on
 * EVERY request — hiding a button is convenience, not a control.
 *
 * NO WORKING-HOUR ARITHMETIC HERE. `identity` owns the tables and the sum (ADR 0007); this screen only
 * shows the rows and the sentences the server returns. It does not check time formats, overlapping
 * shifts or a day that is both a holiday and a swap day: the server checks all four, each with a
 * Vietnamese sentence saying what to fix, and a client copy of those rules would drift unseen.
 */

/* ---- state ------------------------------------------------------------------------------------ */

/** Which form is open. ONE for the whole tab: three drafts at once are three wrong submissions. */
export type CalendarOpenForm =
  | { kieu: "themCa" }
  | { kieu: "suaCa"; ca: identity_caLamViecRa }
  | { kieu: "xoaCa"; ca: identity_caLamViecRa }
  | { kieu: "themNghi" }
  | { kieu: "suaNghi"; ngay: identity_ngayNghiLeRa }
  | { kieu: "xoaNghi"; ngay: identity_ngayNghiLeRa }
  | { kieu: "themLamBu" }
  | { kieu: "suaLamBu"; ca: identity_caLamBuRa }
  | { kieu: "xoaLamBu"; ca: identity_caLamBuRa }
  | null;

/** The draft being typed. All strings — that is what a browser input returns. */
export type CalendarDraft = {
  thu: string;
  batDau: string;
  ketThuc: string;
  ghiChu: string;
  ngay: string;
  ten: string;
  lyDo: string;
};

export const EMPTY_CALENDAR_DRAFT: CalendarDraft = {
  thu: "1",
  batDau: "",
  ketThuc: "",
  ghiChu: "",
  ngay: "",
  ten: "",
  lyDo: "",
};

/** What a row, a section header or the alert block may ask for. */
export type CalendarActions = {
  readonly seedWeek: () => void;
  readonly seedHolidays: () => void;
  readonly addShift: () => void;
  readonly editShift: (c: identity_caLamViecRa) => void;
  readonly deleteShift: (c: identity_caLamViecRa) => void;
  readonly addHoliday: () => void;
  readonly editHoliday: (n: identity_ngayNghiLeRa) => void;
  readonly deleteHoliday: (n: identity_ngayNghiLeRa) => void;
  readonly addSwapDay: () => void;
  readonly editSwapDay: (c: identity_caLamBuRa) => void;
  readonly deleteSwapDay: (c: identity_caLamBuRa) => void;
};

/** The three reads. `null` is NOT READ YET — different from "read, and empty". */
export type CalendarData = {
  readonly week: KetQua<identity_danhSachCaLamViecRa> | null;
  readonly holidays: KetQua<identity_danhSachNgayNghiLeRa> | null;
  readonly swapDays: KetQua<identity_danhSachCaLamBuRa> | null;
};

/** The three tables of the tab. How the SCREEN groups them, not contract data. */
export type CalendarGroup = "week" | "holidays" | "swapDays" | null;

/** Which table owns the open form — a pure function, so a `switch` does not sit between two JSX tags. */
export function calendarGroupOf(open: CalendarOpenForm): CalendarGroup {
  if (open === null) return null;
  switch (open.kieu) {
    case "themCa":
    case "suaCa":
    case "xoaCa":
      return "week";
    case "themNghi":
    case "suaNghi":
    case "xoaNghi":
      return "holidays";
    default:
      return "swapDays";
  }
}

const isDelete = (open: NonNullable<CalendarOpenForm>) =>
  open.kieu === "xoaCa" || open.kieu === "xoaNghi" || open.kieu === "xoaLamBu";

/* ---- data shell ------------------------------------------------------------------------------- */

export function WorkingCalendarTab() {
  const [baseYear] = useState(namTheoDongHoMay);
  const [year, setYear] = useState(baseYear);

  /** Bumped after every successful write to READ AGAIN from the server, never to patch arrays in place. */
  const [reads, setReads] = useState(0);

  const [week, setWeek] = useState<KetQua<identity_danhSachCaLamViecRa> | null>(null);
  /**
   * The two per-year tables keep their result WITH the year that produced it; "loading" is DERIVED from
   * that year differing from the selected one. Setting `null` inside the effect would leave a window,
   * however narrow, where last year's holidays stand under a picker already showing the new year.
   */
  const [holidays, setHolidays] = useState<{ year: number; result: KetQua<identity_danhSachNgayNghiLeRa> } | null>(
    null,
  );
  const [swapDays, setSwapDays] = useState<{ year: number; result: KetQua<identity_danhSachCaLamBuRa> } | null>(
    null,
  );

  const [open, setOpen] = useState<CalendarOpenForm>(null);
  const [draft, setDraft] = useState<CalendarDraft>(EMPTY_CALENDAR_DRAFT);
  const [localError, setLocalError] = useState("");
  const [serverError, setServerError] = useState("");
  const [busy, setBusy] = useState(false);

  const session = usePhien();
  // THREE STATES, NOT TWO: no write button is drawn until the session is read — "not known yet" must
  // act neither as "allowed" nor as "refused".
  const canWrite = session !== null && quyetDinhGhiThoiHan(session).hien;

  useEffect(() => {
    let dropped = false;
    layLichLamViec().then((r) => {
      if (!dropped) setWeek(r);
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  useEffect(() => {
    let dropped = false;
    layNgayNghiLe(year).then((result) => {
      if (!dropped) setHolidays({ year, result });
    });
    layNgayLamBu(year).then((result) => {
      if (!dropped) setSwapDays({ year, result });
    });
    return () => {
      dropped = true;
    };
  }, [year, reads]);

  /**
   * Opens a form. Pressing the SAME add button again closes it (spec: the add button toggles its row);
   * edit and delete always open, on the row they were pressed on.
   */
  const show = useCallback((next: NonNullable<CalendarOpenForm>, initial: CalendarDraft) => {
    const toggles = next.kieu === "themCa" || next.kieu === "themNghi" || next.kieu === "themLamBu";
    setOpen((current) => (toggles && current !== null && current.kieu === next.kieu ? null : next));
    setDraft(initial);
    setLocalError("");
    setServerError("");
  }, []);

  const close = useCallback(() => {
    setOpen(null);
    setDraft(EMPTY_CALENDAR_DRAFT);
    setLocalError("");
    setServerError("");
  }, []);

  /**
   * One write: clear old messages, send, then either toast what was done and READ AGAIN, or show the
   * server's sentence AS WRITTEN — no branching on `code`, no `trace_id`, no HTTP status.
   */
  const run = useCallback(function <T>(call: Promise<KetQua<T>>, sentence: (d: T) => string) {
    setLocalError("");
    setServerError("");
    setBusy(true);
    void call.then((r) => {
      setBusy(false);
      if (!r.ok) {
        setServerError(r.thongBao);
        return;
      }
      setOpen(null);
      setDraft(EMPTY_CALENDAR_DRAFT);
      toast.success(sentence(r.duLieu));
      setReads((n) => n + 1);
    });
  }, []);

  const actions: CalendarActions = {
    seedWeek: () => run(gieoTuanMacDinh(), (d) => cauGieoTuan(d.seeded, d.kept, d.skipped)),
    seedHolidays: () =>
      run(gieoNgayNghiLeMacDinh(year), (d) => cauGieoNgayLe(year, d.seeded, d.kept, d.skipped)),
    addShift: () => show({ kieu: "themCa" }, EMPTY_CALENDAR_DRAFT),
    editShift: (c) =>
      show(
        { kieu: "suaCa", ca: c },
        { ...EMPTY_CALENDAR_DRAFT, thu: String(c.weekday), batDau: nhanGio(c.start), ketThuc: nhanGio(c.end), ghiChu: c.note },
      ),
    deleteShift: (c) => show({ kieu: "xoaCa", ca: c }, EMPTY_CALENDAR_DRAFT),
    addHoliday: () => show({ kieu: "themNghi" }, EMPTY_CALENDAR_DRAFT),
    editHoliday: (n) => show({ kieu: "suaNghi", ngay: n }, { ...EMPTY_CALENDAR_DRAFT, ngay: n.date, ten: n.name }),
    deleteHoliday: (n) => show({ kieu: "xoaNghi", ngay: n }, EMPTY_CALENDAR_DRAFT),
    addSwapDay: () => show({ kieu: "themLamBu" }, EMPTY_CALENDAR_DRAFT),
    editSwapDay: (c) =>
      show(
        { kieu: "suaLamBu", ca: c },
        { ...EMPTY_CALENDAR_DRAFT, ngay: c.date, batDau: nhanGio(c.start), ketThuc: nhanGio(c.end), ten: c.name },
      ),
    deleteSwapDay: (c) => show({ kieu: "xoaLamBu", ca: c }, EMPTY_CALENDAR_DRAFT),
  };

  const submit = useCallback(() => {
    if (open === null || busy) return;

    // DELETE: the one check this screen makes itself — an empty reason is not sent (rule 7: the server
    // keeps the row and requires the reason).
    if (isDelete(open)) {
      const reason = draft.lyDo.trim();
      if (reason === "") {
        setLocalError(LOI_THIEU_LY_DO);
        return;
      }
      if (open.kieu === "xoaCa") run(xoaCaLamViec(open.ca.id, reason), () => DA_XOA_LICH);
      else if (open.kieu === "xoaNghi") run(xoaNgayNghiLe(open.ngay.id, reason), () => DA_XOA_LICH);
      else if (open.kieu === "xoaLamBu") run(xoaNgayLamBu(open.ca.id, reason), () => DA_XOA_LICH);
      return;
    }

    switch (open.kieu) {
      case "themCa":
        run(
          themCaLamViec({ weekday: Number(draft.thu), start: draft.batDau, end: draft.ketThuc, note: draft.ghiChu }),
          () => DA_LUU_LICH,
        );
        return;
      case "suaCa":
        run(
          suaCaLamViec(open.ca.id, {
            weekday: Number(draft.thu),
            start: draft.batDau,
            end: draft.ketThuc,
            note: draft.ghiChu,
          }),
          () => DA_LUU_LICH,
        );
        return;
      case "themNghi":
        run(themNgayNghiLe({ date: draft.ngay, name: draft.ten }), () => DA_LUU_LICH);
        return;
      case "suaNghi":
        run(suaNgayNghiLe(open.ngay.id, { date: draft.ngay, name: draft.ten }), () => DA_LUU_LICH);
        return;
      case "themLamBu":
        run(
          themNgayLamBu({ date: draft.ngay, start: draft.batDau, end: draft.ketThuc, name: draft.ten }),
          () => DA_LUU_LICH,
        );
        return;
      case "suaLamBu":
        run(
          suaNgayLamBu(open.ca.id, { date: draft.ngay, start: draft.batDau, end: draft.ketThuc, name: draft.ten }),
          () => DA_LUU_LICH,
        );
        return;
    }
  }, [busy, draft, open, run]);

  return (
    <WorkingCalendarView
      data={{
        week,
        holidays: holidays !== null && holidays.year === year ? holidays.result : null,
        swapDays: swapDays !== null && swapDays.year === year ? swapDays.result : null,
      }}
      year={year}
      baseYear={baseYear}
      setYear={setYear}
      canWrite={canWrite}
      actions={actions}
      form={
        open === null ? null : (
          <CalendarForm
            open={open}
            draft={draft}
            setDraft={setDraft}
            localError={localError}
            serverError={serverError}
            busy={busy}
            onSubmit={submit}
            onCancel={close}
          />
        )
      }
      formGroup={calendarGroupOf(open)}
      outsideFormError={open === null ? serverError : ""}
      busy={busy}
    />
  );
}

/* ---- presentation ----------------------------------------------------------------------------- */

/**
 * Everything visible on the tab, PURE PRESENTATION — exported so a test renders it with
 * `react-dom/server`, no simulated browser: a decision tested only in a pure module can still fail to
 * reach the page, and blanking the most important sentence of the screen would keep every test green.
 */
export function WorkingCalendarView({
  data,
  year,
  baseYear,
  setYear,
  canWrite,
  actions,
  form,
  formGroup,
  outsideFormError,
  busy,
}: {
  data: CalendarData;
  year: number;
  baseYear: number;
  setYear: (y: number) => void;
  canWrite: boolean;
  actions: CalendarActions;
  /** The open form, or `null`. It opens INSIDE its table's section, above the table — never a dialog. */
  form: ReactNode;
  /** Which table owns that form — passed as a value, never read back from `form`'s props. */
  formGroup: CalendarGroup;
  /** The error of an action that opens no form (the two seed buttons). */
  outsideFormError: string;
  busy: boolean;
}) {
  // Only the weekly table is REQUIRED here (the SLA half of the block lives on "Thời hạn xử lý").
  const block = khoiCanhBao("chuaBiet", tinhTrangBang(data.week));

  return (
    <section className="flex min-w-0 flex-col gap-6 [&>*]:my-0" aria-labelledby="working-calendar-title">
      <h2 id="working-calendar-title" className="an-thi-giac">
        Lịch làm việc của đơn vị
      </h2>
      <p className="text-ink-muted text-[12.5px]">{DAN_LICH_LAM_VIEC}</p>

      <KhoiChuaKhai khoi={block} coQuyenGhi={canWrite} dangGui={busy} onSeedWeek={actions.seedWeek} />

      {outsideFormError !== "" && <InlineError>{outsideFormError}</InlineError>}

      <BangGioLamViec
        kq={data.week}
        coQuyenGhi={canWrite}
        dangGui={busy}
        actions={actions}
        form={formGroup === "week" ? form : null}
      />

      {/* The year of the two per-year tables — the shared field pattern (11.5px label, `selectCls`), as
          the Trường bản đồ group picker. Not `ChonNam`: its legacy `.chon-nam` frame clipped "2026" at
          the bottom here (VALIDATE 08/10/2026), and it is shared with Giải ngân, so it is not restyled.
          Same options (`danhSachNam` around the year read ONCE from the clock) and the same behaviour:
          the year is always visible, never a hidden default (`lib/nam.ts`). */}
      <ConfigField label="Năm của lịch nghỉ lễ và làm bù" htmlFor="nam-lich-lam-viec" className="w-48 max-w-full">
        <select
          id="nam-lich-lam-viec"
          className={formSelectCls}
          value={year}
          onChange={(e) => setYear(Number(e.target.value))}
        >
          {danhSachNam(baseYear).map((n) => (
            <option key={n} value={n}>
              {n}
            </option>
          ))}
        </select>
      </ConfigField>

      <BangNgayNghi
        kq={data.holidays}
        nam={year}
        coQuyenGhi={canWrite}
        dangGui={busy}
        actions={actions}
        form={formGroup === "holidays" ? form : null}
      />
      <BangNgayLamBu
        kq={data.swapDays}
        nam={year}
        coQuyenGhi={canWrite}
        dangGui={busy}
        actions={actions}
        form={formGroup === "swapDays" ? form : null}
      />
    </section>
  );
}

/**
 * The "commune has not finished the required setup" block — the most valuable part of both tabs.
 *
 * IT STATES THE CONSEQUENCE FIRST, THE MISSING LIST SECOND. "No deadline data yet" reads as a job for
 * another day; "cannot register incoming documents or receive petitions" is a job for now.
 *
 * ABSENT WHEN THE TABLES HAVE ROWS, and that negative half carries the weight: an alarm that also shows
 * at a fully configured commune is one people learn to ignore — then ignore the time it is right.
 *
 * Shared by the two tabs since ADR 0079 D2: each passes only ITS table's state, and only its own seed
 * handler — "Thời hạn xử lý" the SLA table and `onSeedSla`, "Lịch làm việc" the weekly table and
 * `onSeedWeek`. A button is drawn only for a missing table whose handler was passed.
 */
// vi-name-ok: existing component moved from tab-thoi-han-xu-ly.tsx (rule 12 inv 3, ADR 0079 D2)
export function KhoiChuaKhai({
  khoi,
  coQuyenGhi,
  dangGui,
  onSeedSla,
  onSeedWeek,
}: {
  khoi: KhoiCanhBao;
  coQuyenGhi: boolean;
  dangGui: boolean;
  onSeedSla?: () => void;
  onSeedWeek?: () => void;
}) {
  if (!khoi.hien) return null;
  const seedSla = khoi.thieuThoiHan && onSeedSla !== undefined;
  const seedWeek = khoi.thieuLichTuan && onSeedWeek !== undefined;

  return (
    <div className="khoi-chua-khai" role="alert" aria-labelledby="tieu-de-chua-khai">
      <h3 id="tieu-de-chua-khai">{KHOI_CHUA_KHAI_TIEU_DE}</h3>
      <p className="hau-qua">{KHOI_CHUA_KHAI_HAU_QUA}</p>
      <ul>
        {khoi.thieuThoiHan && <li>{KHOI_CHUA_KHAI_THIEU_THOI_HAN}</li>}
        {khoi.thieuLichTuan && <li>{KHOI_CHUA_KHAI_THIEU_LICH_TUAN}</li>}
      </ul>

      {/* ONE BUTTON PER MISSING TABLE. A button for a full table does nothing and makes the reader
          doubt the whole block. Without the key no button is drawn — but the consequence sentence
          STILL shows: an account without the key also needs to know why the commune cannot receive
          anything, to go and find the right person. */}
      {coQuyenGhi && (seedSla || seedWeek) && (
        // One solid button per block: the second seed button turns secondary when both are drawn.
        <p className="cum-nut flex flex-wrap gap-2">
          {seedSla && (
            <Button
              type="button"
              variant="primary"
              icon={<Sprout aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              disabled={dangGui}
              onClick={onSeedSla}
            >
              {NUT_GIEO_THOI_HAN}
            </Button>
          )}
          {seedWeek && (
            <Button
              type="button"
              variant={seedSla ? "secondary" : "primary"}
              icon={<Sprout aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              disabled={dangGui}
              onClick={onSeedWeek}
            >
              {NUT_GIEO_TUAN}
            </Button>
          )}
        </p>
      )}
      <p className="ghi-chu">{GHI_CHU_SAU_KHI_GIEO}</p>
    </div>
  );
}

/** A sentence the server wrote (or the one check of this screen), in place, in the spec's error type. */
function InlineError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="text-danger m-0 text-[12px] font-medium">
      {children}
    </p>
  );
}

/** Section head: the spec's section title, an optional one-line explanation, the buttons on the right. */
function SectionHead({
  id,
  title,
  hint,
  children,
}: {
  id: string;
  title: string;
  hint?: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="min-w-0 flex-1 basis-60">
        <h3 id={id} className="text-navy m-0 text-[13px] font-bold">
          {title}
        </h3>
        {hint !== undefined && <p className="text-ink-muted m-0 mt-1 text-[12px]">{hint}</p>}
      </div>
      {children !== undefined && <div className="ml-auto flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  );
}

/** First load / failed read of one table. `null` result = loading; a failure shows the server's words. */
function TableState({ result, loading }: { result: KetQua<unknown> | null; loading: string }) {
  if (result === null) return <ConfigLoading label={loading} />;
  if (result.ok) return null;
  return <ErrorState role="alert" title="Chưa tải được bảng này" message={result.thongBao} className="py-6" />;
}

/** The "+ add" button of a section head (spec: `Button size="sm"` with Plus, the default navy). */
function AddButton({ label, onClick, disabled }: { label: string; onClick: () => void; disabled: boolean }) {
  return (
    <Button
      type="button"
      variant="primary"
      size="sm"
      className={SMALL_BUTTON_CLASS}
      icon={<Plus aria-hidden="true" focusable="false" className="size-3.5" />}
      disabled={disabled}
      onClick={onClick}
    >
      {label}
    </Button>
  );
}

/** A seed button of a section head — outline, beside the add button. */
function SeedButton({ label, onClick, disabled }: { label: string; onClick: () => void; disabled: boolean }) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className={SMALL_BUTTON_CLASS}
      icon={<Sprout aria-hidden="true" focusable="false" className="size-3.5" />}
      disabled={disabled}
      onClick={onClick}
    >
      {label}
    </Button>
  );
}

/** Edit + delete of one row: small outline icon buttons, their words in `title` and the accessible name. */
function EditDeleteActions({
  editLabel,
  deleteLabel,
  onEdit,
  onDelete,
}: {
  editLabel: string;
  deleteLabel: string;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <RowActions>
      <Button type="button" variant="outline" size="sm" title={editLabel} aria-label={editLabel} onClick={onEdit}>
        <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
      </Button>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="text-danger"
        title={deleteLabel}
        aria-label={deleteLabel}
        onClick={onDelete}
      >
        <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
      </Button>
    </RowActions>
  );
}

const ACTIONS_HEAD = (
  <th scope="col">
    <span className="an-thi-giac">Thao tác</span>
  </th>
);

// vi-name-ok: existing component moved from tab-thoi-han-xu-ly.tsx (rule 12 inv 3, ADR 0079 D2)
export function BangGioLamViec({
  kq,
  coQuyenGhi,
  dangGui,
  actions,
  form,
}: {
  kq: KetQua<identity_danhSachCaLamViecRa> | null;
  coQuyenGhi: boolean;
  dangGui: boolean;
  actions: CalendarActions;
  form: ReactNode;
}) {
  const loaded = kq !== null && kq.ok;
  return (
    <section className="flex min-w-0 flex-col gap-3" aria-labelledby="working-hours-title">
      <SectionHead id="working-hours-title" title="Giờ làm việc trong tuần" hint={GIAI_THICH_CA}>
        {coQuyenGhi && loaded && (
          <>
            {/* The weekly seed button here is only for a table that HAS rows (patching a partial set).
                An empty table gets it in the alert block above — one job, one button, one place. */}
            {kq.duLieu.items.length > 0 && <SeedButton label={NUT_GIEO_TUAN} onClick={actions.seedWeek} disabled={dangGui} />}
            <AddButton label={NUT_THEM_CA} onClick={actions.addShift} disabled={dangGui} />
          </>
        )}
      </SectionHead>

      {loaded &&
        kq.duLieu.problems.map((v, i) => <InlineError key={`${v.kind}-${v.weekday ?? "chung"}-${i}`}>{v.message}</InlineError>)}

      {form}

      <TableState result={kq} loading="Đang tải giờ làm việc…" />

      {/* An EMPTY week draws no table: the alert block above says what that means and seeds it. */}
      {loaded && kq.duLieu.items.length > 0 && (
        <ConfigTable
          label="Giờ làm việc trong tuần"
          caption="Các ca làm việc thông thường của đơn vị theo từng thứ trong tuần"
        >
          <thead>
            <tr>
              <th scope="col">Thứ</th>
              <th scope="col">Ca làm việc</th>
              <th scope="col">Ghi chú</th>
              {coQuyenGhi && ACTIONS_HEAD}
            </tr>
          </thead>
          <tbody>
            {kq.duLieu.items.map((c) => (
              <tr key={c.id}>
                <td className="text-navy font-medium">{tenThu(c.weekday)}</td>
                <td>{nhanCa(c.start, c.end)}</td>
                <td>{c.note === "" ? <span className="text-ink-muted">—</span> : c.note}</td>
                {coQuyenGhi && (
                  <td>
                    <EditDeleteActions
                      editLabel={`${NUT_SUA} ca ${tenThu(c.weekday)} ${nhanCa(c.start, c.end)}`}
                      deleteLabel={`${NUT_XOA} ca ${tenThu(c.weekday)} ${nhanCa(c.start, c.end)}`}
                      onEdit={() => actions.editShift(c)}
                      onDelete={() => actions.deleteShift(c)}
                    />
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </ConfigTable>
      )}
    </section>
  );
}

// vi-name-ok: existing component moved from tab-thoi-han-xu-ly.tsx (rule 12 inv 3, ADR 0079 D2)
export function BangNgayNghi({
  kq,
  nam,
  coQuyenGhi,
  dangGui,
  actions,
  form,
}: {
  kq: KetQua<identity_danhSachNgayNghiLeRa> | null;
  nam: number;
  coQuyenGhi: boolean;
  dangGui: boolean;
  actions: CalendarActions;
  form: ReactNode;
}) {
  const loaded = kq !== null && kq.ok;
  const columns = coQuyenGhi ? 3 : 2;
  return (
    <section className="flex min-w-0 flex-col gap-3" aria-labelledby="holidays-title">
      <SectionHead id="holidays-title" title="Ngày nghỉ lễ — đơn vị KHÔNG làm việc">
        {coQuyenGhi && loaded && (
          <>
            <SeedButton label={NUT_GIEO_NGAY_LE} onClick={actions.seedHolidays} disabled={dangGui} />
            <AddButton label={NUT_THEM_NGAY_NGHI} onClick={actions.addHoliday} disabled={dangGui} />
          </>
        )}
      </SectionHead>

      {/* ⚠ THE SENTENCE OWED TO WHOEVER PRESSES SEED, standing beside the button. The route seeds only
          FOUR fixed solar-calendar days and `seeded: 4` reads as "done" — while Tết, Giỗ Tổ Hùng Vương
          and the day next to 02/9 are still missing. Unsaid, the commune believes it is complete and
          every deadline falling over Tết is counted wrong with nothing reporting it. */}
      {loaded && (
        <Notice tone="neutral" icon={TriangleAlert}>
          {CON_THIEU_NGAY_LE}
        </Notice>
      )}

      {form}

      <TableState result={kq} loading="Đang tải ngày nghỉ lễ…" />

      {loaded && (
        <ConfigTable
          label={`Ngày nghỉ lễ năm ${nam}`}
          caption={`Những ngày đơn vị đóng cửa trong năm ${nam}, gồm cả lễ quốc gia lẫn lễ địa phương`}
        >
          <thead>
            <tr>
              <th scope="col">Ngày</th>
              <th scope="col">Tên</th>
              {coQuyenGhi && ACTIONS_HEAD}
            </tr>
          </thead>
          <tbody>
            {kq.duLieu.items.length === 0 ? (
              <EmptyRow colSpan={columns}>
                Năm {nam} chưa khai ngày nghỉ lễ nào, nên mọi thời hạn của năm này đang được đếm như thể đơn vị
                không nghỉ ngày nào.
              </EmptyRow>
            ) : (
              kq.duLieu.items.map((n) => (
                <tr key={n.id}>
                  <td>{nhanNgay(n.date)}</td>
                  <td className="text-navy font-medium">{n.name}</td>
                  {coQuyenGhi && (
                    <td>
                      <EditDeleteActions
                        editLabel={`${NUT_SUA} ngày nghỉ ${nhanNgay(n.date)}`}
                        deleteLabel={`${NUT_XOA} ngày nghỉ ${nhanNgay(n.date)}`}
                        onEdit={() => actions.editHoliday(n)}
                        onDelete={() => actions.deleteHoliday(n)}
                      />
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </ConfigTable>
      )}
    </section>
  );
}

// vi-name-ok: existing component moved from tab-thoi-han-xu-ly.tsx (rule 12 inv 3, ADR 0079 D2)
export function BangNgayLamBu({
  kq,
  nam,
  coQuyenGhi,
  dangGui,
  actions,
  form,
}: {
  kq: KetQua<identity_danhSachCaLamBuRa> | null;
  nam: number;
  coQuyenGhi: boolean;
  dangGui: boolean;
  actions: CalendarActions;
  form: ReactNode;
}) {
  const loaded = kq !== null && kq.ok;
  const columns = coQuyenGhi ? 4 : 3;
  return (
    <section className="flex min-w-0 flex-col gap-3" aria-labelledby="swap-days-title">
      {/* NO SEED BUTTON HERE, and that is the answer rather than a gap: a swap day exists only because
          the Prime Minister announces it for one year, so there is no default set to seed. */}
      <SectionHead id="swap-days-title" title="Ngày làm bù — đơn vị CÓ làm việc">
        {coQuyenGhi && loaded && (
          <AddButton label={NUT_THEM_NGAY_LAM_BU} onClick={actions.addSwapDay} disabled={dangGui} />
        )}
      </SectionHead>

      {loaded &&
        kq.duLieu.problems.map((v, i) => <InlineError key={`${v.kind}-${v.date}-${i}`}>{v.message}</InlineError>)}

      {form}

      <TableState result={kq} loading="Đang tải ngày làm bù…" />

      {loaded && (
        <ConfigTable
          label={`Ngày làm bù năm ${nam}`}
          caption={`Những ngày đơn vị vẫn làm việc trong năm ${nam} dù lịch tuần nói không, kèm giờ làm của chính ngày đó`}
        >
          <thead>
            <tr>
              <th scope="col">Ngày</th>
              <th scope="col">Ca làm việc</th>
              <th scope="col">Theo thông báo</th>
              {coQuyenGhi && ACTIONS_HEAD}
            </tr>
          </thead>
          <tbody>
            {kq.duLieu.items.length === 0 ? (
              <EmptyRow colSpan={columns}>
                Năm {nam} không có ngày làm bù nào. Phần lớn các năm là như vậy — khác hẳn giờ làm việc trong tuần
                để trống, vốn nghĩa là đơn vị không có giờ làm việc nào.
              </EmptyRow>
            ) : (
              kq.duLieu.items.map((c) => (
                <tr key={c.id}>
                  <td className="text-navy font-medium">{nhanNgay(c.date)}</td>
                  <td>{nhanCa(c.start, c.end)}</td>
                  <td>{c.name}</td>
                  {coQuyenGhi && (
                    <td>
                      <EditDeleteActions
                        editLabel={`${NUT_SUA} ca làm bù ${nhanNgay(c.date)}`}
                        deleteLabel={`${NUT_XOA} ca làm bù ${nhanNgay(c.date)}`}
                        onEdit={() => actions.editSwapDay(c)}
                        onDelete={() => actions.deleteSwapDay(c)}
                      />
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </ConfigTable>
      )}
    </section>
  );
}

/* ---- form ------------------------------------------------------------------------------------- */

const CALENDAR_FORM_TITLE: Record<NonNullable<CalendarOpenForm>["kieu"], string> = {
  themCa: "Thêm ca làm việc",
  suaCa: "Sửa ca làm việc",
  xoaCa: "Xoá ca làm việc",
  themNghi: "Thêm ngày nghỉ lễ",
  suaNghi: "Sửa ngày nghỉ lễ",
  xoaNghi: "Xoá ngày nghỉ lễ",
  themLamBu: "Thêm ca làm bù",
  suaLamBu: "Sửa ca làm bù",
  xoaLamBu: "Xoá ca làm bù",
};

/** Grid template from `sm:` up, per form: the fields, then the button cell (`auto`). */
const CALENDAR_FORM_COLUMNS: Record<NonNullable<CalendarOpenForm>["kieu"], string> = {
  themCa: "sm:grid-cols-[10rem_8rem_8rem_minmax(0,1fr)_auto]",
  suaCa: "sm:grid-cols-[10rem_8rem_8rem_minmax(0,1fr)_auto]",
  themNghi: "sm:grid-cols-[10rem_minmax(0,1fr)_auto]",
  suaNghi: "sm:grid-cols-[10rem_minmax(0,1fr)_auto]",
  themLamBu: "sm:grid-cols-[10rem_8rem_8rem_minmax(0,1fr)_auto]",
  suaLamBu: "sm:grid-cols-[10rem_8rem_8rem_minmax(0,1fr)_auto]",
  xoaCa: "sm:grid-cols-[minmax(0,1fr)_auto]",
  xoaNghi: "sm:grid-cols-[minmax(0,1fr)_auto]",
  xoaLamBu: "sm:grid-cols-[minmax(0,1fr)_auto]",
};

/**
 * One form for all nine calendar actions, as the spec's grey row above the table.
 *
 * PURE PRESENTATION: every value comes in through `draft`, every change goes out through `setDraft`, the
 * check lives with the caller — so the two branches nobody sees while developing ("delete reason still
 * empty" and "the server just refused") render with `react-dom/server`.
 *
 * DELETE KEEPS ITS REASON STEP (rule 7: the server keeps the row and requires why): the same row asks
 * for the reason, under the warning that the start time / date stays reserved for good.
 */
export function CalendarForm({
  open,
  draft,
  setDraft,
  localError,
  serverError,
  busy,
  onSubmit,
  onCancel,
}: {
  open: NonNullable<CalendarOpenForm>;
  draft: CalendarDraft;
  setDraft: (d: CalendarDraft) => void;
  localError: string;
  serverError: string;
  busy: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const deleting = isDelete(open);
  const adding = open.kieu === "themCa" || open.kieu === "themNghi" || open.kieu === "themLamBu";
  const swapDayForm = open.kieu === "themLamBu" || open.kieu === "suaLamBu";
  const submitLabel = deleting ? NUT_XAC_NHAN_XOA : adding ? "Thêm" : NUT_LUU;

  return (
    <ConfigFormRow
      columns={CALENDAR_FORM_COLUMNS[open.kieu]}
      aria-label={CALENDAR_FORM_TITLE[open.kieu]}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      {deleting && (
        // The spec's grey notice box (00 §4) — white on the grey row.
        <p className="border-line bg-surface text-ink col-span-full m-0 flex items-start gap-2 rounded-[10px] border border-solid px-3.5 py-2.5 text-[12px]">
          <TriangleAlert aria-hidden="true" focusable="false" className="text-ink-muted mt-0.5 size-4 shrink-0" />
          <span>
            {open.kieu === "xoaCa"
              ? CANH_BAO_XOA_CA
              : open.kieu === "xoaNghi"
                ? CANH_BAO_XOA_NGAY_NGHI
                : CANH_BAO_XOA_NGAY_LAM_BU}
          </span>
        </p>
      )}

      {(open.kieu === "themCa" || open.kieu === "suaCa") && (
        <>
          <ConfigField label={O_THU} htmlFor="o-thu">
            {/* A SELECT, not a number box: the contract counts weekdays ISO 8601 (1 = Monday … 7 =
                Sunday), unlike JavaScript's `Date.getDay()`. A number box invites "2" for Monday, and
                one day off reports no error. */}
            <select
              id="o-thu"
              name="thu"
              className={formSelectCls}
              value={draft.thu}
              onChange={(e) => setDraft({ ...draft, thu: e.target.value })}
            >
              {[1, 2, 3, 4, 5, 6, 7].map((t) => (
                <option key={t} value={t}>
                  {tenThu(t)}
                </option>
              ))}
            </select>
          </ConfigField>
          <TimeFields draft={draft} setDraft={setDraft} />
          <ConfigField label={O_GHI_CHU_CA} htmlFor="o-ghi-chu-ca">
            <input
              id="o-ghi-chu-ca"
              name="ghiChu"
              className={formInputCls}
              value={draft.ghiChu}
              onChange={(e) => setDraft({ ...draft, ghiChu: e.target.value })}
            />
          </ConfigField>
        </>
      )}

      {(open.kieu === "themNghi" || open.kieu === "suaNghi") && (
        <>
          <DateField draft={draft} setDraft={setDraft} />
          <ConfigField label={O_TEN_NGAY_NGHI} htmlFor="o-ten-lich">
            <input
              id="o-ten-lich"
              name="ten"
              className={formInputCls}
              value={draft.ten}
              onChange={(e) => setDraft({ ...draft, ten: e.target.value })}
            />
          </ConfigField>
        </>
      )}

      {swapDayForm && (
        <>
          <DateField draft={draft} setDraft={setDraft} />
          <TimeFields draft={draft} setDraft={setDraft} />
          <ConfigField label={O_TEN_NGAY_LAM_BU} htmlFor="o-ten-lich">
            <input
              id="o-ten-lich"
              name="ten"
              className={formInputCls}
              value={draft.ten}
              onChange={(e) => setDraft({ ...draft, ten: e.target.value })}
              aria-describedby="giai-thich-ten-lam-bu"
            />
          </ConfigField>
        </>
      )}

      {deleting && (
        <ConfigField label={O_LY_DO_XOA} htmlFor="o-ly-do-xoa-lich">
          {/* `required` is the browser's reminder, NOT the check: it misses a field of spaces and can be
              switched off. The real check runs before sending. */}
          <input
            id="o-ly-do-xoa-lich"
            name="lyDo"
            required
            className={formInputCls}
            value={draft.lyDo}
            onChange={(e) => setDraft({ ...draft, lyDo: e.target.value })}
            aria-invalid={localError !== ""}
            aria-describedby="giai-thich-ly-do-xoa"
          />
        </ConfigField>
      )}

      <div className="flex gap-2">
        <Button type="submit" variant={deleting ? "danger" : "primary"} disabled={busy} aria-busy={busy}>
          <BusyLabel busy={busy} label={submitLabel} busyText={deleting ? BUSY_DELETING : BUSY_SAVING} />
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={busy}>
          {NUT_HUY}
        </Button>
      </div>

      {swapDayForm && (
        <p id="giai-thich-ten-lam-bu" className="text-ink-muted col-span-full m-0 text-[11.5px]">
          Ghi rõ thông báo mà ngày này thực hiện, ví dụ “Làm bù nghỉ Tết theo Thông báo số …”. Chính câu đó là câu
          trả lời khi có người hỏi vì sao một hạn chạy qua ngày thứ Bảy.
        </p>
      )}
      {deleting && (
        <p id="giai-thich-ly-do-xoa" className="text-ink-muted col-span-full m-0 text-[11.5px]">
          {GIAI_THICH_LY_DO_XOA}
        </p>
      )}

      {/* TWO ERROR REGIONS, NOT ONE. The local one says "a field is missing"; the server's says "that
          request was refused" — including the 409 explaining why a deleted shift's start time still
          blocks the way. Merged, the second overwrites the first exactly when both need reading. */}
      {localError !== "" && (
        <div className="col-span-full">
          <InlineError>{localError}</InlineError>
        </div>
      )}
      {serverError !== "" && (
        <div className="col-span-full">
          <InlineError>{serverError}</InlineError>
        </div>
      )}
    </ConfigFormRow>
  );
}

/**
 * The two time fields. `type="time"`, NOT free text: the server wants exactly `HH:MM` or `HH:MM:SS` and
 * refuses "7:3" rather than reading 07:03; a text box invites "7h30" and a refusal that cannot say how
 * to type it.
 */
function TimeFields({ draft, setDraft }: { draft: CalendarDraft; setDraft: (d: CalendarDraft) => void }) {
  return (
    <>
      <ConfigField label={O_GIO_BAT_DAU} htmlFor="o-bat-dau">
        <input
          id="o-bat-dau"
          name="batDau"
          type="time"
          className={formInputCls}
          value={draft.batDau}
          onChange={(e) => setDraft({ ...draft, batDau: e.target.value })}
        />
      </ConfigField>
      <ConfigField label={O_GIO_KET_THUC} htmlFor="o-ket-thuc">
        <input
          id="o-ket-thuc"
          name="ketThuc"
          type="time"
          className={formInputCls}
          value={draft.ketThuc}
          onChange={(e) => setDraft({ ...draft, ketThuc: e.target.value })}
        />
      </ConfigField>
    </>
  );
}

/**
 * The date field. `type="date"` emits exactly `YYYY-MM-DD`, the contract's shape; a free text box invites
 * "2/9/2026", and one holiday off by a day is a deadline counted through a day the office is closed.
 */
function DateField({ draft, setDraft }: { draft: CalendarDraft; setDraft: (d: CalendarDraft) => void }) {
  return (
    <ConfigField label={O_NGAY} htmlFor="o-ngay-lich">
      <input
        id="o-ngay-lich"
        name="ngay"
        type="date"
        className={formInputCls}
        value={draft.ngay}
        onChange={(e) => setDraft({ ...draft, ngay: e.target.value })}
      />
    </ConfigField>
  );
}
