"use client";

import { CircleAlert, Star } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import type { KetQua } from "@/lib/api/goi";
import { recordStaffRating, type StaffRatingChannel } from "@/lib/api/phieu-phan-anh";
import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { nhanThoiDiem, RATING_MAX_STARS } from "./nhan-phieu";
import { buttonClass, Glyph, HINT_CLASS, TEXTAREA_CLASS } from "./petition-ui";

/**
 * `Ghi nhận đánh giá của người dân` — prototype `FeedbackDetailDrawer.tsx:516-540`, bug sheet row 60
 * (owner rule 11/10/2026; supersedes the "not built" entry ADR 0062 had put in `PHAN_CHUA_DUNG`).
 *
 * WHAT THE SERVER DOES WITH IT (`service-petitions/internal/app/petition_staff_recorded_rating.go`): one
 * row in a separate table plus the audit entry. The citizen's own `rating` is untouched, no figure counts
 * it, no status moves, NOTHING REOPENS. So the prototype's hint "Một đến hai sao sẽ tự mở lại phiếu" is
 * NOT copied — it would promise the citizen a reopening that never happens (rule 10).
 *
 * A CHILD COMPONENT ON PURPOSE: it holds state, and `ChiTietPhieu` must not gain a hook
 * (`chon-can-bo.test.tsx` seeds that block's hooks by call order).
 *
 * THE PERMISSION GATE IS UX ONLY (rule 5, forbidden #1): the drawer draws this block for `feedback.resolve`
 * holders; the route checks the same key and its 403 sentence would reach the error line verbatim.
 */

export const STAFF_RATING_TITLE = "Ghi nhận đánh giá của người dân";
export const STAFF_RATING_HINT =
  "Dùng khi người dân đánh giá qua điện thoại hoặc tại quầy. Đánh giá thấp được ghi để theo dõi; không tự mở lại phiếu.";
export const STAFF_RATING_SUBMIT = "Ghi nhận đánh giá";
export const STAFF_RATING_NOTE_MAX = 1000;
/** Shown instead of the form once the citizen rated: their verdict is the one that stands. */
export const STAFF_RATING_CITIZEN_RATED = "Người dân đã tự đánh giá phiếu này nên không ghi nhận đánh giá thay.";

/** The two channels the server accepts, with the words the form shows. */
export const STAFF_RATING_CHANNELS: readonly { value: StaffRatingChannel; label: string; via: string }[] = [
  { value: "dien-thoai", label: "Qua điện thoại", via: "điện thoại" },
  { value: "tai-quay", label: "Tại quầy", via: "quầy tiếp nhận" },
];

/** `Cán bộ ghi nhận qua {kênh} — {mã cán bộ}`. An unknown channel is shown as sent, never guessed. */
export function staffRatingByline(channel: string, recordedBy: string): string {
  const via = STAFF_RATING_CHANNELS.find((c) => c.value === channel)?.via ?? channel;
  return `Cán bộ ghi nhận qua ${via} — ${recordedBy}`;
}

export type StaffRatingApi = { record: typeof recordStaffRating };
const DEFAULT_API: StaffRatingApi = { record: recordStaffRating };

function StarRow({ value, label }: { value: number; label: string }) {
  return (
    <span role="img" aria-label={label} className="inline-flex items-center gap-1">
      {Array.from({ length: RATING_MAX_STARS }, (_, i) => (
        <Star
          key={i}
          aria-hidden="true"
          focusable="false"
          className={cn("size-4", i < value ? "fill-tangerine text-tangerine" : "text-ink-muted")}
        />
      ))}
    </span>
  );
}

export function StaffRecordedRatingSection({
  petition,
  onRecorded,
  api = DEFAULT_API,
}: {
  petition: petitions_phieuPhanAnhRa;
  /** The 200 body (the petition carrying the new record). */
  onRecorded: (p: petitions_phieuPhanAnhRa) => void;
  api?: StaffRatingApi;
}) {
  const [stars, setStars] = useState(0);
  const [channel, setChannel] = useState<StaffRatingChannel | "">("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);

  const record = petition.staff_recorded_rating ?? null;
  const citizenRated = petition.rating !== undefined && petition.rating !== null;
  // Code points, as the server counts (`utf8.RuneCountInString`), not UTF-16 units.
  const noteLength = [...note.trim()].length;
  const noteTooLong = noteLength > STAFF_RATING_NOTE_MAX;

  function submit(): void {
    if (stars < 1 || channel === "" || noteTooLong || busy) return;
    setBusy(true);
    setRefusal(null);
    void api.record(petition.code, stars, channel, note).then((kq: KetQua<petitions_phieuPhanAnhRa>) => {
      setBusy(false);
      if (!kq.ok) {
        // 409 `citizen_rated` / `staff_rating_exists`, 400, 403: the server's sentence, verbatim.
        setRefusal(kq.thongBao);
        return;
      }
      toast.success(`Đã ghi nhận ${stars} sao.`);
      onRecorded(kq.duLieu);
    });
  }

  return (
    <section aria-labelledby="tieu-de-ghi-nhan-danh-gia" className="mt-5 border-t border-solid border-line pt-4">
      <h3 id="tieu-de-ghi-nhan-danh-gia" className="m-0 mb-2.5 text-[12.5px] font-bold text-navy">
        {STAFF_RATING_TITLE}
      </h3>

      {record !== null ? (
        // READ-ONLY: one staff record per petition (UNIQUE on the server); it is never edited.
        <div className="rounded-[10px] border border-solid border-line bg-white px-3 py-2.5 [&>p]:m-0">
          <p className="flex flex-wrap items-center gap-1.5">
            <StarRow value={record.stars} label={`${record.stars} sao`} />
            <span className="ml-1 text-[12.5px] font-semibold text-navy">
              {record.stars}/{RATING_MAX_STARS}
            </span>
            <span className="text-[11px] text-ink-muted">{nhanThoiDiem(record.recorded_at)}</span>
          </p>
          <p className="mt-1 text-[11.5px] text-ink-muted">{staffRatingByline(record.channel, record.recorded_by)}</p>
          {/* Staff free text about a citizen's case: a text node, never HTML (rule 13), written nowhere else. */}
          {record.note !== undefined && record.note !== "" && (
            <p className="noi-dung-phan-anh mt-1.5 text-[12.5px] text-navy">{record.note}</p>
          )}
          {citizenRated && <p className="mt-1.5 text-[11.5px] text-ink-muted">{STAFF_RATING_CITIZEN_RATED}</p>}
        </div>
      ) : citizenRated ? (
        <div className="rounded-[10px] border border-solid border-line bg-white px-3 py-2.5 [&>p]:m-0">
          <p className="flex flex-wrap items-center gap-1.5">
            <StarRow value={petition.rating ?? 0} label={`Người dân đánh giá ${petition.rating} sao`} />
            <span className="ml-1 text-[12.5px] font-semibold text-navy">
              {petition.rating}/{RATING_MAX_STARS}
            </span>
          </p>
          <p className="mt-1 text-[11.5px] text-ink-muted">{STAFF_RATING_CITIZEN_RATED}</p>
        </div>
      ) : (
        <form
          className="m-0 flex flex-col gap-2.5"
          onSubmit={(e) => {
            e.preventDefault();
            submit();
          }}
        >
          <p className="m-0 text-[11.5px] text-ink-muted">{STAFF_RATING_HINT}</p>
          <fieldset className="m-0 border-0 p-0">
            <legend className="mb-1 p-0 text-[11.5px] font-semibold text-ink">Số sao</legend>
            <div className="flex flex-wrap gap-1.5">
              {Array.from({ length: RATING_MAX_STARS }, (_, i) => i + 1).map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={stars === value}
                  className={buttonClass(stars === value ? "primary" : "secondary", "sm")}
                  onClick={() => setStars(value)}
                >
                  {value} ★
                </button>
              ))}
            </div>
          </fieldset>
          <fieldset className="m-0 border-0 p-0">
            <legend className="mb-1 p-0 text-[11.5px] font-semibold text-ink">Người dân đánh giá</legend>
            <div className="flex flex-wrap gap-x-4 gap-y-1.5">
              {STAFF_RATING_CHANNELS.map((c) => (
                <label key={c.value} className="inline-flex min-h-8 items-center gap-1.5 text-[12.5px] text-navy">
                  <input
                    type="radio"
                    name="kenh-danh-gia"
                    value={c.value}
                    checked={channel === c.value}
                    onChange={() => setChannel(c.value)}
                  />
                  {c.label}
                </label>
              ))}
            </div>
          </fieldset>
          <div>
            <label htmlFor="ghi-chu-danh-gia" className="mb-1 block text-[11.5px] leading-tight font-semibold text-ink">
              Ghi chú (không bắt buộc)
            </label>
            <textarea
              id="ghi-chu-danh-gia"
              rows={2}
              className={cn(TEXTAREA_CLASS, "bg-white text-[12.5px] md:text-[12.5px]")}
              value={note}
              aria-invalid={noteTooLong || undefined}
              onChange={(e) => setNote(e.target.value)}
            />
            <p className={cn(HINT_CLASS, noteTooLong && "text-danger")}>
              {noteLength}/{STAFF_RATING_NOTE_MAX} ký tự
            </p>
          </div>
          {refusal !== null && (
            <div className="flex items-start gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
              <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
              <p className="thong-bao-loi" role="alert">
                {refusal}
              </p>
            </div>
          )}
          <div>
            <button
              type="submit"
              className={buttonClass("primary", "sm")}
              disabled={busy || stars < 1 || channel === "" || noteTooLong}
            >
              {STAFF_RATING_SUBMIT}
            </button>
          </div>
        </form>
      )}
    </section>
  );
}
