// service for exchanging current user settings with UI
package settings

import (
	"net/http"
	"sync"

	"github.com/getlantern/flashlight/config"
	"github.com/getlantern/launcher"

	"github.com/getlantern/flashlight/ui"
	"github.com/getlantern/golog"
)

const (
	messageType = `Settings`
)

var (
	log           = golog.LoggerFor("flashlight.settings")
	service       *ui.Service
	cfgMutex      sync.RWMutex
	settingsMutex sync.RWMutex
	baseSettings  *Settings
	httpClient    *http.Client
)

type Settings struct {
	Version      string
	BuildDate    string
	RevisionDate string
	AutoReport   bool
	AutoLaunch   bool
	ProxyAll     bool
}

func Configure(cfg *config.Config, version, revisionDate string, buildDate string) {

	cfgMutex.Lock()
	defer cfgMutex.Unlock()

	if service == nil {
		// base settings are always written
		baseSettings = &Settings{
			Version:      version,
			BuildDate:    buildDate,
			RevisionDate: revisionDate,
			AutoReport:   *cfg.AutoReport,
			AutoLaunch:   *cfg.AutoLaunch,
			ProxyAll:     cfg.Client.ProxyAll,
		}

		err := start(baseSettings)
		if err != nil {
			log.Errorf("Unable to register settings service: %q", err)
			return
		}
		go read()
	} else {
		if *cfg.AutoLaunch != baseSettings.AutoLaunch {
			// autolaunch setting modified on disk
			launcher.CreateLaunchFile(*cfg.AutoLaunch)
		}
		baseSettings.AutoReport = *cfg.AutoReport
		baseSettings.AutoLaunch = *cfg.AutoLaunch
		baseSettings.ProxyAll = cfg.Client.ProxyAll
	}
}

// start the settings service
// that synchronizes Lantern's configuration
// with every UI client
func start(baseSettings *Settings) error {
	var err error

	helloFn := func(write func(interface{}) error) error {
		log.Debugf("Sending Lantern settings to new client")
		settingsMutex.RLock()
		defer settingsMutex.RUnlock()
		return write(baseSettings)
	}
	service, err = ui.Register(messageType, nil, helloFn)
	return err
}

func read() {
	log.Tracef("Reading settings messages!!")
	for msg := range service.In {
		log.Tracef("Read settings message!! %q", msg)
		settings := (msg).(map[string]interface{})
		err := config.Update(func(updated *config.Config) error {

			if autoReport, ok := settings["autoReport"].(bool); ok {
				baseSettings.AutoReport = autoReport
				*updated.AutoReport = autoReport
			} else if proxyAll, ok := settings["proxyAll"].(bool); ok {
				baseSettings.ProxyAll = proxyAll
				updated.Client.ProxyAll = proxyAll
			} else if autoLaunch, ok := settings["autoLaunch"].(bool); ok {
				launcher.CreateLaunchFile(autoLaunch)
				baseSettings.AutoLaunch = autoLaunch
				*updated.AutoLaunch = autoLaunch
			}
			return nil
		})
		if err != nil {
			log.Errorf("Unable to update settings: %v", err)
		}
	}
}
