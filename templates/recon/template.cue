// Recon mission template.
//
// Discover the target's exposed surface (open ports, running
// services, reachable subdomains). Two agent nodes run
// sequentially: nmap-style scan followed by enrichment via
// passive sources.
//
// Override before submitting:
//   target_ref: "<target-name-or-id>"
//
// Spec: mission-authoring-cue Requirement 7.

mission: {
	name:        "recon"
	description: "Reconnaissance across a target's exposed surface."
	version:     "1.0.0"
	target_ref:  ""

	nodes: {
		scan: {
			id:   "scan"
			type: "NODE_TYPE_AGENT"
			agent_config: {
				agent_name: "nmap-agent"
			}
		}
		enrich: {
			id:   "enrich"
			type: "NODE_TYPE_AGENT"
			agent_config: {
				agent_name: "shodan-agent"
			}
		}
	}
	edges: [
		{from: "scan", to: "enrich"},
	]
	entry_points: ["scan"]
	exit_points: ["enrich"]
}
