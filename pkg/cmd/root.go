package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/thirdeyenick/kubectl-free/pkg/table"
	"github.com/thirdeyenick/kubectl-free/pkg/util"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"

	// Initialize all known client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	memorySortResource sortResource = "memory"
	cpuSortResource    sortResource = "cpu"
)

type sortResource string

// String implements the stringer interface
func (s *sortResource) String() string {
	if s == nil {
		return "null"
	}
	return string(*s)
}

// Set sets the content of the sortResource
func (s *sortResource) Set(v string) error {
	if v != memorySortResource.String() && v != cpuSortResource.String() {
		return fmt.Errorf("can only sort by %q and %q, not by given %q", memorySortResource.String(), cpuSortResource.String(), v)
	}
	*s = sortResource(v)
	return nil
}

// Type returns the type
func (s *sortResource) Type() string {
	return "sortResource"
}

var (
	// DfLong defines long description
	freeLong = templates.LongDesc(`
		Show various requested resources on Kubernetes nodes.
	`)

	// DfExample defines command examples
	freeExample = templates.Examples(`
		# Show pod resource usage of Kubernetes nodes (default namespace is "default").
		kubectl free

		# Show pod resource usage of Kubernetes nodes (all namespaces).
		kubectl free --all-namespaces

		# Show pod resource usage of Kubernetes nodes with number of pods and containers.
		kubectl free --pod

		# Using label selector.
		kubectl free -l key=value

		# Print raw(bytes) usage.
		kubectl free --bytes --without-unit

		# Using binary prefix unit (GiB, MiB, etc)
		kubectl free -g -B

		# List resources of containers in pods on nodes.
		kubectl free --list

		# List resources of containers in pods on nodes with image information.
		kubectl free --list --list-image

		# Print container even if that has no resources/limits.
		kubectl free --list --list-all

		# Do you like emoji? 😃
		kubectl free --emoji
		kubectl free --list --emoji
	`)
)

// FreeOptions is struct of df options
type FreeOptions struct {
	configFlags *genericclioptions.ConfigFlags
	genericclioptions.IOStreams

	// general options
	labelSelector string
	table         *table.OutputTable
	pod           bool
	emojiStatus   bool
	allNamespaces bool
	noHeaders     bool
	noMetrics     bool
	compactView   bool

	// unit options
	bytes       bool
	kByte       bool
	mByte       bool
	gByte       bool
	withoutUnit bool
	binPrefix   bool

	// color output options
	nocolor       bool
	warnThreshold int64
	critThreshold int64

	// list options
	list               bool
	listContainerImage bool
	listAll            bool

	// sort options
	sortByResource sortResource

	// k8s clients
	nodeClient        clientv1.NodeInterface
	podClient         clientv1.PodInterface
	metricsPodClient  metricsv1beta1.PodMetricsInterface
	metricsNodeClient metricsv1beta1.NodeMetricsInterface

	// table headers
	freeTableHeaders []string
	listTableHeaders []string
}

// NewFreeOptions is an instance of FreeOptions
func NewFreeOptions(streams genericclioptions.IOStreams) *FreeOptions {
	return &FreeOptions{
		configFlags:        genericclioptions.NewConfigFlags(true),
		bytes:              false,
		kByte:              false,
		mByte:              true,
		gByte:              false,
		withoutUnit:        false,
		binPrefix:          false,
		nocolor:            false,
		warnThreshold:      60,
		critThreshold:      90,
		IOStreams:          streams,
		labelSelector:      "",
		list:               false,
		listContainerImage: false,
		listAll:            false,
		pod:                false,
		emojiStatus:        false,
		table:              table.NewOutputTable(os.Stdout),
		allNamespaces:      true,
		noHeaders:          false,
		noMetrics:          false,
		sortByResource:     memorySortResource,
		compactView:        true,
	}
}

// NewCmdFree is a cobra command wrapping
func NewCmdFree(f cmdutil.Factory, streams genericclioptions.IOStreams, version, commit, date string) *cobra.Command {
	o := NewFreeOptions(streams)

	cmd := &cobra.Command{
		Use:     "kubectl free",
		Short:   "Show various requested resources on Kubernetes nodes.",
		Long:    freeLong,
		Example: freeExample,
		Version: version,
		Run: func(c *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, c, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run(args))
		},
	}

	// bool options
	cmd.Flags().BoolVarP(&o.bytes, "bytes", "b", o.bytes, `Use 1-byte (1-Byte) blocks rather than the default.`)
	cmd.Flags().BoolVarP(&o.kByte, "kilobytes", "k", o.kByte, `Use 1024-byte (1-Kbyte) blocks rather than the default.`)
	cmd.Flags().BoolVarP(&o.mByte, "megabytes", "m", o.mByte, `Use 1048576-byte (1-Mbyte) blocks rather than the default.`)
	cmd.Flags().BoolVarP(&o.gByte, "gigabytes", "g", o.gByte, `Use 1073741824-byte (1-Gbyte) blocks rather than the default.`)
	cmd.Flags().BoolVarP(&o.binPrefix, "binary-prefix", "B", o.binPrefix, `Use 1024 for basic unit calculation instead of 1000. (print like "KiB")`)
	cmd.Flags().BoolVarP(&o.withoutUnit, "without-unit", "", o.withoutUnit, `Do not print size with unit string.`)
	cmd.Flags().Var(&o.sortByResource, "sort-by-resource", "Sort container list by CPU or memory usage.")
	cmd.Flags().BoolVarP(&o.nocolor, "no-color", "", o.nocolor, `Print without ansi color.`)
	cmd.Flags().BoolVarP(&o.pod, "pod", "p", o.pod, `Show pod count and limit.`)
	cmd.Flags().BoolVarP(&o.list, "list", "", o.list, `Show container list on node.`)
	cmd.Flags().BoolVarP(&o.listContainerImage, "list-image", "", o.listContainerImage, `Show pod list on node with container image.`)
	cmd.Flags().BoolVarP(&o.listAll, "list-all", "", o.listAll, `Show pods even if they have no requests/limit`)
	cmd.Flags().BoolVarP(&o.emojiStatus, "emoji", "", o.emojiStatus, `Let's smile!! 😃 😭`)
	cmd.Flags().BoolVarP(&o.allNamespaces, "all-namespaces", "", o.allNamespaces, `If present, list pod resources(limits) across all namespaces. Namespace in current context is ignored even if specified with --namespace.`)
	cmd.Flags().BoolVarP(&o.noHeaders, "no-headers", "", o.noHeaders, `Do not print table headers.`)
	cmd.Flags().BoolVarP(&o.noMetrics, "no-metrics", "", o.noMetrics, `Do not print node/pods/containers usage from metrics-server.`)
	cmd.Flags().BoolVarP(&o.compactView, "compact-view", "", o.compactView, `Only print usage of pods/containers in a compact view.`)

	// int64 options
	cmd.Flags().Int64VarP(&o.warnThreshold, "warn-threshold", "", o.warnThreshold, `Threshold of warn(yellow) color for USED column.`)
	cmd.Flags().Int64VarP(&o.critThreshold, "crit-threshold", "", o.critThreshold, `Threshold of critical(red) color for USED column.`)

	// string option
	cmd.Flags().StringVarP(&o.labelSelector, "selector", "l", o.labelSelector, `Selector (label query) to filter on.`)

	o.configFlags.AddFlags(cmd.Flags())

	// add the klog flags
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// version command template
	cmd.SetVersionTemplate("Version: " + version + ", GitCommit: " + commit + ", BuildDate: " + date + "\n")

	return cmd
}

// Complete prepares k8s clients
func (o *FreeOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {

	// get k8s client
	client, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}

	// node client
	o.nodeClient = client.CoreV1().Nodes()

	// metric client
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	mclient, err := o.setMetricsClient(config)
	if err != nil {
		return err
	}

	// pod and metrics client
	if o.allNamespaces {
		// --all-namespace flag
		o.podClient = client.CoreV1().Pods(v1.NamespaceAll)
		o.metricsPodClient = mclient.MetricsV1beta1().PodMetricses(v1.NamespaceAll)
	} else {
		if *o.configFlags.Namespace == "" {
			// default namespace is "default"
			o.podClient = client.CoreV1().Pods(v1.NamespaceDefault)
			o.metricsPodClient = mclient.MetricsV1beta1().PodMetricses(v1.NamespaceDefault)
		} else {
			// targeted namespace (--namespace flag)
			o.podClient = client.CoreV1().Pods(*o.configFlags.Namespace)
			o.metricsPodClient = mclient.MetricsV1beta1().PodMetricses(*o.configFlags.Namespace)
		}
	}
	o.metricsNodeClient = mclient.MetricsV1beta1().NodeMetricses()

	// prepare table header
	o.prepareFreeTableHeader()
	o.prepareListTableHeader()

	return nil
}

// Validate ensures that all required arguments and flag values are provided
func (o *FreeOptions) Validate() error {

	// validate threshold
	if err := util.ValidateThreshold(o.warnThreshold, o.critThreshold); err != nil {
		return err
	}

	return nil
}

// Run printing disk usage of images
func (o *FreeOptions) Run(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// get nodes
	nodes, err := util.GetNodes(ctx, o.nodeClient, args, o.labelSelector)
	if err != nil {
		return nil
	}

	// list pods and return
	if o.list {
		if err := o.showPodsOnNode(ctx, nodes); err != nil {
			return err
		}
		return nil
	}

	// print cpu/mem/pod resource usage
	if err := o.showFree(ctx, nodes); err != nil {
		return err
	}

	return nil
}

// prepareFreeTableHeader defines table headers for free usage
func (o *FreeOptions) prepareFreeTableHeader() {

	hName := "NAME"
	hStatus := "STATUS"
	hCPUUse := "CPU/use"
	hCPUReq := "CPU/req"
	hCPULim := "CPU/lim"
	hCPUAlloc := "CPU/alloc"
	hCPUUseP := "CPU/use%"
	hCPUReqP := "CPU/req%"
	hCPULimP := "CPU/lim%"
	hMEMUse := "MEM/use"
	hMEMReq := "MEM/req"
	hMEMLim := "MEM/lim"
	hMEMAlloc := "MEM/alloc"
	hMEMUseP := "MEM/use%"
	hMEMReqP := "MEM/req%"
	hMEMLimP := "MEM/lim%"
	hPods := "PODS"
	hPodsAlloc := "PODS/alloc"
	hContainers := "CONTAINERS"

	if !o.nocolor {
		// hack: avoid breaking column by escape char
		util.DefaultColor(&hStatus)  // STATUS
		util.DefaultColor(&hCPUUseP) // CPU/use%
		util.DefaultColor(&hCPUReqP) // CPU/req%
		util.DefaultColor(&hCPULimP) // CPU/lim%
		util.DefaultColor(&hMEMUseP) // MEM/use%
		util.DefaultColor(&hMEMReqP) // MEM/req%
		util.DefaultColor(&hMEMLimP) // MEM/lim%
	}

	baseHeader := []string{
		hName,
		hStatus,
	}

	cpuHeader := []string{
		hCPUReq,
		hCPULim,
		hCPUAlloc,
	}

	cpuPHeader := []string{
		hCPUReqP,
		hCPULimP,
	}

	memHeader := []string{
		hMEMReq,
		hMEMLim,
		hMEMAlloc,
	}

	memPHeader := []string{
		hMEMReqP,
		hMEMLimP,
	}

	podHeader := []string{
		hPods,
		hPodsAlloc,
		hContainers,
	}

	if !o.noMetrics {
		// insert metrics columns
		cpuHeader = append([]string{hCPUUse}, cpuHeader...)
		cpuPHeader = append([]string{hCPUUseP}, cpuPHeader...)
		memHeader = append([]string{hMEMUse}, memHeader...)
		memPHeader = append([]string{hMEMUseP}, memPHeader...)
	}

	// finally, join all columns
	fth := []string{}

	fth = append(fth, baseHeader...)
	fth = append(fth, cpuHeader...)
	fth = append(fth, cpuPHeader...)
	fth = append(fth, memHeader...)
	fth = append(fth, memPHeader...)

	if o.pod {
		fth = append(fth, podHeader...)
	}

	o.freeTableHeaders = fth
}

// prepareListTableHeader defines table headers for --list
func (o *FreeOptions) prepareListTableHeader() {

	hNode := "NODE NAME"
	hNameSpace := "NAMESPACE"
	hPod := "POD NAME"
	hPodIP := "POD IP"
	hPodStatus := "POD STATUS"
	hPodAge := "POD AGE"
	hContainer := "CONTAINER"
	hCPUUse := "CPU/use"
	hCPUReq := "CPU/req"
	hCPULim := "CPU/lim"
	hMEMUse := "MEM/use"
	hMEMReq := "MEM/req"
	hMEMLim := "MEM/lim"
	hImage := "IMAGE"

	if !o.nocolor {
		// hack: avoid breaking column by escape char
		util.DefaultColor(&hPodStatus) // POD STATUS
	}

	baseHeader := []string{
		hNode,
		hNameSpace,
	}

	var podHeader []string
	if !o.compactView {
		podHeader = []string{
			hPod,
			hPodAge,
			hPodIP,
			hPodStatus,
		}
	} else {
		podHeader = []string{
			hPod,
			hPodStatus,
		}
	}

	containerHeader := []string{
		hContainer,
	}

	cpuHeader := []string{
		hCPUReq,
		hCPULim,
	}

	memHeader := []string{
		hMEMReq,
		hMEMLim,
	}

	imageHeader := []string{
		hImage,
	}

	if !o.noMetrics {
		if o.compactView {
			cpuHeader = []string{hCPUUse}
			memHeader = []string{hMEMUse}
		} else {
			// insert metrics columns
			cpuHeader = append([]string{hCPUUse}, cpuHeader...)
			memHeader = append([]string{hMEMUse}, memHeader...)
		}
	}

	// finally, join all columns
	lth := []string{}

	lth = append(lth, baseHeader...)
	lth = append(lth, podHeader...)
	lth = append(lth, containerHeader...)
	lth = append(lth, cpuHeader...)
	lth = append(lth, memHeader...)

	if o.listContainerImage {
		lth = append(lth, imageHeader...)
	}

	o.listTableHeaders = lth
}

// setMetricsClient sets metrics client
func (o *FreeOptions) setMetricsClient(config *rest.Config) (*metrics.Clientset, error) {

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return metricsClient, nil
}

// toUnit calculate and add unit for int64
func (o *FreeOptions) toUnit(i int64) string {

	var unitbytes int64
	var unitstr string

	if o.binPrefix {
		unitbytes, unitstr = util.GetBinUnit(o.bytes, o.kByte, o.mByte, o.gByte)
	} else {
		unitbytes, unitstr = util.GetSiUnit(o.bytes, o.kByte, o.mByte, o.gByte)
	}

	// -H adds human readable unit
	unit := ""
	if !o.withoutUnit {
		unit = unitstr
	}

	return strconv.FormatInt(i/unitbytes, 10) + unit
}

// toUnitOrDash returns "-" if "i" is 0, otherwise returns toUnit()
func (o *FreeOptions) toUnitOrDash(i int64) string {

	if i == 0 {
		return "-"
	}

	return o.toUnit(i)
}

// toMilliUnitOrDash returns "-" if "i" is 0, otherwise returns MilliQuantity
func (o *FreeOptions) toMilliUnitOrDash(i int64) string {

	if i == 0 {
		return "-"
	}

	if o.withoutUnit {
		// return raw value
		return strconv.FormatInt(i, 10)
	}

	return resource.NewMilliQuantity(i, resource.DecimalSI).String()
}

// toColorPercent returns colored strings
//
//	percentage < warn : Green
//
// warn < percentage < crit : Yellow
// crit < percentage        : Red
func (o *FreeOptions) toColorPercent(i int64) string {
	p := strconv.FormatInt(i, 10) + "%"

	if o.nocolor {
		// nothing to do
		return p
	}

	switch {
	case i < o.warnThreshold:
		// percentage < warn : Green
		util.Green(&p)
	case i < o.critThreshold:
		// warn < percentage < crit : Yellow
		util.Yellow(&p)
	default:
		// crit < percentage : Red
		util.Red(&p)
	}

	return p
}
