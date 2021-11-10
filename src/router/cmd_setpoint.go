package router

import (
	"math"
	"strconv"

	mill "github.com/futurehomeno/edge-mill-adapter/millapi"
	"github.com/futurehomeno/edge-mill-adapter/model"
	"github.com/futurehomeno/fimpgo"
	log "github.com/sirupsen/logrus"
)

func (fc *FromFimpRouter) setpointSet(oldMsg *fimpgo.Message, config mill.Config) {
	addr := oldMsg.Addr.ServiceAddress

	val, _ := oldMsg.Payload.GetStrMapValue()
	valFloat, err := strconv.ParseFloat(val["temp"], 64)
	if err != nil {
		log.Error("Could not convert to float, something wrong in setpoint value. Declining request, value: ", val["temp"], ", error: ", err)
		return
	}

	newTemp := strconv.Itoa(int(math.Ceil(valFloat)))
	deviceID := addr

	if err := config.TempControl(fc.configs.Auth.AccessToken, deviceID, newTemp); err != nil {
		log.Error("Something went wrong when changing temperature, err: ", err)
		return
	}

	adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "thermostat", ServiceAddress: addr}
	msg := fimpgo.NewMessage("evt.setpoint.report", "thermostat", fimpgo.VTypeStrMap, val, nil, nil, oldMsg.Payload)
	fc.mqt.Publish(adr, msg)
	log.Info("Temperature setpoint updated, new setpoint ", newTemp)
	return
}
