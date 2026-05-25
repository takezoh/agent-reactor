package agent

// dynamicToolSpecs returns the SPEC §10.5 client-side tools advertised on
// thread/start. linear_graphql is advertised only when the Linear client is
// configured (tracker api_key + endpoint), since the orchestrator executes the
// tool on the agent's behalf via item/tool/call.
func (r *Runner) dynamicToolSpecs() []any {
	if r.LinearClient == nil {
		return nil
	}
	return []any{linearGraphQLToolSpec()}
}

// linearGraphQLToolSpec is the DynamicToolSpec for the linear_graphql tool.
// Shape (name/description/inputSchema) matches the codex thread/start
// dynamicTools entry; inputSchema is the §10.5 {query, variables} input.
func linearGraphQLToolSpec() map[string]any {
	return map[string]any{
		"name":        "linear_graphql",
		"description": "Execute a raw GraphQL query or mutation against Linear using the orchestrator's configured auth. Use this to read or transition issues (e.g. move to In Progress / Done), comment, and introspect the schema.",
		"inputSchema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"query"},
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "GraphQL query or mutation document to execute against Linear.",
				},
				"variables": map[string]any{
					"type":                 []any{"object", "null"},
					"description":          "Optional GraphQL variables object.",
					"additionalProperties": true,
				},
			},
		},
	}
}
