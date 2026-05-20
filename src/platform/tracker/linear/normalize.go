package linear

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/platform/tracker"
)

func normalizeIssue(n rawNode) (tracker.Issue, error) {
	createdAt, err := parseTime(n.CreatedAt)
	if err != nil {
		return tracker.Issue{}, fmt.Errorf("createdAt: %w", err)
	}
	updatedAt, err := parseTime(n.UpdatedAt)
	if err != nil {
		return tracker.Issue{}, fmt.Errorf("updatedAt: %w", err)
	}
	return tracker.Issue{
		ID:          n.ID,
		Identifier:  n.Identifier,
		Title:       n.Title,
		Description: n.Description,
		Priority:    normalizePriority(n.Priority),
		State:       n.State.Name,
		BranchName:  n.BranchName,
		URL:         n.URL,
		Labels:      normalizeLabels(n.Labels.Nodes),
		BlockedBy:   normalizeBlockers(n.InverseRelations.Nodes),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// normalizeLabels lowercases each label name (§11.3).
func normalizeLabels(nodes []rawLabel) []string {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]string, len(nodes))
	for i, l := range nodes {
		out[i] = strings.ToLower(l.Name)
	}
	return out
}

// normalizeBlockers derives blocked_by from inverse "blocks" relations (§11.3).
func normalizeBlockers(nodes []rawRelNode) []tracker.Blocker {
	var out []tracker.Blocker
	for _, n := range nodes {
		if n.Type != "blocks" {
			continue
		}
		out = append(out, tracker.Blocker{
			ID:         n.Issue.ID,
			Identifier: n.Issue.Identifier,
			State:      n.Issue.State.Name,
		})
	}
	return out
}

// normalizePriority returns an *int only when the value is a whole number (§11.3).
func normalizePriority(f *float64) *int {
	if f == nil {
		return nil
	}
	if *f != math.Trunc(*f) {
		return nil
	}
	v := int(*f)
	return &v
}

// parseTime parses ISO-8601 timestamps from Linear (§11.3).
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	return t, err
}
