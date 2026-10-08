"use client";

import { CloudOff, Plus, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardFooter } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { bangTraTuKetQua, traTen } from "@/features/cau-hinh/tra-danh-muc";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { TRANG_DAU, coTrangTruoc } from "@/features/cau-hinh/ngan-xep-con-tro";
import type { NganXepConTro } from "@/features/cau-hinh/ngan-xep-con-tro";
import { danhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import { usePhien } from "@/features/phien/phien-hien-tai";
import {
  getCitizenLetter,
  getCitizenLetterLog,
  listCitizenLetters,
  type LetterListFilter,
  type LetterScope,
} from "@/lib/api/citizen-letters";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_citizenLetterItemOut,
  documents_citizenLetterOut,
  documents_letterLogOut,
  identity_danhBaChonNguoiRa, // vi-name-ok: generated contract type, imported not declared (rule 12 inv 3)
  page_Result_documents_citizenLetterItemOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { HeaderOr, LetterImportButton } from "./document-pending";
import { Glyph, plainFrame, type RegisterFrame } from "./document-ui";
import {
  DUE_TONE_CLASS,
  LETTER_STATUS_GROUPS,
  WITHHELD_SENDER,
  WITHHELD_SUMMARY,
  canBookLetters,
  canRaiseLetterTask,
  daysOpenView,
  dueLabel,
  isPastDue,
  letterDate,
  letterNumber,
  letterTypeLabel,
  mayWorkOnLetter,
  senderView,
  stageDueAt,
} from "./letter-display";
import { LetterDrawer } from "./letter-drawer";
import { LETTER_ENTRY_SUBMIT, LetterEntryDialog } from "./letter-entry-dialog";
import { LetterImportDialog } from "./letter-import-dialog";
import { LetterScopeFilter, LetterStatusBadge } from "./letter-ui";
import { FilterSelect, doiLocVeTrangDau } from "./loc-so-van-ban";
import { DieuHuongTrang, REGISTER_HEAD_ROW, REGISTER_ROW } from "./so-van-ban-den";

/** The prototype's words (`DocumentWorkspace.tsx:264-267`, `PetitionTable.tsx:25`). */
export const LETTER_REGISTER_INTRO =
  "Sổ theo dõi đơn khiếu nại, tố cáo, kiến nghị và đề nghị của công dân. Hệ thống nhắc khi một người " +
  "gửi lại đơn có nội dung tương tự.";
export const LETTER_REGISTER_EMPTY = "Sổ đơn thư chưa có bản ghi nào.";
/** The merged-duplicate line under a row's summary (`PetitionTable.tsx:78-82`). */
export const MERGED_DUPLICATE = "Đã gộp vì trùng đơn trước";

/** `id` of a row's number button — where focus returns when the drawer closes. Only the opaque id. */
export function letterRowButtonId(id: string): string {
  return `xem-don-thu-${id}`;
}

/** The four filters of the second row (`DocumentWorkspace.tsx:276-364`). */
export type LetterFilters = {
  receivedFrom: string;
  receivedTo: string;
  holdingUnit: string;
  assignee: string;
  /** ADR 0084's display group (`status_group`) — the prototype's status filter. */
  statusGroup: string;
};

export const NO_FILTERS: LetterFilters = { receivedFrom: "", receivedTo: "", holdingUnit: "", assignee: "", statusGroup: "" };

/** "Bỏ {n} bộ lọc" counts the second row only — the scope is a view, not a filter (prototype :81-83). */
export function activeFilterCount(f: LetterFilters): number {
  return Object.values(f).filter((v) => v !== "").length;
}

/** "Hôm nay" as the browser's date input writes it — the upper bound of both date inputs. */
function todayISO(): string {
  const d = new Date();
  const two = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

/**
 * `Đơn thư công dân` — the citizen-letter register (ADR 0078 #2), wired to `GET /api/v1/citizen-letters`.
 * The prototype's tab (`DocumentWorkspace.tsx:261-379`, `PetitionTable.tsx`): scope + one line, the
 * four-filter row, the table in a card; the booking dialog and the drawer open over it.
 *
 * READS ONCE PER SCREEN: the org-unit catalogue and the staff directory (skills/load-data-once). Every
 * write re-reads from the server instead of patching rows here.
 *
 * NO GATE ON THE TABLE: an account without `petition.read` gets the server's 403 sentence in the
 * table, verbatim — a client gate guessing it would be a second place that can disagree with the
 * server. The WRITE controls are gated by key (convenience; the server checks every call).
 */
export function LetterRegister({
  frame = plainFrame,
  onBooked,
}: {
  frame?: RegisterFrame;
  /** A letter was booked (by hand or from Excel) — the workspace re-reads the tab's count. */
  onBooked?: () => void;
} = {}) {
  const [scope, setScope] = useState<LetterScope>("all");
  const [filters, setFilters] = useState<LetterFilters>(NO_FILTERS);
  const [stack, setStack] = useState<NganXepConTro>(TRANG_DAU);
  const [reads, setReads] = useState(0);
  const [result, setResult] = useState<{
    query: LetterListFilter;
    answer: KetQua<page_Result_documents_citizenLetterItemOut>;
  } | null>(null);
  const [units, setUnits] = useState<BangTraDanhMuc>({ pha: "dangDoc" });
  const [directory, setDirectory] = useState<DanhBaTheoMa | null>(null);
  const [staff, setStaff] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);

  const [entry, setEntry] = useState<number | null>(null);
  // Bumped per opening, as the dialog's `key`: a new opening is a new attempt (file, check, key).
  const [importing, setImporting] = useState<number | null>(null);
  const [openId, setOpenId] = useState<string | null>(null);
  const [drawerReads, setDrawerReads] = useState(0);
  const [drawer, setDrawer] = useState<{
    id: string;
    letter: KetQua<documents_citizenLetterOut>;
    log: KetQua<documents_letterLogOut>;
  } | null>(null);

  const session = usePhien();
  const permissions = session !== null && session.ok ? session.duLieu.permissions : [];
  const staffCode = session !== null && session.ok ? session.duLieu.staff.code : "";
  const canBook = canBookLetters(permissions);
  const canRaiseTask = canRaiseLetterTask(permissions);

  const query = useMemo<LetterListFilter>(
    () => ({
      scope,
      receivedFrom: filters.receivedFrom,
      receivedTo: filters.receivedTo,
      holdingUnit: filters.holdingUnit,
      assignee: filters.assignee,
      statusGroup: filters.statusGroup,
      cursor: stack.hienTai,
    }),
    [scope, filters, stack],
  );

  useEffect(() => {
    let dropped = false;
    listCitizenLetters(query).then((answer) => {
      if (!dropped) setResult({ query, answer });
    });
    return () => {
      dropped = true;
    };
  }, [query, reads]);

  useEffect(() => {
    let dropped = false;
    layDanhMucBoPhan().then((k) => {
      if (!dropped) setUnits(bangTraTuKetQua(k));
    });
    layDanhBaChonNguoi().then((k) => {
      if (dropped) return;
      // The raw answer too: the task dialog's `Người thực hiện` reads it (one read for the screen).
      setStaff(k);
      if (k.ok) setDirectory(danhBaTheoMa(k.duLieu.items));
    });
    return () => {
      dropped = true;
    };
  }, []);

  useEffect(() => {
    if (openId === null) return;
    let dropped = false;
    // ONE assignment for both reads: the drawer never shows a letter beside the log of another moment.
    Promise.all([getCitizenLetter(openId), getCitizenLetterLog(openId)]).then(([letter, log]) => {
      if (!dropped) setDrawer({ id: openId, letter, log });
    });
    return () => {
      dropped = true;
    };
  }, [openId, drawerReads]);

  const changeFilter = useCallback((apply: () => void) => doiLocVeTrangDau(apply, setStack), []);

  const closeDrawer = useCallback(() => {
    const id = openId;
    setOpenId(null);
    setDrawer(null);
    if (id !== null) requestAnimationFrame(() => document.getElementById(letterRowButtonId(id))?.focus());
  }, [openId]);

  const current = drawer !== null && drawer.id === openId ? drawer : null;
  const answer = result !== null && result.query === query ? result.answer : null;
  const now = new Date();
  const openLetter = current !== null && current.letter.ok ? current.letter.duLieu : null;

  const headerActions = canBook ? (
    <>
      <Button
        type="button"
        variant="primary"
        className="h-10 justify-center px-4 text-[13px]"
        icon={<Glyph icon={Plus} />}
        aria-haspopup="dialog"
        onClick={() => setEntry((n) => (n ?? 0) + 1)}
      >
        {LETTER_ENTRY_SUBMIT}
      </Button>
      <HeaderOr />
      <LetterImportButton onClick={() => setImporting((n) => (n ?? 0) + 1)} />
    </>
  ) : null;

  return (
    <LetterRegisterView
      frame={frame}
      headerActions={headerActions}
      answer={answer}
      onReload={() => setReads((n) => n + 1)}
      now={now}
      scope={scope}
      onScope={(s) => changeFilter(() => setScope(s))}
      filters={filters}
      onFilters={(f) => changeFilter(() => setFilters(f))}
      units={units}
      directory={directory}
      stack={stack}
      goToPage={setStack}
      openId={openId}
      onOpen={(row) => {
        setOpenId(row.id);
        setDrawer(null);
      }}
      overlays={
        <>
          {entry !== null && (
            <LetterEntryDialog
              key={entry}
              units={units}
              onClose={() => setEntry(null)}
              onBooked={(letter) => {
                setEntry(null);
                toast.success(`Đã vào sổ đơn thư số ${letterNumber(letter.number, letter.year)}.`);
                setReads((n) => n + 1);
                onBooked?.();
              }}
            />
          )}
          {importing !== null && (
            <LetterImportDialog
              key={importing}
              onClose={() => setImporting(null)}
              onImported={() => {
                // Re-read, never splice: the server issued the numbers and the deadlines.
                setReads((n) => n + 1);
                onBooked?.();
              }}
            />
          )}
          {openId !== null && (
            <LetterDrawer
              key={openId}
              letter={current?.letter ?? null}
              log={current?.log ?? null}
              now={now}
              units={units}
              directory={directory}
              canBook={canBook}
              canRaiseTask={canRaiseTask}
              staff={staff}
              mayWork={openLetter !== null && mayWorkOnLetter(permissions, staffCode, openLetter.assignee_code ?? "")}
              onChanged={() => {
                setDrawerReads((n) => n + 1);
                setReads((n) => n + 1);
              }}
              onClose={closeDrawer}
            />
          )}
        </>
      }
    />
  );
}

/**
 * Everything the tab shows, PURE PRESENTATION — exported so a test can render it on fixed data.
 * The dialog and the drawer are SIBLINGS of the section (`overlays`): the section's `[&>*]:my-0` must
 * never reach a top-layer box.
 */
export function LetterRegisterView({
  frame = plainFrame,
  headerActions = null,
  answer,
  onReload,
  now,
  scope,
  onScope,
  filters,
  onFilters,
  units,
  directory,
  stack,
  goToPage,
  openId = null,
  onOpen,
  overlays = null,
}: {
  frame?: RegisterFrame;
  headerActions?: ReactNode;
  answer: KetQua<page_Result_documents_citizenLetterItemOut> | null;
  onReload?: () => void;
  now: Date;
  scope: LetterScope;
  onScope: (scope: LetterScope) => void;
  filters: LetterFilters;
  onFilters: (filters: LetterFilters) => void;
  units: BangTraDanhMuc;
  directory: DanhBaTheoMa | null;
  stack: NganXepConTro;
  goToPage: (stack: NganXepConTro) => void;
  openId?: string | null;
  onOpen: (row: documents_citizenLetterItemOut) => void;
  overlays?: ReactNode;
}) {
  const count = activeFilterCount(filters);
  // The prototype's red "{n} đơn đang quá hạn." — DERIVED from the rows on this page, against `now`;
  // omitted when there is none (every letter has no deadline today, ADR 0078 #3). Never a stored flag.
  const pastDue = answer !== null && answer.ok ? answer.duLieu.items.filter((row) => isPastDue(row, now)).length : 0;
  const subtitleExtra =
    pastDue > 0 ? <span className="font-semibold text-danger"> {pastDue} đơn đang quá hạn.</span> : undefined;
  const set = (patch: Partial<LetterFilters>) => onFilters({ ...filters, ...patch });

  const body = (
    <>
      <section className="flex min-w-0 flex-col gap-3 [&>*]:my-0" aria-labelledby="tieu-de-so-don-thu">
        <h2 id="tieu-de-so-don-thu" className="an-thi-giac">
          Sổ đơn thư công dân
        </h2>

        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <LetterScopeFilter value={scope} onChange={onScope} />
          <p className="m-0 max-w-lg text-[12.5px] text-ink-muted">{LETTER_REGISTER_INTRO}</p>
        </div>

        <div id="letter-filters" className="flex min-w-0 flex-wrap items-end gap-2.5">
          <DateFilter
            id="don-thu-tu-ngay"
            label="Từ ngày"
            value={filters.receivedFrom}
            max={filters.receivedTo === "" ? todayISO() : filters.receivedTo}
            onChange={(v) => set({ receivedFrom: v })}
          />
          <DateFilter
            id="don-thu-den-ngay"
            label="Đến ngày"
            value={filters.receivedTo}
            min={filters.receivedFrom === "" ? undefined : filters.receivedFrom}
            max={todayISO()}
            onChange={(v) => set({ receivedTo: v })}
          />
          <FilterSelect id="don-thu-loc-don-vi" label="Lọc theo đơn vị chủ quản" value={filters.holdingUnit} onChange={(v) => set({ holdingUnit: v })}>
            <option value="">Tất cả đơn vị</option>
            {units.pha === "xong" &&
              [...units.ten].map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
          </FilterSelect>
          <FilterSelect id="don-thu-loc-can-bo" label="Lọc theo cán bộ chủ quản" value={filters.assignee} onChange={(v) => set({ assignee: v })}>
            <option value="">Tất cả cán bộ</option>
            {directory !== null &&
              [...directory.values()].map((person) => (
                <option key={person.code} value={person.code}>
                  {person.full_name}
                  {person.position ? ` — ${person.position}` : ""}
                </option>
              ))}
          </FilterSelect>
          {/* The prototype's groups, in its order, without "Chờ phân công" (ADR 0084 #5). */}
          <FilterSelect id="don-thu-loc-trang-thai" label="Lọc theo trạng thái" value={filters.statusGroup} onChange={(v) => set({ statusGroup: v })}>
            <option value="">Tất cả trạng thái</option>
            {LETTER_STATUS_GROUPS.map((g) => (
              <option key={g.code} value={g.code}>
                {g.label}
              </option>
            ))}
          </FilterSelect>
          {count > 0 && (
            <button
              type="button"
              onClick={() => onFilters(NO_FILTERS)}
              className="mb-2 cursor-pointer border-0 bg-transparent p-0 [font-family:inherit] text-[12.5px] font-semibold text-brand hover:underline"
            >
              Bỏ {count} bộ lọc
            </button>
          )}
        </div>

        <Card className="bg-white">
          <LetterTable answer={answer} now={now} units={units} openId={openId} onOpen={onOpen} onReload={onReload} />
          {answer !== null && answer.ok && (answer.duLieu.has_more || coTrangTruoc(stack)) && (
            <CardFooter className="justify-end">
              <DieuHuongTrang
                nganXep={stack}
                conTroTiep={answer.duLieu.next_cursor}
                conTrangSau={answer.duLieu.has_more}
                diToiTrang={goToPage}
              />
            </CardFooter>
          )}
        </Card>
      </section>
      {overlays}
    </>
  );

  return frame(headerActions, body, subtitleExtra);
}

/** The prototype's labelled date input (`DocumentWorkspace.tsx:277-298`): 11.5px label, 36px box. */
function DateFilter({
  id,
  label,
  value,
  min,
  max,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  min?: string;
  max?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label htmlFor={id} className="flex flex-col text-[11.5px] text-ink-muted">
      {label}
      <input
        id={id}
        type="date"
        value={value}
        min={min}
        max={max}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 box-border block h-9 min-h-0 rounded-md border border-solid border-line bg-white px-3 [font-family:inherit] text-[12.5px] text-ink outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      />
    </label>
  );
}

/**
 * Column widths of the register — the prototype's table is `min-w-[900px]` with free-width columns,
 * which at 1280px (240px sidebar + 2×28px page padding ⇒ ~966px with the scrollbar) scrolled sideways
 * and pushed `Trạng thái` out of view. Owner request v2 §4.3: no horizontal scrollbar from 1280px,
 * `Đang giữ` / `Người gửi` at most three lines, `Nội dung` the flexible column cut with "…". So the
 * table is `table-fixed` with these widths (cell padding px-3 included), and `Nội dung` takes the rest.
 * `Trạng thái` is sized for the widest badge ("Lưu, không thụ lý"; the badge never wraps).
 * LOCAL on purpose: `REGISTER_TH` / `REGISTER_TD` are shared with Văn bản đến/đi, which keep theirs.
 */
const LETTER_COLUMNS: readonly { label: string; width?: string }[] = [
  { label: "Số", width: "w-[52px]" },
  { label: "Ngày nhận", width: "w-[92px]" },
  { label: "Người gửi", width: "w-[136px]" },
  { label: "Loại đơn", width: "w-[96px]" },
  { label: "Nội dung" },
  { label: "Đang giữ", width: "w-[124px]" },
  { label: "Số ngày xử lý", width: "w-[88px]" },
  { label: "Hạn giải quyết", width: "w-[104px]" },
  { label: "Trạng thái", width: "w-[160px]" },
];
/** The prototype's header cell (`PetitionTable.tsx:35-43`): no `nowrap`, so a long label wraps. */
const LETTER_TH = "px-3 py-2.5 align-bottom font-semibold";
const LETTER_TD = "px-3 py-2.5 align-top break-words";

/**
 * The register table — the prototype's nine columns in its order (`PetitionTable.tsx:33-45`). A row
 * opens the drawer; the number is a real button for the keyboard. Masking is the SERVER's: the phone
 * as sent, never a link; a denunciation's sender and summary are one fixed muted sentence each.
 *
 * Empty — with or without a filter — is the prototype's ONE sentence (`PetitionTable.tsx:22-27`).
 *
 * Hook-free: tests render it as a plain function.
 */
export function LetterTable({
  answer,
  now,
  units,
  openId = null,
  onOpen,
  onReload,
}: {
  answer: KetQua<page_Result_documents_citizenLetterItemOut> | null;
  now: Date;
  units: BangTraDanhMuc;
  openId?: string | null;
  onOpen: (row: documents_citizenLetterItemOut) => void;
  onReload?: () => void;
}) {
  if (answer === null) {
    // The prototype's petition first load: TWO 36px bars (`DocumentWorkspace.tsx:367-371`).
    return (
      <div className="flex flex-col gap-2 p-4">
        <p role="status" className="an-thi-giac">
          Đang tải sổ đơn thư…
        </p>
        <Skeleton className="h-9 w-full rounded-md" />
        <Skeleton className="h-9 w-full rounded-md" />
      </div>
    );
  }
  if (!answer.ok) {
    return (
      <EmptyState
        icon={CloudOff}
        tone="neutral"
        title="Chưa tải được sổ đơn thư"
        description={
          <span className="text-danger-600" role="alert">
            {answer.thongBao}
          </span>
        }
        action={
          onReload !== undefined ? (
            <Button type="button" variant="secondary" icon={<Glyph icon={RefreshCw} />} onClick={onReload}>
              Tải lại
            </Button>
          ) : undefined
        }
      />
    );
  }
  if (answer.duLieu.items.length === 0) {
    return (
      <p className="m-0 p-6 text-center text-[12.5px] text-ink-muted">{LETTER_REGISTER_EMPTY}</p>
    );
  }

  return (
    // `relative`: the scroller must contain its cells' visually-hidden spans, or the page scrolls
    // sideways (`BangVanBanDen`, measured 08/10/2026).
    <div className="relative overflow-x-auto" role="region" aria-label="Sổ đơn thư công dân" tabIndex={0}>
      {/* min-w: below it (narrow windows) the region scrolls instead of crushing `Nội dung` to nothing. */}
      <table className="w-full min-w-[940px] table-fixed border-collapse text-[12.5px]">
        <caption className="an-thi-giac">Các đơn thư đã vào sổ, đơn mới vào sổ trước</caption>
        <colgroup>
          {LETTER_COLUMNS.map((c) => (
            <col key={c.label} className={c.width} />
          ))}
        </colgroup>
        <thead>
          <tr className={REGISTER_HEAD_ROW}>
            {LETTER_COLUMNS.map(({ label }) => (
              <th key={label} scope="col" className={LETTER_TH}>
                {label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {answer.duLieu.items.map((row) => (
            <LetterRow key={row.id} row={row} now={now} units={units} open={row.id === openId} onOpen={onOpen} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function LetterRow({
  row,
  now,
  units,
  open,
  onOpen,
}: {
  row: documents_citizenLetterItemOut;
  now: Date;
  units: BangTraDanhMuc;
  open: boolean;
  onOpen: (row: documents_citizenLetterItemOut) => void;
}) {
  const number = letterNumber(row.number, row.year);
  const sender = senderView(row);
  const days = daysOpenView(row);
  // The drawer figure's rule (`stageDueAt`), so the row and the drawer never show two deadlines.
  const due = dueLabel(stageDueAt(row), row.is_closed, now);
  const merged = (row.related_letter_id ?? "") !== "";
  const unit = row.holding_unit_id ?? "";
  const unitLookup = traTen(units, unit);
  return (
    <tr
      // The prototype tints a late row; the tint is the SAME derived comparison as the deadline cell.
      className={cn(REGISTER_ROW, "cursor-pointer align-top", isPastDue(row, now) && "bg-danger/4", merged && "opacity-60")}
      onClick={(e) => {
        if ((e.target as Element).closest("button, a, input, select, textarea")) return;
        onOpen(row);
      }}
    >
      <td className={LETTER_TD}>
        <button
          type="button"
          id={letterRowButtonId(row.id)}
          aria-expanded={open}
          aria-haspopup="dialog"
          aria-label={`Xem chi tiết đơn số ${number}`}
          className="cursor-pointer border-0 bg-transparent p-0 [font-family:inherit] text-[12.5px] font-semibold text-navy tabular-nums hover:underline"
          onClick={() => onOpen(row)}
        >
          {row.number}
        </button>
      </td>
      <td className={cn(LETTER_TD, "whitespace-nowrap tabular-nums")}>{letterDate(row.received_date)}</td>
      <td className={LETTER_TD}>
        {sender.kind === "withheld" ? (
          <span className="text-[12px] text-ink-muted italic">{WITHHELD_SENDER}</span>
        ) : sender.kind === "unknown" ? (
          <span className="text-ink-muted">Không rõ người gửi</span>
        ) : (
          <>
            <span className="block font-semibold text-navy">{sender.name}</span>
            {sender.phone !== null && <span className="block text-[11px] text-ink-muted tabular-nums">{sender.phone}</span>}
          </>
        )}
      </td>
      <td className={LETTER_TD}>{letterTypeLabel(row.letter_type)}</td>
      <td className={LETTER_TD}>
        {row.summary !== null ? (
          // One line, cut with "…" (owner request v2 §4.3); the full text is in the drawer.
          <span className="block truncate">{row.summary}</span>
        ) : (
          <span className="block text-[12px] text-ink-muted italic">{row.summary_withheld ? WITHHELD_SUMMARY : "—"}</span>
        )}
        {merged && <span className="text-[11px] font-semibold text-tangerine">{MERGED_DUPLICATE}</span>}
      </td>
      <td className={LETTER_TD}>
        {unit === "" ? (
          <>
            <span aria-hidden="true">—</span>
            <span className="an-thi-giac">Chưa chuyển bộ phận nào</span>
          </>
        ) : unitLookup.loai === "coTen" ? (
          unitLookup.ten
        ) : unitLookup.loai === "dangDoc" ? (
          "Đang tải…"
        ) : (
          // Not in the catalogue (or it failed to load): the id still names exactly one unit.
          unit
        )}
      </td>
      <td className={cn(LETTER_TD, "tabular-nums")}>
        <span className={days.stopped ? "text-ink-muted" : "font-semibold text-navy"}>{days.text}</span>
        {days.note !== null && <span className="block text-[11px] text-ink-muted">{days.note}</span>}
      </td>
      <td className={LETTER_TD}>
        <span className={DUE_TONE_CLASS[due.tone]}>{due.text}</span>
      </td>
      <td className={LETTER_TD}>
        <LetterStatusBadge group={row.status_group} />
      </td>
    </tr>
  );
}
