# echo-gcs-middleware

---

This Echo middleware provides a static file store powered by Google Cloud Storage as its backend. In detail, it facilitates the efficient delivery of static files—such as HTML, CSS, JavaScript, or image files—stored in Google Cloud Storage directly to end users. By leveraging this middleware, you can host static content on GCS without embedding these files directly within your application code, enabling seamless access to the content via HTTP requests.

# Example

---

```go
e := echo.NewStaticServerMiddleware()

type FilesConfig struct {
    Bucket       string
    Region       string 
    SPA          bool   // Enable Single Page Application mode
    Index        string // Default file to serve (e.g., "index.html")
    Client       *storage.Client // Allow external injection of GCS client
}

fs := s3middleware.NewStaticServerMiddleware(s3middleware.FilesConfig{
  Region: "us-east-1",    // can also be assigned using AWS_REGION environment variable
  SPA: true,              // enable fallback which will try Index if the first path is not found
  Index: "login.html",
  Summary: func(ctx context.Context, data map[string]interface{}) {
    log.Printf("processed s3 request: %+v", data)
  },
  OnErr: func(ctx context.Context, err error) {
    log.Printf("failed to process s3 request: %+v", err)
  },
})

// serve static files from the supplied bucket
e.Use()
```