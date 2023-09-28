package oauth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleResponse struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Verified   bool   `json:"verified_email"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Picture    string `json:"picture"`
	Locale     string `json:"locale"`
}

func configGoogle() *oauth2.Config {
	clientId := os.Getenv("OAUTH2_GOOGLE_CLIENTID")
	clientSecret := os.Getenv("OAUTH2_GOOGLE_CLIENTSECRET")
	redirectUrl := os.Getenv("OAUTH2_GOOGLE_REDIRECTURL")

	conf := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUrl,
		Scopes:       []string{"profile", "email"},
		Endpoint:     google.Endpoint,
	}
	return conf
}

func GoogleGetProfile(token string) (*GoogleResponse, error) {
	reqURL, err := url.Parse("https://www.googleapis.com/oauth2/v1/userinfo")
	if err != nil {
		return nil, err
	}

	res := &http.Request{
		Method: "GET",
		URL:    reqURL,
		Header: map[string][]string{"Authorization": {"Bearer " + token}, "Accept": {"application/json"}},
	}
	req, err := http.DefaultClient.Do(res)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	if req.StatusCode != 200 {
		return nil, errors.New("unexpected status code")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var data GoogleResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
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

	profile, err := GoogleGetProfile(token.AccessToken)
	if err != nil {
		panic(err)
	}
	return c.Status(200).JSON(fiber.Map{"email": profile.Email, "login": true})
}
