# Futurehome LVI Adapter

Adapter connects with api and retrieves list of homes, rooms and devices connected to your LVI user.

Use `make deb-arm` to make package. 

After adapter is installed on hub, go to playground -> LVI -> settings -> login

After logging in this message will be sent to FIMP:

`Topic`
```
pt:j1/mt:evt/rt:dev/rn:lvi/ad:1
```

`Payload`
```json
    {
    "type": "cmd.auth.set_tokens",
    "serv": "lvi",
    "val_t": "str_map",
    "val": {
        "username": "appUsername",
        "password": "appPassword"
        }
    }
```

The program then does some magic to retrieve a unique authorization code, access_token, refresh_token, expireTime and refresh_expireTime. When a valid access_token is received you can get information about all homes, rooms and devices, as well as setting temperature on devices. Access_token is valid for 2 hours, and when it expires the adapter automatically refreshes all tokens. The refresh_token is valid for 30 days, meaning that if the adapter is turned off for more than 30 days you will need to log in again.

The program saves all configs such as credentials, expiretimes and devices so that you only need to use `cmd.auth.set_tokens` once. 

***

After logging into the LVI app in playgrounds, all devices connected to your LVI user will be included in the Futurehome app. To activate a device you need to place it in a room, and then set the room temperature. Your device will then periodically send temperature reports, and will be controlled automatically by Futurehome's climate controll.

Initially the devices will send temperature reports every 5 minutes. This can be changed at any time by going to playground -> LVI -> settings -> advanced setup -> `Poll Time`. You can set Poll Time to any whole number from 1 to inf minutes. 

If you have devices on your LVI account that you dont want in the Futurehome app, simply go to device and click `delete`. If you change your mind, or delete a device by accident, you can reinclude all devices by going to playground -> LVI -> settings -> advanced setup -> `sync`. 
***

## Services and interfaces
#### Service name
`thermostat`
#### Interfaces
Type | Interface               | Value type | Description
-----|-------------------------|------------|------------------
in   | cmd.mode.get_report     | null       |
in   | cmd.mode.set            | string     |  set thermostat mode
out  | evt.mode.report         | string     |
-|||
in   | cmd.setpoint.get_report | string     | value is a set-point type
in   | cmd.setpoint.set        | str_map    | val = {"type":"heat", "temp":"21.5", "unit":"C"}
out  | evt.setpoint.report     | str_map    | val = {"type":"heat", "temp":"21.5", "unit":"C"}

#### Service name
`sensor_temp`
#### Interfaces
Type | Interface               | Value type | Description
-----|-------------------------|------------|------------------
in   | cmd.sensor.get_report   | null       | 
in   | evt.sensor.report       | float      | measured temperature
