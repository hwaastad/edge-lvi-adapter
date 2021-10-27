package router

import (
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"strings"

	"github.com/futurehomeno/fimpgo"
	log "github.com/sirupsen/logrus"

	mill "github.com/futurehomeno/mill/millapi"
	"github.com/futurehomeno/mill/model"
)

type FromFimpRouter struct {
	inboundMsgCh fimpgo.MessageCh
	mqt          *fimpgo.MqttTransport
	instanceID   string
	appLifecycle *model.Lifecycle
	configs      *model.Configs
	states       *model.States
}

type ListReportRecord struct {
	Address        string `json:"address"`
	Alias          string `json:"alias"`
	WakeupInterval string `json:"wakeup_int"`
	PowerSource    string `json:"power_source"`
}

func NewFromFimpRouter(mqt *fimpgo.MqttTransport, appLifecycle *model.Lifecycle, configs *model.Configs, states *model.States) *FromFimpRouter {
	fc := FromFimpRouter{inboundMsgCh: make(fimpgo.MessageCh, 5), mqt: mqt, appLifecycle: appLifecycle, configs: configs, states: states}
	fc.mqt.RegisterChannel("ch1", fc.inboundMsgCh)
	return &fc
}

func (fc *FromFimpRouter) Start() {

	// TODO: Choose either adapter or app topic

	// ------ Adapter topics ---------------------------------------------
	fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:dev/rn:%s/ad:1/#", model.ServiceName))
	fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:ad/rn:%s/ad:1", model.ServiceName))
	fc.mqt.Subscribe("pt:j1/mt:evt/rt:cloud/rn:auth-api/ad:1")

	// ------ Application topic -------------------------------------------
	//fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:app/rn:%s/ad:1",model.ServiceName))

	go func(msgChan fimpgo.MessageCh) {
		for {
			select {
			case newMsg := <-msgChan:
				fc.routeFimpMessage(newMsg)
			}
		}
	}(fc.inboundMsgCh)
}

func (fc *FromFimpRouter) routeFimpMessage(newMsg *fimpgo.Message) {
	config := mill.Config{}
	client := mill.Client{}
	ns := model.NetworkService{}

	if fc.configs.IsConfigured() {
		fc.appLifecycle.SetConnectionState(model.ConnStateConnected)
		fc.appLifecycle.SetConfigState(model.ConfigStateConfigured)
	} else {
		fc.appLifecycle.SetConfigState(model.ConfigStateNotConfigured)
		fc.appLifecycle.SetConnectionState(model.ConnStateDisconnected)
	}

	// Get new tokens if expires_in is exceeded. expireTime lasts for two hours, refreshExpireTime lasts for 30 days.
	if fc.configs.Auth.ExpireTime != 0 {
		millis := time.Now().UnixNano() / 1000000
		if millis > fc.configs.Auth.ExpireTime && millis < fc.configs.Auth.RefreshExpireTime {
			var err error
			accessToken, refreshToken, expireTime, refreshExpireTime, err := config.RefreshToken(fc.configs.Auth.RefreshToken)
			if err == nil {
				fc.configs.Auth.AccessToken = accessToken
				fc.configs.Auth.RefreshToken = refreshToken
				fc.configs.Auth.ExpireTime = expireTime
				fc.configs.Auth.RefreshExpireTime = refreshExpireTime
			} else {
				fc.configs.Auth.ExpireTime = 1
			}
			fc.states.SaveToFile()
		} else if millis > fc.configs.Auth.RefreshExpireTime {
			log.Error("30 day refreshExpireTime has expired. Restard adapter or send cmd.auth.login")
		}
	}

	// Update home- room- and devicelists
	fc.states.DeviceCollection, fc.states.RoomCollection, fc.states.HomeCollection, fc.states.IndependentDeviceCollection = nil, nil, nil, nil
	fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection = client.UpdateLists(fc.configs.Auth.AccessToken, fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection)
	fc.states.SaveToFile()
	log.Debug(" ")
	log.Debug("New fimp msg")
	addr := strings.Replace(newMsg.Addr.ServiceAddress, "_0", "", 1)
	switch newMsg.Payload.Service {
	case "thermostat":
		log.Debug("Service: thermostat")
		addr = strings.Replace(addr, "l", "", 1)
		switch newMsg.Payload.Type {
		case "cmd.setpoint.set":
			val, _ := newMsg.Payload.GetStrMapValue()
			valFloat, err := strconv.ParseFloat(val["temp"], 64)
			if err != nil {
				// Handle error
				log.Error("Could not convert to float, something wrong in setpoint value. Declining request, value: ", val["temp"], ", error: ", err)
				break
			}

			newTemp := strconv.Itoa(int(math.Ceil(valFloat)))
			deviceID := addr
			if config.TempControl(fc.configs.Auth.AccessToken, deviceID, newTemp) {
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.setpoint.report", "thermostat", fimpgo.VTypeStrMap, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
				log.Info("Temperature setpoint updated, new setpoint ", newTemp)
			} else {
				log.Error("Something went wrong when changing temperature")
			}

		case "cmd.setpoint.get_report":
			// You can ONLY get setpoint_report from devices that are independent(!). All devices have "holiday_temp" attribute, which for some reason is set temp on independent devices.
			// Will always be 0 if it is not an independent device.
			deviceIndex, err := fc.states.FindDeviceFromDeviceID(addr)
			if err != nil {
				log.Error(fmt.Errorf("Can't find device from deviceID, error: ", err))
			}
			device := reflect.ValueOf(fc.states.DeviceCollection[deviceIndex])
			setpointTemp := strconv.FormatInt(device.FieldByName("SetpointTemp").Interface().(int64), 10)

			if setpointTemp != "0" {
				val := map[string]interface{}{
					"type": "heat",
					"temp": setpointTemp,
					"unit": "C",
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.setpoint.report", "thermostat", fimpgo.VTypeStrMap, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.mode.set":
			val, _ := newMsg.Payload.GetStringValue()
			log.Debug("Trying to set new mode: ", val)

			deviceIndex, err := fc.states.FindDeviceFromDeviceID(addr)
			if err != nil {
				log.Error(fmt.Errorf("Can't find device from deviceID, error: ", err))
			}
			device := reflect.ValueOf(fc.states.DeviceCollection[deviceIndex])
			currentSetTemp := device.FieldByName("SetpointTemp").Interface().(int64)
			log.Debug("setpointTemp: ", currentSetTemp)

			if config.ModeControl(fc.configs.Auth.AccessToken, addr, currentSetTemp, val) {
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.mode.report", "thermostat", fimpgo.VTypeString, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
				log.Info("Mode updated, new mode: ", val)
			} else {
				log.Error("Something went wrong when changing mode")
			}
			// Do we need this? Will/should allways be heat

		case "cmd.mode.get_report":
			val := "heat"

			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}
			msg := fimpgo.NewMessage("evt.mode.report", "thermostat", fimpgo.VTypeString, val, nil, nil, newMsg.Payload)
			fc.mqt.Publish(adr, msg)
		}

	case "sensor_temp":
		log.Debug("Service: sensor_temp")
		addr = strings.Replace(addr, "l", "", 1)
		switch newMsg.Payload.Type {
		case "cmd.sensor.get_report":
			deviceIndex, err := fc.states.FindDeviceFromDeviceID(addr)
			if err != nil {
				// handle err
				log.Error(fmt.Errorf("Can't find device from deviceID, error: ", err))
			}
			device := reflect.ValueOf(fc.states.DeviceCollection[deviceIndex])
			currentTemp := device.FieldByName("CurrentTemp").Interface().(float32)

			val := currentTemp
			props := fimpgo.Props{}
			props["unit"] = "C"

			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "sensor_temp", ServiceAddress: addr}
			msg := fimpgo.NewMessage("evt.sensor.report", "sensor_temp", fimpgo.VTypeFloat, val, props, nil, newMsg.Payload)
			fc.mqt.Publish(adr, msg)
		}

	case model.ServiceName:

		log.Debug("New payload type ", newMsg.Payload.Type)
		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: model.ServiceName, ResourceAddress: "1"}
		switch newMsg.Payload.Type {

		case "cmd.auth.login":
			newadr, msg, err := fc.configs.GetHubToken(newMsg)
			if err != nil {
				log.Error("Something went wrong when getting hub token")
			} else {
				fc.mqt.Publish(newadr, msg)
			}

			fc.configs.UID = newMsg.Payload.UID

		case "cmd.auth.set_tokens":
			if fc.configs.Auth.AuthorizationCode != "" {
				fc.configs.Auth.AccessToken, fc.configs.Auth.RefreshToken, fc.configs.Auth.ExpireTime, fc.configs.Auth.RefreshExpireTime = config.NewClient(fc.configs.Auth.AuthorizationCode, fc.configs.Password, fc.configs.Username)
				fc.configs.Username = ""
				fc.configs.Password = ""
				fc.configs.SaveToFile()
				fc.states.SaveToFile()
			} else {
			}

			if fc.configs.Auth.AccessToken != "" {
				fc.appLifecycle.SetAuthState(model.AuthStateAuthenticated)
				log.Debug("All tokens received and saved.")
				loginval := map[string]interface{}{
					"errors":  nil,
					"success": true,
				}
				newadr, err := fimpgo.NewAddressFromString("pt:j1/mt:rsp/rt:cloud/rn:remote-client/ad:smarthome-app")
				if err != nil {
					log.Debug("Could not make login response topic")
				}
				msg := fimpgo.NewMessage("evt.pd7.response", "vinculum", fimpgo.VTypeObject, loginval, nil, nil, newMsg.Payload)
				msg.CorrelationID = fc.configs.UID
				fc.mqt.Publish(newadr, msg)
			} else {
				fc.appLifecycle.SetAuthState(model.AuthStateNotAuthenticated)
				log.Info("Login failed, please try again")
				loginval := map[string]interface{}{
					"errors":  "Wrong username or password",
					"success": false,
				}
				newadr, err := fimpgo.NewAddressFromString("pt:j1/mt:rsp/rt:cloud/rn:remote-client/ad:smarthome-app")
				if err != nil {
					log.Debug("Could not make login response topic")
				}
				msg := fimpgo.NewMessage("evt.pd7.response", "vinculum", fimpgo.VTypeObject, loginval, nil, nil, newMsg.Payload)
				msg.CorrelationID = fc.configs.UID
				fc.mqt.Publish(newadr, msg)
			}

			msg := fimpgo.NewMessage("evt.auth.status_report", model.ServiceName, fimpgo.VTypeObject, fc.appLifecycle.GetAllStates(), nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

			// Delete previously saved nodes, if there are any for some reason
			fc.states.DeviceCollection, fc.states.RoomCollection, fc.states.HomeCollection, fc.states.IndependentDeviceCollection = nil, nil, nil, nil
			fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection = client.UpdateLists(fc.configs.Auth.AccessToken, fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection)

			msg = fimpgo.NewMessage("evt.network.get_all_nodes_report", model.ServiceName, fimpgo.VTypeObject, fc.states.DeviceCollection, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

			for i := 0; i < len(fc.states.DeviceCollection); i++ {
				inclReport := ns.SendInclusionReport(i, fc.states.DeviceCollection)

				msg := fimpgo.NewMessage("evt.thing.inclusion_report", "mill", fimpgo.VTypeObject, inclReport, nil, nil, nil)
				adr := fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "mill", ResourceAddress: "1"}
				fc.mqt.Publish(&adr, msg)
			}
			fc.configs.SaveToFile()
			fc.states.SaveToFile()

		case "cmd.auth.logout":
			fc.configs.Auth.AccessToken = ""
			fc.appLifecycle.SetConfigState(model.ConfigStateNotConfigured)
			fc.appLifecycle.SetAuthState(model.AuthStateNotAuthenticated)
			fc.appLifecycle.SetConnectionState(model.ConnStateDisconnected)
			for i := 0; i < len(fc.states.DeviceCollection); i++ {
				device := reflect.ValueOf(fc.states.DeviceCollection[i])
				deviceID := strconv.FormatInt(device.FieldByName("DeviceID").Interface().(int64), 10)
				val := map[string]interface{}{
					"address": deviceID,
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "mill", ResourceAddress: "1"}
				msg := fimpgo.NewMessage("evt.thing.exclusion_report", "mill", fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

			fc.states.DeviceCollection, fc.states.RoomCollection, fc.states.HomeCollection, fc.states.IndependentDeviceCollection = nil, nil, nil, nil
			fc.configs.LoadDefaults()
			fc.states.LoadDefaults()

			val2 := map[string]interface{}{
				"errors":  nil,
				"success": true,
			}
			msg := fimpgo.NewMessage("evt.pd7.response", "vinculum", fimpgo.VTypeObject, val2, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				log.Error("Could not respond to wanted request")
			}
			log.Info("Logged out and deleted all devices.")

		case "cmd.network.get_all_nodes":
			// This case saves all homes, rooms and devices, but only sends devices back to fimp.
			fc.states.DeviceCollection, fc.states.RoomCollection, fc.states.HomeCollection, fc.states.IndependentDeviceCollection = nil, nil, nil, nil
			fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection = client.UpdateLists(fc.configs.Auth.AccessToken, fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection)
			report := []ListReportRecord{}
			if len(fc.states.DeviceCollection) == 0 {
				fmt.Errorf("There are no devices")
				return
			}
			for i := 0; i < len(fc.states.DeviceCollection); i++ {
				device := reflect.ValueOf(fc.states.DeviceCollection[i])
				deviceID := strconv.FormatInt(device.FieldByName("DeviceID").Interface().(int64), 10)
				name := device.FieldByName("DeviceName").Interface().(string)
				rec := ListReportRecord{Address: deviceID, Alias: "Mill " + name, PowerSource: "ac", WakeupInterval: "-1"}
				report = append(report, rec)
			}

			msg := fimpgo.NewMessage("evt.network.get_all_nodes_report", model.ServiceName, fimpgo.VTypeObject, report, nil, nil, newMsg.Payload)
			msg.Source = "mill"
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}
			fc.states.SaveToFile()

		case "cmd.system.sync":

			// only
			fc.states.DeviceCollection, fc.states.RoomCollection, fc.states.HomeCollection, fc.states.IndependentDeviceCollection = nil, nil, nil, nil
			fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection = client.UpdateLists(fc.configs.Auth.AccessToken, fc.states.HomeCollection, fc.states.RoomCollection, fc.states.DeviceCollection, fc.states.IndependentDeviceCollection)
			log.Debug(fc.configs.Auth.AccessToken)

			for i := 0; i < len(fc.states.DeviceCollection); i++ {
				inclReport := ns.SendInclusionReport(i, fc.states.DeviceCollection)

				msg := fimpgo.NewMessage("evt.thing.inclusion_report", "mill", fimpgo.VTypeObject, inclReport, nil, nil, newMsg.Payload)
				adr := fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "mill", ResourceAddress: "1"}
				fc.mqt.Publish(&adr, msg)
			}

			val2 := model.ButtonActionResponse{
				Operation:       "cmd.system.sync",
				OperationStatus: "ok",
				Next:            "reload",
				ErrorCode:       "",
				ErrorText:       "",
			}

			msg := fimpgo.NewMessage("evt.app.config_action_report", model.ServiceName, fimpgo.VTypeObject, val2, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				log.Error("Could not respond to wanted request")
			}
			log.Info("All devices synced")

		case "cmd.system.set_poll_time":
			log.Debug("pollTime case")

		case "cmd.app.get_manifest":
			mode, err := newMsg.Payload.GetStringValue()
			if err != nil {
				log.Error("Incorrect request format ")
				return
			}
			manifest := model.NewManifest()
			err = manifest.LoadFromFile(filepath.Join(fc.configs.GetDefaultDir(), "app-manifest.json"))
			if err != nil {
				log.Error("Failed to load manifest file .Error :", err.Error())
				return
			}
			if mode == "manifest_state" {
				manifest.AppState = *fc.appLifecycle.GetAllStates()
				fc.configs.ConnectionState = string(fc.appLifecycle.ConnectionState())
				fc.configs.Errors = fc.appLifecycle.LastError()
				manifest.ConfigState = fc.configs
			}
			if errConf := manifest.GetAppConfig("errors"); errConf != nil {
				if fc.configs.Errors == "" {
					errConf.Hidden = true
				} else {
					errConf.Hidden = false
				}
			}

			connectButton := manifest.GetButton("connect")
			disconnectButton := manifest.GetButton("disconnect")
			if connectButton != nil && disconnectButton != nil {
				if fc.appLifecycle.ConnectionState() == model.ConnStateConnected {
					connectButton.Hidden = true
					disconnectButton.Hidden = false
				} else {
					connectButton.Hidden = false
					disconnectButton.Hidden = true
				}
			}
			if syncButton := manifest.GetButton("sync"); syncButton != nil {
				if fc.appLifecycle.ConnectionState() == model.ConnStateConnected {
					syncButton.Hidden = false
				} else {
					syncButton.Hidden = false
				}
			}
			pollTimeBlock := manifest.GetUIBlock("poll_time_min")
			if pollTimeBlock != nil {
				pollTimeBlock.Hidden = false
			}
			settingsBlock := manifest.GetUIBlock("settings")
			if settingsBlock != nil {
				settingsBlock.Hidden = false
			}
			msg := fimpgo.NewMessage("evt.app.manifest_report", model.ServiceName, fimpgo.VTypeObject, manifest, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.app.get_state":
			msg := fimpgo.NewMessage("evt.app.manifest_report", model.ServiceName, fimpgo.VTypeObject, fc.appLifecycle.GetAllStates(), nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.config.get_extended_report":

			msg := fimpgo.NewMessage("evt.config.extended_report", model.ServiceName, fimpgo.VTypeObject, fc.configs, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.config.extended_set":
			conf := model.Configs{}
			err := newMsg.Payload.GetObjectValue(&conf)
			if err != nil {
				// TODO: This is an example . Add your logic here or remove
				log.Error("Can't parse configuration object")
				return
			}
			pollTimeMin := conf.PollTimeMin
			_, err = strconv.Atoi(pollTimeMin)

			if err != nil {
				log.Error(fmt.Sprintf("%q is not a number or contains illegal symbols.", pollTimeMin))
			} else {
				fc.configs.PollTimeMin = pollTimeMin
				fc.configs.SaveToFile()
				log.Info("App reconfigured, new configs: ", fc.configs)
				// TODO: This is an example . Add your logic here or remove
			}

			configReport := model.ConfigReport{
				OpStatus: "ok",
				AppState: *fc.appLifecycle.GetAllStates(),
			}
			msg := fimpgo.NewMessage("evt.app.config_report", model.ServiceName, fimpgo.VTypeObject, configReport, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.log.set_level":
			// Configure log level
			level, err := newMsg.Payload.GetStringValue()
			if err != nil {
				return
			}
			logLevel, err := log.ParseLevel(level)
			if err == nil {
				log.SetLevel(logLevel)
				fc.configs.LogLevel = level
				fc.configs.SaveToFile()
				fc.states.SaveToFile()
			}
			log.Info("Log level updated to = ", logLevel)

		case "cmd.system.reconnect":
			// This is optional operation.
			fc.appLifecycle.PublishEvent(model.EventConfigured, "from-fimp-router", nil)

			val := model.ButtonActionResponse{
				Operation:       "cmd.system.reconnect",
				OperationStatus: "ok",
				Next:            "config",
				ErrorCode:       "",
				ErrorText:       "",
			}
			msg := fimpgo.NewMessage("evt.app.config_action_report", model.ServiceName, fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.app.factory_reset":
			val := model.ButtonActionResponse{
				Operation:       "cmd.app.factory_reset",
				OperationStatus: "ok",
				Next:            "config",
				ErrorCode:       "",
				ErrorText:       "",
			}
			fc.appLifecycle.SetConfigState(model.ConfigStateNotConfigured)
			fc.appLifecycle.SetAppState(model.AppStateNotConfigured, nil)
			fc.appLifecycle.SetAuthState(model.AuthStateNotAuthenticated)
			msg := fimpgo.NewMessage("evt.app.config_action_report", model.ServiceName, fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.thing.get_inclusion_report":
			deviceID, err := newMsg.Payload.GetStringValue()
			if err != nil {
				// handle err
				log.Error(fmt.Errorf("Can't get strValue, error: ", err))
			}
			nodeID, err := fc.states.FindDeviceFromDeviceID(deviceID)
			if err != nil { // normal error handling did not work for some reason, find out why
				// handle error
				log.Error("error") // this never executes
			}
			if nodeID != 9999 { // using this method instead
				inclReport := ns.SendInclusionReport(nodeID, fc.states.DeviceCollection)

				msg := fimpgo.NewMessage("evt.thing.inclusion_report", "mill", fimpgo.VTypeObject, inclReport, nil, nil, nil)
				adr := fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "mill", ResourceAddress: "1"}
				fc.mqt.Publish(&adr, msg)
			}

		case "cmd.thing.inclusion":
			//flag , _ := newMsg.Payload.GetBoolValue()
			// TODO: This is an example . Add your logic here or remove
		case "cmd.thing.delete":
			// remove device from network
			val, err := newMsg.Payload.GetStrMapValue()
			if err != nil {
				log.Error("Wrong msg format")
				return
			}
			deviceID := val["address"]
			deviceExists, err := fc.states.FindDeviceFromDeviceID(deviceID)
			if deviceExists != 9999 {
				val := map[string]interface{}{
					"address": deviceID,
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "mill", ResourceAddress: "1"}
				msg := fimpgo.NewMessage("evt.thing.exclusion_report", "mill", fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
				log.Info("Device with deviceID: ", deviceID, " has been removed from network.")
			}

		case "cmd.app.uninstall":
			for i := 0; i < len(fc.states.DeviceCollection); i++ {
				device := reflect.ValueOf(fc.states.DeviceCollection[i])
				deviceID := strconv.FormatInt(device.FieldByName("DeviceID").Interface().(int64), 10)
				val := map[string]interface{}{
					"address": deviceID,
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "mill", ResourceAddress: "1"}
				msg := fimpgo.NewMessage("evt.thing.exclusion_report", "mill", fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}
		}

	case "auth-api":
		fc.configs.Auth.AuthorizationCode, fc.configs.HubToken = config.GetAuthCode(newMsg)

		msg := fimpgo.NewMessage("cmd.auth.set_tokens", model.ServiceName, fimpgo.VTypeString, "", nil, nil, newMsg.Payload)
		newadr, err := fimpgo.NewAddressFromString("pt:j1/mt:cmd/rt:ad/rn:mill/ad:1")
		if err != nil {
			log.Debug(err)
		}
		fc.mqt.Publish(newadr, msg)
	}
}
