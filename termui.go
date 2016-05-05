// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"

	lib "github.com/uber-common/cpustat/lib"
	"github.com/uber-common/termui"
)

const chartBackingSize = 1024

var sysChart *termui.LineChart
var procChart *termui.LineChart
var mainList *termui.List
var quitChan chan string

var sysChartData = make(map[string][]float64)
var procChartData = make(map[int][]float64)

func tuiFatal(reason string) {
	termui.StopLoop()
	termui.Close()
	quitChan <- reason
	close(quitChan)
}

// these values are from colorbrewer2.org
var colorValues = []string{
	"#9e0142",
	"#d53e4f",
	"#fdae61",
	"#fee08b",
	"#66c2a5",
	"#3288bd",
	"#5e4fa2",
	"#6e5fb2",
	"#7f3b08",
	"#ffffff",
	"#8073ac",
	"#542788",
	"#a6cee3",
	"#33a02c",
	"#b2df8a",
}
var colorList []termui.Attribute
var graphColors map[string]termui.Attribute
var dataLabels []string

func tuiInit(ch chan string, interval int) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			runtime.Stack(buf, false)
			tuiFatal(fmt.Sprintf("%s\n\n%s\n", r, string(buf)))
		}
	}()

	quitChan = ch

	//	termui.DebugFilename = "tuidebug"
	//	termui.Debug("cpustat termui starting...")

	colorList = make([]termui.Attribute, 0)
	for pos, colorStr := range colorValues {
		r, _ := strconv.ParseUint(colorStr[1:3], 16, 8)
		g, _ := strconv.ParseUint(colorStr[3:5], 16, 8)
		b, _ := strconv.ParseUint(colorStr[5:7], 16, 8)
		newAttr := termui.ColorRGB24(int(r), int(g), int(b)) | termui.AttrBold
		colorList = append(colorList, newAttr)
		termui.AddColorMap(fmt.Sprintf("color%d", pos), newAttr)
	}

	dataLabels = make([]string, 1024)
	for i := range dataLabels {
		val := fmt.Sprintf("%.1f", math.Abs(float64((i-1023)*interval)/1000.0))
		if strings.HasSuffix(val, ".0") {
			dataLabels[i] = val[:len(val)-2]
		} else {
			dataLabels[i] = val
		}
	}

	sysChartData["usr"] = make([]float64, 0, chartBackingSize)
	sysChartData["sys"] = make([]float64, 0, chartBackingSize)

	if err := termui.Init(); err != nil {
		panic(err)
	}
	termui.SetOutputMode(termui.Output256)

	sysChart = termui.NewLineChart()
	sysChart.Name = "sysChart"
	sysChart.Border = false
	sysChart.BorderLabel = "        total usr/sys time" // label with no border is odd, use manual padding
	sysChart.Height = termui.TermHeight() / 2
	sysChart.YFloor = 0.0
	sysChart.LineColor["usr"] = termui.ColorCyan | termui.AttrBold
	sysChart.LineColor["sys"] = termui.ColorRed | termui.AttrBold

	procChart = termui.NewLineChart()
	procChart.Name = "procChart"
	procChart.Border = false
	procChart.BorderLabel = "       top procs"
	procChart.Height = termui.TermHeight() / 2
	procChart.YFloor = 0.0

	mainList = termui.NewList()
	mainList.Border = false
	mainList.Items = []string{"[gathering list of top processes](fg-red,bg-white)"}
	mainList.Height = termui.TermHeight() / 2

	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(6, 0, procChart),
			termui.NewCol(6, 0, sysChart),
		),
		termui.NewRow(
			termui.NewCol(12, 0, mainList),
		),
	)

	termui.Body.Align()
	termui.Render(termui.Body)

	termui.Handle("/sys/kbd/q", func(termui.Event) {
		tuiFatal("closing from keyboard")
	})

	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		mainList.Height = termui.TermHeight() / 2
		procChart.Height = termui.TermHeight() / 2
		sysChart.Height = termui.TermHeight() / 2
		termui.Body.Width = termui.TermWidth()
		termui.Body.Align()
		termui.Render(termui.Body)
	})

	termui.Loop()
}

// this is a lot of copy/paste from dumpStats. Would be good to refactor this to share.
func tuiListUpdate(infoMap lib.ProcInfoMap, list lib.Pidlist, procSum lib.ProcSampleMap,
	procHist lib.ProcStatsHistMap, taskHist lib.TaskStatsHistMap,
	sysSum *lib.SystemStats, sysHist *lib.SystemStatsHist, jiffy, interval, samples int) {

	// if something in here panics, the output goes to the screen, which conflicts with termbox mode.
	// try to capture this and quit termbox before we print the crash.
	defer func() {
		if r := recover(); r != nil {
			tuiFatal(fmt.Sprint(r))
		}
	}()

	scale := func(val float64) float64 {
		return val / float64(jiffy) / float64(interval) * 1000 * 100
	}
	scaleSum := func(val float64, count int64) float64 {
		valSec := val / float64(jiffy)
		sampleSec := float64(interval) * float64(count) / 1000.0
		ret := (valSec / sampleSec) * 100
		return ret
	}
	scaleSumUs := func(val float64, count int64) float64 {
		valSec := val / 1000 / 1000 / float64(interval)
		sampleSec := float64(interval) * float64(count) / 1000.0
		return (valSec / sampleSec) * 100
	}

	graphColors = make(map[string]termui.Attribute)
	mainList.Items = make([]string, len(list)+1)
	colorPos := 0

	mainList.Items[0] = fmt.Sprint("                      name    pid     min     max     usr     sys    runq     iow    swap   vcx   icx   ctime   rss nice thrd  sam\n")

	for i, pid := range list {
		sampleCount := procHist[pid].Ustime.TotalCount()

		var cpuDelay, blockDelay, swapDelay, nvcsw, nivcsw string

		if proc, ok := procSum[pid]; ok == true {
			cpuDelay = trim(scaleSumUs(float64(proc.Task.Cpudelaytotal), sampleCount), 7)
			blockDelay = trim(scaleSumUs(float64(proc.Task.Blkiodelaytotal), sampleCount), 7)
			swapDelay = trim(scaleSumUs(float64(proc.Task.Swapindelaytotal), sampleCount), 7)
			nvcsw = formatNum(proc.Task.Nvcsw)
			nivcsw = formatNum(proc.Task.Nivcsw)
		} // silently ignore missing data in fancy mode

		strPid := fmt.Sprint(pid)
		graphColors[strPid] = colorList[colorPos]

		mainList.Items[i+1] = fmt.Sprintf("[%26s %6d](fg-color%d) %7s %7s %7s %7s %7s %7s %7s %5s %5s %7s %5s %4d %4d %4d",
			trunc(infoMap[pid].Friendly, 28),
			pid,
			colorPos,
			trim(scale(float64(procHist[pid].Ustime.Min())), 7),
			trim(scale(float64(procHist[pid].Ustime.Max())), 7),
			trim(scaleSum(float64(procSum[pid].Proc.Utime), sampleCount), 7),
			trim(scaleSum(float64(procSum[pid].Proc.Stime), sampleCount), 7),
			cpuDelay,
			blockDelay,
			swapDelay,
			nvcsw,
			nivcsw,
			trim(scaleSum(float64(procSum[pid].Proc.Cutime+procSum[pid].Proc.Cstime), sampleCount), 7),
			formatMem(procSum[pid].Proc.Rss),
			infoMap[pid].Nice,
			procSum[pid].Proc.Numthreads,
			sampleCount,
		)
		colorPos = (colorPos + 1) % len(colorList)
	}

	termui.Render(mainList)
}

func tuiGraphUpdate(procDelta lib.ProcSampleMap, sysDelta *lib.SystemStats, topPids lib.Pidlist,
	jiffy, interval uint32) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			runtime.Stack(buf, false)
			tuiFatal(fmt.Sprintf("%s\n\n%s\n", r, string(buf)))
		}
	}()

	scale := func(val float64) float64 {
		return val / float64(jiffy) / float64(interval) * 1000 * 100
	}

	sysChartData["usr"] = append(sysChartData["usr"], scale(float64(sysDelta.Usr)))
	sysChartData["sys"] = append(sysChartData["sys"], scale(float64(sysDelta.Sys)))

	dataPoints := (sysChart.InnerWidth() * 2) - 14 // WTF is this magic number for?
	dataStart := dataPoints
	for name, data := range sysChartData {
		if len(data) < dataPoints {
			dataStart = len(data)
		}
		sysChart.Data[name] = data[len(data)-dataStart:]
	}
	sysChart.DataLabels = dataLabels[len(dataLabels)-dataPoints:]
	termui.Render(sysChart)

	updatedPids := make(map[int]bool)

	for pid, delta := range procDelta {
		updatedPids[pid] = true
		if _, ok := procChartData[pid]; ok == false {
			procChartData[pid] = make([]float64, 1, chartBackingSize)
			procChartData[pid][0] = scale(float64(delta.Proc.Utime + delta.Proc.Stime))
		} else {
			procChartData[pid] = append(procChartData[pid], scale(float64(delta.Proc.Utime+delta.Proc.Stime)))
		}
	}

	for _, pid := range topPids {
		if updatedPids[pid] == false {
			procChartData[pid] = append(procChartData[pid], 0)
		}
	}

	graphData := make(map[string][]float64)
	for _, pid := range topPids {
		if pid == 0 { // skip uninitialized values
			continue
		}
		data := procChartData[pid]
		dataStart := dataPoints
		if len(data) < dataPoints {
			dataStart = len(data)
		}
		strPid := fmt.Sprint(pid)
		graphData[strPid] = data[len(data)-dataStart:]
	}
	procChart.Data = graphData
	procChart.LineColor = graphColors
	procChart.DataLabels = dataLabels[len(dataLabels)-dataPoints:]
	termui.Render(procChart)
}
