package machine

import (
	"k8s.io/apimachinery/pkg/api/equality"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	providerconfigv1 "sigs.k8s.io/cluster-api-provider-aws/pkg/apis/awsproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"github.com/aws/aws-sdk-go/service/ec2"

	log "github.com/sirupsen/logrus"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"github.com/ghodss/yaml"
)

type codec interface {
	DecodeFromProviderConfig(clusterv1.ProviderConfig, runtime.Object) error
	DecodeProviderStatus(*runtime.RawExtension, runtime.Object) error
	EncodeProviderStatus(runtime.Object) (*runtime.RawExtension, error)
}

// updateStatus calculates the new machine status, checks if anything has changed, and updates if so.
func (a *Actuator) updateStatus(machine *clusterv1.Machine, instance *ec2.Instance, mLog log.FieldLogger) error {
	mLog.Debug("updating status")

	// Starting with a fresh status as we assume full control of it here.
	awsStatus, err := ProviderStatusFromMachine(a.codec, machine)
	if err != nil {
		mLog.Errorf("error decoding machine provider status: %v", err)
		return err
	}
	// Save this, we need to check if it changed later.
	networkAddresses := []corev1.NodeAddress{}

	// Instance may have existed but been deleted outside our control, clear it's status if so:
	if instance == nil {
		awsStatus.InstanceID = nil
		awsStatus.InstanceState = nil
	} else {
		awsStatus.InstanceID = instance.InstanceId
		awsStatus.InstanceState = instance.State.Name
		if instance.PublicIpAddress != nil {
			networkAddresses = append(networkAddresses, corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: *instance.PublicIpAddress,
			})
		}
		if instance.PrivateIpAddress != nil {
			networkAddresses = append(networkAddresses, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: *instance.PrivateIpAddress,
			})
		}
		if instance.PublicDnsName != nil {
			networkAddresses = append(networkAddresses, corev1.NodeAddress{
				Type:    corev1.NodeExternalDNS,
				Address: *instance.PublicDnsName,
			})
		}
		if instance.PrivateDnsName != nil {
			networkAddresses = append(networkAddresses, corev1.NodeAddress{
				Type:    corev1.NodeInternalDNS,
				Address: *instance.PrivateDnsName,
			})
		}
	}
	mLog.Debug("finished calculating AWS status")
	awsStatus.Conditions = SetAWSMachineProviderCondition(awsStatus.Conditions, providerconfigv1.MachineCreation, corev1.ConditionTrue, MachineCreationSucceeded, "machine successfully created", UpdateConditionIfReasonOrMessageChange)

	// TODO(jchaloup): do we really need to update tis?
	// origInstanceID := awsStatus.InstanceID
	// if !StringPtrsEqual(origInstanceID, awsStatus.InstanceID) {
	// 	mLog.Debug("AWS instance ID changed, clearing LastELBSync to trigger adding to ELBs")
	// 	awsStatus.LastELBSync = nil
	// }
	err = a.updateMachineStatus(machine, awsStatus, mLog, networkAddresses)
	if err != nil {
		return err
	}

	// If machine state is still pending, we will return an error to keep the controllers
	// attempting to update status until it hits a more permanent state. This will ensure
	// we get a public IP populated more quickly.
	if awsStatus.InstanceState != nil && *awsStatus.InstanceState == ec2.InstanceStateNamePending {
		mLog.Infof("instance state still pending, returning an error to requeue")
		return &clustererror.RequeueAfterError{RequeueAfter: requeueAfterSeconds * time.Second}
	}
	return nil
}

func (a *Actuator) updateMachineStatus(machine *clusterv1.Machine, awsStatus *providerconfigv1.AWSMachineProviderStatus, mLog log.FieldLogger, networkAddresses []corev1.NodeAddress) error {
	awsStatusRaw, err := EncodeProviderStatus(a.codec, awsStatus)
	if err != nil {
		mLog.Errorf("error encoding AWS provider status: %v", err)
		return err
	}

	machineCopy := machine.DeepCopy()
	if machineCopy.Status.ProviderStatus == nil {
		machineCopy.Status.ProviderStatus = &runtime.RawExtension{}
	}
	machineCopy.Status.ProviderStatus = awsStatusRaw
	if networkAddresses != nil {
		machineCopy.Status.Addresses = networkAddresses
	}

	if !equality.Semantic.DeepEqual(machine.Status, machineCopy.Status) {
		mLog.Info("machine status has changed, updating")
		time := metav1.Now()
		machineCopy.Status.LastUpdated = &time

		_, err := a.clusterClient.ClusterV1alpha1().Machines(machineCopy.Namespace).Update(machineCopy)
		if err != nil {
			mLog.Errorf("error updating machine status: %v", err)
			return err
		}
	} else {
		mLog.Debug("status unchanged")
	}

	return nil
}

// EncodeAWSMachineProviderStatus encodes the machine status into RawExtension
func EncodeProviderStatus(codec codec, awsStatus *providerconfigv1.AWSMachineProviderStatus) (*runtime.RawExtension, error) {
	return codec.EncodeProviderStatus(awsStatus)
}

func ProviderConfigFromMachine(codec codec, machine *clusterv1.Machine) (*providerconfigv1.AWSMachineProviderConfig, error) {
	//machineProviderCfg := &providerconfigv1.AWSMachineProviderConfig{}
	//err := codec.DecodeFromProviderConfig(machine.Spec.ProviderConfig, machineProviderCfg)
	//return machineProviderCfg, err
	var config providerconfigv1.AWSMachineProviderConfig
	if err := yaml.Unmarshal(machine.Spec.ProviderConfig.Value.Raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// AWSMachineProviderStatusFromClusterAPIMachine gets the machine provider status from the specified machine.
func ProviderStatusFromMachine(codec codec, m *clusterv1.Machine) (*providerconfigv1.AWSMachineProviderStatus, error) {
	status := &providerconfigv1.AWSMachineProviderStatus{}
	err := codec.DecodeProviderStatus(m.Status.ProviderStatus, status)
	return status, err
}
