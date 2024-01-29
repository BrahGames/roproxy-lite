package main

import (
	"log"
	"time"
	"os"
	"github.com/valyala/fasthttp"
	"strconv"
	"strings"
)

var timeout, _ = strconv.Atoi(os.Getenv("TIMEOUT"))
var retries, _ = strconv.Atoi(os.Getenv("RETRIES"))
var port = os.Getenv("PORT")

var client *fasthttp.Client

func main() {
	h := requestHandler
	
	client = &fasthttp.Client{
		ReadTimeout: time.Duration(timeout) * time.Second,
		MaxIdleConnDuration: 60 * time.Second,
	}

	if err := fasthttp.ListenAndServe(":" + port, h); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
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

    // Extract the path and query from the original URI
    originalURI := string(ctx.Request.Header.RequestURI())
    pathAndQuery := strings.SplitN(originalURI, "?", 2)
    var path, query string
    path = pathAndQuery[0]

    if len(pathAndQuery) > 1 {
        query = "?" + pathAndQuery[1]
    }

    // Construct the target URL with both path and query
    targetURL := "https://www.roblox.com" + path + query
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
    err := client.Do(req, resp)
    if err != nil {
        fasthttp.ReleaseResponse(resp)
        return makeRequest(ctx, attempt + 1)
    } else {
        return resp
    }
}
