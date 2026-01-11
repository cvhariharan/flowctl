package webhooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
	MaxRedirects   = 3
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Request struct {
	URL         string
	Body        string
	ContentType string
	Headers     []Header
	Event       string
	DeliveryID  string
}

type Response struct {
	StatusCode int
	Duration   time.Duration
}

type Sender struct {
	client       *http.Client
	logger       *slog.Logger
	maxRedirects int
}

func NewSender(logger *slog.Logger) *Sender {
	s := &Sender{
		logger:       logger,
		maxRedirects: MaxRedirects,
	}

	s.client = &http.Client{
		Timeout: DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= s.maxRedirects {
				return errors.New("too many redirects")
			}
			if strings.ToLower(req.URL.Scheme) != "https" {
				return errors.New("redirected to non-https URL")
			}
			return nil
		},
	}

	return s
}

func (s *Sender) Send(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.URL) == "" {
		return Response{}, errors.New("missing webhook URL")
	}

	parsed, err := url.Parse(req.URL)
	if err != nil {
		return Response{}, fmt.Errorf("invalid webhook URL: %w", err)
	}
	if strings.ToLower(parsed.Scheme) != "https" {
		return Response{}, errors.New("webhook URL must use https")
	}
	if parsed.Host == "" {
		return Response{}, errors.New("webhook URL must include a host")
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), strings.NewReader(req.Body))
	if err != nil {
		return Response{}, err
	}

	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("User-Agent", "flowctl")
	if req.Event != "" {
		httpReq.Header.Set("X-Flowctl-Event", req.Event)
	}
	if req.DeliveryID != "" {
		httpReq.Header.Set("X-Flowctl-Delivery-Id", req.DeliveryID)
	}

	for _, header := range req.Headers {
		if strings.TrimSpace(header.Key) == "" {
			continue
		}
		if strings.EqualFold(header.Key, "Content-Type") {
			continue
		}
		httpReq.Header.Set(header.Key, header.Value)
	}

	start := time.Now()
	resp, err := s.client.Do(httpReq)
	duration := time.Since(start)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("webhook request failed", "error", err)
		}
		return Response{StatusCode: 0, Duration: duration}, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return Response{StatusCode: resp.StatusCode, Duration: duration}, nil
}
