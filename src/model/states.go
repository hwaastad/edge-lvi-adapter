package model

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/hwaastad/edge-lvi-adapter/utils"
	log "github.com/sirupsen/logrus"
)

type States struct {
	path         string
	LogFile      string `json:"log_file"`
	LogLevel     string `json:"log_level"`
	LogFormat    string `json:"log_format"`
	WorkDir      string `json:"-"`
	ConfiguredAt string `json:"configuret_at"`
	ConfiguredBy string `json:"configures_by"`

	HomeCollection   []interface{}
	RoomCollection   []interface{}
	DeviceCollection []interface{}
}

func NewStates(workDir string) *States {
	state := &States{WorkDir: workDir}
	state.path = filepath.Join(workDir, "data", "state.json")
	if !utils.FileExists(state.path) {
		log.Info("State file doesn't exist.Loading default state")
		defaultStateFile := filepath.Join(workDir, "defaults", "state.json")
		err := utils.CopyFile(defaultStateFile, state.path)
		if err != nil {
			fmt.Print(err)
			panic("Can't copy state file.")
		}
	}
	return state
}

func (st *States) LoadFromFile() error {
	stateFileBody, err := ioutil.ReadFile(st.path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(stateFileBody, st)
	if err != nil {
		return err
	}
	return nil
}

func (st *States) SaveToFile() error {
	st.ConfiguredBy = "auto"
	st.ConfiguredAt = time.Now().Format(time.RFC3339)
	bpayload, err := json.Marshal(st)
	err = ioutil.WriteFile(st.path, bpayload, 0664)
	if err != nil {
		return err
	}
	return err
}

func (st *States) GetDataDir() string {
	return filepath.Join(st.WorkDir, "data")
}

func (st *States) GetDefaultDir() string {
	return filepath.Join(st.WorkDir, "defaults")
}

func (st *States) LoadDefaults() error {
	stateFile := filepath.Join(st.WorkDir, "data", "state.json")
	os.Remove(stateFile)
	log.Info("State file doesn't exist.Loading default state")
	defaultStateFile := filepath.Join(st.WorkDir, "defaults", "state.json")
	return utils.CopyFile(defaultStateFile, stateFile)
}

func (st *States) IsConfigured() bool {
	// TODO : Add logic here
	// I need to save AccessToken, RefreshToken, ExpireTime, RefreshExpireTime, HomeCollection, RoomCollection, DeviceCollection, IndependentDeviceCollection
	// if (cf.Auth.AccessToken && cf.Auth.RefreshToken && cf.Auth.ExpireTime && cf.Auth.RefreshExpireTime && cf.HomeCollection && cf.RoomCollection && cf.DeviceCollection && cf.IndependentDeviceCollection) != "" {
	// 	return true
	// }
	return true
}

type StateReport struct {
	OpStatus string    `json:"op_status"`
	AppState AppStates `json:"app_state"`
}

func (st *States) FindDeviceFromDeviceID(addr string) (index int, err error) {
	// cf.LoadFromFile()

	for i := 0; i < len(st.DeviceCollection); i++ {
		val := reflect.ValueOf(st.DeviceCollection[i])
		deviceId := strconv.FormatInt(val.FieldByName("device_id").Interface().(int64), 10)
		if deviceId == addr {
			index = i
			return index, nil
		}
	}
	index = 9999 // using err did not work
	log.Debug(err)
	return index, err
}
