// Package gcsmiddleware provides middleware for serving static files from Google Cloud Storage (GCS)
// in Echo web framework applications. It supports both traditional static file serving and
// Single Page Application (SPA) hosting with customizable configuration options.
package gcsmiddleware

import (
	"cloud.google.com/go/storage"
	"bytes"
	"context"
	"compress/gzip"
	"fmt"
	"github.com/labstack/echo/v4"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// GCSStaticConfig holds configuration details for a static server setup.
// This configuration is used to initialize the middleware with necessary GCS settings
// and behavioral options.
type GCSStaticConfig struct {
	// Client is the initialized Google Cloud Storage client
	Client *storage.Client

	// BucketName is the name of the GCS bucket to serve files from
	BucketName string

	// IgnorePath is a list of paths that should bypass this middleware
	IgnorePath []string

	// IsSPA indicates whether the server should handle routes as a Single Page Application.
	// When true, missing files will fall back to serving index.html
	IsSPA bool

	// RootPath specifies the base path from which files are served.
	// For example, if RootPath is "/static/", a request to "/static/css/style.css"
	// will serve the file at "css/style.css" in the bucket
	RootPath string

	// EnableCompression enables gzip/brotli compression for text-based files
	EnableCompression bool

	// CompressionLevel specifies the compression level (1-9, higher means better compression but slower)
	// Default is 6 if not specified
	CompressionLevel int

	// MinSizeForCompression specifies the minimum file size in bytes for compression
	// Files smaller than this size will not be compressed
	MinSizeForCompression int64
}

// FilesStore manages the GCS client and handles file operations.
// It implements the StaticServerMiddlewareInterface for serving static files.
type FilesStore struct {
	config GCSStaticConfig
}

// StaticServerMiddlewareInterface defines methods for handling server headers and file retrieval
// in a static server. This interface allows for easy mocking in tests and flexibility
// in implementation.
type StaticServerMiddlewareInterface interface {
	// ServerHeader handles adding server headers to the response and serves files
	ServerHeader(next echo.HandlerFunc) echo.HandlerFunc
}

// NewGCSStaticMiddleware initializes and returns a new instance of FilesStore with the provided GCSStaticConfig.
// It creates a middleware that can serve static files from Google Cloud Storage.
//
// Parameters:
//   - config: GCSStaticConfig containing the necessary configuration for GCS connection and behavior
//
// Returns:
//   - StaticServerMiddlewareInterface that can be used with Echo's Use() method
func NewGCSStaticMiddleware(config GCSStaticConfig) StaticServerMiddlewareInterface {
	return &FilesStore{
		config: config,
	}
}

// ServerHeader is a middleware that handles serving files from a GCS bucket.
// It processes the request path, retrieves files from GCS, and sets appropriate
// response headers. For SPA mode, it falls back to serving index.html for missing files.
//
// Parameters:
//   - next: The next middleware handler in the chain
//
// Returns:
//   - echo.HandlerFunc that processes the request and serves the file
func (s *FilesStore) ServerHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, ignorePath := range s.config.IgnorePath {
			if c.Request().URL.Path == ignorePath {
				return next(c)
			}
		}
		filePath := s.filePath(c)
		
		// Prepare paths for potential parallel retrieval
		paths := []string{filePath}
		if s.config.IsSPA {
			paths = append(paths, "index.html") // Add index.html for SPA mode
		}

		// Get files in parallel
		results := s.getFiles(paths)

		// Process main file result
		fileResult := results[0]
		if fileResult.Err != nil {
			if s.config.IsSPA {
				// Use index.html result if available
				indexResult := results[1]
				if indexResult.Err == nil {
					c.Response().Header().Set("Content-Length", strconv.FormatInt(indexResult.Size, 10))
					return c.Blob(http.StatusOK, indexResult.ContentType, indexResult.Body)
				}
			}
			return c.NoContent(http.StatusNotFound)
		}

		// Check if compression is possible
		if s.shouldCompress(fileResult.ContentType, fileResult.Size) {
			acceptEncoding := c.Request().Header.Get("Accept-Encoding")
			var encoding string
			if strings.Contains(acceptEncoding, "br") {
				encoding = "br"
			} else if strings.Contains(acceptEncoding, "gzip") {
				encoding = "gzip"
			}

			if encoding != "" {
				compressed, err := s.compressData(fileResult.Body, encoding)
				if err == nil {
					c.Response().Header().Set("Content-Encoding", encoding)
					c.Response().Header().Set("Content-Length", strconv.Itoa(len(compressed)))
					c.Response().Header().Set("Vary", "Accept-Encoding")
					return c.Blob(http.StatusOK, fileResult.ContentType, compressed)
				}
			}
		}

		c.Response().Header().Set("Content-Length", strconv.FormatInt(fileResult.Size, 10))
		return c.Blob(http.StatusOK, fileResult.ContentType, fileResult.Body)
	}
}

// filePath processes the request URL path according to the configuration settings.
// It handles both SPA and non-SPA paths, applying the root path prefix and
// index.html fallback as needed.
//
// Parameters:
//   - ctx: The Echo context containing the request information
//
// Returns:
//   - string representing the processed file path to be used for GCS object retrieval
func (s *FilesStore) filePath(ctx echo.Context) string {
	reqPath := ctx.Request().URL.Path
	rootPath := s.config.RootPath
	if rootPath[0] != '/' {
		rootPath = "/" + rootPath
	}
	if rootPath[len(rootPath)-1:] != "/" {
		rootPath = rootPath + "/"
	}
	reqPath = strings.Replace(reqPath, rootPath, "", 1)
	if s.config.IsSPA {
		base := path.Base(reqPath)
		if !strings.Contains(base, ".") {
			if reqPath == "" || reqPath == "/" {
				reqPath = "index.html"
			} else {
				reqPath = strings.TrimPrefix(reqPath, "/") + "/index.html"
			}
		}
		if base == "." {
			reqPath = "index.html"
		}
	}
	return reqPath
}

// mimeTypeMap contains common file extensions and their corresponding MIME types
var mimeTypeMap = map[string]string{
	".html": "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
	".json": "application/json",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".txt":  "text/plain",
	".pdf":  "application/pdf",
	".woff": "font/woff",
	".woff2": "font/woff2",
	".ttf":  "font/ttf",
	".eot":  "application/vnd.ms-fontobject",
}

// getContentType determines the content type of a file based on its extension
// If the extension is not recognized, it falls back to the provided fallback type
// or "application/octet-stream" if no fallback is provided
func getContentType(filePath string, fallback string) string {
	// Remove query parameters if present
	if idx := strings.Index(filePath, "?"); idx != -1 {
		filePath = filePath[:idx]
	}

	ext := strings.ToLower(path.Ext(filePath))
	if mimeType, ok := mimeTypeMap[ext]; ok {
		return mimeType
	}

	// For unknown extensions, prioritize the fallback over mime package
	if fallback != "" {
		return fallback
	}

	// Try to use the standard mime package as a fallback
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		// Some MIME types from the mime package include additional parameters
		// (e.g., "text/plain; charset=utf-8") which we want to strip
		if idx := strings.Index(mimeType, ";"); idx != -1 {
			mimeType = mimeType[:idx]
		}
		return strings.TrimSpace(mimeType)
	}

	return "application/octet-stream"
}

// FileResult represents the result of a file retrieval operation
type FileResult struct {
	Body        []byte
	ContentType string
	Size        int64
	Err         error
}

// getFile retrieves a file from Google Cloud Storage using the specified path.
// It handles the GCS object reading and returns the file contents along with
// the content type and size.
//
// Parameters:
//   - path: The path to the file in the GCS bucket
//
// Returns:
//   - body: The file contents as a byte slice
//   - contentType: The MIME type of the file
//   - size: The size of the file in bytes
//   - err: Any error encountered during the file retrieval process
func (s *FilesStore) getFile(path string) (body []byte, contentType string, size int64, err error) {
	obj := s.config.Client.Bucket(s.config.BucketName).Object(path)
	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return nil, "", 0, err
	}

	reader, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, "", 0, err
	}
	defer reader.Close()

	fileBinary, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", 0, err
	}

	// Get content type from file extension first, falling back to GCS metadata
	contentType = getContentType(path, attrs.ContentType)
	return fileBinary, contentType, attrs.Size, nil
}

// getFileAsync retrieves a file from GCS asynchronously
func (s *FilesStore) getFileAsync(path string, resultChan chan<- FileResult) {
	body, contentType, size, err := s.getFile(path)
	resultChan <- FileResult{
		Body:        body,
		ContentType: contentType,
		Size:        size,
		Err:         err,
	}
}

// getFiles retrieves multiple files from GCS in parallel
func (s *FilesStore) getFiles(paths []string) []FileResult {
	resultChan := make(chan FileResult, len(paths))
	results := make([]FileResult, len(paths))

	// Start goroutines for each file
	for _, path := range paths {
		go s.getFileAsync(path, resultChan)
	}

	// Collect results
	for i := 0; i < len(paths); i++ {
		result := <-resultChan
		results[i] = result
	}

	return results
}

// compressData compresses the input data using the specified encoding
func (s *FilesStore) compressData(data []byte, encoding string) ([]byte, error) {
	var buf bytes.Buffer
	var writer io.WriteCloser

	switch encoding {
	case "gzip":
		level := s.config.CompressionLevel
		if level == 0 {
			level = 6 // default compression level
		}
		writer, _ = gzip.NewWriterLevel(&buf, level)
	case "br":
		// Note: brotli compression requires additional dependency
		// You may want to add github.com/andybalholm/brotli
		return nil, fmt.Errorf("brotli compression not implemented")
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
	}

	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// shouldCompress determines if the file should be compressed based on its content type and size
func (s *FilesStore) shouldCompress(contentType string, size int64) bool {
	if !s.config.EnableCompression {
		return false
	}

	if size < s.config.MinSizeForCompression {
		return false
	}

	compressibleTypes := map[string]bool{
		"text/html":                true,
		"text/css":                 true,
		"text/plain":               true,
		"text/xml":                 true,
		"application/javascript":    true,
		"application/json":         true,
		"application/xml":          true,
		"application/x-javascript": true,
		"application/ld+json":      true,
	}

	return compressibleTypes[contentType]
}
