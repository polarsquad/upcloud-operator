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

// toMatchers converts the spec matchers to SDK matchers.
func toMatchers(in []lb.RuleMatcher) []upcloud.LoadBalancerMatcher {
	out := make([]upcloud.LoadBalancerMatcher, 0, len(in))
	for i := range in {
		m := in[i]
		uc := upcloud.LoadBalancerMatcher{Type: upcloud.LoadBalancerMatcherType(m.Type), Inverse: m.Inverse}
		switch m.Type {
		case lb.MatcherTypeSrcIP:
			if m.SrcIP != nil {
				uc.SrcIP = &upcloud.LoadBalancerMatcherSourceIP{Value: m.SrcIP.Value}
			}
		case lb.MatcherTypeSrcPort:
			if m.SrcPort != nil {
				uc.SrcPort = &upcloud.LoadBalancerMatcherInteger{Method: integerMethod(m.SrcPort.Method), Value: m.SrcPort.Value, RangeStart: rangeStart(m.SrcPort.RangeStart), RangeEnd: rangeEnd(m.SrcPort.RangeEnd)}
			}
		case lb.MatcherTypeBodySize:
			if m.BodySize != nil {
				uc.BodySize = &upcloud.LoadBalancerMatcherInteger{Method: integerMethod(m.BodySize.Method), Value: m.BodySize.Value, RangeStart: rangeStart(m.BodySize.RangeStart), RangeEnd: rangeEnd(m.BodySize.RangeEnd)}
			}
		case lb.MatcherTypePath:
			if m.Path != nil {
				uc.Path = &upcloud.LoadBalancerMatcherString{Method: stringMethod(m.Path.Method), Value: m.Path.Value, IgnoreCase: m.Path.IgnoreCase}
			}
		case lb.MatcherTypeURL:
			if m.URL != nil {
				uc.URL = &upcloud.LoadBalancerMatcherString{Method: stringMethod(m.URL.Method), Value: m.URL.Value, IgnoreCase: m.URL.IgnoreCase}
			}
		case lb.MatcherTypeURLQuery:
			if m.URLQuery != nil {
				uc.URLQuery = &upcloud.LoadBalancerMatcherString{Method: stringMethod(m.URLQuery.Method), Value: m.URLQuery.Value, IgnoreCase: m.URLQuery.IgnoreCase}
			}
		case lb.MatcherTypeHost:
			if m.Host != nil {
				uc.Host = &upcloud.LoadBalancerMatcherHost{Value: m.Host.Value}
			}
		case lb.MatcherTypeHTTPMethod:
			if m.HTTPMethod != nil {
				uc.HTTPMethod = &upcloud.LoadBalancerMatcherHTTPMethod{Value: httpMethod(m.HTTPMethod.Value)}
			}
		case lb.MatcherTypeHTTPStatus:
			if m.HTTPStatus != nil {
				uc.HTTPStatus = &upcloud.LoadBalancerMatcherInteger{Method: integerMethod(m.HTTPStatus.Method), Value: m.HTTPStatus.Value, RangeStart: rangeStart(m.HTTPStatus.RangeStart), RangeEnd: rangeEnd(m.HTTPStatus.RangeEnd)}
			}
		case lb.MatcherTypeCookie:
			if m.Cookie != nil {
				uc.Cookie = &upcloud.LoadBalancerMatcherStringWithArgument{Method: stringMethod(m.Cookie.Method), Name: m.Cookie.Name, Value: m.Cookie.Value, IgnoreCase: m.Cookie.IgnoreCase}
			}
		case lb.MatcherTypeHeader:
			if m.Header != nil {
				uc.Header = &upcloud.LoadBalancerMatcherStringWithArgument{Method: stringMethod(m.Header.Method), Name: m.Header.Name, Value: m.Header.Value, IgnoreCase: m.Header.IgnoreCase}
			}
		case lb.MatcherTypeRequestHeader:
			if m.RequestHeader != nil {
				uc.RequestHeader = &upcloud.LoadBalancerMatcherStringWithArgument{Method: stringMethod(m.RequestHeader.Method), Name: m.RequestHeader.Name, Value: m.RequestHeader.Value, IgnoreCase: m.RequestHeader.IgnoreCase}
			}
		case lb.MatcherTypeResponseHeader:
			if m.ResponseHeader != nil {
				uc.ResponseHeader = &upcloud.LoadBalancerMatcherStringWithArgument{Method: stringMethod(m.ResponseHeader.Method), Name: m.ResponseHeader.Name, Value: m.ResponseHeader.Value, IgnoreCase: m.ResponseHeader.IgnoreCase}
			}
		case lb.MatcherTypeURLParam:
			if m.URLParam != nil {
				uc.URLParam = &upcloud.LoadBalancerMatcherStringWithArgument{Method: stringMethod(m.URLParam.Method), Name: m.URLParam.Name, Value: m.URLParam.Value, IgnoreCase: m.URLParam.IgnoreCase}
			}
		case lb.MatcherTypeNumMembersUp:
			if m.NumMembersUp != nil {
				uc.NumMembersUp = &upcloud.LoadBalancerMatcherNumMembersUp{Method: integerMethod(m.NumMembersUp.Method), Value: m.NumMembersUp.Value, Backend: m.NumMembersUp.Backend}
			}
		}
		out = append(out, uc)
	}
	return out
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
