package kubernetes_test

import (
	"strings"
	"testing"

	kubefilter "github.com/devr-tools/szr/internal/filters/kubernetes"
)

func TestSummarizeKubectlApply(t *testing.T) {
	input := strings.Join([]string{
		"deployment.apps/api configured",
		"deployment.apps/worker configured",
		"service/api unchanged",
		"service/worker unchanged",
		"configmap/env unchanged",
		"secret/creds unchanged",
		"namespace/staging created",
		"Warning: resource configmaps/env is missing the kubectl.kubernetes.io/last-applied-configuration annotation",
	}, "\n")

	got := kubefilter.SummarizeKubectlApply(input, 8)
	for _, want := range []string{
		"resources: configured=2 unchanged=4 created=1",
		"configured: deployment.apps/api, deployment.apps/worker",
		"unchanged: service/api, service/worker, configmap/env +1 more",
		"created: namespace/staging",
		"Warning: resource configmaps/env",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl apply summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret/creds") {
		t.Fatalf("expected sampled unchanged names to omit secret/creds:\n%s", got)
	}
}

func TestSummarizeKubectlDelete(t *testing.T) {
	input := strings.Join([]string{
		`pod "api-1" deleted`,
		`pod "api-2" deleted`,
		`deployment.apps "api" deleted`,
		`Error from server (NotFound): pods "ghost" not found`,
	}, "\n")

	got := kubefilter.SummarizeKubectlApply(input, 6)
	for _, want := range []string{
		"resources: deleted=3",
		"deleted: pod/api-1, pod/api-2, deployment.apps/api",
		`Error from server (NotFound): pods "ghost" not found`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl delete summary:\n%s", want, got)
		}
	}
}

func TestSummarizeKubectlRolloutCollapse(t *testing.T) {
	input := strings.Join([]string{
		`Waiting for deployment "web" rollout to finish: 0 of 3 updated replicas are available...`,
		`Waiting for deployment "web" rollout to finish: 1 of 3 updated replicas are available...`,
		`Waiting for deployment "web" rollout to finish: 2 of 3 updated replicas are available...`,
		`deployment "web" successfully rolled out`,
	}, "\n")

	got := kubefilter.SummarizeKubectlRollout(input, 6)
	for _, want := range []string{
		"progress: collapsed 2 rollout updates",
		`Waiting for deployment "web" rollout to finish: 2 of 3 updated replicas are available...`,
		`deployment "web" successfully rolled out`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl rollout summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "0 of 3") {
		t.Fatalf("expected earlier rollout progress lines to be collapsed:\n%s", got)
	}
}

func TestSummarizeKubectlRolloutError(t *testing.T) {
	input := strings.Join([]string{
		`Waiting for deployment "web" rollout to finish: 1 of 3 updated replicas are available...`,
		`error: deployment "web" exceeded its progress deadline`,
	}, "\n")

	got := kubefilter.SummarizeKubectlRollout(input, 6)
	for _, want := range []string{
		`Waiting for deployment "web" rollout to finish: 1 of 3 updated replicas are available...`,
		`error: deployment "web" exceeded its progress deadline`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl rollout error summary:\n%s", want, got)
		}
	}
}

func TestSummarizeKubectlDiff(t *testing.T) {
	input := strings.Join([]string{
		"diff -u -N /tmp/LIVE-123/apps.v1.Deployment.default.web /tmp/MERGED-456/apps.v1.Deployment.default.web",
		"--- /tmp/LIVE-123/apps.v1.Deployment.default.web",
		"+++ /tmp/MERGED-456/apps.v1.Deployment.default.web",
		"@@ -5,7 +5,7 @@",
		"   labels:",
		"     app: web",
		"-  replicas: 2",
		"+  replicas: 3",
		`+  revision: "4"`,
		"diff -u -N /tmp/LIVE-123/v1.ConfigMap.default.cfg /tmp/MERGED-456/v1.ConfigMap.default.cfg",
		"--- /tmp/LIVE-123/v1.ConfigMap.default.cfg",
		"+++ /tmp/MERGED-456/v1.ConfigMap.default.cfg",
		"@@ -2,3 +2,3 @@",
		"-  key: old",
		"+  key: new",
	}, "\n")

	got := kubefilter.SummarizeKubectlDiff(input, 6)
	for _, want := range []string{
		"diff: 2 objects changed",
		"apps.v1.Deployment.default.web: +2 -1",
		"v1.ConfigMap.default.cfg: +1 -1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl diff summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "replicas: 3") {
		t.Fatalf("expected attribute-level diff lines to be dropped:\n%s", got)
	}
}

func TestKubectlMutationRecoveryInfo(t *testing.T) {
	applyInput := strings.Join([]string{
		"deployment.apps/api configured",
		"service/api unchanged",
		"namespace/staging created",
		"Warning: something noteworthy",
	}, "\n")
	if kind, summary, requireRawCapture := kubefilter.KubectlApplyRecoveryInfo(applyInput, 3); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected kubectl apply recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	rolloutInput := strings.Join([]string{
		`Waiting for deployment "web" rollout to finish: 1 of 3 updated replicas are available...`,
		`Waiting for deployment "web" rollout to finish: 2 of 3 updated replicas are available...`,
		`deployment "web" successfully rolled out`,
	}, "\n")
	if kind, summary, requireRawCapture := kubefilter.KubectlRolloutRecoveryInfo(rolloutInput, 2); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected kubectl rollout recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
