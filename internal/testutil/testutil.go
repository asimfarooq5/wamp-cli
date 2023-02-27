package testutil

import (
	"testing"

	"github.com/gammazero/nexus/v3/router"
	"github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func NewTestRouter(t *testing.T, realm string) router.Router {
	realmConfig := &router.RealmConfig{
		URI:           wamp.URI(realm),
		AnonymousAuth: true,
	}
	config := &router.Config{
		RealmConfigs: []*router.RealmConfig{realmConfig},
	}
	rout, err := router.NewRouter(config, log.New())
	require.NoError(t, err)

	return rout
}
