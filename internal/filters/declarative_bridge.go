package filters

import "github.com/devr-tools/szr/internal/filters/declarative"

func RenderDeclarativeBuiltin(name, input string, maxLines int) string {
	result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
	if err != nil {
		return ""
	}
	return result.Text
}

func DeclarativeBuiltinRecoveryInfo(name, noun, input string, maxLines int) (string, string, bool) {
	result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
	if err != nil {
		return NoRecovery()
	}
	return DeclarativeFullOutputRecovery(result, noun)
}

func NewDeclarativeBuiltinReducer(
	name string,
	noun string,
	maxLines int,
	stdoutEnabled bool,
	stderrEnabled bool,
) *BufferedTextReducer {
	renderBuiltin := func(input string) string {
		result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
		if err != nil {
			return ""
		}
		return result.Text
	}
	recoveryBuiltin := func(input string) (string, string, bool) {
		result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
		if err != nil {
			return NoRecovery()
		}
		return DeclarativeFullOutputRecovery(result, noun)
	}
	return NewBufferedTextReducerWithRecovery(
		stdoutEnabled,
		stderrEnabled,
		renderBuiltin,
		recoveryBuiltin,
	)
}
