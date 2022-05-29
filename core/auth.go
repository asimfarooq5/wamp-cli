package core

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/nexus/v3/wamp/crsign"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/pbkdf2"
)

func getAnonymousAuthConfig(realm string, serializer serialize.Serialization, authid string,
	authrole string) client.Config {

	helloDict := wamp.Dict{}
	if authid != "" {
		helloDict["authid"] = authid
	}

	if authrole != "" {
		helloDict["authrole"] = authrole
	}

	cfg := client.Config{
		Realm:         realm,
		Logger:        logger,
		HelloDetails:  helloDict,
		Serialization: serializer,
	}

	return cfg
}

func getTicketAuthConfig(realm string, serializer serialize.Serialization, authid string, authrole string,
	ticket string) client.Config {

	helloDict := wamp.Dict{}
	if authid != "" {
		helloDict["authid"] = authid
	}

	if authrole != "" {
		helloDict["authrole"] = authrole
	}

	cfg := client.Config{
		Realm:        realm,
		Logger:       logger,
		HelloDetails: helloDict,
		AuthHandlers: map[string]client.AuthFunc{
			"ticket": func(c *wamp.Challenge) (string, wamp.Dict) {
				return ticket, wamp.Dict{}
			},
		},
		Serialization: serializer,
	}

	return cfg
}

func getCRAAuthConfig(realm string, serializer serialize.Serialization, authid string, authrole string,
	secret string) client.Config {

	helloDict := wamp.Dict{}
	if authid != "" {
		helloDict["authid"] = authid
	}

	if authrole != "" {
		helloDict["authrole"] = authrole
	}

	cfg := client.Config{
		Realm:        realm,
		Logger:       logger,
		HelloDetails: helloDict,
		AuthHandlers: map[string]client.AuthFunc{
			"wampcra": func(c *wamp.Challenge) (string, wamp.Dict) {
				ch, _ := wamp.AsString(c.Extra["challenge"])
				// If the client needed to lookup a user's key, this would require decoding
				// the JSON-encoded challenge string and getting the authid.  For this
				// example assume that client only operates as one user and knows the key
				// to use.
				saltStr, _ := wamp.AsString(c.Extra["salt"])
				// If no salt given, use raw password as key.
				if saltStr == "" {
					return crsign.SignChallenge(ch, []byte(secret)), wamp.Dict{}
				}

				// If salting info give, then compute a derived key using PBKDF2.
				salt := []byte(saltStr)
				iters, _ := wamp.AsInt64(c.Extra["iterations"])
				keylen, _ := wamp.AsInt64(c.Extra["keylen"])

				if iters == 0 {
					iters = 1000
				}
				if keylen == 0 {
					keylen = 32
				}

				// Compute derived key.
				dk := pbkdf2.Key([]byte(secret), salt, int(iters), int(keylen), sha256.New)
				// Get base64 bytes. see https://github.com/gammazero/nexus/issues/252
				derivedKey := []byte(base64.StdEncoding.EncodeToString(dk))

				return crsign.SignChallenge(ch, derivedKey), wamp.Dict{}
			},
		},
		Serialization: serializer,
	}

	return cfg
}

func getCryptosignAuthConfig(realm string, serializer serialize.Serialization, authid string, authrole string,
	privateKey string) client.Config {
	helloDict := wamp.Dict{}
	if authid != "" {
		helloDict["authid"] = authid
	}

	if authrole != "" {
		helloDict["authrole"] = authrole
	}

	privkey, _ := hex.DecodeString(privateKey)
	var pvk ed25519.PrivateKey

	if len(privkey) == 32 {
		pvk = ed25519.NewKeyFromSeed(privkey)
	} else if len(privkey) == 64 {
		pvk = ed25519.NewKeyFromSeed(privkey[:32])
	} else {
		logger.Fatal("Invalid private key. Cryptosign private key must be either 32 or 64 characters long")
	}

	key := pvk.Public().(ed25519.PublicKey)
	publicKey := hex.EncodeToString(key)
	helloDict["authextra"] = wamp.Dict{"pubkey": publicKey}

	cfg := client.Config{
		Realm:        realm,
		Logger:       logger,
		HelloDetails: helloDict,
		AuthHandlers: map[string]client.AuthFunc{
			"cryptosign": func(c *wamp.Challenge) (string, wamp.Dict) {
				challengeHex, _ := wamp.AsString(c.Extra["challenge"])
				challengeBytes, _ := hex.DecodeString(challengeHex)

				signed := ed25519.Sign(pvk, challengeBytes)
				signedHex := hex.EncodeToString(signed)
				result := signedHex + challengeHex
				return result, wamp.Dict{}
			},
		},
		Serialization: serializer,
	}

	return cfg
}