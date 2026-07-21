package spond

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

type Client struct {
	apiClient api.ClientInterface
	token     string
}

func New(baseURL string, opts ...api.ClientOption) (*Client, error) {
	apiClient, err := api.NewClient(baseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("create api client: %w", err)
	}

	return &Client{apiClient: apiClient}, nil
}

func NewWithAPIClient(apiClient api.ClientInterface) *Client {
	return &Client{apiClient: apiClient}
}

func (c *Client) Login(ctx context.Context, email, password string) error {
	body := api.PostAuth2LoginJSONRequestBody{
		Email:    openapi_types.Email(email),
		Password: password,
	}

	response, err := c.apiClient.PostAuth2Login(ctx, body)
	if err != nil {
		return fmt.Errorf("call login endpoint: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return fmt.Errorf("login failed with status %d: %s", response.StatusCode, string(payload))
	}

	var authResponse api.AuthResponse
	if err := json.NewDecoder(response.Body).Decode(&authResponse); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}

	if authResponse.AccessToken.Token == nil || *authResponse.AccessToken.Token == "" {
		return fmt.Errorf("login response did not include access token")
	}

	c.token = *authResponse.AccessToken.Token
	return nil
}

func (c *Client) FetchEvents(ctx context.Context, maxEvents int) ([]api.Event, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	scheduled := false
	params := &api.GetSpondsParams{
		Max:       &maxEvents,
		Scheduled: &scheduled,
	}

	response, err := c.apiClient.GetSponds(ctx, params, c.bearerRequestEditor())
	if err != nil {
		return nil, fmt.Errorf("call events endpoint: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("fetch events failed with status %d: %s", response.StatusCode, string(payload))
	}

	var events []api.Event
	if err := json.NewDecoder(response.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("decode events response: %w", err)
	}

	return events, nil
}

func (c *Client) FetchProfile(ctx context.Context) (api.Profile, error) {
	if c.token == "" {
		return api.Profile{}, fmt.Errorf("not authenticated")
	}

	response, err := c.apiClient.GetProfile(ctx, c.bearerRequestEditor())
	if err != nil {
		return api.Profile{}, fmt.Errorf("call profile endpoint: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return api.Profile{}, fmt.Errorf("fetch profile failed with status %d: %s", response.StatusCode, string(payload))
	}

	var profile api.Profile
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		return api.Profile{}, fmt.Errorf("decode profile response: %w", err)
	}

	return profile, nil
}

func (c *Client) FetchGroups(ctx context.Context) ([]api.Group, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	response, err := c.apiClient.GetGroups(ctx, c.bearerRequestEditor())
	if err != nil {
		return nil, fmt.Errorf("call groups endpoint: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("fetch groups failed with status %d: %s", response.StatusCode, string(payload))
	}

	var groups []api.Group
	if err := json.NewDecoder(response.Body).Decode(&groups); err != nil {
		return nil, fmt.Errorf("decode groups response: %w", err)
	}

	return groups, nil
}

func (c *Client) Token() string {
	return c.token
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) bearerRequestEditor() api.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+c.token)
		return nil
	}
}
