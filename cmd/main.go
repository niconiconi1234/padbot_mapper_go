package main

import (
	"fmt"
	"github.com/kubeedge/mappers-go/mapper-sdk-go/pkg/models"
	"github.com/niconiconi1234/padbot_mapper_go/driver"
	"github.com/kubeedge/mappers-go/mapper-sdk-go/pkg/service"
)

func main() {
	fmt.Println("Padbot Mapper Started")
	var driver models.ProtocolDriver = &driver.PadbotDriver{}
	service.Bootstrap("padbot-protocol", driver)
}
