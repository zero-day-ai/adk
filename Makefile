# ADK top-level Makefile.
#
# The ADK contains two roots: cli/ (gibson CLI binary + tooling)
# and templates/ (mission templates dual-published as CUE + JSON +
# MDX). This Makefile drives the templates pipeline; use
# cli/Makefile for the CLI itself.

.PHONY: templates templates-export templates-vet templates-check ensure-cue ensure-cli

# Templates ship as triplets: template.cue (authoring source),
# template.json (cue export output, committed for dashboard
# consumption), template.mdx (handwritten description).
TEMPLATES := recon webapp-scan secrets-audit compliance-check

# ensure-cue: fail-fast guard for the cue binary.
ensure-cue:
	@command -v cue >/dev/null 2>&1 || { \
		echo "ERROR: cue binary not found on PATH." >&2; \
		echo "  Install with: go install cuelang.org/go/cmd/cue@v0.16.1" >&2; \
		exit 1; \
	}

# templates-export: regenerate template.json for every template
# from its template.cue source via the gibson CLI (schema-aware,
# single code path). Produces proto-shaped JSON (camelCase keys)
# matching the format the daemon and dashboard consume.
# Spec: mission-authoring-cue Requirement 7.
templates-export: ensure-cli
	@for t in $(TEMPLATES); do \
		echo "exporting templates/$$t/template.json"; \
		./cli/bin/gibson mission render templates/$$t/template.cue > templates/$$t/template.json; \
	done

# ensure-cli: build the gibson CLI binary used for schema-aware vet.
ensure-cli:
	@(cd cli && go build -o bin/gibson ./cmd/gibson)

# templates-vet: assert each template.cue is structurally valid by
# running it through the gibson CLI validator, which resolves the SDK
# CUE schema via the embedded overlay (single code path, no raw cue).
templates-vet: ensure-cli
	@for t in $(TEMPLATES); do \
		echo "validate templates/$$t/template.cue"; \
		./cli/bin/gibson mission validate templates/$$t/template.cue || exit 1; \
	done

# templates-check: regenerate template.json files and assert
# `git diff --exit-code`. PRs that change template.cue without
# regenerating template.json fail.
# Spec: mission-authoring-cue Requirement 9 (drift gate).
templates-check: templates-vet templates-export
	@git diff --exit-code templates/ || { \
		echo "ERROR: template.json drifted from template.cue." >&2; \
		echo "  Run \`make templates-export\` and commit the result." >&2; \
		exit 1; \
	}
	@echo "templates-check: ok"

# templates: alias the most common workflow (vet + export).
templates: templates-vet templates-export
