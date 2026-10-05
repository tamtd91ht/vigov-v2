"use client";

import { CircleCheck, FileText, LockKeyhole, Pencil, Send, Trash2, X } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { Notice } from "@/components/ui/notice";
import { traTen, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import { nhanThoiDiem, staffNameWithCode, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_danhSachLichSuChuyenRa,
  documents_lichSuChuyenRa,
  documents_vanBanDenRa,
} from "@/lib/api/schema.gen";

import {
  DAN_CHUYEN_XU_LY,
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
  O_DO_KHAN,
  O_LOAI_VAN_BAN,
  O_LY_DO_CHUYEN,
  REFERENCE_FIELD_LABEL,
  ROUTING_PERSON_PLACEHOLDER,
  ROUTING_REASON_PLACEHOLDER,
  SUMMARY_FIELD_LABEL,
  TIEU_DE_DONG_THOI_GIAN,
  TIEU_DE_KHOI_CHUYEN,
  nhanBoPhanDangGiu,
  nhanCanBo,
  nhanDoKhan,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanSoVaoSo,
  nhanTieuDeVanBanDen,
  nhanTrangThai,
  nhanTuBoPhan,
  trangThaiHanVanBan,
  type BanChuyen,
} from "./nhan-van-ban";
import { RaiseTaskButton, StatusChangeRow } from "./document-pending";
import { DeadlineMark, DocumentStatusBadge, Glyph, UrgencyBadge } from "./document-ui";

/** Heading id of the detail — the dialog's accessible name. */
export const INCOMING_DETAIL_TITLE_ID = "tieu-de-ngan-van-ban-den";

/**
 * Ngăn chi tiết MỘT văn bản đến — the PROTOTYPE's right-hand drawer (`DocumentDetailDrawer.tsx`,
 * ADR 0068 lần 5), drawn in the shared `LargeDialog` (pinned right, full height, Esc and ✕ close it):
 *
 *   header      number · arrival date (the dialog's NAME), the summary large, the issuing body;
 *               Sửa · Gỡ khỏi sổ (`document.create`) · ✕ on the right
 *   status      the status strip — four disabled "chuyển sang" chips with one "?", then the
 *               current status, type and urgency
 *   facts       three cells: Hạn xử lý · Bộ phận đang giữ · Người vào sổ
 *   body        left: trích yếu, six figures, `Chuyển thành nhiệm vụ` ("?"), the routing block;
 *               right (from 768px; under the left column below): the routing timeline
 *
 * THE DIALOG'S NAME IS THE NUMBER AND THE DATE, NEVER THE SUMMARY: the prototype's title is the
 * summary, but a name goes into the accessibility tree, and a summary is free text that may name a
 * citizen (rule 3, forbidden #4). The summary is still the large line under it.
 *
 * BỐN NÚT ĐỔI TRẠNG THÁI LÀ CHỖ GIỮ VÔ HIỆU mang dấu "?" (`StatusChangeRow`, ADR 0068 §14): bộ trạng
 * thái riêng của văn bản đến đã chốt (30/09/2026) nhưng máy chủ chưa có tuyến đổi trạng thái.
 *
 * KHÔNG CÓ NÚT NÀO SỬA HAY XOÁ MỘT DÒNG LỊCH SỬ: bảng lịch sử chỉ-thêm ở tầng CSDL (trigger
 * `lich_su_chuyen_chi_them`) và hợp đồng không có tuyến nào làm việc ấy (luật 7, cấm #5).
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
    <LargeDialog titleId={INCOMING_DETAIL_TITLE_ID} onDismiss={onDong}>
      <header className="flex shrink-0 items-start gap-3 border-b border-solid border-line bg-surface px-5 py-4">
        <div className="min-w-0 flex-1">
          <h2
            id={INCOMING_DETAIL_TITLE_ID}
            tabIndex={-1}
            className="m-0 text-xs leading-snug font-semibold text-ink-500 tabular-nums"
          >
            {tieuDe}
          </h2>
          {doc !== null && (
            <>
              <p className="m-0 mt-0.5 text-base leading-snug font-bold break-words text-ink-900">{doc.summary}</p>
              <p className="m-0 mt-1 text-[13px] text-ink-500">{doc.issuing_body}</p>
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
        <IconButton type="button" label={NUT_DONG_CHI_TIET} onClick={onDong}>
          <X aria-hidden="true" />
        </IconButton>
      </header>

      {vb === null && (
        <div className="p-5">
          <p role="status" className="an-thi-giac">
            {DANG_TAI_CHI_TIET}
          </p>
          {/* First-load placeholder (spec §8b). */}
          <div aria-hidden="true" className="flex flex-col gap-3">
            <span className="h-4 w-3/4 rounded bg-line motion-safe:animate-pulse" />
            <span className="h-[26px] w-64 max-w-full rounded-full bg-line motion-safe:animate-pulse" />
            <span className="h-3 w-1/2 rounded bg-line motion-safe:animate-pulse" />
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
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
          {/* The prototype's status strip. Disabled placeholder; the current status follows. */}
          <div className="flex shrink-0 flex-col gap-2.5 border-b border-solid border-line bg-surface px-5 py-3">
            <StatusChangeRow />
            <p className="m-0 flex flex-wrap items-center gap-2" aria-label="Trạng thái và thuộc tính">
              <DocumentStatusBadge status={doc.status}>{nhanTrangThai(doc.status)}</DocumentStatusBadge>{" "}
              <Badge tone="neutral" icon={FileText}>
                {nhanLoaiVanBan(traTen(traLoai, doc.document_type))}
              </Badge>{" "}
              <UrgencyBadge urgency={doc.urgency ?? ""}>{nhanDoKhan(doc.urgency ?? "")}</UrgencyBadge>
            </p>
          </div>

          <FactsRow doc={doc} bayGio={bayGio} traBoPhan={traBoPhan} danhBa={danhBa} />

          <div className="flex min-w-0 flex-1 flex-col md:flex-row">
            <div className="flex min-w-0 flex-1 flex-col gap-4 bg-surface-muted px-5 py-4 [&>*]:my-0">
              <section aria-labelledby="tieu-de-trich-yeu-den" className="flex flex-col gap-2">
                <h3 id="tieu-de-trich-yeu-den" className="m-0 text-[13px] font-bold text-ink-900">
                  {SUMMARY_FIELD_LABEL}
                </h3>
                <p className="m-0 rounded-[10px] border border-solid border-line bg-surface px-3 py-2.5 text-[15px] leading-relaxed whitespace-pre-line text-ink-900">
                  {doc.summary}
                </p>
              </section>

              <dl className="m-0 grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
                <Figure label={REFERENCE_FIELD_LABEL}>
                  {doc.reference_no === undefined || doc.reference_no === "" ? "Không ghi" : doc.reference_no}
                </Figure>
                <Figure label={ISSUED_ON_FIELD_LABEL}>{nhanNgayCoThe(doc.document_date ?? "")}</Figure>
                <Figure label={O_LOAI_VAN_BAN}>{nhanLoaiVanBan(traTen(traLoai, doc.document_type))}</Figure>
                <Figure label={O_CO_QUAN_BAN_HANH}>{doc.issuing_body}</Figure>
                <Figure label="Số đến">{nhanSoVaoSo(doc.number, doc.year)}</Figure>
                <Figure label={O_DO_KHAN}>{nhanDoKhan(doc.urgency ?? "")}</Figure>
              </dl>

              <div>
                <RaiseTaskButton />
              </div>

              {cauDaXong !== "" && (
                <p role="status" className="flex items-center gap-2 text-sm font-medium text-success-600">
                  <Glyph icon={CircleCheck} className="size-[18px] shrink-0" />
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
            <aside className="shrink-0 border-t border-solid border-line bg-surface px-4 py-3 md:w-[24rem] md:border-t-0 md:border-l">
              <DongThoiGianChuyen lichSu={lichSu} traBoPhan={traBoPhan} danhBa={danhBa} />
            </aside>
          </div>
        </div>
      )}
    </LargeDialog>
  );
}

/** The prototype's three titled cells under the status strip. */
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
  // SUY RA LÚC VẼ, không lưu (luật 10, bất biến 3) — cùng hàm cột "Hạn xử lý" của bảng dùng.
  const han = trangThaiHanVanBan(doc.due_at, bayGio);
  return (
    <div className="grid shrink-0 grid-cols-1 gap-px border-b border-solid border-line bg-line sm:grid-cols-3">
      <Fact label="Hạn xử lý">
        <DeadlineMark deadline={han} />
        <p className="m-0 mt-0.5 text-xs text-ink-500">Hệ thống ấn định lúc vào sổ</p>
      </Fact>
      <Fact label="Bộ phận đang giữ">
        <p className="m-0 text-sm text-ink-900">{nhanBoPhanDangGiu(traTen(traBoPhan, doc.holding_unit ?? ""))}</p>
        <p className="m-0 mt-0.5 text-xs text-ink-500">{nhanCanBo(doc.assignee ?? "", danhBa)}</p>
      </Fact>
      <Fact label="Người vào sổ">
        <p className="m-0 text-sm text-ink-900">{staffNameWithCode(doc.created_by, danhBa)}</p>
        <p className="m-0 mt-0.5 text-xs text-ink-500 tabular-nums">{nhanThoiDiem(doc.created_at)}</p>
      </Fact>
    </div>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0 bg-surface px-4 py-2.5">
      <p className="m-0 mb-1 text-[11px] font-bold tracking-wide text-ink-500 uppercase">{label}</p>
      {children}
    </div>
  );
}

function Figure({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-[11px] font-semibold tracking-wide text-ink-500 uppercase">{label}</dt>
      <dd className="m-0 mt-0.5 text-sm font-semibold break-words text-ink-900">{children}</dd>
    </div>
  );
}

/**
 * Dòng thời gian chuyển tiếp — CHỈ ĐỌC, GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ (cũ nhất trước).
 *
 * Không sắp lại ở client: thứ tự là thứ tự các lần chuyển thật, và máy chủ đã xếp theo `routed_at`.
 * The prototype's line order: from → to, then time · who, then the reason; our status and "Phụ
 * trách" stay as two more lines. Người chuyển và cán bộ được giao hiện `Họ tên (CB-…)`, or the bare
 * code when the directory does not know it — see `nhanCanBo`.
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
    <section aria-labelledby="tieu-de-dong-thoi-gian" className="flex min-w-0 flex-col gap-2.5">
      <h3 id="tieu-de-dong-thoi-gian" className="m-0 text-[13px] font-bold text-ink-900">
        {TIEU_DE_DONG_THOI_GIAN}
      </h3>
      {lichSu === null && (
        <p role="status" className="m-0 text-[13px] text-ink-500">
          {DANG_TAI_LICH_SU}
        </p>
      )}
      {lichSu !== null && !lichSu.ok && (
        <p className="thong-bao-loi m-0" role="alert">
          {lichSu.thongBao}
        </p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length === 0 && (
        <p className="m-0 text-[13px] text-ink-500">{LICH_SU_RONG}</p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length > 0 && (
        // A rail on the left, one dot per routing. Read-only — no control on any line (rule 7, #5).
        <ol className="relative m-0 flex list-none flex-col gap-4 border-l border-solid border-line p-0 pl-5">
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
    <li className="relative flex min-w-0 flex-col gap-1 text-[13px] text-ink-700 [&>p]:m-0">
      <span
        aria-hidden="true"
        className="absolute top-1 -left-[26px] size-2.5 rounded-full border-2 border-solid border-brand-500 bg-surface"
      />
      <p className="font-semibold text-ink-900">
        {nhanTuBoPhan(traTen(traBoPhan, d.from_unit ?? ""))} → {nhanBoPhanDangGiu(traTen(traBoPhan, d.to_unit))}
      </p>
      <p className="text-xs text-ink-500">
        <time dateTime={d.routed_at} className="tabular-nums">
          {nhanThoiDiem(d.routed_at)}
        </time>{" "}
        · <strong className="font-medium text-ink-700">{staffNameWithCode(d.routed_by, danhBa)}</strong>
      </p>
      <p>
        <DocumentStatusBadge status={d.status}>{nhanTrangThai(d.status)}</DocumentStatusBadge>
      </p>
      <p className="text-xs text-ink-500">Phụ trách: {nhanCanBo(d.assignee ?? "", danhBa)}</p>
      <p className="break-words text-ink-900">{d.reason}</p>
    </li>
  );
}

/**
 * Khối "Chuyển cho bộ phận khác" — the prototype's routing box (unit, person, reason, `Chuyển và ghi
 * vết`), giữ nguyên phép kiểm (`guiChuyenVanBan` ở `thao-tac-van-ban.ts`).
 *
 * THE PERSON IS PICKED BY NAME from the staff directory the register already read once — the value
 * sent is still the staff CODE (`CB-…`), the only thing the contract's `assignee` accepts. Directory
 * not loaded (still reading, or refused): the code is typed instead, as before, so routing never
 * waits on the directory.
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
    <section aria-labelledby="tieu-de-khoi-chuyen" className="flex min-w-0 flex-col gap-2.5 border-t border-solid border-line pt-4">
      <h3 id="tieu-de-khoi-chuyen" className="m-0 text-[13px] font-bold text-ink-900">
        {TIEU_DE_KHOI_CHUYEN}
      </h3>
      <form
        className="flex min-w-0 flex-col gap-3 rounded-[10px] border border-solid border-line bg-surface p-3"
        aria-labelledby="tieu-de-khoi-chuyen"
        onSubmit={(e) => {
          e.preventDefault();
          onGui();
        }}
      >
        <Notice tone="legal" icon={LockKeyhole}>
          {DAN_CHUYEN_XU_LY}
        </Notice>

        <div className="grid min-w-0 gap-3 sm:grid-cols-2">
          <Field label={O_DEN_BO_PHAN} htmlFor="o-den-bo-phan" kind="select" grow="auto">
            <select
              id="o-den-bo-phan"
              name="denBoPhan"
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
          </Field>

          {danhBa !== null ? (
            <Field label={O_CAN_BO_XU_LY} htmlFor="o-can-bo-xu-ly" kind="select" grow="auto">
              <select
                id="o-can-bo-xu-ly"
                name="canBoXuLy"
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
            </Field>
          ) : (
            <Field label={O_CAN_BO_XU_LY} htmlFor="o-can-bo-xu-ly" grow="auto">
              <input
                id="o-can-bo-xu-ly"
                name="canBoXuLy"
                value={ban.canBoXuLy}
                autoComplete="off"
                onChange={(e) => datBan({ ...ban, canBoXuLy: e.target.value })}
                placeholder="Mã cán bộ — để trống nếu để bộ phận tự phân công"
              />
            </Field>
          )}
        </div>

        <Field label={O_LY_DO_CHUYEN} htmlFor="o-ly-do-chuyen" grow="auto">
          <input
            id="o-ly-do-chuyen"
            name="lyDoChuyen"
            required
            autoComplete="off"
            value={ban.lyDo}
            placeholder={ROUTING_REASON_PLACEHOLDER}
            onChange={(e) => datBan({ ...ban, lyDo: e.target.value })}
            aria-invalid={loi === LOI_THIEU_LY_DO_CHUYEN}
          />
        </Field>

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
