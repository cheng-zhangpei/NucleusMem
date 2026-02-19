package memspace

import (
	"NucleusMem/pkg/configs"
	"github.com/stretchr/testify/assert"
	"testing"
)

// test the conn with the kv and the unit test of the memspace
func TestMemSpace_Basic(t *testing.T) {
	filepath := "../../configs/file/memspace_1001.yaml"
	config, err := configs.LoadMemSpaceConfigFromYAML(filepath)
	assert.NoError(t, err)
	memspace, err := NewMemSpace(config)
	assert.NoError(t, err)
	httpServer := NewMemSpaceHTTPServer(memspace)
	err = httpServer.Start()
}
