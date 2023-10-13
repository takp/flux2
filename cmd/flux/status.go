/*
Copyright 2020 The Flux authors

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

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// statusable is used to see if a resource is considered ready in the usual way
type statusable interface {
	adapter
	// this is implemented by ObjectMeta
	GetGeneration() int64
	getObservedGeneration() int64
}

// oldConditions represents the deprecated API which is sunsetting.
type oldConditions interface {
	// this is usually implemented by GOTK API objects because it's used by pkg/apis/meta
	GetStatusConditions() *[]metav1.Condition
}

func statusableConditions(object statusable) []metav1.Condition {
	if s, ok := object.(meta.ObjectWithConditions); ok {
		return s.GetConditions()
	}

	if s, ok := object.(oldConditions); ok {
		return *s.GetStatusConditions()
	}

	return []metav1.Condition{}
}

func isReady(kubeClient client.Client, namespacedName types.NamespacedName, object statusable) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		err := kubeClient.Get(ctx, namespacedName, object.asClientObject())
		if err != nil {
			return false, err
		}

		// Confirm the state we are observing is for the current generation
		if object.GetGeneration() != object.getObservedGeneration() {
			return false, nil
		}

		if c := apimeta.FindStatusCondition(statusableConditions(object), meta.ReadyCondition); c != nil {
			switch c.Status {
			case metav1.ConditionTrue:
				return true, nil
			case metav1.ConditionFalse:
				return false, fmt.Errorf(c.Message)
			}
		}
		return false, nil
	}
}

func buildComponentObjectRefs(components ...string) ([]object.ObjMetadata, error) {
	var objRefs []object.ObjMetadata
	for _, deployment := range components {
		objRefs = append(objRefs, object.ObjMetadata{
			Namespace: *kubeconfigArgs.Namespace,
			Name:      deployment,
			GroupKind: schema.GroupKind{Group: "apps", Kind: "Deployment"},
		})
	}
	return objRefs, nil
}
