package profiles

import "szr/internal/engine"

func Builtins(maxLines int) []engine.Profile {
	list := coreProfiles(maxLines)
	list = append(list, jsProfiles(maxLines)...)
	return list
}
