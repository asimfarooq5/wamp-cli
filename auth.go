package main

import (
	"errors"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/nexus/v3/wamp/crsign"
	"net/http"
)

type testKeyStore struct {
	provider string
	secret   string
	ticket   string
	cookie   *http.Cookie

	authByCookie bool
}

var goodSecret string

var tks = &testKeyStore{
	provider: "static",
	secret:   goodSecret,
}

func (ks *testKeyStore) AuthKey(authid, authmethod string) ([]byte, error) {
	if authid != "test" {
		return nil, errors.New("no such user: " + authid)
	}
	switch authmethod {
	case "wampcra":
		// Lookup the user's key.
		return []byte(ks.secret), nil
		//case "ticket":
		//  return []byte(ks.ticket), nil
	}
	return nil, errors.New("unsupported authmethod")
}

func (ks *testKeyStore) AuthRole(authid string) (string, error) {
	if authid != "test" {
		return "", errors.New("no such user: " + authid)
	}
	return "user", nil
}

func (ks *testKeyStore) PasswordInfo(authid string) (string, int, int) {
	return "", 0, 0
}

func (ks *testKeyStore) Provider() string { return ks.provider }

func (ks *testKeyStore) AlreadyAuth(authid string, details wamp.Dict) bool {
	v, err := wamp.DictValue(details, []string{"transport", "auth", "cookie"})
	if err != nil {
		return false
	}
	cookie := v.(*http.Cookie)

	if cookie.Value == ks.cookie.Value {
		ks.authByCookie = true
		return true
	}
	return false
}

func clientAuthFunc(c *wamp.Challenge) (string, wamp.Dict) {
	var sig string
	switch c.AuthMethod {
	case "wampcra":
		sig = crsign.RespondChallenge(goodSecret, c, nil)

		details := wamp.Dict{}
		details["authid"] = "test"
		nextCookie := &http.Cookie{Name: "nexus-wamp-cookie", Value: "a1b2c3"}
		authDict := wamp.Dict{"nextcookie": nextCookie}
		details["transport"] = wamp.Dict{"auth": authDict}

	}
	return sig, wamp.Dict{}
}
