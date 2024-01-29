package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

var timeout, _ = strconv.Atoi(os.Getenv("TIMEOUT"))
var retries, _ = strconv.Atoi(os.Getenv("RETRIES"))
var port = os.Getenv("PORT")
var proxyURL = "YOUR_ORIGINAL_URL" // Replace with the actual URL you want to proxy

var client *fasthttp.Client

func main() {
	r := router.New()

	client = &fasthttp.Client{
		ReadTimeout:        time.Duration(timeout) * time.Second,
		MaxIdleConnDuration: 60 * time.Second,
	}

	r.GET("/*path", requestHandler)

	if err := fasthttp.ListenAndServe(":"+port, r.Handler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	// Extract the requested path from the URL
	requestedPath := string(ctx.Path())

	// Construct the URL to proxy to
	url := proxyURL + requestedPath

	response := makeRequest(ctx, url, 1)

	body := response.Body()
	ctx.SetBody(body)
	ctx.SetStatusCode(response.StatusCode())
	response.Header.VisitAll(func(key, value []byte) {
		ctx.Response.Header.Set(string(key), string(value))
	})
}

func makeRequest(ctx *fasthttp.RequestCtx, url string, attempt int) *fasthttp.Response {
	if attempt > retries {
		resp := fasthttp.AcquireResponse()
		resp.SetBody([]byte("Proxy failed to connect. Please try again."))
		resp.SetStatusCode(500)

		return resp
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetMethod(string(ctx.Method()))
	req.SetRequestURI(url)
	req.SetBody(ctx.Request.Body())
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})
	req.Header.Set("User-Agent", "RoProxy")
	req.Header.Del("Roblox-Id")
	resp := fasthttp.AcquireResponse()

	err := client.Do(req, resp)

	if err != nil {
		fasthttp.ReleaseResponse(resp)
		return makeRequest(ctx, url, attempt+1)
	} else {
		return resp
	}
}
