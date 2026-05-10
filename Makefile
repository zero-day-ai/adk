# ADK top-level Makefile.
#
# The ADK contains two roots: cli/ (gibson CLI binary + tooling)
# and templates/ (mission templates dual-published as CUE + JSON +
# MDX). This Makefile drives the templates pipeline; use
# cli/Makefile for the CLI itself.

.PHONY: templates templates-export templates-vet templates-check ensure-cue

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
# from its template.cue source.
# Spec: mission-authoring-cue Requirement 7.
templates-export: ensure-cue
	@for t in $(TEMPLATES); do \
		echo "exporting templates/$$t/template.json"; \
		cue export templates/$$t/template.cue --out json -e mission > templates/$$t/template.json; \
	done

# templates-vet: assert each template.cue concretely evaluates
# (no unresolved fields, no contradictions). Run by CI on every PR.
templates-vet: ensure-cue
	@for t in $(TEMPLATES); do \
		echo "vet templates/$$t/template.cue"; \
		cue vet templates/$$t/template.cue || exit 1; \
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
