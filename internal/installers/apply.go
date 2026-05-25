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
		if err := applyFile(plan.Paths.RepoRoot, file, mkdirAll, readFile, writeFile, chmod, remove); err != nil {
			return err
		}
	}

	return nil
}

func applyFile(
	repoRoot string,
	file File,
	mkdirAll func(string, os.FileMode) error,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	chmod func(string, os.FileMode) error,
	remove func(string) error,
) error {
	switch file.Strategy {
	case StrategyDelete:
		return deleteFile(repoRoot, file.Path, remove)
	case StrategyUnmerge:
		return unmergeFile(repoRoot, file, readFile, writeFile, chmod, remove)
	default:
		return writeFileContent(file, mkdirAll, readFile, writeFile, chmod)
	}
}

func deleteFile(repoRoot, path string, remove func(string) error) error {
	if err := remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	removeEmptyParents(filepath.Dir(path), repoRoot, remove)
	return nil
}

func unmergeFile(
	repoRoot string,
	file File,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	chmod func(string, os.FileMode) error,
	remove func(string) error,
) error {
	existing, err := readFile(file.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	content, changed := dematerialize(string(existing), file)
	if !changed {
		return nil
	}
	if content == "" {
		return deleteFile(repoRoot, file.Path, remove)
	}
	return persistFile(file, content, writeFile, chmod)
}

func writeFileContent(
	file File,
	mkdirAll func(string, os.FileMode) error,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	chmod func(string, os.FileMode) error,
) error {
	if err := mkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
		return err
	}

	existing, err := readExistingFile(file.Path, readFile)
	if err != nil {
		return err
	}

	content := materialize(string(existing), file)
	return persistFile(file, content, writeFile, chmod)
}

func readExistingFile(path string, readFile func(string) ([]byte, error)) ([]byte, error) {
	data, err := readFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func persistFile(
	file File,
	content string,
	writeFile func(string, []byte, os.FileMode) error,
	chmod func(string, os.FileMode) error,
) error {
	if err := writeFile(file.Path, []byte(content), file.Mode); err != nil {
		return err
	}
	if file.Mode == 0 {
		return nil
	}
	return chmod(file.Path, file.Mode)
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
