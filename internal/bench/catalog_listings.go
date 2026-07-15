package bench

// listingSpecs covers the tabular listing profiles: bare env dumps, docker
// image tables, and glab pipeline listings.
var listingSpecs = []Spec{
	{
		Name:        "env-dump",
		Class:       "env-print",
		Description: "Bare env dump with sixty variables, secret-looking values, a long PATH, and a giant LS_COLORS.",
		ProfileName: "env-print",
		Command:     []string{"env"},
		Display:     []string{"env"},
		StdoutFile:  "testdata/env_dump.txt",
		ExpectedContains: []string{
			"env: 60 vars",
			"PATH: 10 entries:",
			"HOME=/Users/devbot",
			"<redacted len=",
		},
		MinTokenSavings: 62,
		MinQualityScore: 75,
	},
	{
		Name:        "docker-images-listing",
		Class:       "docker-images",
		Description: "Default docker images table with twenty images including three dangling layers.",
		ProfileName: "docker-images",
		Command:     []string{"docker", "images"},
		Display:     []string{"docker", "images"},
		StdoutFile:  "testdata/docker_images.txt",
		ExpectedContains: []string{
			"images: 20 (total ",
			"dangling <none>: 3 (1.9GB)",
			"registry.acme.dev/platform/api:1.42.0 812MB (2d)",
		},
		MinTokenSavings: 60,
		MinQualityScore: 75,
	},
	{
		Name:        "glab-pipeline-list",
		Class:       "glab-pipeline",
		Description: "glab pipeline list with forty rows where a handful of failed, running, and canceled entries carry the signal.",
		ProfileName: "glab-pipeline",
		Command:     []string{"glab", "pipeline", "list"},
		Display:     []string{"glab", "pipeline", "list"},
		StdoutFile:  "testdata/glab_pipeline_list.txt",
		ExpectedContains: []string{
			"pipelines: 40 (success=34 failed=3",
			"failed: #418223 feat/retry-backoff-queue (3h)",
			"... +31 more success",
		},
		MinTokenSavings: 70,
		MinQualityScore: 75,
	},
}
