package main

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

type conf struct {
	BackupFolder     string `yaml:"backupFolder"`
	Account1Login    string `yaml:"account1_login"`
	Account1Password string `yaml:"account1_password"`
	Account2Login    string `yaml:"account2_login"`
	Account2Password string `yaml:"account2_password"`
	Account3Login    string `yaml:"account3_login"`
	Account3Password string `yaml:"account3_password"`
	Account4Login    string `yaml:"account4_login"`
	Account4Password string `yaml:"account4_password"`
}

func getConfiguration(confFile string) (*conf, error) {
	var c conf
	yamlFile, err := ioutil.ReadFile(confFile)

	err = yaml.Unmarshal(yamlFile, &c)

	return &c, err
}
