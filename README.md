# kubectl free

This is a fork of https://github.com/makocchi-git/kubectl-free with just a few
minor changes (like sorting pods of nodes by mem/cpu usage).

Print pod resources/limits usage on Kubernetes node(s) like a linux "free" command.  

```shell
$ kubectl free
NAME    STATUS   CPU/use   CPU/req   CPU/lim   CPU/alloc   CPU/use%   CPU/req%   CPU/lim%   MEM/use    MEM/req    MEM/lim    MEM/alloc   MEM/use%   MEM/req%   MEM/lim%
node1   Ready    58m       704m      304m      3600m       1%         19%        8%         2144333K   807403K    375390K    5943857K    36%        13%        6%
node2   Ready    235m      350m      2100m     3600m       6%         9%         58%        2061467K   260046K    1304428K   5943857K    34%        4%         21%
node3   Ready    222m      2030m     12900m    3600m       6%         56%        358%       2935312K   3736783K   8347396K   5943865K    49%        62%        140%
```

And list containers of pod on Kubernetes node(s).

```shell
$ kubectl free --list node1
NODE NAME  NAMESPACE     POD NAME                               POD AGE   POD IP       POD STATUS   CONTAINER            CPU/use   CPU/req   CPU/lim   MEM/use   MEM/req   MEM/lim
node1      default       nginx-7cdbd8cdc9-q2bbg                 3d22h     10.112.2.43  Running      nginx                2m        100m      2         27455K    134217K   1073741K
node1      kube-system   coredns-69dc677c56-chfcm               9d        10.112.3.2   Running      coredns              3m        100m      -         17420K    73400K    178257K
node1      kube-system   kube-flannel-ds-amd64-4b4s2            9d        10.1.2.3     Running      kube-flannel         4m        100m      100m      13877K    52428K    52428K
node1      kube-system   kube-state-metrics-69bcc79474-wvmmk    9d        10.112.3.3   Running      kube-state-metrics   11m       104m      104m      33382K    113246K   113246K
node1      kube-system   kube-state-metrics-69bcc79474-wvmmk    9d        10.112.3.3   Running      addon-resizer        1m        100m      100m      8511K     31457K    31457K
...
```

## Install

`kubectl-free` binary is available at [release page](https://github.com/thirdeyenick/kubectl-free/releases) or you can make binary.

```shell
$ make
$ mv _output/kubectl-free /usr/local/bin/.
```

```
# Happy free time!
$ kubectl free
```

## Usage

```shell
# Show pod resource usage of Kubernetes nodes (include all namespaces by
# default). It also includes containers with no resources/limits.
kubectl free

# Show pod resource usage of Kubernetes nodes with number of pods and containers.
kubectl free --pod

# Using label selector for nodes to include.
kubectl free -l key=value

# Print raw(bytes) usage.
kubectl free --bytes --without-unit

# Using binary prefix unit (GiB, MiB, etc). By default it uses MiB.
kubectl free -g -B

# List resources of containers in pods on nodes.
kubectl free --list

# List resources of containers in pods on specific nodes.
kubectl free --list <node names...>

# List resources of containers in pods on nodes with image information.
kubectl free --list --list-image
```
## Tests

Tests are currently failing as more adoptions are needed.

## License

This software is released under the MIT License.
