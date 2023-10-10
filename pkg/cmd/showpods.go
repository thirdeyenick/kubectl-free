package cmd

import (
	"context"
	"time"

	"github.com/makocchi-git/kubectl-free/pkg/util"
	"golang.org/x/exp/slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	metricsapiv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type formattedResource struct {
	*resource.Quantity
	formatted string
}

type podInfo struct {
	nodeName                 string
	podNamespace             string
	podName                  string
	podAge                   string
	podIP                    string
	podStatus                string
	containerName            string
	containerCPUUsed         *formattedResource
	containerCPURequested    string
	containerCPULimit        string
	containerMemoryUsed      *formattedResource
	containerMemoryRequested string
	containerMemoryLimit     string
	containerImage           string
}

func (o *FreeOptions) sortEntries(items []podInfo) []podInfo {
	switch o.sortByResource {
	case memorySortResource:
		slices.SortFunc(items, func(a, b podInfo) bool {
			if a.containerMemoryUsed == nil && b.containerMemoryUsed == nil {
				return false
			}
			if a.containerMemoryUsed != nil && b.containerMemoryUsed == nil {
				return false
			}
			if a.containerMemoryUsed == nil && b.containerMemoryUsed != nil {
				return true
			}
			return a.containerMemoryUsed.Quantity.Cmp(*b.containerMemoryUsed.Quantity) < 0
		})
	case cpuSortResource:
		slices.SortFunc(items, func(a, b podInfo) bool {
			if a.containerCPUUsed == nil && b.containerCPUUsed == nil {
				return false
			}
			if a.containerCPUUsed != nil && b.containerCPUUsed == nil {
				return false
			}
			if a.containerCPUUsed == nil && b.containerCPUUsed != nil {
				return true
			}
			return a.containerCPUUsed.Quantity.Cmp(*b.containerCPUUsed.Quantity) < 0
		})
	}
	return items
}

func (o *FreeOptions) podInfoToRow(info podInfo) []string {
	var result []string
	if o.compactView {
		result = []string{
			info.nodeName,
			info.podNamespace,
			info.podName,
			info.podStatus,
			info.containerName,
		}
	} else {
		result = []string{
			info.nodeName,
			info.podNamespace,
			info.podName,
			info.podAge,
			info.podIP,
			info.podStatus,
			info.containerName,
		}
	}

	if !o.noMetrics && info.containerCPUUsed != nil {
		result = append(result, info.containerCPUUsed.formatted)
	}
	if !o.compactView {
		result = append(result, info.containerCPURequested, info.containerCPULimit)
	}
	if !o.noMetrics && info.containerMemoryUsed != nil {
		result = append(result, info.containerMemoryUsed.formatted)
	}
	if !o.compactView {
		result = append(result, info.containerMemoryRequested, info.containerCPULimit)
	}
	if o.listContainerImage {
		result = append(result, info.containerImage)
	}
	return result
}

func (o *FreeOptions) showPodsOnNode(ctx context.Context, nodes []v1.Node) error {

	// set table header
	if !o.noHeaders {
		o.table.Header = o.listTableHeaders
	}

	// get pod metrics
	var podMetrics *metricsapiv1beta1.PodMetricsList
	if !o.noMetrics && o.metricsPodClient != nil {
		podMetrics, _ = o.metricsPodClient.List(ctx, metav1.ListOptions{})
	}

	// node loop
	for _, node := range nodes {

		// node name
		nodeName := node.ObjectMeta.Name

		// get pods on node
		pods, perr := util.GetPods(ctx, o.podClient, nodeName)
		if perr != nil {
			return perr
		}
		nodePods := []podInfo{}

		// node loop
		for _, pod := range pods.Items {
			// pod information
			podName := pod.ObjectMeta.Name
			podNamespace := pod.ObjectMeta.Namespace
			podIP := pod.Status.PodIP
			podStatus := util.GetPodStatus(string(pod.Status.Phase), o.nocolor, o.emojiStatus)
			podCreationTime := pod.ObjectMeta.CreationTimestamp.UTC()
			podCreationTimeDiff := time.Since(podCreationTime)
			podAge := "<unknown>"
			if !podCreationTime.IsZero() {
				podAge = duration.HumanDuration(podCreationTimeDiff)
			}
			// container loop
			for _, container := range pod.Spec.Containers {
				containerName := container.Name
				containerImage := container.Image
				cCpuRequested := container.Resources.Requests.Cpu().MilliValue()
				cCpuLimit := container.Resources.Limits.Cpu().MilliValue()
				cMemRequested := container.Resources.Requests.Memory().Value()
				cMemLimit := container.Resources.Limits.Memory().Value()

				row := podInfo{
					nodeName:      nodeName,      // node name
					podNamespace:  podNamespace,  // namespace
					podName:       podName,       // pod name
					podAge:        podAge,        // pod age
					podIP:         podIP,         // pod ip
					podStatus:     podStatus,     // pod status
					containerName: containerName, // container name
				}

				if !o.noMetrics && podMetrics != nil {
					cpuUsed, memoryUsed := util.GetContainerMetrics(podMetrics, podName, containerName)
					if cpuUsed != nil {
						row.containerCPUUsed = &formattedResource{
							Quantity:  cpuUsed,
							formatted: o.toMilliUnitOrDash(cpuUsed.MilliValue()),
						}
					}
					if memoryUsed != nil {
						row.containerMemoryUsed = &formattedResource{
							Quantity:  memoryUsed,
							formatted: o.toUnitOrDash(memoryUsed.Value()),
						}
					}
				}

				// skip if the requested/limit resources are not set
				if !o.listAll {
					if cCpuRequested == 0 && cCpuLimit == 0 && cMemRequested == 0 && cMemLimit == 0 {
						continue
					}
				}

				row.containerCPURequested = o.toMilliUnitOrDash(cCpuRequested)
				row.containerCPULimit = o.toMilliUnitOrDash(cCpuLimit)
				row.containerMemoryRequested = o.toUnitOrDash(cMemRequested)
				row.containerMemoryLimit = o.toUnitOrDash(cMemLimit)

				if o.listContainerImage {
					row.containerImage = containerImage
				}
				nodePods = append(nodePods, row)
			}
		}

		if !o.noMetrics {
			nodePods = o.sortEntries(nodePods)
		}
		for _, containerOfPod := range nodePods {
			o.table.AddRow(o.podInfoToRow(containerOfPod))

		}
	}
	o.table.Print()

	return nil
}
