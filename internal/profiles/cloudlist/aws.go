package cloudlist

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	cloudfilter "github.com/devr-tools/szr/internal/filters/cloudlist"
	"github.com/devr-tools/szr/internal/profilekit"
)

// awsProfiles are the aws subcommands with dedicated output shapes; they sit
// ahead of the generic cloud-list and cloud-logs profiles so everything not
// claimed here keeps falling through to the generic JSON handling.
func awsProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		awsLogEventsProfile(maxLines),
		awsLambdaFunctionsProfile(maxLines),
		awsStackEventsProfile(maxLines),
		awsS3LsProfile(maxLines),
		awsECSProfile(maxLines),
	}
}

func isAWSSubcommand(args []string, service string, verbs ...string) bool {
	if len(args) == 0 || args[0] != "aws" {
		return false
	}
	positional := positionalArgs(args[1:], awsValueFlags)
	if len(positional) < 2 || positional[0] != service {
		return false
	}
	for _, verb := range verbs {
		if positional[1] == verb {
			return true
		}
	}
	return false
}

func prepareAWSJSONOutput(inv engine.Invocation) []string {
	return appendInventoryFlag(inv.Command, []string{"--output"}, []string{"--output="}, "--output", "json")
}

type awsSummarizer struct {
	render   func(string, int) string
	recovery func(string, int) (string, string, bool)
}

func (s awsSummarizer) Render(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(_ engine.Invocation, exec engine.Execution) string {
		return s.render(exec.Stdout, maxLines)
	}
}

func (s awsSummarizer) StreamRender(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(
		true,
		false,
		func(input string) string {
			return s.render(input, budget.MaxLines)
		},
		func(input string) (string, string, bool) {
			return s.recovery(input, budget.MaxLines)
		},
	)
}

func awsLogEventsProfile(maxLines int) engine.Profile {
	summarizer := awsSummarizer{render: cloudfilter.SummarizeAWSLogEvents, recovery: cloudfilter.AWSLogEventsRecoveryInfo}
	return engine.Profile{
		Name:             "aws-log-events",
		Description:      "Deduplicates CloudWatch log events into message templates with counts, keeping every distinct error.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match: func(inv engine.Invocation) bool {
			return isAWSSubcommand(inv.Display, "logs", "filter-log-events", "get-log-events")
		},
		Prepare:      prepareAWSJSONOutput,
		Render:       summarizer.Render(maxLines),
		StreamRender: summarizer.StreamRender,
		ParseBytes:   profilekit.ParseStdout,
		Explain: []string{
			"Matches `aws logs filter-log-events` and `aws logs get-log-events` ahead of the generic cloud log profile.",
			"Groups events whose messages differ only in numbers or identifiers into counted templates and keeps every distinct error template.",
		},
	}
}

func awsLambdaFunctionsProfile(maxLines int) engine.Profile {
	summarizer := awsSummarizer{render: cloudfilter.SummarizeAWSLambdaFunctions, recovery: cloudfilter.AWSLambdaFunctionsRecoveryInfo}
	return engine.Profile{
		Name:             "aws-lambda-functions",
		Description:      "Reduces Lambda function listings to one name/runtime/size line per function.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match: func(inv engine.Invocation) bool {
			return isAWSSubcommand(inv.Display, "lambda", "list-functions")
		},
		Prepare:      prepareAWSJSONOutput,
		Render:       summarizer.Render(maxLines),
		StreamRender: summarizer.StreamRender,
		ParseBytes:   profilekit.ParseStdout,
		Explain: []string{
			"Matches `aws lambda list-functions` ahead of the generic cloud inventory profile.",
			"Keeps function name, runtime, code size, memory, and timeout per line with a runtime breakdown header.",
		},
	}
}

func awsStackEventsProfile(maxLines int) engine.Profile {
	summarizer := awsSummarizer{render: cloudfilter.SummarizeAWSStackEvents, recovery: cloudfilter.AWSStackEventsRecoveryInfo}
	return engine.Profile{
		Name:             "aws-stack-events",
		Description:      "Surfaces CloudFormation stack event failures with reasons before progress noise.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match: func(inv engine.Invocation) bool {
			return isAWSSubcommand(inv.Display, "cloudformation", "describe-stack-events")
		},
		Prepare:      prepareAWSJSONOutput,
		Render:       summarizer.Render(maxLines),
		StreamRender: summarizer.StreamRender,
		ParseBytes:   profilekit.ParseStdout,
		Explain: []string{
			"Matches `aws cloudformation describe-stack-events` ahead of the generic cloud inventory profile.",
			"Keeps failed events with their status reasons first (folding repeated cancellations) and counts the rest.",
		},
	}
}

func awsS3LsProfile(maxLines int) engine.Profile {
	summarizer := awsSummarizer{render: cloudfilter.SummarizeAWSS3Ls, recovery: cloudfilter.AWSS3LsRecoveryInfo}
	return engine.Profile{
		Name:             "aws-s3-ls",
		Description:      "Summarizes `aws s3 ls` listings to entry counts, total size, and the largest objects.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(25),
		Match: func(inv engine.Invocation) bool {
			return isAWSSubcommand(inv.Display, "s3", "ls")
		},
		Render:       summarizer.Render(maxLines),
		StreamRender: summarizer.StreamRender,
		ParseBytes:   profilekit.ParseStdout,
		Explain: []string{
			"Matches `aws s3 ls` for buckets, prefixes, and object listings.",
			"Reports object and prefix counts with total size and keeps the largest objects plus any --summarize footer.",
		},
	}
}

func awsECSProfile(maxLines int) engine.Profile {
	summarizer := awsSummarizer{render: cloudfilter.SummarizeAWSECS, recovery: cloudfilter.AWSECSRecoveryInfo}
	return engine.Profile{
		Name:             "aws-ecs-state",
		Description:      "Summarizes ECS listings and service descriptions around desired/running state.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match: func(inv engine.Invocation) bool {
			return isAWSSubcommand(inv.Display, "ecs", "list-clusters", "list-services", "list-tasks", "describe-services")
		},
		Prepare:      prepareAWSJSONOutput,
		Render:       summarizer.Render(maxLines),
		StreamRender: summarizer.StreamRender,
		ParseBytes:   profilekit.ParseStdout,
		Explain: []string{
			"Matches `aws ecs list-clusters`, `list-services`, `list-tasks`, and `describe-services` ahead of the generic cloud inventory profile.",
			"Shortens ARN listings to resource names and reduces service descriptions to status, running/desired counts, and rollout state with failures first.",
		},
	}
}
