package main

import ui "github.com/gizak/termui"

func main() {
	ui.DebugFilename = "tuidebug"
	ui.Debug("tuitest termui starting...")

	data := make(map[string][]float64)
	data["usr"] = make([]float64, 1, 1024)

	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

	lc := ui.NewLineChart()
	lc.Border = true
	lc.Height = ui.TermHeight()
	lc.YFloor = 0.0
	lc.Mode = "braille"
	lc.Data = data
	lc.LineColor["usr"] = ui.ColorCyan

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, lc),
		),
	)

	ui.Body.Align()
	ui.Render(ui.Body)

	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})

	ui.Handle("/sys/kbd/n", func(ui.Event) {
		next := data["usr"][len(data["usr"])-1] + 1
		data["usr"] = append(data["usr"], next)
		ui.Render(lc)
	})

	ui.Handle("/sys/kbd/d", func(ui.Event) {
		next := data["usr"][len(data["usr"])-1] - 1
		data["usr"] = append(data["usr"], next)
		ui.Render(lc)
	})

	ui.Handle("/sys/kbd/j", func(ui.Event) {
		next := data["usr"][len(data["usr"])-1] + 5
		data["usr"] = append(data["usr"], next)
		ui.Render(lc)
	})

	ui.Handle("/sys/kbd/m", func(ui.Event) {
		next := data["usr"][len(data["usr"])-1]
		data["usr"] = append(data["usr"], next)
		ui.Render(lc)
	})

	ui.Loop()
}

// ⢂⠒⠒⠂
// ⢀⠒⠒⠒
