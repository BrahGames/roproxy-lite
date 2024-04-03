package main

import (
    "net/url"
    "log"
    "time"
    "os"
    "strings"
    "github.com/valyala/fasthttp"
    "strconv"
)

var timeout, _ = strconv.Atoi(os.Getenv("TIMEOUT"))
var retries, _ = strconv.Atoi(os.Getenv("RETRIES"))
var port = os.Getenv("PORT")

var client *fasthttp.Client

func main() {
    client = &fasthttp.Client{
        ReadTimeout: time.Duration(timeout) * time.Second,
        MaxIdleConnDuration: 60 * time.Second,
    }

    if err := fasthttp.ListenAndServe(":" + port, requestHandler); err != nil {
        log.Fatalf("Error in ListenAndServe: %s", err)
    }
}

func requestHandler(ctx *fasthttp.RequestCtx) {
    // Call makeRequest with initial attempt number
    response := makeRequest(ctx, 1)
    defer fasthttp.ReleaseResponse(response)

    // Check if the response Content-Type is application/json
    contentType := string(response.Header.Peek("Content-Type"))
    if !strings.Contains(contentType, "application/json") {
        ctx.SetStatusCode(fasthttp.StatusUnsupportedMediaType)
        ctx.SetBody([]byte("Unsupported format. Only JSON format is supported."))
        return
    }

    // Set the response body in ctx
    body := response.Body()
    ctx.SetBody(body)

    // Set Content-Type to application/json
    ctx.Response.Header.SetContentType("application/json")

    // Set the status code
    ctx.SetStatusCode(response.StatusCode())
}

func makeRequest(ctx *fasthttp.RequestCtx, attempt int) *fasthttp.Response {
    if attempt > retries {
        log.Printf("Exceeded maximum retry attempts")
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Proxy failed to connect. Please try again."))
        resp.SetStatusCode(500)
        return resp
    }

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)
    req.Header.SetMethod(string(ctx.Method()))

    originalURI := string(ctx.Request.Header.RequestURI())
    log.Printf("Original URI: %s", originalURI)

    parsedURI, err := url.ParseRequestURI(originalURI)
    if err != nil {
        log.Printf("Error parsing request URI: %s", err)
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Invalid URL format."))
        resp.SetStatusCode(400)
        return resp
    }

    targetURL := "https://" + strings.TrimPrefix(parsedURI.Path, "/proxy/")
    if parsedURI.RawQuery != "" {
        targetURL += "?" + parsedURI.RawQuery
    }

    log.Printf("Forwarding request to: %s", targetURL)
    req.SetRequestURI(targetURL)

    req.SetBody(ctx.Request.Body())
    ctx.Request.Header.VisitAll(func(key, value []byte) {
        if string(key) != "Host" {
            req.Header.Set(string(key), string(value))
        }
    })

    resp := fasthttp.AcquireResponse()
    err = client.Do(req, resp)
    if err != nil {
        log.Printf("Error making forwarded request: %s", err)
        fasthttp.ReleaseResponse(resp)
        return makeRequest(ctx, attempt + 1)
    }

    return resp
}
