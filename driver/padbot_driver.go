package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"time"
)

type PadbotDriver struct {
	Status               PadbotDriverStatus `json:"data,omitempty"`
	Healthy              bool               `json:"healthy,omitempty"`
	BaseURL              string             `json:"baseURL,omitempty"`
	stopUpdateStatusChan chan bool
	stopUpdateHealthChan chan bool
}

type PadbotDriverStatus struct {
	BatteryPercentage int64  `json:"batteryPercentage,omitempty"`
	BatteryStatus     string `json:"batteryStatus,omitempty"`
	ActionStatus      string `json:"actionStatus,omitempty"`
	NavigationStatus  string `json:"navigationStatus,omitempty"`
	RobotLocation     string `json:"robotLocation,omitempty"`
}

type PadbotProtocolCommonConfig struct {
	CustomizedValues struct {
		PadbotBaseURL string `json:"padbotBaseURL,omitempty"`
	} `json:"customizedValues,omitempty"`
}

type PadbotProtocolConfig struct {
	ProtocolName string `json:"protocolName,omitempty"`
	ConfigData   struct {
		PadbotBaseURL string `json:"padbotBaseURL,omitempty"`
	} `json:"configData,omitempty"`
}

type PadBotProtocolVisitorConfig struct {
	ProtocolName string `json:"protocolName,omitempty"`
	ConfigData   struct {
		PropertyName string `json:"propertyName,omitempty"`
	} `json:"configData,omitempty"`
}

// InitDevice provide configmap parsing to specific protocols
func (d *PadbotDriver) InitDevice(protocolCommon []byte) (err error) {
	// 反序列化PadbotProtocolCommonConfig
	protocolCommonConfig := &PadbotProtocolCommonConfig{}
	err = json.Unmarshal(protocolCommon, protocolCommonConfig)
	if err != nil {
		klog.Error("Unmarshal protocolCommon error in InitDevice, err: ", err)
		return err
	}

	// 注册baseURL
	d.BaseURL = protocolCommonConfig.CustomizedValues.PadbotBaseURL

	// 终止健康和状态更新的channel
	d.stopUpdateHealthChan = make(chan bool)
	d.stopUpdateStatusChan = make(chan bool)

	go func() {
		for {
			select {
			case <-d.stopUpdateHealthChan:
				return
			default:
				d.updateHealth()
				time.Sleep(1 * time.Second)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-d.stopUpdateStatusChan:
				return
			default:
				d.updateStatus()
				time.Sleep(1 * time.Second)
			}
		}
	}()

	return nil
}

// ReadDeviceData  is an interface that reads data from a specific device, data is a type of string
func (d *PadbotDriver) ReadDeviceData(protocolCommon []byte, visitor []byte, protocol []byte) (data interface{}, err error) {
	res := ""
	protocolConfig := &PadbotProtocolConfig{}
	visitorConfig := &PadBotProtocolVisitorConfig{}
	protocolCommonConfig := &PadbotProtocolCommonConfig{}

	err1 := json.Unmarshal(protocolCommon, protocolCommonConfig)
	err2 := json.Unmarshal(visitor, visitorConfig)
	err3 := json.Unmarshal(protocol, protocolConfig)

	if err1 != nil || err2 != nil || err3 != nil {
		klog.Error("Unmarshal error in ReadDeviceData, err1: ", err1, "err2: ", err2, "err3: ", err3)
		return res, err
	}

	propertyName := visitorConfig.ConfigData.PropertyName
	switch propertyName {
	case "batteryPercentage":
		res = fmt.Sprint(d.Status.BatteryPercentage)
	case "batteryStatus":
		res = d.Status.BatteryStatus
	case "actionStatus":
		res = d.Status.ActionStatus
	case "navigationStatus":
		res = d.Status.NavigationStatus
	case "robotLocation":
		res = d.Status.RobotLocation
	default:
		klog.Error("Unknown propertyName: ", propertyName)
	}

	if len(res) == 0 {
		res = "UNKNOWN"
	}

	return res, nil
}

// WriteDeviceData is an interface that write data to a specific device, data's DataType is Consistent with configmap
func (d *PadbotDriver) WriteDeviceData(data interface{}, protocolCommon []byte, visitor []byte, protocol []byte) (err error) {
	protocolCommonConfig := &PadbotProtocolCommonConfig{}
	visitorConfig := &PadBotProtocolVisitorConfig{}
	protocolConfig := &PadbotProtocolConfig{}

	err1 := json.Unmarshal(protocolCommon, protocolCommonConfig)
	err2 := json.Unmarshal(visitor, visitorConfig)
	err3 := json.Unmarshal(protocol, protocolConfig)

	if err1 != nil || err2 != nil || err3 != nil {
		klog.Error("Unmarshal error in WriteDeviceData, err1: ", err1, "err2: ", err2, "err3: ", err3)
		return err
	}

	propertyName := visitorConfig.ConfigData.PropertyName

	switch propertyName {
	case "robotLocation":
		navigationURL := d.BaseURL + "/navigation"
		targetPoint := data.(string)
		if targetPoint == "" || targetPoint == "UNKNOWN" { // 忽略UNKNOWN
			return nil
		}

		go func() {
			// 发送导航请求
			postData := fmt.Sprintf("{\"targetPoint\": \"%s\"}", targetPoint)
			rsp, err := http.Post(navigationURL, "application/json", bytes.NewBuffer([]byte(postData)))
			if err != nil {
				klog.Error("Post navigation request error, err: ", err)
				return
			}
			if rsp.StatusCode != 200 {
				klog.Error("Post navigation request error, statusCode: ", rsp.StatusCode)
			}
			defer rsp.Body.Close()
		}()

	default:
		klog.Error("Unknown propertyNam or not support write: ", propertyName)
	}

	return nil
}

// StopDevice is an interface to stop all devices
func (d *PadbotDriver) StopDevice() (err error) {
	d.stopUpdateHealthChan <- true
	d.stopUpdateStatusChan <- true
	return nil
}

// GetDeviceStatus is an interface to get the device status true is OK , false is DISCONNECTED
func (d *PadbotDriver) GetDeviceStatus(protocolCommon []byte, visitor []byte, protocol []byte) (status bool) {
	return d.Healthy
}

func (d *PadbotDriver) updateHealth() {
	if len(d.BaseURL) == 0 {
		return
	}
	healthCheckURL := d.BaseURL + "/health"
	response, err := http.Get(healthCheckURL)

	if err != nil || response.StatusCode != 200 {
		klog.Error("Get health error, err: ", err)
		d.Healthy = false
		return
	} else {
		d.Healthy = true
	}

	defer response.Body.Close()

}

func (d *PadbotDriver) updateStatus() {
	if len(d.BaseURL) == 0 {
		return
	}

	finalStatus := PadbotDriverStatus{}
	finalStatus.ActionStatus = "UNKNOWN"
	finalStatus.BatteryPercentage = -1
	finalStatus.BatteryStatus = "UNKNOWN"
	finalStatus.NavigationStatus = "UNKNOWN"
	finalStatus.RobotLocation = "UNKNOWN"

	defer func ()  {
		d.Status = finalStatus
	}()

	statusURL := d.BaseURL + "/status"
	response, err := http.Get(statusURL)

	if err != nil || response.StatusCode != 200 {
		klog.Error("Get status error, err: ", err)
		return
	}

	defer response.Body.Close()

	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		klog.Error("Read response body error, err: ", err)
		return
	}

	var status PadbotDriverStatus
	err = json.Unmarshal(resBody, &status)

	if err != nil {
		klog.Error("Unmarshal response body error, err: ", err)
		return
	}

	finalStatus = status

}
