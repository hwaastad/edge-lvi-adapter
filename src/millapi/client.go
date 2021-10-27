package mill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/futurehomeno/mill/model"

	"github.com/futurehomeno/fimpgo"
	"github.com/futurehomeno/fimpgo/utils"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultBaseURL is mill api url
	baseURL = "https://api.millheat.com/"
	// applyAccessTokenURL is mill api to get access_token and refresh_token
	applyAccessTokenURL = baseURL + "share/applyAccessToken"
	// authURL is mill api to get authorization_code
	authURL = baseURL + "share/applyAuthCode"
	// refreshURL is mill api to update access_token and refresh_token
	refreshURL = baseURL + "share/refreshtoken?refreshtoken="

	// deviceControlForOpenApiURL is mill api to controll individual devices
	deviceControlURL = baseURL + "uds/deviceControlForOpenApi"
	// getIndependentDevicesURL is mill api to get list of devices in unassigned room
	getIndependentDevicesURL = baseURL + "uds/getIndependentDevices"
	// selectDevicebyRoomURL is mill api to search device list by room
	selectDevicebyRoomURL = baseURL + "uds/selectDevicebyRoom"
	// selectHomeListURL is mill api to search housing list
	selectHomeListURL = baseURL + "uds/selectHomeList"
	// selectRoombyHomeURL is mill api to search room list by home
	selectRoombyHomeURL = baseURL + "uds/selectRoombyHome"
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

	Password string `json:"password"`
	Username string `json:"username"`

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
		Homes              []Home   `json:"homeList"`
		Rooms              []Room   `json:"roomList"`
		Devices            []Device `json:"deviceList"`
		IndependentDevices []Device `json:"deviceInfoList"`
	} `json:"data"`
}

// Device is a mill heater
type Device struct {
	MaxTemperature       int     `json:"maxTemperature"`
	MaxTemperatureMsg    string  `json:"maxTemperatureMsg"`
	ChangeTemperature    int     `json:"changeTemperature"`
	CanChangeTemp        int     `json:"canChangeTemp"`
	DeviceID             int64   `json:"deviceId"`
	DeviceName           string  `json:"deviceName"`
	ChangeTemperatureMsg string  `json:"changeTemperatureMsg"`
	Mac                  string  `json:"mac"`
	DeviceStatus         int     `json:"deviceStatus"`
	HeaterFlag           int     `json:"heaterFlag"`
	SubDomainID          int     `json:"subDomainId"`
	ControlType          int     `json:"controlType"`
	CurrentTemp          float32 `json:"currentTemp"`
	SetpointTemp         int64   `json:"holidayTemp"`
}

type Home struct {
	HomeName         string      `json:"homeName"`
	IsHoliday        int         `json:"isHoliday"`
	HolidayStartTime int         `json:"holidayStartTime"`
	TimeZone         string      `json:"timeZone"`
	ModeMinute       int         `json:"modeMinute"`
	ModeStartTime    int64       `json:"modeStartTime"`
	HolidayTemp      int         `json:"holidayTemp"`
	ModeHour         int         `json:"modeHour"`
	CurrentMode      int         `json:"currentMode"`
	HolidayEndTime   int         `json:"holidayEndTime"`
	HomeType         interface{} `json:"homeType"`
	HomeID           int64       `json:"homeId"`
	ProgramID        int64       `json:"programId"`
}

type Room struct {
	MaxTemperature       int           `json:"maxTemperature"`
	IndependentDeviceIds []interface{} `json:"independentDeviceIds"`
	MaxTemperatureMsg    string        `json:"maxTemperatureMsg"`
	ChangeTemperature    int           `json:"changeTemperature"`
	ControlSource        string        `json:"controlSource"`
	ComfortTemp          int           `json:"comfortTemp"`
	RoomProgram          string        `json:"roomProgram"`
	AwayTemp             int           `json:"awayTemp"`
	AvgTemp              int           `json:"avgTemp"`
	ChangeTemperatureMsg string        `json:"changeTemperatureMsg"`
	RoomID               int64         `json:"roomId"`
	RoomName             string        `json:"roomName"`
	CurrentMode          int           `json:"currentMode"`
	HeatStatus           int           `json:"heatStatus"`
	OffLineDeviceNum     int           `json:"offLineDeviceNum"`
	Total                int           `json:"total"`
	IndependentCount     int           `json:"independentCount"`
	SleepTemp            int           `json:"sleepTemp"`
	OnlineDeviceNum      int           `json:"onlineDeviceNum"`
	IsOffline            int           `json:"isOffline"`
}

// NewClient create a handle authentication to Mill API
func (config *Config) NewClient(authCode string, password string, username string) (string, string, int64, int64) {
	urlpassword := url.QueryEscape(password)
	urlusername := url.QueryEscape(username)
	url := applyAccessTokenURL + "?password=" + urlpassword + "&username=" + urlusername
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't post accessToken request, error: ", err))
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authorization_code", authCode)

	resp, err := http.DefaultClient.Do(req)
	processHTTPResponse(resp, err, config)

	accessToken := config.Data.AccessToken
	refreshToken := config.Data.RefreshToken
	expireTime := config.Data.ExpireTime
	refreshExpireTime := config.Data.RefreshExpireTime
	if err != nil {
		return "", "", 0, 0
	}
	defer resp.Body.Close()
	return accessToken, refreshToken, expireTime, refreshExpireTime
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

func (c *Client) GetAllDevices(accessToken string) ([]Device, []Room, []Home, []Device, error) {
	homes, err := c.GetHomeList(accessToken)
	var allDevices []Device
	var allRooms []Room
	var allHomes []Home
	var allIndependentDevices []Device
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't get home list, error: ", err))
	}
	for home := range homes.Data.Homes {
		allHomes = append(allHomes, homes.Data.Homes[home])
		rooms, err := c.GetRoomList(accessToken, homes.Data.Homes[home].HomeID)
		if err != nil {
			// handle err
			log.Error(fmt.Errorf("Can't get room list, error: ", err))
		}
		for room := range rooms.Data.Rooms {
			allRooms = append(allRooms, rooms.Data.Rooms[room])
			devices, err := c.GetDeviceList(accessToken, rooms.Data.Rooms[room].RoomID)
			for device := range devices.Data.Devices {
				allDevices = append(allDevices, devices.Data.Devices[device])
			}
			if err != nil {
				// handle err
				log.Error(fmt.Errorf("Can't get device list, error: ", err))
			}
		}
		// Get all independent devices
		independentDevices, err := c.GetIndependentDevices(accessToken, homes.Data.Homes[home].HomeID)
		if err != nil {
			// handle err
			log.Error(fmt.Errorf("Can't get independent device list, error: ", err))
		}
		for device := range independentDevices.Data.IndependentDevices {
			allDevices = append(allDevices, independentDevices.Data.IndependentDevices[device])
			allIndependentDevices = append(allIndependentDevices, independentDevices.Data.IndependentDevices[device])
		}
	}
	return allDevices, allRooms, allHomes, allIndependentDevices, nil
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
func (c *Client) GetRoomList(accessToken string, homeID int64) (*Client, error) {
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
func (c *Client) GetDeviceList(accessToken string, roomID int64) (*Client, error) {
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

func (c *Client) GetIndependentDevices(accessToken string, homeId int64) (*Client, error) {
	url := fmt.Sprintf("%s%s%d", getIndependentDevicesURL, "?homeId=", homeId)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		// handle err
		log.Error(fmt.Errorf("Can't get independent device list, error: ", err))
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
		return fmt.Errorf("%s%d", "errorcode from request: ", cf.ErrorCode)
	}
	return nil
	// return fmt.Errorf("errorcode from request: " + strconv.Itoa(cf.ErrorCode))
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

func (c *Client) UpdateLists(accessToken string, hc []interface{}, rc []interface{}, dc []interface{}, idc []interface{}) (homelist []interface{}, roomlist []interface{}, devicelist []interface{}, independentdevicelist []interface{}) {
	allDevices, allRooms, allHomes, allIndependentDevices, err := c.GetAllDevices(accessToken)
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
	for device := range allIndependentDevices {
		idc = append(idc, allIndependentDevices[device])
	}
	return hc, rc, dc, idc
}
