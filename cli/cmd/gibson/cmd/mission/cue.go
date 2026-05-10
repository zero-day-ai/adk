package mission

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/parser"
)

// compileCUE compiles raw CUE bytes and returns the context plus
// the resulting cue.Value. Errors carry the CUE library's source
// position information.
func compileCUE(src []byte) (*cue.Context, cue.Value, error) {
	ctx := cuecontext.New()
	expr, err := parser.ParseFile("mission.cue", src, parser.ParseComments)
	if err != nil {
		return nil, cue.Value{}, fmt.Errorf("cue parse: %w", err)
	}
	v := ctx.BuildFile(expr)
	if err := v.Err(); err != nil {
		return nil, cue.Value{}, fmt.Errorf("cue build: %w", err)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return nil, cue.Value{}, fmt.Errorf("cue validate: %w", err)
	}
	return ctx, v, nil
}

// cuePath returns a cue.Path for the named field. Used by the
// mission loader to look up the conventional `mission:` key.
func cuePath(ctx *cue.Context, name string) cue.Path {
	return cue.MakePath(cue.Str(name))
}
