package declarative

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed builtins/*.json
var builtinFiles embed.FS

var (
	loadBuiltinsOnce sync.Once
	builtinSpecs     map[string]Spec
	builtinCompiled  map[string]compiledSpec
	builtinLoadErr   error
)

func Builtin(name string) (Spec, error) {
	specs, err := Builtins()
	if err != nil {
		return Spec{}, err
	}
	spec, ok := specs[name]
	if !ok {
		return Spec{}, fmt.Errorf("unknown declarative reducer %q", name)
	}
	return spec, nil
}

func Builtins() (map[string]Spec, error) {
	loadBuiltinsOnce.Do(func() {
		entries, err := builtinFiles.ReadDir("builtins")
		if err != nil {
			builtinLoadErr = err
			return
		}

		loaded := make(map[string]Spec, len(entries))
		compiledLoaded := make(map[string]compiledSpec, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join("builtins", entry.Name())
			data, err := builtinFiles.ReadFile(path)
			if err != nil {
				builtinLoadErr = err
				return
			}
			var spec Spec
			if err := json.Unmarshal(data, &spec); err != nil {
				builtinLoadErr = fmt.Errorf("parse builtin reducer %s: %w", path, err)
				return
			}
			if spec.Name == "" {
				spec.Name = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			}
			if err := Validate(spec); err != nil {
				builtinLoadErr = fmt.Errorf("validate builtin reducer %s: %w", path, err)
				return
			}
			compiled, err := compileValidatedSpec(spec, Options{})
			if err != nil {
				builtinLoadErr = fmt.Errorf("compile builtin reducer %s: %w", path, err)
				return
			}
			loaded[spec.Name] = spec
			compiledLoaded[spec.Name] = compiled
		}
		builtinSpecs = loaded
		builtinCompiled = compiledLoaded
	})
	if builtinLoadErr != nil {
		return nil, builtinLoadErr
	}

	out := make(map[string]Spec, len(builtinSpecs))
	for name, spec := range builtinSpecs {
		out[name] = spec
	}
	return out, nil
}

func compiledBuiltin(name string) (compiledSpec, error) {
	if _, err := Builtins(); err != nil {
		return compiledSpec{}, err
	}
	spec, ok := builtinCompiled[name]
	if !ok {
		return compiledSpec{}, fmt.Errorf("unknown declarative reducer %q", name)
	}
	return spec, nil
}

func BuiltinNames() ([]string, error) {
	specs, err := Builtins()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
