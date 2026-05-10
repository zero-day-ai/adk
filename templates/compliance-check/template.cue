// Cloud-config compliance check.
//
// Single-agent mission auditing cloud configuration against a
// policy baseline (e.g., CIS benchmarks for AWS / GCP / Azure).
//
// Override before submitting:
//   target_ref: "<cloud-target-name-or-id>"
//
// Spec: mission-authoring-cue Requirement 7.

mission: {
	name:        "compliance-check"
	description: "Audit cloud configuration against a policy baseline."
	version:     "1.0.0"
	target_ref:  ""

	nodes: {
		inspect: {
			id:   "inspect"
			type: "NODE_TYPE_AGENT"
			agent_config: {
				agent_name: "compliance-agent"
			}
		}
	}
	entry_points: ["inspect"]
	exit_points: ["inspect"]
}
