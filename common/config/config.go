package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

var Conf *Config

type Config struct {
	APP APP `yaml:"app"`
}

type APP struct {
	Port       string `yaml:"port"`
	LogPath    string `yaml:"log-path"`
	BufferSize int64  `yaml:"buffer-size"`
	BucketSize int64  `yaml:"bucket-size"`
}

func InitConf() error {
	Conf = &Config{}
	err := getConf("config.yaml", Conf) // 尝试在当前目录查找
	if err != nil {
		log.Printf("[Setting] Config init failed: %v", err)
		return err
	}
	return nil
}

// 从yaml配置文件中获取配置
func getConf(yamlFilePath string, confModel *Config) error {
	file, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(file, confModel)
	if err != nil {
		return err
	}
	return nil
}
