/*
Copyright 2018 The Kubernetes authors.

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
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	machineactuator "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/aws/actuators/machine"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/apis/awsproviderconfig/v1alpha1"
	"github.com/golang/glog"
	log "github.com/sirupsen/logrus"
	awsclient "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/aws/client"
	"os"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, func(m manager.Manager) error {
		//// the following line exists to make glog happy, for more information, see: https://github.com/kubernetes/kubernetes/issues/17162
		//flag.CommandLine.Parse([]string{})
		//pflag.Parse()
		//
		//logs.InitLogs()
		//defer logs.FlushLogs()

		config := m.GetConfig()
		//client, err := m.GetClient()
		//if err != nil {
		//	glog.Fatalf("Could not create client for talking to the apiserver: %v", err)
		//}

		client, err := clientset.NewForConfig(config)
		if err != nil {
			glog.Fatalf("Could not create client for talking to the apiserver: %v", err)
		}


		kubeClient, err := kubernetes.NewForConfig(config)
		if err != nil {
			glog.Fatalf("Could not create kubernetes client to talk to the apiserver: %v", err)
		}

		log.SetOutput(os.Stdout)
		if lvl, err := log.ParseLevel("debug"); err != nil {
			log.Panic(err)
		} else {
			log.SetLevel(lvl)
		}

		logger := log.WithField("controller", "awsMachine")

		codec, err := v1alpha1.NewCodec()
		if err != nil {
			glog.Fatal(err)
		}

		params := machineactuator.ActuatorParams{
			ClusterClient:    client,
			KubeClient:       kubeClient,
			AwsClientBuilder: awsclient.NewClient,
			Logger:           logger,
			Codec: 			  codec,
		}

		actuator, err := machineactuator.NewActuator(params)
		if err != nil {
			glog.Fatalf("Could not create AWS machine actuator: %v", err)
		}
		return machine.AddWithActuator(m, actuator)
	})
}
