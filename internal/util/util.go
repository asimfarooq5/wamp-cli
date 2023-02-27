package util

import (
	"fmt"
	"strings"
)

func ErrorFromErrorChannel(resC chan error) error {
	var errs []string
	for err := range resC {
		if err != nil {
			errs = append(errs, fmt.Sprintf("- %v", err))
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("got error[s]:\n%v", strings.Join(errs, "\n"))
	}
	return nil
}
