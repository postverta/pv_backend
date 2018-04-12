package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// User is kind of special, as it is not stored locally, but in the Auth0 service.
// We use the management API to read it.

type UserClient struct {
	AccessToken string
	Mutex       sync.Mutex
}

func (uc *UserClient) RenewToken() error {
	data := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     "fT5pAAd7D2kT67I3xZh4rF8ZlB7rwCUP",
		"client_secret": "dqrGbAXea6PHRPCKdVmdUkGrgvoyeLZ55wSAsm6gxZDw1ixZX-_faNysaJ6mhxPM",
		"audience":      "https://postverta.auth0.com/api/v2/",
	}
	url := "https://postverta.auth0.com/oauth/token"
	buf, _ := json.Marshal(data)
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error:%s", resp.Status)
	}

	tokenData := make(map[string]interface{})
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&tokenData)
	if err != nil {
		return err
	}

	token, ok := tokenData["access_token"].(string)
	if !ok {
		return fmt.Errorf("Cannot access access_token field:%v", tokenData)
	}

	uc.AccessToken = token
	return nil
}

func (uc *UserClient) GetUser(id string) (*User, error) {
	uc.Mutex.Lock()
	if uc.AccessToken == "" {
		err := uc.RenewToken()
		if err != nil {
			uc.Mutex.Unlock()
			return nil, err
		}
	}
	uc.Mutex.Unlock()

	client := &http.Client{}
	trials := 0
	for {
		if trials > 3 {
			return nil, fmt.Errorf("Too many retries")
		}
		trials++

		url := fmt.Sprintf("https://postverta.auth0.com/api/v2/users/%s", id)
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("authorization", fmt.Sprintf("Bearer %s", uc.AccessToken))
		req.Header.Add("content-type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 200 {
			dec := json.NewDecoder(resp.Body)
			user := &User{}
			err = dec.Decode(user)
			resp.Body.Close()
			if err != nil {
				return nil, err
			} else {
				return user, nil
			}
		} else if resp.StatusCode == 401 {
			resp.Body.Close()
			uc.Mutex.Lock()
			err = uc.RenewToken()
			if err != nil {
				uc.Mutex.Unlock()
				return nil, err
			}
			uc.Mutex.Unlock()
			// retry after renewing token
		} else if resp.StatusCode != 429 {
			resp.Body.Close()
			return nil, fmt.Errorf("API error:%s", resp.Status)
		}
	}
}
