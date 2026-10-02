"use client";

import { ChevronDown, Plus, ShieldX } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useId, useState, type FormEvent } from "react";

import { controlClass, FieldError, FormMessage, labelClass, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { Notice } from "@/components/ui/notice";
import { useOperator } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { createCommune, listProvinces, type Province } from "@/lib/api";
import { cn } from "@/lib/cn";
import { communeError, type CommuneField } from "@/lib/errors";
import { canCreateCommune } from "@/lib/permissions";

/**
 * `/xa/moi` — create a commune: name, province from the `tinh_thanh` catalogue, primary domain
 * (ADR 0048 §01/10 #4). Nothing else is seeded — §01/10 #1 — and the form says so, because the
 * operator is the one who will be asked "why is the new commune empty".
 */

export const CREATE_ACTION = "tạo xã";

export const EMPTY_COMMUNE_NOTICE =
  "Xã mới được tạo ở trạng thái trống: chưa có vai trò, cán bộ, phòng ban, thời hạn xử lý, giờ làm việc hay ngày nghỉ lễ. Đây là nghiệp vụ của xã: quản trị viên của xã tự khai báo sau khi đăng nhập. Khu vận hành không khai thay.";

/**
 * ADR 0046 §Quyết định 2. The password is NOT named here, nor where it is kept: this screen is
 * read by every operator, and the seed password opens `admin` at every commune not yet seeded.
 */
export const FIRST_ADMIN_NOTICE =
  "Người quản trị đầu tiên của xã đăng nhập tại tên miền của xã: gõ “admin” vào ô thư điện tử và dùng mật khẩu khởi tạo đặt trong cấu hình bí mật của nền tảng (bộ phận kỹ thuật quản lý; màn này không hiển thị). Cách này chỉ dùng được khi nền tảng đang bật mật khẩu khởi tạo, và chỉ ở lần đăng nhập đầu tiên khi xã chưa từng có tài khoản “admin”; sau đó hệ thống bắt đổi mật khẩu ngay.";

export const CREATE_FORBIDDEN_NOTICE =
  "Tài khoản vận hành của bạn không có quyền tạo xã (cần cả quyền quản lý xã và quản lý tên miền). Nếu cần, đề nghị người quản lý tài khoản vận hành cấp quyền.";

export function CreateCommuneNotices() {
  return (
    <section aria-label="Lưu ý trước khi tạo xã">
      <Notice tone="info">
        <p>{EMPTY_COMMUNE_NOTICE}</p>
        <p>{FIRST_ADMIN_NOTICE}</p>
      </Notice>
    </section>
  );
}

/** The permission HINT: once the keys are known and lack one, say so instead of a form that 403s. */
export function CreateCommuneGate({ permissionKeys, ready }: { permissionKeys: readonly string[]; ready: boolean }) {
  if (ready && !canCreateCommune(permissionKeys)) {
    return (
      <Notice tone="neutral" icon={ShieldX} role="status">
        {CREATE_FORBIDDEN_NOTICE}
      </Notice>
    );
  }
  return <CreateCommuneForm />;
}

export function CreateCommunePage() {
  const operator = useOperator();
  return (
    <CreateCommuneGate
      ready={operator.status === "ready"}
      permissionKeys={operator.status === "ready" ? operator.operator.permission_keys : []}
    />
  );
}

type Errors = Partial<Record<CommuneField, string>>;

function CreateCommuneForm() {
  const router = useRouter();
  const guarded = useGuardedError();
  const provinceId = useId();
  const [provinces, setProvinces] = useState<Province[] | null>(null);
  const [provinceLoadError, setProvinceLoadError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [province, setProvince] = useState("");
  const [domain, setDomain] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<Errors>({});

  useEffect(() => {
    let alive = true;
    listProvinces().then(
      (items) => {
        if (alive) setProvinces(items);
      },
      (err: unknown) => {
        if (alive) setProvinceLoadError(guarded(err, "xem danh mục tỉnh, thành phố"));
      },
    );
    return () => {
      alive = false;
    };
  }, [guarded]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    // Checked here only so the person is told before a round trip; the server re-checks all three.
    const local: Errors = {};
    if (name.trim() === "") local.name = "Hãy nhập tên xã.";
    if (province === "") local.province = "Hãy chọn tỉnh, thành phố.";
    if (domain.trim() === "") local.domain = "Hãy nhập tên miền chính của xã.";
    setErrors(local);
    if (Object.keys(local).length > 0) return;

    setSubmitting(true);
    try {
      const created = await createCommune({ name, provinceId: province, primaryDomain: domain.trim() });
      router.push(`/xa/${encodeURIComponent(created.id)}`);
    } catch (err) {
      let field: CommuneField = "form";
      const text = guarded(err, CREATE_ACTION, (e) => {
        const known = communeError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setErrors({ [field]: text });
      setSubmitting(false);
    }
  }

  const provinceErrorId = provinceId + "-error";

  // One column at 320px, two from 640px (name across both). Labels above every control.
  return (
    <Card as="form" method="post" onSubmit={submit} noValidate>
      <CardContent className="flex flex-col gap-5">
        <CreateCommuneNotices />
        <div className="grid grid-cols-1 gap-x-4 gap-y-5 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <TextField
              label="Tên xã"
              hint="Ghi đủ loại đơn vị và tên (“Xã …”, “Phường …”, “Đặc khu …”), đúng như văn bản thành lập."
              name="name"
              type="text"
              autoComplete="off"
              maxLength={200}
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={submitting}
              error={errors.name}
            />
          </div>
          <div className="flex min-w-0 flex-col gap-1.5">
            <label htmlFor={provinceId} className={labelClass}>
              Tỉnh, thành phố
            </label>
            <div className="relative">
              <select
                id={provinceId}
                name="province_id"
                value={province}
                onChange={(e) => setProvince(e.target.value)}
                disabled={submitting || provinces === null}
                aria-invalid={errors.province ? true : undefined}
                aria-describedby={errors.province ? provinceErrorId : undefined}
                className={cn(controlClass, "cursor-pointer appearance-none pr-9")}
              >
                <option value="">{provinces === null ? "Đang tải danh mục…" : "Chọn tỉnh, thành phố"}</option>
                {(provinces ?? []).map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              <ChevronDown
                aria-hidden="true"
                focusable="false"
                strokeWidth={1.8}
                className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-ink-500"
              />
            </div>
            {errors.province ? <FieldError id={provinceErrorId} text={errors.province} /> : null}
          </div>
          <TextField
            label="Tên miền chính"
            hint="Tên miền trần, không kèm https:// hay đường dẫn. Cán bộ xã mở trang quản trị của xã tại tên miền này."
            name="primary_domain"
            type="text"
            inputMode="url"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            disabled={submitting}
            error={errors.domain}
          />
        </div>
        <FormMessage text={errors.form ?? provinceLoadError} />
      </CardContent>
      <CardFooter className="justify-end">
        <Button
          type="submit"
          variant="primary"
          disabled={submitting}
          aria-busy={submitting}
          icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          {submitting ? "Đang tạo xã…" : "Tạo xã"}
        </Button>
      </CardFooter>
    </Card>
  );
}
