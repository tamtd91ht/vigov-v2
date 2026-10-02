"use client";

import { CircleCheck, ClipboardPlus } from "lucide-react";
import { useEffect, useState } from "react";

import { FormGiaoViec } from "@/features/nhiem-vu/so-nhiem-vu";
import type { DanhMucNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import { createTaskFromPetition } from "@/lib/api/phieu-phan-anh";
import type {
  identity_danhBaChonNguoiRa,
  petitions_petitionTaskIn,
} from "@/lib/api/schema.gen";
import { QUYEN_DUYET_GIA_HAN } from "@/lib/quyen";

import {
  PETITION_TASK_BUTTON,
  petitionTaskCreated,
  petitionTaskSourceNote,
  petitionTaskTitle,
  TASK_REGISTER_HREF,
  TASK_REGISTER_LINK_LABEL,
} from "./nhan-phieu";
import { ACT_CLASS, buttonClass, Glyph } from "./petition-ui";

/**
 * `Tạo nhiệm vụ` on the petition drawer (`docs/ui-ux/09` §13) — POST …/tasks.
 *
 * THE SAME FORM AS `+ Giao việc mới`, NOT A SECOND ONE: `FormGiaoViec` is reused exactly as the meeting
 * screen reuses it for "Tách thành nhiệm vụ" (`features/bien-ban/so-bien-ban.tsx`). A copy here would be
 * two forms posting the same shape, and the day one gains a field the other stays green (rule 9, #2).
 *
 * WHY A COMPONENT OF ITS OWN, NOT HOOKS IN `ChiTietPhieu`: `chon-can-bo.test.tsx` calls `ChiTietPhieu` as
 * a plain function and seeds its `useState` calls BY ORDER; a hook added there shifts every seed, and a
 * `useEffect` there throws. This block reads the network and holds its own state.
 *
 * CATALOGUES AND THE LEADER DIRECTORY ARE READ ON FIRST OPEN, not on every drawer: most drawers never
 * open this form. The whole-commune directory is the drawer's own (`danhBa`) — one read, not two.
 */
export function PetitionTaskBlock({
  lookupCode,
  danhBa,
  onCreated,
}: {
  lookupCode: string;
  /** The drawer's staff directory (`null` = still loading) — the same answer, not a second read. */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  /** After a 201: the drawer re-reads the petition log, where the server just wrote `tao-nhiem-vu`. */
  onCreated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [catalogue, setCatalogue] = useState<DanhMucNhiemVu>(EMPTY_CATALOGUE);
  const [leaders, setLeaders] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createdCode, setCreatedCode] = useState<string | null>(null);
  // Bumped after each success, as the `key` of the view: a new opening is a new Idempotency-Key, while a
  // failed send keeps the form — and its key — for the retry.
  const [openings, setOpenings] = useState(0);

  useEffect(() => {
    if (!open || loaded) return;
    let dropped = false;
    Promise.all([layLoaiNhiemVu(), layMucUuTienNhiemVu(), layKhoiNhiemVu(), layDanhMucBoPhan()]).then(
      ([types, priorities, blocs, units]) => {
        if (dropped) return;
        // A catalogue that fails leaves its select empty; it does not break the drawer.
        setCatalogue({
          loai: types.ok ? types.duLieu.items : [],
          mucUuTien: priorities.ok ? priorities.duLieu.items : [],
          khoi: blocs.ok ? blocs.duLieu.items : [],
          boPhan: units.ok ? units.duLieu.items : [],
        });
        setLoaded(true);
      },
    );
    // `Lãnh đạo giao việc` offers only holders of `task.extend` — that person approves extensions
    // (ADR 0038). The whole-commune directory must never feed that box.
    layDanhBaChonNguoi(undefined, QUYEN_DUYET_GIA_HAN).then((kq) => {
      if (!dropped) setLeaders(kq);
    });
    return () => {
      dropped = true;
    };
  }, [open, loaded]);

  function send(body: petitions_petitionTaskIn, idempotencyKey: string): void {
    setSending(true);
    createTaskFromPetition(lookupCode, body, idempotencyKey).then((kq) => {
      setSending(false);
      if (!kq.ok) {
        // VERBATIM, whatever the 409's code — the refusal list is the server's (see the API function).
        setError(kq.thongBao);
        return;
      }
      setError(null);
      setCreatedCode(kq.duLieu.code);
      setOpen(false);
      setOpenings((n) => n + 1);
      onCreated();
    });
  }

  return (
    <PetitionTaskView
      key={openings}
      lookupCode={lookupCode}
      open={open}
      catalogue={catalogue}
      danhBa={danhBa}
      leaders={leaders}
      sending={sending}
      error={error}
      createdCode={createdCode}
      toggle={() => {
        setOpen((o) => !o);
        setError(null);
        setCreatedCode(null);
      }}
      cancel={() => {
        setOpen(false);
        setError(null);
      }}
      send={send}
    />
  );
}

const EMPTY_CATALOGUE: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/** Stateless: everything it draws comes in, so a test renders every phase without a DOM. */
export function PetitionTaskView({
  lookupCode,
  open,
  catalogue,
  danhBa,
  leaders,
  sending,
  error,
  createdCode,
  toggle,
  cancel,
  send,
}: {
  lookupCode: string;
  open: boolean;
  catalogue: DanhMucNhiemVu;
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  leaders: KetQua<identity_danhBaChonNguoiRa> | null;
  sending: boolean;
  error: string | null;
  createdCode: string | null;
  toggle: () => void;
  cancel: () => void;
  send: (body: petitions_petitionTaskIn, idempotencyKey: string) => void;
}) {
  // One act of the drawer's `Xử lý phiếu` card (`ACT_CLASS`): secondary, because the card's solid
  // button is the processing act itself; this one books follow-up work.
  return (
    <div className={ACT_CLASS}>
      <div className="cum-nut">
        <button
          type="button"
          className={buttonClass("secondary")}
          id="nut-tao-nhiem-vu-tu-phieu"
          aria-expanded={open}
          disabled={sending}
          onClick={toggle}
        >
          <Glyph icon={ClipboardPlus} />
          {PETITION_TASK_BUTTON}
        </button>
      </div>

      {createdCode !== null && (
        <p className="ghi-chu m-0 inline-flex flex-wrap items-center gap-1" role="status">
          <Glyph icon={CircleCheck} className="size-3.5 shrink-0 text-success-600" />
          {petitionTaskCreated(createdCode)} <a href={TASK_REGISTER_HREF}>{TASK_REGISTER_LINK_LABEL}</a>
        </p>
      )}

      {open && (
        <div>
          <p className="ghi-chu mt-0">{petitionTaskSourceNote(lookupCode)}</p>
          <FormGiaoViec
            danhMuc={catalogue}
            danhBa={danhBa}
            danhBaLanhDao={leaders}
            // The route takes `documents` and `note` (`petitions_petitionTaskIn`), unlike the meeting
            // split — so the §7.2 lists are drawn here.
            coDanhSachVanBan
            dangGui={sending}
            loi={error}
            huy={cancel}
            // `FormGiaoViec` hands over `petitions_taoNhiemVuVao`; `createTaskFromPetition` copies it
            // field by field, which is where `source` / `source_id` are left behind.
            giaoViec={(body, key) => send(body, key)}
            tieuDeCoSan={petitionTaskTitle(lookupCode)}
          />
        </div>
      )}
    </div>
  );
}
