package main

import (
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/futurehomeno/fimpgo"
	"github.com/futurehomeno/fimpgo/discovery"
	"github.com/futurehomeno/fimpgo/edgeapp"

	lvi "github.com/hwaastad/edge-lvi-adapter/lviapi"
	"github.com/hwaastad/edge-lvi-adapter/model"
	"github.com/hwaastad/edge-lvi-adapter/router"
	"github.com/hwaastad/edge-lvi-adapter/utils"
	log "github.com/sirupsen/logrus"
)

func main() {
	var workDir string
	flag.StringVar(&workDir, "c", "", "Work dir")
	flag.Parse()
	if workDir == "" {
		workDir = "./"
	} else {
		fmt.Println("Work dir ", workDir)
	}
	appLifecycle := model.NewAppLifecycle()
	configs := model.NewConfigs(workDir)
	states := model.NewStates(workDir)
	err := configs.LoadFromFile()
	if err != nil {
		fmt.Print(err)
		panic("Can't load config file.")
	}
	err = states.LoadFromFile()
	if err != nil {
		fmt.Print(err)
		panic("Can't load state file.")
	}
	client := lvi.Client{}
	config := lvi.Config{}

	utils.SetupLog(configs.LogFile, configs.LogLevel, configs.LogFormat)
	log.Info("--------------Starting lvi----------------")
	log.Info("Work directory : ", configs.WorkDir)
	appLifecycle.PublishEvent(model.EventConfiguring, "main", nil)

	mqtt := fimpgo.NewMqttTransport(configs.MqttServerURI, configs.MqttClientIdPrefix, configs.MqttUsername, configs.MqttPassword, true, 1, 1)
	err = mqtt.Start()
	responder := discovery.NewServiceDiscoveryResponder(mqtt)
	responder.RegisterResource(model.GetDiscoveryResource())
	responder.Start()

	fimpRouter := router.NewFromFimpRouter(mqtt, appLifecycle, configs, states)
	fimpRouter.Start()

	appLifecycle.SetConnectionState(model.ConnStateDisconnected)
	if configs.IsConfigured() && err == nil {
		appLifecycle.SetConfigState(model.ConfigStateConfigured)
		appLifecycle.SetAppState(model.AppStateRunning, nil)
		appLifecycle.SetConnectionState(model.ConnStateConnected)
	} else {
		appLifecycle.SetConfigState(model.ConfigStateNotConfigured)
		appLifecycle.SetAppState(model.AppStateNotConfigured, nil)
		appLifecycle.SetConnectionState(model.ConnStateDisconnected)
	}

	if configs.IsAuthenticated() && err == nil {
		appLifecycle.SetAuthState(model.AuthStateAuthenticated)
	} else {
		appLifecycle.SetAuthState(model.AuthStateNotAuthenticated)
	}
	//------------------ Sample code --------------------------------------
	if err := edgeapp.NewSystemCheck().WaitForInternet(5 * time.Minute); err == nil {
		log.Info("<main> Internet connection - OK")
	} else {
		log.Error("<main> Internet connection - ERROR")
	}

	if err != nil {
		log.Error("Can't connect to broker. Error:", err.Error())
	} else {
		log.Info("Connected")
	}
	appLifecycle.SetAppState(model.AppStateRunning, nil)
	//------------------ Sample code --------------------------------------
	PollString := configs.PollTimeMin
	PollTime, err := strconv.Atoi(PollString)
	for {
		appLifecycle.WaitForState("main", model.AppStateRunning)
		log.Info("Starting ticker")
		ticker := time.NewTicker(time.Duration(PollTime) * time.Minute)
		for ; true; <-ticker.C {
			if configs.Auth.ExpireTime != 0 {
				log.Debug("Checking expireTime")
				millis := time.Now().UnixNano() / 1000000
				if millis > configs.Auth.ExpireTime && millis < configs.Auth.RefreshExpireTime {
					log.Debug("Trying to set new tokens")
					accessToken, refreshToken, expireTime, refreshExpireTime, err := config.RefreshToken(configs.Auth.RefreshToken)
					log.Debug(err)
					if err == nil {
						configs.Auth.AccessToken = accessToken
						configs.Auth.RefreshToken = refreshToken
						configs.Auth.ExpireTime = expireTime
						configs.Auth.RefreshExpireTime = refreshExpireTime
						appLifecycle.SetConnectionState(model.ConnStateConnected)
					} else {
						configs.Auth.ExpireTime = 1
						appLifecycle.SetConnectionState(model.ConnStateDisconnected)
					}
					states.SaveToFile()
					configs.SaveToFile()
				} else if millis > configs.Auth.RefreshExpireTime {
					log.Error("30 day refreshExpireTime has expired. Restard adapter or send cmd.auth.login")
				} else {
					log.Debug("expiretime is OK")
				}
			}
			states.DeviceCollection, states.RoomCollection, states.HomeCollection = nil, nil, nil
			states.HomeCollection, states.RoomCollection, states.DeviceCollection = client.UpdateLists(configs.Auth.AccessToken, states.HomeCollection, states.RoomCollection, states.DeviceCollection)

			for i := 0; i < len(states.DeviceCollection); i++ {
				device := reflect.ValueOf(states.DeviceCollection[i])
				deviceId := strconv.FormatInt(device.FieldByName("device_id").Interface().(int64), 10)
				currentTemp := device.FieldByName("temperature_air").Interface().(float64)
				tempVal := currentTemp
				props := fimpgo.Props{}
				props["unit"] = "C"

				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "sensor_temp", ServiceAddress: deviceId}
				msg := fimpgo.NewMessage("evt.sensor.report", "sensor_temp", fimpgo.VTypeFloat, tempVal, props, nil, nil)
				mqtt.Publish(adr, msg)

				// setpointTemp := strconv.FormatInt(device.FieldByName("SetpointTemp").Interface().(int64), 10)
				// setpointVal := map[string]interface{}{
				// 	"type": "heat",
				// 	"temp": setpointTemp,
				// 	"unit": "C",
				// }
				// if setpointTemp != "0" {
				// 	adr = &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: deviceId}
				// 	msg = fimpgo.NewMessage("evt.setpoint.report", "thermostat", fimpgo.VTypeStrMap, setpointVal, nil, nil, nil)
				// 	mqtt.Publish(adr, msg)
				// }
				// -----------------------------------------------------------------------------------------------
			}
			states.SaveToFile()
		}
		appLifecycle.WaitForState(model.AppStateNotConfigured, "main")
	}

	mqtt.Stop()
	time.Sleep(5 * time.Second)
}
