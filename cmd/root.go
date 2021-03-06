package cmd

import (
	"finala/config"
	"finala/serverutil"
	"finala/storage"
	"finala/visibility"
	"finala/webserver"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var (

	// storageDriver defind the storage driver. See value option in avilableStorageDrivers variable
	storageDriver string

	// storageConnectionString defind the storage connection string
	storageConnectionString string

	// disableClearStorageData will delete all historical storage data
	disableClearStorageData bool

	// Open UI dashboard with unutilized prepared dashboard
	disableUI bool

	// Open UI dashboad with given port. On default localhost:9090
	uiPort int

	// cfgFile contine the path to the configuration file
	cfgFile string

	// Cfg include the application configuration
	Cfg config.Config

	// Storage will manage to storage work
	Storage *storage.MySQLManager

	// err define for a global cmd error
	err error

	// avilableStorageDrivers present the available storage driver types
	avilableStorageDrivers = []string{"mysql", "postgres", "sqlite3", "mssql"}

	webserverStopper serverutil.StopFunc
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "finala",
	Short: "Analyze wasteful and unused resources to cut unwanted expenses ",
	Long: `A resource cloud scanner that analyzes and reports about wasteful and unused resources to cut unwanted expenses.
The tool is based on yaml definitions (no code), by default configuration OR given yaml file and the report output will be saved in a given storage.`,
}

// Execute will expose all cobra commands
func Execute() {

	if err := rootCmd.Execute(); err != nil {
		log.WithError(err)
		os.Exit(1)
	}

	if webserverStopper != nil {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)

		<-stop // block until we are requested to stop
		webserverStopper()
	}

}

func runWebserver(storage *storage.MySQLManager, port int) serverutil.StopFunc {

	webserverManager := webserver.NewServer(uiPort, storage)

	go func() {
		time.Sleep(time.Second * 2)
		openbrowser(fmt.Sprintf("http://localhost:%d/static/", port))
	}()

	return serverutil.RunAll(webserverManager).StopFunc
}

// init cobra commands
func init() {

	cobra.OnInitialize(initCmd)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&storageDriver, "storage-driver", "sqlite3", fmt.Sprintf("Storage driver. (Options: %s)", strings.Join(avilableStorageDrivers, ",")))
	rootCmd.PersistentFlags().StringVar(&storageConnectionString, "storage-connection-string", "DB.db", "Storage connection string. Default will be DB.db")
	rootCmd.PersistentFlags().BoolVar(&disableClearStorageData, "disable-clear-storage", false, "Clear storage data")
	rootCmd.PersistentFlags().BoolVar(&disableUI, "disable-ui", false, "Disable UI dashboard view")
	rootCmd.PersistentFlags().IntVar(&uiPort, "ui-port", 9090, "UI port. default 9090")
}

// initCmd will prepare the configuration and validate the common flag parametes
func initCmd() {

	// Validate yaml file
	if !strings.HasSuffix(cfgFile, ".yaml") {
		log.WithField("file", cfgFile).Error("Configuration file must be a yaml file")
		os.Exit(1)
	}

	// Validate storage driver type
	validStorageDriver := false
	for _, driver := range avilableStorageDrivers {
		if driver == storageDriver {
			validStorageDriver = true
			break
		}
	}

	if !validStorageDriver {
		log.WithField("storage", storageDriver).Errorf("Unsupported storage driver. available storage types: %s", strings.Join(avilableStorageDrivers, ","))
		os.Exit(1)
	}

	// Loading configuration file
	Cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		os.Exit(1)
	}

	if !disableClearStorageData {
		switch storageDriver {
		case "sqlite3":
			os.Remove(storageConnectionString)
		default:
			// TBD
		}
	}

	visibility.SetLoggingLevel(Cfg.LogLevel)
	Storage = storage.NewStorageManager(storageDriver, storageConnectionString)

	if !disableUI {
		webserverStopper = runWebserver(Storage, uiPort)
	}

}

// openbrowser will open a browser with given URL
func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}
