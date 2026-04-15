package ast_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/taskfile/ast"
)

func decodeTS(t *testing.T, yamlSrc string) *ast.Timestamps {
	t.Helper()
	var holder struct {
		Timestamps *ast.Timestamps `yaml:"timestamps"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(yamlSrc), &holder))
	return holder.Timestamps
}

func TestTimestampsDecodesTrueFalseString(t *testing.T) {
	t.Parallel()

	ts := decodeTS(t, "timestamps: true\n")
	require.NotNil(t, ts)
	assert.True(t, ts.IsSet())
	assert.True(t, ts.On())
	assert.Empty(t, ts.Format)

	ts = decodeTS(t, "timestamps: false\n")
	require.NotNil(t, ts)
	assert.True(t, ts.IsSet())
	assert.False(t, ts.On())

	ts = decodeTS(t, "timestamps: \"[%H:%M:%S]\"\n")
	require.NotNil(t, ts)
	assert.True(t, ts.On())
	assert.Equal(t, "[%H:%M:%S]", ts.Format)

	// Quoted "true" / "false" still behave as bools — users who need a
	// literal format containing only "true" are out of luck, and that's
	// fine; documented.
	ts = decodeTS(t, "timestamps: \"true\"\n")
	require.NotNil(t, ts)
	assert.True(t, ts.On())
	assert.Empty(t, ts.Format)
}

func TestTimestampsOmittedIsNil(t *testing.T) {
	t.Parallel()
	ts := decodeTS(t, "other: 1\n")
	assert.Nil(t, ts)
}

func TestTimestampsDeepCopyIndependent(t *testing.T) {
	t.Parallel()
	on := true
	orig := &ast.Timestamps{Enabled: &on, Format: "%H"}
	copy := orig.DeepCopy()
	require.NotNil(t, copy)
	// Mutating the copy's Enabled pointer must not affect orig.
	off := false
	copy.Enabled = &off
	assert.True(t, orig.On())
	assert.False(t, copy.On())
}
