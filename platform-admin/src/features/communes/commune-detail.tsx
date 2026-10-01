"use client";

import { useEffect, useState, type FormEvent } from "react";

import { Dialog } from "@/components/dialog";
import { FormMessage, TextAreaField, TextField } from "@/components/form-parts";
import { usePermissionKeys } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  addDomain,
  ApiError,
  attachMiniApp,
  correctName,
  getCommune,
  setActivation,
  setPrimaryDomain,
  type CommuneDetail,
} from "@/lib/api";
import { communeError, type CommuneField } from "@/lib/errors";
import { canManageCommune, canManageDomains, canManageMiniApps } from "@/lib/permissions";

import { formatDateTime, miniAppModeLabel, StatusBadge } from "./commune-parts";

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
        <h1 className="page-title">Thông tin xã</h1>
        {loadError === null ? (
          <p role="status" className="loading-line">
            Đang tải thông tin xã…
          </p>
        ) : null}
        <FormMessage text={loadError} />
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

  return (
    <>
      <h1 className="page-title">{commune.name}</h1>
      <dl className="facts">
        <div>
          <dt>Tỉnh, thành phố</dt>
          <dd>{commune.province}</dd>
        </div>
        <div>
          <dt>Trạng thái</dt>
          <dd>
            <StatusBadge active={commune.active} />
          </dd>
        </div>
      </dl>

      {communeAllowed ? (
        <section className="panel" aria-labelledby="commune-admin">
          <h2 id="commune-admin" className="section-title">
            Tên và trạng thái
          </h2>
          <div className="button-row">
            <NameCorrection commune={commune} onChanged={onChanged} />
            <ActivationToggle commune={commune} onChanged={onChanged} />
          </div>
        </section>
      ) : null}

      <section className="panel" aria-labelledby="commune-domains">
        <h2 id="commune-domains" className="section-title">
          Tên miền
        </h2>
        <DomainList commune={commune} allowed={domainsAllowed} onChanged={onChanged} />
        {domainsAllowed ? <AddDomainForm commune={commune} onChanged={onChanged} /> : null}
      </section>

      <section className="panel" aria-labelledby="commune-mini-apps">
        <h2 id="commune-mini-apps" className="section-title">
          Mini App riêng của xã
        </h2>
        <MiniAppTable apps={commune.mini_apps} />
        {miniAppsAllowed ? <AttachMiniAppForm commune={commune} onAttached={onMiniAppAttached} /> : null}
      </section>
    </>
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

  if (commune.domains.length === 0) return <p>Xã chưa có tên miền nào.</p>;

  return (
    <>
      <ul className="domain-list">
        {commune.domains.map((d, i) => (
          <li key={d}>
            <code>{d}</code>
            {i === 0 ? <span className="tag">Tên miền chính</span> : null}
            {i > 0 && allowed ? (
              <button
                type="button"
                className="link-button"
                onClick={() => {
                  setError(null);
                  setTarget(d);
                }}
              >
                Đặt làm tên miền chính
              </button>
            ) : null}
          </li>
        ))}
      </ul>
      <Dialog open={target !== null} title="Đặt tên miền chính" onClose={() => !busy && setTarget(null)}>
        <p>
          Đặt <code>{target}</code> làm tên miền chính của {commune.name}? Các tên miền khác của xã vẫn được giữ.
        </p>
        <FormMessage text={error} />
        <div className="button-row">
          <button type="button" className="primary-button" onClick={confirm} disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Đặt làm tên miền chính"}
          </button>
          <button type="button" className="secondary-button" onClick={() => setTarget(null)} disabled={busy}>
            Huỷ
          </button>
        </div>
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
    <form className="inline-form" method="post" onSubmit={submit} noValidate>
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
      <button type="submit" className="secondary-button" disabled={busy} aria-busy={busy}>
        {busy ? "Đang thêm…" : "Thêm tên miền"}
      </button>
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
      <button type="button" className="secondary-button" onClick={openDialog}>
        Sửa lỗi gõ trong tên xã
      </button>
      <Dialog open={open} title="Sửa lỗi gõ trong tên xã" onClose={() => !busy && setOpen(false)}>
        <form method="post" onSubmit={submit} noValidate>
          <p className="notice-box">{NAME_CORRECTION_NOTICE}</p>
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
          <div className="button-row">
            <button type="submit" className="primary-button" disabled={busy} aria-busy={busy}>
              {busy ? "Đang lưu…" : "Lưu tên đã sửa"}
            </button>
            <button type="button" className="secondary-button" onClick={() => setOpen(false)} disabled={busy}>
              Huỷ
            </button>
          </div>
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
      <button
        type="button"
        className={deactivating ? "danger-button" : "secondary-button"}
        onClick={() => {
          setReason("");
          setError(null);
          setOpen(true);
        }}
      >
        {label}
      </button>
      <Dialog open={open} title={label} onClose={() => !busy && setOpen(false)}>
        <form method="post" onSubmit={submit} noValidate>
          {deactivating ? (
            <p className="warning-box">
              <strong>Lưu ý: </strong>
              {DEACTIVATE_WARNING}
            </p>
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
          <div className="button-row">
            <button
              type="submit"
              className={deactivating ? "danger-button" : "primary-button"}
              disabled={busy}
              aria-busy={busy}
            >
              {busy ? "Đang lưu…" : label}
            </button>
            <button type="button" className="secondary-button" onClick={() => setOpen(false)} disabled={busy}>
              Huỷ
            </button>
          </div>
        </form>
      </Dialog>
    </>
  );
}

// --- Mini Apps -------------------------------------------------------------------------------

export function MiniAppTable({ apps }: { apps: CommuneDetail["mini_apps"] }) {
  if (apps.length === 0) return <p>Xã chưa gắn Mini App riêng nào.</p>;
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            <th scope="col">App ID</th>
            <th scope="col">Chế độ</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Gắn lúc</th>
            <th scope="col">Người gắn</th>
          </tr>
        </thead>
        <tbody>
          {apps.map((a) => (
            <tr key={a.app_id}>
              <td>
                <code>{a.app_id}</code>
              </td>
              <td>{miniAppModeLabel(a.mode)}</td>
              <td>
                <StatusBadge active={a.active} />
              </td>
              <td>{formatDateTime(a.created_at)}</td>
              <td>{a.created_by}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AttachMiniAppForm({
  commune,
  onAttached,
}: {
  commune: CommuneDetail;
  onAttached: (app: CommuneDetail["mini_apps"][number]) => void;
}) {
  const guarded = useGuardedError();
  const [appId, setAppId] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: CommuneField; text: string } | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const id = appId.trim();
    if (!/^\d+$/.test(id)) {
      setError({ field: "appId", text: "App ID chỉ gồm chữ số, tối đa 32 chữ số." });
      return;
    }
    setBusy(true);
    setError(null);
    try {
      onAttached(await attachMiniApp(commune.id, { appId: id, note: note.trim() === "" ? undefined : note }));
      setAppId("");
      setNote("");
    } catch (err) {
      let field: CommuneField = "form";
      const text = guarded(err, "gắn Mini App", (e) => {
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
    <form className="inline-form" method="post" onSubmit={submit} noValidate>
      <TextField
        label="App ID của Mini App riêng"
        hint="Dãy chữ số Zalo cấp cho Mini App của xã."
        name="app_id"
        type="text"
        inputMode="numeric"
        autoComplete="off"
        maxLength={32}
        value={appId}
        onChange={(e) => setAppId(e.target.value)}
        disabled={busy}
        error={error?.field === "appId" ? error.text : null}
      />
      <TextAreaField
        label="Ghi chú (không bắt buộc)"
        name="note"
        rows={2}
        maxLength={500}
        value={note}
        onChange={(e) => setNote(e.target.value)}
        disabled={busy}
        error={error?.field === "note" ? error.text : null}
      />
      <FormMessage text={error && error.field !== "appId" && error.field !== "note" ? error.text : null} />
      <button type="submit" className="secondary-button" disabled={busy} aria-busy={busy}>
        {busy ? "Đang gắn…" : "Gắn Mini App"}
      </button>
    </form>
  );
}
