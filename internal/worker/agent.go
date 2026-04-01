package worker

// CoderAgentName is the default worker agent name. Used as fallback when
// SpawnConfig.AgentName is empty. The runtime value comes from the pipeline
// stage assignee in pipelines.toml.
const CoderAgentName = "code-lead"
