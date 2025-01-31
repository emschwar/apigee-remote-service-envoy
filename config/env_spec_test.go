// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config defines the API Runtime Control config and provides
// the config loading and validation functions.

package config

// NOTE: This file should be kept free from any additional dependencies,
// especially those that are not commonly used libraries.
import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
)

func TestValidateEnvironmentSpecs(t *testing.T) {
	tests := []struct {
		desc    string
		configs []EnvironmentSpec
		hasErr  bool
		wantErr string
	}{
		{
			desc:    "good environment specs",
			configs: []EnvironmentSpec{createGoodEnvSpec()},
		},
		{
			desc: "good environment specs",
			configs: []EnvironmentSpec{{
				ID: "spec",
				APIs: []APISpec{{
					ID: "api",
					Authentication: AuthenticationRequirement{
						Requirements: JWTAuthentication{Name: "oidc"},
					},
					ConsumerAuthorization: ConsumerAuthorization{
						In: []APIOperationParameter{{
							Match: JWTClaim{Name: "client_id", Requirement: "oidc"},
						}},
					},
				}},
			}},
		},
		{
			desc:    "empty environment spec id",
			configs: []EnvironmentSpec{{}},
			hasErr:  true,
			wantErr: "environment spec IDs must be non-empty",
		},
		{
			desc: "duplicate environment spec ids",
			configs: []EnvironmentSpec{
				{
					ID: "duplicate-config",
				},
				{
					ID: "duplicate-config",
				},
			},
			hasErr:  true,
			wantErr: "environment spec IDs must be unique, got multiple duplicate-config",
		},
		{
			desc: "empty API name",
			configs: []EnvironmentSpec{
				{
					ID:   "spec",
					APIs: []APISpec{{}},
				},
			},
			hasErr:  true,
			wantErr: "API spec IDs must be non-empty",
		},
		{
			desc: "duplicate API basepaths",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID:       "api-1",
							BasePath: "/v1",
						},
						{
							ID:       "api-2",
							BasePath: "/v1",
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "API spec basepaths within each environment spec must be unique, got multiple /v1",
		},
		{
			desc: "empty operation name",
			configs: []EnvironmentSpec{
				{
					ID:   "spec",
					APIs: []APISpec{{ID: "api", Operations: []APIOperation{{}}}},
				},
			},
			hasErr:  true,
			wantErr: "operation names must be non-empty",
		},
		{
			desc: "duplicate operation names",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							Operations: []APIOperation{
								{
									Name: "duplicate-op",
								},
								{
									Name: "duplicate-op",
								},
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "operation names within each API must be unique, got multiple duplicate-op",
		},
		{
			desc: "bad operation method",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{{ID: "api", Operations: []APIOperation{{
						Name:        "op",
						HTTPMatches: []HTTPMatch{{Method: "foo"}},
					}}}},
				},
			},
			hasErr:  true,
			wantErr: "operation \"op\" uses an invalid HTTP method \"foo\"",
		},
		{
			desc: "duplicate jwt authentication requirement names",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							Authentication: AuthenticationRequirement{
								Requirements: AllAuthenticationRequirements([]AuthenticationRequirement{
									{
										Requirements: JWTAuthentication{Name: "duplicate-jwt"},
									},
									{
										Requirements: AnyAuthenticationRequirements([]AuthenticationRequirement{
											{
												Requirements: JWTAuthentication{Name: "duplicate-jwt"},
											},
										}),
									},
								}),
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "JWT authentication requirement names within each API or operation must be unique, got multiple duplicate-jwt",
		},
		{
			desc: "empty JWT authentication name",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							Authentication: AuthenticationRequirement{
								Requirements: AllAuthenticationRequirements([]AuthenticationRequirement{
									{
										Requirements: JWTAuthentication{},
									},
								}),
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "JWT authentication requirement names must be non-empty",
		},
		{
			desc: "empty header",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							ConsumerAuthorization: ConsumerAuthorization{
								In: []APIOperationParameter{
									{
										Match: Header(""),
									},
								},
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "header in API operation parameter match must be non-empty",
		},
		{
			desc: "empty query",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							Authentication: AuthenticationRequirement{
								Requirements: AllAuthenticationRequirements([]AuthenticationRequirement{
									{
										Requirements: JWTAuthentication{
											Name: "jwt",
											In: []APIOperationParameter{
												{
													Match: Query(""),
												},
											},
										},
									},
								}),
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "query in API operation parameter match must be non-empty",
		},
		{
			desc: "empty jwt claim name",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							Operations: []APIOperation{
								{
									Name: "op",
									ConsumerAuthorization: ConsumerAuthorization{
										In: []APIOperationParameter{
											{
												Match: JWTClaim{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "JWT claim name in API operation parameter match must be non-empty",
		},
		{
			desc: "consumer authz refers to non-existing jwt requirement",
			configs: []EnvironmentSpec{
				{
					ID: "spec",
					APIs: []APISpec{
						{
							ID: "api",
							Operations: []APIOperation{
								{
									Name: "op",
									ConsumerAuthorization: ConsumerAuthorization{
										In: []APIOperationParameter{
											{
												Match: JWTClaim{
													Name:        "client_id",
													Requirement: "no-such-thing",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			hasErr:  true,
			wantErr: "JWT claim requirement \"no-such-thing\" does not exist",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := ValidateEnvironmentSpecs(test.configs); (err != nil) != test.hasErr {
				t.Errorf("c.ValidateEnvironmentSpecs() returns no error, should have got error")
			} else if test.wantErr != "" && test.wantErr != err.Error() {
				t.Errorf("c.ValidateEnvironmentSpecs() returns error %v, want %s", err, test.wantErr)
			}
		})
	}
}

func TestMarshalAndUnmarshalAuthenticationRequirement(t *testing.T) {
	tests := []struct {
		desc string
		want *AuthenticationRequirement
	}{
		{
			desc: "valid jwt",
			want: &AuthenticationRequirement{
				Disabled: true,
				Requirements: JWTAuthentication{
					Name:       "foo",
					Issuer:     "bar",
					In:         []APIOperationParameter{{Match: Header("header")}},
					JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
				},
			},
		},
		{
			desc: "valid any enclosing jwt",
			want: &AuthenticationRequirement{
				Disabled: true,
				Requirements: AnyAuthenticationRequirements([]AuthenticationRequirement{
					{
						Requirements: JWTAuthentication{
							Name:       "foo",
							Issuer:     "bar",
							In:         []APIOperationParameter{{Match: Header("header")}},
							JWKSSource: RemoteJWKS{URL: "url1", CacheDuration: time.Hour},
						},
					},
					{
						Requirements: JWTAuthentication{
							Name:       "bar",
							Issuer:     "foo",
							In:         []APIOperationParameter{{Match: Query("query")}},
							JWKSSource: RemoteJWKS{URL: "url2", CacheDuration: time.Hour},
						},
					},
				}),
			},
		},
		{
			desc: "valid all enclosing jwt",
			want: &AuthenticationRequirement{
				Disabled: true,
				Requirements: AllAuthenticationRequirements([]AuthenticationRequirement{
					{
						Requirements: JWTAuthentication{
							Name:       "foo",
							Issuer:     "bar",
							In:         []APIOperationParameter{{Match: Header("header")}},
							JWKSSource: RemoteJWKS{URL: "url1", CacheDuration: time.Hour},
						},
					},
					{
						Requirements: JWTAuthentication{
							Name:       "bar",
							Issuer:     "foo",
							In:         []APIOperationParameter{{Match: Query("query")}},
							JWKSSource: RemoteJWKS{URL: "url2", CacheDuration: time.Hour},
						},
					},
				}),
			},
		},
		{
			desc: "valid any enclosing all and jwt",
			want: &AuthenticationRequirement{
				Disabled: true,
				Requirements: AnyAuthenticationRequirements([]AuthenticationRequirement{
					{
						Requirements: AllAuthenticationRequirements([]AuthenticationRequirement{
							{
								Disabled: true,
								Requirements: JWTAuthentication{
									Name:       "foo",
									Issuer:     "bar",
									In:         []APIOperationParameter{{Match: Header("header")}},
									JWKSSource: RemoteJWKS{URL: "url1", CacheDuration: time.Hour},
								},
							},
							{
								Requirements: JWTAuthentication{
									Name:       "bar",
									Issuer:     "foo",
									In:         []APIOperationParameter{{Match: Query("query")}},
									JWKSSource: RemoteJWKS{URL: "url2", CacheDuration: time.Hour},
								},
							},
						}),
					},
					{
						Requirements: JWTAuthentication{
							Name:       "bac",
							Issuer:     "foo",
							In:         []APIOperationParameter{{Match: Query("query")}},
							JWKSSource: RemoteJWKS{URL: "url3", CacheDuration: 2 * time.Hour},
						},
					},
				}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			out, err := yaml.Marshal(test.want)
			if err != nil {
				t.Fatalf("yaml.Marshal() returns unexpected: %v", err)
			}
			got := &AuthenticationRequirement{}
			if err := yaml.Unmarshal(out, got); err != nil {
				t.Errorf("yaml.Unmarshal() returns unexpected: %v", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Marshal and unmarshal results in unexpected AuthenticationRequirement diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnmarshalAuthenticationRequirementError(t *testing.T) {
	tests := []struct {
		desc    string
		data    []byte
		wantErr string
	}{
		{
			desc: "bad jwt format",
			data: []byte(`jwt: bad`),
		},
		{
			desc: "any and jwt coexist",
			data: []byte(`
any:
- jwt:
    name: foo
    issuer: bar
    in:
    - header: header
    remote_jwks:
      url: url1
      cache_duration: 1h
jwt:
  name: bar
  issuer: foo
  in:
  - query: query
  remote_jwks:
    url: url2
    cache_duration: 1h
`),
			wantErr: "precisely one of jwt, any or all should be set",
		},
		{
			desc: "all and jwt coexist",
			data: []byte(`
all:
- jwt:
    name: foo
    issuer: bar
    in:
    - header: header
    remote_jwks:
      url: url1
      cache_duration: 1h
jwt:
  name: bar
  issuer: foo
  in:
  - query: query
  remote_jwks:
    url: url2
    cache_duration: 1h
`),
			wantErr: "precisely one of jwt, any or all should be set",
		},
		{
			desc: "all and any coexist",
			data: []byte(`
all:
- jwt:
    name: foo
    issuer: bar
    in:
    - header: header
    remote_jwks:
      url: url1
      cache_duration: 1h
any:
- jwt:
    name: foo
    issuer: bar
    in:
    - header: header
    remote_jwks:
      url: url1
      cache_duration: 1h
`),
			wantErr: "precisely one of jwt, any or all should be set",
		},
		{
			desc: "disabled:true should eliminate validation err",
			data: []byte(`
disabled: true
all:
- jwt:
    name: foo
    issuer: bar
    in:
    - header: header
    remote_jwks:
      url: url1
      cache_duration: 1h
any:
- jwt:
    name: foo
    issuer: bar
    in:
    - header: header
    remote_jwks:
      url: url1
      cache_duration: 1h
`),
			wantErr: "",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			a := &AuthenticationRequirement{}
			if err := yaml.Unmarshal(test.data, a); test.wantErr != "" && err == nil {
				t.Errorf("yaml.Unmarshal() returns no error, should have got error")
			} else if test.wantErr != "" && err.Error() != test.wantErr {
				t.Errorf("yaml.Unmarshal() returns error %v, want %s", err, test.wantErr)
			}
		})
	}
}

func TestMarshalAndUnmarshalJWTAuthentication(t *testing.T) {
	tests := []struct {
		desc string
		want *JWTAuthentication
	}{
		{
			desc: "valid remote_jwks",
			want: &JWTAuthentication{
				Name:       "foo",
				Issuer:     "bar",
				In:         []APIOperationParameter{{Match: Header("header")}, {Match: Query("query")}},
				JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			out, err := yaml.Marshal(test.want)
			if err != nil {
				t.Fatalf("yaml.Marshal() returns unexpected: %v", err)
			}
			got := &JWTAuthentication{}
			if err := yaml.Unmarshal(out, got); err != nil {
				t.Errorf("yaml.Unmarshal() returns unexpected: %v", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Marshal and unmarshal results in unexpected JWTAuthentication diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnmarshalJWTAuthenticationError(t *testing.T) {
	tests := []struct {
		desc    string
		data    []byte
		wantErr string
	}{
		{
			desc: "no jwks source",
			data: []byte(`
name: foo
issuer: bar
in:
- header: header
`),
			wantErr: "remote jwks not found",
		},
		{
			desc: "bad audiences format",
			data: []byte(`
name: foo
issuer: bar
audiences: bad
remote_jwks:
  url: url
in:
- header: header
`),
		},
		{
			desc: "bad jwks source format",
			data: []byte(`
name: foo
issuer: bar
remote_jwks: bad
in:
- header: header
`),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			p := &JWTAuthentication{}
			if err := yaml.Unmarshal(test.data, p); err == nil {
				t.Errorf("yaml.Unmarshal() returns no error, should have got error")
			} else if test.wantErr != "" && err.Error() != test.wantErr {
				t.Errorf("yaml.Unmarshal() returns error %v, want %s", err, test.wantErr)
			}
		})
	}
}

type testJWKSSource string

func (testJWKSSource) jwksSource() {}

func TestMarshalJWTAuthenticationError(t *testing.T) {

	p := JWTAuthentication{
		Name:       "foo",
		Issuer:     "bar",
		In:         []APIOperationParameter{{Match: Header("header")}},
		JWKSSource: testJWKSSource("bad"),
	}
	wantErr := "unsupported jwks source"

	if _, err := yaml.Marshal(p); err == nil {
		t.Errorf("yaml.Marshal() returns no error, should have got error")
	} else if err.Error() != wantErr {
		t.Errorf("yaml.Marshal() returns error %v, want %s", err, wantErr)
	}
}

func TestMarshalAndUnmarshalAPIOperationParameter(t *testing.T) {
	tests := []struct {
		desc string
		want *APIOperationParameter
	}{
		{
			desc: "valid API operation parameter with header",
			want: &APIOperationParameter{Match: Header("header")},
		},
		{
			desc: "valid API operation parameter with query",
			want: &APIOperationParameter{Match: Query("query")},
		},
		{
			desc: "valid API operation parameter with jwt claim",
			want: &APIOperationParameter{Match: JWTClaim{Requirement: "foo", Name: "bar"}},
		},
		{
			desc: "valid API operation parameter with jwt claim and transformation",
			want: &APIOperationParameter{
				Match:          JWTClaim{Requirement: "foo", Name: "bar"},
				Transformation: StringTransformation{Template: "temp", Substitution: "sub"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			out, err := yaml.Marshal(test.want)
			if err != nil {
				t.Fatalf("yaml.Marshal() returns unexpected: %v", err)
			}
			got := &APIOperationParameter{}
			if err := yaml.Unmarshal(out, got); err != nil {
				t.Errorf("yaml.Unmarshal() returns unexpected: %v", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Marshal and unmarshal results in unexpected APIOperationParameter diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnmarshalAPIOperationParameterError(t *testing.T) {
	tests := []struct {
		desc    string
		data    []byte
		wantErr string
	}{
		{
			desc: "transformation in bad format",
			data: []byte(`
header: header
transformation: bad
`),
		},
		{
			desc: "jwt_claim in bad format",
			data: []byte(`
jwt_claim: bad
`),
		},
		{
			desc: "jwt claim and header coexist",
			data: []byte(`
jwt_claim:
  requirement: foo
  name: bar
header: header
`),
			wantErr: "precisely one header, query or jwt_claim should be set, got 2",
		},
		{
			desc: "jwt claim and query coexist",
			data: []byte(`
jwt_claim:
  requirement: foo
  name: bar
query: query
`),
			wantErr: "precisely one header, query or jwt_claim should be set, got 2",
		},
		{
			desc: "header and query coexist",
			data: []byte(`
header: header
query: query
`),
			wantErr: "precisely one header, query or jwt_claim should be set, got 2",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			p := &APIOperationParameter{}
			if err := yaml.Unmarshal(test.data, p); err == nil {
				t.Errorf("yaml.Unmarshal() returns no error, should have got error")
			} else if test.wantErr != "" && err.Error() != test.wantErr {
				t.Errorf("yaml.Unmarshal() returns error %v, want %s", err, test.wantErr)
			}
		})
	}
}

type testParamMatch string

func (testParamMatch) paramMatch() {}

func TestMarshalAPIOperationParameterError(t *testing.T) {

	p := APIOperationParameter{
		Match: testParamMatch("bad"),
	}
	wantErr := "unsupported match type"

	if _, err := yaml.Marshal(p); err == nil {
		t.Errorf("yaml.Marshal() returns no error, should have got error")
	} else if err.Error() != wantErr {
		t.Errorf("yaml.Marshal() returns error %v, want %s", err, wantErr)
	}
}

func TestAuthenticationRequirementTypes(t *testing.T) {
	j := JWTAuthentication{}
	j.authenticationRequirements()

	any := AnyAuthenticationRequirements{}
	any.authenticationRequirements()

	all := AllAuthenticationRequirements{}
	all.authenticationRequirements()
}

func TestJWKSSourceTypes(t *testing.T) {
	j := RemoteJWKS{}
	j.jwksSource()
}

func TestCORSPolicy(t *testing.T) {
	tests := []struct {
		desc                string
		allowOrigins        []string
		allowOriginsRegexes []string
		wantEmpty           bool
	}{
		{"empty", []string{}, []string{}, true},
		{"allowOrigins", []string{"*"}, []string{}, false},
		{"allowOriginRegexes", []string{}, []string{"*"}, false},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			c := CorsPolicy{
				AllowOrigins:        test.allowOrigins,
				AllowOriginsRegexes: test.allowOriginsRegexes,
			}
			if test.wantEmpty != c.IsEmpty() {
				t.Errorf("want %v got %v", test.wantEmpty, c.IsEmpty())
			}
		})
	}
}

func TestParamMatchTypes(t *testing.T) {
	h := Header("header")
	h.paramMatch()

	q := Query("query")
	q.paramMatch()

	j := JWTClaim{}
	j.paramMatch()
}

func createGoodEnvSpec() EnvironmentSpec {
	envSpecs := []EnvironmentSpec{{
		ID: "good-env-config",
		APIs: []APISpec{
			{
				ID:       "apispec1",
				BasePath: "/v1",
				Authentication: AuthenticationRequirement{
					Requirements: AnyAuthenticationRequirements{
						AuthenticationRequirement{
							Requirements: AllAuthenticationRequirements{
								AuthenticationRequirement{
									Requirements: JWTAuthentication{
										Name:       "foo",
										Issuer:     "issuer",
										JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
										In: []APIOperationParameter{
											{
												Match: Header("jwt"),
												Transformation: StringTransformation{
													Template:     "{identity}",
													Substitution: "{identity}",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				ConsumerAuthorization: ConsumerAuthorization{
					In: []APIOperationParameter{
						{Match: Query("x-api-key")},
						{Match: Header("x-api-key")},
					},
				},
				Operations: []APIOperation{
					{
						Name: "op-1",
						HTTPMatches: []HTTPMatch{
							{
								PathTemplate: "/petstore",
								Method:       "GET",
							},
						},
					},
					{
						Name: "op-2",
						HTTPMatches: []HTTPMatch{
							{
								PathTemplate: "/bookshop",
								Method:       "POST",
							},
						},
					},
					{
						Name: "op-3",
						HTTPMatches: []HTTPMatch{
							{
								PathTemplate: "/airport",
								Method:       "GET",
							},
						},
						Authentication: AuthenticationRequirement{
							Requirements: JWTAuthentication{
								Name:       "foo",
								Issuer:     "issuer",
								Audiences:  []string{"foo", "bac"},
								JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
								In: []APIOperationParameter{
									{
										Match: Header("jwt"),
									},
								},
							},
						},
					},
					{
						Name: "op-4",
						HTTPMatches: []HTTPMatch{
							{
								PathTemplate: "/noauthz",
								Method:       "GET",
							},
						},
						ConsumerAuthorization: ConsumerAuthorization{
							Disabled: true,
						},
					},
				},
				HTTPRequestTransforms: HTTPRequestTransforms{
					HeaderTransforms: NameValueTransforms{
						Add: []AddNameValue{
							{Name: "x-apigee-target", Value: "target"},
						},
					},
					PathTransform: "/target_prefix/{path}",
				},
				Cors: CorsPolicy{
					AllowOrigins: []string{"*"},
				},
			},
			{
				ID:       "apispec2",
				BasePath: "/v2",
				Authentication: AuthenticationRequirement{
					Requirements: JWTAuthentication{
						Name:       "foo",
						Issuer:     "issuer-0",
						JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
						In: []APIOperationParameter{
							{Match: Header("jwt")},
							{Match: Header("x-custom-auth-token")},
						},
					},
				},
				Operations: []APIOperation{
					{
						Name: "op-3",
						HTTPMatches: []HTTPMatch{
							{
								PathTemplate: "/petstore",
								Method:       anyMethod,
							},
						},
						Authentication: AuthenticationRequirement{
							Requirements: JWTAuthentication{
								Name:       "foo",
								Issuer:     "issuer",
								JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
								In:         []APIOperationParameter{{Match: Header("jwt")}},
							},
						},
						ConsumerAuthorization: ConsumerAuthorization{
							In: []APIOperationParameter{
								{Match: Query("x-api-key2")},
								{Match: Header("x-api-key2")},
								{
									Match: Header("authorization"),
									Transformation: StringTransformation{
										Template:     "Bearer {token}",
										Substitution: "{token}",
									},
								},
							},
						},
					},
					{
						Name: "op-4",
						HTTPMatches: []HTTPMatch{
							{
								PathTemplate: "/petstore/pets",
								Method:       "GET",
							},
						},
						Authentication: AuthenticationRequirement{
							Requirements: AllAuthenticationRequirements{
								AuthenticationRequirement{
									Requirements: JWTAuthentication{
										Name:       "foo",
										Issuer:     "issuer2",
										JWKSSource: RemoteJWKS{URL: "url2", CacheDuration: time.Hour},
										In:         []APIOperationParameter{{Match: Header("jwt")}},
									},
								},
								AuthenticationRequirement{
									Requirements: JWTAuthentication{
										Name:       "bar",
										Issuer:     "issuer2",
										JWKSSource: RemoteJWKS{URL: "url2", CacheDuration: time.Hour},
										In:         []APIOperationParameter{{Match: Header("jwt")}},
									},
								},
							},
						},
					},
				},
			},
			{
				ID:         "no-operations-api",
				BasePath:   "/v3",
				Operations: []APIOperation{},
				Authentication: AuthenticationRequirement{
					Requirements: JWTAuthentication{
						Name:       "foo",
						Issuer:     "issuer",
						JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
						In:         []APIOperationParameter{{Match: Header("jwt")}},
					},
				},
				ConsumerAuthorization: ConsumerAuthorization{
					In: []APIOperationParameter{
						{Match: Query("x-api-key")},
						{Match: Header("x-api-key")},
					},
				},
			},
			{
				ID:       "empty-operation",
				BasePath: "/v4/*",
				Operations: []APIOperation{
					{
						Name:        "empty",
						HTTPMatches: []HTTPMatch{},
					},
				},
				Authentication: AuthenticationRequirement{
					Requirements: JWTAuthentication{
						Name:       "foo",
						Issuer:     "issuer",
						JWKSSource: RemoteJWKS{URL: "url", CacheDuration: time.Hour},
						In:         []APIOperationParameter{{Match: Header("jwt")}},
					},
				},
				ConsumerAuthorization: ConsumerAuthorization{
					In: []APIOperationParameter{
						{Match: Query("x-api-key")},
						{Match: Header("x-api-key")},
					},
				},
			}},
	}}
	_ = ValidateEnvironmentSpecs(envSpecs)
	return envSpecs[0]
}
