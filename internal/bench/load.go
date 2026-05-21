package bench

import (
	"embed"
	"fmt"

	"szr/internal/engine"
)

//go:embed testdata/*
var embeddedFixtures embed.FS

type ReadFileFunc func(name string) ([]byte, error)

func Fixtures() ([]Fixture, error) {
	return LoadFixtures(embeddedFixtures.ReadFile, Specs())
}

func MustFixtures() []Fixture {
	return MustLoadFixtures(Fixtures)
}

func MustLoadFixtures(load func() ([]Fixture, error)) []Fixture {
	fixtures, err := load()
	if err != nil {
		panic(err)
	}
	return fixtures
}

func LoadFixtures(readFile ReadFileFunc, specs []Spec) ([]Fixture, error) {
	if readFile == nil {
		return nil, fmt.Errorf("missing fixture reader")
	}

	fixtures := make([]Fixture, 0, len(specs))
	for _, spec := range specs {
		stdout, err := readText(readFile, spec.StdoutFile)
		if err != nil {
			return nil, err
		}
		stderr, err := readText(readFile, spec.StderrFile)
		if err != nil {
			return nil, err
		}

		fixtures = append(fixtures, Fixture{
			Name:             spec.Name,
			Class:            spec.Class,
			Description:      spec.Description,
			ProfileName:      spec.ProfileName,
			Invocation:       engine.Invocation{Command: append([]string(nil), spec.Command...), Display: append([]string(nil), spec.Display...), Cwd: spec.Cwd},
			Execution:        engine.Execution{Stdout: stdout, Stderr: stderr, ExitCode: spec.ExitCode},
			ExpectedContains: append([]string(nil), spec.ExpectedContains...),
		})
	}
	return fixtures, nil
}

func readText(readFile ReadFileFunc, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := readFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}
