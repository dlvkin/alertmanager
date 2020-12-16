package nacos

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

/*
  @Time: 2020/12/16 下午5:17
*/

var logger log.Logger
var enviromentFig *DisConfig
var singleOnce sync.Once
// DisConfig local yml or yaml in local
type DisConfig struct {
	Config *viper.Viper
}

// ConfigInstance get instance  of config
func ConfigInstance() *DisConfig {
	if enviromentFig == nil {
		singleOnce.Do(func() {
			enviromentFig = &DisConfig{
			 	Config: viper.New(),
			}
			enviromentFig.Config.AutomaticEnv()
		})
	}
	return enviromentFig
}

// SetEnviroment set viper in config
func (enviromentFig *DisConfig) SetEnviroment(viper *viper.Viper) {
	enviromentFig.Config = viper
}


// ReadLocationConfig get config content
func (environmentFig *DisConfig) ReadLocationConfig(defaultPath string) (error,string) {
	nacosFile, err := ioutil.ReadFile(defaultPath)
	if err != nil {
		level.Error(logger).Log("msg", "Error on alert update", "err", err)
		return err,""
	}
	readContent := string(nacosFile)
	for _,value := range os.Environ() {
		keyValue := strings.Split(value,"=")
		if len(keyValue)==2 {
			find := fmt.Sprintf("${%s}",keyValue[0])
			if strings.Index(readContent,find)>0 {
				readContent = strings.ReplaceAll(readContent,find,keyValue[1])
			}
		}
	}
	return nil,readContent
}
