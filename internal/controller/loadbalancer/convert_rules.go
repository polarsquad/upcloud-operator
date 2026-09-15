package loadbalancer

import (
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
)

// integerMethod maps the spec integer matcher method to the SDK enum.
func integerMethod(m string) upcloud.LoadBalancerIntegerMatcherMethod {
	return upcloud.LoadBalancerIntegerMatcherMethod(m)
}

// stringMethod maps the spec string matcher method to the SDK enum.
func stringMethod(m string) upcloud.LoadBalancerStringMatcherMethod {
	return upcloud.LoadBalancerStringMatcherMethod(m)
}

// httpMethod maps the spec HTTP method to the SDK enum.
func httpMethod(m string) upcloud.LoadBalancerHTTPMatcherMethod {
	return upcloud.LoadBalancerHTTPMatcherMethod(m)
}

// rangeStart converts a *int range bound to the SDK's int (0 when nil).
func rangeStart(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// rangeEnd converts a *int range bound to the SDK's int (0 when nil).
func rangeEnd(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// toMatchers converts the spec matchers to SDK matchers. Each matcher type has
// a dedicated builder so the dispatcher stays a simple switch (low
// cyclomatic complexity).
func toMatchers(in []lb.RuleMatcher) []upcloud.LoadBalancerMatcher {
	out := make([]upcloud.LoadBalancerMatcher, 0, len(in))
	for i := range in {
		out = append(out, toMatcher(in[i]))
	}
	return out
}

func toMatcher(m lb.RuleMatcher) upcloud.LoadBalancerMatcher {
	uc := upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherType(m.Type), Inverse: m.Inverse}
	switch m.Type {
	case lb.MatcherTypeSrcIP:
		uc.SrcIP = srcIP(m)
	case lb.MatcherTypeSrcPort:
		uc.SrcPort = intMatcher(m.SrcPort)
	case lb.MatcherTypeBodySize:
		uc.BodySize = intMatcher(m.BodySize)
	case lb.MatcherTypePath:
		uc.Path = strMatcher(m.Path)
	case lb.MatcherTypeURL:
		uc.URL = strMatcher(m.URL)
	case lb.MatcherTypeURLQuery:
		uc.URLQuery = strMatcher(m.URLQuery)
	case lb.MatcherTypeHost:
		uc.Host = host(m)
	case lb.MatcherTypeHTTPMethod:
		uc.HTTPMethod = httpMethodMatcher(m)
	case lb.MatcherTypeHTTPStatus:
		uc.HTTPStatus = intMatcher(m.HTTPStatus)
	case lb.MatcherTypeCookie:
		uc.Cookie = strArgMatcher(m.Cookie)
	case lb.MatcherTypeHeader:
		uc.Header = strArgMatcher(m.Header)
	case lb.MatcherTypeRequestHeader:
		uc.RequestHeader = strArgMatcher(m.RequestHeader)
	case lb.MatcherTypeResponseHeader:
		uc.ResponseHeader = strArgMatcher(m.ResponseHeader)
	case lb.MatcherTypeURLParam:
		uc.URLParam = strArgMatcher(m.URLParam)
	case lb.MatcherTypeNumMembersUp:
		uc.NumMembersUp = numMembersUp(m)
	}
	return uc
}

func srcIP(m lb.RuleMatcher) *upcloud.LoadBalancerMatcherSourceIP {
	if m.SrcIP == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherSourceIP{Value: m.SrcIP.Value}
}

func host(m lb.RuleMatcher) *upcloud.LoadBalancerMatcherHost {
	if m.Host == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherHost{Value: m.Host.Value}
}

func httpMethodMatcher(m lb.RuleMatcher) *upcloud.LoadBalancerMatcherHTTPMethod {
	if m.HTTPMethod == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherHTTPMethod{Value: httpMethod(m.HTTPMethod.Value)}
}

func numMembersUp(m lb.RuleMatcher) *upcloud.LoadBalancerMatcherNumMembersUp {
	if m.NumMembersUp == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherNumMembersUp{Method: integerMethod(m.NumMembersUp.Method), Value: m.NumMembersUp.Value, Backend: m.NumMembersUp.Backend}
}

// intMatcher converts a *MatcherInteger to the SDK form (nil passes through).
func intMatcher(in *lb.MatcherInteger) *upcloud.LoadBalancerMatcherInteger {
	if in == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherInteger{Method: integerMethod(in.Method), Value: in.Value, RangeStart: rangeStart(in.RangeStart), RangeEnd: rangeEnd(in.RangeEnd)}
}

// strMatcher converts a *MatcherString to the SDK form (nil passes through).
func strMatcher(in *lb.MatcherString) *upcloud.LoadBalancerMatcherString {
	if in == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherString{Method: stringMethod(in.Method), Value: in.Value, IgnoreCase: in.IgnoreCase}
}

// strArgMatcher converts a *MatcherStringWithArgument to the SDK form (nil
// passes through).
func strArgMatcher(in *lb.MatcherStringWithArgument) *upcloud.LoadBalancerMatcherStringWithArgument {
	if in == nil {
		return nil
	}
	return &upcloud.LoadBalancerMatcherStringWithArgument{Method: stringMethod(in.Method), Name: in.Name, Value: in.Value, IgnoreCase: in.IgnoreCase}
}

// toActions converts the spec actions to SDK actions.
func toActions(in []lb.RuleAction) []upcloud.LoadBalancerAction {
	out := make([]upcloud.LoadBalancerAction, 0, len(in))
	for i := range in {
		a := in[i]
		uc := upcloud.LoadBalancerAction{Type: upcloud.LoadBalancerActionType(a.Type)}
		switch a.Type {
		case lb.ActionTypeUseBackend:
			if a.UseBackend != nil {
				uc.UseBackend = &upcloud.LoadBalancerActionUseBackend{Backend: a.UseBackend.Backend}
			}
		case lb.ActionTypeTCPReject:
			uc.TCPReject = &upcloud.LoadBalancerActionTCPReject{}
		case lb.ActionTypeHTTPReturn:
			if a.HTTPReturn != nil {
				uc.HTTPReturn = &upcloud.LoadBalancerActionHTTPReturn{Status: a.HTTPReturn.Status, ContentType: a.HTTPReturn.ContentType, Payload: a.HTTPReturn.Payload}
			}
		case lb.ActionTypeHTTPRedirect:
			if a.HTTPRedirect != nil {
				scheme := upcloud.LoadBalancerActionHTTPRedirectSchemeHTTP
				if a.HTTPRedirect.Scheme == "https" {
					scheme = upcloud.LoadBalancerActionHTTPRedirectSchemeHTTPS
				}
				uc.HTTPRedirect = &upcloud.LoadBalancerActionHTTPRedirect{Location: a.HTTPRedirect.Location, Scheme: scheme, Status: a.HTTPRedirect.Status}
			}
		case lb.ActionTypeHTTPRewritePath:
			if a.HTTPRewritePath != nil {
				uc.HTTPRewritePath = &upcloud.LoadBalancerActionHTTPRewritePath{MatchPattern: a.HTTPRewritePath.MatchPattern, RewriteTo: a.HTTPRewritePath.RewriteTo}
			}
		case lb.ActionTypeHTTPRewriteURI:
			if a.HTTPRewriteURI != nil {
				uc.HTTPRewriteURI = &upcloud.LoadBalancerActionHTTPRewriteURI{MatchPattern: a.HTTPRewriteURI.MatchPattern, RewriteTo: a.HTTPRewriteURI.RewriteTo}
			}
		case lb.ActionTypeSetForwardedHeaders:
			uc.SetForwardedHeaders = &upcloud.LoadBalancerActionSetForwardedHeaders{}
		case lb.ActionTypeSetRequestHeader:
			if a.SetRequestHeader != nil {
				uc.SetRequestHeader = &upcloud.LoadBalancerActionSetHeader{Header: a.SetRequestHeader.Header, Value: a.SetRequestHeader.Value}
			}
		case lb.ActionTypeSetResponseHeader:
			if a.SetResponseHeader != nil {
				uc.SetResponseHeader = &upcloud.LoadBalancerActionSetHeader{Header: a.SetResponseHeader.Header, Value: a.SetResponseHeader.Value}
			}
		}
		out = append(out, uc)
	}
	return out
}

// toRule converts the spec to a request frontend rule (for create/replace).
func toRule(r *lb.LoadBalancerFrontendRule) request.LoadBalancerFrontendRule {
	cond := upcloud.LoadBalancerMatchingConditionAnd
	if r.Spec.MatchingCondition != "" {
		cond = upcloud.LoadBalancerMatchingCondition(r.Spec.MatchingCondition)
	}
	return request.LoadBalancerFrontendRule{
		Name:              r.ExternalName(),
		Priority:          r.Spec.Priority,
		MatchingCondition: cond,
		Matchers:          toMatchers(r.Spec.Matchers),
		Actions:           toActions(r.Spec.Actions),
	}
}
