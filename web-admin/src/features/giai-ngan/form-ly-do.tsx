"use client";

import { LockKeyholeOpen, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Field } from "@/components/ui/field";
import { BusyLabel } from "@/features/danh-ba/busy-label";

import { CAU_THIEU_LY_DO, LY_DO_TOI_DA, lyDoDuDung } from "./nhan-ghi-giai-ngan";

/**
 * Hộp xác nhận KÈM Ô LÝ DO, dùng chung cho ba thao tác `budget.confirm` có thân:
 * gỡ chứng từ · mở khoá chứng từ · xoá dự án.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * VÌ SAO LÀ MỘT Ô NHẬP CHỨ KHÔNG PHẢI MỘT NÚT `Đồng ý`. Đặc tả không biết điều này — §8.2 chỉ vẽ
 * `🗑 Gỡ`, §7 không vẽ gì cho việc xoá dự án. Máy chủ thì đòi `reason` trong thân và từ chối chuỗi
 * rỗng bằng một câu riêng, vì luật 7 bất biến 1 kể tên ba cột `deleted_at` · `deleted_by` ·
 * `delete_reason`, và câu hỏi mở #29 chốt rằng lý do MỞ KHOÁ cũng bắt buộc: những lần mở khoá đã
 * xảy ra không dựng lại được từ bất kỳ nguồn nào sau đó.
 *
 * Nên một hộp xác nhận không có ô lý do ở đây không phải là "gọn hơn" — nó là một biểu mẫu chắc
 * chắn nhận 400 ở mọi lần bấm.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * KHÔNG CÓ GIÁ TRỊ MẶC ĐỊNH CHO Ô LÝ DO. Một câu gợi sẵn kiểu "Nhập sai" là câu sẽ nằm trong vết
 * kiểm toán của hàng trăm bản ghi, và một vết kiểm toán mà mọi dòng giống nhau thì không trả lời
 * được câu hỏi nó sinh ra để trả lời.
 */
export function FormLyDo({
  tieuDe,
  moTa,
  nhanNut,
  dangGui,
  loi,
  huy,
  xacNhan,
  tone = "danger",
  busyText = "Đang lưu…",
}: {
  /** The SPECIFIC question of the confirm box (spec v2 §7), e.g. "Gỡ dự án X?". Also the form's name. */
  tieuDe: string;
  /** Câu nói TRƯỚC hậu quả của thao tác. Nhận vào chứ không viết cứng: ba thao tác, ba hậu quả. */
  moTa: string;
  nhanNut: string;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  xacNhan: (lyDo: string) => void;
  /** `danger` for a removal (red icon and button); `default` for an unlock, which destroys nothing. */
  tone?: "danger" | "default";
  /** Words of the submit button while the request runs; the button keeps its width (`BusyLabel`). */
  busyText?: string;
}) {
  const [lyDo, datLyDo] = useState("");
  const duDieuKien = lyDoDuDung(lyDo);

  function guiNgay(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!duDieuKien) return;
    xacNhan(lyDo.trim());
  }

  return (
    <ConfirmDialog
      as="form"
      tone={tone}
      icon={tone === "danger" ? Trash2 : LockKeyholeOpen}
      title={tieuDe}
      titleAs="h4"
      aria-label={tieuDe}
      onSubmit={guiNgay}
      actions={
        <>
          <Button
            type="submit"
            variant={tone === "danger" ? "danger" : "primary"}
            disabled={dangGui || !duDieuKien}
            aria-busy={dangGui || undefined}
          >
            <BusyLabel busy={dangGui} label={nhanNut} busyText={busyText} />
          </Button>
          <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
            Huỷ
          </Button>
        </>
      }
    >
      <p className="m-0">{moTa}</p>

      <Field
        label="Lý do *"
        htmlFor="ly-do-giai-ngan"
        grow="auto"
        hint={!duDieuKien ? CAU_THIEU_LY_DO : undefined}
      >
        <textarea
          id="ly-do-giai-ngan"
          name="ly-do-giai-ngan"
          rows={3}
          value={lyDo}
          maxLength={LY_DO_TOI_DA}
          onChange={(e) => datLyDo(e.target.value)}
        />
      </Field>

      {/* NGUYÊN VĂN câu máy chủ: 409 của các tuyến này mang đúng quy tắc nghiệp vụ đã từ chối
          ("dự án còn chứng từ", "người vừa khoá không tự mở lại được"), và viết lại nó ở client là
          dựng bản sao thứ hai của một quy tắc rồi để nó trôi. */}
      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}
    </ConfirmDialog>
  );
}
