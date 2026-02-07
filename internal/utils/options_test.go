package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInterpolateVariables(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		variables map[string]interface{}
		want      string
		wantErr   bool
	}{
		{
			name:      "no variables",
			url:       "https://api.example.com/options",
			variables: map[string]interface{}{},
			want:      "https://api.example.com/options",
			wantErr:   false,
		},
		{
			name:      "single variable",
			url:       "https://api.example.com/options?env={{environment}}",
			variables: map[string]interface{}{"environment": "production"},
			want:      "https://api.example.com/options?env=production",
			wantErr:   false,
		},
		{
			name: "multiple variables",
			url:  "https://api.example.com/options?env={{env}}&region={{region}}&type={{type}}",
			variables: map[string]interface{}{
				"env":    "production",
				"region": "us-east-1",
				"type":   "services",
			},
			want:    "https://api.example.com/options?env=production&region=us-east-1&type=services",
			wantErr: false,
		},
		{
			name:      "missing variable",
			url:       "https://api.example.com/options?env={{environment}}",
			variables: map[string]interface{}{},
			wantErr:   true,
		},
		{
			name:      "numeric variable",
			url:       "https://api.example.com/options?max={{limit}}",
			variables: map[string]interface{}{"limit": 100},
			want:      "https://api.example.com/options?max=100",
			wantErr:   false,
		},
		{
			name:      "boolean variable",
			url:       "https://api.example.com/options?active={{is_active}}",
			variables: map[string]interface{}{"is_active": true},
			want:      "https://api.example.com/options?active=true",
			wantErr:   false,
		},
		{
			name:      "variable with spaces",
			url:       "https://api.example.com/search?query={{search_term}}",
			variables: map[string]interface{}{"search_term": "test value"},
			want:      "https://api.example.com/search?query=test value",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InterpolateVariables(tt.url, tt.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("InterpolateVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("InterpolateVariables() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFetchRemoteOptions(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *httptest.Server
		url       string
		want      []string
		wantErr   bool
	}{
		{
			name: "successful fetch",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					options := []Option{
						{Name: "option1", Selected: false},
						{Name: "option2", Selected: true},
						{Name: "option3", Selected: false},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(options)
				}))
			},
			want:    []string{"option1", "option2", "option3"},
			wantErr: false,
		},
		{
			name: "empty options",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					options := []Option{}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(options)
				}))
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "options with empty names skipped",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					options := []Option{
						{Name: "option1", Selected: false},
						{Name: "", Selected: false},
						{Name: "option2", Selected: false},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(options)
				}))
			},
			want:    []string{"option1", "option2"},
			wantErr: false,
		},
		{
			name: "server returns error",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error"))
				}))
			},
			wantErr: true,
		},
		{
			name: "invalid json response",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("not valid json"))
				}))
			},
			wantErr: true,
		},
		{
			name: "empty url",
			setupMock: func() *httptest.Server {
				return nil
			},
			url:     "",
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMock()
			if server != nil {
				defer server.Close()
				tt.url = server.URL
			}

			got, err := FetchRemoteOptions(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchRemoteOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("FetchRemoteOptions() got %d items, want %d items", len(got), len(tt.want))
					return
				}
				for i, v := range got {
					if v != tt.want[i] {
						t.Errorf("FetchRemoteOptions()[%d] = %s, want %s", i, v, tt.want[i])
					}
				}
			}
		})
	}
}

func TestFetchRemoteOptionsWithVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env := r.URL.Query().Get("env")
		region := r.URL.Query().Get("region")

		options := []Option{}
		if env == "prod" && region == "us-east" {
			options = []Option{
				{Name: "service1", Selected: false},
				{Name: "service2", Selected: false},
			}
		} else if env == "dev" {
			options = []Option{
				{Name: "dev-service", Selected: false},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(options)
	}))
	defer server.Close()

	tests := []struct {
		name      string
		url       string
		variables map[string]interface{}
		want      []string
		wantErr   bool
	}{
		{
			name:      "successful interpolation and fetch",
			url:       server.URL + "?env={{environment}}&region={{region}}",
			variables: map[string]interface{}{"environment": "prod", "region": "us-east"},
			want:      []string{"service1", "service2"},
			wantErr:   false,
		},
		{
			name:      "single variable",
			url:       server.URL + "?env={{environment}}",
			variables: map[string]interface{}{"environment": "dev"},
			want:      []string{"dev-service"},
			wantErr:   false,
		},
		{
			name:      "missing variable",
			url:       server.URL + "?env={{environment}}",
			variables: map[string]interface{}{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchRemoteOptionsWithVariables(tt.url, tt.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchRemoteOptionsWithVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("FetchRemoteOptionsWithVariables() got %d items, want %d items", len(got), len(tt.want))
					return
				}
				for i, v := range got {
					if v != tt.want[i] {
						t.Errorf("FetchRemoteOptionsWithVariables()[%d] = %s, want %s", i, v, tt.want[i])
					}
				}
			}
		})
	}
}
