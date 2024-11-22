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
type GCSStaticConfig struct {
	Client     *storage.Client
	BucketName string
	IgnorePath []string
	IsSPA      bool
	RootPath   string
}

// FilesStore manages the GCS client
type FilesStore struct {
	config GCSStaticConfig
}

// StaticServerMiddlewareInterface defines methods for handling server headers and file retrieval in a static server
// ServerHeader handles adding server headers to the response
type StaticServerMiddlewareInterface interface {
	ServerHeader(next echo.HandlerFunc) echo.HandlerFunc
}

// NewGCSStaticMiddleware initializes and returns a new instance of FilesStore with the provided GCSStaticConfig.
func (s *FilesStore) NewGCSStaticMiddleware(config GCSStaticConfig) StaticServerMiddlewareInterface {
	return &FilesStore{
		config: config,
	}
}

// ServerHeader is a middleware that handles serving files from a GCS bucket, bypassing specified paths. It sets the response status and content type accordingly.
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
			return c.NoContent(http.StatusNotFound)
		}

		return c.Blob(http.StatusOK, contentType, file)
	}
}

// FilePath returns the processed file path from the requested URL, considering the server configuration and SPA settings.
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
		if base == "" {
			path = "/index.html"
		}
	}
	return path
}

// GetFile retrieves the file located at the specified path from the GCS bucket.
// It returns the file content as a byte slice, the content type, and any error encountered during the process.
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
