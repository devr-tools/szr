package updates

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func (s *Service) verifyInstalledVersion(ctx context.Context, target string) error {
	installed, err := s.installedVersion(ctx)
	if err != nil {
		return fmt.Errorf("upgrade command succeeded but the installed version could not be verified: %w", err)
	}
	if _, ok := parseVersion(installed); !ok {
		return fmt.Errorf("upgrade command succeeded but the installed binary reported an unrecognized version %q", installed)
	}
	if compareVersions(installed, target) < 0 {
		return fmt.Errorf("upgrade command succeeded but the installed binary still reports %s (expected %s); the upstream package source may be stale", installed, target)
	}
	return nil
}

func (s *Service) reportedBinaryVersion(ctx context.Context) (string, error) {
	execPath, err := s.executable()
	if err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, execPath, "--version").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return "", errors.New("installed binary reported no version")
	}
	return fields[len(fields)-1], nil
}
