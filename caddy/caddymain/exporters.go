// Copyright 2018 Light Code Labs, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package caddymain

// This file contains the utilities to enable distributed tracing and monitoring
// with OpenCensus https://opencensus.io. Its inclusion in Caddy is to enable
// trace propagation in cases where Caddy calls out to another backend or
// even when serving files or being the main server itself.
// With this integration, we can also collect runtime metrics in addition
// to HTTP server and client metrics.
//
// To examine the metrics, one can use:
// a) AWS X-Ray
// b) Stackdriver Monitoring and Tracing
// c) Prometheus
// ... and other exporters as needed

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	// Distributed tracing and monitoring imports
	xray "github.com/census-instrumentation/opencensus-go-exporter-aws"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/exporter/zipkin"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

type runtimeMetrics struct {
	reportPeriod time.Duration
}

const dimensionless = "1"

var (
	mFrees        = stats.Int64("frees", "The number of frees", dimensionless)
	mHeapAllocs   = stats.Int64("heap_allocs", "The number of heap allocations", dimensionless)
	mHeapObjects  = stats.Int64("heap_objects", "The number of objects allocated on the heap", dimensionless)
	mHeapReleased = stats.Int64("heap_released", "The number of objects released from the heap", dimensionless)
	mPtrLookups   = stats.Int64("ptr_lookups", "The number of pointer lookups", dimensionless)
	mStackSys     = stats.Int64("stack_sys", "The memory used by stack spans and OS thread stacks", dimensionless)
)

var memStatsViews = []*view.View{
	{Name: "mem_frees", Measure: mFrees, Description: "The number of frees", Aggregation: view.Count()},
	{Name: "mem_allocs", Measure: mHeapAllocs, Description: "The number of heap allocations", Aggregation: view.Count()},
	{Name: "mem_heap_objects", Measure: mHeapObjects, Description: "The number of objects allocated on the heap", Aggregation: view.Count()},
	{Name: "mem_heap_released", Measure: mHeapReleased, Description: "The number of objects released from the heap", Aggregation: view.Count()},
	{Name: "mem_ptr_lookups", Measure: mPtrLookups, Description: "The number of pointer lookups", Aggregation: view.Count()},
	{
		Name: "mem_stack_sys", Measure: mStackSys, Description: "The memory used by stack spans and OS thread stacks",
		Aggregation: view.Distribution(
			0, 1<<10, 10*1<<10, 100*1<<10, 1<<20, 10*1<<20, 100*1<<20,
			1<<30, 10<<30, 100<<30, 1<<40, 10<<40, 100<<40, 1<<50, 10*1<<50, 100*1<<50,
			1<<60, 10*1<<60, 100*1<<60, 1<<62, 10*1<<62, 100*1<<62, 1<<63),
	},
}

// do is a blocking routine that periodically
// runs the runtime metrics collector.
func (rm *runtimeMetrics) cycle(cancel chan bool) {
	var period time.Duration
	if rm != nil {
		period = rm.reportPeriod
	}
	if period <= 0 {
		period = 15 * time.Second
	}

	ms := new(runtime.MemStats)
	ctx := context.Background()

	for {
		select {
		case <-cancel:
			return

		case <-time.After(period):
			runtime.ReadMemStats(ms)
			stats.Record(ctx, mHeapAllocs.M(int64(ms.HeapAlloc)), mFrees.M(int64(ms.Frees)), mPtrLookups.M(int64(ms.Lookups)),
				mStackSys.M(int64(ms.StackSys)), mHeapObjects.M(int64(ms.HeapObjects)), mHeapReleased.M(int64(ms.HeapReleased)),
			)
		}
	}
}

func observabilityLogf(fmt_ string, args ...interface{}) {
	log.Printf("[observability] %s", fmt.Sprintf(fmt_, args...))
}

func createObservabilityExporters(observabilityOptions string) error {
	observabilityOptions = strings.TrimSpace(observabilityOptions)
	if observabilityOptions == "" {
		// Nothing to do
		return nil
	}

	exportsConfig := observabilityOptions
	var sampleRate = -1.0
	if sampleRateDelimCount := strings.Count(observabilityOptions, ";"); sampleRateDelimCount > 0 {
		if sampleRateDelimCount != 1 {
			return fmt.Errorf(`Expecting only one single delimiter ";" got %d in %q`, sampleRateDelimCount, observabilityOptions)
		}

		// In this case we've got: <sampleRate>;<exporter>[:exporter_options...]
		splits := strings.Split(observabilityOptions, ";")
		rateStr := splits[0]
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil {
			return fmt.Errorf("Failed to parse a float out of %q got error %v", observabilityOptions, err)
		}
		sampleRate = rate
		exportsConfig = splits[1]
	}

	observabilityLogf("SampleRate: %.3f\n", sampleRate)
	defaultSampler, skipExporters := extractSampler(sampleRate)
	trace.ApplyConfig(trace.Config{DefaultSampler: defaultSampler})

	if skipExporters {
		return nil
	}

	if err := createExporters(exportsConfig); err != nil {
		return err
	}

	if err := view.Register(ochttp.DefaultServerViews...); err != nil {
		return fmt.Errorf("Failed to register ochttp.DefaultServerViews: %v", err)
	}
	if err := view.Register(ochttp.DefaultClientViews...); err != nil {
		return fmt.Errorf("Failed to register ochttp.DefaultClientViews: %v", err)
	}

	// Collecting runtime stats
	rmc := new(runtimeMetrics)
	observabilityLogf("Registering runtimeMetrics")
	if err := view.Register(memStatsViews...); err != nil {
		return fmt.Errorf("Failed to register memory statistics views: %v", err)
	}
	go rmc.cycle(nil)

	return nil
}

func extractSampler(sampleRate float64) (sampler trace.Sampler, skipExporters bool) {
	switch {
	case sampleRate <= 0.0:
		sampler = trace.NeverSample()
		// If the rate is <= 0.0, in this case the user absolutely
		// doesn't want any exporters nor metrics.
		skipExporters = true
	case sampleRate >= 1.0:
		sampler = trace.AlwaysSample()
	default:
		sampler = trace.ProbabilitySampler(sampleRate)
	}
	return sampler, skipExporters
}

func createExporters(exportersConfig string) error {
	exportersConfig = strings.TrimSpace(exportersConfig)
	if exportersConfig == "" {
		return nil
	}

	// Sample content:
	//  "prometheus:port=8978,stackdriver:tracing=true:monitoring=true:metrics-prefix=foo_bar,aws-xray,zipkin"
	// which is then split into
	//  ["prometheus:port=8978", "stackdriver:tracing=true:monitoring=true:metrics-prefix=foo_bar", "aws-xray", "zipkin"]
	exporterConfigsSplits := strings.Split(exportersConfig, ",")

	for _, exporterConfig := range exporterConfigsSplits {
		nameConfigSplits := strings.Split(exporterConfig, ":")
		name := strings.ToLower(nameConfigSplits[0])
		var keyValues []string
		if len(nameConfigSplits) > 1 {
			keyValues = nameConfigSplits[1:]
		}

		switch name {
		case "aws-xray":
			if err := parseAndRegisterAWSXRayExporter(keyValues); err != nil {
				return fmt.Errorf("AWS X-Ray exporter: %v", err)
			}

		case "prometheus":
			if err := parseAndRegisterPrometheusExporter(keyValues); err != nil {
				return fmt.Errorf("Prometheus exporter: %v", err)
			}

		case "stackdriver":
			if err := parseAndRegisterStackdriverExporter(keyValues); err != nil {
				return fmt.Errorf("Stackdriver exporter: %v", err)
			}

		case "zipkin":
			if err := parseAndRegisterZipkinExporter(keyValues); err != nil {
				return fmt.Errorf("Zipkin exporter: %v", err)
			}

		case "jaeger":
			if err := parseAndRegisterJaegerExporter(keyValues); err != nil {
				return fmt.Errorf("Jaeger exporter: %v", err)
			}
		}
	}

	return nil
}

const defaultMetricsNamespace = "caddyserver"

func parseAndRegisterPrometheusExporter(keyValues []string) error {
	namespace := defaultMetricsNamespace
	portStr := "9999"

	for _, kv := range keyValues {
		kvSplit := strings.Split(kv, "=")

		switch strings.ToLower(kvSplit[0]) {
		case "port":
			if len(kvSplit) > 1 {
				portStr = kvSplit[1]
			}

		case "namespace":
			if len(kvSplit) > 1 {
				namespace = kvSplit[1]
			}

		default:
			// TODO: Add other Prometheus configuration key-value pairs here.
		}
	}

	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse port from Prometheus configuration from configuration %q: %v", portStr, err)
	}

	pe, err := prometheus.NewExporter(prometheus.Options{Namespace: namespace})
	if err != nil {
		return err
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		addr := fmt.Sprintf(":%d", port)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Failed to serve Prometheus endpoint: %v", err)
		}
	}()

	view.RegisterExporter(pe)
	observabilityLogf("Successfully registered Prometheus exporter\n")

	return nil
}

func parseAndRegisterAWSXRayExporter(keyValues []string) error {
	xe, err := xray.NewExporter(xray.WithVersion("latest"))
	if err != nil {
		return err
	}

	trace.RegisterExporter(xe)
	observabilityLogf("Successfully register AWS X-Ray exporter\n")

	return nil
}

func parseAndRegisterStackdriverExporter(keyValues []string) error {
	metricsPrefix := defaultMetricsNamespace
	monitoring := false
	tracing := false
	projectID := ""

	for _, keyValue := range keyValues {
		kvSplit := strings.Split(keyValue, "=")

		switch strings.ToLower(kvSplit[0]) {
		case "tracing":
			tracing = len(kvSplit) > 1 && (kvSplit[1] == "1" || strings.ToLower(kvSplit[1]) == "true")
		case "monitoring":
			monitoring = len(kvSplit) > 1 && (kvSplit[1] == "1" || strings.ToLower(kvSplit[1]) == "true")
		case "metrics-prefix":
			if len(kvSplit) > 1 {
				metricsPrefix = kvSplit[1]
			}
		case "project-id":
			if len(kvSplit) > 1 {
				projectID = kvSplit[1]
			}
		}
	}

	se, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:    projectID,
		MetricPrefix: metricsPrefix,
	})
	if err != nil {
		log.Fatalf("Failed to create Stackdriver exporter: %v", err)
	}

	if tracing {
		trace.RegisterExporter(se)
		observabilityLogf("Successfully register Stackdriver Trace exporter\n")
	}
	if monitoring {
		view.RegisterExporter(se)
		observabilityLogf("Successfully register Stackdriver Monitoring exporter\n")
	}

	return nil
}

func parseAndRegisterZipkinExporter(keyValues []string) error {
	localEndpointURI := "192.168.1.5:5454"
	reporterURI := "http://localhost:9411/api/v2/spans"
	serviceName := "server"

	for _, keyValue := range keyValues {
		kvSplit := strings.Split(keyValue, "=")

		switch strings.ToLower(kvSplit[0]) {
		case "local":
			if len(kvSplit) > 0 {
				localEndpointURI = kvSplit[1]
			}
		case "reporter":
			if len(kvSplit) > 0 {
				reporterURI = kvSplit[1]
			}
		case "service-name":
			if len(kvSplit) > 1 {
				serviceName = kvSplit[1]
			}
		}
	}

	localEndpoint, err := openzipkin.NewEndpoint(serviceName, localEndpointURI)
	if err != nil {
		log.Fatalf("Failed to create Zipkin localEndpoint with URI %q error: %v", localEndpointURI, err)
	}

	reporter := zipkinHTTP.NewReporter(reporterURI)
	ze := zipkin.NewExporter(reporter, localEndpoint)
	trace.RegisterExporter(ze)
	log.Printf("Successfully registered Zipkin exporter")

	return nil
}

func parseAndRegisterJaegerExporter(keyValues []string) error {
	agentEndpointURI := "localhost:6831"
	collectorEndpointURI := "http://localhost:9411"
	serviceName := defaultMetricsNamespace

	for _, kv := range keyValues {
		kvSplit := strings.Split(kv, "=")

		switch strings.ToLower(kvSplit[0]) {
		case "collector":
			if len(kvSplit) > 0 {
				collectorEndpointURI = kvSplit[1]
			}
		case "agent":
			if len(kvSplit) > 0 {
				agentEndpointURI = kvSplit[1]
			}
		case "service-name":
			if len(kvSplit) > 1 {
				serviceName = kvSplit[1]
			}
		}
	}

	je, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint: agentEndpointURI,
		Endpoint:      collectorEndpointURI,
		ServiceName:   serviceName,
	})
	if err != nil {
		return err
	}

	trace.RegisterExporter(je)
	log.Printf("Successfully registered Jaeger exporter")

	return nil
}
