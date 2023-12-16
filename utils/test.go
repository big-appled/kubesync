package main

import (
	"os"

	"github.com/big-appled/kubesync/syncer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
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
	dstRestConfig := config
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

	kubesyncer := syncer.NewSyncHandler(srcRestConfig, dstRestConfig, gvr, ns, nsMap, wait.NeverStop)

	err = kubesyncer.Sync()
	if err != nil {
		panic(err.Error())
	}

	// wait for the event
	<-eventChan

	// exit
	// close(eventChan)
}
