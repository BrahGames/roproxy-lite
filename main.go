package main

import (
    "fmt"
    "log"
    "net/url"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/valyala/fasthttp"
)

var (
    timeout = getIntFromEnv("TIMEOUT", 10) // Defaulting to 10 seconds if not set
    retries = getIntFromEnv("RETRIES", 3)  // Defaulting to 3 retries if not set
    port    = getStrFromEnv("PORT", "8080") // Defaulting to port 8080 if not set
)

var client = &fasthttp.Client{
    ReadTimeout:            time.Duration(timeout) * time.Second,
    MaxIdleConnDuration:    60 * time.Second,
    MaxConnsPerHost:        100,
    TLSConfig:              &tls.Config{InsecureSkipVerify: true}, // Enable if you trust the target server
}

func main() {
    if err := fasthttp.ListenAndServe(":"+port, requestHandler); err != nil {
        log.Fatalf("Error in ListenAndServe: %s", err)
    }
}

func requestHandler(ctx *fasthttp.RequestCtx) {
    response := makeRequest(ctx, 1)
    defer fasthttp.ReleaseResponse(response)

    // Forward the response status code
    ctx.SetStatusCode(response.StatusCode())

    // Forward the response body
    ctx.Write(response.Body())

    // Forward the Content-Type header
    ctx.Response.Header.SetContentType(string(response.Header.Peek("Content-Type")))
}

func makeRequest(ctx *fasthttp.RequestCtx, attempt int) *fasthttp.Response {
    if attempt > retries {
        resp := fasthttp.AcquireResponse()
        resp.SetStatusCode(fasthttp.StatusServiceUnavailable)
        resp.SetBodyString("Proxy failed to connect after retries.")
        return resp
    }

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)

    // Set the request method (GET, POST, etc.)
    req.Header.SetMethod(string(ctx.Method()))

    // Construct the target URL from the request
    targetURL := constructTargetURL(string(ctx.Request.URI().FullURI()))
    req.SetRequestURI(targetURL)

    // Forward the incoming request's headers to the target request
    forwardHeaders(ctx, req)

    // Execute the request
    resp := fasthttp.AcquireResponse()
    if err := client.Do(req, resp); err != nil {
        log.Printf("Attempt %d: error making request to %s: %s", attempt, targetURL, err)
        return makeRequest(ctx, attempt+1)
    }

    return resp
}

// constructTargetURL creates the target URL from the proxy request.
func constructTargetURL(requestURI string) string {
    parsedURI, err := url.Parse(requestURI)
    if err != nil {
        log.Fatalf("Error parsing request URI: %s", err)
    }

    // Remove the /proxy/ prefix and reconstruct the URL
    newPath := strings.TrimPrefix(parsedURI.Path, "/proxy/")
    return fmt.Sprintf("https://%s%s", newPath, parsedURI.RawQuery)
}

// forwardHeaders forwards headers from the incoming request to the target request.
func forwardHeaders(ctx *fasthttp.RequestCtx, req *fasthttp.Request) {
    ctx.Request.Header.VisitAll(func(key, value []byte) {
        // Skip forwarding the Host header to avoid issues with virtual hosting
        if string(key) != "Host" {
            req.Header.SetBytesKV(key, value)
        }
    })
}

// getIntFromEnv retrieves an integer environment variable or returns a default value.
func getIntFromEnv(key string, defaultValue int) int {
    valueStr := os.Getenv(key)
    if value, err := strconv.Atoi(valueStr); err == nil {
        return value
    }
    return defaultValue
}

// getStrFromEnv retrieves a string environment variable or returns a default value.
func getStrFromEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
