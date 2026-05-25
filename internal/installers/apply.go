package installers

import (
	"os"
	"path/filepath"
)

func Apply(plan Plan) error {
	return ApplyWith(plan, os.MkdirAll, os.ReadFile, os.WriteFile, os.Chmod, os.Remove)
}

func ApplyWith(
	plan Plan,
	mkdirAll func(string, os.FileMode) error,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	chmod func(string, os.FileMode) error,
	remove func(string) error,
) error {
	for _, file := range plan.Files {
		switch file.Strategy {
		case StrategyDelete:
			if err := remove(file.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
			removeEmptyParents(filepath.Dir(file.Path), plan.Paths.RepoRoot, remove)
		case StrategyUnmerge:
			existing, err := readFile(file.Path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			content, changed := dematerialize(string(existing), file)
			if !changed {
				continue
			}
			if content == "" {
				if err := remove(file.Path); err != nil && !os.IsNotExist(err) {
					return err
				}
				removeEmptyParents(filepath.Dir(file.Path), plan.Paths.RepoRoot, remove)
				continue
			}
			if err := writeFile(file.Path, []byte(content), file.Mode); err != nil {
				return err
			}
			if file.Mode != 0 {
				if err := chmod(file.Path, file.Mode); err != nil {
					return err
				}
			}
		default:
			if err := mkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
				return err
			}

			existing := []byte(nil)
			if data, err := readFile(file.Path); err == nil {
				existing = data
			} else if !os.IsNotExist(err) {
				return err
			}

			content := materialize(string(existing), file)
			if err := writeFile(file.Path, []byte(content), file.Mode); err != nil {
				return err
			}
			if file.Mode != 0 {
				if err := chmod(file.Path, file.Mode); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func removeEmptyParents(dir, stop string, remove func(string) error) {
	stop = filepath.Clean(stop)
	for dir != "." && dir != string(filepath.Separator) {
		if filepath.Clean(dir) == stop {
			return
		}
		if err := remove(dir); err != nil {
			if os.IsNotExist(err) {
				dir = filepath.Dir(dir)
				continue
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}
