package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/fkwhite/quic-go"

	"golang.org/x/net/http/httpguts"
)

type roundTripCloser interface {
	RoundTripOpt(*http.Request, RoundTripOpt) (*http.Response, error)
	io.Closer
}

// RoundTripper implements the http.RoundTripper interface
type RoundTripper struct {
	mutex sync.Mutex

	// DisableCompression, if true, prevents the Transport from
	// requesting compression with an "Accept-Encoding: gzip"
	// request header when the Request contains no existing
	// Accept-Encoding value. If the Transport requests gzip on
	// its own and gets a gzipped response, it's transparently
	// decoded in the Response.Body. However, if the user
	// explicitly requested gzip it is not automatically
	// uncompressed.
	DisableCompression bool

	// TLSClientConfig specifies the TLS configuration to use with
	// tls.Client. If nil, the default configuration is used.
	TLSClientConfig *tls.Config

	// QuicConfig is the quic.Config used for dialing new connections.
	// If nil, reasonable default values will be used.
	QuicConfig *quic.Config

	// Enable support for HTTP/3 datagrams.
	// If set to true, QuicConfig.EnableDatagram will be set.
	// See https://www.ietf.org/archive/id/draft-schinazi-masque-h3-datagram-02.html.
	EnableDatagrams bool

	// Additional HTTP/3 settings.
	// It is invalid to specify any settings defined by the HTTP/3 draft and the datagram draft.
	AdditionalSettings map[uint64]uint64

	// When set, this callback is called for the first unknown frame parsed on a bidirectional stream.
	// It is called right after parsing the frame type.
	// If parsing the frame type fails, the error is passed to the callback.
	// In that case, the frame type will not be set.
	// Callers can either ignore the frame and return control of the stream back to HTTP/3
	// (by returning hijacked false).
	// Alternatively, callers can take over the QUIC stream (by returning hijacked true).
	StreamHijacker func(FrameType, quic.Connection, quic.Stream, error) (hijacked bool, err error)

	// When set, this callback is called for unknown unidirectional stream of unknown stream type.
	// If parsing the stream type fails, the error is passed to the callback.
	// In that case, the stream type will not be set.
	UniStreamHijacker func(StreamType, quic.Connection, quic.ReceiveStream, error) (hijacked bool)

	// Dial specifies an optional dial function for creating QUIC
	// connections for requests.
	// If Dial is nil, quic.DialAddrEarlyContext will be used.
	Dial func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error)

	// MaxResponseHeaderBytes specifies a limit on how many response bytes are
	// allowed in the server's response header.
	// Zero means to use a default limit.
	MaxResponseHeaderBytes int64

	clients map[string]roundTripCloser
}

// RoundTripOpt are options for the Transport.RoundTripOpt method.
type RoundTripOpt struct {
	// OnlyCachedConn controls whether the RoundTripper may create a new QUIC connection.
	// If set true and no cached connection is available, RoundTripOpt will return ErrNoCachedConn.
	OnlyCachedConn bool
	// DontCloseRequestStream controls whether the request stream is closed after sending the request.
	// If set, context cancellations have no effect after the response headers are received.
	DontCloseRequestStream bool
}

var (
	_ http.RoundTripper = &RoundTripper{}
	_ io.Closer         = &RoundTripper{}
)

// ErrNoCachedConn is returned when RoundTripper.OnlyCachedConn is set
var ErrNoCachedConn = errors.New("http3: no cached connection was available")

// RoundTripOpt is like RoundTrip, but takes options.
func (r *RoundTripper) RoundTripOpt(req *http.Request, opt RoundTripOpt) (*http.Response, error) {
	if req.URL == nil {
		closeRequestBody(req)
		return nil, errors.New("http3: nil Request.URL")
	}
	if req.URL.Host == "" {
		closeRequestBody(req)
		return nil, errors.New("http3: no Host in request URL")
	}
	if req.Header == nil {
		closeRequestBody(req)
		return nil, errors.New("http3: nil Request.Header")
	}

	if req.URL.Scheme == "https" {
		for k, vv := range req.Header {
			if !httpguts.ValidHeaderFieldName(k) {
				return nil, fmt.Errorf("http3: invalid http header field name %q", k)
			}
			for _, v := range vv {
				if !httpguts.ValidHeaderFieldValue(v) {
					return nil, fmt.Errorf("http3: invalid http header field value %q for key %v", v, k)
				}
			}
		}
	} else {
		closeRequestBody(req)
		return nil, fmt.Errorf("http3: unsupported protocol scheme: %s", req.URL.Scheme)
	}

	if req.Method != "" && !validMethod(req.Method) {
		closeRequestBody(req)
		return nil, fmt.Errorf("http3: invalid method %q", req.Method)
	}

	hostname := authorityAddr("https", hostnameFromRequest(req))
	cl, err := r.getClient(hostname, opt.OnlyCachedConn)
	if err != nil {
		return nil, err
	}
	return cl.RoundTripOpt(req, opt)
}

// RoundTrip does a round trip.
func (r *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.RoundTripOpt(req, RoundTripOpt{})
}

func (r *RoundTripper) getClient(hostname string, onlyCached bool) (roundTripCloser, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.clients == nil {
		r.clients = make(map[string]roundTripCloser)
	}

	client, ok := r.clients[hostname]
	if !ok {
		if onlyCached {
			return nil, ErrNoCachedConn
		}
		var err error
		client, err = newClient(
			hostname,
			r.TLSClientConfig,
			&roundTripperOpts{
				EnableDatagram:     r.EnableDatagrams,
				DisableCompression: r.DisableCompression,
				MaxHeaderBytes:     r.MaxResponseHeaderBytes,
				StreamHijacker:     r.StreamHijacker,
				UniStreamHijacker:  r.UniStreamHijacker,
			},
			r.QuicConfig,
			r.Dial,
		)
		if err != nil {
			return nil, err
		}
		r.clients[hostname] = client
	}
	return client, nil
}

// Close closes the QUIC connections that this RoundTripper has used
func (r *RoundTripper) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, client := range r.clients {
		if err := client.Close(); err != nil {
			return err
		}
	}
	r.clients = nil
	return nil
}

func closeRequestBody(req *http.Request) {
	if req.Body != nil {
		req.Body.Close()
	}
}

func validMethod(method string) bool {
	/*
				     Method         = "OPTIONS"                ; Section 9.2
		   		                    | "GET"                    ; Section 9.3
		   		                    | "HEAD"                   ; Section 9.4
		   		                    | "POST"                   ; Section 9.5
		   		                    | "PUT"                    ; Section 9.6
		   		                    | "DELETE"                 ; Section 9.7
		   		                    | "TRACE"                  ; Section 9.8
		   		                    | "CONNECT"                ; Section 9.9
		   		                    | extension-method
		   		   extension-method = token
		   		     token          = 1*<any CHAR except CTLs or separators>
	*/
	return len(method) > 0 && strings.IndexFunc(method, isNotToken) == -1
}

// copied from net/http/http.go
func isNotToken(r rune) bool {
	return !httpguts.IsTokenRune(r)
}
