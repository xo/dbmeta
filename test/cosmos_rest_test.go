package test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The REST client of the Cosmos DB fixture.
//
// The SQL of Cosmos DB reads and never writes, and the driver of dbimp reads
// only (D190, D228). A database, a container, a stored procedure and a user are
// resources of the REST API, so the fixture sends the requests itself. The
// signature of a request is the HMAC of its verb, resource and date under the
// account key, and the standard library has all of it. The key is read from the
// DSN and held as bytes, and no error or log line of this file ever holds it:
// url.Parse and the transport put the text they fail on into their errors, so
// every error here is a fixed message.

// cosmosAPIVersion is the version of the REST API that the requests name. It is
// the one the driver of dbimp names.
const cosmosAPIVersion = "2018-12-31"

// cosmosAccount is one account and the database that the fixture builds in.
type cosmosAccount struct {
	// base is the endpoint, such as https://name.documents.azure.com.
	base string
	// database is the database that the path of the DSN names.
	database string
	// container is the container that the path of the DSN names, if it names one.
	container string

	key    []byte
	client *http.Client
}

// parseCosmosDSN reads a DSN of the driver of dbimp:
// cosmos://x:key@account.documents.azure.com/database/container. The key is
// the password, in base64 text.
func parseCosmosDSN(dsn string) (*cosmosAccount, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, errors.New("the DSN is not a URL")
	}
	if u.Scheme != "cosmos" || u.Host == "" || u.User == nil {
		return nil, errors.New("the DSN is not of the form cosmos://user:key@host/database")
	}
	pass, _ := u.User.Password()
	key, err := base64.StdEncoding.DecodeString(pass)
	if err != nil || len(key) == 0 {
		return nil, errors.New("the key of the DSN is not base64 text")
	}
	a := &cosmosAccount{
		base:   "https://" + u.Host,
		key:    key,
		client: &http.Client{Timeout: time.Minute},
	}
	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 2)
	a.database = parts[0]
	if len(parts) == 2 {
		a.container = parts[1]
	}
	return a, nil
}

// cosmosResource returns the type and the link of the resource that path names,
// as the text that the key signs holds them. A path of an odd number of
// segments names a list, and its link is the parent. A path of an even number
// names one resource, and its link is the path.
func cosmosResource(path string) (string, string) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) == 1 && segs[0] == "" {
		return "", ""
	}
	if len(segs)%2 == 1 {
		return segs[len(segs)-1], strings.Join(segs[:len(segs)-1], "/")
	}
	return segs[len(segs)-2], strings.Join(segs, "/")
}

// sign sets the headers of a request that the master key signs.
func (a *cosmosAccount) sign(req *http.Request) {
	date := strings.ToLower(time.Now().UTC().Format(http.TimeFormat))
	kind, link := cosmosResource(req.URL.Path)
	text := strings.ToLower(req.Method) + "\n" + strings.ToLower(kind) + "\n" + link + "\n" + date + "\n\n"
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(text))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set("X-Ms-Date", date)
	req.Header.Set("X-Ms-Version", cosmosAPIVersion)
	req.Header.Set("Authorization", url.QueryEscape("type=master&ver=1.0&sig="+sig))
}

// cosmosReply is the answer to one request.
type cosmosReply struct {
	Status int
	Body   []byte
	// Charge is the request units that the request cost.
	Charge float64
}

// do sends one request. body is nil, a string, or a value that is sent as JSON.
// It sends the request again after HTTP 429, for as long as ctx lasts.
func (a *cosmosAccount) do(ctx context.Context, method, path string, header map[string]string, body any) (cosmosReply, error) {
	var b []byte
	switch body := body.(type) {
	case nil:
	case string:
		b = []byte(body)
	default:
		var err error
		if b, err = json.Marshal(body); err != nil {
			return cosmosReply{}, errors.New("writing the body")
		}
	}
	for {
		req, err := http.NewRequestWithContext(ctx, method, a.base+path, bytes.NewReader(b))
		if err != nil {
			return cosmosReply{}, errors.New("making the request")
		}
		req.Header.Set("Accept", "application/json")
		if len(b) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		a.sign(req)
		res, err := a.client.Do(req)
		if err != nil {
			return cosmosReply{}, fmt.Errorf("sending %s %s: the request failed", method, path)
		}
		out, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return cosmosReply{}, fmt.Errorf("reading the answer to %s %s", method, path)
		}
		charge, _ := strconv.ParseFloat(res.Header.Get("X-Ms-Request-Charge"), 64)
		reply := cosmosReply{Status: res.StatusCode, Body: out, Charge: charge}
		if res.StatusCode != http.StatusTooManyRequests {
			return reply, nil
		}
		wait := time.Second
		if ms, err := strconv.Atoi(res.Header.Get("X-Ms-Retry-After-Ms")); err == nil && ms > 0 {
			wait = time.Duration(ms) * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return reply, fmt.Errorf("waiting to send %s %s: %w", method, path, ctx.Err())
		case <-time.After(wait):
		}
	}
}
