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
		{lb.MatcherTypeSrcIP, lb.RuleMatcher{Type: lb.MatcherTypeSrcIP, SrcIP: &lb.MatcherSrcIP{Value: "1.2.3.4"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeSrcIP, SrcIP: &upcloud.LoadBalancerMatcherSourceIP{Value: "1.2.3.4"}}},
		{lb.MatcherTypeSrcPort, lb.RuleMatcher{Type: lb.MatcherTypeSrcPort, SrcPort: &lb.MatcherInteger{Method: "equal", Value: 443}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeSrcPort, SrcPort: &upcloud.LoadBalancerMatcherInteger{Method: upcloud.LoadBalancerIntegerMatcherMethodEqual, Value: 443}}},
		{lb.MatcherTypeBodySize, lb.RuleMatcher{Type: lb.MatcherTypeBodySize, BodySize: &lb.MatcherInteger{Method: "greater", Value: 10}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeBodySize, BodySize: &upcloud.LoadBalancerMatcherInteger{Method: upcloud.LoadBalancerIntegerMatcherMethodGreater, Value: 10}}},
		{lb.MatcherTypePath, lb.RuleMatcher{Type: lb.MatcherTypePath, Path: &lb.MatcherString{Method: "starts", Value: testAPIPath, IgnoreCase: &ic}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypePath, Path: &upcloud.LoadBalancerMatcherString{Method: upcloud.LoadBalancerStringMatcherMethodStarts, Value: testAPIPath, IgnoreCase: &ic}}},
		{lb.MatcherTypeURL, lb.RuleMatcher{Type: lb.MatcherTypeURL, URL: &lb.MatcherString{Method: testMethodExact, Value: "/"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeURL, URL: &upcloud.LoadBalancerMatcherString{Method: upcloud.LoadBalancerStringMatcherMethodExact, Value: "/"}}},
		{lb.MatcherTypeURLQuery, lb.RuleMatcher{Type: lb.MatcherTypeURLQuery, URLQuery: &lb.MatcherString{Method: "exists", Value: ""}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeURLQuery, URLQuery: &upcloud.LoadBalancerMatcherString{Method: upcloud.LoadBalancerStringMatcherMethodExists}}},
		{lb.MatcherTypeHost, lb.RuleMatcher{Type: lb.MatcherTypeHost, Host: &lb.MatcherHost{Value: "a.example.com"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHost, Host: &upcloud.LoadBalancerMatcherHost{Value: "a.example.com"}}},
		{lb.MatcherTypeHTTPMethod, lb.RuleMatcher{Type: lb.MatcherTypeHTTPMethod, HTTPMethod: &lb.MatcherHTTPMethod{Value: "GET"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHTTPMethod, HTTPMethod: &upcloud.LoadBalancerMatcherHTTPMethod{Value: upcloud.LoadBalancerHTTPMatcherMethodGet}}},
		{lb.MatcherTypeHTTPStatus, lb.RuleMatcher{Type: lb.MatcherTypeHTTPStatus, HTTPStatus: &lb.MatcherInteger{Method: "range", Value: 0, RangeStart: intP(400), RangeEnd: intP(499)}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHTTPStatus, HTTPStatus: &upcloud.LoadBalancerMatcherInteger{Method: upcloud.LoadBalancerIntegerMatcherMethodRange, RangeStart: 400, RangeEnd: 499}}},
		{lb.MatcherTypeCookie, lb.RuleMatcher{Type: lb.MatcherTypeCookie, Cookie: &lb.MatcherStringWithArgument{Method: testMethodExact, Name: "sid", Value: "x"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeCookie, Cookie: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodExact, Name: "sid", Value: "x"}}},
		{lb.MatcherTypeHeader, lb.RuleMatcher{Type: lb.MatcherTypeHeader, Header: &lb.MatcherStringWithArgument{Method: "exists", Name: "x-api"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeHeader, Header: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodExists, Name: "x-api"}}},
		{lb.MatcherTypeRequestHeader, lb.RuleMatcher{Type: lb.MatcherTypeRequestHeader, RequestHeader: &lb.MatcherStringWithArgument{Method: testMethodExact, Name: "x-a", Value: "1"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeRequestHeader, RequestHeader: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodExact, Name: "x-a", Value: "1"}}},
		{lb.MatcherTypeResponseHeader, lb.RuleMatcher{Type: lb.MatcherTypeResponseHeader, ResponseHeader: &lb.MatcherStringWithArgument{Method: "regexp", Name: "x-b", Value: "^.*"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeResponseHeader, ResponseHeader: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodRegexp, Name: "x-b", Value: "^.*"}}},
		{lb.MatcherTypeURLParam, lb.RuleMatcher{Type: lb.MatcherTypeURLParam, URLParam: &lb.MatcherStringWithArgument{Method: "ends", Name: "id", Value: "1"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeURLParam, URLParam: &upcloud.LoadBalancerMatcherStringWithArgument{Method: upcloud.LoadBalancerStringMatcherMethodEnds, Name: "id", Value: "1"}}},
		{lb.MatcherTypeNumMembersUp, lb.RuleMatcher{Type: lb.MatcherTypeNumMembersUp, NumMembersUp: &lb.MatcherNumMembersUp{Method: "greater", Value: 1, Backend: "be"}},
			upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherTypeNumMembersUp, NumMembersUp: &upcloud.LoadBalancerMatcherNumMembersUp{Method: upcloud.LoadBalancerIntegerMatcherMethodGreater, Value: 1, Backend: "be"}}},
	}
	for _, tc := range cases {
		got := toMatchers([]lb.RuleMatcher{tc.in})
		g.Expect(got).To(HaveLen(1))
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
		{lb.ActionTypeUseBackend, lb.RuleAction{Type: lb.ActionTypeUseBackend, UseBackend: &lb.ActionUseBackend{Backend: "be"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeUseBackend, UseBackend: &upcloud.LoadBalancerActionUseBackend{Backend: "be"}}},
		{lb.ActionTypeTCPReject, lb.RuleAction{Type: lb.ActionTypeTCPReject},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeTCPReject, TCPReject: &upcloud.LoadBalancerActionTCPReject{}}},
		{lb.ActionTypeHTTPReturn, lb.RuleAction{Type: lb.ActionTypeHTTPReturn, HTTPReturn: &lb.ActionHTTPReturn{Status: 200, ContentType: "text/plain", Payload: "hi"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPReturn, HTTPReturn: &upcloud.LoadBalancerActionHTTPReturn{Status: 200, ContentType: "text/plain", Payload: "hi"}}},
		{lb.ActionTypeHTTPRedirect, lb.RuleAction{Type: lb.ActionTypeHTTPRedirect, HTTPRedirect: &lb.ActionHTTPRedirect{Location: "https://x", Scheme: "https", Status: 301}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPRedirect, HTTPRedirect: &upcloud.LoadBalancerActionHTTPRedirect{Location: "https://x", Scheme: upcloud.LoadBalancerActionHTTPRedirectSchemeHTTPS, Status: 301}}},
		{lb.ActionTypeHTTPRewritePath, lb.RuleAction{Type: lb.ActionTypeHTTPRewritePath, HTTPRewritePath: &lb.ActionHTTPRewritePath{MatchPattern: "^/a", RewriteTo: "/b"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPRewritePath, HTTPRewritePath: &upcloud.LoadBalancerActionHTTPRewritePath{MatchPattern: "^/a", RewriteTo: "/b"}}},
		{lb.ActionTypeHTTPRewriteURI, lb.RuleAction{Type: lb.ActionTypeHTTPRewriteURI, HTTPRewriteURI: &lb.ActionHTTPRewriteURI{MatchPattern: "^/c", RewriteTo: "/d"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeHTTPRewriteURI, HTTPRewriteURI: &upcloud.LoadBalancerActionHTTPRewriteURI{MatchPattern: "^/c", RewriteTo: "/d"}}},
		{lb.ActionTypeSetForwardedHeaders, lb.RuleAction{Type: lb.ActionTypeSetForwardedHeaders},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeSetForwardedHeaders, SetForwardedHeaders: &upcloud.LoadBalancerActionSetForwardedHeaders{}}},
		{lb.ActionTypeSetRequestHeader, lb.RuleAction{Type: lb.ActionTypeSetRequestHeader, SetRequestHeader: &lb.ActionSetHeader{Header: "x-h", Value: "v"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeSetRequestHeader, SetRequestHeader: &upcloud.LoadBalancerActionSetHeader{Header: "x-h", Value: "v"}}},
		{lb.ActionTypeSetResponseHeader, lb.RuleAction{Type: lb.ActionTypeSetResponseHeader, SetResponseHeader: &lb.ActionSetHeader{Header: "x-r", Value: "w"}},
			upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionTypeSetResponseHeader, SetResponseHeader: &upcloud.LoadBalancerActionSetHeader{Header: "x-r", Value: "w"}}},
	}
	for _, tc := range cases {
		got := toActions([]lb.RuleAction{tc.in})
		g.Expect(got).To(HaveLen(1))
		g.Expect(got[0]).To(Equal(tc.want), tc.name)
	}
}
