package kubernetes_test

import (
	"strings"
	"testing"

	kubefilter "github.com/devr-tools/szr/internal/filters/kubernetes"
)

func TestSummarizeKubectlGet(t *testing.T) {
	input := `{"kind":"PodList","items":[{"kind":"Pod","metadata":{"name":"api-7d9","namespace":"default"},"status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":1}]}},{"kind":"Pod","metadata":{"name":"worker-6f4","namespace":"jobs"},"status":{"phase":"Pending","containerStatuses":[{"ready":false,"restartCount":0}]}}]}`
	got := kubefilter.SummarizeKubectlGet(input, 4)
	for _, want := range []string{
		"pod: 2 items",
		"pod api-7d9 ns=default phase=Running ready=1/1 restarts=1",
		"pod worker-6f4 ns=jobs phase=Pending ready=0/1 restarts=0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl get summary:\n%s", want, got)
		}
	}
}

func TestSummarizeKubectlDescribe(t *testing.T) {
	input := strings.Join([]string{
		"Name:           api-7d9",
		"Namespace:      default",
		"Node:           node-a",
		"Status:         Running",
		"IP:             10.0.0.10",
		"Events:",
		"  Type     Reason     Age   From     Message",
		"  Warning  BackOff    1m    kubelet  Back-off restarting failed container",
		"  Normal   Pulled     2m    kubelet  Container image pulled",
	}, "\n")
	got := kubefilter.SummarizeKubectlDescribe(input, 6)
	for _, want := range []string{
		"Name:           api-7d9",
		"Namespace:      default",
		"Status:         Running",
		"Warning  BackOff",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl describe summary:\n%s", want, got)
		}
	}
}

func TestSummarizeKubectlLogs(t *testing.T) {
	input := strings.Join([]string{
		"api-7d9/api ERROR failed to connect",
		"api-7d9/api ERROR failed to connect",
		"worker-6f4/worker panic: queue broken",
	}, "\n")
	got := kubefilter.SummarizeKubectlLogs(input, 5)
	for _, want := range []string{
		"sources: 2",
		"api-7d9/api: ERROR failed to connect (x2)",
		"worker-6f4/worker: panic: queue broken",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl logs summary:\n%s", want, got)
		}
	}
}

func TestKubectlRecoveryInfo(t *testing.T) {
	getInput := `{"kind":"PodList","items":[{"kind":"Pod","metadata":{"name":"api-7d9","namespace":"default"},"status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":1}]}},{"kind":"Pod","metadata":{"name":"worker-6f4","namespace":"jobs"},"status":{"phase":"Pending","containerStatuses":[{"ready":false,"restartCount":0}]}},{"kind":"Pod","metadata":{"name":"cron-1","namespace":"jobs"},"status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":0}]}}]}`
	if kind, summary, requireRawCapture := kubefilter.KubectlGetRecoveryInfo(getInput, 3); kind != "full-output" || summary != "omitted 1 additional resources" || !requireRawCapture {
		t.Fatalf("unexpected kubectl get recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	describeInput := strings.Join([]string{
		"Name:           api-7d9",
		"Namespace:      default",
		"Node:           node-a",
		"Status:         Running",
		"IP:             10.0.0.10",
		"Reason:         CrashLoopBackOff",
	}, "\n")
	if kind, summary, requireRawCapture := kubefilter.KubectlDescribeRecoveryInfo(describeInput, 3); kind != "full-output" || summary != "omitted 3 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected kubectl describe recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	logsInput := strings.Join([]string{
		"api-7d9/api ERROR failed to connect",
		"api-7d9/api WARN backing off",
		"worker-6f4/worker panic: queue broken",
	}, "\n")
	if kind, summary, requireRawCapture := kubefilter.KubectlLogsRecoveryInfo(logsInput, 2); kind != "full-output" || summary != "omitted 2 additional log lines" || !requireRawCapture {
		t.Fatalf("unexpected kubectl logs recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
