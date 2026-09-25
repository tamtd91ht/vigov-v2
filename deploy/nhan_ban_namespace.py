#!/usr/bin/env python3
"""Clean a `kubectl get -o json` List so it can be CREATED in another namespace.

Used ONCE by `deploy/Jenkinsfile` (HANH_DONG=nhan-ban-staging), chosen by the owner on
2026-09-25: prod is a copy of the staging workloads that were built by hand in Rancher, not of
the manifests in `deploy/base`.

    kubectl -n vigov-staging get deploy,svc -o json \
      | python3 deploy/nhan_ban_namespace.py vigov-staging vigov-prod <names already in prod> \
      | kubectl -n vigov-prod create -f -

WHAT IT DOES, and why each line exists:
  - drops server-owned fields (uid, resourceVersion, status, clusterIP…) — `create` refuses them
  - replaces EVERY occurrence of the source namespace name with the target one. That is the
    point, not a side effect: staging env values such as `…vigov-staging.svc:9090` must reach
    prod's own services, never staging's. A prod pod dialling staging is a cross-environment
    leak nobody would see in a log.
  - skips objects whose `Kind/name` already exists in the target: create-only, never overwrite.
  - keeps only workloads whose name starts with `vigov-` — `vihat-zalo-miniapp` belongs to
    another repository.

It never reads Secrets: they are copied by name elsewhere, and a Secret's data must not pass
through a script that prints anything.
"""
import json
import sys

BO_METADATA = ("uid", "resourceVersion", "creationTimestamp", "managedFields", "generation",
               "selfLink", "ownerReferences", "namespace")
BO_ANNOTATION = ("deployment.kubernetes.io/revision",
                 "kubectl.kubernetes.io/last-applied-configuration")


def lam_sach(obj: dict) -> dict:
    obj.pop("status", None)
    md = obj.get("metadata", {})
    for k in BO_METADATA:
        md.pop(k, None)
    ann = md.get("annotations") or {}
    for k in BO_ANNOTATION:
        ann.pop(k, None)
    if not ann:
        md.pop("annotations", None)
    if obj.get("kind") == "Service":
        spec = obj.get("spec", {})
        for k in ("clusterIP", "clusterIPs", "healthCheckNodePort"):
            spec.pop(k, None)
    return obj


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: nhan_ban_namespace.py <from-ns> <to-ns> [Kind/name ...]", file=sys.stderr)
        return 2
    nguon, dich = sys.argv[1], sys.argv[2]
    da_co = set(sys.argv[3:])
    danh_sach = json.load(sys.stdin)
    ra = []
    for obj in danh_sach.get("items", []):
        ten = obj.get("metadata", {}).get("name", "")
        khoa = f'{obj.get("kind")}/{ten}'
        if not ten.startswith("vigov-"):
            print(f"   bỏ qua {khoa} — không phải workload của ViGov", file=sys.stderr)
            continue
        if khoa in da_co:
            print(f"   bỏ qua {khoa} — {dich} đã có, không ghi đè", file=sys.stderr)
            continue
        s = json.dumps(lam_sach(obj), ensure_ascii=False).replace(nguon, dich)
        ra.append(json.loads(s))
        print(f"   sẽ tạo {khoa}", file=sys.stderr)
    json.dump({"apiVersion": "v1", "kind": "List", "items": ra}, sys.stdout, ensure_ascii=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
