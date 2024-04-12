package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type PluginLister struct {
	ResUpdateChan chan dpm.PluginNameList
}

var ResourceNamespace = "device.example.com"
var PluginName = "gpu"

func (p *PluginLister) GetResourceNamespace() string {
	return ResourceNamespace
}

func (p *PluginLister) Discover(pluginListCh chan dpm.PluginNameList) {
	pluginListCh <- dpm.PluginNameList{PluginName}
}

func (p *PluginLister) NewPlugin(name string) dpm.PluginInterface {
	return &Plugin{}
}

type Plugin struct {
}

func (p *Plugin) GetDevicePluginOptions(ctx context.Context, e *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	options := &pluginapi.DevicePluginOptions{
		PreStartRequired: true,
	}
	return options, nil
}

func (p *Plugin) PreStartContainer(ctx context.Context, r *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (p *Plugin) GetPreferredAllocation(ctx context.Context, r *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

func (p *Plugin) ListAndWatch(e *pluginapi.Empty, r pluginapi.DevicePlugin_ListAndWatchServer) error {
	devices := []*pluginapi.Device{}
	for i := 0; i < 8; i++ {
		devices = append(devices, &pluginapi.Device{
			// 这里注意要和 device 名称保持一致
			ID:     fmt.Sprintf("device_%d", i),
			Health: pluginapi.Healthy,
		})
	}
	for {
		// 每分钟注册一次
		fmt.Printf("register devices at %v \n", time.Now())
		r.Send(&pluginapi.ListAndWatchResponse{
			Devices: devices,
		})
		time.Sleep(time.Second * 60)
	}
}

func (p *Plugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	responses := &pluginapi.AllocateResponse{}
	for _, req := range r.ContainerRequests {
		// DevicePlugin 已经支持 CDI Device
		cdidevices := []*pluginapi.CDIDevice{}
		for _, id := range req.DevicesIDs {
			cdidevices = append(cdidevices, &pluginapi.CDIDevice{
				Name: fmt.Sprintf("%s/%s=%s", ResourceNamespace, PluginName, id),
			})
		}
		responses.ContainerResponses = append(responses.ContainerResponses, &pluginapi.ContainerAllocateResponse{
			CDIDevices: cdidevices,
		})
	}
	return responses, nil
}

func main() {
	m := dpm.NewManager(&PluginLister{})
	m.Run()
}
