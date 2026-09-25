/**
 * Liveness/readiness for the k8s probe — answers without any commune.
 *
 * The probe connects with the POD IP as Host, which is no commune. Every page resolves the
 * commune from Host and answers 404 for that (rule 1, invariant 3 — correctly), so probing a
 * page would mark every healthy pod as failed. This route makes NO upstream call and reads no
 * Host on purpose: "is this process serving" must not depend on identity being up, or one slow
 * identity pod restarts every web-admin pod with it.
 */
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export function GET(): Response {
  return new Response("ok", {
    status: 200,
    headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" },
  });
}
