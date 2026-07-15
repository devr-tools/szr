package profiles_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

// TestDockerImagesProfile pins matching for the images/image-ls spellings,
// the tab-format prepare, and a render that collapses dangling images while
// keeping repo:tag, size, and a compact age per image.
func TestDockerImagesProfile(t *testing.T) {
	list := profiles.Builtins(6)
	dockerImages := testutil.FindProfile(t, list, "docker-images")

	for _, display := range [][]string{
		{"docker", "images"},
		{"docker", "image", "ls"},
		{"docker", "image", "list"},
	} {
		if !dockerImages.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected docker-images to match %v", display)
		}
	}
	for _, display := range [][]string{
		{"docker", "image", "inspect", "app"},
		{"docker", "ps"},
		{"podman", "images"},
	} {
		if dockerImages.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected docker-images not to match %v", display)
		}
	}

	format := "{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}"
	if got := dockerImages.Prepare(engine.Invocation{Command: []string{"docker", "images"}}); !reflect.DeepEqual(got, []string{"docker", "images", "--format", format}) {
		t.Fatalf("unexpected docker images prepare: %#v", got)
	}
	if got := dockerImages.Prepare(engine.Invocation{Command: []string{"docker", "images", "-q"}}); !reflect.DeepEqual(got, []string{"docker", "images", "-q"}) {
		t.Fatalf("expected quiet listing to be preserved: %#v", got)
	}
	if got := dockerImages.Prepare(engine.Invocation{Command: []string{"docker", "image", "ls", "--format", "json"}}); !reflect.DeepEqual(got, []string{"docker", "image", "ls", "--format", "json"}) {
		t.Fatalf("expected explicit format to be preserved: %#v", got)
	}

	rendered := dockerImages.Render(engine.Invocation{}, engine.Execution{Stdout: strings.Join([]string{
		"app\tlatest\t812MB\t2 days ago",
		"<none>\t<none>\t97.4MB\t3 months ago",
		"api\t1.2.0\t1.24GB\tAbout an hour ago",
	}, "\n")})
	for _, want := range []string{
		"images: 3 (total 2.1GB)",
		"dangling <none>: 1 (97MB)",
		"app:latest 812MB (2d)",
		"api:1.2.0 1.24GB (1h)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in docker images render:\n%s", want, rendered)
		}
	}
}

func TestDockerImagesTableAndRecovery(t *testing.T) {
	list := profiles.Builtins(6)
	dockerImages := testutil.FindProfile(t, list, "docker-images")

	rendered := dockerImages.Render(engine.Invocation{}, engine.Execution{Stdout: strings.Join([]string{
		"REPOSITORY          TAG          IMAGE ID       CREATED        SIZE",
		"registry.acme.dev/platform/api   1.42.0   3f8a9b2c1d4e   2 weeks ago    812MB",
	}, "\n")})
	if !strings.Contains(rendered, "images: 1 (total 812MB)") || !strings.Contains(rendered, "registry.acme.dev/platform/api:1.42.0 812MB (2w)") {
		t.Fatalf("expected table-format parse in render:\n%s", rendered)
	}

	stream := dockerImages.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	stream.ConsumeStdout([]byte(strings.Join([]string{
		"app\tlatest\t812MB\t2 days ago",
		"api\t1.2.0\t1.24GB\t5 days ago",
		"cron\t0.3.1\t54MB\t6 weeks ago",
	}, "\n")))
	recovery, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable docker images reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional images" || !requireRawCapture {
		t.Fatalf("unexpected docker images recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
