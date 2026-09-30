/**
 * PHẢN ÁNH TRONG APP RIÊNG CỦA XÃ — trên sổ phản ánh THẬT của xã (29/09/2026, thay bản trải nghiệm chỉ
 * trong bộ nhớ). Giao diện theo ADR 0050: chỗ xung đột theo kho yêu cầu, còn lại theo prototype khách
 * (`../vigov-require/apps/miniapp` — `NewFeedbackPage`, `FeedbackDetailPage`, `StatusChip`, `RatingBlock`).
 *
 * DỮ LIỆU ĐI QUA ĐÚNG CLIENT CỦA APP CHUNG (`api/goi-vigov.ts`: `guiPhanAnh`, `phanAnhCuaToi`, `traCuuPhieu`,
 * `ratePetition` qua `PetitionRating`) — một client, hai giao diện. Không tệp nào ở đây gọi mạng hay cầm
 * bearer: phiên nằm ở `api/phien-vigov.ts`, và chỉ mở ở việc cá nhân đầu tiên, sau lời giải thích
 * (`commune-session.ts`, gọi từ `TrangXa.tsx`).
 *
 *   · Gửi: Lĩnh vực → Mô tả (tên xã đọc lại ngay trên nút gửi — README §Non-negotiables #5) → Đã gửi, với
 *     MÃ TRA CỨU máy chủ cấp (luật 10 #1). Một lần bấm = một `Idempotency-Key` (`api/lan-gui.ts`): "Gửi
 *     lại" sau mạng rớt dùng lại cùng khoá, nên không bao giờ thành hai phiếu.
 *   · Bước 1 là DANH MỤC LĨNH VỰC CỦA XÃ (`/my-citizen-report-fields`, tải khi màn mở — tức SAU cổng),
 *     theo thứ tự của xã; mã chọn được đi lên thành `field`. Không có danh sách dự phòng (ADR 0060 §3): 503
 *     là một câu kèm "Thử lại". 400 `field_not_offered` (xã vừa đổi danh mục) → tải lại, chọn lại.
 *   · Người dân thấy BỐN nhóm trạng thái (`status-groups.ts`); mã lạ hiện câu trung tính, không đoán nhóm,
 *     không in mã thô — ở chip, ở danh sách lọc, ở dòng thời gian.
 *   · 401 / 403 `chua_xac_thuc_so`: phiên bị quên (`dropCommuneAppSession`) và việc đang làm đi lại qua cổng
 *     (`onSessionLost`) — cùng lần gửi, cùng khoá.
 *   · Nháp đang soạn giữ trên máy (ADR 0050 #7) qua `draftStore` lớp vỏ tiêm; tệp này không chạm kho lưu trữ.
 *     Toạ độ KHÔNG vào nháp.
 *
 * Mọi ô nhập đi qua `o-nhap.tsx` — tệp duy nhất của nửa nhà nước được có ô nhập.
 */
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import {
  citizenReportFields,
  guiPhanAnh, // vi-name-ok: existing export, not renamed (rule 12 #3)
  type KetQuaDanhSach, // vi-name-ok: existing export, not renamed (rule 12 #3)
  type KetQuaGoi, // vi-name-ok: existing export, not renamed (rule 12 #3)
  phanAnhCuaToi, // vi-name-ok: existing export, not renamed (rule 12 #3)
  traCuuPhieu, // vi-name-ok: existing export, not renamed (rule 12 #3)
} from "../api/goi-vigov";
import {
  type CitizenField,
  DO_DAI_TOI_DA,
  type PhieuCuaToi, // vi-name-ok: existing contract type, not renamed (rule 12 #3)
  type PhieuCuaToiTomTat, // vi-name-ok: existing contract type, not renamed (rule 12 #3)
  type SceneLocation,
  thanGuiPhanAnh, // vi-name-ok: existing export, not renamed (rule 12 #3)
} from "../api/hop-dong-phan-anh";
import { type LanGui, taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import type { ReopenWithPhone } from "../api/mo-phien-vigov";
import { DEMO_BUILD, DEMO_CITIZEN_PHONE } from "../../lib/demo-build";
import { thoiDiemVN } from "../../lib/thoi-diem";

import { BieuTuong, type TenBieuTuong } from "./BieuTuong"; // vi-name-ok: existing exported type, imported not renamed
import { nhanLinhVuc } from "./khung";
import { DauManCon, KhoiTrangThai, type Tone, TrangCon } from "./khung-xa";
import {
  CUA_TOI,
  GUI,
  giaiThichTrangThai,
  KENH_CHUA_MO,
  KHAN_CAP,
  LOI_GUI,
  nhanTrangThai,
  THE_PHIEU,
  TRA_CUU,
  XA_PA,
  XA_TN,
} from "./noi-dung";
import { ONhapDoan, ONhapDong } from "./o-nhap";
import { PetitionRating } from "./PetitionRating";
import {
  type GetSceneLocation,
  SceneLocationControl,
  type SceneLocationWords,
  useSceneLocation,
} from "./scene-location";
import {
  type FeedbackDraftStore,
  kiemNhapPhieu,
  type LoiNhapPhieu,
  NHAN_BUOC,
  NHAN_NHOM,
  nhomCua,
  type NhomLoc,
  type NhapPhieu,
  VONG_DOI,
} from "./trai-nghiem";

/**
 * What a screen does when the server says the session is gone (401) or unverified (403
 * `chua_xac_thuc_so`): hand the act to `TrangXa`, which forgets the session and runs the act again through
 * the gate. `retry` repeats EXACTLY the act (same body, same key).
 */
export type OnSessionLost = (retry: () => void) => void;

/**
 * Nhãn một trong BỐN nhóm người dân thấy (ADR 0050 #5). An unknown code gets the shared app's neutral
 * sentence (`nhanTrangThai`) and a neutral style — never a guessed group, never the raw code.
 */
export function ChipTrangThai({ tt }: { tt: string }) {
  const nhom = nhomCua(tt);
  if (nhom === null) return <span className="xa-chip-tt xa-chip-tt--unknown">{nhanTrangThai(tt)}</span>;
  return <span className={`xa-chip-tt xa-chip-tt--${nhom}`}>{NHAN_NHOM[nhom]}</span>;
}

const at = (iso: string) => thoiDiemVN(iso) ?? "";
/** The day part of `thoiDiemVN` ("dd/MM/yyyy HH:mm" → "dd/MM/yyyy"); "" when unreadable. */
const dayOf = (iso: string) => (thoiDiemVN(iso) ?? "").split(" ")[0] ?? "";

/**
 * One row of "Phản ánh của tôi" — on the home screen and in the list (`PROTOTYPE.md` §6.3 FeedbackCard):
 * `#code` and the day it was sent, the field, the content on one line, the status chip. NO photo box: the
 * box appears only when a petition HAS photos (decision 12), and none do until the photo card — an empty
 * "Không ảnh" frame on every card would be a placeholder standing for nothing.
 */
export function PetitionCard({ petition, onOpen }: { petition: PhieuCuaToiTomTat; onOpen: () => void }) {
  return (
    <button type="button" className="xa-the xa-petition-card" onClick={onOpen}>
      <span className="xa-petition-card__body">
        <span className="xa-petition-card__head">
          <strong className="xa-petition-card__code">#{petition.ma_tra_cuu}</strong>
          <span className="xa-phu">{dayOf(petition.goc_dem_han)}</span>
        </span>
        <span className="xa-petition-card__field">{nhanLinhVuc(petition.linh_vuc, petition.nhan_linh_vuc)}</span>
        <span className="xa-petition-card__excerpt">{petition.trich_noi_dung}</span>
        <ChipTrangThai tt={petition.trang_thai} />
      </span>
      <BieuTuong ten="chevron-right" co={22} />
    </button>
  );
}

/* ═══════════════════════════════ PHẢN ÁNH CỦA TÔI — trạng thái danh sách ═══════════════════════════════ */

export type ListFailure = "server" | "network" | "closed";

/**
 * The list of THIS citizen's petitions, as loaded in this open. `idle` = not loaded (no session yet, or it
 * was dropped): the screens then offer the gate, never a guessed empty list.
 */
export type MyPetitions =
  | { readonly kind: "idle" }
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly failure: ListFailure }
  | {
      readonly kind: "ready";
      readonly items: readonly PhieuCuaToiTomTat[];
      readonly cursor: string;
      readonly hasMore: boolean;
      readonly loadingMore: boolean;
      readonly moreFailure: ListFailure | null;
    };

/** One list call's result → failure kind, `"session"` (gate again), or the page. PURE. */
export function listOutcome(kq: KetQuaDanhSach): ListFailure | "session" | Extract<KetQuaDanhSach, { kieu: "xong" }> {
  switch (kq.kieu) {
    case "xong":
      return kq;
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return "session";
    case "chua-cau-hinh":
      return "closed";
    case "loi-mang":
      return "network";
    default:
      return "server";
  }
}

export function listFailureText(f: ListFailure): string {
  if (f === "network") return CUA_TOI.loi_mang;
  if (f === "closed") return KENH_CHUA_MO.cau;
  return CUA_TOI.loi_may_chu;
}

/**
 * The list, loaded page by page through `phanAnhCuaToi` — no "whose" parameter: citizen and commune come
 * from the session on the server (rule 4 forbidden #1, rule 1 forbidden #2).
 *
 * The in-flight guard is a ref, not state: StrictMode runs effects twice, and two first pages appended
 * would show every petition twice.
 */
export function useMyPetitions(onSessionLost: OnSessionLost) {
  const [state, setState] = useState<MyPetitions>({ kind: "idle" });
  const busy = useRef(false);
  const current = useRef<MyPetitions>(state);
  current.current = state;

  async function load() {
    if (busy.current) return;
    busy.current = true;
    setState({ kind: "loading" });
    const out = listOutcome(await phanAnhCuaToi(""));
    busy.current = false;
    if (out === "session") {
      setState({ kind: "idle" });
      onSessionLost(() => void load());
      return;
    }
    if (typeof out === "string") {
      setState({ kind: "failed", failure: out });
      return;
    }
    setState({
      kind: "ready",
      items: out.trang.muc,
      cursor: out.trang.con_tro,
      hasMore: out.trang.con_nua,
      loadingMore: false,
      moreFailure: null,
    });
  }

  async function loadMore() {
    const s = current.current;
    if (busy.current || s.kind !== "ready" || !s.hasMore) return;
    busy.current = true;
    setState({ ...s, loadingMore: true, moreFailure: null });
    const out = listOutcome(await phanAnhCuaToi(s.cursor));
    busy.current = false;
    if (out === "session") {
      setState({ ...s, loadingMore: false });
      onSessionLost(() => void loadMore());
      return;
    }
    if (typeof out === "string") {
      setState({ ...s, loadingMore: false, moreFailure: out });
      return;
    }
    setState({
      kind: "ready",
      items: [...s.items, ...out.trang.muc],
      cursor: out.trang.con_tro,
      hasMore: out.trang.con_nua,
      loadingMore: false,
      moreFailure: null,
    });
  }

  /** Load once if nothing is loaded yet — after the gate opened a session for any personal act. */
  function loadIfIdle() {
    if (current.current.kind === "idle") void load();
  }

  return { state, load, loadMore, loadIfIdle };
}

/** "Hơn n" when more pages exist: a count of loaded rows is a floor, never presented as the total. */
export function countLabel(n: number, hasMore: boolean): string {
  return hasMore ? `${n}+` : String(n);
}

/* ═══════════════════════════════ DANH SÁCH ═══════════════════════════════ */

const GROUPS: readonly NhomLoc[] = ["tat-ca", "da-tiep-nhan", "dang-xu-ly", "da-xu-ly-xong", "da-dong"];
const filterLabel = (n: NhomLoc) => (n === "tat-ca" ? XA_TN.loc_tat_ca : NHAN_NHOM[n]);

/** The card that stands where the list will be until the citizen asks to see it (no session yet). */
export function NeedSessionCard({ onOpen }: { onOpen: () => void }) {
  return (
    <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-can-phien">
      <h2 className="xa-dau-khoi__tieu-de" id="xa-can-phien">
        {XA_PA.need_session_title}
      </h2>
      <p>{XA_PA.need_session_body}</p>
      <button type="button" className="xa-nut" onClick={onOpen}>
        {XA_PA.need_session_button}
      </button>
    </section>
  );
}

/** Loading / failed / idle for both the home block and the list. `null` when the list is ready. */
export function ListStatus(props: { state: MyPetitions; onOpen: () => void; onRetry: () => void }) {
  const { state } = props;
  if (state.kind === "idle") return <NeedSessionCard onOpen={props.onOpen} />;
  if (state.kind === "loading") return <KhoiTrangThai bieu_tuong="chat" cau={CUA_TOI.dang_tai} dang_tai />;
  if (state.kind === "failed") {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={listFailureText(state.failure)}
        nut={state.failure === "closed" ? undefined : { nhan: CUA_TOI.nut_thu_lai, onBam: props.onRetry }}
      />
    );
  }
  return null;
}

export function PetitionList(props: {
  state: MyPetitions;
  onOpenPetition: (code: string) => void;
  onLookup: () => void;
  /** No session yet: the citizen asks to see the list (goes through the gate). */
  onOpen: () => void;
  onRetry: () => void;
  onLoadMore: () => void;
}) {
  const { state } = props;
  const [filter, setFilter] = useState<NhomLoc>("tat-ca");
  const items = state.kind === "ready" ? state.items : [];
  const shown = useMemo(
    () => (filter === "tat-ca" ? items : items.filter((p) => nhomCua(p.trang_thai) === filter)),
    [items, filter],
  );
  const count = (n: NhomLoc) => (n === "tat-ca" ? items.length : items.filter((p) => nhomCua(p.trang_thai) === n).length);

  return (
    <>
      <button type="button" className="xa-the xa-hang xa-hang--vien" onClick={props.onLookup}>
        <span className="xa-o-bt xa-mau--xanh" aria-hidden="true">
          <BieuTuong ten="search" co={24} />
        </span>
        <span className="xa-hang__chu">
          <strong>{XA_PA.tra_cuu_tieu_de}</strong>
          <span className="xa-phu">{XA_PA.tra_cuu_goi_y}</span>
        </span>
        <BieuTuong ten="right" co={20} />
      </button>
      <ListStatus state={state} onOpen={props.onOpen} onRetry={props.onRetry} />
      {state.kind === "ready" && (
        <>
          <div className="xa-chips" role="group" aria-label={XA_TN.loc_nhom}>
            {GROUPS.map((k) => (
              <button
                key={k}
                type="button"
                className={`xa-chip${filter === k ? " xa-chip--on" : ""}`}
                aria-pressed={filter === k}
                onClick={() => setFilter(k)}
              >
                {filterLabel(k)} ({countLabel(count(k), state.hasMore)})
              </button>
            ))}
          </div>
          {shown.length === 0 ? (
            // Nothing sent yet: the title and what to do next — the "Gửi phản ánh mới" button right below
            // (§6.3, `04-phan-anh-cua-toi-rong.png`). A filter with nothing in it needs no such hint.
            items.length === 0 ? (
              <KhoiTrangThai bieu_tuong="message-square-plus" cau={XA_PA.chua_co_phieu} hint={XA_PA.empty_list_hint} />
            ) : (
              <KhoiTrangThai bieu_tuong="chat" cau={XA_TN.loc_trong} />
            )
          ) : (
            <ul className="xa-ds">
              {shown.map((p) => (
                <li key={p.ma_tra_cuu}>
                  <PetitionCard petition={p} onOpen={() => props.onOpenPetition(p.ma_tra_cuu)} />
                </li>
              ))}
            </ul>
          )}
          {state.moreFailure !== null && (
            <p className="xa-loi-o" role="alert">
              {listFailureText(state.moreFailure)}
            </p>
          )}
          {state.loadingMore && (
            <p className="xa-phu" role="status">
              {CUA_TOI.dang_tai_them}
            </p>
          )}
          {state.hasMore && !state.loadingMore && (
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onLoadMore}>
              {CUA_TOI.nut_xem_them}
            </button>
          )}
        </>
      )}
    </>
  );
}

/* ═══════════════════════════════ CHI TIẾT ═══════════════════════════════ */

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <p className="xa-nhan-o">{label}</p>
      <div>{children}</div>
    </>
  );
}

/** One fact of the summary card: an icon (decoration — the label says it), the label, the value. */
function Fact({ icon, label, children }: { icon: TenBieuTuong; label: string; children: ReactNode }) {
  return (
    <div className="xa-facts__row">
      <dt>
        <BieuTuong ten={icon} co={20} />
        <span>{label}</span>
      </dt>
      <dd>{children}</dd>
    </div>
  );
}

/**
 * Dòng thời gian — CHỈ các bước đã qua (prototype: sự kiện đã xảy ra, `feedback-adapter.ts:128-137`), nhãn
 * của prototype. Hai nhánh kết thúc dừng sau "Đang phân loại". THUẦN, xuất để test.
 */
export function buocDaQua(tt: string): string[] {
  if (tt === "khong-tiep-nhan" || tt === "chuyen-cap-tren") return ["da-tiep-nhan", "dang-phan-loai", tt];
  const i = VONG_DOI.indexOf(tt);
  return i < 0 ? [tt] : VONG_DOI.slice(0, i + 1);
}

/** A step's words: its label, or — for a code this app does not know — the neutral sentence, never the code. */
export function stepLabel(tt: string): string {
  return Object.prototype.hasOwnProperty.call(NHAN_BUOC, tt) ? NHAN_BUOC[tt]! : nhanTrangThai(tt);
}

/**
 * The main-path steps still AHEAD of `tt`, in order — drawn greyed with "chưa tới bước này" (`PROTOTYPE.md`
 * §6.4). PURE, exported for tests. `buocDaQua` is untouched: it still says only what has happened.
 *
 * Nothing ahead for the two terminal branches (they left the main path), for an unknown code (its place on
 * the path is not known, and guessing it would promise steps that may never come), and after the last step.
 */
export function stepsAhead(tt: string): string[] {
  const i = VONG_DOI.indexOf(tt);
  return i < 0 ? [] : VONG_DOI.slice(i + 1);
}

/**
 * The timeline (§6.4): a 32px dot per step. Reached steps — the current one included — are green-ink with a
 * ✓; the current step also carries `aria-current` and its explanation. Steps ahead are greyed with the words
 * "chưa tới bước này", so the state is said in words and not by the grey alone. The only time shown is the
 * one the API has (sent at, on the first step); none is invented for the others.
 */
export function DongThoiGian({ petition }: { petition: PhieuCuaToi }) {
  const steps = buocDaQua(petition.trang_thai);
  const ahead = stepsAhead(petition.trang_thai);
  return (
    <ol className="xa-dong-tg">
      {steps.map((tt, i) => {
        const last = i === steps.length - 1;
        return (
          <li key={tt} className="xa-dong-tg__buoc xa-dong-tg__buoc--qua" aria-current={last ? "step" : undefined}>
            <span className="xa-dong-tg__dot" aria-hidden="true">
              <BieuTuong ten="check" co={18} stroke={2.6} />
            </span>
            <span className="xa-dong-tg__text">
              <strong>{stepLabel(tt)}</strong>
              <span className="xa-phu">{XA_PA.don_vi_xu_ly}</span>
              {i === 0 && <span className="xa-phu">{at(petition.goc_dem_han)}</span>}
              {last && giaiThichTrangThai(tt) && <span className="xa-dong-tg__note">{giaiThichTrangThai(tt)}</span>}
            </span>
          </li>
        );
      })}
      {ahead.map((tt) => (
        <li key={tt} className="xa-dong-tg__buoc xa-dong-tg__buoc--cho">
          <span className="xa-dong-tg__dot" aria-hidden="true" />
          <span className="xa-dong-tg__text">
            <span>
              {stepLabel(tt)} — {XA_PA.step_not_reached}
            </span>
          </span>
        </li>
      ))}
    </ol>
  );
}

/**
 * The petition as the server returns it to its sender — no staff notes, no routing history (rule 4
 * forbidden #5). Rating is `PetitionRating`, the same block as the shared app: one network path, one rule.
 */
export function PetitionBody(props: {
  petition: PhieuCuaToi;
  /** Absent = read-only (no rating block), e.g. in a test. */
  onRated?: (p: PhieuCuaToi) => void;
  onReload?: () => void;
  reopenWithPhone?: ReopenWithPhone;
}) {
  const p = props.petition;
  const sender = p.an_danh
    ? XA_PA.giau_ten
    : [p.ho_ten_da_che || THE_PHIEU.khong_ghi_ten, p.dien_thoai_da_che].filter(Boolean).join(" · ");
  return (
    <>
      {/* The summary card (§6.4): field and status on top, the content, then the facts with their icons. */}
      <div className="xa-the xa-the--dem xa-khoi">
        <div className="xa-summary-head">
          <strong className="xa-summary-head__field">{nhanLinhVuc(p.linh_vuc, p.nhan_linh_vuc)}</strong>
          <ChipTrangThai tt={p.trang_thai} />
        </div>
        <p className="xa-phu">
          {XA_PA.ma_phieu}: <strong className="xa-ma">#{p.ma_tra_cuu}</strong>
        </p>
        <p className="xa-giu-dong">{p.noi_dung}</p>
        <dl className="xa-facts">
          <Fact icon="pin" label={THE_PHIEU.dia_chi}>
            {p.dia_chi || THE_PHIEU.dia_chi_trong}
          </Fact>
          <Fact icon="clock" label={THE_PHIEU.gui_luc}>
            {at(p.goc_dem_han)}
          </Fact>
          {/* The server's stored deadline, verbatim — never computed here (rule 10 #2, #4). */}
          <Fact icon="calendar" label={XA_PA.du_kien_xong}>
            {p.han_xu_ly_xong ? at(p.han_xu_ly_xong) : THE_PHIEU.han_xu_ly_chua_co}
          </Fact>
          <Fact icon="user" label={THE_PHIEU.nguoi_gui}>
            {sender}
          </Fact>
        </dl>
        {p.trang_thai === "khong-tiep-nhan" && <Row label={THE_PHIEU.ly_do_khong_tiep_nhan}>{p.ly_do || XA_PA.chua_ghi}</Row>}
        {p.trang_thai === "chuyen-cap-tren" && (
          <>
            <Row label={THE_PHIEU.co_quan_tiep_nhan}>{p.co_quan_nhan || XA_PA.chua_ghi}</Row>
            <Row label={THE_PHIEU.ly_do_chuyen}>{p.ly_do || XA_PA.chua_ghi}</Row>
          </>
        )}
      </div>
      {/* The commune's result, in its own green frame (§6.4) — only when the commune wrote one. */}
      {p.ket_qua !== "" && (
        <section className="xa-result-box" aria-labelledby="xa-ket-qua-tieu-de">
          <h2 className="xa-dau-khoi__tieu-de" id="xa-ket-qua-tieu-de">
            {THE_PHIEU.ket_qua}
          </h2>
          <p className="xa-giu-dong">{p.ket_qua}</p>
        </section>
      )}
      <div className="xa-the xa-the--dem xa-khoi">
        <h2 className="xa-dau-khoi__tieu-de">{XA_PA.tien_trinh}</h2>
        <DongThoiGian petition={p} />
      </div>
      {props.onRated && props.onReload && (
        // `key` by status: a server-side change (a reopen after a low rating) starts a fresh block.
        <PetitionRating
          key={`${p.ma_tra_cuu}:${p.trang_thai}`}
          petition={p}
          onRated={props.onRated}
          onReload={props.onReload}
          reopenWithPhone={props.reopenWithPhone}
        />
      )}
    </>
  );
}

/** Result of reading ONE petition by code → what the screen shows. PURE. */
export type LookupOutcome =
  | { readonly kind: "found"; readonly petition: PhieuCuaToi }
  | { readonly kind: "session" }
  | { readonly kind: "failed"; readonly text: string };

export function lookupOutcome(kq: KetQuaGoi): LookupOutcome {
  switch (kq.kieu) {
    case "xong":
      return { kind: "found", petition: kq.phieu };
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return { kind: "session" };
    case "khong-thay":
      return { kind: "failed", text: XA_PA.khong_thay_phieu };
    case "chua-cau-hinh":
      return { kind: "failed", text: KENH_CHUA_MO.cau };
    case "loi-mang":
      return { kind: "failed", text: TRA_CUU.loi_mang };
    default:
      return { kind: "failed", text: TRA_CUU.loi_may_chu };
  }
}

type DetailState =
  | { readonly kind: "loading" }
  | { readonly kind: "session" }
  | { readonly kind: "failed"; readonly text: string }
  | { readonly kind: "found"; readonly petition: PhieuCuaToi };

export function PetitionDetail(props: {
  code: string;
  onBack: () => void;
  onSessionLost: OnSessionLost;
  /** A rating or reload changed the petition — the list refreshes. */
  onChanged: () => void;
  reopenWithPhone?: ReopenWithPhone;
}) {
  const { code } = props;
  const [state, setState] = useState<DetailState>({ kind: "loading" });
  const [round, setRound] = useState(0);

  useEffect(() => {
    let alive = true;
    setState({ kind: "loading" });
    void traCuuPhieu(code).then((kq) => {
      if (!alive) return;
      const out = lookupOutcome(kq);
      setState(out);
      if (out.kind === "session") props.onSessionLost(() => setRound((n) => n + 1));
    });
    return () => {
      alive = false;
    };
    // `round` is the reload trigger; `code` is fixed for this screen.
  }, [code, round]);

  const reload = () => setRound((n) => n + 1);

  return (
    <>
      <DauManCon tieu_de={XA_PA.ticket_title(code)} onQuayLai={props.onBack} />
      <TrangCon>
        {state.kind === "loading" && <KhoiTrangThai bieu_tuong="chat" cau={XA_PA.loading_ticket} dang_tai />}
        {state.kind === "session" && (
          <KhoiTrangThai bieu_tuong="alert" loi cau={XA_PA.session_expired} nut={{ nhan: CUA_TOI.nut_thu_lai, onBam: reload }} />
        )}
        {state.kind === "failed" && (
          <KhoiTrangThai bieu_tuong="alert" loi cau={state.text} nut={{ nhan: CUA_TOI.nut_thu_lai, onBam: reload }} />
        )}
        {state.kind === "found" && (
          <PetitionBody
            petition={state.petition}
            onRated={(p) => {
              setState({ kind: "found", petition: p });
              props.onChanged();
            }}
            onReload={reload}
            reopenWithPhone={props.reopenWithPhone}
          />
        )}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ TRA CỨU PHIẾU ═══════════════════════════════ */

/** The typed code, as the route takes it: trimmed, a leading "#" (as the card shows it) removed. */
export function normaliseCode(typed: string): string {
  return typed.trim().replace(/^#/, "").trim();
}

export function PetitionLookup(props: {
  onBack: () => void;
  onSessionLost: OnSessionLost;
  onChanged: () => void;
  reopenWithPhone?: ReopenWithPhone;
}) {
  const [code, setCode] = useState("");
  const [state, setState] = useState<DetailState | { readonly kind: "missing" } | null>(null);

  async function search(value: string) {
    if (value === "") {
      setState({ kind: "missing" });
      return;
    }
    setState({ kind: "loading" });
    const out = lookupOutcome(await traCuuPhieu(value));
    setState(out);
    if (out.kind === "session") props.onSessionLost(() => void search(value));
  }

  return (
    <>
      <DauManCon tieu_de={XA_PA.tra_cuu_tieu_de} onQuayLai={props.onBack} />
      <TrangCon>
        <div className="xa-the xa-the--dem xa-khoi">
          <ONhapDong id="xa-ma-tra-cuu" nhan={XA_PA.ma_phieu} goi_y={XA_PA.tra_cuu_goi_y} gia_tri={code} toi_da={40} onDoi={setCode} />
          <button
            type="button"
            className="xa-nut"
            disabled={state?.kind === "loading"}
            onClick={() => void search(normaliseCode(code))}
          >
            <BieuTuong ten="search" co={20} />
            {TRA_CUU.nut_tra}
          </button>
        </div>
        {state?.kind === "missing" && <KhoiTrangThai bieu_tuong="info" loi cau={XA_PA.thieu_ma} />}
        {state?.kind === "loading" && <KhoiTrangThai bieu_tuong="search" cau={TRA_CUU.dang_tra} dang_tai />}
        {state?.kind === "session" && <KhoiTrangThai bieu_tuong="alert" loi cau={XA_PA.session_expired} />}
        {state?.kind === "failed" && <KhoiTrangThai bieu_tuong="search" loi cau={state.text} />}
        {state?.kind === "found" && (
          <PetitionBody
            petition={state.petition}
            onRated={(p) => {
              setState({ kind: "found", petition: p });
              props.onChanged();
            }}
            onReload={() => void search(state.petition.ma_tra_cuu)}
            reopenWithPhone={props.reopenWithPhone}
          />
        )}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ GỬI — Lĩnh vực → Mô tả → Đã gửi ═══════════════════════════════ */

/**
 * The step bar (`PROTOTYPE.md` §6.2): three 32px circles joined by lines, each with its label BESIDE it.
 * Done = green-ink with a ✓ (and the line after it green); current = brand red (owner's decision, not the
 * prototype's blue) with a bold label and `aria-current`; ahead = page tint with a border. The step is said
 * in words too — the list's name "Bước x/3" — never by colour alone. Exported for tests.
 */
export function StepBar({ step }: { step: 1 | 2 | 3 }) {
  const labels = [XA_TN.buoc_linh_vuc, XA_PA.buoc_mo_ta, XA_PA.buoc_xong];
  return (
    <ol className="xa-thanh-buoc" aria-label={XA_TN.buoc(step, 3)}>
      {labels.map((n, i) => {
        const no = i + 1;
        const tt = no < step ? "xong" : no === step ? "dang" : "cho";
        return (
          <li key={n} className={`xa-thanh-buoc__muc xa-thanh-buoc__muc--${tt}`} aria-current={tt === "dang" ? "step" : undefined}>
            <span className="xa-thanh-buoc__cham">{tt === "xong" ? <BieuTuong ten="check" co={18} stroke={2.6} /> : no}</span>
            <span className="xa-thanh-buoc__label">{n}</span>
          </li>
        );
      })}
    </ol>
  );
}

/** "Bà con" words for the shared location control (`scene-location.tsx`), from `XA_TN` / `XA_PA`. */
export const COMMUNE_LOCATION_WORDS: SceneLocationWords = {
  button: XA_TN.vi_tri_nut,
  button_again: XA_TN.location_again,
  locating: XA_TN.vi_tri_dang_lay,
  why: XA_PA.vi_tri_vi_sao,
  found: XA_TN.location_found,
  zalo_failed: XA_TN.location_zalo_failed,
  failures: {
    "tu-choi": XA_TN.vi_tri_tu_choi,
    "ngoai-zalo": XA_TN.vi_tri_ngoai_zalo,
    "qua-nhieu-lan": XA_TN.location_rate_limited,
    "thu-lai": XA_TN.vi_tri_khong_lay_duoc,
    "tam-ngung": XA_TN.location_unavailable,
  },
};

/* ───────────── THE COMMUNE'S FIELD CATALOGUE (step 1) — `GET /api/v1/my-citizen-report-fields` ───────────── */

export type CatalogueFailure = "unavailable" | "network" | "server" | "closed";

/**
 * The fields this commune offers on the form, loaded when the send screen opens — which is always AFTER
 * the gate, because the route needs a session. There is NO built-in list to fall back on (ADR 0060 §3):
 * failed means "say so, offer Thử lại", never "show twelve names the commune may not use".
 */
export type Catalogue =
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly failure: CatalogueFailure }
  | { readonly kind: "ready"; readonly fields: readonly CitizenField[] };

/** One catalogue call's result → the next state, or `"session"` (gate again). PURE. */
export function catalogueOutcome(kq: Awaited<ReturnType<typeof citizenReportFields>>): Catalogue | "session" {
  switch (kq.kieu) {
    case "xong":
      return { kind: "ready", fields: kq.fields };
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return "session";
    case "field-catalogue-unavailable":
      return { kind: "failed", failure: "unavailable" };
    case "chua-cau-hinh":
      return { kind: "failed", failure: "closed" };
    case "loi-mang":
      return { kind: "failed", failure: "network" };
    default:
      return { kind: "failed", failure: "server" };
  }
}

export function catalogueFailureText(f: CatalogueFailure): string {
  switch (f) {
    case "unavailable":
      return XA_PA.fields_unavailable;
    case "network":
      return XA_PA.fields_network;
    case "closed":
      return KENH_CHUA_MO.cau;
    case "server":
      return XA_PA.fields_server;
  }
}

/** The catalogue of this send screen. Loaded once on mount (ref guard: StrictMode runs effects twice). */
function useFieldCatalogue(onSessionLost: OnSessionLost) {
  const [catalogue, setCatalogue] = useState<Catalogue>({ kind: "loading" });
  const busy = useRef(false);

  async function load() {
    if (busy.current) return;
    busy.current = true;
    setCatalogue({ kind: "loading" });
    const next = catalogueOutcome(await citizenReportFields());
    busy.current = false;
    if (next === "session") {
      onSessionLost(() => void load());
      return;
    }
    setCatalogue(next);
  }

  const started = useRef(false);
  useEffect(() => {
    if (started.current) return;
    started.current = true;
    void load();
  }, []);

  return { catalogue, reload: () => void load() };
}

/** The codes the commune offers now, or `null` while not known (loading / failed). */
export function offeredCodes(c: Catalogue): readonly string[] | null {
  return c.kind === "ready" ? c.fields.map((f) => f.code) : null;
}

/**
 * The platform's lucide icon names (`service-platform/migrations/0011_petition_field.sql:134-145`, and the
 * sixteen `PROTOTYPE.md` §8 says the table must cover) → this app's drawing of the same lucide shape
 * (`BieuTuong.tsx`). Anything else — including a name added to the platform later — gets the NEUTRAL
 * `message-square-plus` (§8's fallback), never a guessed one.
 */
const FIELD_ICON: Readonly<Record<string, TenBieuTuong>> = {
  Trash2: "trash",
  TrafficCone: "traffic-cone",
  Leaf: "leaf",
  Droplets: "droplets",
  Lightbulb: "lightbulb",
  Store: "store",
  ShieldAlert: "shield-alert",
  MessageSquarePlus: "message-square-plus",
  Construction: "construction",
  Factory: "factory",
  Hammer: "hammer",
  MessageSquare: "message-square",
  Stethoscope: "stethoscope",
  UserRoundX: "user-round-x",
  Utensils: "utensils",
  Zap: "zap",
};

/**
 * The platform's six tones → this app's measured colour classes (`xa-mau--*`, one per tone since 30/09/2026 —
 * cyan and red were blue and pink); unknown/absent → neutral.
 */
const FIELD_TONE: Readonly<Record<string, Tone>> = {
  blue: "xanh",
  cyan: "cyan",
  green: "luc",
  orange: "cam",
  purple: "tim",
  red: "red",
};

export function fieldIcon(icon: string | null): TenBieuTuong {
  return icon !== null && Object.prototype.hasOwnProperty.call(FIELD_ICON, icon) ? FIELD_ICON[icon]! : "message-square-plus";
}

export function fieldTone(tone: string | null): Tone {
  return tone !== null && Object.prototype.hasOwnProperty.call(FIELD_TONE, tone) ? FIELD_TONE[tone]! : "navy";
}

/**
 * "Tiếp tục" on a found draft → the form to show and the step to open. PURE, exported for tests.
 *
 * The draft keeps a field CODE. It is restored only while it is still OFFERED (`offered`): a code the
 * commune has since switched off — or a name left by the old temporary list — is dropped and the citizen
 * picks again on step 1. `offered` null (catalogue not loaded yet): the code is kept for now, and the send
 * screen drops it the moment the catalogue arrives without it. Otherwise step 2, the writing step, as the
 * prototype does (`NewFeedbackPage.tsx:222`). An empty name in the draft (an anonymous one keeps none)
 * falls back to the name taken from Zalo in this session.
 */
export function restoreDraft(
  draft: NhapPhieu,
  zaloName: string | null,
  offered: readonly string[] | null,
): { form: NhapPhieu; step: 1 | 2 } {
  const field = offered === null || offered.includes(draft.linh_vuc) ? draft.linh_vuc : "";
  return {
    form: {
      linh_vuc: field,
      noi_dung: draft.noi_dung,
      dia_chi: draft.dia_chi,
      ho_ten: draft.ho_ten !== "" ? draft.ho_ten : (zaloName ?? ""),
      dien_thoai: draft.dien_thoai,
      an_danh: draft.an_danh,
    },
    step: field !== "" ? 2 : 1,
  };
}

/**
 * The form a fresh send screen opens with. PURE, exported for tests. The name field starts with the Zalo
 * name taken at entry, or EMPTY — never a placeholder name: the field is required when not anonymous
 * (`kiemNhapPhieu`), so an empty field makes the citizen type it, while a guessed one would be sent.
 * The name PRE-FILLS; it grants nothing — who sent it is the session's citizen, on the server (rule 4).
 */
export function blankForm(nameFromEntry: string | null): NhapPhieu {
  return {
    linh_vuc: "",
    noi_dung: "",
    dia_chi: "",
    ho_ten: nameFromEntry ?? "",
    // Empty in every build but `--demo`, whose fixed identity pre-fills its number (ADR 0047 §6, 01/10/2026).
    dien_thoai: DEMO_BUILD ? DEMO_CITIZEN_PHONE : "",
    // Gửi ẩn danh là tuỳ chọn của bà con (SRS M4.2, ADR 0050 #3): bật thì không gửi họ tên, số điện thoại.
    an_danh: false,
  };
}

/**
 * The request body for this form + location. PURE. The five fields of the contract, `field` = the CODE
 * picked on step 1 (from the commune's catalogue), plus `lat`/`lng` only when the citizen tapped for them.
 */
export function sendBody(form: NhapPhieu, location: SceneLocation | null): string {
  return thanGuiPhanAnh({
    noi_dung: form.noi_dung,
    dia_chi: form.dia_chi,
    ho_ten: form.ho_ten,
    dien_thoai: form.dien_thoai,
    an_danh: form.an_danh,
    scene_location: location,
    field: form.linh_vuc,
  });
}

type SendFailure = keyof typeof LOI_GUI;

/** One send's result → what the screen does next. PURE. */
export type SendOutcome =
  | { readonly kind: "sent"; readonly petition: PhieuCuaToi }
  | { readonly kind: "session" }
  | { readonly kind: "failed"; readonly failure: SendFailure };

export function sendOutcome(kq: KetQuaGoi): SendOutcome {
  switch (kq.kieu) {
    case "xong":
      return { kind: "sent", petition: kq.phieu };
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      // Asked again through the gate with the SAME attempt: a 401/403 is answered before anything is
      // recorded, so the retry is this send, not a second petition.
      return { kind: "session" };
    case "chua-cau-hinh":
      return { kind: "failed", failure: "kenh-chua-mo" };
    case "khong-thay":
      return { kind: "failed", failure: "loi-may-chu" };
    default:
      return { kind: "failed", failure: kq.kieu };
  }
}

/**
 * Step 1 — the commune's own fields, in its order, with the platform's icon and tone where this app can
 * draw them (neutral otherwise). Loading and failures are WORDS with a next step; an empty catalogue says
 * the commune takes no petitions through the app — it never invents a list to pick from.
 */
export function FieldStep(props: {
  catalogue: Catalogue;
  picked: string;
  /** The last send was refused with `field_not_offered`: explain before the list. */
  fieldChanged: boolean;
  onPick: (code: string) => void;
  onRetry: () => void;
}) {
  const { catalogue } = props;
  if (catalogue.kind === "loading") return <KhoiTrangThai bieu_tuong="text" cau={XA_PA.fields_loading} dang_tai />;
  if (catalogue.kind === "failed") {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={catalogueFailureText(catalogue.failure)}
        nut={catalogue.failure === "closed" ? undefined : { nhan: CUA_TOI.nut_thu_lai, onBam: props.onRetry }}
      />
    );
  }
  if (catalogue.fields.length === 0) return <KhoiTrangThai bieu_tuong="info" cau={XA_PA.fields_empty} />;
  // `CategoryGrid` of §6.2: two columns, each tile filled with its tone, the 32px icon in the tone's ink, the
  // name bold and centred in body ink. The picked tile (after "Đổi") adds a brand border and `aria-checked`.
  return (
    <div className="xa-khoi">
      {props.fieldChanged && (
        <p className="xa-error-box" role="alert">
          {LOI_GUI["field-not-offered"].cau}
        </p>
      )}
      <p className="xa-phu">{XA_PA.chon_linh_vuc}</p>
      <div className="xa-luoi-lv" role="radiogroup" aria-label={XA_TN.buoc_linh_vuc}>
        {catalogue.fields.map((f) => (
          <button
            key={f.code}
            type="button"
            role="radio"
            aria-checked={props.picked === f.code}
            className={`xa-o-lv xa-mau--${fieldTone(f.tone)}${props.picked === f.code ? " xa-o-lv--on" : ""}`}
            onClick={() => props.onPick(f.code)}
          >
            <BieuTuong ten={fieldIcon(f.icon)} co={32} />
            <span className="xa-o-lv__label">{f.label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

export function CommuneSendScreen(props: {
  ten_xa: string;
  /** Họ tên lấy từ Zalo lúc mở app, hoặc `null`. CHỈ để điền sẵn — màn này không gọi Zalo. */
  ho_ten: string | null;
  /** The location exchange, injected by the shell; absent = no location button (outside Zalo, tests). */
  getSceneLocation?: GetSceneLocation;
  /** Draft kept on the phone (ADR 0050 #7) — commune app only. Absent: no draft at all. */
  draftStore?: FeedbackDraftStore;
  onBack: () => void;
  onSessionLost: OnSessionLost;
  /** 201 — the petition as the server recorded it. */
  onSent: (petition: PhieuCuaToi) => void;
  onOpenPetition: (code: string) => void;
}) {
  const { draftStore } = props;
  const [step, setStep] = useState<1 | 2 | 3>(1);
  /**
   * A draft found when the screen opened, until the citizen picks "Tiếp tục" or "Bỏ nháp". While it is
   * pending the form is hidden and nothing is saved, so the old draft cannot be overwritten by a new one
   * before the citizen has answered (the prototype asks first, `NewFeedbackPage.tsx:79`).
   */
  const [draftOffer, setDraftOffer] = useState<NhapPhieu | null>(() => draftStore?.load() ?? null);
  const [form, setForm] = useState<NhapPhieu>(() => blankForm(props.ho_ten));
  const [location, setLocation] = useState<SceneLocation | null>(null);
  const [errors, setErrors] = useState<LoiNhapPhieu>({});
  const [confirmCancel, setConfirmCancel] = useState(false);
  /** The attempt in flight — kept across "Gửi lại" and the gate, dropped when what is sent changes. */
  const [attempt, setAttempt] = useState<LanGui | null>(null);
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<SendFailure | null>(null);
  const [sent, setSent] = useState<PhieuCuaToi | null>(null);
  /** The server said the picked field is no longer offered: say so on step 1 while the citizen re-picks. */
  const [fieldChanged, setFieldChanged] = useState(false);
  /**
   * Step 2 was reached by TAPPING a field (§6.2: the tap goes straight on to writing), so the description box
   * takes the focus. Not after resuming a draft: the keyboard would then cover a
   * form the citizen has not looked at yet.
   */
  const [focusDescribe, setFocusDescribe] = useState(false);
  const { catalogue, reload } = useFieldCatalogue(props.onSessionLost);
  const sceneLocation = useSceneLocation(props.getSceneLocation, (l) => {
    setLocation(l);
    setAttempt(null);
  });

  // A picked code the catalogue does not (or no longer) offer is dropped as soon as the catalogue is known —
  // a restored draft's code, or one the commune switched off. Back to step 1; what was written stays.
  useEffect(() => {
    const offered = offeredCodes(catalogue);
    if (offered === null || form.linh_vuc === "" || offered.includes(form.linh_vuc)) return;
    setForm((t) => ({ ...t, linh_vuc: "" }));
    setAttempt(null);
    setStep((s) => (s === 2 ? 1 : s));
  }, [catalogue, form.linh_vuc]);

  const pickedLabel =
    catalogue.kind === "ready" ? (catalogue.fields.find((f) => f.code === form.linh_vuc)?.label ?? "") : "";

  const edit = (k: "noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai") => (v: string) => {
    setForm((t) => ({ ...t, [k]: v }));
    setAttempt(null);
    setFailure(null);
  };
  const hasContent = form.linh_vuc !== "" || form.noi_dung.trim() !== "" || form.dia_chi.trim() !== "";

  // Save as the citizen types, on the writing step only (the prototype's rule, `NewFeedbackPage.tsx:124`).
  // Not while a found draft is still waiting for an answer, and not after sending (step 3).
  useEffect(() => {
    if (draftStore === undefined || draftOffer !== null || step !== 2 || form.linh_vuc === "") return;
    draftStore.save(form);
  }, [draftStore, draftOffer, step, form]);

  function resumeDraft() {
    if (draftOffer === null) return;
    const restored = restoreDraft(draftOffer, props.ho_ten, offeredCodes(catalogue));
    setForm(restored.form);
    setDraftOffer(null);
    setStep(restored.step);
  }

  function discardDraft() {
    draftStore?.clear();
    setDraftOffer(null);
  }

  /** "Huỷ bỏ" in the cancel dialog: what was typed is dropped — on the phone too. */
  function cancelFeedback() {
    draftStore?.clear();
    props.onBack();
  }

  async function send(a: LanGui) {
    setSending(true);
    setFailure(null);
    const out = sendOutcome(await guiPhanAnh(a));
    setSending(false);
    if (out.kind === "session") {
      props.onSessionLost(() => void send(a));
      return;
    }
    if (out.kind === "failed" && out.failure === "field-not-offered") {
      // The commune changed its list since it was loaded. Reload it and let the citizen pick again; the
      // attempt is dropped — the next send has a different body, so it is a different act (`lan-gui.ts`).
      setAttempt(null);
      setForm((t) => ({ ...t, linh_vuc: "" }));
      setFieldChanged(true);
      setStep(1);
      reload();
      return;
    }
    if (out.kind === "failed") {
      setFailure(out.failure);
      return;
    }
    setAttempt(null);
    draftStore?.clear();
    setSent(out.petition);
    setStep(3);
    props.onSent(out.petition);
  }

  function submit() {
    if (sending) return;
    const e = kiemNhapPhieu(form, {
      thieu: XA_PA.thieu_mo_ta,
      thieu_nguoi_gui: XA_PA.thieu_nguoi_gui,
      qua_dai: (n) => GUI.qua_dai(XA_TN.o_nay, n),
    });
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    let a = attempt;
    if (a === null) {
      try {
        a = taoLanGui(sendBody(form, location));
      } catch {
        setFailure("khong-tao-duoc-khoa");
        return;
      }
      setAttempt(a);
    }
    void send(a);
  }

  function back() {
    if (step === 2) return setStep(1);
    if (step === 1 && hasContent) return setConfirmCancel(true);
    props.onBack();
  }

  const failed = failure === null ? null : LOI_GUI[failure];

  return (
    <>
      <DauManCon tieu_de={GUI.tieu_de} onQuayLai={step === 3 ? props.onBack : back} />
      <StepBar step={step} />
      <TrangCon>
        {confirmCancel && (
          <div className="xa-the xa-the--dem xa-khoi" role="alertdialog" aria-label={XA_TN.hoi_huy_tieu_de}>
            <h2 className="xa-dau-khoi__tieu-de">{XA_TN.hoi_huy_tieu_de}</h2>
            <p>{XA_PA.hoi_huy_cau}</p>
            <button type="button" className="xa-nut" onClick={() => setConfirmCancel(false)}>
              {XA_TN.tiep_tuc_nhap}
            </button>
            <button type="button" className="xa-nut xa-nut--phu xa-nut--do-vien" onClick={cancelFeedback}>
              {XA_TN.huy_bo}
            </button>
          </div>
        )}

        {draftOffer !== null && (
          <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-nhap-tieu-de">
            <h2 className="xa-dau-khoi__tieu-de" id="xa-nhap-tieu-de">
              {XA_PA.draft_title}
            </h2>
            <p>{XA_PA.draft_body}</p>
            <p className="xa-phu">{XA_PA.draft_kept_on_phone}</p>
            <button type="button" className="xa-nut" onClick={resumeDraft}>
              {XA_PA.draft_resume}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={discardDraft}>
              {XA_PA.draft_discard}
            </button>
          </section>
        )}

        {step === 1 && draftOffer === null && (
          <FieldStep
            catalogue={catalogue}
            picked={form.linh_vuc}
            fieldChanged={fieldChanged}
            onRetry={reload}
            onPick={(code) => {
              setForm((t) => ({ ...t, linh_vuc: code }));
              setAttempt(null);
              setFieldChanged(false);
              setFocusDescribe(true);
              setStep(2);
            }}
          />
        )}

        {step === 2 && (
          <div className="xa-the xa-the--dem xa-khoi">
            <ONhapDoan
              id="xa-noi-dung"
              nhan={XA_PA.su_viec}
              goi_y={XA_PA.goi_y_su_viec}
              gia_tri={form.noi_dung}
              toi_da={DO_DAI_TOI_DA.noi_dung}
              bat_buoc
              autoFocus={focusDescribe}
              onDoi={edit("noi_dung")}
            />
            {errors.noi_dung && <p className="xa-loi-o" role="alert">{errors.noi_dung}</p>}
            {/* The picked field, and the way back to change it (§6.2): a real 48px button, not a link. Only when
                a field IS picked — a blank "Lĩnh vực:" would read as a field that failed to load. */}
            {form.linh_vuc !== "" && (
              <div className="xa-field-picked">
                <span className="xa-field-picked__text">
                  {XA_PA.linh_vuc}: <strong>{pickedLabel}</strong>
                </span>
                <button type="button" className="xa-field-change" onClick={() => setStep(1)}>
                  {XA_TN.doi}
                </button>
              </div>
            )}
            {/* SRS M4.2 bắt buộc ảnh/video và vị trí trên bản đồ. Ảnh CHƯA có; vị trí hiện tại lấy được
                nhưng chưa có bản đồ. Không chặn nút gửi vì hai ô ấy (xem `kiemNhapPhieu`). The photo words stay
                until the photo card: no picker frame is drawn for a feature that cannot send a photo yet. */}
            <p className="xa-nhan-o">{XA_PA.anh_bat_buoc}</p>
            <p className="xa-phu">{XA_PA.anh_sap_co}</p>
            <p className="xa-nhan-o">{XA_PA.vi_tri_bat_buoc}</p>
            <p className="xa-phu">{XA_PA.vi_tri_sap_co}</p>
            <ONhapDong id="xa-dia-chi" nhan={XA_PA.dia_chi} goi_y={XA_PA.goi_y_dia_chi} gia_tri={form.dia_chi} toi_da={DO_DAI_TOI_DA.dia_chi} onDoi={edit("dia_chi")} />
            {errors.dia_chi && <p className="xa-loi-o" role="alert">{errors.dia_chi}</p>}
            {props.getSceneLocation && (
              <SceneLocationControl
                words={COMMUNE_LOCATION_WORDS}
                look="commune"
                locating={sceneLocation.locating}
                location={location}
                failure={sceneLocation.failure}
                zalo={sceneLocation.zalo}
                onLocate={() => void sceneLocation.locate()}
              />
            )}
            <div className="xa-hang xa-hang--tinh xa-hang--sat">
              <span className="xa-hang__chu">
                <strong>{XA_PA.an_danh}</strong>
                <span className="xa-phu">{XA_PA.an_danh_giai_thich}</span>
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={form.an_danh}
                aria-label={XA_PA.an_danh}
                className={`xa-cong-tac${form.an_danh ? " xa-cong-tac--bat" : ""}`}
                onClick={() => {
                  setForm((t) => ({ ...t, an_danh: !t.an_danh }));
                  setAttempt(null);
                }}
              >
                <span className="xa-cong-tac__nut" />
              </button>
            </div>
            {!form.an_danh && (
              <>
                <ONhapDong id="xa-ho-ten" nhan={XA_PA.ten_nguoi_pa} goi_y={XA_PA.goi_y_ten} gia_tri={form.ho_ten} toi_da={DO_DAI_TOI_DA.ho_ten} onDoi={edit("ho_ten")} />
                {errors.ho_ten && <p className="xa-loi-o" role="alert">{errors.ho_ten}</p>}
                <ONhapDong id="xa-dien-thoai" nhan={XA_PA.so_dien_thoai} goi_y={XA_PA.goi_y_so} gia_tri={form.dien_thoai} toi_da={DO_DAI_TOI_DA.dien_thoai} kieu_ban_phim="tel" onDoi={edit("dien_thoai")} />
                {errors.dien_thoai && <p className="xa-loi-o" role="alert">{errors.dien_thoai}</p>}
              </>
            )}
            <p className="xa-phu">{form.an_danh ? XA_PA.bat_buoc_an_danh : XA_PA.bat_buoc}</p>
            {draftStore !== undefined && <p className="xa-phu">{XA_PA.draft_kept_on_phone}</p>}
            <div className="xa-ghi-chu">
              <BieuTuong ten="alert" co={22} />
              <p>{KHAN_CAP}</p>
            </div>
          </div>
        )}

        {step === 3 && sent !== null && (
          <SendDone petition={sent} onFollow={() => props.onOpenPetition(sent.ma_tra_cuu)} onHome={props.onBack} />
        )}
      </TrangCon>

      {/* THE SEND FOOTER (§6.2) — a block of the frame column under the scrolling form, so the button is always
          where the thumb is. Above the button, in reading order: THE COMMUNE, READ AGAIN AT THE LAST STEP
          (README §Non-negotiables #5 — the name on the header, which the session was checked against when it
          opened, `openCommuneAppSession`), then the server's refusal in a red box, then "đang gửi". */}
      {step === 2 && (
        <div className="xa-chan-gui xa-send-footer">
          <p className="xa-xa-nhan">{XA_PA.gui_toi(props.ten_xa)}</p>
          {failed !== null && (
            <p className="xa-error-box" role="alert">
              {failed.cau}
            </p>
          )}
          {sending && (
            <p className="xa-phu" role="status">
              {XA_PA.dang_gui}
            </p>
          )}
          {failed !== null && failed.co_the_gui_lai && attempt !== null ? (
            // Same key, same body — never a second petition (`api/lan-gui.ts`).
            <button type="button" className="xa-nut" disabled={sending} onClick={() => void send(attempt)}>
              <BieuTuong ten="send" co={20} />
              {GUI.nut_gui_lai}
            </button>
          ) : (
            <button type="button" className="xa-nut" onClick={submit} disabled={sending || form.noi_dung.trim() === ""}>
              <BieuTuong ten="send" co={20} />
              {GUI.tieu_de}
            </button>
          )}
        </div>
      )}
    </>
  );
}

/**
 * Step 3 — "Đã gửi" (§6.2), shown only after a 201: the check in its green circle (a ≤300ms pop, only under
 * `prefers-reduced-motion: no-preference`), the lookup code the server issued (rule 10 #1), and a blue band
 * with the deadlines the SERVER returned — `han_tiep_nhan`, and `han_xu_ly_xong` when it already has one.
 * Neither is ever computed here (rule 10 #2): an absent deadline is an absent line. Exported for tests.
 */
export function SendDone(props: { petition: PhieuCuaToi; onFollow: () => void; onHome: () => void }) {
  const p = props.petition;
  const acknowledge = p.han_tiep_nhan !== null ? thoiDiemVN(p.han_tiep_nhan) : null;
  const resolve = p.han_xu_ly_xong !== null ? thoiDiemVN(p.han_xu_ly_xong) : null;
  return (
    <div className="xa-ket-qua">
      <span className="xa-ket-qua__dau xa-pop" aria-hidden="true">
        <BieuTuong ten="check-circle" co={64} />
      </span>
      <h2 className="xa-bai__tieu-de">{XA_PA.xong_tieu_de}</h2>
      <p className="xa-phu">{XA_PA.xong_mo_ta}</p>
      <div className="xa-the xa-the--dem xa-ket-qua__ma" role="status">
        <p className="xa-phu">{XA_PA.ma_phieu_cua_ba_con}</p>
        <strong className="xa-ket-qua__code">#{p.ma_tra_cuu}</strong>
      </div>
      {(acknowledge !== null || resolve !== null) && (
        <div className="xa-deadline-band">
          {acknowledge !== null && <p>{XA_PA.acknowledge_by(acknowledge)}</p>}
          {resolve !== null && <p>{XA_PA.resolve_by(resolve)}</p>}
        </div>
      )}
      <button type="button" className="xa-nut" onClick={props.onFollow}>
        {XA_PA.theo_doi}
      </button>
      <button type="button" className="xa-nut xa-nut--phu" onClick={props.onHome}>
        {XA_TN.nut_ve_trang_chu}
      </button>
    </div>
  );
}
