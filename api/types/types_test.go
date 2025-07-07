package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSelectionOptions(t *testing.T) {
	op1, err := NewSelectionOptions(WithHosts("*"))
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "*", op1.ToText())

	op2, err := NewSelectionOptions(WithHosts("*"), WithList([]string{"node1", "node2"}))
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "* and node1,node2", op2.ToText())

	op3, err := NewSelectionOptions(WithHostRegex("node[0-9]+"), WithGrains("os", "CentOS", false),
		WithGrainsRegex("arch", "x86_64"))
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "E@node[0-9]+ or G@os:CentOS and P@arch:x86_64", op3.ToText())

	_, err = NewSelectionOptions()
	if !assert.Error(t, err) {
		return
	}

	op4, err := NewSelectionOptions(WithList([]string{"node1", "node2"}), WithPillar("role", "dev", false), WithPillarRegex("os", "CentOS.*"))
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "node1,node2 or I@role:dev and J@os:CentOS.*", op4.ToText())

	op5, err := NewSelectionOptions(WithHostGroup([]string{"dev"}), WithIPCidr("192.168.0.0/24", false), WithRange("%cloud"))
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "N@dev or S@192.168.0.0/24 and R@%cloud", op5.ToText())
}

func TestParseSelection(t *testing.T) {
	text1 := "*"
	opt1, err := ParseSelection(text1)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "*", opt1.ToText())

	text2 := "node1,node2"
	opt2, err := ParseSelection(text2)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "node1,node2", opt2.ToText())

	text3 := "N@dev or S@192.168.0.0/24 and R@%cloud"
	opt3, err := ParseSelection(text3)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, text3, opt3.ToText())

	text4 := "node1,node2 and E@node[0-9]+ and N@dev and I@role:dev and J@os:CentOS*"
	opt4, err := ParseSelection(text4)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, text4, opt4.ToText())
}
