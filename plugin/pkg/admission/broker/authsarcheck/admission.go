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

package authsarcheck

import (
	"fmt"
	"io"

	"github.com/poy/service-catalog/pkg/apis/servicecatalog"
	"k8s.io/klog"

	authorizationapi "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	kubeclientset "k8s.io/client-go/kubernetes"

	scadmission "github.com/poy/service-catalog/pkg/apiserver/admission"
)

const (
	// PluginName is name of admission plug-in
	PluginName = "BrokerAuthSarCheck"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(io.Reader) (admission.Interface, error) {
		return NewSARCheck()
	})
}

// sarcheck is an implementation of admission.Interface.
// It enforces the creator of a broker has proper access to the auth credentials
type sarcheck struct {
	*admission.Handler
	client kubeclientset.Interface
}

var _ = scadmission.WantsKubeClientSet(&sarcheck{})

func convertToSARExtra(extra map[string][]string) map[string]authorizationapi.ExtraValue {
	if extra == nil {
		return nil
	}

	ret := map[string]authorizationapi.ExtraValue{}
	for k, v := range extra {
		ret[k] = authorizationapi.ExtraValue(v)
	}

	return ret
}

func (s *sarcheck) Admit(a admission.Attributes) error {
	// need to wait for our caches to warm
	if !s.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}
	// only care about resources in our group
	if a.GetResource().Group != servicecatalog.GroupName {
		return nil
	}

	var namespace string
	var secretName string
	// only care about brokers and namespace brokers
	if a.GetResource().GroupResource() == servicecatalog.Resource("clusterservicebrokers") {
		clusterServiceBroker, ok := a.GetObject().(*servicecatalog.ClusterServiceBroker)
		if !ok {
			return errors.NewBadRequest("Resource was marked with kind ClusterServiceBroker, but was unable to be converted")
		}

		if clusterServiceBroker.Spec.AuthInfo == nil {
			// no auth secret to check
			return nil
		}

		var secretRef *servicecatalog.ObjectReference
		if clusterServiceBroker.Spec.AuthInfo.Basic != nil {
			secretRef = clusterServiceBroker.Spec.AuthInfo.Basic.SecretRef
		} else if clusterServiceBroker.Spec.AuthInfo.Bearer != nil {
			secretRef = clusterServiceBroker.Spec.AuthInfo.Bearer.SecretRef
		}

		if secretRef == nil {
			return nil
		}
		klog.V(5).Infof("ClusterServiceBroker %+v: evaluating auth secret ref, with authInfo %q", clusterServiceBroker, secretRef)
		namespace = secretRef.Namespace
		secretName = secretRef.Name
	} else if a.GetResource().GroupResource() == servicecatalog.Resource("servicebrokers") {
		serviceBroker, ok := a.GetObject().(*servicecatalog.ServiceBroker)
		if !ok {
			return errors.NewBadRequest("Resource was marked with kind ServiceBroker, but was unable to be converted")
		}

		if serviceBroker.Spec.AuthInfo == nil {
			// no auth secret to check
			return nil
		}

		var secretRef *servicecatalog.LocalObjectReference
		if serviceBroker.Spec.AuthInfo.Basic != nil {
			secretRef = serviceBroker.Spec.AuthInfo.Basic.SecretRef
		} else if serviceBroker.Spec.AuthInfo.Bearer != nil {
			secretRef = serviceBroker.Spec.AuthInfo.Bearer.SecretRef
		}

		if secretRef == nil {
			return nil
		}
		klog.V(5).Infof("ServiceBroker %+v: evaluating auth secret ref, with authInfo %q", serviceBroker, secretRef)
		namespace = serviceBroker.Namespace
		secretName = secretRef.Name
	}
	// if we didn't get a namespace and name, it wasn't a clusterservicebroker or broker
	if namespace == "" || secretName == "" {
		return nil
	}
	userInfo := a.GetUserInfo()

	sar := &authorizationapi.SubjectAccessReview{
		Spec: authorizationapi.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Namespace: namespace,
				Verb:      "get",
				Group:     corev1.SchemeGroupVersion.Group,
				Version:   corev1.SchemeGroupVersion.Version,
				Resource:  corev1.ResourceSecrets.String(),
				Name:      secretName,
			},
			User:   userInfo.GetName(),
			Groups: userInfo.GetGroups(),
			Extra:  convertToSARExtra(userInfo.GetExtra()),
			UID:    userInfo.GetUID(),
		},
	}
	sar, err := s.client.AuthorizationV1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return err
	}

	if !sar.Status.Allowed {
		return admission.NewForbidden(a, fmt.Errorf("broker forbidden access to auth secret (%s): Reason: %s, EvaluationError: %s", secretName, sar.Status.Reason, sar.Status.EvaluationError))
	}
	return nil
}

// NewSARCheck creates a new subject access review check admission control handler
func NewSARCheck() (admission.Interface, error) {
	return &sarcheck{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

func (s *sarcheck) SetKubeClientSet(client kubeclientset.Interface) {
	s.client = client
}

func (s *sarcheck) ValidateInitialization() error {
	if s.client == nil {
		return fmt.Errorf("missing client")
	}
	return nil
}
