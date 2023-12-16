/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	jibutechcomv1 "github.com/big-appled/kubesync/api/v1"
	"github.com/big-appled/kubesync/setting"
	"github.com/big-appled/kubesync/syncer"
)

type KubeSyncRequest struct {
	Ks *jibutechcomv1.KubeSync
}

type FuncSignal struct {
	CancelCh chan struct{}
	DoneCh   chan struct{}
}

// KubeSyncReconciler reconciles a KubeSync object
type KubeSyncReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Log       logr.Logger
	Mgr       manager.Manager
	chanInput chan *KubeSyncRequest
	chanDone  chan struct{}
	// Map of kubesync name to cancel chans
	CancelMap map[string]*FuncSignal
	mutex     sync.RWMutex
}

//+kubebuilder:rbac:groups=jibutech.com.jibutech.com,resources=kubesyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=jibutech.com.jibutech.com,resources=kubesyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=jibutech.com.jibutech.com,resources=kubesyncs/finalizers,verbs=update

func (r *KubeSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrlResult ctrl.Result, err error) {
	r.Log = log.FromContext(ctx)

	ks := &jibutechcomv1.KubeSync{}
	err = r.Client.Get(ctx, req.NamespacedName, ks)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	res, err := r.ensureFinalizers(ctx, ks)
	if err != nil {
		r.Log.Error(err, "failed to ensure finalizer")
		return ctrl.Result{}, err
	} else if res != nil {
		return *res, nil
	}

	r.Log.Info("Reconciling KubeSync")

	defer func() {
		if err != nil {
			ks.SetNotReady()
		}
		err = r.Client.Status().Update(ctx, ks)
		if err != nil {
			r.Log.Error(err, "failed to update status")
		}
	}()

	ks.SetReady()

	r.chanInput <- &KubeSyncRequest{
		Ks: ks,
	}

	if ks.Spec.Pause {
		ks.SetPaused()
	}
	return ctrl.Result{}, nil
}

func (r *KubeSyncReconciler) serve(input <-chan *KubeSyncRequest, done <-chan struct{}, logger logr.Logger) error {
	go func() {
		for {
			select {
			case newRequest := <-input:
				logger.Info("got new request", "request", newRequest.Ks.Name)
				go func() {
					//nolint: errcheck
					err := r.handleFunction(newRequest, logger)
					if err != nil {
						logger.Error(err, "failed to handle function", "kubesync", newRequest.Ks.Name)
					}
				}()
			case <-done:
				logger.Info("shutdown server")
				return
			}
		}
	}()
	return nil
}

func (r *KubeSyncReconciler) handleFunction(ctx *KubeSyncRequest, logger logr.Logger) error {
	ks := ctx.Ks

	// Start operation
	if ks.Spec.Pause {
		r.stop(ks)
		return nil
	}

	if r.CancelMap != nil && r.CancelMap[ks.Name] != nil {
		// already running, return
		//TODO update
		logger.Info("already running", "ks", ks.Name)
		return nil
	}

	// new kubesync
	cancelCh := make(chan struct{})
	doneCh := make(chan struct{})
	defer close(doneCh)

	// Remember cancel chan
	sig := &FuncSignal{
		CancelCh: cancelCh,
		DoneCh:   doneCh,
	}
	r.mutex.Lock()
	if r.CancelMap == nil {
		r.CancelMap = make(map[string]*FuncSignal)
	}
	r.CancelMap[ks.Name] = sig
	r.mutex.Unlock()
	r.Log.Info("start sync", "ks", ks.Name)
	err := r.run(ks, sig)
	if err != nil {
		logger.Error(err, "run function error", "ks", ks.Name)
		return err
	}

	<-sig.DoneCh
	// completed
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.CancelMap, ks.Name)
	logger.Info("handle function done", "error", err)
	return nil
}

func (r *KubeSyncReconciler) run(ks *jibutechcomv1.KubeSync, sig *FuncSignal) error {
	var err error
	// Read the config file
	srcRestConfig, err := getRestConfig(ks.Namespace, ks.Spec.SrcClusterSecret, r.Client)
	if err != nil {
		return err
	}
	dstRestConfig, err := getRestConfig(ks.Namespace, ks.Spec.DstClusterSecret, r.Client)
	if err != nil {
		return err
	}

	internalCancelCh := make([]chan struct{}, 0)
	for _, resource := range ks.Spec.SyncResources {
		gvr := schema.GroupVersionResource{
			Group:    resource.GVR.Group,
			Version:  resource.GVR.Version,
			Resource: resource.GVR.Resource,
		}
		for _, ns := range resource.Namespaces {
			ch := make(chan struct{})
			internalCancelCh = append(internalCancelCh, ch)
			kubesyncer := syncer.NewSyncHandler(srcRestConfig, dstRestConfig, gvr, ns, ks.Spec.NsMap, ch)
			err = kubesyncer.Sync()
			if err != nil {
				r.Log.Error(err, "failed to sync", "gvr", gvr, "namespace", ns)
				return err
			}
		}
	}

	<-sig.CancelCh
	r.Log.Info("stopping syncer")
	for _, ch := range internalCancelCh {
		close(ch)
	}
	sig.DoneCh <- struct{}{}
	return err
}

func (r *KubeSyncReconciler) stop(ks *jibutechcomv1.KubeSync) {
	r.mutex.Lock()
	sig, ok := r.CancelMap[ks.Name]
	if !ok {
		r.Log.Info("cancel chan not found", "ks", ks.Name)
		return
	}

	r.Log.Info("send cancel signal to kubesync", "ks", ks.Name)
	sig.CancelCh <- struct{}{}

	// delete first, otherwise the lock may hold a long time,
	// if start operation just exits normally, it will try to hold
	// the lock below, which will cause deadlock
	delete(r.CancelMap, ks.Name)
	r.mutex.Unlock()

	// Wait for start signal
	r.Log.Info("wait for done signal from main function", "ks", ks.Name)
	<-sig.DoneCh
	r.Log.Info("main function done", "ks", ks.Name)
}

func getRestConfig(ns, name string, cl client.Client) (*rest.Config, error) {
	var err error
	s := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, s)
	if err != nil {
		return nil, err
	}
	configData, found := s.Data["kubeconfig"]
	if !found {
		return nil, fmt.Errorf("kubeconfig not found in secret %s", name)
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(configData)
	if err != nil {
		return nil, err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	logger := ctrl.Log.WithName("kubesync-controller")
	r.Mgr = mgr

	r.chanInput = make(chan *KubeSyncRequest, 10)
	r.chanDone = make(chan struct{}, 1)
	// nolint: errcheck
	go r.serve(r.chanInput, r.chanDone, logger)
	return ctrl.NewControllerManagedBy(mgr).
		For(&jibutechcomv1.KubeSync{}, builder.WithPredicates(KubeSyncSpecPredicate{})).
		Complete(r)
}

func (r *KubeSyncReconciler) ensureFinalizers(ctx context.Context, instance *jibutechcomv1.KubeSync) (*ctrl.Result, error) {
	logger := r.Log

	return EnsureFinalizers(ctx, instance, setting.KubeSyncFinalizer,
		func() (time.Duration, error) {
			logger.Info("kube sync finalizer start...")
			r.stop(instance)
			return 0, nil
		},
		r.Client, logger, true,
	)
}
