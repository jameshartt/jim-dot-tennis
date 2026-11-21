// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// softAuthenticator is a software passkey authenticator plus the browser
// around it, for tests: it holds ES256 keys and answers the JSON options the
// server sends with the JSON a browser would post back (attestation "none",
// discoverable credentials, user verification performed).
type softAuthenticator struct {
	t      *testing.T
	origin string // what the "browser" reports as the page's origin
	rpID   string // "" means use the one in the options
	creds  []*softCredential

	backupEligible bool // passkey provider that syncs (sets BE and BS)
	// nextID, when set, is used as the next credential's id instead of a
	// random one.
	nextID []byte
}

type softCredential struct {
	id         []byte
	key        *ecdsa.PrivateKey
	rpID       string
	userHandle []byte
	counter    uint32 // 0 forever when the authenticator has no counter
	counts     bool
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func unb64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("bad base64url %q: %v", s, err)
	}
	return b
}

// creationOptions is the part of the server's creation options a browser uses.
type creationOptions struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		RP        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"rp"`
		User struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"user"`
		ExcludeCredentials []struct {
			ID string `json:"id"`
		} `json:"excludeCredentials"`
		AuthenticatorSelection struct {
			ResidentKey        string `json:"residentKey"`
			RequireResidentKey bool   `json:"requireResidentKey"`
			UserVerification   string `json:"userVerification"`
		} `json:"authenticatorSelection"`
		Attestation string `json:"attestation"`
		Timeout     int    `json:"timeout"`
	} `json:"publicKey"`
}

// requestOptions is the part of the server's request options a browser uses.
type requestOptions struct {
	PublicKey struct {
		Challenge        string `json:"challenge"`
		RPID             string `json:"rpId"`
		Timeout          int    `json:"timeout"`
		UserVerification string `json:"userVerification"`
		AllowCredentials []struct {
			Type       string   `json:"type"`
			ID         string   `json:"id"`
			Transports []string `json:"transports"`
		} `json:"allowCredentials"`
	} `json:"publicKey"`
}

func (a *softAuthenticator) clientData(ceremony, challenge string) []byte {
	data, err := json.Marshal(map[string]any{"type": ceremony, "challenge": challenge, "origin": a.origin, "crossOrigin": false})
	if err != nil {
		a.t.Fatal(err)
	}
	return data
}

func (a *softAuthenticator) flags(attested bool) byte {
	flags := byte(0x01 | 0x04) // user present, user verified
	if a.backupEligible {
		flags |= 0x08 | 0x10
	}
	if attested {
		flags |= 0x40
	}
	return flags
}

// create answers navigator.credentials.create(): it makes a new credential
// for the options' user and returns the JSON the page posts to the server.
func (a *softAuthenticator) create(optionsJSON []byte, counts bool) ([]byte, *softCredential) {
	a.t.Helper()
	var opts creationOptions
	if err := json.Unmarshal(optionsJSON, &opts); err != nil {
		a.t.Fatalf("creation options: %v\n%s", err, optionsJSON)
	}
	rpID := a.rpID
	if rpID == "" {
		rpID = opts.PublicKey.RP.ID
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		a.t.Fatal(err)
	}
	id := a.nextID
	a.nextID = nil
	if id == nil {
		id = make([]byte, 20)
		if _, err := rand.Read(id); err != nil {
			a.t.Fatal(err)
		}
	}
	cred := &softCredential{id: id, key: key, rpID: rpID, userHandle: unb64(a.t, opts.PublicKey.User.ID), counts: counts}
	if counts {
		cred.counter = 1
	}

	ecdhKey, err := key.PublicKey.ECDH()
	if err != nil {
		a.t.Fatal(err)
	}
	point := ecdhKey.Bytes() // 0x04 || X || Y
	cose, err := webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: 2, Algorithm: -7},
		Curve:         1,
		XCoord:        point[1:33],
		YCoord:        point[33:65],
	})
	if err != nil {
		a.t.Fatal(err)
	}

	rpHash := sha256.Sum256([]byte(rpID))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, a.flags(true))
	authData = binary.BigEndian.AppendUint32(authData, cred.counter)
	authData = append(authData, make([]byte, 16)...) // AAGUID: all zero
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(id)))
	authData = append(authData, id...)
	authData = append(authData, cose...)

	attObj, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
	if err != nil {
		a.t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"id": b64(id), "rawId": b64(id), "type": "public-key",
		"authenticatorAttachment": "platform",
		"clientExtensionResults":  map[string]any{},
		"response": map[string]any{
			"clientDataJSON":    b64(a.clientData("webauthn.create", opts.PublicKey.Challenge)),
			"attestationObject": b64(attObj),
			"transports":        []string{"internal", "hybrid"},
		},
	})
	if err != nil {
		a.t.Fatal(err)
	}
	a.creds = append(a.creds, cred)
	return body, cred
}

// get answers navigator.credentials.get() with cred (or, when cred is nil,
// the first credential it holds that the options allow).
func (a *softAuthenticator) get(optionsJSON []byte, cred *softCredential) []byte {
	a.t.Helper()
	var opts requestOptions
	if err := json.Unmarshal(optionsJSON, &opts); err != nil {
		a.t.Fatalf("request options: %v\n%s", err, optionsJSON)
	}
	if cred == nil {
		for _, c := range a.creds {
			if len(opts.PublicKey.AllowCredentials) == 0 {
				cred = c
				break
			}
			for _, allowed := range opts.PublicKey.AllowCredentials {
				if allowed.ID == b64(c.id) {
					cred = c
				}
			}
		}
	}
	if cred == nil {
		a.t.Fatal("authenticator holds no credential the options allow")
	}
	if cred.counts {
		cred.counter++
	}
	return a.assertion(opts.PublicKey.Challenge, cred, cred.counter)
}

// assertion signs challenge with cred, reporting the given counter.
func (a *softAuthenticator) assertion(challenge string, cred *softCredential, counter uint32) []byte {
	a.t.Helper()
	rpID := a.rpID
	if rpID == "" {
		rpID = cred.rpID
	}
	rpHash := sha256.Sum256([]byte(rpID))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, a.flags(false))
	authData = binary.BigEndian.AppendUint32(authData, counter)

	clientData := a.clientData("webauthn.get", challenge)
	clientHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte{}, authData...), clientHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, cred.key, digest[:])
	if err != nil {
		a.t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"id": b64(cred.id), "rawId": b64(cred.id), "type": "public-key",
		"clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON":    b64(clientData),
			"authenticatorData": b64(authData),
			"signature":         b64(sig),
			"userHandle":        b64(cred.userHandle),
		},
	})
	if err != nil {
		a.t.Fatal(err)
	}
	return body
}
