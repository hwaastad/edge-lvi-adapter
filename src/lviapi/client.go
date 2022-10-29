package mill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/hwaastad/edge-lvi-adapter/model"

	"github.com/futurehomeno/fimpgo"
	"github.com/futurehomeno/fimpgo/utils"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultBaseURL is mill api url
	baseURL = "https://e3.lvi.eu/api/v0.1/human/"
	// applyAccessTokenURL is mill api to get access_token and refresh_token
	applyAccessTokenURL = baseURL + "share/applyAccessToken"
	// authURL is mill api to get authorization_code
	authURL = baseURL + "user/auth"
	// refreshURL is mill api to update access_token and refresh_token
	refreshURL = baseURL + "share/refreshtoken?refreshtoken="

	// deviceControlForOpenApiURL is mill api to controll individual devices
	deviceControlURL = baseURL + "uds/deviceControlForOpenApi"
	// getIndependentDevicesURL is mill api to get list of devices in unassigned room
	getIndependentDevicesURL = baseURL + "uds/getIndependentDevices2020"
	// selectDevicebyRoomURL is mill api to search device list by room
	selectDevicebyRoomURL = baseURL + "uds/selectDevicebyRoom2020"
	// selectHomeListURL is mill api to search housing list
	selectHomeListURL = baseURL + "uds/selectHomeList"
	// selectRoombyHomeURL is mill api to search room list by home
	selectRoombyHomeURL = baseURL + "uds/selectRoombyHome2020"
)

// Config is used to specify credential to Mill API
// AccessKey : Access Key from api registration at http://api.millheat.com. Key is sent to mail.
// SecretToken: Secret Token from api registration at http://api.millheat.com. Token is sent to mail.
// Username: Your mill app account username
// Password: Your mill app account password
type Config struct {
	ErrorCode  int    `json:"errorCode"`
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
	Success    bool   `json:"success"`

	Password    string `json:"password"`
	Username    string `json:"username"`
	SmartHomeId string `json:"smarthome_id"`

	Data struct {
		AuthorizationCode string `json:"authorization_code"`
		AccessToken       string `json:"access_token"`
		RefreshToken      string `json:"refresh_token"`
		ExpireTime        int64  `json:"expireTime"`
		RefreshExpireTime int64  `json:"refresh_expireTime"`
	} `json:"data"`
}

// Client to make request to Mill API
type Client struct {
	httpResponse *http.Response

	Data struct {
		Homes   []Home   `json:"homeList"`
		Rooms   []Room   `json:"roomList"`
		Devices []Device `json:"deviceList"`
	} `json:"data"`
}

// Device is a mill heater
type Device struct {
	id                     string
	device_id              string
	nom_appareil           string
	num_zonestring         string
	id_appareil            string
	current_temp           string
	consigne_confortstring string
	consigne_hg            string
	consigne_eco           string
	consigne_boost         string
	consigne_manuel        string
	min_set_point          string
	max_set_point          string
	date_start_boost       string
	time_boost             string
	nv_mode                string
	temperature_air        string
	temperature_sol        string
	power_status           string
	pourcent_light         string
	status_com             string
	recep_status_global    string
	gv_mode                string
	puissance_app          string
	smarthome_id           string
	bundle_id              string
	date_update            string
	heating_up             string
	heat_cool              string
	fan_speed              string
	available              string
	fan_status             string
}

type Home struct {
	smarthome_id string
	mac_address  string
	label        string
	general_mode string
	holiday_mode string
	sync_flag    string
	Rooms        []Room
}

type Room struct {
	zone_id          string
	name             string
	num_zone         string
	label_zone_type  string
	picto_zone_type  string
	zone_img_id      string
	address_position string
	Devices          []Device
}

// NewClient create a handle authentication to Mill API
func (config *Config) NewClient(authCode string, password string, username string, smarthome_id string) (string, string, int64, int64, string) {

	data := url.Values{}
	data.Set("email", username)
	data.Set("password", password)
	data.Set("token", authCode)
	data.Set("smarthome_id", smarthome_id)

	url := authURL
	req, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't post accessToken request, error: ", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, config)

	accessToken := config.Data.AccessToken
	refreshToken := config.Data.RefreshToken
	expireTime := config.Data.ExpireTime
	refreshExpireTime := config.Data.RefreshExpireTime
	smartHomeId := config.SmartHomeId

	if err != nil {
		return "", "", 0, 0, ""
	}
	defer resp.Body.Close()
	return accessToken, refreshToken, expireTime, refreshExpireTime, smartHomeId
}

func (config *Config) RefreshToken(refreshToken string) (string, string, int64, int64, error) {
	url := fmt.Sprintf("%s%s", refreshURL, refreshToken)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't post refreshToken request, error: ", err))
	}
	req.Header.Set("Accept", "*/*")

	resp, err := http.DefaultClient.Do(req)
	if processHTTPResponse(resp, err, config) != nil {
		return config.Data.AccessToken, config.Data.RefreshToken, config.Data.ExpireTime, config.Data.RefreshExpireTime, err
	}

	accessToken := config.Data.AccessToken
	newRefreshToken := config.Data.RefreshToken
	expireTime := config.Data.ExpireTime
	refreshExpireTime := config.Data.RefreshExpireTime

	defer resp.Body.Close()
	return accessToken, newRefreshToken, expireTime, refreshExpireTime, nil
}

func (c *Client) GetAllDevices(accessToken string) ([]Device, []Room, []Home, error) {
	homes, err := c.GetHomeList(accessToken)
	var allDevices []Device
	var allRooms []Room
	var allHomes []Home
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't get home list, error: ", err))
	}
	for home := range homes.Data.Homes {
		allHomes = append(allHomes, homes.Data.Homes[home])
		rooms, err := c.GetRoomList(accessToken, homes.Data.Homes[home].smarthome_id)
		if err != nil {
			// handle err
			log.Error(fmt.Errorf("Can't get room list, error: ", err))
		}
		for room := range rooms.Data.Rooms {
			allRooms = append(allRooms, rooms.Data.Rooms[room])
			devices, err := c.GetDeviceList(accessToken, rooms.Data.Rooms[room].zone_id)
			for device := range devices.Data.Devices {
				allDevices = append(allDevices, devices.Data.Devices[device])
			}
			if err != nil {
				// handle err
				log.Error(fmt.Errorf("Can't get device list, error: ", err))
			}
		}

		if err != nil {
			// handle err
			log.Error(fmt.Errorf("Can't get independent device list, error: ", err))
		}

	}
	return allDevices, allRooms, allHomes, nil
}

// GetHomeList sends curl request to get list of homes connected to user
func (c *Client) GetHomeList(accessToken string) (*Client, error) {
	req, err := http.NewRequest("POST", selectHomeListURL, nil)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't get home list, error: ", err))
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Access_token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, c)

	return c, nil
}

// GetRoomList sends curl request to get list of rooms by home
func (c *Client) GetRoomList(accessToken string, homeID string) (*Client, error) {
	url := fmt.Sprintf("%s%s%d", selectRoombyHomeURL, "?homeId=", homeID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't get room list, error: ", err))
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Access_token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, c)
	return c, nil
}

// GetDeviceList sends curl request to get list of devices by room
func (c *Client) GetDeviceList(accessToken string, roomID string) (*Client, error) {
	url := fmt.Sprintf("%s%s%d", selectDevicebyRoomURL, "?roomId=", roomID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Error(fmt.Errorf("Can't get device list, error: ", err))
		// handle err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Access_token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, c)
	return c, nil
}

func (cf *Config) TempControl(accessToken string, deviceId string, newTemp string) error {
	url := fmt.Sprintf("%s%s%s%s%s%s", deviceControlURL, "?deviceId=", deviceId, "&holdTemp=", newTemp, "&operation=1&status=1")
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Access_token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, cf)
	if err != nil {
		return err
	}
	log.Debug("url: ", url)
	if cf.ErrorCode != 0 {
		return fmt.Errorf("errorcode from request: %d", cf.ErrorCode)
	}
	return nil
}

func (cf *Config) ModeControl(accessToken string, deviceId string, oldTemp int64, newMode string) bool {
	var mode int
	if newMode == "heat" {
		mode = 1
	} else if newMode == "off" {
		mode = 0
	} else {
		log.Info("Unsupported mode: ", newMode)
		return false
	}
	url := fmt.Sprintf("%s%s%s%s%d%s%d", deviceControlURL, "?deviceId=", deviceId, "&holdTemp=", oldTemp, "&operation=0&status=", mode)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't controll device, error: ", err))
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Access_token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, cf)
	if err != nil {
		log.Debug("Error in DeviceControl: ", err)
	}

	if cf.ErrorCode == 0 {
		return true
	}
	return false
}

func (cf *Config) GetAuthCode(oldMsg *fimpgo.Message) (string, string) {
	cfs := model.Configs{}
	val, err := oldMsg.Payload.GetStrMapValue()
	if err != nil {
		log.Error("Wrong msg format")
		return "", ""
	}
	cfs.HubToken = val["token"]

	type Payload struct {
		PartnerCode string `json:"partnerCode"`
	}
	data := Payload{
		PartnerCode: "mill",
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		// handle err
		log.Debug("issue with payloadBytes")
	}
	body := bytes.NewReader(payloadBytes)

	var env string
	hubInfo, err := utils.NewHubUtils().GetHubInfo()
	if err == nil && hubInfo != nil {
		env = hubInfo.Environment
	} else {
		// TODO: switch to prod
		env = utils.EnvBeta
	}
	var url string
	if env == utils.EnvBeta {
		url = "https://partners-beta.futurehome.io/api/control/edge/proxy/custom/auth-code"
	} else {
		url = "https://partners.futurehome.io/api/control/edge/proxy/custom/auth-code"
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		// handle err
		log.Debug(fmt.Errorf("Issue when making request to partner-api"))
	}
	req.Header.Set("Authorization", os.ExpandEnv(fmt.Sprintf("%s%s", "Bearer ", cfs.HubToken)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Postman-Token", "65cb80d3-cbd2-4c8d-954a-bb3253b306e5")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, cf)

	authorizationCode := cf.Data.AuthorizationCode
	return authorizationCode, cfs.HubToken
}

// Unmarshall received data into holder struct
func processHTTPResponse(resp *http.Response, err error, holder interface{}) error {
	if err != nil {
		log.Error(fmt.Errorf("API does not respond"))
		return err
	}
	defer resp.Body.Close()
	// check http return code
	if resp.StatusCode != 200 {
		//bytes, _ := ioutil.ReadAll(resp.Body)
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return fmt.Errorf("Bad HTTP return code %d", resp.StatusCode)
	}

	// Unmarshall response into given struct
	if err = json.NewDecoder(resp.Body).Decode(holder); err != nil {
		return err
	}
	return nil
}

func (c *Client) UpdateLists(accessToken string, hc []interface{}, rc []interface{}, dc []interface{}) (homelist []interface{}, roomlist []interface{}, devicelist []interface{}) {
	allDevices, allRooms, allHomes, err := c.GetAllDevices(accessToken)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't update lists, error: ", err))
	}
	for home := range allHomes {
		hc = append(hc, allHomes[home])
	}
	for room := range allRooms {
		rc = append(rc, allRooms[room])
	}
	for device := range allDevices {
		dc = append(dc, allDevices[device])
	}

	return hc, rc, dc
}
