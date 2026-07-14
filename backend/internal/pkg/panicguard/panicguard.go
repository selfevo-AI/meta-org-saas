package panicguard

import (
	"fmt"
	"log"
	"runtime/debug"
)

// Recover logs a recovered panic with its stack trace. Defer it at the top of
// goroutines that must never take down the whole process (background workers,
// per-request stream pumps). Place it before other defers so cleanup still
// runs during unwinding.
func Recover(scope string) {
	if r := recover(); r != nil {
		log.Printf("%s: recovered panic: %v\n%s", scope, r, debug.Stack())
	}
}

// RecoverAs logs a recovered panic and stores it in *errp so loop-based
// workers can route it through their existing error handling and keep
// serving subsequent iterations.
func RecoverAs(scope string, errp *error) {
	if r := recover(); r != nil {
		log.Printf("%s: recovered panic: %v\n%s", scope, r, debug.Stack())
		if errp != nil && *errp == nil {
			*errp = fmt.Errorf("%s: recovered panic: %v", scope, r)
		}
	}
}
