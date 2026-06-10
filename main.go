package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"gocleaner/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed configs/cleaner_rules.json
var embeddedRules []byte

func main() {
	application := app.New(embeddedRules)

	err := wails.Run(&options.App{
		Title:     buildTitle(),
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        application.Startup,
		Bind: []interface{}{
			application,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// buildTitle returns the window title. Keeping encoding-sensitive text
// in a separate function makes it easier to verify UTF-8 correctness.
func buildTitle() string {
	return "GoCleaner - 系统空间清理工具"
}
