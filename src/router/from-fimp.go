package router

import (
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	"strings"

	"github.com/futurehomeno/fimpgo"
	log "github.com/sirupsen/logrus"
	"github.com/thingsplex/mill/mill"
	"github.com/thingsplex/mill/model"
)

type FromFimpRouter struct {
	inboundMsgCh fimpgo.MessageCh
	mqt          *fimpgo.MqttTransport
	instanceID   string
	appLifecycle *model.Lifecycle
	configs      *model.Configs
}

func NewFromFimpRouter(mqt *fimpgo.MqttTransport, appLifecycle *model.Lifecycle, configs *model.Configs) *FromFimpRouter {
	fc := FromFimpRouter{inboundMsgCh: make(fimpgo.MessageCh, 5), mqt: mqt, appLifecycle: appLifecycle, configs: configs}
	fc.mqt.RegisterChannel("ch1", fc.inboundMsgCh)
	return &fc
}

func (fc *FromFimpRouter) Start() {

	// TODO: Choose either adapter or app topic

	// ------ Adapter topics ---------------------------------------------
	fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:dev/rn:%s/ad:1/#", model.ServiceName))
	fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:ad/rn:%s/ad:1", model.ServiceName))

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

	// Get new tokens if expires_in is exceeded. expireTime lasts for two hours, refreshExpireTime lasts for 30 days.
	if fc.configs.Auth.ExpireTime != 0 {
		millis := time.Now().UnixNano() / 1000000
		if millis > fc.configs.Auth.ExpireTime && millis < fc.configs.Auth.RefreshExpireTime {
			fc.configs.Auth.AccessToken, fc.configs.Auth.RefreshToken, fc.configs.Auth.ExpireTime, fc.configs.Auth.RefreshExpireTime = config.RefreshToken(fc.configs.Auth.RefreshToken)
		} else if millis > fc.configs.Auth.RefreshExpireTime {
			log.Debug("30 day refreshExpireTime has expired. Restard adapter or send cmd.auth.login")
		}
	}

	log.Debug("New fimp msg")
	addr := strings.Replace(newMsg.Addr.ServiceAddress, "_0", "", 1)
	switch newMsg.Payload.Service {
	case "thermostat":
		log.Debug("thermostat")
		addr = strings.Replace(addr, "l", "", 1)
		switch newMsg.Payload.Type {
		case "cmd.setpoint.set":
			// add logic
			// val, _ := newMsg.Payload.GetStrMapValue()
			// newTemp := val["temp"]
			// deviceID := addr
			// deviceIndex := fc.configs.FindDeviceFromDeviceID(msg)
			// log.Debug(addr)
			// log.Debug(newTemp)

			// Need online device to test how to use api to change temperature
			// do deviceControl(..) for device to change temperature to newTemp
			// deviceControl(fc.configs.Auth.AccessToken, addr)

			// update devicelist, or room list ??? and read new actual set temperature from mill app
			// fc.configs.DeviceCollection, fc.configs.Roomollection = nil, nil

			// msg := fimpgo.NewMessage("evt.setpoint.report", model.ServiceName, fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
			// if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
			// 	// if response topic is not set , sending back to default application event topic
			// 	fc.mqt.Publish(adr, msg)
			// }

		case "cmd.setpoint.get_report":

			// THIS RETURNS CURRENT TEMP, NOT WANTED TEMP, what is wanted temp in device struct? Need online device to test

			deviceIndex, err := fc.configs.FindDeviceFromDeviceID(addr)
			if err != nil {
				// handle err
			}
			device := reflect.ValueOf(fc.configs.DeviceCollection[deviceIndex])
			currentTemp := fmt.Sprintf("%.2f", device.FieldByName("CurrentTemp").Interface())

			val := map[string]string{
				"type": "heat",
				"temp": currentTemp,
				"unit": "C",
			}

			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}

			msg := fimpgo.NewMessage("evt.setpoint.report", "thermostat", fimpgo.VTypeStrMap, val, nil, nil, newMsg.Payload)

			fc.mqt.Publish(adr, msg)
		case "cmd.mode.set":
			// Do we need this? Will/should allways be heat

		case "cmd.mode.get_report":
			val := "heat"
			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}

			msg := fimpgo.NewMessage("evt.mode.report", "thermostat", fimpgo.VTypeString, val, nil, nil, newMsg.Payload)

			fc.mqt.Publish(adr, msg)
		}

	case "sensor_temp":
		log.Debug("sensor_temp")
		addr = strings.Replace(addr, "l", "", 1)
		switch newMsg.Payload.Type {
		case "cmd.sensor.get_report":
			deviceIndex, err := fc.configs.FindDeviceFromDeviceID(addr)
			if err != nil {
				// handle err
			}
			device := reflect.ValueOf(fc.configs.DeviceCollection[deviceIndex])
			currentTemp := device.FieldByName("CurrentTemp").Interface()

			val := currentTemp
			props := fimpgo.Props{}
			props["unit"] = "C"

			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "sensor_temp", ServiceAddress: addr}
			log.Debug(adr)

			msg := fimpgo.NewMessage("evt.sensor.report", "sensor_temp", fimpgo.VTypeFloat, val, props, nil, newMsg.Payload)

			fc.mqt.Publish(adr, msg)
		}

	case model.ServiceName:

		log.Debug("New payload type ", newMsg.Payload.Type)
		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: model.ServiceName, ResourceAddress: "1"}
		switch newMsg.Payload.Type {

		case "cmd.auth.login":
			fc.configs.Username = newMsg.Payload.Properties["username"]
			fc.configs.Password = newMsg.Payload.Properties["password"]
			fc.configs.AccessKey = newMsg.Payload.Properties["access_key"]
			fc.configs.SecretToken = newMsg.Payload.Properties["secret_token"]

			status := model.AuthStatus{
				Status:    "",
				ErrorText: "",
				ErrorCode: "",
			}

			if fc.configs.Username != "" && fc.configs.Password != "" && fc.configs.AccessKey != "" && fc.configs.SecretToken != "" {
				// Send api requests to get authorizationCode, accessToken, refreshToken, expireTime, refreshExpireTime
				fc.configs.Auth.AuthorizationCode, fc.configs.Auth.AccessToken, fc.configs.Auth.RefreshToken, fc.configs.Auth.ExpireTime, fc.configs.Auth.RefreshExpireTime = config.NewClient(fc.configs.AccessKey, fc.configs.SecretToken, fc.configs.Password, fc.configs.Username)
			} else {
				status.Status = "ERROR"
				status.ErrorText = "Empty username or password or access_key or secret_token"
				log.Debug(status.ErrorText)
			}
			if fc.configs.Auth.AuthorizationCode == "" {
				status.Status = model.AuthStateNotAuthenticated
				log.Debug("No authorization code received")
			} else {
				status.Status = model.AuthStateAuthenticated
			}
			if fc.configs.Auth.AccessToken != "" && fc.configs.Auth.RefreshToken != "" { // add some logic to check expire times as well
				log.Debug("All tokens received and saved.")
			} else {
				status.Status = "ERROR"
				status.ErrorText = "Empty accessToken or refreshToken"
				log.Debug(status.ErrorText)
			}
			log.Debug(fc.configs.Auth.AccessToken)

			msg := fimpgo.NewMessage("evt.auth.status_report", model.ServiceName, fimpgo.VTypeObject, status, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.network.get_all_nodes":
			// This case saves all homes, rooms and devices, but only sends devices back to fimp.
			// Delete previously saved nodes
			fc.configs.DeviceCollection, fc.configs.RoomCollection, fc.configs.HomeCollection = nil, nil, nil

			allDevices, allRooms, allHomes, allIndependentDevices, err := client.GetAllDevices(fc.configs.Auth.AccessToken)
			if err != nil {
				// handle err
			}
			for home := range allHomes {
				fc.configs.HomeCollection = append(fc.configs.HomeCollection, allHomes[home])
			}
			for room := range allRooms {
				fc.configs.RoomCollection = append(fc.configs.RoomCollection, allRooms[room])
			}
			for device := range allDevices {
				fc.configs.DeviceCollection = append(fc.configs.DeviceCollection, allDevices[device])
			}
			for device := range allIndependentDevices {
				fc.configs.IndependentDeviceCollection = append(fc.configs.IndependentDeviceCollection, allIndependentDevices[device])
			}

			msg := fimpgo.NewMessage("evt.network.get_all_nodes_report", model.ServiceName, fimpgo.VTypeObject, fc.configs.DeviceCollection, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

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
				manifest.ConfigState = fc.configs
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
			fc.configs.Param1 = conf.Param1
			fc.configs.Param2 = conf.Param2
			fc.configs.SaveToFile()
			log.Debugf("App reconfigured . New parameters : %v", fc.configs)
			// TODO: This is an example . Add your logic here or remove
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
			}
			log.Info("Log level updated to = ", logLevel)

		case "cmd.system.reconnect":
			// This is optional operation.
			fc.appLifecycle.PublishEvent(model.EventConfigured, "from-fimp-router", nil)
			//val := map[string]string{"status":status,"error":errStr}
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
			}
			log.Debug(deviceID)
			nodeID, err := fc.configs.FindDeviceFromDeviceID(deviceID)
			log.Debug(nodeID)
			if err != nil { // normal error handling did not work for some reason, find out why
				// handle error
				log.Debug("error") // this never executes
			}
			if nodeID != 9999 { // using this method instead
				inclReport := ns.SendInclusionReport(nodeID, fc.configs.DeviceCollection)

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
			deviceID, ok := val["address"]
			if ok {
				// TODO: This is an example . Add your logic here or remove
				log.Info(deviceID)
			} else {
				log.Error("Incorrect address")

			}
		}

	}
}
