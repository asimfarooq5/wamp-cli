package testutil

import (
	"testing"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/router"
	"github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const TestRealm = "wick.test"

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

func NewTestClient(t *testing.T, r router.Router) *client.Client {
	clientConfig := &client.Config{
		Realm: TestRealm,
	}
	c, err := client.ConnectLocal(r, *clientConfig)
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func ConnectedTestClients(t *testing.T) (*client.Client, *client.Client) {
	r := NewTestRouter(t, TestRealm)

	c1 := NewTestClient(t, r)

	c2 := NewTestClient(t, r)

	return c1, c2
}
