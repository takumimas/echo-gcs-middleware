package gcsmiddleware

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
			expected: "index.html",
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

// TestGetContentType tests the getContentType function with various file extensions
// and fallback scenarios.
func TestGetContentType(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		fallback string
		want     string
	}{
		{
			name:     "Common HTML file",
			path:     "index.html",
			fallback: "",
			want:     "text/html",
		},
		{
			name:     "CSS file",
			path:     "styles.css",
			fallback: "",
			want:     "text/css",
		},
		{
			name:     "JavaScript file",
			path:     "app.js",
			fallback: "",
			want:     "application/javascript",
		},
		{
			name:     "JPEG image",
			path:     "photo.jpg",
			fallback: "",
			want:     "image/jpeg",
		},
		{
			name:     "JPEG image (uppercase extension)",
			path:     "photo.JPG",
			fallback: "",
			want:     "image/jpeg",
		},
		{
			name:     "PNG image",
			path:     "icon.png",
			fallback: "",
			want:     "image/png",
		},
		{
			name:     "Unknown extension with fallback",
			path:     "data.xyz",
			fallback: "application/custom",
			want:     "application/custom",
		},
		{
			name:     "Unknown extension without fallback",
			path:     "data.unknown",
			fallback: "",
			want:     "application/octet-stream",
		},
		{
			name:     "No extension with fallback",
			path:     "README",
			fallback: "text/plain",
			want:     "text/plain",
		},
		{
			name:     "Web font file",
			path:     "font.woff2",
			fallback: "",
			want:     "font/woff2",
		},
		{
			name:     "Path with directory",
			path:     "/static/css/styles.css",
			fallback: "",
			want:     "text/css",
		},
		{
			name:     "Path with query parameters",
			path:     "styles.css?v=1.2.3",
			fallback: "",
			want:     "text/css",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContentType(tt.path, tt.fallback)
			if got != tt.want {
				t.Errorf("getContentType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestShouldCompress tests the shouldCompress function with various content types and sizes
func TestShouldCompress(t *testing.T) {
	fs := &FilesStore{
		config: GCSStaticConfig{
			EnableCompression:      true,
			MinSizeForCompression: 1024, // 1KB
		},
	}

	tests := []struct {
		name        string
		contentType string
		size        int64
		want        bool
	}{
		{
			name:        "HTML file above minimum size",
			contentType: "text/html",
			size:        2048,
			want:        true,
		},
		{
			name:        "HTML file below minimum size",
			contentType: "text/html",
			size:        512,
			want:        false,
		},
		{
			name:        "Image file (not compressible)",
			contentType: "image/jpeg",
			size:        2048,
			want:        false,
		},
		{
			name:        "JavaScript file above minimum size",
			contentType: "application/javascript",
			size:        2048,
			want:        true,
		},
		{
			name:        "Empty content type",
			contentType: "",
			size:        2048,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fs.shouldCompress(tt.contentType, tt.size)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCompressData tests the compressData function
func TestCompressData(t *testing.T) {
	fs := &FilesStore{
		config: GCSStaticConfig{
			CompressionLevel: 6,
		},
	}

	tests := []struct {
		name     string
		data     []byte
		encoding string
		wantErr  bool
	}{
		{
			name:     "Gzip compression",
			data:     []byte(strings.Repeat("Hello, World!", 100)), // より大きなデータを使用
			encoding: "gzip",
			wantErr:  false,
		},
		{
			name:     "Brotli compression (not implemented)",
			data:     []byte("Hello, World!"),
			encoding: "br",
			wantErr:  true,
		},
		{
			name:     "Invalid encoding",
			data:     []byte("Hello, World!"),
			encoding: "invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := fs.compressData(tt.data, tt.encoding)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, compressed)
				// 圧縮データは元のデータより小さくなるはず
				assert.Less(t, len(compressed), len(tt.data))
			}
		})
	}
}

// TestContentLength tests the content length calculation
func TestContentLength(t *testing.T) {
	fs := &FilesStore{
		config: GCSStaticConfig{
			EnableCompression: true,
			CompressionLevel: 6,
		},
	}

	largeData := []byte(strings.Repeat("Hello, World!", 100)) // より大きなデータを使用

	tests := []struct {
		name     string
		data     []byte
		encoding string
	}{
		{
			name:     "Uncompressed data",
			data:     largeData,
			encoding: "",
		},
		{
			name:     "Gzip compressed data",
			data:     largeData,
			encoding: "gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if tt.encoding == "gzip" {
				gz, err := gzip.NewWriterLevel(&buf, fs.config.CompressionLevel)
				if err != nil {
					t.Fatal(err)
				}
				_, err = gz.Write(tt.data)
				if err != nil {
					t.Fatal(err)
				}
				err = gz.Close()
				if err != nil {
					t.Fatal(err)
				}
				// 圧縮データは元のデータより小さくなるはず
				assert.Less(t, buf.Len(), len(tt.data))
			} else {
				_, err := buf.Write(tt.data)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, len(tt.data), buf.Len())
			}
		})
	}
}
