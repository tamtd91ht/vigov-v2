import { Archive, ArrowUpRight, CircleCheck, CircleMinus, Inbox, Loader, UserCheck, type LucideIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";

import type { LetterScope } from "@/lib/api/citizen-letters";

import { letterGroupChip, letterGroupLabel } from "./letter-display";

/**
 * Small presentation pieces of the citizen-letter tabs. Hook-free, so a test can render them with
 * `react-dom/server`.
 */

/** ICON + WORD per display group, never colour alone (`Badge`). An unknown code gets the neutral icon. */
const GROUP_ICON: Readonly<Record<string, LucideIcon>> = {
  "moi-vao-so": Inbox,
  "da-phan-cong": UserCheck,
  "dang-xu-ly": Loader,
  "da-giai-quyet": CircleCheck,
  "chuyen-cap-tren": ArrowUpRight,
  "luu-khong-thu-ly": Archive,
};

/**
 * The prototype's status `Badge` (`PetitionTable.tsx:113-115`) — the letter's DISPLAY GROUP, the
 * server's `status_group` (ADR 0084 #2), with the prototype's chip colours.
 */
export function LetterStatusBadge({ group }: { group: string }) {
  return (
    <Badge icon={GROUP_ICON[group] ?? CircleMinus} className={letterGroupChip(group)}>
      {letterGroupLabel(group)}
    </Badge>
  );
}

/** The three choices of the prototype's `ScopeFilter` (`components/common/ScopeFilter.tsx:8-24`). */
const SCOPE_CHOICES: readonly { value: LetterScope; label: string; hint: string }[] = [
  { value: "all", label: "Toàn xã", hint: "Tất cả hồ sơ trong xã" },
  { value: "mine", label: "Giao cho tôi", hint: "Đích danh tôi là người xử lý" },
  { value: "related", label: "Liên quan đến tôi", hint: "Tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ" },
];

/**
 * The prototype's `ScopeFilter`: three joined 36px buttons, 12.5px semibold, the pressed one navy.
 * `mine` / `related` are answered by the server from the SESSION's staff code — nothing here names
 * the person (rule 4, invariant 2 in staff form).
 */
export function LetterScopeFilter({ value, onChange }: { value: LetterScope; onChange: (scope: LetterScope) => void }) {
  return (
    <span
      role="group"
      aria-label="Lọc nhanh theo người xử lý"
      className="inline-flex overflow-hidden rounded-md border border-solid border-line bg-white"
    >
      {SCOPE_CHOICES.map((choice) => {
        const active = choice.value === value;
        return (
          <button
            key={choice.value}
            type="button"
            title={choice.hint}
            aria-pressed={active}
            onClick={() => onChange(choice.value)}
            className={cn(
              "h-9 cursor-pointer border-0 border-r border-solid border-line px-3 [font-family:inherit] text-[12.5px] font-semibold transition-colors last:border-r-0",
              active ? "bg-navy text-white" : "bg-white text-ink hover:bg-canvas",
            )}
          >
            {choice.label}
          </button>
        );
      })}
    </span>
  );
}
