package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
