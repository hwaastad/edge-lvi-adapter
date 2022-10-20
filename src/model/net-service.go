package model

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/futurehomeno/fimpgo/fimptype"
)

type NetworkService struct {
}

func (ns *NetworkService) SendInclusionReport(nodeId int, DeviceCollection []interface{}) fimptype.ThingInclusionReport {
	var deviceId string
	// var err error

	var name, manufacturer string
	var deviceAddr string
	services := []fimptype.Service{}

	thermostatInterfaces := []fimptype.Interface{{
		Type:      "in",
		MsgType:   "cmd.setpoint.set",
		ValueType: "str_map",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.setpoint.report",
		ValueType: "str_map",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.setpoint.get_report",
		ValueType: "string",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.mode.set",
		ValueType: "string",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.mode.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.mode.report",
		ValueType: "string",
		Version:   "1",
	}}

	sensorInterfaces := []fimptype.Interface{{
		Type:      "in",
		MsgType:   "cmd.sensor.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.sensor.report",
		ValueType: "float",
		Version:   "1",
	}}

	thermostatService := fimptype.Service{
		Name:    "thermostat",
		Alias:   "thermostat",
		Address: "/rt:dev/rn:lvi/ad:1/sv:thermostat/ad:",
		Enabled: true,
		Groups:  []string{"ch_0"},
		Props: map[string]interface{}{
			"sup_modes":     []string{"off", "heat"},
			"sup_setpoints": []string{"heat"},
		},
		Interfaces: thermostatInterfaces,
	}

	tempSensorService := fimptype.Service{
		Name:    "sensor_temp",
		Alias:   "Temperature sensor",
		Address: "/rt:dev/rn:lvi/ad:1/sv:sensor_temp/ad:",
		Enabled: true,
		Groups:  []string{"ch_0"},
		Props: map[string]interface{}{
			"sup_units": []string{"C"},
		},
		Tags:             nil,
		PropSetReference: "",
		Interfaces:       sensorInterfaces,
	}

	device := DeviceCollection[nodeId]
	val := reflect.ValueOf(device)
	deviceId = strconv.FormatInt(val.FieldByName("device_id").Interface().(int64), 10)
	manufacturer = "lvi"
	name = val.FieldByName("nom_appareil").Interface().(string)
	serviceAddress := fmt.Sprintf("%s", deviceId)
	thermostatService.Address = thermostatService.Address + serviceAddress
	tempSensorService.Address = tempSensorService.Address + serviceAddress
	services = append(services, thermostatService, tempSensorService)
	deviceAddr = fmt.Sprintf("%s", deviceId)
	powerSource := "ac"

	inclReport := fimptype.ThingInclusionReport{
		IntegrationId:     "",
		Address:           deviceAddr,
		Type:              "",
		ProductHash:       manufacturer,
		CommTechnology:    "wifi",
		ProductName:       name,
		ManufacturerId:    manufacturer,
		DeviceId:          deviceId,
		HwVersion:         "1",
		SwVersion:         "1",
		PowerSource:       powerSource,
		WakeUpInterval:    "-1",
		Security:          "",
		Tags:              nil,
		Groups:            []string{"ch_0"},
		PropSets:          nil,
		TechSpecificProps: nil,
		Services:          services,
	}

	return inclReport
}
