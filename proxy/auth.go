package proxy

import (
	b64 "encoding/base64"
	"fmt"
	"strings"

	uuid "github.com/google/uuid"
)

// Auth checks if a new name space should be provided given a code
type AuthClient struct {
	newUserCode string
	store       Cache
	namespace   string
	isDisabled  bool
}

func NewAuth(config Config) AuthClient {

	a := AuthClient{newUserCode: config.Code, isDisabled: config.DisableAuth}
	a.store = NewRedisClient(config.keyTimeout, config.redisUrl)
	a.namespace = "auth"
	return a
}

func (a *AuthClient) CreateNewUser(code string, inviteCode string) (*string, error) {
	if code == a.newUserCode {
		// check to see if invite code exists
		id, err := a.store.Get(code)
		if err != nil {
			return nil, err
		}
		if id != nil {
			// this means this person has already created this code
			// return error
			return nil, fmt.Errorf("User from invite code %s already exists", code)
		}

		// create a uuid for this new user and stash it
		nuuid := uuid.Parse(inviteCode)
		a.store.Put(a.GenKey(inviteCode), nuuid)
		return nuuid, nil
	}
	return nil, fmt.Errorf("Invalid Code from your client: %s", code)
}

// Authenticate checks the base64 string against stored credentials to give a person access
// data should follow Basic convention: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization
// where the string is encoded in base64 and the decoded content has inviteCode:accessKey
// If the access key is invalid it will return False and nil
// If it is valid it will return the inviteCode
func (a *AuthClient) Authenticate(input string) (bool, *string) {
	// split from word basic
	c := strings.Fields(input)
	base64 := c[len(c)-1]
	//decode
	ua, err := b64.URLEncoding.DecodeString(base64)
	if err != nil {
		return false, nil
	}

	// split by colon
	s := strings.Split(ua, ":")
	inviteCode := s[0]
	accessKey := s[1]

	ak, err := a.store.Get(a.GenKey(inviteCode))
	if err != nil {
		return false, nil
	}
	if ak == nil {
		return false, nil
	}
	return *ak == accessKey, &inviteCode
}

func (a *AuthClient) GenKey(key string) string {
	return fmt.Sprintf("%s.%s", a.namespace, key)
}
