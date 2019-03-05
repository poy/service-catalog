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

package integration

import (
	"log"

	"github.com/poy/service-catalog/pkg/api"
	"github.com/poy/service-catalog/pkg/apis/servicecatalog"
	_ "github.com/poy/service-catalog/pkg/apis/servicecatalog/install"
	"github.com/poy/service-catalog/pkg/apis/servicecatalog/testapi"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/rest"
)

func serviceCatalogAPIGroup() testapi.TestGroup {
	// OOPS: didn't register the right group version
	groupVersion := schema.GroupVersion{Group: servicecatalog.GroupName, Version: "v1beta1"}

	externalGroupVersion := schema.GroupVersion{
		Group:   servicecatalog.GroupName,
		Version: api.Scheme.PrioritizedVersionsForGroup(servicecatalog.GroupName)[0].Version,
	}

	return testapi.NewTestGroup(
		groupVersion,
		servicecatalog.SchemeGroupVersion,
		api.Scheme.KnownTypes(servicecatalog.SchemeGroupVersion),
		api.Scheme.KnownTypes(externalGroupVersion),
	)
}

func init() {
	log.SetFlags(log.Lshortfile)
	testapi.Groups[servicecatalog.GroupName] = serviceCatalogAPIGroup()
}
