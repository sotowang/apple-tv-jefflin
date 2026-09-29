package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

type StatusError struct {
	StatusCode int
	Message    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("tmdb status %d: %s", e.StatusCode, e.Message)
}

type Client struct {
	HTTP                             *http.Client
	BaseURL, Token, Language, Region string
}

func New(token, language, region string, timeout time.Duration) *Client {
	if language == "" {
		language = "zh-CN"
	}
	if region == "" {
		region = "CN"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{HTTP: &http.Client{Timeout: timeout}, BaseURL: "https://api.themoviedb.org/3", Token: strings.TrimSpace(token), Language: language, Region: region}
}
func (c *Client) Name() string  { return "tmdb" }
func (c *Client) Enabled() bool { return c.Token != "" }

func (c *Client) get(ctx context.Context, endpoint string, params url.Values, dst any) error {
	if !c.Enabled() {
		return errors.New("tmdb disabled")
	}
	if c.HTTP.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.HTTP.Timeout)
		defer cancel()
	}
	if params == nil {
		params = url.Values{}
	}
	params.Set("language", c.Language)
	base, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + endpoint)
	if err != nil {
		return err
	}
	base.RawQuery = params.Encode()
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "online-media/2.0")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt == 2 {
				return fmt.Errorf("tmdb GET: %w", err)
			}
		} else {
			media.SetUpstreamStatus(ctx, resp.StatusCode)
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			resp.Body.Close()
			if readErr != nil {
				return fmt.Errorf("tmdb response: %w", readErr)
			}
			if resp.StatusCode == 200 {
				if err := json.Unmarshal(body, dst); err != nil {
					return fmt.Errorf("tmdb decode: %w", err)
				}
				return nil
			}
			if resp.StatusCode == 404 {
				return provider.ErrNotFound
			}
			var payload struct {
				Message string `json:"status_message"`
			}
			_ = json.Unmarshal(body, &payload)
			statusErr := &StatusError{StatusCode: resp.StatusCode, Message: payload.Message}
			if resp.StatusCode != 429 && resp.StatusCode < 500 {
				return statusErr
			}
			if attempt == 2 {
				return statusErr
			}
		}
		wait := time.Duration(attempt+1) * 200 * time.Millisecond
		if resp != nil && resp.StatusCode == 429 {
			if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 && seconds <= 2 {
				wait = time.Duration(seconds) * time.Second
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return errors.New("tmdb retry exhausted")
}
