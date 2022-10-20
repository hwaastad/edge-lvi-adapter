package model

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/futurehomeno/fimpgo"
	log "github.com/sirupsen/logrus"
	"github.com/waastad/edge-lvi-adapter/utils"
)

const ServiceName = "mill"

type Configs struct {
	path               string
	InstanceAddress    string `json:"instance_address"`
	MqttServerURI      string `json:"mqtt_server_uri"`
	MqttUsername       string `json:"mqtt_server_username"`
	MqttPassword       string `json:"mqtt_server_password"`
	MqttClientIdPrefix string `json:"mqtt_client_id_prefix"`
	LogFile            string `json:"log_file"`
	LogLevel           string `json:"log_level"`
	LogFormat          string `json:"log_format"`
	WorkDir            string `json:"-"`
	ConfiguredAt       string `json:"configured_at"`
	ConfiguredBy       string `json:"configured_by"`
	Param1             bool   `json:"param_1"`
	Param2             string `json:"param_2"`
	PollTimeMin        string `json:"poll_time_min"`

	Username string `json:"username"` // this should be moved
	Password string `json:"password"` // this should be moved

	Auth struct {
		AuthorizationCode string `json:"authorization_code"` // this should be moved
		AccessToken       string `json:"access_token"`       // this should be moved
		RefreshToken      string `json:"refresh_token"`      // this should be moved
		ExpireTime        int64  `json:"expireTime"`         // this should be moved
		RefreshExpireTime int64  `json:"refresh_expireTime"` // this should be moved
	}

	ConnectionState string `json:"connection_state"`
	Errors          string `json:"errors"`
	HubToken        string `json:"token"`
	UID             string `json:"uid"`
}

func NewConfigs(workDir string) *Configs {
	conf := &Configs{WorkDir: workDir}
	conf.path = filepath.Join(workDir, "data", "config.json")
	if !utils.FileExists(conf.path) {
		log.Info("Config file doesn't exist.Loading default config")
		defaultConfigFile := filepath.Join(workDir, "defaults", "config.json")
		err := utils.CopyFile(defaultConfigFile, conf.path)
		if err != nil {
			fmt.Print(err)
			panic("Can't copy config file.")
		}
	}
	return conf
}

func (cf *Configs) LoadFromFile() error {
	configFileBody, err := ioutil.ReadFile(cf.path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(configFileBody, cf)
	if err != nil {
		return err
	}
	return nil
}

func (cf *Configs) SaveToFile() error {
	cf.ConfiguredBy = "auto"
	cf.ConfiguredAt = time.Now().Format(time.RFC3339)
	bpayload, err := json.Marshal(cf)
	err = ioutil.WriteFile(cf.path, bpayload, 0664)
	if err != nil {
		return err
	}
	return err
}

func (cf *Configs) GetDataDir() string {
	return filepath.Join(cf.WorkDir, "data")
}

func (cf *Configs) GetDefaultDir() string {
	return filepath.Join(cf.WorkDir, "defaults")
}

func (cf *Configs) LoadDefaults() error {
	configFile := filepath.Join(cf.WorkDir, "data", "config.json")
	os.Remove(configFile)
	log.Info("Config file doesn't exist.Loading default config")
	defaultConfigFile := filepath.Join(cf.WorkDir, "defaults", "config.json")
	return utils.CopyFile(defaultConfigFile, configFile)
}

func (cf *Configs) IsConfigured() bool {
	if cf.Auth.AccessToken != "" {
		return true
	} else {
		return false
	}
}

func (cf *Configs) IsAuthenticated() bool {
	if cf.Auth.AuthorizationCode != "" {
		return true
	} else {
		return false
	}
}

type ConfigReport struct {
	OpStatus string    `json:"op_status"`
	AppState AppStates `json:"app_state"`
}

func (cf *Configs) GetHubToken(oldMsg *fimpgo.Message) (*fimpgo.Address, *fimpgo.FimpMessage, error) {
	// mqt := fimpgo.MqttTransport{}
	err := oldMsg.Payload.GetObjectValue(&cf)
	if err != nil {
		log.Error("Could not get object value")
		return nil, nil, err
	}
	if cf.Username != "" && cf.Password != "" {
		// Get hub token
		val := map[string]interface{}{
			"site_id":     "",
			"hub_id":      "",
			"auth_system": "heimdall",
		}
		msg := fimpgo.NewMessage("cmd.hub_auth.get_jwt", "auth-api", fimpgo.VTypeStrMap, val, nil, nil, nil)
		msg.Source = "clbridge"
		newadr, err := fimpgo.NewAddressFromString("pt:j1/mt:cmd/rt:cloud/rn:auth-api/ad:1")
		if err != nil {
			log.Debug("Could not send hub token request")
			return nil, nil, err
		}
		return newadr, msg, nil
	}
	return nil, nil, err
}
