/**
 * XÃ CỐ ĐỊNH CỦA BẢN DỰNG — tên miền xã mà `scripts/deploy.mjs --domain=<x> --vao-thang` nung vào
 * bundle, hoặc `null` (app chung, và mọi bản dựng/test không qua cờ ấy). Chủ dự án chọn 27/09/2026.
 *
 * Có giá trị thì app mở THẲNG vào xã ấy: không màn giới thiệu ViHAT, không bước "Đúng xã này chưa?"
 * — người dân mở app riêng của xã là đã chọn xã. Tên miền chỉ DẪN GIAO DIỆN như `d` trên QR: tên xã
 * vẫn do `/communes` trả, xã của phiên vẫn do máy chủ quyết. Không cấp quyền gì mà một QR công khai
 * mang `d` không cấp.
 *
 * Sai khuôn thì `null` (mở như app chung) — không bao giờ đoán một xã. Bước dựng đã chặn giá trị
 * sai khuôn (`xaCoDinh` trong scripts/cau-hinh.mjs); đây là lớp thứ hai, rẻ.
 */
import { laTenMien } from "./launch-params";

declare const __VIGOV_XA_CO_DINH__: string | undefined;

export function docXaCoDinh(gia_tri: unknown): string | null {
  return typeof gia_tri === "string" && laTenMien(gia_tri) ? gia_tri : null;
}

export const XA_CO_DINH: string | null = docXaCoDinh(
  typeof __VIGOV_XA_CO_DINH__ === "string" ? __VIGOV_XA_CO_DINH__ : undefined,
);
