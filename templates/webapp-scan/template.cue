// Web application discovery + vulnerability scan.
//
// Crawl the target webapp's reachable surface, then run an
// active vulnerability scan against discovered endpoints.
//
// Override before submitting:
//   target_ref: "<target-name-or-id>"
//
// Spec: mission-authoring-cue Requirement 7.

mission: {
	name:        "webapp-scan"
	description: "Crawl + active scan a web application."
	version:     "1.0.0"
	target_ref:  ""

	nodes: {
		crawl: {
			id:   "crawl"
			type: "NODE_TYPE_AGENT"
			agent_config: {
				agent_name: "webcrawl-agent"
			}
		}
		scan: {
			id:   "scan"
			type: "NODE_TYPE_AGENT"
			agent_config: {
				agent_name: "webvuln-agent"
			}
		}
	}
	edges: [
		{from: "crawl", to: "scan"},
	]
	entry_points: ["crawl"]
	exit_points: ["scan"]
}
