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

import (
	"context"
	"runtime"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
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

// do is a blocking routine that runs the
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
