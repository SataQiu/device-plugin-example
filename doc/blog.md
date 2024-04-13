## 什么是 CDI
CDI 全称 Container Device Interface，是一种 Spec 接口规范，用于 Container Runtime 支持挂载第三方设备，如 GPU、FPGA 等。它引入了「设备作为资源」的抽象概念，设备可以由一个完全限定的名称唯一指定，该名称由设备商 ID，设备类别与一个设备类别下的一个唯一名称组成，格式如下：
```bash
vendor.com/class=unique_name
```
以 Nvidia GPU 为例，其 CDI 设备名称可以被定义为如下样式：
```bash
nvidia.com/gpu=all
```
有关 CDI 的更多定义参考官网：[https://github.com/cncf-tags/container-device-interface](https://github.com/cncf-tags/container-device-interface)

## CDI 解决了什么问题
以 Nvidia GPU 为例，要在容器中使用 GPU，需要在启动 Container 时通过参数挂载设备和依赖的库文件系统，才能确保容器内的应用程序能够正确使用 GPU 设备。

假如是通过手动运行容器，这固然可以做到，我们可以通过类似如下命令实现：
```bash
docker run -d --name example --device xxx:xxx -v xxx:xxx centos:7 
```
但是，在实际场景中会更复杂，比如考虑在 Kubernetes 环境，如何知道设备依赖的库文件系统路径，并优雅地挂载到容器上呢？不同类型的设备依赖各不相同，怎么合理的组织这种依赖关系呢？

既然不同的设备有不同的依赖，可以把需要挂载的设备和库文件系统按照某种格式写入某一个文件中，然后在容器创建时，用户指定这个容器需要根据刚刚定义的文件中的内容实现设备挂载。CDI 正是定义的这样一个规范 Spec 文件，告诉容器运行时如何挂载设备及其依赖库。当然，除了定义了库挂载路径，CDI 还支持诸如 Hook 等操作对 Container 进行修改和配置，可以参考 [https://github.com/cncf-tags/container-device-interface/blob/main/specs-go/config.go](https://github.com/cncf-tags/container-device-interface/blob/main/specs-go/config.go)
##  CDI 与 DevicePlugin 的关系
DevicePlugin 是 Kubernetes 提供的一种设备插件框架，允许通过插件的形式将设备资源注册到 Kubelet，从而能够供容器分配使用。
DevicePlugin 通过 grpc 形式的接口提供服务，具体定义可参考：[https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1/api.proto](https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1/api.proto)
DevicePlugin 一般以 DaemonSet 方式运行，通过 grpc 协议基于 `/var/lib/kubelet/device-plugins/kubelet.sock` 端点与 kubelet 通信，注册可用的硬件资源，该类资源会被 kubelet 识别并暴露到 Node 对象的 Status 字段，例如：
```bash
  status:
    allocatable:
      example.com/mydevice: 8
    capacity:
      example.com/mydevice: 8
```
在容器创建阶段，可以通过资源请求对应的设备，调度器会自动选择有可用设备的节点运行 Pod，举例配置如下：
```bash
   resources:
      requests:
        example.com/mydevice: 4
      limits:
        example.com/mydevice: 4
```
kubelet 在接收到 Pod 创建请求后，通过调用 DevicePlugin grpc 接口实现对资源设备的 Allocate 申请和资源扣减（中间可能涉及到设备的初始化等动作），并调用 Container Runtime 启动容器挂载设备，此时 CDI 负责指导 Container Runtime 正确挂载设备和依赖的库，对容器进行正确的适应性调整。

综上，DevicePlugin 解决的是 Kubernetes 识别和调度分配设备的问题，CDI 解决的是 Container Runtime 创建容器时正确挂载设备的问题。二者都是通过规范接口的形式实现系统的可扩展性，兼容不同的设备类型。

## 使用 DevicePlugin + CDI 的例子
### 1. 创建 Kubernetes 测试集群
本文使用 Kind 创建测试集群（版本 1.29），参考如下命令：
```bash
$ kind version
kind v0.22.0 go1.21.7 darwin/amd64
$ kind create cluster
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.29.2) 🖼 
 ✓ Preparing nodes 📦  
 ✓ Writing configuration 📜 
 ✓ Starting control-plane 🕹️ 
 ✓ Installing CNI 🔌 
 ✓ Installing StorageClass 💾 
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
```
确认集群运行正常：
```bash
$ kubectl get node 
NAME                 STATUS   ROLES           AGE     VERSION
kind-control-plane   Ready    control-plane   7m18s   v1.29.2
```
### 2. 准备模拟设备
由于测试环境没有真实设备可用于验证，这里通过创建设备文件模拟：
```bash
$ docker exec -it kind-control-plane bash
root@kind-control-plane:/# mkdir /dev/mock
root@kind-control-plane:/# cd /dev/mock
root@kind-control-plane:/# mknod /dev/mock/device_0 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_1 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_2 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_3 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_4 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_5 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_6 c 89 1
root@kind-control-plane:/# mknod /dev/mock/device_7 c 89 1
```

> `root@kind-control-plane:/#` 表示命令是在容器内执行的


同样地，创建 so 文件模拟设备依赖的库文件：
```bash
root@kind-control-plane:/# mkdir -p /mock/lib
root@kind-control-plane:/# cd /mock/lib
root@kind-control-plane:/# touch device_0.so device_1.so device_2.so device_3.so device_4.so device_5.so device_6.so device_7.so
```
### 3. 创建和部署 DevicePlugin

这里实现一个最基本的 DevicePlugin，golang 源码如下：
```go
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
```
以上代码已经打包在 GitHub，读者可以直接克隆项目代码使用：
```bash
$ git clone https://github.com/SataQiu/device-plugin-example.git
$ cd device-plugin-example
$ kubectl apply -f ./manifests/daemonset.yaml
```

附上 `daemonset.yaml` 的内容：
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: device-plugin-example
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: device-plugin-example
  template:
    metadata:
      labels:
        name: device-plugin-example
    spec:
      containers:
      - image: shidaqiu/device-plugin-example:v0.1
        name: device-plugin-example
        imagePullPolicy: IfNotPresent
        volumeMounts:
        - name: kubelet
          mountPath: /var/lib/kubelet
      volumes:
      - name: kubelet
        hostPath:
          path: /var/lib/kubelet
```

部署后，检查 DevicePlugin Pod 启动正常：
```bash
$ kubectl get pod -n kube-system -l name=device-plugin-example
NAME                          READY   STATUS    RESTARTS   AGE
device-plugin-example-bd497   1/1     Running   0          21h
```

查看 Node 状态，确认 `device.example.com/gpu` 设备已经注册：
```bash
$ kubectl get node kind-control-plane -oyaml
...
  allocatable:
    cpu: "8"
    device.example.com/gpu: "8"
    ephemeral-storage: 40972512Ki
    hugepages-2Mi: "0"
    memory: 10201684Ki
    pods: "110"
  capacity:
    cpu: "8"
    device.example.com/gpu: "8"
    ephemeral-storage: 40972512Ki
    hugepages-2Mi: "0"
    memory: 10201684Ki
    pods: "110"
```
### 4. 配置 Containerd CDI 规则
CDI 配置文件默认放置在 `/etc/cdi/ ` 和 `/var/run/cdi` 文件夹下，其中
- `/etc/cdi/ ` 一般存储静态配置
- `/var/run/cdi` 一般存储动态配置（例如 CDI 配置是通过 DevicePlugin 动态生成的场景）

本文的模拟设备是静态的，因此在 `/etc/cdi/ ` 下创建模拟设备的挂载规则，文件名：`device-example.yaml`
```bash
root@kind-control-plane:/# mkdir /etc/cdi
root@kind-control-plane:/# vim /etc/cdi/device-example.yaml
```
```yaml
cdiVersion: 0.5.0
kind: device.example.com/gpu
devices:
  - name: device_0
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_0
          path: /dev/mock/device_0
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_0.so
          containerPath: /mock/lib/device_0.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_1
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_1
          path: /dev/mock/device_1
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_1.so
          containerPath: /mock/lib/device_1.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_2
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_2
          path: /dev/mock/device_2
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_2.so
          containerPath: /mock/lib/device_2.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_3
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_3
          path: /dev/mock/device_3
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_3.so
          containerPath: /mock/lib/device_3.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_4
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_4
          path: /dev/mock/device_4
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_4.so
          containerPath: /mock/lib/device_4.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_5
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_5
          path: /dev/mock/device_5
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_5.so
          containerPath: /mock/lib/device_5.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_6
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_6
          path: /dev/mock/device_6
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_6.so
          containerPath: /mock/lib/device_6.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
  - name: device_7
    containerEdits:
      deviceNodes:
        - hostPath: /dev/mock/device_7
          path: /dev/mock/device_7
          type: c
          permissions: rw
      mounts:
        - hostPath: /mock/lib/device_7.so
          containerPath: /mock/lib/device_7.so
          options:
            - ro
            - nosuid
            - nodev
            - bind
```
配置 Containerd 启用 CDI 功能，编辑  `/etc/containerd/config.toml`
```bash
root@kind-control-plane:/# vim /etc/containerd/config.toml
```
在 `[plugins."io.containerd.grpc.v1.cri"]` table 下添加如下配置：

```toml
  enable_cdi = true
  cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
```
重启 Containerd 服务：
```bash
root@kind-control-plane:/# systemctl restart containerd
```

### 5. 部署测试 Pod
准备如下 `example-app.yaml` 文件，申请 4 个 gpu 资源：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-app
spec:
  containers:
  - name: example-app
    image: ubuntu:22.04
    command: ["sleep"]
    args: ["infinity"]
    resources:
      requests:
        device.example.com/gpu: "4"
      limits:
        device.example.com/gpu: "4"
```

部署 Pod 到集群：
```bash
$ kubectl apply -f example-app.yaml
```

检查 Pod 内的设备挂载状态：
```bash
$ kubectl exec -it example-app bash
root@example-app:/# ls /dev/mock
device_4  device_5  device_6  device_7
root@example-app:/# ls /mock/lib
device_4.so  device_5.so  device_6.so  device_7.so
```
可以看到申请的 4 个设备被挂载到了 /dev/mock/device_x，相应的库文件被挂载到 /mock/lib/device_x.so，这正是在 CDI 配置中定义的路径。

查看 Node 的资源使用状态：
```bash
$ kubectl describe node kind-control-plane
...
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource                Requests    Limits
  --------                --------    ------
  cpu                     950m (11%)  100m (1%)
  memory                  290Mi (2%)  390Mi (3%)
  ephemeral-storage       0 (0%)      0 (0%)
  hugepages-2Mi           0 (0%)      0 (0%)
  device.example.com/gpu  4           4
```
看到 `device.example.com/gpu` 已被使用了 4 个。

再创建一个 Pod 使用 3 个 gpu，`example-app-2.yaml`：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-app-2
spec:
  containers:
  - name: example-app-2
    image: ubuntu:22.04
    command: ["sleep"]
    args: ["infinity"]
    resources:
      requests:
        device.example.com/gpu: "3"
      limits:
        device.example.com/gpu: "3"
```

观察 Pod 挂载的设备：
```bash
$ kubectl exec -it example-app-2 bash
root@example-app-2:/# ls /dev/mock/
device_0  device_1  device_2
root@example-app-2:/# ls /mock/lib/
device_0.so  device_1.so  device_2.so
```

再创建一个 Pod 使用 2 个 gpu，`example-app-3.yaml`：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-app-3
spec:
  containers:
  - name: example-app-3
    image: ubuntu:22.04
    command: ["sleep"]
    args: ["infinity"]
    resources:
      requests:
        device.example.com/gpu: "2"
      limits:
        device.example.com/gpu: "2"
```

由于剩余 1 个 gpu 设备，不满足 2 个最低要求，Pod 会处于 Pending 状态：
```bash
$ kubectl describe pod example-app-3
...
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  82s   default-scheduler  0/1 nodes are available: 1 Insufficient device.example.com/gpu. preemption: 0/1 nodes are available: 1 No preemption victims found for incoming pod.
```

本文通过一个简单的例子演示了 CDI 的使用，希望对您有所帮助！

## 相关引用
- https://www.cnblogs.com/haiyux/p/17842489.html
- https://developer.aliyun.com/article/1180698




















