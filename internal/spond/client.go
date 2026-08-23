package spond

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

type Client struct {
	apiClient             api.ClientInterface
	token                 string
	tokenExpiresAt        *time.Time
	refreshToken          string
	refreshTokenExpiresAt *time.Time
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
	c.tokenExpiresAt = authResponse.AccessToken.Expiration
	if authResponse.RefreshToken != nil {
		if authResponse.RefreshToken.Token != nil {
			c.refreshToken = *authResponse.RefreshToken.Token
		}
		c.refreshTokenExpiresAt = authResponse.RefreshToken.Expiration
	}
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

	return c.fetchEventsWithParams(ctx, params)
}

func (c *Client) FetchEventsForGroups(ctx context.Context, maxEvents int, groupIDs []string) ([]api.Event, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	normalizedGroupIDs := normalizeGroupIDs(groupIDs)
	if len(normalizedGroupIDs) == 0 {
		return c.FetchEvents(ctx, maxEvents)
	}

	combined := make([]api.Event, 0, len(normalizedGroupIDs)*8)
	seenByID := map[string]struct{}{}

	for _, groupID := range normalizedGroupIDs {
		scheduled := false
		currentGroupID := groupID
		params := &api.GetSpondsParams{
			Max:       &maxEvents,
			Scheduled: &scheduled,
			GroupId:   &currentGroupID,
		}

		events, err := c.fetchEventsWithParams(ctx, params)
		if err != nil {
			return nil, err
		}

		for i := range events {
			if events[i].GroupId == nil && events[i].SubGroupId == nil {
				events[i].GroupId = &currentGroupID
			}
		}

		for _, event := range events {
			if strings.TrimSpace(event.Id) == "" {
				combined = append(combined, event)
				continue
			}

			if _, exists := seenByID[event.Id]; exists {
				continue
			}

			seenByID[event.Id] = struct{}{}
			combined = append(combined, event)
		}
	}

	return combined, nil
}

func (c *Client) fetchEventsWithParams(ctx context.Context, params *api.GetSpondsParams) ([]api.Event, error) {
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

func normalizeGroupIDs(groupIDs []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(groupIDs))

	for _, groupID := range groupIDs {
		trimmed := strings.TrimSpace(groupID)
		if trimmed == "" {
			continue
		}

		if _, exists := seen[trimmed]; exists {
			continue
		}

		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}

	return items
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

func (c *Client) TokenExpiresAt() *time.Time {
	return c.tokenExpiresAt
}

func (c *Client) RefreshToken() string {
	return c.refreshToken
}

func (c *Client) RefreshTokenExpiresAt() *time.Time {
	return c.refreshTokenExpiresAt
}

func (c *Client) bearerRequestEditor() api.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+c.token)
		return nil
	}
}
