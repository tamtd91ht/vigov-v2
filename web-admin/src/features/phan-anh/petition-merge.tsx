"use client";

import { Combine, Copy, Loader2, Split } from "lucide-react";
import { useEffect, useState, type FormEvent, type MouseEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import type { KetQua } from "@/lib/api/goi";
import { listDuplicateCandidates, mergePetition, unmergePetition } from "@/lib/api/phieu-phan-anh";
import type { petitions_duplicateCandidatesOut, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  cardSenderLabel,
  dateTimeLabel,
  linhVucPhanAnh,
  nhanLinhVuc,
  nhanTrangThai,
  STAFF_CONDUCT_FIELD,
} from "./nhan-phieu";
import { buttonClass, Glyph, HINT_CLASS, LABEL_CLASS, PetitionStatusBadge, TEXTAREA_CLASS } from "./petition-ui";

/**
 * `Có thể trùng với phiếu khác` — the prototype's drawer section (`FeedbackDetailDrawer.tsx:408-439`),
 * built on ADR 0087: merging is a LINK into a main petition, never a closing and never a status.
 *
 * WHAT IS DECIDED HERE IS ONLY WHAT TO DRAW. Which states may merge, chains, `can-bo`, the deadline
 * check — all of it is the server's (migration 0037, `petition_merge.go`); its refusal sentence is shown
 * verbatim inside the dialog. The few status facts below choose a SENTENCE or hide a control that
 * cannot work; they grant nothing (rule 5, forbidden #1).
 *
 * THE MAIN PETITION'S CODE IS ON THE DETAIL ONLY. The list route carries `merged_at` but not
 * `merged_into` / `merged_petitions` (`phieu_phan_anh.go:264-275`), so a drawer opened from a card
 * knows THAT the petition is merged, not into WHICH one. Reading the detail on every open would write a
 * disclosure entry for `feedback.unmask` holders who never asked to see the reporter — so it is an
 * explicit `Xem phiếu chính` instead.
 */

export const DUPLICATES_TITLE = "Có thể trùng với phiếu khác";
export const DUPLICATES_LOADING = "Đang tìm phiếu có thể trùng…";
export const DUPLICATES_NO_LOCATION =
  "Phiếu trùng được dò theo vị trí hiện trường. Phiếu này không có vị trí nên không dò được.";
export const DUPLICATES_TRUNCATED = "Quanh vị trí này có rất nhiều phiếu, danh sách dưới đây có thể chưa đủ.";
export const MERGED_INTO_UNKNOWN = "Phiếu này đã được gộp vào một phiếu chính.";
export const SHOW_MAIN_PETITION = "Xem phiếu chính";
export const UNMERGE_ACTION = "Tách khỏi phiếu chính";
export const UNMERGE_CLOSED = "Phiếu đã kết thúc nên không tách khỏi phiếu chính được nữa.";
export const MERGED_CHILDREN_LABEL = "Các phiếu đã gộp vào phiếu này";
export const MERGE_REASON_LABEL = "Lý do gộp (không bắt buộc)";
export const UNMERGE_REASON_LABEL = "Lý do tách phiếu";
export const MERGE_REASON_HINT = "Chỉ cán bộ đọc được, không gửi người dân.";
export const MERGED_TOAST = "Đã gộp phiếu.";
export const UNMERGED_TOAST = "Đã tách phiếu.";

/** `Không thấy phiếu nào …` with the commune's OWN radius and window (migration 0038, ADR 0087 §6). */
export function duplicatesEmpty(radiusMeters: number, windowDays: number): string {
  return `Không thấy phiếu nào trong vòng ${radiusMeters} m và ${windowDays} ngày.`;
}

/** The prototype's button, verbatim (`FeedbackDetailDrawer.tsx:433`). */
export function mergeActionLabel(mainCode: string): string {
  return `Gộp phiếu này vào ${mainCode}`;
}

export function mergedIntoLabel(mainCode: string): string {
  return `Đã gộp vào phiếu ${mainCode}`;
}

/**
 * The four states BOTH petitions must be in (ADR 0087 §Trả lời #1, `domain.MergeOpen`). Read only to
 * decide whether a search is worth drawing — the server answers an empty list outside them anyway.
 */
const MERGE_OPEN_STATUSES: readonly string[] = ["da-tiep-nhan", "dang-phan-loai", "da-chuyen-xu-ly", "dang-xu-ly"];

/** Statuses after which a merged petition can no longer be unmerged (`domain.ErrUnmergeNotOpen`). */
const UNMERGE_CLOSED_STATUSES: readonly string[] = ["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"];

export function isMerged(p: petitions_phieuPhanAnhRa): boolean {
  return (p.merged_into ?? "") !== "" || (p.merged_at ?? null) !== null;
}

/**
 * Whether the section is drawn at all. The prototype hides it when there is nothing to show; here it is
 * also drawn for a petition that is linked (either side), and for one the server would search.
 * `can-bo` is never merged (ADR 0087 §5) — no search is offered for it.
 */
export function mergeSectionShown(p: petitions_phieuPhanAnhRa): boolean {
  if (isMerged(p) || (p.merged_petitions ?? []).length > 0) return true;
  return p.field !== STAFF_CONDUCT_FIELD && MERGE_OPEN_STATUSES.includes(p.status);
}

export function hasLocation(p: Pick<petitions_phieuPhanAnhRa, "lat" | "lng">): boolean {
  return typeof p.lat === "number" && typeof p.lng === "number";
}

export type MergeApi = {
  list: (code: string) => Promise<KetQua<petitions_duplicateCandidatesOut>>;
  merge: (code: string, mainCode: string, reason?: string) => Promise<KetQua<petitions_phieuPhanAnhRa>>;
  unmerge: (code: string, reason: string) => Promise<KetQua<petitions_phieuPhanAnhRa>>;
};

const DEFAULT_API: MergeApi = { list: listDuplicateCandidates, merge: mergePetition, unmerge: unmergePetition };

/** `/phan-anh?id=<code>` — the screen's own deep link, so a code can also be opened in a new tab. */
export function petitionHref(code: string): string {
  return `/phan-anh?id=${encodeURIComponent(code)}`;
}

/**
 * A code that opens its petition. A real link (new tab works); a plain click opens it in place through
 * `onOpen` — the detail route, exactly as `?id=` does — rather than navigating, which would not re-open
 * a code already in the address bar.
 */
function PetitionLink({ code, onOpen }: { code: string; onOpen?: (code: string) => void }) {
  function click(e: MouseEvent<HTMLAnchorElement>): void {
    if (onOpen === undefined || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    onOpen(code);
  }
  return (
    <a href={petitionHref(code)} onClick={click} className="ma-muc font-semibold text-brand underline-offset-2 hover:underline">
      {code}
    </a>
  );
}

const ROW = "rounded-[10px] border border-solid border-tangerine/25 bg-tangerine/8 px-3 py-2.5 [&>p]:m-0";

export function PetitionMergeSection({
  petition,
  mayMerge,
  onChanged,
  onOpen,
  api = DEFAULT_API,
}: {
  petition: petitions_phieuPhanAnhRa;
  /** `feedback.classify` (ADR 0087 §5). UX only — the route checks the same key and the commune. */
  mayMerge: boolean;
  /** The 200 body of a merge / unmerge. Absent = no write path, no action drawn. */
  onChanged?: (p: petitions_phieuPhanAnhRa) => void;
  /** Opens another petition in place (the detail route). Absent = the codes are plain links. */
  onOpen?: (code: string) => void;
  /** Injected only by tests. */
  api?: MergeApi;
}) {
  const merged = isMerged(petition);
  const children = petition.merged_petitions ?? [];
  const search = !merged && petition.field !== STAFF_CONDUCT_FIELD && MERGE_OPEN_STATUSES.includes(petition.status) && hasLocation(petition);
  const [reload, setReload] = useState(0);
  const key = `${petition.code}|${reload}`;
  const [loaded, setLoaded] = useState<{ key: string; r: KetQua<petitions_duplicateCandidatesOut> } | null>(null);
  const [mergeInto, setMergeInto] = useState<string | null>(null);
  const [unmergeOpen, setUnmergeOpen] = useState(false);
  const mayAct = mayMerge && onChanged !== undefined;

  useEffect(() => {
    if (!search) return;
    let dropped = false;
    api.list(petition.code).then((r) => {
      if (!dropped) setLoaded({ key, r });
    });
    return () => {
      dropped = true;
    };
  }, [api, key, petition.code, search]);

  const result = loaded !== null && loaded.key === key ? loaded.r : null;
  const mainCode = petition.merged_into ?? "";
  const unmergeClosed = UNMERGE_CLOSED_STATUSES.includes(petition.status);

  function changed(p: petitions_phieuPhanAnhRa): void {
    onChanged?.(p);
    setReload((n) => n + 1);
  }

  return (
    <div className="flex flex-col gap-2">
      {merged && (
        <div className={ROW} data-merged="">
          <p className="flex flex-wrap items-center gap-x-1.5 text-[12.5px] font-semibold text-navy">
            <Glyph icon={Combine} className="size-3.5 shrink-0" />
            {mainCode === "" ? (
              MERGED_INTO_UNKNOWN
            ) : (
              <span>
                Đã gộp vào phiếu <PetitionLink code={mainCode} onOpen={onOpen} />
              </span>
            )}
          </p>
          {petition.merged_at != null && (
            <p className="mt-0.5 text-[11px] text-ink-muted tabular-nums">Lúc {dateTimeLabel(petition.merged_at)}</p>
          )}
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {mainCode === "" && onOpen !== undefined && (
              <button type="button" className={buttonClass("secondary", "sm")} onClick={() => onOpen(petition.code)}>
                {SHOW_MAIN_PETITION}
              </button>
            )}
            {mayAct && (
              <button
                type="button"
                aria-haspopup="dialog"
                className={buttonClass("secondary", "sm")}
                disabled={unmergeClosed}
                onClick={() => setUnmergeOpen(true)}
              >
                <Glyph icon={Split} className="size-3.5" />
                {UNMERGE_ACTION}
              </button>
            )}
          </div>
          {mayAct && unmergeClosed && <p className="mt-1 text-[11px] text-ink-muted">{UNMERGE_CLOSED}</p>}
        </div>
      )}

      {children.length > 0 && (
        <div className={ROW} data-merged-children="">
          <p className="text-[11px] font-semibold text-ink-muted uppercase">{MERGED_CHILDREN_LABEL}</p>
          <ul className="m-0 mt-1 flex list-none flex-wrap gap-x-3 gap-y-1 p-0 text-[12.5px]">
            {children.map((c) => (
              <li key={c}>
                <PetitionLink code={c} onOpen={onOpen} />
              </li>
            ))}
          </ul>
        </div>
      )}

      {!merged && !search && MERGE_OPEN_STATUSES.includes(petition.status) && petition.field !== STAFF_CONDUCT_FIELD && (
        <p className="m-0 text-[12px] text-ink-muted">{DUPLICATES_NO_LOCATION}</p>
      )}

      {search && result === null && (
        <p className="m-0 text-[12px] text-ink-muted" role="status">
          {DUPLICATES_LOADING}
        </p>
      )}

      {search && result !== null && !result.ok && (
        <div className="flex flex-wrap items-center gap-2">
          <p className="thong-bao-loi m-0" role="alert">
            {result.thongBao}
          </p>
          <button type="button" className={buttonClass("secondary", "sm")} onClick={() => setReload((n) => n + 1)}>
            Tải lại
          </button>
        </div>
      )}

      {search && result !== null && result.ok && (
        <>
          {result.duLieu.items.length === 0 ? (
            <p className="m-0 text-[12px] text-ink-muted">
              {duplicatesEmpty(result.duLieu.radius_meters, result.duLieu.window_days)}
            </p>
          ) : (
            <ul className="m-0 flex list-none flex-col gap-2 p-0" aria-label={DUPLICATES_TITLE}>
              {result.duLieu.items.map((c) => (
                <li key={c.code} className={ROW}>
                  <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] font-semibold text-navy">
                    <Glyph icon={Copy} className="size-3.5 shrink-0" />
                    <PetitionLink code={c.code} onOpen={onOpen} />
                    <span>· {nhanLinhVuc(linhVucPhanAnh(c.field, c.field_label))}</span>
                    <span className="font-normal text-ink-muted tabular-nums">· {dateTimeLabel(c.clock_from)}</span>
                    <PetitionStatusBadge status={c.status}>{nhanTrangThai(c.status)}</PetitionStatusBadge>
                  </p>
                  {/* The citizen's words, as the card shows them — never in an attribute (rule 3). */}
                  <p className="mt-1 line-clamp-2 text-[12px] text-navy">{c.content}</p>
                  {/* The list route's masked pair; anonymous says so and nothing more. */}
                  <p className="mt-0.5 text-[11px] text-ink-muted">{cardSenderLabel(c)}</p>
                  {mayAct && (
                    <button
                      type="button"
                      aria-haspopup="dialog"
                      className={buttonClass("secondary", "sm", "mt-2")}
                      onClick={() => setMergeInto(c.code)}
                    >
                      {mergeActionLabel(c.code)}
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
          {result.duLieu.truncated && <p className="m-0 text-[11px] text-ink-muted">{DUPLICATES_TRUNCATED}</p>}
        </>
      )}

      {mergeInto !== null && (
        <MergeDialog
          code={petition.code}
          mainCode={mergeInto}
          merge={api.merge}
          onClose={() => setMergeInto(null)}
          onDone={changed}
        />
      )}
      {unmergeOpen && (
        <UnmergeDialog
          code={petition.code}
          mainCode={mainCode}
          unmerge={api.unmerge}
          onClose={() => setUnmergeOpen(false)}
          onDone={changed}
        />
      )}
    </div>
  );
}

const MERGE_TITLE_ID = "petition-merge-title";
export const MERGE_REASON_ID = "ly-do-gop-phieu";
const UNMERGE_TITLE_ID = "petition-unmerge-title";
export const UNMERGE_REASON_ID = "ly-do-tach-phieu";

/**
 * The confirm of `Gộp phiếu này vào …`. Says what the act does and does NOT do (ADR 0087: nothing is
 * closed, each citizen keeps their code, the main petition takes the EARLIER deadline). Any refusal —
 * 409 `merge_deadline_before_origin`, `merge_state`, 400 — stays IN the dialog, verbatim.
 */
export function MergeDialog({
  code,
  mainCode,
  merge,
  onClose,
  onDone,
}: {
  code: string;
  mainCode: string;
  merge: MergeApi["merge"];
  onClose: () => void;
  onDone: (p: petitions_phieuPhanAnhRa) => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function submit(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    merge(code, mainCode, reason).then((r) => {
      setBusy(false);
      if (!r.ok) {
        setError(r.thongBao);
        return;
      }
      toast.success(MERGED_TOAST);
      onDone(r.duLieu);
      onClose();
    });
  }

  return (
    <ModalDialog titleId={MERGE_TITLE_ID} closeDisabled={busy} onDismiss={() => !busy && onClose()}>
      <ModalDialogHeader
        titleId={MERGE_TITLE_ID}
        title={`Gộp phiếu ${code} vào phiếu ${mainCode}`}
        description={
          `Hai phiếu được liên kết, không phiếu nào bị đóng; mỗi người dân giữ mã tra cứu của mình. Hạn xử lý ` +
          `của phiếu ${mainCode} lấy mốc sớm hơn của hai phiếu. Khi phiếu ${mainCode} xử lý xong, phiếu ${code} ` +
          `đóng theo với cùng kết quả.`
        }
      />
      <form className="m-0 flex flex-col gap-4" onSubmit={submit}>
        <div>
          <label htmlFor={MERGE_REASON_ID} className={LABEL_CLASS}>
            {MERGE_REASON_LABEL}
          </label>
          <textarea
            id={MERGE_REASON_ID}
            name={MERGE_REASON_ID}
            rows={3}
            value={reason}
            className={TEXTAREA_CLASS}
            onChange={(e) => setReason(e.target.value)}
          />
          <p className={HINT_CLASS}>{MERGE_REASON_HINT}</p>
        </div>
        {error !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <DialogButtons busy={busy} onClose={onClose} label="Gộp phiếu" disabled={busy} />
      </form>
    </ModalDialog>
  );
}

/**
 * `Tách khỏi phiếu chính` — the reason is REQUIRED (owner, 09/10/2026); the main petition's deadline is
 * not lengthened back (ADR 0087 §Hệ quả). A refusal stays in the dialog, verbatim, the reason kept.
 */
export function UnmergeDialog({
  code,
  mainCode,
  unmerge,
  onClose,
  onDone,
}: {
  code: string;
  /** `""` when the drawer was opened from a card — the title then says "phiếu chính". */
  mainCode: string;
  unmerge: MergeApi["unmerge"];
  onClose: () => void;
  onDone: (p: petitions_phieuPhanAnhRa) => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function submit(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    if (busy || reason.trim() === "") return;
    setBusy(true);
    setError(null);
    unmerge(code, reason).then((r) => {
      setBusy(false);
      if (!r.ok) {
        setError(r.thongBao);
        return;
      }
      toast.success(UNMERGED_TOAST);
      onDone(r.duLieu);
      onClose();
    });
  }

  return (
    <ModalDialog
      titleId={UNMERGE_TITLE_ID}
      closeDisabled={busy}
      initialFocusId={UNMERGE_REASON_ID}
      onDismiss={() => !busy && onClose()}
    >
      <ModalDialogHeader
        titleId={UNMERGE_TITLE_ID}
        title={mainCode === "" ? `Tách phiếu ${code} khỏi phiếu chính` : `Tách phiếu ${code} khỏi phiếu ${mainCode}`}
        description="Phiếu quay lại được xử lý riêng. Hạn xử lý của phiếu chính giữ nguyên."
      />
      <form className="m-0 flex flex-col gap-4" onSubmit={submit}>
        <div>
          <label htmlFor={UNMERGE_REASON_ID} className={LABEL_CLASS}>
            {UNMERGE_REASON_LABEL}
          </label>
          <textarea
            id={UNMERGE_REASON_ID}
            name={UNMERGE_REASON_ID}
            rows={3}
            required
            value={reason}
            className={TEXTAREA_CLASS}
            onChange={(e) => setReason(e.target.value)}
          />
          <p className={HINT_CLASS}>{MERGE_REASON_HINT}</p>
        </div>
        {error !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <DialogButtons busy={busy} onClose={onClose} label="Tách phiếu" disabled={busy || reason.trim() === ""} />
      </form>
    </ModalDialog>
  );
}

function DialogButtons({
  busy,
  onClose,
  label,
  disabled,
}: {
  busy: boolean;
  onClose: () => void;
  label: string;
  disabled: boolean;
}) {
  return (
    <div className="flex flex-wrap justify-end gap-2">
      <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
        Huỷ
      </Button>
      <Button
        type="submit"
        variant="primary"
        icon={busy ? <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" /> : undefined}
        disabled={disabled}
      >
        {label}
      </Button>
    </div>
  );
}
