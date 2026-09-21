import { type ScreenId, TABS, tabDangSang } from "../features/company-intro/screens";

/**
 * Bottom tab bar.
 *
 * Every tab is a real <button> inside a <nav>, so the screen reader announces it and the
 * active one is announced through `aria-current` rather than through colour. Height comes from
 * `--tap-min` (44px): a target smaller than that is one an unsteady hand cannot hit.
 *
 * ⚠ BỐN Ô, MỖI Ô MỘT HÌNH TRÊN MỘT NHÃN CHỮ — bản mẫu của PM, 21/09/2026.
 *
 *   Hình KHÔNG thay chữ, nó đứng TRÊN chữ. Một thanh tab chỉ có hình là một thanh tab phải học
 *   thuộc, và hình cũng là "trạng thái báo bằng hình ảnh đơn thuần" — đúng thứ README cấm ở
 *   yêu cầu #6. Mỗi glyph vì thế `aria-hidden`: nhãn chữ bên dưới đã nói hết.
 *
 *   Ô đang xem được đánh dấu BA CÁCH cùng lúc — `aria-current`, chữ đậm, một vạch trên — nên nó
 *   không bao giờ phụ thuộc vào màu để đọc ra.
 *
 * ⚠ `tabDangSang` CHỨ KHÔNG PHẢI `props.current`: màn "Danh thiếp" không có ô riêng, và nếu so
 * thẳng thì đứng ở đó thanh tab không sáng ô nào. Xem `screens.ts`.
 */
export function TabBar(props: { current: ScreenId; onSelect: (id: ScreenId) => void }) {
  const dang_sang = tabDangSang(props.current);

  return (
    <nav className="tabbar" aria-label="Chuyển màn hình">
      {TABS.map((screen) => {
        const Glyph = screen.cho.glyph;
        return (
          <button
            key={screen.id}
            type="button"
            className="tabbar__item"
            aria-current={screen.id === dang_sang ? "page" : undefined}
            onClick={() => props.onSelect(screen.id)}
          >
            <Glyph className="tabbar__glyph" />
            <span className="tabbar__nhan">{screen.cho.nhan}</span>
          </button>
        );
      })}
    </nav>
  );
}
