package dotnet_test

import (
	"strings"
	"testing"

	dotnetfilter "github.com/devr-tools/szr/internal/filters/dotnet"
)

func TestSummarizeDotnetBuildKeepsCodedDiagnostics(t *testing.T) {
	input := strings.Join([]string{
		"  Determining projects to restore...",
		"  Restored /workspace/billing-svc/Billing.Api/Billing.Api.csproj (in 402 ms).",
		"  Billing.Core -> /workspace/billing-svc/Billing.Core/bin/Debug/net9.0/Billing.Core.dll",
		"/workspace/billing-svc/Billing.Api/Services/InvoiceService.cs(52,21): error CS0103: The name 'invocie' does not exist in the current context [/workspace/billing-svc/Billing.Api/Billing.Api.csproj]",
		"/workspace/billing-svc/Billing.Api/Services/InvoiceService.cs(93,52): error CS1002: ; expected [/workspace/billing-svc/Billing.Api/Billing.Api.csproj]",
		"/workspace/billing-svc/Billing.Api/Controllers/InvoicesController.cs(27,33): warning CS8602: Dereference of a possibly null reference. [/workspace/billing-svc/Billing.Api/Billing.Api.csproj]",
		"Build FAILED.",
		"/workspace/billing-svc/Billing.Api/Services/InvoiceService.cs(52,21): error CS0103: The name 'invocie' does not exist in the current context [/workspace/billing-svc/Billing.Api/Billing.Api.csproj]",
		"/workspace/billing-svc/Billing.Api/Services/InvoiceService.cs(93,52): error CS1002: ; expected [/workspace/billing-svc/Billing.Api/Billing.Api.csproj]",
		"    1 Warning(s)",
		"    2 Error(s)",
		"Time Elapsed 00:00:03.18",
	}, "\n")

	got := dotnetfilter.SummarizeDotnetBuild(input, 8)
	for _, want := range []string{
		"InvoiceService.cs(52,21): error CS0103: The name 'invocie' does not exist",
		"InvoiceService.cs(93,52): error CS1002: ; expected",
		"warning CS8602",
		"Build FAILED.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in dotnet build summary:\n%s", want, got)
		}
	}
	if strings.Count(got, "error CS1002") != 1 {
		t.Fatalf("expected repeated MSBuild diagnostics to be deduplicated:\n%s", got)
	}
	if strings.Contains(got, "Determining projects to restore") || strings.Contains(got, "Restored /workspace") {
		t.Fatalf("expected restore noise to be dropped:\n%s", got)
	}
}

func TestSummarizeDotnetBuildPass(t *testing.T) {
	input := strings.Join([]string{
		"  Determining projects to restore...",
		"  All projects are up-to-date for restore.",
		"  Billing.Core -> /workspace/billing-svc/Billing.Core/bin/Debug/net9.0/Billing.Core.dll",
		"Build succeeded.",
		"    0 Warning(s)",
		"    0 Error(s)",
		"Time Elapsed 00:00:01.42",
	}, "\n")

	got := dotnetfilter.SummarizeDotnetBuild(input, 6)
	if !strings.Contains(got, "Build succeeded.") || !strings.Contains(got, "0 Error(s)") {
		t.Fatalf("expected build success summary:\n%s", got)
	}
}

func TestSummarizeDotnetTestKeepsNameAssertAndAnchor(t *testing.T) {
	input := strings.Join([]string{
		"  Determining projects to restore...",
		"  Billing.Tests -> /workspace/billing-svc/Billing.Tests/bin/Debug/net9.0/Billing.Tests.dll",
		"Test run for /workspace/billing-svc/Billing.Tests/bin/Debug/net9.0/Billing.Tests.dll (.NETCoreApp,Version=v9.0)",
		"Starting test execution, please wait...",
		"A total of 1 test files matched the specified pattern.",
		"  Failed Billing.Tests.InvoiceTests.Applies_Late_Fee [6 ms]",
		"  Error Message:",
		"   Assert.Equal() Failure: Values differ",
		"Expected: 1050",
		"Actual:   1000",
		"  Stack Trace:",
		"     at Billing.Tests.InvoiceTests.Applies_Late_Fee() in /workspace/billing-svc/Billing.Tests/InvoiceTests.cs:line 42",
		"  Passed Billing.Tests.InvoiceTests.Computes_Subtotal [1 ms]",
		"Failed!  - Failed:     1, Passed:     7, Skipped:     0, Total:     8, Duration: 74 ms - Billing.Tests.dll (net9.0)",
	}, "\n")

	got := dotnetfilter.SummarizeDotnetTest(input, 10)
	for _, want := range []string{
		"Failed Billing.Tests.InvoiceTests.Applies_Late_Fee [6 ms]",
		"Assert.Equal() Failure: Values differ",
		"Expected: 1050",
		"Actual:   1000",
		"at Billing.Tests.InvoiceTests.Applies_Late_Fee() in /workspace/billing-svc/Billing.Tests/InvoiceTests.cs:line 42",
		"Failed!  - Failed:     1, Passed:     7",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in dotnet test summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Passed Billing.Tests.InvoiceTests.Computes_Subtotal") {
		t.Fatalf("expected passing test noise to be dropped:\n%s", got)
	}
}

func TestSummarizeDotnetTestNamesSurviveTightBudget(t *testing.T) {
	input := strings.Join([]string{
		"  Failed Billing.Tests.InvoiceTests.Applies_Late_Fee [6 ms]",
		"  Error Message:",
		"   Assert.Equal() Failure: Values differ",
		"  Failed Billing.Tests.InvoiceTests.Rounds_Currency [2 ms]",
		"  Error Message:",
		"   Assert.True() Failure",
		"  Failed Billing.Tests.InvoiceTests.Rejects_Negative_Total [3 ms]",
		"  Error Message:",
		"   Assert.Throws() Failure",
		"Failed!  - Failed:     3, Passed:     5, Skipped:     0, Total:     8, Duration: 91 ms - Billing.Tests.dll (net9.0)",
	}, "\n")

	got := dotnetfilter.SummarizeDotnetTest(input, 4)
	for _, want := range []string{
		"Failed Billing.Tests.InvoiceTests.Applies_Late_Fee",
		"Failed Billing.Tests.InvoiceTests.Rounds_Currency",
		"Failed Billing.Tests.InvoiceTests.Rejects_Negative_Total",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to survive the tight budget:\n%s", want, got)
		}
	}
}

func TestSummarizeDotnetTestPass(t *testing.T) {
	input := strings.Join([]string{
		"Starting test execution, please wait...",
		"A total of 1 test files matched the specified pattern.",
		"Passed!  - Failed:     0, Passed:     8, Skipped:     0, Total:     8, Duration: 52 ms - Billing.Tests.dll (net9.0)",
	}, "\n")

	got := dotnetfilter.SummarizeDotnetTest(input, 4)
	if !strings.Contains(got, "Passed!  - Failed:     0, Passed:     8") {
		t.Fatalf("expected pass summary retention:\n%s", got)
	}
}

func TestDotnetRecoveryInfo(t *testing.T) {
	buildInput := strings.Join([]string{
		"/src/App.cs(5,1): error CS1002: ; expected [/src/App.csproj]",
		"/src/App.cs(9,2): error CS0103: The name 'x' does not exist in the current context [/src/App.csproj]",
		"/src/App.cs(12,3): warning CS0219: The variable 'y' is assigned but never used [/src/App.csproj]",
		"Build FAILED.",
		"    1 Warning(s)",
		"    2 Error(s)",
	}, "\n")
	if kind, summary, requireRawCapture := dotnetfilter.DotnetBuildRecoveryInfo(buildInput, 3); kind != "full-output" || summary == "" || !requireRawCapture {
		t.Fatalf("unexpected dotnet build recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
	if kind, summary, requireRawCapture := dotnetfilter.DotnetBuildRecoveryInfo(buildInput, 12); kind != "" || summary != "" || requireRawCapture {
		t.Fatalf("expected no recovery for roomy budget: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
