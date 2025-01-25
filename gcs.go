// Package gcsmiddleware provides middleware for serving static files from Google Cloud Storage (GCS)
// in Echo web framework applications. It supports both traditional static file serving and
// Single Page Application (SPA) hosting with customizable configuration options.
package gcsmiddleware

import (
	"cloud.google.com/go/storage"
	"context"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	pathLib "path"
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
		file, contentType, err := s.getFile(filePath)
		if err != nil {
			if s.config.IsSPA {
				IndexFile, IndexFileType, Err := s.getFile("index.html")
				if Err == nil {
					return c.Blob(http.StatusOK, IndexFileType, IndexFile)
				}
			}
			return c.NoContent(http.StatusNotFound)
		}

		return c.Blob(http.StatusOK, contentType, file)
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
	path := ctx.Request().URL.Path
	rootPath := s.config.RootPath
	if rootPath[0] != '/' {
		rootPath = "/" + rootPath
	}
	if rootPath[len(rootPath)-1:] != "/" {
		rootPath = rootPath + "/"
	}
	path = strings.Replace(path, rootPath, "", 1)
	if s.config.IsSPA {
		base := pathLib.Base(path)
		if !strings.Contains(base, ".") {
			path = path + "/index.html"
		}
		if base == "." {
			path = "index.html"
		}
	}
	return path
}

// getFile retrieves a file from Google Cloud Storage using the specified path.
// It handles the GCS object reading and returns the file contents along with
// the content type.
//
// Parameters:
//   - path: The path to the file in the GCS bucket
//
// Returns:
//   - body: The file contents as a byte slice
//   - contentType: The MIME type of the file
//   - err: Any error encountered during the file retrieval process
func (s *FilesStore) getFile(path string) (body []byte, contentType string, err error) {
	reader, err := s.config.Client.Bucket(s.config.BucketName).Object(path).NewReader(context.Background())
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()
	fileBinary, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	return fileBinary, reader.Attrs.ContentType, nil
}
