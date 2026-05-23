package lineargql

// White-box tests for the internal validation helpers and the GraphQL
// document scanner. The exported Execute behaviour is tested in client_test.go.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── countOperations ──────────────────────────────────────────────────────────

func TestCountOperations_singleNamedQuery(t *testing.T) {
	assert.Equal(t, 1, countOperations(`query Q { viewer { id } }`))
}

func TestCountOperations_singleNamedMutation(t *testing.T) {
	assert.Equal(t, 1, countOperations(`mutation M($id: ID!) { deleteIssue(id: $id) { success } }`))
}

func TestCountOperations_singleNamedSubscription(t *testing.T) {
	assert.Equal(t, 1, countOperations(`subscription S { issueCreated { id } }`))
}

func TestCountOperations_anonymousOperation(t *testing.T) {
	assert.Equal(t, 1, countOperations(`{ viewer { id } }`))
}

func TestCountOperations_multipleNamedOperations(t *testing.T) {
	src := `
		query A { viewer { id } }
		query B { teams { nodes { id } } }
	`
	assert.Equal(t, 2, countOperations(src))
}

func TestCountOperations_multipleAnonymousOperations(t *testing.T) {
	src := `{ viewer { id } } { teams { nodes { id } } }`
	assert.Equal(t, 2, countOperations(src))
}

func TestCountOperations_namedAndAnonymousMixed(t *testing.T) {
	src := `query A { viewer { id } } { teams { nodes { id } } }`
	assert.Equal(t, 2, countOperations(src))
}

func TestCountOperations_fragmentOnly(t *testing.T) {
	// Fragments are not operations; this is 0 operations.
	assert.Equal(t, 0, countOperations(`fragment F on Issue { id title }`))
}

func TestCountOperations_operationWithFragment(t *testing.T) {
	src := `
		fragment F on Issue { id title }
		query Q { issues { nodes { ...F } } }
	`
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_keywordsInsideStrings(t *testing.T) {
	// "query" and "mutation" inside a string literal must not be counted.
	src := `query Q { search(text: "query mutation subscription") { id } }`
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_keywordsInsideComment(t *testing.T) {
	src := "# query A\nquery Q { viewer { id } }"
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_keywordsInsideBlockString(t *testing.T) {
	src := `query Q { search(text: """query mutation""") { id } }`
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_variableDefaultWithObjectLiteral(t *testing.T) {
	// Object literal inside variable-definition parens must not be confused
	// with an anonymous operation or the operation's selection set.
	src := `query Q($v: InputType = {field: "value"}) { result { id } }`
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_nestedObjectLiteralInVariableDefault(t *testing.T) {
	src := `query Q($v: Complex = {nested: {x: 1}}) { result { id } }`
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_inlineFragmentsInsideBody(t *testing.T) {
	// Inline fragments (... on Type { }) inside the body should not add to count.
	src := `
		query Q {
			issues {
				nodes {
					... on Issue { id }
					... on Draft { title }
				}
			}
		}
	`
	assert.Equal(t, 1, countOperations(src))
}

func TestCountOperations_emptyDocument(t *testing.T) {
	assert.Equal(t, 0, countOperations(""))
}

func TestCountOperations_threeOperations(t *testing.T) {
	src := `
		query A { viewer { id } }
		query B { teams { nodes { id } } }
		mutation C { createIssue(input:{}) { issue { id } } }
	`
	assert.Equal(t, 3, countOperations(src))
}

// ── validateSingleOperation ──────────────────────────────────────────────────

func TestValidateSingleOperation_singleOp(t *testing.T) {
	assert.NoError(t, validateSingleOperation(`query Q { viewer { id } }`))
}

func TestValidateSingleOperation_multipleOps(t *testing.T) {
	src := `query A { viewer { id } } query B { teams { nodes { id } } }`
	err := validateSingleOperation(src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one")
}

// ── validateVariablesObject ───────────────────────────────────────────────────

func TestValidateVariablesObject_nilVariables(t *testing.T) {
	assert.NoError(t, validateVariablesObject(nil))
}

func TestValidateVariablesObject_emptyVariables(t *testing.T) {
	assert.NoError(t, validateVariablesObject(json.RawMessage("")))
}

func TestValidateVariablesObject_nullVariables(t *testing.T) {
	assert.NoError(t, validateVariablesObject(json.RawMessage("null")))
}

func TestValidateVariablesObject_validObject(t *testing.T) {
	assert.NoError(t, validateVariablesObject(json.RawMessage(`{"teamId":"T1"}`)))
}

func TestValidateVariablesObject_emptyObject(t *testing.T) {
	assert.NoError(t, validateVariablesObject(json.RawMessage(`{}`)))
}

func TestValidateVariablesObject_arrayRejected(t *testing.T) {
	err := validateVariablesObject(json.RawMessage(`["a","b"]`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON object")
}

func TestValidateVariablesObject_stringRejected(t *testing.T) {
	err := validateVariablesObject(json.RawMessage(`"hello"`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON object")
}

func TestValidateVariablesObject_numberRejected(t *testing.T) {
	err := validateVariablesObject(json.RawMessage(`42`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON object")
}

func TestValidateVariablesObject_booleanRejected(t *testing.T) {
	err := validateVariablesObject(json.RawMessage(`true`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON object")
}

func TestValidateVariablesObject_whitespaceBeforeObject(t *testing.T) {
	assert.NoError(t, validateVariablesObject(json.RawMessage("  \n\t{\"k\":1}")))
}
