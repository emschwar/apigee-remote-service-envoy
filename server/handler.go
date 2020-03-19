// Copyright 2020 Google LLC
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

package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/apigee/apigee-remote-service-golib/analytics"
	"github.com/apigee/apigee-remote-service-golib/auth"
	"github.com/apigee/apigee-remote-service-golib/product"
	"github.com/apigee/apigee-remote-service-golib/quota"
)

type handler struct {
	managementAPI     *url.URL
	remoteServiceAPI  *url.URL
	fluentdConfigFile string
	orgName           string
	envName           string
	key               string
	secret            string
	apiKeyClaimKey    string

	productMan   *product.Manager
	authMan      *auth.Manager
	analyticsMan analytics.Manager
	quotaMan     *quota.Manager
}

func (h *handler) ManagementAPI() *url.URL {
	return h.managementAPI
}
func (h *handler) RemoteServiceAPI() *url.URL {
	return h.remoteServiceAPI
}
func (h *handler) Organization() string {
	return h.orgName
}
func (h *handler) Environment() string {
	return h.envName
}
func (h *handler) Key() string {
	return h.key
}
func (h *handler) Secret() string {
	return h.secret
}

// NewHandler creates a handler
func NewHandler(config *Config) (*handler, error) {

	var managementAPI, remoteServiceAPI *url.URL
	var err error
	if config.Tenant.ManagementAPI != "" {
		managementAPI, err = url.Parse(config.Tenant.ManagementAPI)
		if err != nil {
			return nil, err
		}
	}
	if config.Tenant.RemoteServiceAPI != "" {
		remoteServiceAPI, err = url.Parse(config.Tenant.RemoteServiceAPI)
		if err != nil {
			return nil, err
		}
	}

	tr := http.DefaultTransport
	httpClient := &http.Client{
		Timeout:   config.Tenant.ClientTimeout,
		Transport: tr,
	}

	productMan, err := product.NewManager(product.Options{
		Client:      httpClient,
		BaseURL:     remoteServiceAPI,
		RefreshRate: config.Products.RefreshRate,
		Key:         config.Tenant.Key,
		Secret:      config.Tenant.Secret,
	})
	if err != nil {
		return nil, err
	}

	authMan, err := auth.NewManager(auth.Options{
		PollInterval:        config.Auth.JWKSPollInterval,
		Client:              httpClient,
		APIKeyCacheDuration: config.Auth.APIKeyCacheDuration,
	})
	if err != nil {
		return nil, err
	}

	quotaMan, err := quota.NewManager(quota.Options{
		BaseURL: remoteServiceAPI,
		Client:  httpClient,
		Key:     config.Tenant.Key,
		Secret:  config.Tenant.Secret,
	})
	if err != nil {
		return nil, err
	}

	tempDirMode := os.FileMode(0700)
	tempDir := config.Global.TempDir
	analyticsDir := filepath.Join(tempDir, "analytics")
	if err := os.MkdirAll(analyticsDir, tempDirMode); err != nil {
		return nil, err
	}

	analyticsMan, err := analytics.NewManager(analytics.Options{
		LegacyEndpoint:     false,
		BufferPath:         analyticsDir,
		StagingFileLimit:   2024,
		BaseURL:            managementAPI,
		Key:                config.Tenant.Key,
		Secret:             config.Tenant.Secret,
		Client:             httpClient,
		SendChannelSize:    10,
		FluentdConfigFile:  config.Tenant.FluentdConfigFile,
		CollectionInterval: time.Minute,
	})

	h := &handler{
		remoteServiceAPI:  remoteServiceAPI,
		managementAPI:     managementAPI,
		fluentdConfigFile: config.Tenant.FluentdConfigFile,
		orgName:           config.Tenant.OrgName,
		envName:           config.Tenant.EnvName,
		key:               config.Tenant.Key,
		secret:            config.Tenant.Secret,
		productMan:        productMan,
		authMan:           authMan,
		analyticsMan:      analyticsMan,
		quotaMan:          quotaMan,
		apiKeyClaimKey:    config.Auth.APIKeyClaim,
	}

	return h, nil
}
