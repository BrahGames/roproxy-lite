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
    response := makeRequest(ctx, 1)
    defer fasthttp.ReleaseResponse(response)

    // Check if response is in JSON format
    contentType := response.Header.Peek("Content-Type")
    if !bytes.HasPrefix(contentType, []byte("application/json")) {
        ctx.SetBody([]byte("Unsupported format. Only JSON responses are supported."))
        ctx.SetStatusCode(fasthttp.StatusUnsupportedMediaType)
        return
    }

    // Set the response body in ctx
    ctx.SetBody(response.Body())
    ctx.SetStatusCode(response.StatusCode())

    // Set Content-Type to application/json
    ctx.Response.Header.SetContentType("application/json")
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

    // Check if request is in JSON format
    requestContentType := ctx.Request.Header.Peek("Content-Type")
    if !bytes.HasPrefix(requestContentType, []byte("application/json")) {
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Unsupported format. Only JSON requests are supported."))
        resp.SetStatusCode(fasthttp.StatusUnsupportedMediaType)
        return resp
    }

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
        // Check if response is in JSON format
        responseContentType := resp.Header.Peek("Content-Type")
        if !bytes.HasPrefix(responseContentType, []byte("application/json")) {
            fasthttp.ReleaseResponse(resp)
            resp = fasthttp.AcquireResponse()
            resp.SetBody([]byte("Unsupported format. Only JSON responses are supported."))
            resp.SetStatusCode(fasthttp.StatusUnsupportedMediaType)
            return resp
        }

        return resp
    }
}
