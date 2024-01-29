why does this link say it is not found, when it is suppose to be found and the path is valid. It has to do with the main logic in the go code. It has to do with the URL parsing. Please fix the code for me.

https://roproxy-rbx-b06be4a9cbcb.herokuapp.com/users/favorites/list-json?assetTypeId=9&userId=3636151326&itemsPerPage=1000000&pageNumber=1

go code:
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

func requestHandler(ctx *fasthttp.RequestCtx) {
	val, ok := os.LookupEnv("KEY")

	if ok && string(ctx.Request.Header.Peek("PROXYKEY")) != val {
		ctx.SetStatusCode(407)
		ctx.SetBody([]byte("Missing or invalid PROXYKEY header."))
		return
	}

	if len(strings.SplitN(string(ctx.Request.Header.RequestURI())[1:], "/", 2)) < 2 {
		ctx.SetStatusCode(400)
		ctx.SetBody([]byte("URL format invalid."))
		return
	}

	response := makeRequest(ctx, 1)

	defer fasthttp.ReleaseResponse(response)

	body := response.Body()
	ctx.SetBody(body)
	ctx.SetStatusCode(response.StatusCode())
	response.Header.VisitAll(func (key, value []byte) {
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

    // Correctly reconstruct the URL
    originalURI := string(ctx.Request.Header.RequestURI())
    splitIndex := strings.Index(originalURI[1:], "/") + 1
    if splitIndex <= 0 {
        resp := fasthttp.AcquireResponse()
        resp.SetBody([]byte("Invalid URL format."))
        resp.SetStatusCode(400)
        return resp
    }

    baseDomain := originalURI[1:splitIndex]
    remainingPath := originalURI[splitIndex:]
    req.SetRequestURI("https://" + baseDomain + "roblox.com" + remainingPath)

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
