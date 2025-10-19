package main

import (
	"changeme/db_service"
	"changeme/proxy_service"
	"embed"
	_ "embed"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"log"
	"time"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var iconFS embed.FS

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "local-proxy",
		Description: "A demo of using raw HTML & CSS",
		Services: []application.Service{
			application.NewService(&db_service.DatabaseService{}),
			application.NewService(&proxy_service.ProxyService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.myapp.local-proxy",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				log.Printf("Second instance launched with args: %v", data.Args)
				log.Printf("Working directory: %s", data.WorkingDir)
				log.Printf("Additional data: %v", data.AdditionalData)
			},
			// Optional: Pass additional data to second instance
			AdditionalData: map[string]string{
				"launchtime": time.Now().String(),
			},
		},
	})

	iconBytes, _ := iconFS.ReadFile("build/appicon.png")

	systray := app.SystemTray.New()
	systray.SetIcon(iconBytes)

	// Create a menu with Open Window and Quit items for the system tray
	trayMenu := application.NewMenu()

	var mainWindow *application.WebviewWindow

	trayMenu.Add("Open Window").OnClick(func(ctx *application.Context) {
		if mainWindow == nil {
			mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
				Title:  "local-proxy",
				Width:  1024,
				Height: 700,
				URL:    "/",
			})
			// If supported by this Wails version, clear reference when closed to allow re-opening
			mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
				mainWindow = nil
			})
		}
		// Show and focus the existing/new window
		mainWindow.Show()
		mainWindow.Focus()
	})

	trayMenu.AddSeparator()

	trayMenu.Add("Quit").OnClick(func(ctx *application.Context) {
		app.Quit()
	})
	systray.SetMenu(trayMenu)

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
