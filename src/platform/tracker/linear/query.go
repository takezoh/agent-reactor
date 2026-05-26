package linear

// nodeFields is the shared GraphQL selection set for Issue nodes.
// Isolation point for §11.2 "keep query construction isolated".
const nodeFields = `
      id identifier title description priority url branchName
      state { name }
      labels { nodes { name } }
      project { name content }
      inverseRelations { nodes { type issue { id identifier state { name } } } }
      createdAt updatedAt`

// projectStateQuery fetches issues filtered by project slugs and state names.
// Used for both FetchCandidateIssues (active states) and FetchIssuesByStates.
const projectStateQuery = `
query($slugs: [String!]!, $states: [String!]!, $after: String) {
  issues(
    filter: {
      project: { slugId: { in: $slugs } }
      state: { name: { in: $states } }
    }
    first: 50
    after: $after
  ) {
    pageInfo { hasNextPage endCursor }
    nodes {` + nodeFields + `
    }
  }
}`

// byIDsQuery fetches issues by their GraphQL IDs.
// §11.2 specifies the variable type as [ID!] (nullable list).
const byIDsQuery = `
query($ids: [ID!], $after: String) {
  issues(
    filter: { id: { in: $ids } }
    first: 50
    after: $after
  ) {
    pageInfo { hasNextPage endCursor }
    nodes {` + nodeFields + `
    }
  }
}`

func projectStateVars(slugs []string, states []string, after string) map[string]any {
	m := map[string]any{"slugs": slugs, "states": states}
	if after != "" {
		m["after"] = after
	}
	return m
}

func byIDsVars(ids []string, after string) map[string]any {
	m := map[string]any{"ids": ids}
	if after != "" {
		m["after"] = after
	}
	return m
}

// Raw GraphQL response types, tightly coupled to the queries above.

type rawPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type rawState struct {
	Name string `json:"name"`
}

type rawLabel struct {
	Name string `json:"name"`
}

type rawLabelConn struct {
	Nodes []rawLabel `json:"nodes"`
}

type rawProject struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type rawIssueRef struct {
	ID         string   `json:"id"`
	Identifier string   `json:"identifier"`
	State      rawState `json:"state"`
}

type rawRelNode struct {
	Type  string      `json:"type"`
	Issue rawIssueRef `json:"issue"`
}

type rawRelConn struct {
	Nodes []rawRelNode `json:"nodes"`
}

type rawNode struct {
	ID               string       `json:"id"`
	Identifier       string       `json:"identifier"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	Priority         *float64     `json:"priority"`
	URL              string       `json:"url"`
	BranchName       string       `json:"branchName"`
	State            rawState     `json:"state"`
	Labels           rawLabelConn `json:"labels"`
	Project          rawProject   `json:"project"`
	InverseRelations rawRelConn   `json:"inverseRelations"`
	CreatedAt        string       `json:"createdAt"`
	UpdatedAt        string       `json:"updatedAt"`
}

type rawIssuesConn struct {
	PageInfo rawPageInfo `json:"pageInfo"`
	Nodes    []rawNode   `json:"nodes"`
}
