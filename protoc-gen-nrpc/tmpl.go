package main

const tFile = `// This code was autogenerated from {{.GetName}}, do not edit.

{{- $pkgName := GoPackageName .}}
{{- $pkgSubject := GetPkgSubject .}}
{{- $pkgSubjectPrefix := GetPkgSubjectPrefix .}}
{{- $pkgSubjectParams := GetPkgSubjectParams .}}
package {{$pkgName}}

import (
	"context"
	"log"
	"time"

	"github.com/golang/protobuf/proto"
	nats "github.com/nats-io/go-nats"
	{{- range  GetExtraImports .}}
	{{.}}
	{{- end}}
	{{- if Prometheus}}
	"github.com/prometheus/client_golang/prometheus"
	{{- end}}
	"github.com/nats-rpc/nrpc"
)

{{- range .Service}}

// {{.GetName}}Server is the interface that providers of the service
// {{.GetName}} should implement.
type {{.GetName}}Server interface {
	{{- range .Method}}
	{{- if ne .GetInputType ".nrpc.NoRequest"}}
	{{- $resultType := GetResultType .}}
	{{.GetName}}(ctx context.Context
		{{- range GetMethodSubjectParams . -}}
		, {{ . }} string
		{{- end -}}
		{{- if ne .GetInputType ".nrpc.Void" -}}
		, req {{GoType .GetInputType}}
		{{- end -}}
		{{- if HasStreamedReply . -}}
		, pushRep func({{GoType .GetOutputType}})
		{{- end -}}
	)
		{{- if ne $resultType ".nrpc.NoReply" }} (
		{{- if and (ne $resultType ".nrpc.Void") (not (HasStreamedReply .)) -}}
		resp {{GoType $resultType}}, {{end -}}
		err error)
		{{- end -}}
	{{- end}}
	{{- end}}
}

{{- if Prometheus}}

var (
	// The request completion time, measured at client-side.
	clientRCTFor{{.GetName}} = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "nrpc_client_request_completion_time_seconds",
			Help:       "The request completion time for calls, measured client-side.",
			Objectives: map[float64]float64{0.9: 0.01, 0.95: 0.01, 0.99: 0.001},
			ConstLabels: map[string]string{
				"service": "{{.GetName}}",
			},
		},
		[]string{"method"})

	// The handler execution time, measured at server-side.
	serverHETFor{{.GetName}} = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "nrpc_server_handler_execution_time_seconds",
			Help:       "The handler execution time for calls, measured server-side.",
			Objectives: map[float64]float64{0.9: 0.01, 0.95: 0.01, 0.99: 0.001},
			ConstLabels: map[string]string{
				"service": "{{.GetName}}",
			},
		},
		[]string{"method"})

	// The counts of calls made by the client, classified by result type.
	clientCallsFor{{.GetName}} = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nrpc_client_calls_count",
			Help: "The count of calls made by the client.",
			ConstLabels: map[string]string{
				"service": "{{.GetName}}",
			},
		},
		[]string{"method", "encoding", "result_type"})

	// The counts of requests handled by the server, classified by result type.
	serverRequestsFor{{.GetName}} = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nrpc_server_requests_count",
			Help: "The count of requests handled by the server.",
			ConstLabels: map[string]string{
				"service": "{{.GetName}}",
			},
		},
		[]string{"method", "encoding", "result_type"})
)
{{- end}}

// {{.GetName}}Handler provides a NATS subscription handler that can serve a
// subscription using a given {{.GetName}}Server implementation.
type {{.GetName}}Handler struct {
	ctx     context.Context
	workers *nrpc.WorkerPool
	nc      nrpc.NatsConn
	server  {{.GetName}}Server

	encodings []string
}

func New{{.GetName}}Handler(ctx context.Context, nc nrpc.NatsConn, s {{.GetName}}Server) *{{.GetName}}Handler {
	return &{{.GetName}}Handler{
		ctx:    ctx,
		nc:     nc,
		server: s,

		encodings: []string{"protobuf"},
	}
}

func New{{.GetName}}ConcurrentHandler(workers *nrpc.WorkerPool, nc nrpc.NatsConn, s {{.GetName}}Server) *{{.GetName}}Handler {
	return &{{.GetName}}Handler{
		workers: workers,
		nc:      nc,
		server:  s,
	}
}

// SetEncodings sets the output encodings when using a '*Publish' function
func (h *{{.GetName}}Handler) SetEncodings(encodings []string) {
	h.encodings = encodings
}

func (h *{{.GetName}}Handler) Subject() string {
	return "{{$pkgSubjectPrefix}}
	{{- range $pkgSubjectParams -}}
		*.
	{{- end -}}
	{{GetServiceSubject .}}
	{{- range GetServiceSubjectParams . -}}
		.*
	{{- end -}}
	.>"
}
{{- $serviceName := .GetName}}
{{- $serviceSubject := GetServiceSubject .}}
{{- $serviceSubjectParams := GetServiceSubjectParams .}}
{{- range .Method}}
{{- if eq .GetInputType ".nrpc.NoRequest"}}

func (h *{{$serviceName}}Handler) {{.GetName}}Publish(
	{{- range $pkgSubjectParams}}pkg{{.}} string, {{end -}}
	{{- range $serviceSubjectParams}}svc{{.}} string, {{end -}}
	{{- range GetMethodSubjectParams .}}mt{{.}} string, {{end -}}
	msg {{GoType .GetOutputType}}) error {
	for _, encoding := range h.encodings {
		rawMsg, err := nrpc.Marshal(encoding, &msg)
		if err != nil {
			log.Printf("{{$serviceName}}Handler.{{.GetName}}Publish: error marshaling the message: %s", err)
			return err
		}
		subject := "{{$pkgSubject}}."
		{{- range $pkgSubjectParams}} + pkg{{.}} + "."{{end -}}
		+ "{{$serviceSubject}}."
		{{- range $serviceSubjectParams}} + svc{{.}} + "."{{end -}}
		+ "{{GetMethodSubject .}}"
		{{- range GetMethodSubjectParams .}} + "." + mt{{.}}{{end}}
		if encoding != "protobuf" {
			subject += "." + encoding
		}
		if err := h.nc.Publish(subject, rawMsg); err != nil {
			return err
		}
	}
	return nil
}
{{- end}}
{{- end}}

{{- if ServiceNeedsHandler .}}

func (h *{{.GetName}}Handler) Handler(msg *nats.Msg) {
	var ctx context.Context
	if h.workers != nil {
		ctx = h.workers.Context
	} else {
		ctx = h.ctx
	}
	request := nrpc.NewRequest(ctx, h.nc, msg.Subject, msg.Reply)
	// extract method name & encoding from subject
	{{ if ne 0 (len $pkgSubjectParams)}}pkgParams{{else}}_{{end -}},
	{{- if ne 0 (len (GetServiceSubjectParams .))}} svcParams{{else}} _{{end -}}
	, name, tail, err := nrpc.ParseSubject(
		"{{$pkgSubject}}", {{len $pkgSubjectParams}}, "{{GetServiceSubject .}}", {{len (GetServiceSubjectParams .)}}, msg.Subject)
	if err != nil {
		log.Printf("{{.GetName}}Hanlder: {{.GetName}} subject parsing failed: %v", err)
		return
	}

	request.MethodName = name
	request.SubjectTail = tail

	{{- range $i, $name := $pkgSubjectParams }}
	request.SetPackageParam("{{$name}}", pkgParams[{{$i}}])
	{{- end }}
	{{- range $i, $name := GetServiceSubjectParams . }}
	request.SetServiceParam("{{$name}}", svcParams[{{$i}}])
	{{- end }}

	// call handler and form response
	var immediateError *nrpc.Error
	switch name {
	{{- range .Method}}
	case "{{GetMethodSubject .}}":
		{{- if eq .GetInputType ".nrpc.NoRequest"}}
		// {{.GetName}} is a no-request method. Ignore it.
		return
		{{- else}}{{/* !NoRequest */}}
		{{- if ne 0 (len (GetMethodSubjectParams .))}}
		var mtParams []string
		{{- end}}
		{{- if eq .GetOutputType ".nrpc.NoReply"}}
		request.NoReply = true
		{{- end}}
		{{if eq 0 (len (GetMethodSubjectParams .))}}_{{else}}mtParams{{end}}, request.Encoding, err = nrpc.ParseSubjectTail({{len (GetMethodSubjectParams .)}}, request.SubjectTail)
		if err != nil {
			log.Printf("{{.GetName}}Hanlder: {{.GetName}} subject parsing failed: %v", err)
			break
		}
		var req {{GoType .GetInputType}}
		if err := nrpc.Unmarshal(request.Encoding, msg.Data, &req); err != nil {
			log.Printf("{{.GetName}}Handler: {{.GetName}} request unmarshal failed: %v", err)
			immediateError = &nrpc.Error{
				Type: nrpc.Error_CLIENT,
				Message: "bad request received: " + err.Error(),
			}
{{- if Prometheus}}
			serverRequestsFor{{$serviceName}}.WithLabelValues(
				"{{.GetName}}", request.Encoding, "unmarshal_fail").Inc()
{{- end}}
		} else {
			{{- if HasStreamedReply .}}
			request.EnableStreamedReply()
			request.Handler = func(ctx context.Context)(proto.Message, error){
				err := h.server.{{.GetName}}(ctx
				{{- range $i, $p := GetMethodSubjectParams . -}}
				, mtParams[{{ $i }}]
				{{- end -}}
				{{- if ne .GetInputType ".nrpc.Void" -}}
				, req
				{{- end -}}
				, func(rep {{GoType .GetOutputType}}){
					request.SendStreamReply(&rep)
				})
				return nil, err
			}
			{{- else }}
			request.Handler = func(ctx context.Context)(proto.Message, error){
				{{- if eq .GetOutputType ".nrpc.NoReply" -}}
				var innerResp nrpc.NoReply
				{{else}}
				{{if eq .GetOutputType ".nrpc.Void" -}}
				var innerResp nrpc.Void
				{{else}}innerResp, {{end -}}
				err := {{end -}}
				h.server.{{.GetName}}(ctx
				{{- range $i, $p := GetMethodSubjectParams . -}}
				, mtParams[{{ $i }}]
				{{- end -}}
				{{- if ne .GetInputType ".nrpc.Void" -}}
				, req
				{{- end -}}
				)
				if err != nil {
					return nil, err
				}
				return &innerResp, err
			}
			{{- end }}
		}
		{{- end}}{{/* not HasStreamedReply */}}
{{- end}}{{/* range .Method */}}
	default:
		log.Printf("{{.GetName}}Handler: unknown name %q", name)
		immediateError = &nrpc.Error{
			Type: nrpc.Error_CLIENT,
			Message: "unknown name: " + name,
		}
{{- if Prometheus}}
		serverRequestsFor{{.GetName}}.WithLabelValues(
			"{{.GetName}}", request.Encoding, "name_fail").Inc()
{{- end}}
	}

{{- if Prometheus}}
	request.AfterReply = func(request *nrpc.Request, success, replySuccess bool) {
		if !replySuccess {
			serverRequestsFor{{$serviceName}}.WithLabelValues(
				request.MethodName, request.Encoding, "sendreply_fail").Inc()
		}
		if success {
			serverRequestsFor{{$serviceName}}.WithLabelValues(
				request.MethodName, request.Encoding, "success").Inc()
		} else {
			serverRequestsFor{{$serviceName}}.WithLabelValues(
				request.MethodName, request.Encoding, "handler_fail").Inc()
		}
		// report metric to Prometheus
		serverHETFor{{$serviceName}}.WithLabelValues(request.MethodName).Observe(
			request.Elapsed().Seconds())
	}

{{- end}}
	if immediateError == nil {
		if h.workers != nil {
			// Try queuing the request
			if err := h.workers.QueueRequest(request); err != nil {
				log.Printf("nrpc: Error queuing the request: %s", err)
			}
		} else {
			// Run the handler synchronously
			request.RunAndReply()
		}
	}

	if immediateError != nil {
		if err := request.SendReply(nil, immediateError); err != nil {
			log.Printf("{{.GetName}}Handler: {{.GetName}} handler failed to publish the response: %s", err)
{{- if Prometheus}}
			serverRequestsFor{{$serviceName}}.WithLabelValues(
				request.MethodName, request.Encoding, "handler_fail").Inc()
{{- end}}
		}
{{- if Prometheus}}
		serverHETFor{{$serviceName}}.WithLabelValues(request.MethodName).Observe(
			request.Elapsed().Seconds())
{{- end}}
	} else {
	}
}
{{- end}}

type {{.GetName}}Client struct {
	nc      nrpc.NatsConn
	{{- if ne 0 (len $pkgSubject)}}
	PkgSubject string
	{{- end}}
	{{- range $pkgSubjectParams}}
	PkgParam{{ . }} string
	{{- end}}
	Subject string
	{{- range GetServiceSubjectParams .}}
	SvcParam{{ . }} string
	{{- end}}
	Encoding string
	Timeout time.Duration
}

func New{{.GetName}}Client(nc nrpc.NatsConn
	{{- range $pkgSubjectParams -}}
	, pkgParam{{.}} string
	{{- end -}}
	{{- range GetServiceSubjectParams . -}}
	, svcParam{{ . }} string
	{{- end -}}
	) *{{.GetName}}Client {
	return &{{.GetName}}Client{
		nc:      nc,
		{{- if ne 0 (len $pkgSubject)}}
		PkgSubject: "{{$pkgSubject}}",
		{{- end}}
		{{- range $pkgSubjectParams}}
		PkgParam{{.}}: pkgParam{{.}},
		{{- end}}
		Subject: "{{GetServiceSubject .}}",
		{{- range GetServiceSubjectParams .}}
		SvcParam{{.}}: svcParam{{.}},
		{{- end}}
		Encoding: "protobuf",
		Timeout: 5 * time.Second,
	}
}
{{- $serviceName := .GetName}}
{{- $serviceSubjectParams := GetServiceSubjectParams .}}
{{- range .Method}}
{{- $resultType := GetResultType .}}
{{- if eq .GetInputType ".nrpc.NoRequest"}}

func (c *{{$serviceName}}Client) {{.GetName}}Subject(
	{{range GetMethodSubjectParams .}}mt{{.}} string,{{end}}
) string {
	subject := {{ if ne 0 (len $pkgSubject) -}}
		c.PkgSubject + "." + {{end}}
	{{- range $pkgSubjectParams -}}
		c.PkgParam{{.}} + "." + {{end -}}
	c.Subject + "." + {{range $serviceSubjectParams -}}
		c.SvcParam{{.}} + "." + {{end -}}
	"{{GetMethodSubject .}}"
	{{- range GetMethodSubjectParams .}} + "." + mt{{.}}{{end}}
	if c.Encoding != "protobuf" {
		subject += "." + c.Encoding
	}
	return subject
}

type {{$serviceName}}{{.GetName}}Subscription struct {
	*nats.Subscription
	
	encoding string
}

func (s *{{$serviceName}}{{.GetName}}Subscription) Next(timeout time.Duration) (next {{GoType .GetOutputType}}, err error) {
	msg, err := s.Subscription.NextMsg(timeout)
	if err != nil {
		return
	}
	err = nrpc.Unmarshal(s.encoding, msg.Data, &next)
	return
}

func (c *{{$serviceName}}Client) {{.GetName}}SubscribeSync(
	{{range GetMethodSubjectParams .}}mt{{.}} string,{{end}}
) (sub *{{$serviceName}}{{.GetName}}Subscription, err error) {
	subject := c.{{.GetName}}Subject(
		{{range GetMethodSubjectParams .}}mt{{.}},{{end}}
	)
	natsSub, err := c.nc.SubscribeSync(subject)
	if err != nil {
		return
	}
	sub = &{{$serviceName}}{{.GetName}}Subscription{natsSub, c.Encoding}
	return
}

func (c *{{$serviceName}}Client) {{.GetName}}Subscribe(
	{{range GetMethodSubjectParams .}}mt{{.}} string,{{end}}
	handler func ({{GoType .GetOutputType}}),
) (sub *nats.Subscription, err error) {
	subject := c.{{.GetName}}Subject(
		{{range GetMethodSubjectParams .}}mt{{.}},{{end}}
	)
	sub, err = c.nc.Subscribe(subject, func(msg *nats.Msg){
		var pmsg {{GoType .GetOutputType}}
		err := nrpc.Unmarshal(c.Encoding, msg.Data, &pmsg)
		if err != nil {
			log.Printf("{{$serviceName}}Client.{{.GetName}}Subscribe: Error decoding, %s", err)
			return
		}
		handler(pmsg)
	})
	return
}

func (c *{{$serviceName}}Client) {{.GetName}}SubscribeChan(
	{{range GetMethodSubjectParams .}}mt{{.}} string,{{end}}
) (<-chan {{GoType .GetOutputType}}, *nats.Subscription, error) {
	ch := make(chan {{GoType .GetOutputType}})
	sub, err := c.{{.GetName}}Subscribe(
		{{- range GetMethodSubjectParams .}}mt{{.}}, {{end -}}
		func (msg {{GoType .GetOutputType}}) {
		ch <- msg
	})
	return ch, sub, err
}

{{- else if HasStreamedReply .}}

func (c *{{$serviceName}}Client) {{.GetName}}(
	ctx context.Context,
	{{- range GetMethodSubjectParams . -}}
	{{ . }} string,
	{{- end}}
	{{- if ne .GetInputType ".nrpc.Void"}}
	req {{GoType .GetInputType}},
	{{- end}}
	cb func (context.Context, {{GoType .GetOutputType}}),
) error {
{{- if Prometheus}}
	start := time.Now()
{{- end}}
	subject := {{ if ne 0 (len $pkgSubject) -}}
		c.PkgSubject + "." + {{end}}
	{{- range $pkgSubjectParams -}}
		c.PkgParam{{.}} + "." + {{end -}}
	c.Subject + "." + {{range $serviceSubjectParams -}}
		c.SvcParam{{.}} + "." + {{end -}}
	"{{GetMethodSubject .}}"
	{{- range GetMethodSubjectParams . }} + "." + {{ . }}{{ end }}

	sub, err := nrpc.StreamCall(ctx, c.nc, subject
		{{- if ne .GetInputType ".nrpc.Void" -}}
		, &req
		{{- else -}}
		, &nrpc.Void{}
		{{- end -}}
		, c.Encoding, c.Timeout)
	if err != nil {
		{{- if Prometheus}}
		clientCallsFor{{$serviceName}}.WithLabelValues(
			"{{.GetName}}", c.Encoding, "error").Inc()
		{{- end}}
		return err
	}

	var res {{GoType .GetOutputType}}
	for {
		err = sub.Next(&res)
		if err != nil {
			break
		}
		cb(ctx, res)
	}
	if err == nrpc.ErrEOS {
		err = nil
	}
{{- if Prometheus}}
	// report total time taken to Prometheus
	elapsed := time.Since(start).Seconds()
	clientRCTFor{{$serviceName}}.WithLabelValues("{{.GetName}}").Observe(elapsed)
	clientCallsFor{{$serviceName}}.WithLabelValues(
		"{{.GetName}}", c.Encoding, "success").Inc()
{{- end}}
	return err
}
{{- else}}

func (c *{{$serviceName}}Client) {{.GetName}}(
	{{- range GetMethodSubjectParams . -}}
	{{ . }} string, {{ end -}}
	{{- if ne .GetInputType ".nrpc.Void" -}}
	req {{GoType .GetInputType}}
	{{- end -}}) (
		{{- if not (eq $resultType ".nrpc.Void" ".nrpc.NoReply") -}}
		resp {{GoType $resultType}}, {{end -}}
		err error) {
{{- if Prometheus}}
	start := time.Now()
{{- end}}

	subject := {{ if ne 0 (len $pkgSubject) -}}
		c.PkgSubject + "." + {{end}}
	{{- range $pkgSubjectParams -}}
		c.PkgParam{{.}} + "." + {{end -}}
	c.Subject + "." + {{range $serviceSubjectParams -}}
		c.SvcParam{{.}} + "." + {{end -}}
	"{{GetMethodSubject .}}"
	{{- range GetMethodSubjectParams . }} + "." + {{ . }}{{ end }}

	// call
	{{- if eq .GetInputType ".nrpc.Void"}}
	var req {{GoType .GetInputType}}
	{{- end}}
	{{- if eq .GetOutputType ".nrpc.Void" ".nrpc.NoReply"}}
	var resp {{GoType .GetOutputType}}
	{{- end}}
	err = nrpc.Call(&req, &resp, c.nc, subject, c.Encoding, c.Timeout)
	if err != nil {
{{- if Prometheus}}
		clientCallsFor{{$serviceName}}.WithLabelValues(
			"{{.GetName}}", c.Encoding, "call_fail").Inc()
{{- end}}
		return // already logged
	}

{{- if Prometheus}}

	// report total time taken to Prometheus
	elapsed := time.Since(start).Seconds()
	clientRCTFor{{$serviceName}}.WithLabelValues("{{.GetName}}").Observe(elapsed)
	clientCallsFor{{$serviceName}}.WithLabelValues(
		"{{.GetName}}", c.Encoding, "success").Inc()
{{- end}}

	return
}
{{- end}}
{{- end}}
{{- end}}

type Client struct {
	nc      nrpc.NatsConn
	defaultEncoding string
	defaultTimeout time.Duration
	{{- if ne 0 (len $pkgSubject)}}
	pkgSubject string
	{{- end}}
	{{- range $pkgSubjectParams}}
	pkgParam{{ . }} string
	{{- end}}

	{{- range .Service}}
	{{.GetName}} *{{.GetName}}Client
	{{- end}}
}

func NewClient(nc nrpc.NatsConn
	{{- range $pkgSubjectParams -}}
	, pkgParam{{.}} string
	{{- end -}}) *Client {
	c := Client{
		nc: nc,
		defaultEncoding: "protobuf",
		defaultTimeout: 5*time.Second,
		{{- if ne 0 (len $pkgSubject)}}
		pkgSubject: "{{$pkgSubject}}",
		{{- end}}
		{{- range $pkgSubjectParams}}
		pkgParam{{.}}: pkgParam{{.}},
		{{- end}}
	}
	{{- range .Service}}
	{{- if eq 0 (len (GetServiceSubjectParams .))}}
	c.{{.GetName}} = New{{.GetName}}Client(nc
	{{- range $pkgSubjectParams -}}
		, c.pkgParam{{ . }}
	{{- end}})
	{{- end}}
	{{- end}}
	return &c
}

func (c *Client) SetEncoding(encoding string) {
	c.defaultEncoding = encoding
	{{- range .Service}}
	if c.{{.GetName}} != nil {
		c.{{.GetName}}.Encoding = encoding
	}
	{{- end}}
}

func (c *Client) SetTimeout(t time.Duration) {
	c.defaultTimeout = t
	{{- range .Service}}
	if c.{{.GetName}} != nil {
		c.{{.GetName}}.Timeout = t
	}
	{{- end}}
}

{{- range .Service}}
{{- if ne 0 (len (GetServiceSubjectParams .))}}

func (c *Client) Set{{.GetName}}Params(
	{{- range GetServiceSubjectParams .}}
	{{ . }} string,
	{{- end}}
) {
	c.{{.GetName}} = New{{.GetName}}Client(
		c.nc,
	{{- range $pkgSubjectParams}}
		c.pkgParam{{ . }},
	{{- end}}
	{{- range GetServiceSubjectParams .}}
		{{ . }},
	{{- end}}
	)
	c.{{.GetName}}.Encoding = c.defaultEncoding
	c.{{.GetName}}.Timeout = c.defaultTimeout
}

func (c *Client) New{{.GetName}}(
	{{- range GetServiceSubjectParams .}}
	{{ . }} string,
	{{- end}}
) *{{.GetName}}Client {
	client := New{{.GetName}}Client(
		c.nc,
	{{- range $pkgSubjectParams}}
		c.pkgParam{{ . }},
	{{- end}}
	{{- range GetServiceSubjectParams .}}
		{{ . }},
	{{- end}}
	)
	client.Encoding = c.defaultEncoding
	client.Timeout = c.defaultTimeout
	return client
}
{{- end}}
{{- end}}

{{- if Prometheus}}

func init() {
{{- range .Service}}
	// register metrics for service {{.GetName}}
	prometheus.MustRegister(clientRCTFor{{.GetName}})
	prometheus.MustRegister(serverHETFor{{.GetName}})
	prometheus.MustRegister(clientCallsFor{{.GetName}})
	prometheus.MustRegister(serverRequestsFor{{.GetName}})
{{- end}}
}
{{- end}}`
