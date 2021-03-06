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

package changevalidator

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/klog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	informers "github.com/poy/service-catalog/pkg/client/informers_generated/internalversion"
	internalversion "github.com/poy/service-catalog/pkg/client/listers_generated/servicecatalog/internalversion"

	"github.com/poy/service-catalog/pkg/apis/servicecatalog"
	scadmission "github.com/poy/service-catalog/pkg/apiserver/admission"
)

const (
	// PluginName is name of admission plug-in
	PluginName = "ServicePlanChangeValidator"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(io.Reader) (admission.Interface, error) {
		return NewDenyPlanChangeIfNotUpdatable()
	})
}

// denyPlanChangeIfNotUpdatable is an implementation of admission.Interface.
// It checks if the Service Instance is being updated with a Service Plan and
// blocks the operation if the Service Class is set to PlanUpdatable=false
type denyPlanChangeIfNotUpdatable struct {
	*admission.Handler
	scLister       internalversion.ClusterServiceClassLister
	spLister       internalversion.ClusterServicePlanLister
	instanceLister internalversion.ServiceInstanceLister
}

var _ = scadmission.WantsInternalServiceCatalogInformerFactory(&denyPlanChangeIfNotUpdatable{})

func (d *denyPlanChangeIfNotUpdatable) Admit(a admission.Attributes) error {
	// we need to wait for our caches to warm
	if !d.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	// We only care about service Instances
	if a.GetResource().Group != servicecatalog.GroupName || a.GetResource().GroupResource() != servicecatalog.Resource("serviceinstances") {
		return nil
	}
	instance, ok := a.GetObject().(*servicecatalog.ServiceInstance)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Instance but was unable to be converted")
	}

	if instance.Spec.ClusterServiceClassRef == nil {
		return nil // user chose a service class that doesn't exist
	}

	sc, err := d.scLister.Get(instance.Spec.ClusterServiceClassRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(5).Infof("Could not locate service class %v, can not determine if UpdateablePlan.", instance.Spec.ClusterServiceClassRef.Name)
			return nil // should this be `return err`? why would we allow the instance in if we cannot determine it is updatable?
		}
		klog.Error(err)
		return admission.NewForbidden(a, err)
	}

	if sc.Spec.PlanUpdatable {
		return nil
	}

	if instance.Spec.GetSpecifiedClusterServicePlan() != "" {
		lister := d.instanceLister.ServiceInstances(instance.Namespace)
		origInstance, err := lister.Get(instance.Name)
		if err != nil {
			klog.Errorf("Error locating instance %v/%v", instance.Namespace, instance.Name)
			return err
		}

		externalPlanNameUpdated := instance.Spec.ClusterServicePlanExternalName != origInstance.Spec.ClusterServicePlanExternalName
		externalPlanIDUpdated := instance.Spec.ClusterServicePlanExternalID != origInstance.Spec.ClusterServicePlanExternalID
		k8sPlanUpdated := instance.Spec.ClusterServicePlanName != origInstance.Spec.ClusterServicePlanName
		if externalPlanNameUpdated || externalPlanIDUpdated || k8sPlanUpdated {
			var oldPlan, newPlan string
			if externalPlanNameUpdated {
				oldPlan = origInstance.Spec.ClusterServicePlanExternalName
				newPlan = instance.Spec.ClusterServicePlanExternalName
			} else if externalPlanIDUpdated {
				oldPlan = origInstance.Spec.ClusterServicePlanExternalID
				newPlan = instance.Spec.ClusterServicePlanExternalID
			} else {
				oldPlan = origInstance.Spec.ClusterServicePlanName
				newPlan = instance.Spec.ClusterServicePlanName
			}
			klog.V(4).Infof("update Service Instance %v/%v request specified Plan %v while original instance had %v", instance.Namespace, instance.Name, newPlan, oldPlan)
			msg := fmt.Sprintf("The Service Class %v does not allow plan changes.", sc.Name)
			klog.Error(msg)
			return admission.NewForbidden(a, errors.New(msg))
		}
	}

	return nil
}

// NewDenyPlanChangeIfNotUpdatable creates a new admission control handler that
// blocks updates to an instance service plan if the instance has
// PlanUpdatable=false
// specified Service Class
func NewDenyPlanChangeIfNotUpdatable() (admission.Interface, error) {
	return &denyPlanChangeIfNotUpdatable{
		Handler: admission.NewHandler(admission.Update),
	}, nil
}

func (d *denyPlanChangeIfNotUpdatable) SetInternalServiceCatalogInformerFactory(f informers.SharedInformerFactory) {
	scInformer := f.Servicecatalog().InternalVersion().ClusterServiceClasses()
	instanceInformer := f.Servicecatalog().InternalVersion().ServiceInstances()
	d.instanceLister = instanceInformer.Lister()
	d.scLister = scInformer.Lister()
	spInformer := f.Servicecatalog().InternalVersion().ClusterServicePlans()
	d.spLister = spInformer.Lister()

	readyFunc := func() bool {
		return scInformer.Informer().HasSynced() && instanceInformer.Informer().HasSynced() && spInformer.Informer().HasSynced()
	}

	d.SetReadyFunc(readyFunc)
}

func (d *denyPlanChangeIfNotUpdatable) ValidateInitialization() error {
	if d.scLister == nil {
		return errors.New("missing service class lister")
	}
	if d.spLister == nil {
		return errors.New("missing service plan lister")
	}
	if d.instanceLister == nil {
		return errors.New("missing instance lister")
	}
	return nil
}
