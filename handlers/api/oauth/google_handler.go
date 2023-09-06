package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthProvider struct {
	name                string
	state               string
	codeVerifier        string
	codeChallenge       string
	codeChallengeMethod string
	authUrl             string
}

type GoogleResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Verified bool   `json:"verified_email"`
	Picture  string `json:"picture"`
}

func configGoogle() *oauth2.Config {
	clientId := os.Getenv("OAUTH2_GOOGLE_CLIENTID")
	clientSecret := os.Getenv("OAUTH2_GOOGLE_CLIENTSECRET")
	redirectUrl := os.Getenv("OAUTH2_GOOGLE_REDIRECTURL")

	conf := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUrl,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}
	return conf
}

func GetEmail(token string) string {
	reqURL, err := url.Parse("https://www.googleapis.com/oauth2/v1/userinfo")

	if err != nil {
		return err.Error()
	}

	ptoken := fmt.Sprintf("Bearer %s", token)
	res := &http.Request{
		Method: "GET",
		URL:    reqURL,
		Header: map[string][]string{
			"Authorization": {ptoken}},
	}
	req, err := http.DefaultClient.Do(res)
	if err != nil {
		panic(err)

	}
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var data GoogleResponse
	errorz := json.Unmarshal(body, &data)
	if errorz != nil {

		panic(errorz)
	}
	return data.Email
}

func GoogleRequest(c *fiber.Ctx) error {
	path := configGoogle()
	authCodeURL := path.AuthCodeURL("state")
	return c.Redirect(authCodeURL)
}

func AuthCallbackGoogle(c *fiber.Ctx) error {
	token, err := configGoogle().Exchange(c.Context(), c.FormValue("code"))
	if err != nil {
		panic(err)
	}

	email := GetEmail(token.AccessToken)
	return c.Status(200).JSON(fiber.Map{"email": email, "login": true})
}
