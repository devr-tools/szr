package installers

import (
	"os"
	"path/filepath"
)

func Apply(plan Plan) error {
	return ApplyWith(plan, os.MkdirAll, os.ReadFile, os.WriteFile, os.Chmod)
}

func ApplyWith(
	plan Plan,
	mkdirAll func(string, os.FileMode) error,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	chmod func(string, os.FileMode) error,
) error {
	for _, file := range plan.Files {
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

	return nil
}
