package util

import (
	"fmt"
	"strings"
)

func ErrorFromErrorChannel(resC chan error) error {
	close(resC)
	var errs []string
	for err := range resC {
		if err != nil {
			errTxt := strings.TrimPrefix(err.Error(), "got error[s]:\n- ")
			errs = append(errs, fmt.Sprintf("- %v", errTxt))
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("got error[s]:\n%v", strings.Join(errs, "\n"))
	}
	return nil
}
