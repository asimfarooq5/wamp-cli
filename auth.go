package main

import (
	"errors"
	"github.com/gammazero/nexus/router/auth"
	"github.com/gammazero/nexus/transport"

	//"github.com/gammazero/nexus/transport"
	"github.com/gammazero/nexus/wamp"
	"net/http"
	"testing"
	"time"
)

type testKeyStore struct {
	provider string
	secret   string
	cookie   *http.Cookie

	authByCookie bool
}
const (
	goodSecret = "test"
)
var tks = &testKeyStore{
	provider: "static",
	secret:   goodSecret,
	cookie: nil,
	authByCookie: true,
}

func (ks *testKeyStore) AuthKey(authid, authmethod string) ([]byte, error) {
	if authid != "test" {
		return nil, errors.New("no such user: " + authid)
	}
	switch authmethod {
	case "wampcra":
		// Lookup the user's key.
		return []byte("pass"), nil
	}
	return nil, nil
}

func (ks *testKeyStore) PasswordInfo(authid string) (string, int, int) {
	return "", 0, 0
}

func (ks *testKeyStore) Provider() string { return ks.provider }

func (ks *testKeyStore) AuthRole(authid string) (string, error) {
	if authid != "test" {
		return "", errors.New("no such user: " + authid)
	}
	return "main", nil
}


func TestCRAuth(t *testing.T) {
	cp, rp := transport.LinkedPeers()
	defer cp.Close()
	defer rp.Close()

	crAuth := auth.NewCRAuthenticator(tks, time.Second)
	sid := wamp.ID(212)

	// Test with missing authid
	details := wamp.Dict{}
	welcome, err := crAuth.Authenticate(sid, details, rp)
	if err == nil {
		t.Fatal("expected error with missing authid")
	}

	// Test with unknown authid.
	details["authid"] = "unknown"
	welcome, err = crAuth.Authenticate(sid, details, rp)
	if err == nil {
		t.Fatal("expected error from unknown authid")
	}

	// Test with known authid.
	details["authid"] = "jdoe"
	nextCookie := &http.Cookie{Name: "nexus-wamp-cookie", Value: "a1b2c3"}
	authDict := wamp.Dict{"nextcookie": nextCookie}
	details["transport"] = wamp.Dict{"auth": authDict}

	welcome, err = crAuth.Authenticate(sid, details, rp)
	if err != nil {
		t.Fatal("challenge failed: ", err.Error())
	}
	if welcome == nil {
		t.Fatal("received nil welcome msg")
	}
	if welcome.MessageType() != wamp.WELCOME {
		t.Fatal("expected WELCOME message, got: ", welcome.MessageType())
	}
	if s, _ := wamp.AsString(welcome.Details["authmethod"]); s != "wampcra" {
		t.Fatal("invalid authmethod in welcome details")
	}
	if s, _ := wamp.AsString(welcome.Details["authrole"]); s != "user" {
		t.Fatal("incorrect authrole in welcome details")
	}

	tks.secret = "bad"

	// Test with bad ticket.
	details["authid"] = "jdoe"
	welcome, err = crAuth.Authenticate(sid, details, rp)
	if err == nil {
		t.Fatal("expected error with bad key")
	}

	authDict["cookie"] = &http.Cookie{Name: "nexus-wamp-cookie", Value: "a1b2c3"}
	authDict["nextcookie"] = &http.Cookie{Name: "nexus-wamp-cookie", Value: "xyz123"}
	welcome, err = crAuth.Authenticate(sid, details, rp)
	if err != nil {
		t.Fatal("challenge failed: ", err.Error())
	}}