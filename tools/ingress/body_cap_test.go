package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// What these tests defend: the owner's body caps of 09/10/2026 (ADR 0052 §Sửa đổi 09/10/2026) —
// 55 MB on the web, comms, petitions and platform hosts, 25 MB everywhere else, a 180 s proxy
// read/send timeout on the upload hosts only — and that each
// overlay actually patches BOTH Ingress objects.

// Literal copies of the decision, deliberately not read from uploadHosts: a test that took its
// expectation from the list it checks would pass whatever the list said.
var pinnedUploadHosts = []string{webProd, "comms." + apiProd, "petitions." + apiProd, "platform." + apiProd}

const (
	annBodySize    = "nginx.ingress.kubernetes.io/proxy-body-size"
	annBuffering   = "nginx.ingress.kubernetes.io/proxy-request-buffering"
	annReadTimeout = "nginx.ingress.kubernetes.io/proxy-read-timeout"
	annSendTimeout = "nginx.ingress.kubernetes.io/proxy-send-timeout"
)

func TestBodyCapsMatchOwnerDecision(t *testing.T) {
	objs := readIngressObjects(t)
	up, rest := objs[0], objs[1]

	if got := up.Metadata.Annotations[annBodySize]; got != "55m" {
		t.Errorf("%s: %s = %q, owner decided 55m", up.Metadata.Name, annBodySize, got)
	}
	if got := up.Metadata.Annotations[annBuffering]; got != "off" {
		t.Errorf("%s: %s = %q, owner chose streaming (off)", up.Metadata.Name, annBuffering, got)
	}
	// The owner-approved 180 s upload bound: nginx's 60 s default would 504 a scan/re-encode that
	// finishes later, while the service still stores the file. Literal, not uploadProxyTimeout.
	for _, ann := range []string{annReadTimeout, annSendTimeout} {
		if got := up.Metadata.Annotations[ann]; got != "180" {
			t.Errorf("%s: %s = %q, owner approved 180", up.Metadata.Name, ann, got)
		}
		if _, set := rest.Metadata.Annotations[ann]; set {
			t.Errorf("%s sets %s — only the upload object carries the long bound", rest.Metadata.Name, ann)
		}
	}
	if got := rest.Metadata.Annotations[annBodySize]; got != "25m" {
		t.Errorf("%s: %s = %q, every non-upload host keeps 25m", rest.Metadata.Name, annBodySize, got)
	}
	if _, set := rest.Metadata.Annotations[annBuffering]; set {
		t.Errorf("%s sets %s — only the upload object streams", rest.Metadata.Name, annBuffering)
	}
	for _, o := range objs {
		if o.Metadata.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"] != "true" {
			t.Errorf("%s lost ssl-redirect", o.Metadata.Name)
		}
	}

	var upHosts []string
	for _, r := range up.Spec.Rules {
		upHosts = append(upHosts, r.Host)
	}
	want := append([]string(nil), pinnedUploadHosts...)
	sort.Strings(upHosts)
	sort.Strings(want)
	if strings.Join(upHosts, ",") != strings.Join(want, ",") {
		t.Errorf("%s hosts = %v, owner decided %v", up.Metadata.Name, upHosts, want)
	}
	if len(up.Spec.Rules) == 0 || up.Spec.Rules[0].Host != webProd {
		t.Errorf("%s rule 0 must be the web host %s — the overlay patches index 0 as web", up.Metadata.Name, webProd)
	}
	for _, r := range rest.Spec.Rules {
		for _, h := range pinnedUploadHosts {
			if r.Host == h {
				t.Errorf("%s carries upload host %s", rest.Metadata.Name, h)
			}
		}
	}
}

func TestIngressObjectsPartitionsHosts(t *testing.T) {
	luats := []luatIngress{
		{Duong: "/api/v1/content-items", DichVu: "comms", Cong: "rest"},
		{Duong: "/api/v1/citizen-reports", DichVu: "petitions", Cong: "rest"},
		{Duong: "/api/v1/mini-app-ids", DichVu: "platform", Cong: "rest", Phu: []string{"/api/v1/mini-app-ids"}},
		{Duong: "/api/v1/org-units", DichVu: "identity", Cong: "rest"},
		{Duong: "/", DichVu: dichVuWeb, Cong: "http", BatHet: true},
	}
	objs, err := ingressObjects(luats)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 || objs[0].Name != "vigov" || objs[1].Name != "vigov-no-upload" {
		t.Fatalf("objects = %+v", objs)
	}
	names := func(o ingressObject) string {
		var s []string
		for _, h := range o.Hosts {
			s = append(s, h.DichVu)
		}
		return strings.Join(s, ",")
	}
	if got := names(objs[0]); got != "comms,petitions,platform" || !objs[0].Web || !objs[0].Stream || objs[0].BodyCap != "55m" {
		t.Errorf("upload object = %s web=%v stream=%v cap=%s", got, objs[0].Web, objs[0].Stream, objs[0].BodyCap)
	}
	if got := names(objs[1]); got != "identity" || objs[1].Web || objs[1].Stream || objs[1].BodyCap != "25m" {
		t.Errorf("no-upload object = %s web=%v stream=%v cap=%s", got, objs[1].Web, objs[1].Stream, objs[1].BodyCap)
	}
}

// A name in uploadHosts with no public host is a stale list: the owner's cap would apply to
// nothing, silently. The generator stops.
func TestIngressObjectsRefusesStaleUploadList(t *testing.T) {
	_, err := ingressObjects([]luatIngress{
		{Duong: "/api/v1/citizen-reports", DichVu: "petitions", Cong: "rest"},
		{Duong: "/api/v1/mini-app-ids", DichVu: "platform", Cong: "rest", Phu: []string{"/api/v1/mini-app-ids"}},
		{Duong: "/api/v1/org-units", DichVu: "identity", Cong: "rest"},
	})
	if err == nil || !strings.Contains(err.Error(), "comms") {
		t.Fatalf("err = %v, want a refusal naming comms", err)
	}
}

// The overlays bind each patch to its object in kustomization.yaml — a hand-written file this
// generator does not own. A missing entry is not a build error: kustomize simply leaves that
// Ingress with base's PROD hosts, so staging would claim prod hostnames on the shared controller.
// Hence a check on the real files.
func TestOverlayKustomizationPatchesEveryIngress(t *testing.T) {
	root := goc(t)
	for mt := range moHinhChot {
		p := filepath.Join(root, "deploy", "overlays", mt, "kustomization.yaml")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var k struct {
			Patches []struct {
				Path   string `yaml:"path"`
				Target struct {
					Kind string `yaml:"kind"`
					Name string `yaml:"name"`
				} `yaml:"target"`
			} `yaml:"patches"`
		}
		if err := yaml.Unmarshal(raw, &k); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, name := range pinnedIngressNames {
			file := pinnedPatchFiles[name]
			found := false
			for _, e := range k.Patches {
				if e.Path == file && e.Target.Kind == "Ingress" && e.Target.Name == name {
					found = true
				}
			}
			if !found {
				t.Errorf("deploy/overlays/%s/kustomization.yaml has no patch `path: %s` with target Ingress %s — "+
					"add `- path: %s\\n    target: { group: networking.k8s.io, version: v1, kind: Ingress, name: %s }`",
					mt, file, name, file, name)
			}
		}
	}
}
