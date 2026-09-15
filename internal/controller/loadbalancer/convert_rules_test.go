package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
)

// TestConvertMatchers round-trips every matcher type through toMatchers and
// compares against the expected SDK matcher.
func TestConvertMatchers(t *testing.T) {
	g := NewWithT(t)
	ic := true
	cases := []struct {
		name string
		in   lb.RuleMatcher
		want upcloud.LoadBalancerMatcher
	}{
		{"src_ip", lb.RuleMatcher{Type: "src_ip", SrcIP: &lb.MatcherSrcIP{Value: "1.2.3.4"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeSrcIP, SrcIP: &upcloud.LoadBalancerMatcherSourceIP{Value: "1.2.3.4"}}},
		{"src_port", lb.RuleMatcher{Type: "src_port", SrcPort: &lb.MatcherInteger{Method: "equal", Value: 443}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeSrcPort, SrcPort: &upcloud.LoadBalancerMatcherInteger{Method: upcloud.LoadBalancerIntegerMatcherMethodEqual, Value: 443}}},
		{"body_size", lb.RuleMatcher{Type: "body_size", BodySize: &lb.MatcherInteger{Method: "greater", Value: 10}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeBodySize, BodySize: &upcloud.LoadBalancerMatcherInteger{Method: upcloud.LoadBalancerIntegerMatcherMethodGreater, Value: 10}}},
		{"path", lb.RuleMatcher{Type: "path", Path: &lb.MatcherString{Method: "starts", Value: "/api", IgnoreCase: &ic}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypePath, Path: &upcloud.LoadBalancerMatcherString{Method: upcloud.LoadBalancerStringMatcherMethodStarts, Value: "/api", IgnoreCase: &ic}}},
		{"url", lb.RuleMatcher{Type: "url", URL: &lb.MatcherString{Method: "exact", Value: "/"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeURL, URL: &upcloud.LoadBalancerMatcherString{Method: upcloud.LoadBalancerStringMatcherMethodExact, Value: "/"}}},
		{"url_query", lb.RuleMatcher{Type: "url_query", URLQuery: &lb.MatcherString{Method: "exists", Value: ""}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeURLQuery, URLQuery: &upcloud.LoadBalancerMatcherString{Method: upcloud.LoadBalancerStringMatcherMethodExists}}},
		{"host", lb.RuleMatcher{Type: "host", Host: &lb.MatcherHost{Value: "a.example.com"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHost, Host: &upcloud.LoadBalancerMatcherHost{Value: "a.example.com"}}},
		{"http_method", lb.RuleMatcher{Type: "http_method", HTTPMethod: &lb.MatcherHTTPMethod{Value: "GET"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHTTPMethod, HTTPMethod: &upcloud.LoadBalancerMatcherHTTPMethod{Value: upcloud.LoadBalancerHTTPMatcherMethodGet}}},
		{"http_status", lb.RuleMatcher{Type: "http_status", HTTPStatus: &lb.MatcherInteger{Method: "range", Value: 0, RangeStart: intP(400), RangeEnd: intP(499)}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHTTPStatus, HTTPStatus: &upcloud.LoadBalancerMatcherInteger{Method: upcloud.LoadBalancerIntegerMatcherMethodRange, RangeStart: 400, RangeEnd: 499}}},
		{"cookie", lb.RuleMatcher{Type: "cookie", Cookie: &lb.MatcherStringWithArgument{Method: "exact", Name: "sid", Value: "x"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeCookie, Cookie: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodExact, Name: "sid", Value: "x"}}},
		{"header", lb.RuleMatcher{Type: "header", Header: &lb.MatcherStringWithArgument{Method: "exists", Name: "x-api"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHeader, Header: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodExists, Name: "x-api"}}},
		{"request_header", lb.RuleMatcher{Type: "request_header", RequestHeader: &lb.MatcherStringWithArgument{Method: "exact", Name: "x-a", Value: "1"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeRequestHeader, RequestHeader: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodExact, Name: "x-a", Value: "1"}}},
		{"response_header", lb.RuleMatcher{Type: "response_header", ResponseHeader: &lb.MatcherStringWithArgument{Method: "regexp", Name: "x-b", Value: "^.*"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeResponseHeader, ResponseHeader: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodRegexp, Name: "x-b", Value: "^.*"}}},
		{"url_param", lb.RuleMatcher{Type: "url_param", URLParam: &lb.MatcherStringWithArgument{Method: "ends", Name: "id", Value: "1"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeURLParam, URLParam: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodEnds, Name: "id", Value: "1"}}},
		{"num_members_up", lb.RuleMatcher{Type: "num_members_up", NumMembersUp: &lb.MatcherNumMembersUp{Method: "greater", Value: 1, Backend: "be"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeNumMembersUp, NumMembersUp: &upcloud.LoadBalancerMatcherNumMembersUp{Method: upcloud.LoadBalancerIntegerMatcherMethodGreater, Value: 1, Backend: "be"}}},
	}
	for _, tc := range cases {
		got := toMatchers([]lb.RuleMatcher{tc.in})
		g.Expect(len(got)).To(Equal(1))
		g.Expect(got[0]).To(Equal(tc.want), tc.name)
	}
}

func intP(v int) *int { return &v }

// TestConvertActions round-trips every action type through toActions and
// compares against the expected SDK action.
func TestConvertActions(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		name string
		in   lb.RuleAction
		want upcloud.LoadBalancerAction
	}{
		{"use_backend", lb.RuleAction{Type: "use_backend", UseBackend: &lb.ActionUseBackend{Backend: "be"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeUseBackend, UseBackend: &upcloud.LoadBalancerActionUseBackend{Backend: "be"}}},
		{"tcp_reject", lb.RuleAction{Type: "tcp_reject"},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeTCPReject, TCPReject: &upcloud.LoadBalancerActionTCPReject{}}},
		{"http_return", lb.RuleAction{Type: "http_return", HTTPReturn: &lb.ActionHTTPReturn{Status: 200, ContentType: "text/plain", Payload: "hi"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPReturn, HTTPReturn: &upcloud.LoadBalancerActionHTTPReturn{Status: 200, ContentType: "text/plain", Payload: "hi"}}},
		{"http_redirect", lb.RuleAction{Type: "http_redirect", HTTPRedirect: &lb.ActionHTTPRedirect{Location: "https://x", Scheme: "https", Status: 301}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPRedirect, HTTPRedirect: &upcloud.LoadBalancerActionHTTPRedirect{Location: "https://x", Scheme: upcloud.LoadBalancerActionHTTPRedirectSchemeHTTPS, Status: 301}}},
		{"http_rewrite_path", lb.RuleAction{Type: "http_rewrite_path", HTTPRewritePath: &lb.ActionHTTPRewritePath{MatchPattern: "^/a", RewriteTo: "/b"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPRewritePath, HTTPRewritePath: &upcloud.LoadBalancerActionHTTPRewritePath{MatchPattern: "^/a", RewriteTo: "/b"}}},
		{"http_rewrite_uri", lb.RuleAction{Type: "http_rewrite_uri", HTTPRewriteURI: &lb.ActionHTTPRewriteURI{MatchPattern: "^/c", RewriteTo: "/d"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPRewriteURI, HTTPRewriteURI: &upcloud.LoadBalancerActionHTTPRewriteURI{MatchPattern: "^/c", RewriteTo: "/d"}}},
		{"set_forwarded_headers", lb.RuleAction{Type: "set_forwarded_headers"},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeSetForwardedHeaders, SetForwardedHeaders: &upcloud.LoadBalancerActionSetForwardedHeaders{}}},
		{"set_request_header", lb.RuleAction{Type: "set_request_header", SetRequestHeader: &lb.ActionSetHeader{Header: "x-h", Value: "v"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeSetRequestHeader, SetRequestHeader: &upcloud.LoadBalancerActionSetHeader{Header: "x-h", Value: "v"}}},
		{"set_response_header", lb.RuleAction{Type: "set_response_header", SetResponseHeader: &lb.ActionSetHeader{Header: "x-r", Value: "w"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeSetResponseHeader, SetResponseHeader: &upcloud.LoadBalancerActionSetHeader{Header: "x-r", Value: "w"}}},
	}
	for _, tc := range cases {
		got := toActions([]lb.RuleAction{tc.in})
		g.Expect(len(got)).To(Equal(1))
		g.Expect(got[0]).To(Equal(tc.want), tc.name)
	}
}
