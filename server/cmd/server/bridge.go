// 桥接：将 models 内部函数重新暴露为顶层函数，便于 main.go 引用
package main

import (
	"strconv"

	"github.com/linkall/server/internal/models"
)

func modelsFindDeviceByCode(code string) (*models.Device, string, error) {
	return models.FindDeviceByCode(code)
}

func modelsSetDeviceOnline(code string, online bool) {
	models.SetDeviceOnline(code, online)
}

func modelsUpdateDeviceMeta(code, name, os, app, ip, tag, notes string) (int64, error) {
	if err := models.UpdateDeviceMeta(code, name, os, app, ip, tag, notes); err != nil {
		return 0, err
	}
	return 1, nil
}

func modelsCountDevices() (int, error) { return models.CountDevices() }

func itoa(n int) string { return strconv.Itoa(n) }
