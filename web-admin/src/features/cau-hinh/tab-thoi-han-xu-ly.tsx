"use client";

import { AlertTriangle, Pencil, Plus, Trash2 } from "lucide-react";
import {
  Fragment,
  useCallback,
  useEffect,
  useState,
  type KeyboardEvent,
} from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { PendingButton, PendingFeature } from "@/components/ui/pending-feature";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhSachSLARa,
  identity_dongSLARa,
} from "@/lib/api/schema.gen";
import {
  UNASSIGNED_HOLD_KEY,
  gieoThoiHanMacDinh,
  layThoiHanXuLy,
  readCitizenReportFieldLabels,
  suaThoiHanXuLy,
} from "@/lib/api/thoi-han-xu-ly";
import { cn } from "@/lib/cn";

import { khoiCanhBao, tinhTrangBang } from "./chua-cau-hinh";
import {
  ConfigLoading,
  ConfigTable,
  RowActions,
  SMALL_BUTTON_CLASS,
} from "./config-ui";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import {
  DA_LUU_THOI_HAN,
  NUT_HUY,
  NUT_LUU,
  SLA_BANNER_APPLIES,
  SLA_BANNER_DUE_SOON_COLUMN,
  SLA_BANNER_DUE_SOON_LEAD,
  SLA_BANNER_DUE_SOON_REST,
  SLA_BANNER_HOLIDAYS,
  SLA_BANNER_LEAD,
  SLA_BANNER_UNIT,
  UNASSIGNED_HOLD_OFF,
  afterHoursCell,
  cauGieoThoiHan,
  hoursCell,
  nhanLinhVuc,
  nhanLoaiViec,
} from "./nhan-thoi-han";
import { quyetDinhGhiThoiHan, slaFieldLabelReadDecision } from "./quyen-tab";
// vi-name-ok: imports the existing exports of sua-thoi-han.ts unchanged (rule 12 invariant 3)
import {
  COT_GIO,
  NHAN_COT,
  banTuDong,
  soanSua,
  type BanNhapGio,
  type KhoaGio,
} from "./sua-thoi-han";
import { KhoiChuaKhai } from "./working-calendar-tab";

/**
 * Tab "Thời hạn xử lý" — spec `08-thoi-han-xu-ly.md` (ADR 0079), prototype `SlaTable.tsx`: the banner,
 * the add button, the table edited IN PLACE (`SlaTable.tsx:200-217`), nothing below it but one note.
 *
 * THE THREE CALENDAR TABLES LEFT THIS TAB (owner, 08/10/2026, ADR 0079 D1/D2) for "Lịch làm việc"
 * (`working-calendar-tab.tsx`), and this tab is gated on `admin.sla` by the frame: its one read,
 * `GET /api/v1/sla`, declares that key (`sla.go` says why), so without it there is nothing to show.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VIỆC SỐ MỘT CỦA MÀN NÀY KHÔNG PHẢI MỘT BẢNG ĐẸP. Nó là làm cho một xã mới biết mình đang thiếu
 * gì và bấm được nút gieo.
 *
 * Chuỗi phía sau `POST /api/v1/incoming-documents` đi qua `identity.ResolveDeadlines`, và hàm ấy
 * từ chối khi bảng thời hạn rỗng, từ chối khi lịch làm việc rỗng. Một xã vừa nhận hệ thống vì thế
 * KHÔNG vào sổ được văn bản đến và KHÔNG nhận được phản ánh — và lỗi ấy hiện ra ở một màn hình
 * khác hẳn màn hình sửa được nó. Khối `KhoiChuaKhai` là thứ thay cho một bước hướng dẫn ban đầu
 * không tồn tại; ở tab này nó chỉ nói về bảng thời hạn, ở tab Lịch làm việc thì về lịch tuần.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * "+ Thêm thời hạn cho một lĩnh vực" AND "Xoá thời hạn riêng" ARE DISABLED "?" PLACEHOLDERS (ADR 0068
 * §14, ADR 0079 #5): no route adds a field row — a field code from the client must be matched against
 * the tier-1 codes in `platform`, and that read has no ADR (ADR 0026 stop condition #2) — and no route
 * deletes one. Each stands where the prototype draws it.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP: các tuyến ghi khai `RequirePermission("admin.sla")` và
 * kiểm trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * KHÔNG MỘT PHÉP CỘNG GIỜ LÀM VIỆC NÀO Ở ĐÂY. `identity` sở hữu bảng và sở hữu phép cộng (ADR 0007).
 */

/** Looked up by name so a renamed entry fails a test rather than drawing `undefined`. */
const ADD_FIELD_SLA = PHAN_CHUA_DUNG.find(
  (p) => p.ten === "Thêm thời hạn cho một lĩnh vực",
)!;
const DELETE_FIELD_SLA = PHAN_CHUA_DUNG.find(
  (p) => p.ten === "Xoá thời hạn riêng",
)!;

/** Spec 08's pencil title — also the start of each pencil's accessible name. */
const EDIT_TITLE = "Sửa thời hạn";

/** Columns that count from a moment (a missed deadline, a hold) and so read "sau {n} giờ". */
const AFTER_COLUMNS: ReadonlySet<KhoaGio> = new Set<KhoaGio>([
  "escalate_leader_hours",
  "escalate_president_hours",
  UNASSIGNED_HOLD_KEY,
]);

const EMPTY_DRAFT: BanNhapGio = {
  acknowledge_hours: "",
  resolve_hours: "",
  due_soon_hours: "",
  escalate_leader_hours: "",
  escalate_president_hours: "",
  unassigned_hold_hours: "",
};

/** Marks a number box the browser could not parse; `soanSua` refuses it like any other non-number. */
const BAD_NUMBER = "?";

/**
 * What a `type="number"` box holds. A box the browser cannot parse ("-", "1e") reports `value === ""`,
 * and an empty sixth box MEANS "không báo" (`null`) — so a typo there would silently turn a report off
 * with "Đã lưu" on screen. `badInput` keeps it a refusal instead.
 */
function readNumberBox(el: HTMLInputElement): string {
  return el.validity.badInput ? BAD_NUMBER : el.value;
}

/** Hai thao tác mà một dòng hoặc khối cảnh báo có thể yêu cầu. */
export type ThaoTacThoiHan = {
  readonly gieoThoiHan: () => void;
  readonly suaThoiHan: (d: identity_dongSLARa) => void;
};

/** Lượt đọc của tab. `null` là CHƯA đọc xong — khác hẳn "đọc xong và rỗng". */
export type DuLieuTab = {
  readonly thoiHan: KetQua<identity_danhSachSLARa> | null;
};

/**
 * The row being edited in place. Two error slots, never merged: the local one says "a box is wrong",
 * the server one says "the request was refused" — one overwriting the other hides half of what to fix.
 */
export type InlineEdit = {
  readonly rowId: string;
  readonly draft: BanNhapGio;
  readonly localError: string;
  readonly serverError: string;
  readonly busy: boolean;
  readonly setDraft: (d: BanNhapGio) => void;
  readonly onSave: () => void;
  readonly onCancel: () => void;
};

/* ---- vỏ đọc dữ liệu ------------------------------------------------------------------------ */

export function TabThoiHanXuLy() {
  /** Tăng sau mỗi lần ghi thành công để ĐỌC LẠI từ máy chủ, không vá mảng tại chỗ. */
  const [lanDoc, datLanDoc] = useState(0);
  const [thoiHan, datThoiHan] = useState<KetQua<identity_danhSachSLARa> | null>(
    null,
  );

  const [editing, setEditing] = useState<identity_dongSLARa | null>(null);
  const [draft, setDraft] = useState<BanNhapGio>(EMPTY_DRAFT);
  const [localError, setLocalError] = useState("");
  const [serverError, setServerError] = useState("");
  const [seedError, setSeedError] = useState("");
  const [busy, setBusy] = useState(false);

  const phien = usePhien();
  /**
   * BA TRẠNG THÁI, KHÔNG HAI: chưa đọc xong phiên thì chưa vẽ nút ghi nào. "Chưa biết" không được
   * hành xử như "có quyền", và cũng không được hành xử như "thiếu quyền".
   */
  const coQuyenGhi = phien !== null && quyetDinhGhiThoiHan(phien).hien;

  /**
   * Field code → the commune's label, for the "Lĩnh vực" column only (SLA-03). Read ONCE per screen,
   * and only by an account holding `admin.lookup` — the key that route declares; this tab is
   * `admin.sla`, so many accounts here lack it, and for them the column keeps the raw code rather
   * than sending a request bound to answer 403. Not re-read after a write: no SLA write changes it.
   */
  const canReadFieldLabels =
    phien !== null && slaFieldLabelReadDecision(phien).hien;
  const [fieldLabels, setFieldLabels] = useState<ReadonlyMap<string, string>>(
    () => new Map(),
  );
  useEffect(() => {
    if (!canReadFieldLabels) return;
    let bo = false;
    readCitizenReportFieldLabels().then((m) => {
      if (!bo) setFieldLabels(m);
    });
    return () => {
      bo = true;
    };
  }, [canReadFieldLabels]);

  useEffect(() => {
    let bo = false;
    layThoiHanXuLy().then((kq) => {
      if (!bo) datThoiHan(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  const startEdit = useCallback((d: identity_dongSLARa) => {
    setEditing(d);
    setDraft(banTuDong(d));
    setLocalError("");
    setServerError("");
  }, []);

  const cancelEdit = useCallback(() => {
    setEditing(null);
    setDraft(EMPTY_DRAFT);
    setLocalError("");
    setServerError("");
  }, []);

  /** Success → toast + READ AGAIN; refusal → the server's sentence AS WRITTEN, in place. */
  const seed = useCallback(() => {
    if (busy) return;
    setSeedError("");
    setBusy(true);
    void gieoThoiHanMacDinh().then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setSeedError(kq.thongBao);
        return;
      }
      toast.success(cauGieoThoiHan(kq.duLieu.seeded, kq.duLieu.kept));
      datLanDoc((n) => n + 1);
    });
  }, [busy]);

  const save = useCallback(() => {
    if (editing === null || busy) return;
    setServerError("");
    const soan = soanSua(editing, draft);
    if (!soan.ok) {
      setLocalError(soan.loi);
      return;
    }
    setLocalError("");
    setBusy(true);
    void suaThoiHanXuLy(editing.id, soan.than).then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setServerError(kq.thongBao);
        return;
      }
      setEditing(null);
      setDraft(EMPTY_DRAFT);
      toast.success(DA_LUU_THOI_HAN);
      datLanDoc((n) => n + 1);
    });
  }, [busy, draft, editing]);

  return (
    <ManThoiHanXuLy
      du={{ thoiHan }}
      coQuyenGhi={coQuyenGhi}
      fieldLabels={fieldLabels}
      thaoTac={{ gieoThoiHan: seed, suaThoiHan: startEdit }}
      loiMayChuNgoaiForm={seedError}
      dangGui={busy}
      edit={
        editing === null
          ? null
          : {
              rowId: editing.id,
              draft,
              localError,
              serverError,
              busy,
              setDraft,
              onSave: save,
              onCancel: cancelEdit,
            }
      }
    />
  );
}

/* ---- phần trình bày ------------------------------------------------------------------------ */

/**
 * Toàn bộ phần nhìn thấy được của tab, THUẦN TRÌNH BÀY.
 *
 * XUẤT RA để bài kiểm kết xuất được bằng `react-dom/server` mà không cần trình duyệt giả lập: mọi
 * QUYẾT ĐỊNH (câu nào hiện, nút nào có) phải được canh ở chỗ nó ra tới trang, không chỉ trong module
 * thuần.
 */
export function ManThoiHanXuLy({
  du,
  coQuyenGhi,
  fieldLabels = new Map(),
  thaoTac,
  loiMayChuNgoaiForm,
  dangGui,
  edit = null,
}: {
  du: DuLieuTab;
  coQuyenGhi: boolean;
  /** Field code → label for the "Lĩnh vực" column; empty map = show raw codes. Omitted = empty. */
  fieldLabels?: ReadonlyMap<string, string>;
  thaoTac: ThaoTacThoiHan;
  /** Lỗi của nút gieo — the one action that opens no row. */
  loiMayChuNgoaiForm: string;
  dangGui: boolean;
  /** The row being edited in place, or `null`. */
  edit?: InlineEdit | null;
}) {
  // Only the SLA table here; the weekly-hours half of the block lives on "Lịch làm việc" (ADR 0079 D2).
  const khoi = khoiCanhBao(tinhTrangBang(du.thoiHan), "chuaBiet");
  const kq = du.thoiHan;
  const rows = kq !== null && kq.ok ? kq.duLieu.items : [];

  return (
    <section className="space-y-3" aria-labelledby="tieu-de-thoi-han">
      <h2 id="tieu-de-thoi-han" className="an-thi-giac">
        Thời hạn xử lý
      </h2>
      <KhoiChuaKhai
        khoi={khoi}
        coQuyenGhi={coQuyenGhi}
        dangGui={dangGui}
        onSeedSla={thaoTac.gieoThoiHan}
      />

      {/* Spec 08 "Hộp giải thích" (`SlaTable.tsx:68-78`). `border-solid`: preflight is off here, so a
          bare `border` draws nothing. */}
      <div className="border-brand/25 bg-brand/8 text-navy flex gap-2.5 rounded-[10px] border border-solid p-3 text-[12.5px]">
        <AlertTriangle
          aria-hidden="true"
          focusable="false"
          className="text-brand mt-0.5 size-4 shrink-0"
        />
        <span>
          {SLA_BANNER_LEAD}
          <b>{SLA_BANNER_UNIT}</b>
          {SLA_BANNER_HOLIDAYS}
          {SLA_BANNER_APPLIES}
          <br />
          {SLA_BANNER_DUE_SOON_LEAD}
          <b>{SLA_BANNER_DUE_SOON_COLUMN}</b>
          {SLA_BANNER_DUE_SOON_REST}
        </span>
      </div>

      {coQuyenGhi && (
        // NO RE-SEED BUTTON once the table has rows (owner, 08/10/2026: "Bỏ hết, đúng prototype"). The
        // only seed button is the empty table's, inside `KhoiChuaKhai`.
        <div className="flex justify-end">
          {/* `SlaTable.tsx:80-87`, disabled "?" (ADR 0068 §14). The class reaches the inner button. */}
          <PendingButton
            info={ADD_FIELD_SLA}
            variant="primary"
            size="sm"
            className="[&>button]:min-h-0"
            icon={
              <Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />
            }
          />
        </div>
      )}

      {loiMayChuNgoaiForm !== "" && (
        <p role="alert" className="text-danger m-0 text-[12.5px]">
          {loiMayChuNgoaiForm}
        </p>
      )}

      {kq === null && <ConfigLoading label="Đang tải bảng thời hạn xử lý…" />}
      {kq !== null && !kq.ok && (
        // NGUYÊN VĂN câu máy chủ viết.
        <ErrorState
          role="alert"
          title="Chưa tải được bảng này"
          message={kq.thongBao}
          className="py-6"
        />
      )}

      {/* SAI SÓT CỦA CHÍNH DỮ LIỆU XÃ, máy chủ suy ra từ đúng những dòng nó vừa trả về. Tuyến vẫn trả
          200 có chủ ý: đây là màn hình SỬA nó, nên phải mở được kể cả khi đang hỏng. */}
      {kq !== null &&
        kq.ok &&
        kq.duLieu.problems.map((v, i) => (
          <p
            role="alert"
            className="text-danger m-0 text-[12.5px]"
            key={`${v.kind}-${v.work_kind}-${i}`}
          >
            {v.message}
          </p>
        ))}

      {rows.length > 0 && (
        <SlaTable
          rows={rows}
          coQuyenGhi={coQuyenGhi}
          fieldLabels={fieldLabels}
          thaoTac={thaoTac}
          dangGui={dangGui}
          edit={edit}
        />
      )}
    </section>
  );
}

/** One figure, read-only: "{n} giờ", "sau {n} giờ", or "—" for an unset sixth figure. */
function FigureText({
  column,
  value,
}: {
  column: KhoaGio;
  value: number | null;
}) {
  if (value === null)
    return (
      <>
        <span aria-hidden="true">—</span>
        <span className="sr-only">{UNASSIGNED_HOLD_OFF}</span>
      </>
    );
  return (
    <>{AFTER_COLUMNS.has(column) ? afterHoursCell(value) : hoursCell(value)}</>
  );
}

/**
 * The SLA table (`SlaTable.tsx:96-126`) plus the sixth figure "Giữ chưa phân công" before the actions
 * (ADR 0079 decision 3). GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ VỀ: sắp lại theo tên lĩnh vực sẽ tách dòng mặc
 * định khỏi nhóm của nó.
 */
function SlaTable({
  rows,
  coQuyenGhi,
  fieldLabels,
  thaoTac,
  dangGui,
  edit,
}: {
  rows: readonly identity_dongSLARa[];
  coQuyenGhi: boolean;
  fieldLabels: ReadonlyMap<string, string>;
  thaoTac: ThaoTacThoiHan;
  dangGui: boolean;
  edit: InlineEdit | null;
}) {
  const columnCount = 2 + COT_GIO.length + (coQuyenGhi ? 1 : 0);

  return (
    // FITS 1144px WITHOUT SCROLLING THE ACTIONS AWAY: `.data-table thead th` is `nowrap` (legacy layer,
    // so a utility beats it), and the six long figure headings alone overran the content width. Headings
    // wrap; each figure column keeps 4.5rem so "Tiếp nhận" does not collapse to one word per line. Rough
    // budget, 16px cell padding included: kind ~166 + field ~186 + 6 × 88 + actions ~86 ≈ 970px read,
    // ≈ 1050px while editing (6 × (80px input + 16)). Body cells stay `nowrap` (ConfigTable).
    <div className="[&_th]:min-w-[4.5rem] [&_th]:whitespace-normal">
      <ConfigTable
        label="Thời hạn xử lý"
        caption="Số giờ làm việc cho từng loại việc và lĩnh vực của đơn vị"
      >
        <thead>
          <tr>
            <th scope="col">Loại việc</th>
            <th scope="col">Lĩnh vực</th>
            {COT_GIO.map((c) => (
              <th scope="col" key={c}>
                {NHAN_COT[c]}
              </th>
            ))}
            {coQuyenGhi && (
              <th scope="col">
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {rows.map((d) => {
            const kindText = nhanLoaiViec(d.work_kind);
            const fieldText = nhanLinhVuc(d.field, d.is_default, fieldLabels);
            const editing =
              coQuyenGhi && edit !== null && edit.rowId === d.id ? edit : null;
            return (
              <Fragment key={d.id}>
                <tr>
                  <td className="text-navy font-medium">{kindText}</td>
                  <td className={d.is_default ? "text-ink-muted" : undefined}>
                    {fieldText}
                  </td>
                  {COT_GIO.map((c) => (
                    <td
                      key={c}
                      className={
                        c === "resolve_hours" ? "font-semibold" : undefined
                      }
                    >
                      {editing === null ? (
                        <FigureText column={c} value={d[c]} />
                      ) : (
                        <HoursInput column={c} edit={editing} />
                      )}
                    </td>
                  ))}
                  {coQuyenGhi && (
                    <td>
                      <RowActions>
                        {editing !== null ? (
                          <>
                            <Button
                              type="button"
                              variant="primary"
                              size="sm"
                              disabled={editing.busy}
                              aria-busy={editing.busy}
                              onClick={editing.onSave}
                            >
                              <BusyLabel
                                busy={editing.busy}
                                label={NUT_LUU}
                                busyText={BUSY_SAVING}
                              />
                            </Button>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              disabled={editing.busy}
                              onClick={editing.onCancel}
                            >
                              {NUT_HUY}
                            </Button>
                          </>
                        ) : (
                          <>
                            <Button
                              type="button"
                              variant="secondary"
                              size="sm"
                              title={EDIT_TITLE}
                              aria-label={`${EDIT_TITLE} ${kindText} — ${fieldText}`}
                              disabled={dangGui}
                              onClick={() => thaoTac.suaThoiHan(d)}
                            >
                              <Pencil
                                aria-hidden="true"
                                focusable="false"
                                className="size-3.5"
                              />
                            </Button>
                            {/* Hidden on the default row (spec 08): every field without its own row
                              falls back to it. Disabled "?" elsewhere — no delete route yet. */}
                            {!d.is_default && (
                              <PendingFeature info={DELETE_FIELD_SLA}>
                                <Button
                                  type="button"
                                  variant="secondary"
                                  size="sm"
                                  className={cn(
                                    SMALL_BUTTON_CLASS,
                                    "text-danger",
                                  )}
                                  title={DELETE_FIELD_SLA.ten}
                                  aria-label={`${DELETE_FIELD_SLA.ten} ${kindText} — ${fieldText}`}
                                  disabled
                                >
                                  <Trash2
                                    aria-hidden="true"
                                    focusable="false"
                                    className="size-3.5"
                                  />
                                </Button>
                              </PendingFeature>
                            )}
                          </>
                        )}
                      </RowActions>
                    </td>
                  )}
                </tr>
                {editing !== null &&
                  (editing.localError !== "" || editing.serverError !== "") && (
                    <tr>
                      <td colSpan={columnCount}>
                        <div className="space-y-1 whitespace-normal">
                          {editing.localError !== "" && (
                            <p
                              role="alert"
                              className="text-danger m-0 text-[12.5px]"
                            >
                              {editing.localError}
                            </p>
                          )}
                          {editing.serverError !== "" && (
                            <p
                              role="alert"
                              className="text-danger m-0 text-[12.5px]"
                            >
                              {editing.serverError}
                            </p>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
              </Fragment>
            );
          })}
        </tbody>
      </ConfigTable>
    </div>
  );
}

/** One figure in edit mode (`SlaTable.tsx:200-214`): Enter saves, Esc cancels. */
function HoursInput({ column, edit }: { column: KhoaGio; edit: InlineEdit }) {
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      edit.onSave();
    } else if (e.key === "Escape") {
      e.preventDefault();
      edit.onCancel();
    }
  };
  return (
    <input
      type="number"
      min={0}
      name={column}
      aria-label={`${NHAN_COT[column]} (giờ)`}
      className={cn(controlClass, "h-8 w-20 text-[12.5px]")}
      value={edit.draft[column]}
      disabled={edit.busy}
      onChange={(e) =>
        edit.setDraft({
          ...edit.draft,
          [column]: readNumberBox(e.currentTarget),
        })
      }
      onKeyDown={onKeyDown}
    />
  );
}
