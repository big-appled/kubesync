package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Read the config file
	configData, err := os.ReadFile("C://Users/candy/.kube/config")
	if err != nil {
		panic(err.Error())
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(configData)
	if err != nil {
		panic(err.Error())
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		panic(err.Error())
	}

	srcRestConfig := config
	dstRestCondif := config
	/*
		gvr := schema.GroupVersionResource{
			Group:    "migration.yinhestor.com",
			Version:  "v1",
			Resource: "droperationrequests",
		}
		ns := "qiming-migration"
		nsMap := map[string]string{
			ns: "test1",
		}
	*/
	gvr := schema.GroupVersionResource{
		Group:    "ys.jibudata.com",
		Version:  "v1beta1",
		Resource: "users",
	}
	ns := ""
	var nsMap map[string]string

	// create a channel to receive the event
	eventChan := make(chan struct{})

	sh := NewSyncHandler(srcRestConfig, dstRestCondif, gvr, ns, nsMap, eventChan)

	err = sh.Sync()
	if err != nil {
		panic(err.Error())
	}

	// wait for the event
	<-eventChan

	// exit
	// close(eventChan)
}

type SyncHandler struct {
	srcRestConfig *rest.Config
	dstRestConfig *rest.Config
	gvr           schema.GroupVersionResource
	ns            string
	name          string
	nsMap         map[string]string
	eventChan     chan struct{}

	dstClient dynamic.NamespaceableResourceInterface
}

func NewSyncHandler(srcRestConfig, dstRestConfig *rest.Config, gvr schema.GroupVersionResource, ns string, nsMap map[string]string, eventChan chan struct{}) *SyncHandler {
	return &SyncHandler{
		srcRestConfig: srcRestConfig,
		dstRestConfig: dstRestConfig,
		gvr:           gvr,
		ns:            ns,
		nsMap:         nsMap,
		eventChan:     eventChan,
	}
}

func (s *SyncHandler) Sync() error {
	// create the dynamic client
	srcDynClient, err := dynamic.NewForConfig(s.srcRestConfig)
	if err != nil {
		return err
	}

	dstDynClient, err := dynamic.NewForConfig(s.dstRestConfig)
	if err != nil {
		return err
	}

	// get the custom resource definition
	s.dstClient = dstDynClient.Resource(s.gvr)

	//dynamicinformer.NewDynamicSharedInformerFactory(dynClient, rs,  )
	// add event handler
	informer := dynamicinformer.NewFilteredDynamicInformer(
		srcDynClient,
		s.gvr,
		s.ns,
		0,
		cache.Indexers{},
		nil,
	)

	_, err = informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err = s.createOrUpdate(obj)
			if err != nil {
				panic(err.Error())
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			err = s.createOrUpdate(newObj)
			if err != nil {
				panic(err.Error())
			}
		},
		DeleteFunc: func(obj interface{}) {
			err = s.delete(obj)
			if err != nil {
				panic(err.Error())
			}
		},
	})
	if err != nil {
		panic(err.Error())
	}

	// start the informers
	go informer.Informer().Run(wait.NeverStop)

	// stop the informers
	//informer.Informer().
	return nil
}

func (s *SyncHandler) create(obj interface{}) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("cannot convert to unstructured, obj: %v", obj)
	}
	fmt.Println("resource added:", u.GroupVersionKind(), "name:", u.GetName(), "namespace:", u.GetNamespace())
	ns := findNamespace(u.GetNamespace(), s.nsMap)
	u.SetNamespace(ns)
	u.SetResourceVersion("")
	u.SetOwnerReferences(nil)

	_, err := s.dstClient.Namespace(ns).Create(context.TODO(), u, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		fmt.Println("create error", err.Error())
		return err
	}

	return nil
}

func (s *SyncHandler) createOrUpdate(obj interface{}) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("cannot convert to unstructured, obj: %v", obj)
	}
	if s.name != "" && u.GetName() != s.name {
		return nil
	}
	ns := findNamespace(u.GetNamespace(), s.nsMap)

	uu, err := s.dstClient.Namespace(ns).Get(context.TODO(), u.GetName(), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if uu == nil {
		err = s.create(obj)
	} else {
		err = s.update(uu, obj)
	}
	return err
}

func (s *SyncHandler) update(objOld, objNew interface{}) error {
	var err error
	u, ok := objNew.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("cannot convert to unstructured, obj: %v", objNew)
	}
	uu, ok := objOld.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("cannot convert to unstructured, obj: %v", objNew)
	}
	fmt.Println("resource updated:", u.GroupVersionKind(), "name:", u.GetName(), "namespace:", u.GetNamespace())
	ns := findNamespace(u.GetNamespace(), s.nsMap)
	u.SetNamespace(ns)
	uu.SetNamespace(ns)
	u.SetResourceVersion(uu.GetResourceVersion())
	u.SetUID(uu.GetUID())
	u.SetOwnerReferences(nil)
	if u.GetDeletionTimestamp() != nil {
		return nil
	}

	//data, err := client.MergeFrom(uu).Data(u)
	data, err := generatePatch(uu, u)
	if err != nil {
		return err
	}
	if data == nil {
		fmt.Println("no changes")
		return nil
	}
	_, err = s.dstClient.Namespace(ns).Patch(context.TODO(), u.GetName(), types.MergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		fmt.Println("update error", err.Error())
		return err
	}
	return nil
}

func (s *SyncHandler) delete(obj interface{}) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("cannot convert to unstructured, obj: %v", obj)
	}
	if s.name != "" && u.GetName() != s.name {
		return nil
	}
	fmt.Println("resource deleted:", u.GroupVersionKind(), "name:", u.GetName(), "namespace:", u.GetNamespace())
	ns := findNamespace(u.GetNamespace(), s.nsMap)
	u.SetNamespace(ns)

	err := s.dstClient.Namespace(ns).Delete(context.TODO(), u.GetName(), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func findNamespace(old string, m map[string]string) string {
	for k, v := range m {
		if k == old && v != "" {
			return v
		}
	}
	return old
}

// generatePatch will calculate a JSON merge patch for an object's desired state.
// If the passed in objects are already equal, nil is returned.
func generatePatch(fromCluster, desired *unstructured.Unstructured) ([]byte, error) {
	// If the objects are already equal, there's no need to generate a patch.
	if equality.Semantic.DeepEqual(fromCluster, desired) {
		return nil, nil
	}

	desiredBytes, err := json.Marshal(desired.Object)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal desired object, error: %v", err)
	}

	fromClusterBytes, err := json.Marshal(fromCluster.Object)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal in-cluster object, error: %v", err)
	}

	patchBytes, err := jsonpatch.CreateMergePatch(fromClusterBytes, desiredBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to create merge patch, error: %v", err)
	}

	return patchBytes, nil
}
