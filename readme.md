# echo-gcs-middleware

---

**This is a testing phase and not suitable for production use.**

This Echo middleware provides a static file store powered by Google Cloud Storage as its backend. In detail, it facilitates the efficient delivery of static files—such as HTML, CSS, JavaScript, or image files—stored in Google Cloud Storage directly to end users. By leveraging this middleware, you can host static content on GCS without embedding these files directly within your application code, enabling seamless access to the content via HTTP requests.

---

# Usage

## Installation
```
go get github.com/takumimas/echo-gcs-middleware
```

## Example

```go
func main() {
	ctx := context.Background()

	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer func() {
		if err := gcsClient.Close(); err != nil {
			log.Printf("Failed to close GCS client: %v", err)
		}
	}()

	e := echo.New()

	// Configure GCS middleware
	gcsConfig := gcsmiddleware.GCSStaticConfig{
		Client:     gcsClient,
		BucketName: "gcs-echo-test",
		IgnorePath: nil,  // Specify paths to ignore if any
		IsSPA:      true, // Set to true if serving a Single Page Application
		RootPath:   "/",   // Set the root path
	}

	// Create the GCS middleware
	gcsMiddleware := gcsmiddleware.NewGCSStaticMiddleware(gcsConfig)

	// Register the middleware with Echo
	e.Use(gcsMiddleware.ServerHeader)

	port := ":1323"
	log.Printf("Starting server on port %s...", port)
	if err := e.Start(port); err != nil {
		log.Fatalf("Failed to start the server: %v", err)
	}
}
```

**IsSPA:**

When IsSPA is set to true, any 404 errors will automatically redirect to index.html. This is useful for Single Page Applications (SPAs) where routing is handled client-side and all non-static paths should serve the main entry point (index.html).

**RootPath:**

If you set RootPath to a specific value, such as /app/, it adjusts the base path from which files are served. For example, when RootPath is set to /app/, a request to /app/ will serve the file located at app/index.html.



