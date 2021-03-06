/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	"github.com/poy/service-catalog/pkg/api"
	servicecatalogrest "github.com/poy/service-catalog/pkg/registry/servicecatalog/rest"
	settingsrest "github.com/poy/service-catalog/pkg/registry/settings/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/pkg/version"
	restclient "k8s.io/client-go/rest"
)

const (
	apiServerName = "service-catalog-apiserver"
)

func restStorageProviders(
	defaultNamespace string,
	restClient restclient.Interface,
) []RESTStorageProvider {
	return []RESTStorageProvider{
		servicecatalogrest.StorageProvider{
			DefaultNamespace: defaultNamespace,
			RESTClient:       restClient,
		},
		settingsrest.StorageProvider{
			RESTClient: restClient,
		},
	}
}

func completeGenericConfig(cfg *genericapiserver.RecommendedConfig) genericapiserver.CompletedConfig {
	cfg.Serializer = api.Codecs
	completedCfg := cfg.Complete()

	version := version.Get()
	// Setting this var enables the version resource. We should populate the
	// fields of the object from above if we wish to have our own output. Or
	// establish our own version object somewhere else.
	cfg.Version = &version
	return completedCfg
}

func createSkeletonServer(genericCfg genericapiserver.CompletedConfig) (*ServiceCatalogAPIServer, error) {
	genericServer, err := genericCfg.New(apiServerName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	return &ServiceCatalogAPIServer{
		GenericAPIServer: genericServer,
	}, nil
}
