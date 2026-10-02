import { SearchX } from "lucide-react";

/** 404. Says nothing about communes or hosts: a probe gets the answer a typo gets. */
export default function NotFound() {
  return (
    <main className="flex min-h-dvh items-center justify-center px-4 py-10">
      <div className="flex max-w-md flex-col items-center gap-2 text-center">
        <span aria-hidden="true" className="mb-2 grid size-[72px] place-items-center rounded-full bg-brand-50 text-brand-600">
          <SearchX className="size-8" strokeWidth={1.6} focusable="false" />
        </span>
        <h1 className="m-0 text-2xl leading-tight font-bold text-ink-900">Không tìm thấy trang</h1>
        <p className="m-0 text-sm text-ink-500">Đường dẫn bạn truy cập không tồn tại.</p>
      </div>
    </main>
  );
}
