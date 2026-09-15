/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or distributed under the License is
distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied. See the License for the specific language
governing permissions and limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/polarsquad/upcloud-operator/api/common"
)

// Rule matcher type constants. The RuleMatcher.Type field is one of these;
// the CEL rule on RuleMatcher enforces the matching sub-struct is set.
const (
	MatcherTypeSrcIP          = "src_ip"
	MatcherTypeSrcPort        = "src_port"
	MatcherTypeBodySize       = "body_size"
	MatcherTypePath           = "path"
	MatcherTypeURL            = "url"
	MatcherTypeURLQuery       = "url_query"
	MatcherTypeHost           = "host"
	MatcherTypeHTTPMethod     = "http_method"
	MatcherTypeHTTPStatus     = "http_status"
	MatcherTypeCookie         = "cookie"
	MatcherTypeHeader         = "header"
	MatcherTypeRequestHeader  = "request_header"
	MatcherTypeResponseHeader = "response_header"
	MatcherTypeURLParam       = "url_param"
	MatcherTypeNumMembersUp   = "num_members_up"
)

// Rule action type constants. The RuleAction.Type field is one of these; the
// CEL rule on RuleAction enforces the matching sub-struct is set.
const (
	ActionTypeUseBackend          = "use_backend"
	ActionTypeTCPReject           = "tcp_reject"
	ActionTypeHTTPReturn          = "http_return"
	ActionTypeHTTPRedirect        = "http_redirect"
	ActionTypeHTTPRewritePath     = "http_rewrite_path"
	ActionTypeHTTPRewriteURI      = "http_rewrite_uri"
	ActionTypeSetForwardedHeaders = "set_forwarded_headers"
	ActionTypeSetRequestHeader    = "set_request_header"
	ActionTypeSetResponseHeader   = "set_response_header"
)

// MatcherSrcIP is a "src_ip" matcher.
type MatcherSrcIP struct {
	Value string `json:"value"`
}

// MatcherInteger is a matcher over an integer (src_port, body_size,
// http_status).
type MatcherInteger struct {
	// +kubebuilder:validation:Enum=equal;greater;greater_or_equal;less;less_or_equal;range
	Method string `json:"method"`
	Value  int    `json:"value"`
	// +optional
	RangeStart *int `json:"rangeStart,omitempty"`
	// +optional
	RangeEnd *int `json:"rangeEnd,omitempty"`
}

// MatcherString is a matcher over a string (path, url, url_query).
type MatcherString struct {
	// +kubebuilder:validation:Enum=exact;substring;regexp;starts;ends;domain;ip;exists
	Method string `json:"method"`
	Value  string `json:"value"`
	// +optional
	IgnoreCase *bool `json:"ignoreCase,omitempty"`
}

// MatcherStringWithArgument is a matcher over a named string (cookie, header,
// request_header, response_header, url_param).
type MatcherStringWithArgument struct {
	// +kubebuilder:validation:Enum=exact;substring;regexp;starts;ends;domain;ip;exists
	Method string `json:"method"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	// +optional
	IgnoreCase *bool `json:"ignoreCase,omitempty"`
}

// MatcherHost is a "host" matcher.
type MatcherHost struct {
	Value string `json:"value"`
}

// MatcherHTTPMethod is a "http_method" matcher.
type MatcherHTTPMethod struct {
	Value string `json:"value"`
}

// MatcherNumMembersUp is a "num_members_up" matcher.
type MatcherNumMembersUp struct {
	// +kubebuilder:validation:Enum=equal;greater;greater_or_equal;less;less_or_equal;range
	Method string `json:"method"`
	Value  int    `json:"value"`
	// Backend is the name of a LoadBalancerBackend.
	Backend string `json:"backend"`
}

// RuleMatcher is one matcher on a rule. Type selects which sub-struct is used;
// exactly one sub-struct matching Type may be set (enforced by CEL).
type RuleMatcher struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=src_ip;src_port;body_size;path;url;url_query;host;http_method;http_status;cookie;header;request_header;response_header;url_param;num_members_up
	Type string `json:"type"`
	// +optional
	Inverse *bool `json:"inverse,omitempty"`
	// +optional
	SrcIP *MatcherSrcIP `json:"srcIP,omitempty"`
	// +optional
	SrcPort *MatcherInteger `json:"srcPort,omitempty"`
	// +optional
	BodySize *MatcherInteger `json:"bodySize,omitempty"`
	// +optional
	Path *MatcherString `json:"path,omitempty"`
	// +optional
	URL *MatcherString `json:"url,omitempty"`
	// +optional
	URLQuery *MatcherString `json:"urlQuery,omitempty"`
	// +optional
	Host *MatcherHost `json:"host,omitempty"`
	// +optional
	HTTPMethod *MatcherHTTPMethod `json:"httpMethod,omitempty"`
	// +optional
	HTTPStatus *MatcherInteger `json:"httpStatus,omitempty"`
	// +optional
	Cookie *MatcherStringWithArgument `json:"cookie,omitempty"`
	// +optional
	Header *MatcherStringWithArgument `json:"header,omitempty"`
	// +optional
	RequestHeader *MatcherStringWithArgument `json:"requestHeader,omitempty"`
	// +optional
	ResponseHeader *MatcherStringWithArgument `json:"responseHeader,omitempty"`
	// +optional
	URLParam *MatcherStringWithArgument `json:"urlParam,omitempty"`
	// +optional
	NumMembersUp *MatcherNumMembersUp `json:"numMembersUp,omitempty"`

	// +kubebuilder:validation:XValidation:rule="(self.type == 'src_ip' ? has(self.srcIP) : !has(self.srcIP)) && (self.type == 'src_port' ? has(self.srcPort) : !has(self.srcPort)) && (self.type == 'body_size' ? has(self.bodySize) : !has(self.bodySize)) && (self.type == 'path' ? has(self.path) : !has(self.path)) && (self.type == 'url' ? has(self.url) : !has(self.url)) && (self.type == 'url_query' ? has(self.urlQuery) : !has(self.urlQuery)) && (self.type == 'host' ? has(self.host) : !has(self.host)) && (self.type == 'http_method' ? has(self.httpMethod) : !has(self.httpMethod)) && (self.type == 'http_status' ? has(self.httpStatus) : !has(self.httpStatus)) && (self.type == 'cookie' ? has(self.cookie) : !has(self.cookie)) && (self.type == 'header' ? has(self.header) : !has(self.header)) && (self.type == 'request_header' ? has(self.requestHeader) : !has(self.requestHeader)) && (self.type == 'response_header' ? has(self.responseHeader) : !has(self.responseHeader)) && (self.type == 'url_param' ? has(self.urlParam) : !has(self.urlParam)) && (self.type == 'num_members_up' ? has(self.numMembersUp) : !has(self.numMembersUp))",message="exactly the sub-struct matching type must be set"
}

// ActionUseBackend is a "use_backend" action.
type ActionUseBackend struct {
	// Backend is the name of a LoadBalancerBackend.
	Backend string `json:"backend"`
}

// ActionHTTPReturn is an "http_return" action.
type ActionHTTPReturn struct {
	Status int `json:"status"`
	// +optional
	ContentType string `json:"contentType,omitempty"`
	// +optional
	Payload string `json:"payload,omitempty"`
}

// ActionHTTPRedirect is an "http_redirect" action.
type ActionHTTPRedirect struct {
	Location string `json:"location"`
	// +optional
	// +kubebuilder:validation:Enum=http;https
	Scheme string `json:"scheme,omitempty"`
	// +optional
	Status int `json:"status,omitempty"`
}

// ActionHTTPRewritePath is an "http_rewrite_path" action.
type ActionHTTPRewritePath struct {
	MatchPattern string `json:"matchPattern"`
	RewriteTo    string `json:"rewriteTo"`
}

// ActionHTTPRewriteURI is an "http_rewrite_uri" action.
type ActionHTTPRewriteURI struct {
	MatchPattern string `json:"matchPattern"`
	RewriteTo    string `json:"rewriteTo"`
}

// ActionSetHeader is a "set_request_header" or "set_response_header" action.
type ActionSetHeader struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}

// RuleAction is one action on a rule. Type selects which sub-struct is used;
// exactly one sub-struct matching Type may be set (enforced by CEL).
type RuleAction struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=use_backend;tcp_reject;http_return;http_redirect;http_rewrite_path;http_rewrite_uri;set_forwarded_headers;set_request_header;set_response_header
	Type string `json:"type"`
	// +optional
	UseBackend *ActionUseBackend `json:"useBackend,omitempty"`
	// +optional
	HTTPReturn *ActionHTTPReturn `json:"httpReturn,omitempty"`
	// +optional
	HTTPRedirect *ActionHTTPRedirect `json:"httpRedirect,omitempty"`
	// +optional
	HTTPRewritePath *ActionHTTPRewritePath `json:"httpRewritePath,omitempty"`
	// +optional
	HTTPRewriteURI *ActionHTTPRewriteURI `json:"httpRewriteURI,omitempty"`
	// +optional
	SetRequestHeader *ActionSetHeader `json:"setRequestHeader,omitempty"`
	// +optional
	SetResponseHeader *ActionSetHeader `json:"setResponseHeader,omitempty"`

	// +kubebuilder:validation:XValidation:rule="(self.type == 'use_backend' ? has(self.useBackend) : !has(self.useBackend)) && (self.type == 'http_return' ? has(self.httpReturn) : !has(self.httpReturn)) && (self.type == 'http_redirect' ? has(self.httpRedirect) : !has(self.httpRedirect)) && (self.type == 'http_rewrite_path' ? has(self.httpRewritePath) : !has(self.httpRewritePath)) && (self.type == 'http_rewrite_uri' ? has(self.httpRewriteURI) : !has(self.httpRewriteURI)) && (self.type == 'set_request_header' ? has(self.setRequestHeader) : !has(self.setRequestHeader)) && (self.type == 'set_response_header' ? has(self.setResponseHeader) : !has(self.setResponseHeader))",message="exactly the sub-struct matching type must be set"
}

// LoadBalancerFrontendRuleSpec defines the desired state of a frontend rule.
type LoadBalancerFrontendRuleSpec struct {
	// FrontendRef points at the owning LoadBalancerFrontend. Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="frontendRef is immutable"
	FrontendRef common.LocalObjectReference `json:"frontendRef"`
	// Name of the rule in UpCloud. Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`
	// Priority: 0-100, higher is evaluated first.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Priority int `json:"priority"`
	// MatchingCondition combines the matchers; defaults to "and".
	// +optional
	// +kubebuilder:default=and
	// +kubebuilder:validation:Enum=and;or
	MatchingCondition string `json:"matchingCondition,omitempty"`
	// Matchers is the set of matchers.
	// +optional
	Matchers []RuleMatcher `json:"matchers,omitempty"`
	// Actions are evaluated in order.
	// +optional
	Actions []RuleAction `json:"actions,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerFrontendRuleStatus defines the observed state of
// LoadBalancerFrontendRule.
type LoadBalancerFrontendRuleStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent load balancer.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// FrontendName is the owning frontend's name in UpCloud.
	FrontendName string `json:"frontendName,omitempty"`
	// Name of the rule in UpCloud.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerFrontendRule is a load balancer frontend rule.
type LoadBalancerFrontendRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerFrontendRuleSpec   `json:"spec"`
	Status LoadBalancerFrontendRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerFrontendRuleList contains a list of LoadBalancerFrontendRule.
type LoadBalancerFrontendRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerFrontendRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerFrontendRule{}, &LoadBalancerFrontendRuleList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (o *LoadBalancerFrontendRule) GetConditions() []metav1.Condition { return o.Status.Conditions }

// SetConditions implements reconciler.Object.
func (o *LoadBalancerFrontendRule) SetConditions(c []metav1.Condition) { o.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (o *LoadBalancerFrontendRule) GetDeletionPolicy() common.DeletionPolicy {
	return o.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (o *LoadBalancerFrontendRule) GetExternalID() string { return o.Status.Name }

// ExternalName is the name used in UpCloud.
func (o *LoadBalancerFrontendRule) ExternalName() string {
	if o.Spec.Name != "" {
		return o.Spec.Name
	}
	return o.Name
}

// RefNames returns the referenced frontend name, for watches.
func (o *LoadBalancerFrontendRule) RefNames() []string {
	return []string{o.Spec.FrontendRef.Name}
}
