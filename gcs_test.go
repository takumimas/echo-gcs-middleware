package gcsmiddleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestFilePath tests the filePath method of FilesStore.
func TestFilePath(t *testing.T) {
	// Define test cases
	tests := []struct {
		name       string
		requestURL string
		config     GCSStaticConfig
		expected   string
	}{
		{
			name:       "Root path with trailing slash, not SPA",
			requestURL: "/static/css/style.css",
			config: GCSStaticConfig{
				RootPath: "/static/",
				IsSPA:    false,
			},
			expected: "css/style.css",
		},
		{
			name:       "Root path without trailing slash, not SPA",
			requestURL: "/static/css/style.css",
			config: GCSStaticConfig{
				RootPath: "/static",
				IsSPA:    false,
			},
			expected: "css/style.css",
		},
		{
			name:       "Root path without leading slash, not SPA",
			requestURL: "/static/css/style.css",
			config: GCSStaticConfig{
				RootPath: "static/",
				IsSPA:    false,
			},
			expected: "css/style.css",
		},
		{
			name:       "SPA with path lacking extension",
			requestURL: "/app/dashboard",
			config: GCSStaticConfig{
				RootPath: "/app/",
				IsSPA:    true,
			},
			expected: "dashboard/index.html",
		},
		{
			name:       "SPA with path having extension",
			requestURL: "/app/main.js",
			config: GCSStaticConfig{
				RootPath: "/app/",
				IsSPA:    true,
			},
			expected: "main.js",
		},
		{
			name:       "Non-SPA with path lacking extension",
			requestURL: "/content/page",
			config: GCSStaticConfig{
				RootPath: "/content/",
				IsSPA:    false,
			},
			expected: "page",
		},
		{
			name:       "Root path as single slash",
			requestURL: "/css/style.css",
			config: GCSStaticConfig{
				RootPath: "/",
				IsSPA:    false,
			},
			expected: "css/style.css",
		},
		{
			name:       "Complex path with nested directories in SPA",
			requestURL: "/app/admin/settings",
			config: GCSStaticConfig{
				RootPath: "/app/",
				IsSPA:    true,
			},
			expected: "admin/settings/index.html",
		},
		{
			name:       "Complex path with query parameters",
			requestURL: "/static/js/app.js?v=1.2.3",
			config: GCSStaticConfig{
				RootPath: "/static/",
				IsSPA:    false,
			},
			expected: "js/app.js?v=1.2.3",
		},
		{
			name:       "Path matching root, SPA",
			requestURL: "/app/",
			config: GCSStaticConfig{
				RootPath: "/app/",
				IsSPA:    true,
			},
			expected: "/index.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new FilesStore with the test config
			fs := &FilesStore{
				config: tt.config,
			}

			// Create a new Echo context with the request URL
			e := echo.New()
			req := &http.Request{
				URL: &url.URL{
					Path: tt.requestURL,
				},
			}
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			// Call the filePath method
			actual := fs.filePath(ctx)

			// Assert that the actual path matches the expected path
			assert.Equal(t, tt.expected, actual)
		})
	}
}
