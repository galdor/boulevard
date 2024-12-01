package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestControlAPIStatus(t *testing.T) {
	require := require.New(t)

	c := testControlAPIClient(t)

	var res *http.Response
	var err error
	var status ServiceStatus

	status = ServiceStatus{}
	res, err = c.Call("status", nil, &status)
	require.NoError(err)
	require.Equal(200, res.StatusCode)
	require.Equal(testService.Cfg.BuildId, status.BuildId)
}
