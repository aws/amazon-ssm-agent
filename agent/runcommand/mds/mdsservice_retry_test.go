// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identitymocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testTimeout = 50 * time.Millisecond

// fakeCreds hands out a new access key on every Retrieve, so each signature shows which fetch it used.
// failFrom > 0 makes that fetch and later ones fail; blockFrom > 0 makes them wait for the context.
type fakeCreds struct{ retrieved, invalidated, failFrom, blockFrom atomic.Int32 }

func (f *fakeCreds) Retrieve(ctx context.Context) (aws.Credentials, error) {
	n := f.retrieved.Add(1)
	if from := f.failFrom.Load(); from > 0 && n >= from {
		return aws.Credentials{}, fmt.Errorf("no credentials")
	}
	if from := f.blockFrom.Load(); from > 0 && n >= from {
		<-ctx.Done()
		return aws.Credentials{}, ctx.Err()
	}
	return aws.Credentials{AccessKeyID: fmt.Sprintf("AK%d", n), SecretAccessKey: "sk"}, nil
}

func (f *fakeCreds) Invalidate() { f.invalidated.Add(1) }

// mdsCall is one request as the server saw it.
type mdsCall struct {
	body string
	auth []string
}

// retryEnv is an MDS service wired by NewService to a local server.
type retryEnv struct {
	svc    Service
	log    *logmocks.Mock
	creds  *fakeCreds
	mu     sync.Mutex
	calls  []mdsCall
	delays []time.Duration
	conns  atomic.Int32
}

func (e *retryEnv) seen() ([]mdsCall, []time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]mdsCall(nil), e.calls...), append([]time.Duration(nil), e.delays...)
}

// newRetryEnv starts a server whose handler answers attempt n (from 0), and records retry delays instead of sleeping.
func newRetryEnv(t *testing.T, timeout time.Duration, handler func(n int, w http.ResponseWriter, r *http.Request)) *retryEnv {
	e := &retryEnv{log: logmocks.NewSilentMockLog(), creds: &fakeCreds{}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		e.mu.Lock()
		n := len(e.calls)
		e.calls = append(e.calls, mdsCall{body: string(body), auth: r.Header.Values("Authorization")})
		e.mu.Unlock()
		handler(n, w, r)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			e.conns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)

	saved := sleepDelay
	sleepDelay = func(d time.Duration) {
		e.mu.Lock()
		e.delays = append(e.delays, d)
		e.mu.Unlock()
	}
	t.Cleanup(func() { sleepDelay = saved })

	cfg := appconfig.SsmagentConfig{}
	cfg.Mds.Endpoint = srv.URL
	id := &identitymocks.IAgentIdentity{}
	id.On("Region").Return("us-east-1", nil)
	id.On("CredentialsProvider").Return(e.creds)
	ctx := &contextmocks.Mock{}
	ctx.On("Log").Return(e.log)
	ctx.On("AppConfig").Return(cfg)
	ctx.On("Identity").Return(id)
	e.svc = NewService(ctx, timeout)
	return e
}

// hold keeps the request open until the client gives up.
func hold(r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(5 * time.Second):
	}
}

func reply(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// failThenOK answers the first attempt with status and body, and later attempts with an empty 200.
func failThenOK(status int, body string) func(int, http.ResponseWriter, *http.Request) {
	return func(n int, w http.ResponseWriter, _ *http.Request) {
		if n == 0 {
			reply(w, status, body)
			return
		}
		reply(w, 200, "{}")
	}
}

func requestID(t *testing.T, body string) string {
	var in GetMessagesInput
	assert.NoError(t, json.Unmarshal([]byte(body), &in))
	assert.NotNil(t, in.MessagesRequestId)
	return *in.MessagesRequestId
}

func TestPollTimeoutResendsSameIdAfter100ms(t *testing.T) {
	e := newRetryEnv(t, testTimeout, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			hold(r)
			return
		}
		reply(w, 200, `{"Messages":[{"MessageId":"m-1"}]}`)
	})

	out, err := e.svc.GetMessages(e.log, "i-1")

	assert.NoError(t, err)
	if assert.Len(t, out.Messages, 1) {
		assert.Equal(t, "m-1", *out.Messages[0].MessageId)
	}
	calls, delays := e.seen()
	if assert.Len(t, calls, 2) {
		assert.Equal(t, requestID(t, calls[0].body), requestID(t, calls[1].body))
	}
	assert.Equal(t, []time.Duration{100 * time.Millisecond}, delays)
	assert.Equal(t, 1, retryLogCount(e.log, "Debugf"))
	for _, c := range e.log.Calls {
		assert.NotEqual(t, "Warnf", c.Method, "an idle poll logs no warning: %v", c.Arguments)
	}
}

// retryLogCount counts "Retrying MDS" lines logged at the given level.
func retryLogCount(log *logmocks.Mock, method string) int {
	n := 0
	for _, c := range log.Calls {
		if c.Method == method && strings.HasPrefix(c.Arguments.String(0), "Retrying MDS") {
			n++
		}
	}
	return n
}

func TestCallStopsAfterFourAttempts(t *testing.T) {
	e := newRetryEnv(t, testTimeout, func(_ int, w http.ResponseWriter, _ *http.Request) {
		reply(w, 500, `{"__type":"InternalServerException","message":"boom"}`)
	})

	err := e.svc.AcknowledgeMessage(e.log, "m-1")

	assert.ErrorContains(t, err, "boom")
	calls, _ := e.seen()
	assert.Len(t, calls, 4)
	assert.Equal(t, 3, retryLogCount(e.log, "Debugf"))
	assert.Zero(t, retryLogCount(e.log, "Warnf"), "retries are routine diagnostics")
}

func TestErrorRetriesBackOff1s2s4s(t *testing.T) {
	for name, tc := range map[string]struct {
		call    func(Service, *logmocks.Mock) error
		handler func(int, http.ResponseWriter, *http.Request)
	}{
		"ack timing out": {
			call:    func(s Service, l *logmocks.Mock) error { return s.AcknowledgeMessage(l, "m-1") },
			handler: func(_ int, _ http.ResponseWriter, r *http.Request) { hold(r) },
		},
		"poll getting 503": {
			call:    func(s Service, l *logmocks.Mock) error { _, err := s.GetMessages(l, "i-1"); return err },
			handler: func(_ int, w http.ResponseWriter, _ *http.Request) { reply(w, 503, `{}`) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, testTimeout, tc.handler)

			_ = tc.call(e.svc, e.log)

			_, delays := e.seen()
			if assert.Len(t, delays, 3) {
				for i, d := range delays {
					low := time.Duration(1000<<i) * time.Millisecond
					assert.True(t, d >= low && d < low*3/2, "delay %d = %v, want [%v, %v)", i, d, low, low*3/2)
				}
			}
		})
	}
}

func TestBackoffJitterSpansHalfASecond(t *testing.T) {
	var longest time.Duration
	for i := 0; i < 200; i++ {
		longest = max(longest, retryDelay(opAcknowledgeMessage, nil, 0))
	}
	assert.GreaterOrEqual(t, longest, 1400*time.Millisecond)
}

func TestRetryableStatusAndCodesAreRetried(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"bare ThrottlingException": {400, `{"__type":"ThrottlingException"}`},
	}
	for _, s := range []int{500, 429, 502, 503, 504} {
		cases[fmt.Sprint("status ", s)] = struct {
			status int
			body   string
		}{s, `{"__type":"SomeError"}`}
	}
	for _, code := range []string{
		"RequestError", "RequestTimeout", "ResponseTimeout", "RequestTimeoutException",
		"ProvisionedThroughputExceededException", "ThrottledException", "Throttling",
		"ThrottlingException", "RequestLimitExceeded", "RequestThrottled",
		"RequestThrottledException", "TooManyRequestsException", "PriorRequestNotComplete",
		"TransactionInProgressException", "EC2ThrottledException",
	} {
		cases[code] = struct {
			status int
			body   string
		}{400, `{"__type":"com.amazon.coral.availability#` + code + `"}`}
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, testTimeout, failThenOK(tc.status, tc.body))

			err := e.svc.AcknowledgeMessage(e.log, "m-1")

			assert.NoError(t, err)
			calls, _ := e.seen()
			assert.Len(t, calls, 2)
		})
	}
}

func TestClientErrorIsSentOnce(t *testing.T) {
	for name, body := range map[string]string{
		"invalid message id": `{"__type":"com.amazon#InvalidMessageIdException","message":"bad id"}`,
		"empty body":         ``,
		"blank body":         "  \n",
		"two # in type":      `{"__type":"a#b#ThrottlingException"}`,
		"clock skew":         `{"__type":"RequestExpired"}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, testTimeout, failThenOK(400, body))

			err := e.svc.AcknowledgeMessage(e.log, "m-1")

			assert.Error(t, err)
			calls, _ := e.seen()
			assert.Len(t, calls, 1)
		})
	}
}

// ack and poll call a reply-less and a reply-carrying MDS API.
var (
	ack  = func(s Service, l *logmocks.Mock) error { return s.AcknowledgeMessage(l, "m-1") }
	poll = func(s Service, l *logmocks.Mock) error { _, err := s.GetMessages(l, "i-1"); return err }
)

func TestUnparseableReplyIsRetriedThenFails(t *testing.T) {
	for name, tc := range map[string]struct {
		call   func(Service, *logmocks.Mock) error
		status int
		body   string
	}{
		"html error page":     {ack, 400, `<html>proxy error</html>`},
		"json string error":   {ack, 400, `"x"`},
		"json array error":    {ack, 400, `[]`},
		"numeric error type":  {ack, 400, `{"__type":42}`},
		"numeric message":     {ack, 400, `{"message":42}`},
		"bad 200 poll answer": {poll, 200, `not json`},
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, testTimeout, func(_ int, w http.ResponseWriter, _ *http.Request) { reply(w, tc.status, tc.body) })

			err := tc.call(e.svc, e.log)

			if tc.status >= 300 {
				assert.ErrorContains(t, err, fmt.Sprintf("HTTP %d: %s", tc.status, tc.body), "the reply body stays in the error")
			} else {
				assert.Error(t, err)
			}
			calls, _ := e.seen()
			assert.Len(t, calls, 4)
		})
	}
}

func TestNoReplyCallsIgnoreTheReplyBody(t *testing.T) {
	calls := map[string]func(Service, *logmocks.Mock) error{
		"ack":    ack,
		"delete": func(s Service, l *logmocks.Mock) error { return s.DeleteMessage(l, "m-1") },
		"fail":   func(s Service, l *logmocks.Mock) error { return s.FailMessage(l, "m-1", InternalHandlerException) },
	}
	bodies := map[string]func(*testing.T, http.ResponseWriter){
		"not json": func(_ *testing.T, w http.ResponseWriter) { reply(w, 200, `not json`) },
		"truncated": func(t *testing.T, w http.ResponseWriter) {
			hijackAndClose(t, w, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n{", false)
		},
	}
	for callName, call := range calls {
		for bodyName, handler := range bodies {
			t.Run(callName+" "+bodyName, func(t *testing.T) {
				e := newRetryEnv(t, time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) { handler(t, w) })

				assert.NoError(t, call(e.svc, e.log))

				seen, _ := e.seen()
				assert.Len(t, seen, 1)
			})
		}
	}
}

func TestExpiredTokenDropsCachedCredentials(t *testing.T) {
	for _, code := range []string{"ExpiredToken", "ExpiredTokenException"} {
		t.Run(code, func(t *testing.T) {
			e := newRetryEnv(t, testTimeout, failThenOK(403, `{"__type":"com.amazon.coral.service#`+code+`"}`))

			err := e.svc.AcknowledgeMessage(e.log, "m-1")

			assert.NoError(t, err)
			assert.Equal(t, int32(1), e.creds.invalidated.Load())
			calls, _ := e.seen()
			assert.Len(t, calls, 2)
		})
	}
}

func TestExpiredRepliesDropCredentialsWarnOnce(t *testing.T) {
	e := newRetryEnv(t, testTimeout, func(_ int, w http.ResponseWriter, _ *http.Request) {
		reply(w, 403, `{"__type":"ExpiredTokenException"}`)
	})

	assert.Error(t, ack(e.svc, e.log))

	assert.Equal(t, int32(4), e.creds.invalidated.Load(), "dropped after every expired reply, so the next call signs fresh")
	warned := 0
	for _, c := range e.log.Calls {
		if c.Method == "Warnf" && strings.HasPrefix(c.Arguments.String(0), "MDS %s got %s; dropping cached credentials") {
			warned++
		}
	}
	assert.Equal(t, 1, warned)
}

func TestStopEndsTheCallWithoutAnotherSend(t *testing.T) {
	for name, tc := range map[string]struct {
		handler   func(http.ResponseWriter)
		maxSleeps int // a body read cut by Stop may wait out one retry sleep
	}{
		"while waiting for headers": {func(http.ResponseWriter) {}, 0},
		"while reading the reply": {func(w http.ResponseWriter) {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
		}, 1},
		"while reading an error reply": {func(w http.ResponseWriter) {
			w.WriteHeader(503)
			w.(http.Flusher).Flush()
		}, 1},
	} {
		t.Run(name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			e := newRetryEnv(t, 5*time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) {
				tc.handler(w)
				close(started)
				<-release // keep the reply open, so the agent's read ends only by Stop
			})
			t.Cleanup(func() { close(release) })
			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = e.svc.GetMessages(e.log, "i-1")
			}()
			<-started

			e.svc.Stop()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("GetMessages still running 1s after Stop")
			}
			calls, delays := e.seen()
			assert.Len(t, calls, 1)
			assert.LessOrEqual(t, len(delays), tc.maxSleeps)
			for _, d := range delays {
				assert.GreaterOrEqual(t, d, time.Second, "a canceled read is not a poll timeout")
			}
		})
	}
}

func TestRetryIsSignedAgainWithFreshCredentials(t *testing.T) {
	e := newRetryEnv(t, testTimeout, failThenOK(503, `{}`))

	assert.NoError(t, e.svc.AcknowledgeMessage(e.log, "m-1"))

	assert.Zero(t, e.creds.invalidated.Load(), "a 503 is not a credential error")
	calls, _ := e.seen()
	if assert.Len(t, calls, 2) {
		for i, c := range calls {
			if assert.Len(t, c.auth, 1) {
				assert.Contains(t, c.auth[0], fmt.Sprintf("Credential=AK%d/", i+1))
			}
		}
	}
}

func TestExhaustedCallLogsAtDebug(t *testing.T) {
	e := newRetryEnv(t, testTimeout, func(_ int, w http.ResponseWriter, _ *http.Request) { reply(w, 500, `{}`) })

	_ = ack(e.svc, e.log)

	exhausted := mock.MatchedBy(func(f string) bool { return strings.HasPrefix(f, "MDS %s failed after") })
	e.log.AssertCalled(t, "Debugf", exhausted, mock.Anything)
	e.log.AssertNotCalled(t, "Warnf", exhausted, mock.Anything)
}

func TestRetrySigningFailureLogsAndStops(t *testing.T) {
	e := newRetryEnv(t, testTimeout, failThenOK(503, `{}`))
	e.creds.failFrom.Store(2) // the first signature works, the retry's fails

	assert.ErrorContains(t, e.svc.AcknowledgeMessage(e.log, "m-1"), "no credentials")

	assert.Equal(t, int32(2), e.creds.retrieved.Load())
	calls, _ := e.seen()
	assert.Len(t, calls, 1)
	e.log.AssertCalled(t, "Debugf", mock.MatchedBy(func(f string) bool { return strings.HasPrefix(f, "MDS %s request could not be signed") }), mock.Anything)
}

// hijackAndClose sends raw, then closes the connection; reset makes the close an RST.
func hijackAndClose(t *testing.T, w http.ResponseWriter, raw string, reset bool) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if !assert.NoError(t, err) {
		return
	}
	_, _ = io.WriteString(conn, raw)
	if tcp, ok := conn.(*net.TCPConn); ok && reset {
		_ = tcp.SetLinger(0)
	}
	_ = conn.Close()
}

func TestResetAfterSendIsNotResent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("an RST on close is only deterministic on Linux; TestNetErrorsRetriedUnlessServerMayHaveActed covers the rule")
	}
	for name, tc := range map[string]struct {
		call  func(Service, *logmocks.Mock) error
		raw   string
		sends int
	}{
		"before the reply":         {ack, "", 1},
		"while reading the reply":  {poll, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n{", 1},
		"while reading a 503 body": {ack, "HTTP/1.1 503 Busy\r\nContent-Length: 100\r\n\r\n{", 4},
		"while reading a 400 body": {ack, "HTTP/1.1 400 Bad\r\nContent-Length: 100\r\n\r\n{\"__ty", 1},
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) {
				hijackAndClose(t, w, tc.raw, true)
			})

			assert.ErrorContains(t, tc.call(e.svc, e.log), "connection reset")

			calls, _ := e.seen()
			assert.Len(t, calls, tc.sends)
		})
	}
}

func TestTruncatedReplyIsRetried(t *testing.T) {
	e := newRetryEnv(t, time.Second, func(n int, w http.ResponseWriter, _ *http.Request) {
		if n == 0 {
			hijackAndClose(t, w, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n{", false)
			return
		}
		reply(w, 200, "{}")
	})

	assert.NoError(t, poll(e.svc, e.log))

	calls, _ := e.seen()
	assert.Len(t, calls, 2)
}

func TestNetErrorsRetriedUnlessServerMayHaveActed(t *testing.T) {
	for name, tc := range map[string]struct {
		err   error
		retry bool
	}{
		"refused":           {&url.Error{Op: "Post", Err: errors.New("dial tcp: connect: connection refused")}, true},
		"dial failure":      {&url.Error{Op: "Post", Err: &net.OpError{Op: "dial", Err: errors.New("no route")}}, true},
		"read reset":        {&url.Error{Op: "Post", Err: &net.OpError{Op: "read", Err: errors.New("read: connection reset by peer")}}, false},
		"proxy refused":     {&url.Error{Op: "Post", Err: &net.OpError{Op: "proxyconnect", Err: errors.New("connect: connection refused")}}, true},
		"write reset":       {&url.Error{Op: "Post", Err: &net.OpError{Op: "write", Err: errors.New("write: connection reset by peer")}}, true},
		"closed connection": {&url.Error{Op: "Post", Err: &net.OpError{Op: "read", Err: errors.New("use of closed network connection")}}, true},
		"write broken pipe": {&url.Error{Op: "Post", Err: &net.OpError{Op: "write", Err: errors.New("write: broken pipe")}}, true},
		"other net error":   {&url.Error{Op: "Post", Err: &net.OpError{Op: "read", Err: errors.New("bad record")}}, false},
		"unexpected EOF":    {io.ErrUnexpectedEOF, true},
		"request canceled":  {errors.New("net/http: request canceled"), false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.retry, isRetryableNetError(tc.err))
		})
	}
}

func TestUncachedProviderLogsItKeepsCredentials(t *testing.T) {
	log := logmocks.NewSilentMockLog()
	c := &Ssmmds{log: log, credentialProvider: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, nil
	})}

	c.invalidateCredentials()

	log.AssertCalled(t, "Debugf", mock.MatchedBy(func(f string) bool { return strings.HasPrefix(f, "MDS credentials provider") }), mock.Anything)
}

func TestCutShortErrorBodyIsRetried(t *testing.T) {
	e := newRetryEnv(t, time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) {
		hijackAndClose(t, w, "HTTP/1.1 400 Bad Request\r\nContent-Length: 100\r\n\r\n", false)
	})

	assert.Error(t, ack(e.svc, e.log))

	calls, _ := e.seen()
	assert.Len(t, calls, 4)
}

func TestStopDuringCredentialFetchReturns(t *testing.T) {
	e := newRetryEnv(t, 5*time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) { reply(w, 200, "{}") })
	e.creds.blockFrom.Store(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = poll(e.svc, e.log)
	}()
	assert.Eventually(t, func() bool { return e.creds.retrieved.Load() == 1 }, time.Second, time.Millisecond)

	e.svc.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("GetMessages still fetching credentials 1s after Stop")
	}
}

func TestBadPollAnswerLeavesNoMessages(t *testing.T) {
	e := newRetryEnv(t, testTimeout, func(n int, w http.ResponseWriter, _ *http.Request) {
		if n == 0 {
			reply(w, 200, `{"Messages":[{"MessageId":"m-1"}],"MessagesRequestId":5}`)
			return
		}
		reply(w, 200, "  ")
	})

	out, err := e.svc.GetMessages(e.log, "i-1")

	assert.NoError(t, err)
	assert.Empty(t, out.Messages)
	calls, _ := e.seen()
	assert.Len(t, calls, 2)
}

// recordingTransport answers each attempt in turn and keeps every request it was given.
type recordingTransport struct {
	statuses []int
	seen     []*http.Request
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	status := r.statuses[len(r.seen)]
	r.seen = append(r.seen, req)
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}}, nil
}

func TestRetryLeavesEarlierAttemptHeadersAlone(t *testing.T) {
	saved := sleepDelay
	sleepDelay = func(time.Duration) {}
	t.Cleanup(func() { sleepDelay = saved })
	rt := &recordingTransport{statuses: []int{503, 200}}
	c := &Ssmmds{client: &http.Client{Transport: rt}, endpoint: "http://mds.test", region: "us-east-1",
		credentialProvider: &fakeCreds{}, log: logmocks.NewSilentMockLog()}
	req, _ := c.AcknowledgeMessageRequest(&AcknowledgeMessageInput{MessageId: aws.String("m-1")})

	assert.NoError(t, c.executeRequest(req, req.Data))

	if assert.Len(t, rt.seen, 2) {
		assert.Contains(t, rt.seen[0].Header.Get("Authorization"), "Credential=AK1/")
		assert.Contains(t, rt.seen[1].Header.Get("Authorization"), "Credential=AK2/")
	}
}

func TestDefaultRetryWaitReallySleeps(t *testing.T) {
	start := time.Now()

	sleepDelay(20 * time.Millisecond)

	assert.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
}

func TestRedirectReplyIsAnError(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		sends  int
	}{
		"300 with an html page": {300, `<html>moved</html>`, 4},
		"304 with no body":      {304, ``, 1},
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, testTimeout, func(_ int, w http.ResponseWriter, _ *http.Request) { reply(w, tc.status, tc.body) })

			assert.Error(t, ack(e.svc, e.log))

			calls, _ := e.seen()
			assert.Len(t, calls, tc.sends)
		})
	}
}

func TestCompleteReplyCountsDespiteEarlyClose(t *testing.T) {
	cut := func(status, body string) string {
		return "HTTP/1.1 " + status + "\r\nContent-Length: 500\r\n\r\n" + body
	}
	for name, tc := range map[string]struct {
		raw     string
		call    func(Service, *logmocks.Mock) error
		wantErr bool
	}{
		"poll answer":            {cut("200 OK", `{"Messages":[{"MessageId":"m-1"}]}`), poll, false},
		"poll answer then reset": {"RST:" + cut("200 OK", `{"Messages":[{"MessageId":"m-1"}]}`), poll, false},
		"ack reply":              {cut("200 OK", `{}`), ack, false},
		"client error":           {cut("400 Bad", `{"__type":"InvalidMessageIdException"}`), ack, true},
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetryEnv(t, time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) {
				raw, reset := strings.CutPrefix(tc.raw, "RST:")
				hijackAndClose(t, w, raw, reset)
			})

			err := tc.call(e.svc, e.log)

			assert.Equal(t, tc.wantErr, err != nil, "err = %v", err)
			calls, _ := e.seen()
			assert.Len(t, calls, 1)
		})
	}
}

// cancelingTransport cancels the call's context as the reply arrives, like a Stop() racing a finished poll.
type cancelingTransport struct{ cancel context.CancelFunc }

func (c cancelingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.cancel()
	return &http.Response{StatusCode: 200, Header: http.Header{},
		Body: io.NopCloser(strings.NewReader(`{"Messages":[{"MessageId":"m-1"}]}`))}, nil
}

func TestStopAfterFullReplyKeepsTheReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Ssmmds{client: &http.Client{Transport: cancelingTransport{cancel}}, endpoint: "http://mds.test",
		region: "us-east-1", credentialProvider: &fakeCreds{}, log: logmocks.NewSilentMockLog()}
	req, out := c.GetMessagesRequest(&GetMessagesInput{Destination: aws.String("i-1")})
	req.SetContext(ctx)

	assert.NoError(t, c.executeRequest(req, req.Data))

	assert.Len(t, out.Messages, 1)
}

func TestAckRepliesReuseTheConnection(t *testing.T) {
	e := newRetryEnv(t, time.Second, func(_ int, w http.ResponseWriter, _ *http.Request) { reply(w, 200, `{"ok":true}`) })

	assert.NoError(t, ack(e.svc, e.log))
	assert.NoError(t, ack(e.svc, e.log))

	assert.Equal(t, int32(1), e.conns.Load())
}

// cancelOnReadTransport answers with a body whose read is cut by a Stop, then fails once stopped.
type cancelOnReadTransport struct {
	status int
	cancel context.CancelFunc
	trips  *int
}

type cancelingBody struct{ cancel context.CancelFunc }

func (b cancelingBody) Read([]byte) (int, error) { b.cancel(); return 0, context.Canceled }
func (b cancelingBody) Close() error             { return nil }

func (c cancelOnReadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*c.trips++
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: c.status, Header: http.Header{}, Body: cancelingBody{c.cancel}}, nil
}

func TestStoppedReadWaitsOneRetrySleep(t *testing.T) {
	for _, status := range []int{200, 400} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var delays []time.Duration
			saved := sleepDelay
			sleepDelay = func(d time.Duration) { delays = append(delays, d) }
			t.Cleanup(func() { sleepDelay = saved })
			ctx, cancel := context.WithCancel(context.Background())
			trips := 0
			c := &Ssmmds{client: &http.Client{Transport: cancelOnReadTransport{status, cancel, &trips}}, endpoint: "http://mds.test",
				region: "us-east-1", credentialProvider: &fakeCreds{}, log: logmocks.NewSilentMockLog()}
			req, _ := c.GetMessagesRequest(&GetMessagesInput{Destination: aws.String("i-1")})
			req.SetContext(ctx)

			assert.ErrorIs(t, c.executeRequest(req, req.Data), context.Canceled)

			if assert.Len(t, delays, 1) {
				assert.GreaterOrEqual(t, delays[0], time.Second)
			}
			assert.Equal(t, 2, trips, "the retry after the sleep fails fast")
		})
	}
}
