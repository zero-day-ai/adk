# ADK top-level Makefile.
#
# The ADK contains two roots: cli/ (gibson CLI binary + tooling)
# and templates/ (mission templates dual-published as CUE + JSON +
# MDX). This Makefile drives the templates pipeline and cross-cutting
# checks (check-cue-fresh); use cli/Makefile for the CLI itself.

.PHONY: templates templates-export templates-vet templates-check \
	ensure-cue ensure-cli check check-cue-fresh generate regen-cue

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

# regen-cue: regenerate the embedded mission CUE schema in the gibson
# CLI from the SDK proto. Requires the SDK sibling clone at ../sdk and
# the cue binary on PATH. See scripts/regen-cue.sh for the pipeline
# (cue import proto + ADK-specific package/import normalization).
# Spec: zero-day-ai/adk#27.
regen-cue:
	@scripts/regen-cue.sh

# check-cue-fresh: drift gate for the ADK-embedded CUE schema.
# FAILS CI when the committed *_proto_gen.cue under
# cli/cmd/gibson/cmd/mission/schema/ has drifted from the SDK proto.
# Two modes (see scripts/check-cue-fresh.sh for the full contract):
#   FULL       — SDK sibling present: regen + byte-diff.
#   STRUCTURAL — SDK sibling absent : sentinel-header check only.
# Spec: zero-day-ai/adk#27 (mission-author-experience epic, M3).
check-cue-fresh:
	@scripts/check-cue-fresh.sh

# generate: ALIAS for the maintainer workflow that refreshes the
# embedded mission CUE schema. Failure of check-cue-fresh resolves
# by running `make generate` and committing the result.
generate: regen-cue

# check: top-level aggregate target. Use this as the local pre-push
# smoke test. Currently runs check-cue-fresh only; templates-check is
# tracked separately under zero-day-ai/adk#28 (pre-existing whitespace
# drift in committed template.json) and will be re-added to this
# aggregate once that issue lands.
check: check-cue-fresh
	@echo "check: ok"
