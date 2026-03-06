package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissionCheckerImpl_Empty(t *testing.T) {
	pc := &PermissionCheckerImpl{
		namespace: "default",
	}

	assert.NotNil(t, pc)
	assert.Equal(t, "default", pc.namespace)
}

func TestPermissionChecker_Interface(t *testing.T) {
	pc := &PermissionCheckerImpl{
		namespace: "default",
	}

	// Verify it implements the interface
	var _ PermissionChecker = pc
}
