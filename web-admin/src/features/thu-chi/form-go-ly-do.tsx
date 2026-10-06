"use client";

import { Trash2 } from "lucide-react";
import type { FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Field } from "@/components/ui/field";
import { BusyLabel } from "@/features/danh-ba/busy-label";

/**
 * Hộp xác nhận GỠ — **có ô lý do**, và ô ấy không phải để cho đẹp.
 *
 * §6 chỉ vẽ "hộp xác nhận"; máy chủ đòi `reason` trong thân và từ chối 400 khi thiếu (luật 7 bất
 * biến 1 kể tên `delete_reason`). Một hộp chỉ có nút Đồng ý sẽ nhận 400 ở mọi lần bấm.
 *
 * TỆP RIÊNG vì hai nơi dùng nó — bảng (gỡ bảng, gỡ khoản mục) và hộp `⇄` (gỡ đợt) — và hộp `⇄`
 * nằm ở tệp khác; để nó trong `bang-thu-chi.tsx` là một vòng nhập giữa hai tệp.
 *
 * Lý do chỉ gồm khoảng trắng bị chặn ở đây chứ không chỉ bằng `required`: trình duyệt coi `" "` là
 * đã điền, còn máy chủ thì không, và cán bộ nhận một 400 cho một ô trông như đã điền.
 */
export function FormGoKemLyDo({
  tieuDe,
  canhBao,
  dangGui,
  huy,
  luu,
  idTruong = "go-reason",
  submitLabel = "Gỡ",
  formId,
  className,
}: {
  /** `id` of the `<form>` — a surrounding `ModalDialog` names itself after it (`aria-labelledby`). */
  formId?: string;
  /** Extra classes on the box, e.g. `shadow-none` inside a `ModalDialog` that already draws a frame. */
  className?: string;
  /** The SPECIFIC question of the confirm box (spec v2 §7), e.g. "Gỡ bảng X?". */
  tieuDe: string;
  canhBao: string;
  dangGui: boolean;
  huy: () => void;
  luu: (lyDo: string) => void;
  /** `id` của ô lý do — hộp `⇄` truyền một `id` khác để không trùng với hộp gỡ của bảng. */
  idTruong?: string;
  /** The confirm button names the action ("Gỡ bảng"), never "OK" (spec v2 §7). */
  submitLabel?: string;
}) {
  return (
    <ConfirmDialog
      as="form"
      tone="danger"
      icon={Trash2}
      id={formId}
      className={className}
      title={tieuDe}
      aria-label={tieuDe}
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const lyDo = String(fd.get("reason") ?? "").trim();
        if (lyDo === "") return;
        luu(lyDo);
      }}
      actions={
        <>
          <Button type="submit" variant="danger" disabled={dangGui} aria-busy={dangGui || undefined}>
            <BusyLabel busy={dangGui} label={submitLabel} busyText="Đang gỡ…" />
          </Button>
          <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
            Huỷ
          </Button>
        </>
      }
    >
      <p className="m-0">{canhBao}</p>
      <Field label="Lý do gỡ (bắt buộc, được lưu cùng bản ghi)" htmlFor={idTruong} grow="auto">
        <input id={idTruong} name="reason" className="o-nhap" type="text" required maxLength={500} />
      </Field>
    </ConfirmDialog>
  );
}
