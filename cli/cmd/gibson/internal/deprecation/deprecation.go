// Package deprecation provides the one-line deprecation banner that
// the back-compat verb shims (gibson agent enroll, gibson tool enroll,
// gibson plugin <verb>) emit when GIBSON_DEPRECATION_WARNINGS=1 is set.
//
// The banner is opt-in (default off) to avoid surprising adopters who
// still have Makefiles wired to the old verb forms. The next major
// release of the ADK removes the aliases entirely; CHANGELOG advertises
// the cutover.
package deprecation

import (
	"fmt"
	"io"
	"os"
	"sync"
)

const env = "GIBSON_DEPRECATION_WARNINGS"

// once guards a single banner per process invocation, so chained
// shims (e.g. plugin enroll → component register) don't print twice.
var once sync.Once

// Notify writes one line to stderr if GIBSON_DEPRECATION_WARNINGS=1.
// The format is intentionally short:
//
//	deprecated: `gibson <old>` → use `gibson <new>` (set GIBSON_DEPRECATION_WARNINGS=0 to silence)
//
// Subsequent calls in the same process are no-ops.
func Notify(oldForm, newForm string) {
	if os.Getenv(env) != "1" {
		return
	}
	once.Do(func() {
		writeBanner(os.Stderr, oldForm, newForm)
	})
}

// NotifyTo is the test-friendly variant — same once.Do semantics but
// writes to the supplied writer. Reset between tests via Reset().
func NotifyTo(w io.Writer, oldForm, newForm string) {
	if os.Getenv(env) != "1" {
		return
	}
	once.Do(func() {
		writeBanner(w, oldForm, newForm)
	})
}

// Reset clears the once.Do, intended for tests only.
func Reset() {
	once = sync.Once{}
}

func writeBanner(w io.Writer, oldForm, newForm string) {
	fmt.Fprintf(w,
		"deprecated: `gibson %s` → use `gibson %s` (set GIBSON_DEPRECATION_WARNINGS=0 to silence)\n",
		oldForm, newForm)
}
