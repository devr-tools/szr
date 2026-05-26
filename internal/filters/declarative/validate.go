package declarative

import (
	"fmt"
	"strings"
)

func Validate(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if spec.Head < 0 {
		return fmt.Errorf("%s: head must be >= 0", spec.Name)
	}
	if spec.Tail < 0 {
		return fmt.Errorf("%s: tail must be >= 0", spec.Name)
	}
	if spec.Head > 0 && spec.Tail > 0 {
		return fmt.Errorf("%s: head and tail cannot both be set", spec.Name)
	}
	if spec.MaxLineWidth < 0 {
		return fmt.Errorf("%s: max_line_width must be >= 0", spec.Name)
	}
	return nil
}
