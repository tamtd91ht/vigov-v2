"use client";

import { ArrowRight, CircleCheck, Pencil, Send, Trash2, X } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { PendingMarker } from "@/components/ui/pending-feature";
import { traTen, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import { nhanThoiDiem, staffNameWithCode, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import type {
  documents_danhSachLichSuChuyenRa,
  documents_lichSuChuyenRa,
  documents_vanBanDenRa,
} from "@/lib/api/schema.gen";

import {
  DANG_TAI_CHI_TIET,
  DANG_TAI_LICH_SU,
  ISSUED_ON_FIELD_LABEL,
  LICH_SU_RONG,
  LOI_THIEU_LY_DO_CHUYEN,
  NUT_DONG_CHI_TIET,
  NUT_GO,
  NUT_SUA,
  NUT_XAC_NHAN_CHUYEN,
  O_CAN_BO_XU_LY,
  O_CO_QUAN_BAN_HANH,
  O_DEN_BO_PHAN,
  O_LOAI_VAN_BAN,
  O_LY_DO_CHUYEN,
  REFERENCE_FIELD_LABEL,
  ROUTING_PERSON_PLACEHOLDER,
  ROUTING_REASON_PLACEHOLDER,
  SUMMARY_FIELD_LABEL,
  TIEU_DE_DONG_THOI_GIAN,
  TIEU_DE_KHOI_CHUYEN,
  deadlineDay,
  incomingDeadlineState,
  incomingStatusStep,
  nhanBoPhanDangGiu,
  nhanCanBo,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanSoVaoSo,
  nhanTieuDeVanBanDen,
  nhanTrangThai,
  nhanTuBoPhan,
  type BanChuyen,
} from "./nhan-van-ban";
import { RaiseTaskButton, StatusChangeRow, pendingPart } from "./document-pending";
import { DeadlineMark, DocumentStatusBadge, Glyph } from "./document-ui";

/** Heading id of the detail — the dialog's accessible name. */
export const INCOMING_DETAIL_TITLE_ID = "tieu-de-ngan-van-ban-den";

/**
 * Ngăn chi tiết MỘT văn bản đến — the PROTOTYPE's right-hand drawer (`DocumentDetailDrawer.tsx`,
 * ADR 0078), drawn in the shared `LargeDialog` (pinned right, full height, Esc and ✕ close it) at the
 * prototype's 72rem:
 *
 *   header      "Số đến 7/2026 · đến ngày …" (the dialog's NAME), the summary at 15px, the issuing
 *               body; Sửa · Gỡ khỏi sổ (`document.create`) · ✕ on the right
 *   status      the C2 strip — five disabled chips with one "?" — then the STORED status in words
 *   facts       three cells: Hạn xử lý · Bộ phận đang giữ · Nguồn vào sổ ("?")
 *   body        left (grey): trích yếu, the figures, `Chuyển thành nhiệm vụ` ("?"), the routing box;
 *               right (white, 24rem from 768px; under the left column below): the routing timeline
 *
 * THE DIALOG'S NAME IS THE NUMBER AND THE DATE, NEVER THE SUMMARY: a name goes into the accessibility
 * tree, and a summary is free text that may name a citizen (rule 3, forbidden #4).
 *
 * KHÔNG CÓ NÚT NÀO SỬA HAY XOÁ MỘT DÒNG LỊCH SỬ: bảng lịch sử chỉ-thêm ở tầng CSDL (trigger
 * `lich_su_chuyen_chi_them`) và hợp đồng không có tuyến nào làm việc ấy (luật 7, cấm #5).
 *
 * A successful routing is said IN the drawer (`cauDaXong`), not as a toast: the drawer is a native
 * modal in the top layer, and a toast would be drawn under its backdrop.
 */
export function NganVanBanDen({
  vb,
  lichSu,
  bayGio,
  traLoai,
  traBoPhan,
  coQuyenChuyen,
  ban,
  datBan,
  loi,
  dangGui,
  cauDaXong,
  onGui,
  onDong,
  danhBa = null,
  coQuyenGhi = false,
  onSua,
  onGo,
}: {
  /** `null` = đang đọc. Lỗi hiện NGUYÊN câu máy chủ — 404 là một câu chung cho mọi ca. */
  vb: KetQua<documents_vanBanDenRa> | null;
  lichSu: KetQua<documents_danhSachLichSuChuyenRa> | null;
  /** Thời điểm hiện tại TRUYỀN VÀO: quá hạn là phép so sánh lúc vẽ, không phải trường lưu sẵn. */
  bayGio: Date;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
  /** `document.route` — TIỆN DỤNG, không phải biện pháp: máy chủ kiểm lại trên từng yêu cầu. */
  coQuyenChuyen: boolean;
  ban: BanChuyen;
  datBan: (b: BanChuyen) => void;
  loi: string;
  dangGui: boolean;
  cauDaXong: string;
  onGui: () => void;
  onDong: () => void;
  /**
   * The commune's staff directory by code, read ONCE by the register (`GET /api/v1/staff-directory`).
   * `null` = not loaded or failed: every staff cell then shows the bare `CB-…` code (VBD-07).
   */
  danhBa?: DanhBaTheoMa | null;
  /** `document.create` — draws Sửa and Gỡ khỏi sổ in the header. UX only: the server checks again. */
  coQuyenGhi?: boolean;
  onSua?: (vb: documents_vanBanDenRa) => void;
  onGo?: (vb: documents_vanBanDenRa) => void;
}) {
  const doc = vb !== null && vb.ok ? vb.duLieu : null;
  const tieuDe = doc !== null ? nhanTieuDeVanBanDen(doc.number, doc.year, doc.received_date) : "Chi tiết văn bản đến";

  return (
    // The prototype's 72rem drawer (`DocumentDetailDrawer.tsx:76`), passed as a size — the shared
    // panel stays as it is for every other screen.
    <LargeDialog
      titleId={INCOMING_DETAIL_TITLE_ID}
      onDismiss={onDong}
      className="bg-white md:w-[min(72rem,98vw)] xl:w-[min(72rem,98vw)]"
    >
      <header className="flex shrink-0 items-start gap-3 border-0 border-b border-solid border-line bg-white px-5 py-4">
        <div className="min-w-0 flex-1">
          <h2
            id={INCOMING_DETAIL_TITLE_ID}
            tabIndex={-1}
            className="m-0 text-[11px] leading-snug font-semibold text-ink-muted tabular-nums outline-none"
          >
            {tieuDe}
          </h2>
          {doc !== null && (
            <>
              <p className="m-0 mt-0.5 text-[15px] leading-snug font-bold break-words text-navy">{doc.summary}</p>
              <p className="m-0 mt-1 text-[11.5px] text-ink-muted">{doc.issuing_body}</p>
            </>
          )}
        </div>
        {doc !== null && coQuyenGhi && (
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              icon={<Glyph icon={Pencil} />}
              aria-haspopup="dialog"
              onClick={() => onSua?.(doc)}
            >
              {NUT_SUA}
            </Button>
            <Button
              type="button"
              variant="danger"
              size="sm"
              icon={<Glyph icon={Trash2} />}
              aria-haspopup="dialog"
              onClick={() => onGo?.(doc)}
            >
              {NUT_GO}
            </Button>
          </div>
        )}
        {/* The prototype's ✕: an outline square button (`variant="outline" size="icon"`). */}
        <IconButton type="button" variant="secondary" size="md" label={NUT_DONG_CHI_TIET} onClick={onDong}>
          <X aria-hidden="true" />
        </IconButton>
      </header>

      {vb === null && (
        <div className="p-6">
          <p role="status" className="an-thi-giac">
            {DANG_TAI_CHI_TIET}
          </p>
          {/* The prototype's first-load blocks (`DocumentDetailDrawer.tsx:81-84`). */}
          <div aria-hidden="true" className="flex flex-col gap-3">
            <span className="block h-7 w-3/4 rounded-md bg-line motion-safe:animate-pulse" />
            <span className="block h-24 w-full rounded-md bg-line motion-safe:animate-pulse" />
          </div>
        </div>
      )}
      {vb !== null && !vb.ok && (
        <div className="p-5">
          <p className="thong-bao-loi m-0" role="alert">
            {vb.thongBao}
          </p>
        </div>
      )}

      {doc !== null && (
        // Below 768px the whole body scrolls as one column; from 768px the two columns scroll on their
        // own under a fixed strip and facts row, as in the prototype.
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:overflow-hidden">
          <div className="shrink-0 border-0 border-b border-solid border-line bg-white px-5 py-3">
            <StatusChangeRow current={incomingStatusStep(doc.status)} />
            {/* The STORED status, in words — always, since a code with no C2 step lights no chip. */}
            <p
              className="m-0 mt-2 flex flex-wrap items-center gap-2 text-[11.5px] text-ink-muted"
              aria-label="Trạng thái đang ghi"
            >
              <DocumentStatusBadge status={doc.status}>{nhanTrangThai(doc.status)}</DocumentStatusBadge>
            </p>
          </div>

          <FactsRow doc={doc} bayGio={bayGio} traBoPhan={traBoPhan} danhBa={danhBa} />

          <div className="flex min-w-0 flex-col md:min-h-0 md:flex-1 md:flex-row">
            <div className="min-w-0 flex-1 bg-canvas px-5 py-4 md:overflow-y-auto">
              <section aria-labelledby="tieu-de-trich-yeu-den">
                <h3 id="tieu-de-trich-yeu-den" className="m-0 mb-2 text-[12.5px] font-bold text-navy">
                  {SUMMARY_FIELD_LABEL}
                </h3>
                <p className="m-0 rounded-[10px] border border-solid border-line bg-white px-3 py-2.5 text-[13px] whitespace-pre-line text-navy">
                  {doc.summary}
                </p>
              </section>

              <dl className="m-0 mt-4 grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
                <Figure label={REFERENCE_FIELD_LABEL}>
                  {doc.reference_no === undefined || doc.reference_no === "" ? "Không ghi" : doc.reference_no}
                </Figure>
                <Figure label={ISSUED_ON_FIELD_LABEL}>{nhanNgayCoThe(doc.document_date ?? "")}</Figure>
                <Figure label={O_LOAI_VAN_BAN}>{nhanLoaiVanBan(traTen(traLoai, doc.document_type))}</Figure>
                <Figure label={O_CO_QUAN_BAN_HANH}>{doc.issuing_body}</Figure>
                <Figure label="Số đến">{nhanSoVaoSo(doc.number, doc.year)}</Figure>
                <Figure label="Ghi chú" pending={pendingPart("Ghi chú văn bản đến")}>
                  —
                </Figure>
              </dl>

              <div className="mt-4">
                <RaiseTaskButton />
              </div>

              {cauDaXong !== "" && (
                <p role="status" className="m-0 mt-4 flex items-center gap-2 text-[12.5px] font-medium text-success-600">
                  <Glyph icon={CircleCheck} className="size-4 shrink-0" />
                  {cauDaXong}
                </p>
              )}

              {coQuyenChuyen && (
                <KhoiChuyenXuLy
                  ban={ban}
                  datBan={datBan}
                  traBoPhan={traBoPhan}
                  danhBa={danhBa}
                  loi={loi}
                  dangGui={dangGui}
                  onGui={onGui}
                />
              )}
            </div>

            {/* The timeline in its own column on the right, as in the prototype: "where has this
                document been" is what the handler looks at most. Under the left column below 768px. */}
            <aside className="shrink-0 border-0 border-t border-solid border-line bg-white px-4 py-3 md:w-[24rem] md:overflow-y-auto md:border-t-0 md:border-l">
              <DongThoiGianChuyen lichSu={lichSu} traBoPhan={traBoPhan} danhBa={danhBa} />
            </aside>
          </div>
        </div>
      )}
    </LargeDialog>
  );
}

/**
 * The prototype's three titled cells under the status strip (`DocumentDetailDrawer.tsx:348-379`). The
 * third, "Nguồn vào sổ", is the disabled "?" cell: the record has no source field (ADR 0078 #6).
 */
function FactsRow({
  doc,
  bayGio,
  traBoPhan,
  danhBa,
}: {
  doc: documents_vanBanDenRa;
  bayGio: Date;
  traBoPhan: BangTraDanhMuc;
  danhBa: DanhBaTheoMa | null;
}) {
  // SUY RA LÚC VẼ, không lưu (luật 10, bất biến 3) — cùng hàm cột "Hạn xử lý" của bảng dùng: quá hạn
  // chỉ khi văn bản còn MỞ.
  const han = incomingDeadlineState(doc.due_at, doc.status, bayGio);
  return (
    <div className="grid shrink-0 grid-cols-1 gap-px border-0 border-b border-solid border-line bg-line sm:grid-cols-3">
      <Fact label="Hạn xử lý">
        <div className="text-[12.5px]">
          <DeadlineMark deadline={han} compact />
        </div>
        {/* The prototype's second line (`DocumentDetailDrawer.tsx:353-357`). */}
        <p className="m-0 mt-0.5 text-[11px] text-ink-muted">
          {doc.due_at === "" ? "Loại văn bản này chưa đặt hạn" : `Hạn cuối ${deadlineDay(doc.due_at)}`}
        </p>
      </Fact>
      <Fact label="Bộ phận đang giữ">
        {/* The prototype's own words for "nobody yet" (`DocumentDetailDrawer.tsx:360-369`). The shared
            labels keep theirs: the table draws "—" and the routing select needs a placeholder. */}
        <p className="m-0 text-[12.5px] text-navy">
          {(doc.holding_unit ?? "") === ""
            ? "Chưa chuyển cho ai"
            : nhanBoPhanDangGiu(traTen(traBoPhan, doc.holding_unit ?? ""))}
        </p>
        <p className="m-0 mt-0.5 text-[11px] text-ink-muted">
          {(doc.assignee ?? "") === "" ? "Chưa chỉ định người xử lý" : nhanCanBo(doc.assignee ?? "", danhBa)}
        </p>
      </Fact>
      <Fact label="Nguồn vào sổ" pending={pendingPart("Nguồn nhập văn bản đến")}>
        <p className="m-0 text-[12.5px] text-ink-muted">
          <span aria-hidden="true">—</span>
        </p>
      </Fact>
    </div>
  );
}

/** One titled cell. A `pending` cell carries its "?" beside the title — outside the title's words. */
function Fact({
  label,
  pending,
  children,
}: {
  label: string;
  pending?: { ten: string; viSao: string };
  children: ReactNode;
}) {
  return (
    <div className="min-w-0 bg-white px-4 py-2.5">
      <div className="mb-1 flex items-center gap-1.5">
        <p className="m-0 text-[10.5px] font-bold tracking-wide text-ink-muted uppercase">{label}</p>
        {pending !== undefined && <PendingMarker info={pending} />}
      </div>
      {children}
    </div>
  );
}

/** The prototype's figure (`DocumentDetailDrawer.tsx:525-540`): a 10.5px caps label, a navy value. */
function Figure({
  label,
  pending,
  children,
}: {
  label: string;
  pending?: { ten: string; viSao: string };
  children: ReactNode;
}) {
  return (
    <div className="min-w-0">
      <dt className="flex items-center gap-1.5 text-[10.5px] font-semibold tracking-wide text-ink-muted uppercase">
        {label}
        {pending !== undefined && <PendingMarker info={pending} />}
      </dt>
      <dd className={pending !== undefined ? "m-0 mt-0.5 text-[12.5px] text-ink-muted" : "m-0 mt-0.5 text-[12.5px] font-semibold break-words text-navy"}>
        {children}
      </dd>
    </div>
  );
}

/**
 * Dòng thời gian chuyển tiếp — CHỈ ĐỌC, GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ (cũ nhất trước).
 *
 * Không sắp lại ở client: thứ tự là thứ tự các lần chuyển thật, và máy chủ đã xếp theo `routed_at`.
 * The prototype's rail and lines (`DocumentDetailDrawer.tsx:472-503`): from → to, then time · who,
 * then the reason; "Phụ trách" stays as one more muted line. Người chuyển và cán bộ được giao hiện
 * `Họ tên (CB-…)`, or the bare code when the directory does not know it — see `nhanCanBo`.
 */
export function DongThoiGianChuyen({
  lichSu,
  traBoPhan,
  danhBa = null,
}: {
  lichSu: KetQua<documents_danhSachLichSuChuyenRa> | null;
  traBoPhan: BangTraDanhMuc;
  danhBa?: DanhBaTheoMa | null;
}) {
  return (
    <section aria-labelledby="tieu-de-dong-thoi-gian" className="flex min-w-0 flex-col">
      <h3 id="tieu-de-dong-thoi-gian" className="m-0 mb-2.5 text-[12.5px] font-bold text-navy">
        {TIEU_DE_DONG_THOI_GIAN}
      </h3>
      {lichSu === null && (
        <p role="status" className="m-0 text-[12.5px] text-ink-muted">
          {DANG_TAI_LICH_SU}
        </p>
      )}
      {lichSu !== null && !lichSu.ok && (
        <p className="thong-bao-loi m-0" role="alert">
          {lichSu.thongBao}
        </p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length === 0 && (
        <p className="m-0 text-[12.5px] text-ink-muted">{LICH_SU_RONG}</p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length > 0 && (
        // A rail on the left, one dot per routing. Read-only — no control on any line (rule 7, #5).
        <ol className="relative m-0 flex list-none flex-col gap-4 border-0 border-l border-solid border-line p-0 pl-5">
          {lichSu.duLieu.items.map((d) => (
            <DongLichSu key={d.id} d={d} traBoPhan={traBoPhan} danhBa={danhBa} />
          ))}
        </ol>
      )}
    </section>
  );
}

function DongLichSu({
  d,
  traBoPhan,
  danhBa,
}: {
  d: documents_lichSuChuyenRa;
  traBoPhan: BangTraDanhMuc;
  danhBa: DanhBaTheoMa | null;
}) {
  return (
    <li className="relative min-w-0 [&>p]:m-0">
      <span
        aria-hidden="true"
        className="absolute top-1 -left-[26px] size-2.5 rounded-full border-2 border-solid border-brand bg-white"
      />
      <p className="flex flex-wrap items-center gap-1.5 text-[12.5px] font-semibold text-navy">
        <span>{nhanTuBoPhan(traTen(traBoPhan, d.from_unit ?? ""))}</span>
        <ArrowRight aria-hidden="true" focusable="false" className="size-3.5 text-ink-muted" />
        <span className="an-thi-giac">đến</span>
        <span>{nhanBoPhanDangGiu(traTen(traBoPhan, d.to_unit))}</span>
      </p>
      <p className="text-[11.5px] text-ink-muted">
        <time dateTime={d.routed_at} className="tabular-nums">
          {nhanThoiDiem(d.routed_at)}
        </time>{" "}
        · <strong className="font-medium text-ink">{staffNameWithCode(d.routed_by, danhBa)}</strong>
      </p>
      <p className="mt-1 text-[12px] break-words text-ink">{d.reason}</p>
    </li>
  );
}

/** The prototype's routing selects (`common/HandoverFields.tsx`): 36px, `rounded-md`, white, 12.5px. */
const ROUTING_CONTROL =
  "mt-1 box-border h-9 min-h-0 w-full min-w-0 rounded-md border border-solid border-line bg-white pl-2.5 [font-family:inherit] text-[12.5px] text-navy outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50";
/** The prototype's `Label` at 11.5px. */
const ROUTING_LABEL = "block text-[11.5px] leading-none font-medium text-ink";

/**
 * Khối "Chuyển cho bộ phận khác" — the prototype's routing box (`DocumentDetailDrawer.tsx:427-464`:
 * unit, person, reason, `Chuyển và ghi vết`, on white inside the grey column), giữ nguyên phép kiểm
 * (`guiChuyenVanBan` ở `thao-tac-van-ban.ts`).
 *
 * THE TITLE IS NOT THE PROTOTYPE'S "…, không đổi trạng thái": on THIS server the first routing of a
 * new document moves it from `moi-vao-so` to `da-phan-cong` (`domain/van_ban.go:375`), so that phrase
 * would be false on the very routing staff do most.
 *
 * THE PERSON IS PICKED BY NAME from the staff directory the register already read once — the value
 * sent is still the staff CODE (`CB-…`), the only thing the contract's `assignee` accepts. Directory
 * not loaded (still reading, or refused): the code is typed instead, so routing never waits on it.
 *
 * `reason` LÀ CHỮ TỰ DO CÓ THỂ NHẮC TÊN MỘT CÔNG DÂN: nó chỉ sống trong trạng thái React và trong
 * thân `POST`, không vào log, URL, `localStorage` hay một thuộc tính nào (luật 3).
 */
export function KhoiChuyenXuLy({
  ban,
  datBan,
  traBoPhan,
  danhBa = null,
  loi,
  dangGui,
  onGui,
}: {
  ban: BanChuyen;
  datBan: (b: BanChuyen) => void;
  traBoPhan: BangTraDanhMuc;
  danhBa?: DanhBaTheoMa | null;
  loi: string;
  dangGui: boolean;
  onGui: () => void;
}) {
  return (
    <section
      aria-labelledby="tieu-de-khoi-chuyen"
      className="mt-5 flex min-w-0 flex-col border-0 border-t border-solid border-line pt-4"
    >
      <h3 id="tieu-de-khoi-chuyen" className="m-0 mb-2.5 text-[12.5px] font-bold text-navy">
        {TIEU_DE_KHOI_CHUYEN}
      </h3>
      <form
        className="flex min-w-0 flex-col gap-3 rounded-[10px] border border-solid border-line bg-white p-3"
        aria-labelledby="tieu-de-khoi-chuyen"
        onSubmit={(e) => {
          e.preventDefault();
          onGui();
        }}
      >
        <div className="grid min-w-0 gap-3 sm:grid-cols-2">
          <div className="flex min-w-0 flex-col items-stretch gap-0">
            <label htmlFor="o-den-bo-phan" className={ROUTING_LABEL}>
              {O_DEN_BO_PHAN}
            </label>
            <select
              id="o-den-bo-phan"
              name="denBoPhan"
              className={cn(ROUTING_CONTROL, "pr-9")}
              value={ban.denBoPhan}
              onChange={(e) => datBan({ ...ban, denBoPhan: e.target.value })}
            >
              <option value="">— Chọn bộ phận —</option>
              {traBoPhan.pha === "xong" &&
                [...traBoPhan.ten].map(([id, ten]) => (
                  <option key={id} value={id}>
                    {ten}
                  </option>
                ))}
            </select>
          </div>

          <div className="flex min-w-0 flex-col items-stretch gap-0">
            <label htmlFor="o-can-bo-xu-ly" className={ROUTING_LABEL}>
              {O_CAN_BO_XU_LY}
            </label>
            {danhBa !== null ? (
              <select
                id="o-can-bo-xu-ly"
                name="canBoXuLy"
                className={cn(ROUTING_CONTROL, "pr-9")}
                value={ban.canBoXuLy}
                onChange={(e) => datBan({ ...ban, canBoXuLy: e.target.value })}
              >
                <option value="">{ROUTING_PERSON_PLACEHOLDER}</option>
                {[...danhBa.values()].map((c) => (
                  <option key={c.code} value={c.code}>
                    {c.full_name}
                    {c.position ? ` — ${c.position}` : ""}
                  </option>
                ))}
                {/* A code typed or kept that the directory does not list still shows as itself. */}
                {ban.canBoXuLy !== "" && !danhBa.has(ban.canBoXuLy) && (
                  <option value={ban.canBoXuLy}>{ban.canBoXuLy}</option>
                )}
              </select>
            ) : (
              <input
                id="o-can-bo-xu-ly"
                name="canBoXuLy"
                className={cn(ROUTING_CONTROL, "pr-2.5")}
                value={ban.canBoXuLy}
                autoComplete="off"
                onChange={(e) => datBan({ ...ban, canBoXuLy: e.target.value })}
                placeholder="Mã cán bộ — để trống nếu để bộ phận tự phân công"
              />
            )}
          </div>
        </div>

        <div className="flex min-w-0 flex-col">
          <label htmlFor="o-ly-do-chuyen" className={ROUTING_LABEL}>
            {O_LY_DO_CHUYEN}
          </label>
          <input
            id="o-ly-do-chuyen"
            name="lyDoChuyen"
            required
            autoComplete="off"
            className={cn(ROUTING_CONTROL, "rounded-lg border-input pr-2.5")}
            value={ban.lyDo}
            placeholder={ROUTING_REASON_PLACEHOLDER}
            onChange={(e) => datBan({ ...ban, lyDo: e.target.value })}
            aria-invalid={loi === LOI_THIEU_LY_DO_CHUYEN}
          />
        </div>

        {/* CÂU TỪ CHỐI RA NGUYÊN VĂN, dù từ phép kiểm ở client hay từ máy chủ. */}
        {loi !== "" && (
          <p className="thong-bao-loi m-0" role="alert">
            {loi}
          </p>
        )}

        <div>
          <Button
            type="submit"
            variant="primary"
            icon={<Glyph icon={Send} />}
            disabled={dangGui}
            aria-busy={dangGui || undefined}
          >
            <BusyLabel busy={dangGui} label={NUT_XAC_NHAN_CHUYEN} busyText="Đang chuyển…" />
          </Button>
        </div>
      </form>
    </section>
  );
}
