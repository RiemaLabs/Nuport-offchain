package config

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/RiemaLabs/nuport-offchain/das"
)

//go:embed config.json
var configDody []byte

func NewConfig() *NubitDAServerConfig {
	if configDody == nil {
		fmt.Println("config.json not found, use default config")
		return &DefaultNubitDAServerConfig
	}

	var config NubitDAServerConfig
	err := json.Unmarshal(configDody, &config)
	if err != nil {
		panic(err)
	}
	return &config
}

type NubitDAServerConfig struct {
	RPCPort uint64 `json:"rpc-port"`

	RPCServerBodyLimit int `json:"rpc-server-body-limit"`

	NubitDa das.DAConfig `json:"nubit"`

	LogLevel string `json:"log-level"`
	LogType  string `json:"log-type"`

	Metrics bool `json:"metrics"`
	PProf   bool `json:"pprof"`
}

var DefaultNubitDAServerConfig = NubitDAServerConfig{
	RPCPort:  9876,
	LogLevel: "INFO",
	LogType:  "plaintext",
	Metrics:  false,
	PProf:    false,
}
