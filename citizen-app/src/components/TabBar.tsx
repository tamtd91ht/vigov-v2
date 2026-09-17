import { SCREENS, type ScreenId } from "../features/company-intro/screens";

/**
 * Bottom tab bar.
 *
 * Every tab is a real <button> inside a <nav>, so the screen reader announces it and the
 * active one is announced through `aria-current` rather than through colour. Height comes from
 * `--tap-min` (44px): a target smaller than that is one an unsteady hand cannot hit.
 */
export function TabBar(props: { current: ScreenId; onSelect: (id: ScreenId) => void }) {
  return (
    <nav className="tabbar" aria-label="Chuyển màn hình">
      {SCREENS.map((screen) => (
        <button
          key={screen.id}
          type="button"
          className="tabbar__item"
          aria-current={screen.id === props.current ? "page" : undefined}
          onClick={() => props.onSelect(screen.id)}
        >
          {screen.tabLabel}
        </button>
      ))}
    </nav>
  );
}
