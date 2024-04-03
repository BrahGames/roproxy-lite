package main

import (
    "log"
    "net/url"
    "os"
    "strconv"
    "strings"
    "time"
    "github.com/valyala/fasthttp"
)

var (
    timeout, _ = strconv.Atoi(os.Getenv("TIMEOUT"))
    retries, _ = strconv.Atoi(os.Getenv("RETRIES"))
    port       = os.Getenv("PORT")
    client     = &fasthttp.Client{
        ReadTimeout:         time.Duration(timeout) * time.Second,
        MaxIdleConnDuration: 60 * time.Second,
    }
)

func main() {
    if err := fasthttp.ListenAndServe(":"+port, requestHandler); err != nil {
        log.Fatalf("Error in ListenAndServe: %s", err)
    }
}

func requestHandler(ctx *fasthttp.RequestCtx) {
    response := makeRequest(ctx, 1)
    defer fasthttp.ReleaseResponse(response)

    // Forwarding the response
    ctx.Response.SetStatusCode(response.StatusCode())
    ctx.Response.SetBody(response.Body())
    ctx.Response.Header.SetContentType(string(response.Header.Peek("Content-Type")))
}

func makeRequest(ctx *fasthttp.RequestCtx, attempt int) *fasthttp.Response {
    if attempt > retries {
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Proxy failed to connect. Please try again."))
        resp.SetStatusCode(fasthttp.StatusInternalServerError)
        return resp
    }

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)
    
    // Setting the method and headers for the new request
    req.Header.SetMethodBytes(ctx.Method())

    originalURI := string(ctx.Request.URI().FullURI())
    if !strings.HasPrefix(originalURI, "/proxy/") {
        log.Printf("Invalid request path: %s", originalURI)
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Invalid request path."))
        resp.SetStatusCode(fasthttp.StatusBadRequest)
        return resp
    }

    // Construct the target URL by removing the '/proxy/' prefix
    targetURL := "https://" + strings.TrimPrefix(originalURI, "/proxy/")
    req.SetRequestURI(targetURL)

    ctx.Request.Header.VisitAll(func(key, value []byte) {
        if string(key) != "Host" && string(key) != "User-Agent" {
            req.Header.SetBytesKV(key, value)
        }
    })

    // Setting a custom User-Agent for the proxy
    req.Header.Set("User-Agent", "CustomProxy")

    // Making the request to the target URL
    resp := fasthttp.AcquireResponse()
    err := client.Do(req, resp)
    if err != nil {
        log.Printf("Error making request to %s: %s", targetURL, err)
        fasthttp.ReleaseResponse(resp)
        return makeRequest(ctx, attempt+1)
    }

    return resp
}
