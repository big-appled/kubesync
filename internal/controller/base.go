package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	jibutechcomv1 "github.com/big-appled/kubesync/api/v1"
	"github.com/big-appled/kubesync/utils/predicatebase"
)

type CheckFunc func() (time.Duration, error)

func EnsureFinalizers(ctx context.Context, obj client.Object, finalizerName string, fn CheckFunc, cl client.Client, logger logr.Logger, reconcileForUpdate ...bool) (*ctrl.Result, error) {
	// if obj is not terminating, add finalizer if necessary
	if obj.GetDeletionTimestamp().IsZero() {
		if !sets.NewString(obj.GetFinalizers()...).Has(finalizerName) {
			logger.Info("adding finalizer", "name", finalizerName)
			controllerutil.AddFinalizer(obj, finalizerName)
			if err := cl.Update(ctx, obj); err != nil {
				return nil, err
			}
			if len(reconcileForUpdate) > 0 && reconcileForUpdate[0] {
				return &ctrl.Result{RequeueAfter: 50 * time.Millisecond}, nil
			}
		}

		return nil, nil
	}

	// no-op if no finalizer is found
	if !sets.NewString(obj.GetFinalizers()...).Has(finalizerName) {
		return &ctrl.Result{}, nil
	}

	requeue, err := fn()
	if err != nil {
		logger.Error(err, "Validation failed in finalizer")
		return nil, err
	} else if requeue > 0 {
		logger.Info(fmt.Sprintf("Re-check in finalizer after %s", requeue))
		return &ctrl.Result{RequeueAfter: requeue}, err
	}

	logger.Info("Removing finalizer", "name", finalizerName)
	controllerutil.RemoveFinalizer(obj, finalizerName)
	if err := cl.Update(ctx, obj); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return nil, err
	}

	return &ctrl.Result{}, nil
}

type KubeSyncSpecPredicate struct {
	predicatebase.IgnoreAllPredicate
}

// Create responds the Create events on server startup, a real Create event will be ignored.
func (p KubeSyncSpecPredicate) Create(e event.CreateEvent) bool {
	return true
}

// Update returns true when status is changed.
func (p KubeSyncSpecPredicate) Update(e event.UpdateEvent) bool {
	new, ok := e.ObjectNew.(*jibutechcomv1.KubeSync)
	if !ok {
		return false
	}
	// check if deletion
	if !new.GetDeletionTimestamp().IsZero() {
		return true
	}

	old, ok := e.ObjectOld.(*jibutechcomv1.KubeSync)
	if !ok {
		return false
	}

	if !reflect.DeepEqual(new.Spec, old.Spec) {
		return true
	}

	return false
}

func (p KubeSyncSpecPredicate) Delete(e event.DeleteEvent) bool {
	return true
}
