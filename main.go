package main

import (
    "net/url"
    "log"
    "time"
    "os"
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

    // Set the response in ctx
    ctx.SetBody(response.Body())
    ctx.SetStatusCode(response.StatusCode())

    // Set Content-Type to text/plain
    ctx.Response.Header.SetContentType("text/plain")

    response.Header.VisitAll(func(key, value []byte) {
        ctx.Response.Header.Set(string(key), string(value))
    })
}

func makeRequest(ctx *fasthttp.RequestCtx, attempt int) *fasthttp.Response {
    if attempt > retries {
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Proxy failed to connect. Please try again."))
        resp.SetStatusCode(500)
        return resp
    }

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)
    req.Header.SetMethod(string(ctx.Method()))

    // Use net/url to parse the original URI
    originalURI := string(ctx.Request.Header.RequestURI())
    parsedURI, err := url.ParseRequestURI(originalURI)
    if err != nil {
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Invalid URL format."))
        resp.SetStatusCode(400)
        return resp
    }

    // Reconstruct the URL
    targetURL := "https://www.roblox.com" + parsedURI.Path
    if parsedURI.RawQuery != "" {
        targetURL += "?" + parsedURI.RawQuery
    }
    req.SetRequestURI(targetURL)

    // Copy request headers and body
    req.SetBody(ctx.Request.Body())
    ctx.Request.Header.VisitAll(func(key, value []byte) {
        req.Header.Set(string(key), string(value))
    })
    req.Header.Set("User-Agent", "RoProxy")
    req.Header.Del("Roblox-Id")

    // Make the request
    resp := fasthttp.AcquireResponse()
    err = client.Do(req, resp)
    if err != nil {
        fasthttp.ReleaseResponse(resp)
        return makeRequest(ctx, attempt + 1)
    } else {
        return resp
    }
}
