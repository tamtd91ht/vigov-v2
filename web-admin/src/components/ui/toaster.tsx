"use client";

import { useEffect, useRef } from "react";
import { Toaster as SonnerToaster } from "sonner";

/**
 * The ONE toast region of the app (ADR 0068 §Sửa đổi 07/10/2026 lần 6 #4), mounted once in the root
 * layout. Screens call `toast.success(…)` / `toast.error(…)` from `sonner`; they never mount a second
 * region — two regions would show each toast twice and announce it twice.
 *
 * POSITION: bottom-RIGHT (owner 18/10/2026: "error nên show ra qua toast ở góc dưới bên phải màn hình sẽ dễ
 * quan sát hơn") — the prototype had `bottom-center`, which sits exactly behind a dialog's footer.
 * `richColors` and `closeButton` stay the prototype's. The two labels sonner would otherwise read out in
 * English are given in Vietnamese.
 *
 * ABOVE EVERY DIALOG. Our dialogs open with `showModal()`, which puts them in the browser's TOP LAYER; a
 * toast region in the ordinary page is painted UNDER them, so "Số điện thoại đã có trong danh bạ" was hidden
 * behind the very dialog that caused it (owner 18/10/2026, screenshot of "Thêm người dân"). The region is
 * therefore a `popover="manual"` element, itself in the top layer, and it is RE-RAISED every time a dialog
 * opens: the top layer stacks in insertion order, so hiding and showing the popover again puts it above the
 * dialog that just opened. Done once here by wrapping `HTMLDialogElement.prototype.showModal`, so every
 * dialog of the app — present and future — is covered without each one remembering to call a helper.
 *
 * A toast REPORTS an outcome; it never replaces an error shown in place. A form's validation error stays
 * under its field (lần 6 #4), because a toast disappears and the field does not.
 *
 * sonner inserts its stylesheet as a `<style>` element at runtime — the reason it was refused in ADR 0068
 * §4, lifted by lần 6 #4. A strict CSP will need to allow it (`tools/security_debt.json`,
 * `missing-security-headers`).
 */
export function Toaster() {
  const layer = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = layer.current;
    // jsdom and old browsers have no Popover API: the region then stays an ordinary fixed element.
    if (el === null || typeof el.showPopover !== "function") return;
    const raise = () => {
      if (el.matches(":popover-open")) el.hidePopover();
      el.showPopover();
    };
    raise();
    const proto = HTMLDialogElement.prototype;
    const original = proto.showModal;
    proto.showModal = function showModalAndRaiseToasts(this: HTMLDialogElement) {
      original.call(this);
      raise();
    };
    return () => {
      proto.showModal = original;
    };
  }, []);

  return (
    <div
      ref={layer}
      popover="manual"
      data-toast-layer=""
      // Undo the popover's UA box (border, padding, centring, background): the layer is a full-viewport,
      // click-through sheet; only sonner's own list takes pointer events (globals.css has no rule for it).
      className="pointer-events-none fixed inset-0 m-0 h-full max-h-none w-full max-w-none overflow-visible border-0 bg-transparent p-0 [&_[data-sonner-toaster]]:pointer-events-auto"
    >
      <SonnerToaster
        position="bottom-right"
        richColors
        closeButton
        containerAriaLabel="Thông báo"
        toastOptions={{ closeButtonAriaLabel: "Đóng thông báo" }}
      />
    </div>
  );
}
