"use client";

import {
  Globe,
  Landmark,
  Pencil,
  Plus,
  Power,
  QrCode as QrCodeIcon,
  ScrollText,
  Smartphone,
  Star,
  TriangleAlert,
} from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, TextAreaField, TextField } from "@/components/form-parts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { PageHeader } from "@/components/ui/page-header";
import { SkeletonRows } from "@/components/ui/skeleton";
import { usePermissionKeys } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  addDomain,
  ApiError,
  correctName,
  getCommune,
  setActivation,
  setPrimaryDomain,
  type CommuneDetail,
} from "@/lib/api";
import { communeError, type CommuneField } from "@/lib/errors";
import { OperatorLog } from "@/features/operator-log/operator-log";
import { canIssueQr, canManageCommune, canManageDomains, canManageMiniApps } from "@/lib/permissions";

import { StatusBadge } from "./commune-parts";
import { LaunchLinkCard } from "./launch-link-card";
import { MiniAppSection } from "./mini-app-section";

/**
 * `/xa/[id]` — one commune's registry record and the wave-1 writes on it (ADR 0048 §01/10 #4, #5).
 *
 * WHAT IS DELIBERATELY ABSENT: removing a domain, or pointing a domain at another commune. Neither
 * exists by decision (§01/10 #5): repointing is the merger path and goes with merger
 * (`skills/admin-unit-merge`), a separate batch. Renaming an administrative unit is not here
 * either — the name dialog corrects a TYPING MISTAKE, and its wording says so.
 *
 * Every control is shown by the operator's `ops.*` keys (`lib/permissions.ts`) — a hint; the
 * server checks each call. Every write answers the commune as the registry now holds it, and that
 * answer replaces what is on screen: the page never patches its own copy.
 *
 * The page itself is a READ open to any `ops.*` key (ADR 0073 #1); so is its operator log (#2). The
 * QR card needs `ops.qr.issue`.
 *
 * LAYOUT (ADR 0068 look, presentation only): a page header with the commune's name, then one card
 * per concern — name and status, domains, Mini Apps, QR, operator log. The commune's name is printed exactly
 * as the registry holds it, never rebuilt (ADR 0068 §7).
 */

type Props = { communeId: string };

export const DEACTIVATE_WARNING =
  "Sau khi ngừng hoạt động, các trang của xã trên mọi tên miền của xã sẽ ngừng hoạt động trong khoảng 30 giây. Dữ liệu của xã được giữ nguyên; có thể bật hoạt động trở lại.";

export const NAME_CORRECTION_NOTICE =
  "Chỉ dùng để sửa lỗi gõ khi nhập tên (sai chính tả, thiếu dấu, thừa ký tự). Đây không phải đổi tên đơn vị hành chính: đổi tên, sáp nhập hay chia tách xã không làm ở màn này. Lý do và tên trước, sau khi sửa được ghi vào nhật ký.";

function fieldError(err: ApiError): { field: CommuneField; text: string } | null {
  return communeError(err);
}

export function CommuneDetailScreen({ communeId }: Props) {
  const guarded = useGuardedError();
  const keys = usePermissionKeys();
  const [commune, setCommune] = useState<CommuneDetail | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    getCommune(communeId).then(
      (c) => {
        if (alive) setCommune(c);
      },
      (err: unknown) => {
        if (!alive) return;
        setLoadError(guarded(err, "xem thông tin xã", (e) => fieldError(e)?.text ?? null));
      },
    );
    return () => {
      alive = false;
    };
  }, [communeId, guarded]);

  if (commune === null) {
    return (
      <>
        <PageHeader icon={Landmark} title="Thông tin xã" />
        {loadError === null ? (
          <Card>
            <p role="status" className="sr-only">
              Đang tải thông tin xã…
            </p>
            <SkeletonRows rows={4} columns={2} />
          </Card>
        ) : (
          // The server's sentence verbatim (an unknown id reads "not found"); no reload mechanism.
          <Card as="section">
            <ErrorState role="alert" title="Chưa tải được thông tin xã" message={loadError} />
          </Card>
        )}
      </>
    );
  }

  return (
    <CommuneDetailBody
      commune={commune}
      permissionKeys={keys}
      onChanged={setCommune}
      onMiniAppAttached={(app) => setCommune((c) => (c ? { ...c, mini_apps: [...c.mini_apps, app] } : c))}
    />
  );
}

function SectionCard({
  id,
  title,
  icon: Icon,
  children,
}: {
  id: string;
  title: string;
  icon: typeof Globe;
  children: ReactNode;
}) {
  return (
    <Card as="section" aria-labelledby={id}>
      <CardHeader>
        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
        <CardTitle id={id}>{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">{children}</CardContent>
    </Card>
  );
}

/** The record and its controls. No loading here, so the gated views are rendered in tests. */
export function CommuneDetailBody({
  commune,
  permissionKeys,
  onChanged,
  onMiniAppAttached,
}: {
  commune: CommuneDetail;
  permissionKeys: readonly string[];
  onChanged: (c: CommuneDetail) => void;
  onMiniAppAttached: (app: CommuneDetail["mini_apps"][number]) => void;
}) {
  const domainsAllowed = canManageDomains(permissionKeys);
  const communeAllowed = canManageCommune(permissionKeys);
  const miniAppsAllowed = canManageMiniApps(permissionKeys);
  const qrAllowed = canIssueQr(permissionKeys);

  return (
    <div className="flex max-w-[1120px] min-w-0 flex-col gap-4">
      <PageHeader icon={Landmark} title={commune.name} className="mb-1" />

      <SectionCard id="commune-admin" title="Tên và trạng thái" icon={Landmark}>
        <dl className="m-0 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-xs font-medium text-ink-500">Tỉnh, thành phố</dt>
            <dd className="m-0 text-sm font-semibold text-ink-900">{commune.province}</dd>
          </div>
          <div className="flex min-w-0 flex-col items-start gap-1">
            <dt className="text-xs font-medium text-ink-500">Trạng thái</dt>
            <dd className="m-0">
              <StatusBadge active={commune.active} />
            </dd>
          </div>
        </dl>
        {communeAllowed ? (
          <div className="flex flex-wrap gap-2 border-t border-line pt-4">
            <NameCorrection commune={commune} onChanged={onChanged} />
            <ActivationToggle commune={commune} onChanged={onChanged} />
          </div>
        ) : null}
      </SectionCard>

      <SectionCard id="commune-domains" title="Tên miền" icon={Globe}>
        <DomainList commune={commune} allowed={domainsAllowed} onChanged={onChanged} />
        {domainsAllowed ? <AddDomainForm commune={commune} onChanged={onChanged} /> : null}
      </SectionCard>

      <SectionCard id="commune-mini-apps" title="Mini App riêng của xã" icon={Smartphone}>
        <MiniAppSection
          commune={commune}
          allowed={miniAppsAllowed}
          onChanged={onChanged}
          onMiniAppAttached={onMiniAppAttached}
        />
      </SectionCard>

      {/* ops.qr.issue guards the link itself (a read with its own key), so without it the card is
          absent rather than a card that can only ever say "forbidden". */}
      {qrAllowed ? (
        <SectionCard id="commune-launch-link" title="Mã QR mở Mini App" icon={QrCodeIcon}>
          <LaunchLinkCard communeId={commune.id} />
        </SectionCard>
      ) : null}

      <SectionCard id="commune-operator-log" title="Nhật ký vận hành của xã" icon={ScrollText}>
        <OperatorLog communeId={commune.id} />
      </SectionCard>
    </div>
  );
}

// --- domains ---------------------------------------------------------------------------------

function DomainList({
  commune,
  allowed,
  onChanged,
}: {
  commune: CommuneDetail;
  allowed: boolean;
  onChanged: (c: CommuneDetail) => void;
}) {
  const guarded = useGuardedError();
  const [target, setTarget] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function confirm() {
    if (target === null || busy) return;
    setBusy(true);
    setError(null);
    try {
      onChanged(await setPrimaryDomain(commune.id, target));
      setTarget(null);
    } catch (err) {
      setError(guarded(err, "đặt tên miền chính", (e) => fieldError(e)?.text ?? null));
    } finally {
      setBusy(false);
    }
  }

  if (commune.domains.length === 0) {
    return <EmptyState icon={Globe} tone="neutral" title="Xã chưa có tên miền nào." className="py-6" />;
  }

  return (
    <>
      <ul className="m-0 flex list-none flex-col divide-y divide-line rounded-xl border border-line p-0">
        {commune.domains.map((d, i) => (
          <li key={d} className="flex min-h-12 flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2">
            <code className="min-w-0 font-mono text-[13px] break-all text-ink-900">{d}</code>
            {i === 0 ? (
              <Badge tone="info" icon={Star}>
                Tên miền chính
              </Badge>
            ) : null}
            {i > 0 && allowed ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="ml-auto"
                icon={<Star aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={() => {
                  setError(null);
                  setTarget(d);
                }}
              >
                Đặt làm tên miền chính
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
      <Dialog open={target !== null} title="Đặt tên miền chính" icon={Star} onClose={() => !busy && setTarget(null)}>
        <p>
          Đặt <code className="font-mono text-[13px] break-all text-ink-900">{target}</code> làm tên miền chính của{" "}
          {commune.name}? Các tên miền khác của xã vẫn được giữ.
        </p>
        <FormMessage text={error} />
        <DialogActions>
          <Button type="button" variant="primary" onClick={confirm} disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Đặt làm tên miền chính"}
          </Button>
          <Button type="button" variant="secondary" onClick={() => setTarget(null)} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

function AddDomainForm({ commune, onChanged }: { commune: CommuneDetail; onChanged: (c: CommuneDetail) => void }) {
  const guarded = useGuardedError();
  const [domain, setDomain] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: CommuneField; text: string } | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (domain.trim() === "") {
      setError({ field: "domain", text: "Hãy nhập tên miền cần thêm." });
      return;
    }
    setBusy(true);
    setError(null);
    try {
      onChanged(await addDomain(commune.id, domain.trim()));
      setDomain("");
    } catch (err) {
      let field: CommuneField = "form";
      const text = guarded(err, "thêm tên miền", (e) => {
        const known = fieldError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="flex max-w-xl flex-col gap-3 border-t border-line pt-4" method="post" onSubmit={submit} noValidate>
      <TextField
        label="Thêm tên miền"
        hint="Tên miền trần, không kèm https:// hay đường dẫn. Tên miền đã thêm không gỡ được ở màn này."
        name="domain"
        type="text"
        inputMode="url"
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        value={domain}
        onChange={(e) => setDomain(e.target.value)}
        disabled={busy}
        error={error?.field === "domain" ? error.text : null}
      />
      <FormMessage text={error && error.field !== "domain" ? error.text : null} />
      <div>
        <Button
          type="submit"
          variant="secondary"
          disabled={busy}
          aria-busy={busy}
          icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          {busy ? "Đang thêm…" : "Thêm tên miền"}
        </Button>
      </div>
    </form>
  );
}

// --- name correction and activation -------------------------------------------------------------

function NameCorrection({ commune, onChanged }: { commune: CommuneDetail; onChanged: (c: CommuneDetail) => void }) {
  const guarded = useGuardedError();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(commune.name);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: CommuneField; text: string } | null>(null);

  function openDialog() {
    setName(commune.name);
    setReason("");
    setError(null);
    setOpen(true);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (name.trim() === "") return setError({ field: "name", text: "Hãy nhập tên xã đã sửa." });
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do sửa." });
    setBusy(true);
    setError(null);
    try {
      onChanged(await correctName(commune.id, { name, reason }));
      setOpen(false);
    } catch (err) {
      let field: CommuneField = "form";
      const text = guarded(err, "sửa tên xã", (e) => {
        const known = fieldError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button
        type="button"
        variant="secondary"
        onClick={openDialog}
        icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      >
        Sửa lỗi gõ trong tên xã
      </Button>
      <Dialog open={open} title="Sửa lỗi gõ trong tên xã" icon={Pencil} onClose={() => !busy && setOpen(false)}>
        <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
          <Notice tone="info">{NAME_CORRECTION_NOTICE}</Notice>
          <TextField
            label="Tên xã sau khi sửa"
            name="name"
            type="text"
            autoComplete="off"
            maxLength={200}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={busy}
            error={error?.field === "name" ? error.text : null}
          />
          <TextAreaField
            label="Lý do sửa"
            hint="Ví dụ: nhập thiếu dấu khi tạo xã. Tối đa 500 ký tự."
            name="reason"
            rows={3}
            maxLength={500}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={busy}
            error={error?.field === "reason" ? error.text : null}
          />
          <FormMessage text={error && error.field !== "name" && error.field !== "reason" ? error.text : null} />
          <DialogActions>
            <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
              {busy ? "Đang lưu…" : "Lưu tên đã sửa"}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setOpen(false)} disabled={busy}>
              Huỷ
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </>
  );
}

function ActivationToggle({ commune, onChanged }: { commune: CommuneDetail; onChanged: (c: CommuneDetail) => void }) {
  const guarded = useGuardedError();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: CommuneField; text: string } | null>(null);
  const deactivating = commune.active;
  const label = deactivating ? "Ngừng hoạt động xã" : "Bật hoạt động trở lại";
  // The dialog asks the SPECIFIC question (web-admin confirm box); the confirm button keeps the
  // action's own name. The commune's name is inserted verbatim, never rebuilt.
  const question = deactivating ? `Ngừng hoạt động ${commune.name}?` : `Bật hoạt động trở lại cho ${commune.name}?`;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      onChanged(await setActivation(commune.id, { active: !deactivating, reason }));
      setOpen(false);
    } catch (err) {
      let field: CommuneField = "form";
      const text = guarded(err, deactivating ? "ngừng hoạt động xã" : "bật hoạt động xã", (e) => {
        const known = fieldError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button
        type="button"
        variant={deactivating ? "danger" : "secondary"}
        icon={<Power aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        onClick={() => {
          setReason("");
          setError(null);
          setOpen(true);
        }}
      >
        {label}
      </Button>
      <Dialog
        open={open}
        title={question}
        tone={deactivating ? "danger" : "default"}
        icon={deactivating ? TriangleAlert : Power}
        onClose={() => !busy && setOpen(false)}
      >
        <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
          {deactivating ? (
            <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
              {DEACTIVATE_WARNING}
            </Notice>
          ) : (
            <p>Các trang của xã trên các tên miền của xã sẽ hoạt động trở lại.</p>
          )}
          <TextAreaField
            label="Lý do"
            hint="Bắt buộc, tối đa 500 ký tự. Lý do được ghi vào nhật ký."
            name="reason"
            rows={3}
            maxLength={500}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={busy}
            error={error?.field === "reason" ? error.text : null}
          />
          <FormMessage text={error && error.field !== "reason" ? error.text : null} />
          <DialogActions>
            <Button type="submit" variant={deactivating ? "danger-solid" : "primary"} disabled={busy} aria-busy={busy}>
              {busy ? "Đang lưu…" : label}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setOpen(false)} disabled={busy}>
              Huỷ
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </>
  );
}
