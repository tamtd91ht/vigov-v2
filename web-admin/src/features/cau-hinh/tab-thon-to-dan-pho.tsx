"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";

import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layLoaiDonViDanCu } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhBaChonNguoiRa,
  identity_danhSachLoaiDonViDanCuRa,
  identity_thonToDanPhoRa,
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";
import { QUYEN_QUAN_LY_SO_DO, quyetDinhTheoKhoa } from "@/lib/quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { IMPORT_BUTTON } from "./excel-import-flow";
import { ExcelImportPanel } from "./excel-import-panel";
import { RESIDENTIAL_UNIT_IMPORT_TARGET } from "./excel-import-targets";
import { NUT_HUY, NUT_LUU } from "./nhan-so-do";
import {
  loaiDonVi,
  lopLoaiDonVi,
  lopTrangThaiDiaBan,
  nhanLoaiDonVi,
  nhanSoDem,
  nhanTrangThaiDiaBan,
  GHI_CHU_CHI_XEM_THON,
  THON_RONG,
} from "./nhan-thon";
import {
  ADD_BUTTON,
  CODE_HELP_CREATE,
  CONFIRM_RETIRE_BUTTON,
  COUNT_HELP,
  CREATE_TITLE,
  EDIT_BUTTON,
  FIELD_CODE,
  FIELD_HEAD,
  FIELD_HOUSEHOLDS,
  FIELD_NAME,
  FIELD_ORDER,
  FIELD_POPULATION,
  FIELD_TYPE,
  HEAD_HELP,
  HEAD_NONE,
  NAME_HELP,
  NO_WRITE_PERMISSION,
  ORDER_HELP,
  REACTIVATE_BUTTON,
  RESIDENTIAL_UNIT_API,
  RETIRE_BUTTON,
  TYPE_HELP,
  TYPE_NONE,
  codeReadOnly,
  draftForCreate,
  draftFromUnit,
  editButtonLabel,
  editTitle,
  headOptions,
  openCreate,
  reactivateButtonLabel,
  retireButtonLabel,
  retireConfirmSentence,
  submitResidentialUnitForm,
  toggleResidentialUnitActive,
  typeOptions,
  type PickerOption,
  type ResidentialUnitDraft,
  type ResidentialUnitFormOpen,
} from "./residential-unit-form";

/**
 * Tab "Thôn / Tổ dân phố" — `docs/ui-ux/14-cau-hinh.md §2`: danh sách địa bàn của đơn vị; thêm, sửa,
 * Ngừng dùng / Dùng lại, và nhập từ Excel (người dùng chốt 29/09/2026, ADR 0059 §2).
 *
 * TAB RIÊNG, KHÔNG NHẬP VÀO TAB DANH MỤC, dù cả hai cùng đọc một tuyến kiểu danh sách: một thôn
 * không phải một mục danh mục. Hợp đồng nói ra điều đó bằng chính tên trường (`name` chứ không
 * phải `label`), máy chủ nói ra bằng hai con số chỉ một địa bàn có thật mới có (số hộ, nhân
 * khẩu), và đặc tả tách đôi thành hai tab.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * DANH SÁCH HIỆN CHO MỌI TÀI KHOẢN, CHỈ NÚT GHI MỚI ẨN — cùng khuôn tab Sơ đồ tổ chức, và vì cùng một
 * lý do ở máy chủ: `GET /api/v1/residential-units` khai `any-authenticated` (tên địa bàn có ở bộ lọc
 * phản ánh), còn hai tuyến ghi và ba tuyến nhập khai `RequirePermission("admin.org")`. Ẩn nút là TIỆN
 * DỤNG, không phải biện pháp: máy chủ kiểm khoá trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * KHÔNG CÓ XOÁ, CÓ CHỦ Ý. "Ngừng dùng" là `active: false`; địa bàn ở lại danh sách, hồ sơ đang trỏ
 * vào nó vẫn in được tên (`residential_unit_write.go:20-24`). Ngừng dùng hỏi xác nhận một lần, nói rõ
 * điều ấy; Dùng lại thì không — nó chỉ đưa một địa bàn trở lại ô chọn.
 *
 * ĐỌC LẠI SAU MỖI LẦN GHI, KHÔNG VÁ TẠI CHỖ: thứ tự và nhãn loại là của máy chủ.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** Ba nhánh rời nhau — cùng khuôn với `danh-ba-can-bo.tsx`. Không nhánh nào suy ra được từ nhánh khác. */
export type ResidentialUnitsLoad =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly units: readonly identity_thonToDanPhoRa[] };

/** What a row's buttons call. */
export type ResidentialUnitActions = {
  readonly add: () => void;
  readonly edit: (u: identity_thonToDanPhoRa) => void;
  readonly askRetire: (u: identity_thonToDanPhoRa) => void;
  readonly confirmRetire: () => void;
  readonly cancelRetire: () => void;
  readonly reactivate: (u: identity_thonToDanPhoRa) => void;
  readonly openImport: () => void;
};

const NAME_INPUT_ID = "o-ten-thon";

export function TabThonToDanPho() {
  const [load, setLoad] = useState<ResidentialUnitsLoad>({ phase: "loading" });
  const [reads, setReads] = useState(0);

  const [open, setOpen] = useState<ResidentialUnitFormOpen | null>(null);
  const [draft, setDraft] = useState<ResidentialUnitDraft>(draftForCreate());
  const [localError, setLocalError] = useState("");
  const [serverError, setServerError] = useState("");
  const [sending, setSending] = useState(false);
  const [doneSentence, setDoneSentence] = useState("");

  const [retiring, setRetiring] = useState<identity_thonToDanPhoRa | null>(null);
  /** A toggle refused by the server, said under the row it was pressed on. */
  const [rowError, setRowError] = useState<{ id: string; message: string } | null>(null);
  const [importOpen, setImportOpen] = useState(false);

  const [types, setTypes] = useState<KetQua<identity_danhSachLoaiDonViDanCuRa> | null>(null);
  const [directory, setDirectory] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);

  const openedBy = useRef<string | null>(null);

  const phien = usePhien();
  /** Ba trạng thái: chưa đọc xong phiên thì chưa vẽ nút ghi nào, cũng chưa nói "thiếu quyền". */
  const decision = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_SO_DO);
  const canWrite = decision !== null && decision.hien;

  /**
   * ĐỌC LẠI theo `reads`. KHÔNG ĐỆM QUA LẦN MỞ MÀN HÌNH: danh sách địa bàn là dữ liệu của MỘT đơn vị,
   * và một bản đệm sống lâu hơn yêu cầu, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng
   * của một lần dữ liệu đơn vị này hiện trên màn hình đơn vị khác (luật 1, cấm #1).
   */
  useEffect(() => {
    let dropped = false;
    layDanhSachThonToDanPho().then((kq) => {
      if (dropped) return;
      setLoad(kq.ok ? { phase: "ready", units: kq.duLieu.items } : { phase: "error", message: kq.thongBao });
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  // The two pickers' sources, read only for an account that can write — nobody else opens the form.
  useEffect(() => {
    if (!canWrite) return;
    let dropped = false;
    layLoaiDonViDanCu().then((kq) => {
      if (!dropped) setTypes(kq);
    });
    layDanhBaChonNguoi().then((kq) => {
      if (!dropped) setDirectory(kq);
    });
    return () => {
      dropped = true;
    };
  }, [canWrite]);

  // Focus: into the name box when the form opens; back to the opening button when it closes.
  useEffect(() => {
    if (open !== null) {
      document.getElementById(NAME_INPUT_ID)?.focus();
      return;
    }
    if (openedBy.current !== null) {
      document.getElementById(openedBy.current)?.focus();
      openedBy.current = null;
    }
  }, [open]);

  const clearMessages = useCallback(() => {
    setLocalError("");
    setServerError("");
    setDoneSentence("");
    setRowError(null);
  }, []);

  const actions: ResidentialUnitActions = {
    add: () => {
      openedBy.current = ADD_BUTTON_ID;
      clearMessages();
      setRetiring(null);
      setImportOpen(false);
      setOpen(openCreate(() => crypto.randomUUID()));
      setDraft(draftForCreate());
    },
    edit: (u) => {
      openedBy.current = editButtonId(u.id);
      clearMessages();
      setRetiring(null);
      setImportOpen(false);
      setOpen({ kind: "edit", unit: u });
      setDraft(draftFromUnit(u));
    },
    askRetire: (u) => {
      clearMessages();
      setOpen(null);
      setRetiring(u);
    },
    cancelRetire: () => setRetiring(null),
    confirmRetire: () => {
      if (retiring === null || sending) return;
      const u = retiring;
      setSending(true);
      void toggleResidentialUnitActive(u, RESIDENTIAL_UNIT_API).then((r) => {
        setSending(false);
        setRetiring(null);
        if (r.kind === "done") {
          setDoneSentence(r.sentence);
          setReads((n) => n + 1);
        } else if (r.kind === "serverError") {
          setRowError({ id: u.id, message: r.message });
        }
      });
    },
    reactivate: (u) => {
      if (sending) return;
      clearMessages();
      setOpen(null);
      setRetiring(null);
      setSending(true);
      void toggleResidentialUnitActive(u, RESIDENTIAL_UNIT_API).then((r) => {
        setSending(false);
        if (r.kind === "done") {
          setDoneSentence(r.sentence);
          setReads((n) => n + 1);
        } else if (r.kind === "serverError") {
          setRowError({ id: u.id, message: r.message });
        }
      });
    },
    openImport: () => {
      clearMessages();
      setOpen(null);
      setRetiring(null);
      setImportOpen(true);
    },
  };

  const submit = () => {
    if (open === null || sending) return;
    setLocalError("");
    setServerError("");
    setSending(true);
    void submitResidentialUnitForm(open, draft, RESIDENTIAL_UNIT_API).then((r) => {
      setSending(false);
      if (r.kind === "localError") {
        setLocalError(r.message);
        return;
      }
      if (r.kind === "serverError") {
        // The form KEEPS what was typed and KEEPS the create key: the next press is a retry of the
        // SAME add, never a second unit.
        setServerError(r.message);
        return;
      }
      setOpen(null);
      setDoneSentence(r.sentence);
      setReads((n) => n + 1);
    });
  };

  const current = open !== null && open.kind === "edit" ? open.unit : null;
  const form =
    open === null ? null : (
      <ResidentialUnitForm
        open={open}
        draft={draft}
        setDraft={setDraft}
        typeChoices={typeOptions(
          types !== null && types.ok ? types.duLieu.items : [],
          current === null ? null : { code: current.type_code, label: current.type_label },
        )}
        headChoices={headOptions(
          directory !== null && directory.ok ? directory.duLieu.items : [],
          current === null ? null : { code: current.head_staff_code, name: current.head_staff_name },
        )}
        typesError={types !== null && !types.ok ? types.thongBao : ""}
        directoryError={directory !== null && !directory.ok ? directory.thongBao : ""}
        localError={localError}
        serverError={serverError}
        sending={sending}
        onSubmit={submit}
        onCancel={() => {
          setOpen(null);
          setLocalError("");
          setServerError("");
        }}
      />
    );

  return (
    <section className="tab-thon-to-dan-pho" aria-labelledby="tieu-de-thon">
      <h2 id="tieu-de-thon">Thôn / Tổ dân phố</h2>
      {decision !== null && !decision.hien && decision.vi === "khong-doc-duoc" && (
        <p className="thong-bao-loi" role="alert">
          {decision.thongBao}
        </p>
      )}
      <ResidentialUnitsView
        load={load}
        canWrite={canWrite}
        missingPermission={decision !== null && !decision.hien && decision.vi === "khong-du-quyen"}
        actions={actions}
        sending={sending}
        doneSentence={doneSentence}
        topPanel={
          importOpen && canWrite ? (
            <ExcelImportPanel
              target={RESIDENTIAL_UNIT_IMPORT_TARGET}
              onImported={() => setReads((n) => n + 1)}
              onClose={() => setImportOpen(false)}
            />
          ) : (
            form
          )
        }
        retiring={retiring}
        rowError={rowError}
      />
    </section>
  );
}

const ADD_BUTTON_ID = "nut-them-thon";

/** `id` of a row's edit button — focus returns there when the form closes. */
export function editButtonId(unitId: string): string {
  return `nut-sua-thon-${unitId}`;
}

/**
 * The tab's body: guidance, buttons, the list. PURE PRESENTATION and EXPORTED so the test renders the
 * allowed AND the denied case with `react-dom/server` — "no write button without `admin.org`" is a
 * fact of this JSX, not of a pure function.
 */
export function ResidentialUnitsView({
  load,
  canWrite,
  missingPermission,
  actions,
  sending,
  doneSentence,
  topPanel,
  retiring,
  rowError,
}: {
  load: ResidentialUnitsLoad;
  canWrite: boolean;
  missingPermission: boolean;
  actions: ResidentialUnitActions;
  sending: boolean;
  doneSentence: string;
  topPanel: ReactNode;
  retiring: identity_thonToDanPhoRa | null;
  rowError: { id: string; message: string } | null;
}) {
  return (
    <>
      <p className="ghi-chu">{GHI_CHU_CHI_XEM_THON}</p>

      {canWrite && (
        <p className="cum-nut">
          <button type="button" className="nut-phu" onClick={actions.openImport}>
            {IMPORT_BUTTON}
          </button>
          <button type="button" className="nut-phu" id={ADD_BUTTON_ID} onClick={actions.add}>
            {ADD_BUTTON}
          </button>
        </p>
      )}
      {missingPermission && <p className="trang-thai-rong">{NO_WRITE_PERMISSION}</p>}
      {doneSentence !== "" && <p role="status">{doneSentence}</p>}

      {topPanel}

      {load.phase === "loading" && <p role="status">Đang tải danh sách địa bàn…</p>}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`. */}
      {load.phase === "error" && (
        <p className="thong-bao-loi" role="alert">
          {load.message}
        </p>
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI. Máy chủ trả `items: []`, không bao giờ `null`. */}
      {load.phase === "ready" && load.units.length === 0 && <p className="trang-thai-rong">{THON_RONG}</p>}

      {load.phase === "ready" && load.units.length > 0 && (
        <UnitsTable
          units={load.units}
          canWrite={canWrite}
          actions={actions}
          sending={sending}
          retiring={retiring}
          rowError={rowError}
        />
      )}
    </>
  );
}

/**
 * Bảng địa bàn. Giữ nguyên thứ tự máy chủ trả về. KHÔNG LỌC BỎ địa bàn đã ngừng dùng: một hồ sơ đã
 * lập ở đó vẫn phải tra ra được tên của nó, và đây là chỗ duy nhất đưa nó vào dùng lại được.
 */
function UnitsTable({
  units,
  canWrite,
  actions,
  sending,
  retiring,
  rowError,
}: {
  units: readonly identity_thonToDanPhoRa[];
  canWrite: boolean;
  actions: ResidentialUnitActions;
  sending: boolean;
  retiring: identity_thonToDanPhoRa | null;
  rowError: { id: string; message: string } | null;
}) {
  const columns = canWrite ? 8 : 7;
  return (
    <div className="bang-cuon" role="region" aria-label="Danh sách thôn / tổ dân phố" tabIndex={0}>
      <table className="bang-danh-muc">
        <caption className="an-thi-giac">Danh sách thôn và tổ dân phố của đơn vị</caption>
        <thead>
          <tr>
            <th scope="col">Tên</th>
            <th scope="col">Mã</th>
            <th scope="col">Loại</th>
            <th scope="col">{FIELD_HEAD}</th>
            <th scope="col">Số hộ</th>
            <th scope="col">Nhân khẩu</th>
            <th scope="col">Trạng thái</th>
            {canWrite && <th scope="col">Thao tác</th>}
          </tr>
        </thead>
        <tbody>
          {units.map((t) => {
            // Ba ca của cột Loại (`nhan-thon.ts`): không ca nào để lại một ô trống.
            const loai = loaiDonVi(t.type_code, t.type_label);
            return (
              <UnitRows key={t.id}>
                <tr>
                  <td>{t.name}</td>
                  {/* Mã font mono: một slug được gõ lại và đọc qua điện thoại. */}
                  <td className="ma-muc">{t.code}</td>
                  <td>
                    <span className={lopLoaiDonVi(loai)}>{nhanLoaiDonVi(loai)}</span>
                  </td>
                  <td>{t.head_staff_code === "" ? HEAD_NONE : t.head_staff_name}</td>
                  {/* `null` KHÔNG hiện thành `0` (`nhan-thon.ts`). */}
                  <td>{nhanSoDem(t.household_count)}</td>
                  <td>{nhanSoDem(t.population_count)}</td>
                  <td>
                    <span className={lopTrangThaiDiaBan(t.active)}>{nhanTrangThaiDiaBan(t.active)}</span>
                  </td>
                  {canWrite && (
                    <td>
                      <span className="cum-nut">
                        <button
                          type="button"
                          className="nut-phu"
                          id={editButtonId(t.id)}
                          aria-label={editButtonLabel(t.name)}
                          onClick={() => actions.edit(t)}
                          disabled={sending}
                        >
                          {EDIT_BUTTON}
                        </button>
                        {t.active ? (
                          <button
                            type="button"
                            className="nut-phu"
                            aria-label={retireButtonLabel(t.name)}
                            onClick={() => actions.askRetire(t)}
                            disabled={sending}
                          >
                            {RETIRE_BUTTON}
                          </button>
                        ) : (
                          <button
                            type="button"
                            className="nut-phu"
                            aria-label={reactivateButtonLabel(t.name)}
                            onClick={() => actions.reactivate(t)}
                            disabled={sending}
                          >
                            {REACTIVATE_BUTTON}
                          </button>
                        )}
                      </span>
                    </td>
                  )}
                </tr>
                {canWrite && retiring !== null && retiring.id === t.id && (
                  <tr>
                    <td colSpan={columns}>
                      <p>{retireConfirmSentence(t.name)}</p>
                      <p className="cum-nut">
                        <button type="button" className="nut-chinh" onClick={actions.confirmRetire} disabled={sending}>
                          {CONFIRM_RETIRE_BUTTON}
                        </button>
                        <button type="button" className="nut-phu" onClick={actions.cancelRetire} disabled={sending}>
                          {NUT_HUY}
                        </button>
                      </p>
                    </td>
                  </tr>
                )}
                {rowError !== null && rowError.id === t.id && (
                  <tr>
                    <td colSpan={columns}>
                      <p className="thong-bao-loi" role="alert">
                        {rowError.message}
                      </p>
                    </td>
                  </tr>
                )}
              </UnitRows>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/** A keyed fragment for one unit's row and the rows under it (confirm, error). */
function UnitRows({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

/**
 * The add / edit form. PURE PRESENTATION — every value in through `draft`, every change out through
 * `setDraft`; bodies are built in `residential-unit-form.ts`. EXPORTED so the server's refusal is
 * rendered VERBATIM in a test.
 *
 * `Mã` IS AN INPUT ONLY WHEN ADDING. On edit it is text: an editable code box promises what the server
 * refuses with 400 `code_not_editable` (rule 7, invariant 3).
 */
export function ResidentialUnitForm({
  open,
  draft,
  setDraft,
  typeChoices,
  headChoices,
  typesError,
  directoryError,
  localError,
  serverError,
  sending,
  onSubmit,
  onCancel,
}: {
  open: ResidentialUnitFormOpen;
  draft: ResidentialUnitDraft;
  setDraft: (d: ResidentialUnitDraft) => void;
  typeChoices: readonly PickerOption[];
  headChoices: readonly PickerOption[];
  typesError: string;
  directoryError: string;
  localError: string;
  serverError: string;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const title = open.kind === "edit" ? editTitle(open.unit.name) : CREATE_TITLE;
  return (
    <form
      className="form-danh-muc"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        // Esc closes like `Huỷ` — except while sending, so a send in flight is not lost from view.
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <h4>{title}</h4>

      <TextBox id={NAME_INPUT_ID} label={FIELD_NAME} help={NAME_HELP} value={draft.name} onChange={(v) => setDraft({ ...draft, name: v })} />

      {open.kind === "create" ? (
        <TextBox id="o-ma-thon" label={FIELD_CODE} help={CODE_HELP_CREATE} value={draft.code} onChange={(v) => setDraft({ ...draft, code: v })} />
      ) : (
        <p className="ghi-chu">{codeReadOnly(open.unit.code)}</p>
      )}

      <div className="o-nhap">
        <label htmlFor="o-loai-thon">{FIELD_TYPE}</label>
        <select
          id="o-loai-thon"
          value={draft.typeCode}
          onChange={(e) => setDraft({ ...draft, typeCode: e.target.value })}
          aria-describedby="giai-thich-loai-thon"
        >
          <option value="">{TYPE_NONE}</option>
          {typeChoices.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <p className="ghi-chu" id="giai-thich-loai-thon">
          {TYPE_HELP}
        </p>
        {typesError !== "" && (
          <p className="thong-bao-loi" role="alert">
            {typesError}
          </p>
        )}
      </div>

      <div className="o-nhap">
        <label htmlFor="o-truong-thon">{FIELD_HEAD}</label>
        <select
          id="o-truong-thon"
          value={draft.headStaffCode}
          onChange={(e) => setDraft({ ...draft, headStaffCode: e.target.value })}
          aria-describedby="giai-thich-truong-thon"
        >
          <option value="">{HEAD_NONE}</option>
          {headChoices.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <p className="ghi-chu" id="giai-thich-truong-thon">
          {HEAD_HELP}
        </p>
        {directoryError !== "" && (
          <p className="thong-bao-loi" role="alert">
            {directoryError}
          </p>
        )}
      </div>

      {/* `inputMode="numeric"`, not `type="number"`: a scroll wheel over a number box changes the value
          without the user noticing — and a blank box must stay blank ("chưa nhập"), never become 0. */}
      <TextBox id="o-so-ho" label={FIELD_HOUSEHOLDS} help={COUNT_HELP} numeric value={draft.households} onChange={(v) => setDraft({ ...draft, households: v })} />
      <TextBox id="o-nhan-khau" label={FIELD_POPULATION} help={COUNT_HELP} numeric value={draft.population} onChange={(v) => setDraft({ ...draft, population: v })} />
      <TextBox id="o-thu-tu-thon" label={FIELD_ORDER} help={ORDER_HELP} numeric value={draft.order} onChange={(v) => setDraft({ ...draft, order: v })} />

      {/* TWO ERROR AREAS: a local one (nothing sent) and the server's sentence, verbatim. */}
      {localError !== "" && (
        <p className="thong-bao-loi" role="alert">
          {localError}
        </p>
      )}
      {serverError !== "" && (
        <p className="thong-bao-loi" role="alert">
          {serverError}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={sending}>
          {NUT_LUU}
        </button>
        <button type="button" className="nut-phu" onClick={onCancel} disabled={sending}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}

function TextBox({
  id,
  label,
  help,
  value,
  onChange,
  numeric = false,
}: {
  id: string;
  label: string;
  help: string;
  value: string;
  onChange: (v: string) => void;
  numeric?: boolean;
}) {
  return (
    <div className="o-nhap">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        value={value}
        inputMode={numeric ? "numeric" : undefined}
        onChange={(e) => onChange(e.target.value)}
        aria-describedby={`${id}-giai-thich`}
      />
      <p className="ghi-chu" id={`${id}-giai-thich`}>
        {help}
      </p>
    </div>
  );
}
