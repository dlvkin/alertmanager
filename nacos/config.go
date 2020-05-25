package nacos

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"strings"
)

/*
  @Author:hunterfox
  @Time: 2020/3/20 下午5:17

*/

var logger log.Logger
var disconfig *DisConfig
// DisConfig local yml yaml in local
type DisConfig struct {
	Config *viper.Viper
}

// ConfigInstance get instance  of config
func ConfigInstance() *DisConfig {
	if disconfig == nil {
		disconfig = &DisConfig{
			Config: viper.New(),
		}
		disconfig.Config.AutomaticEnv()
	}
	return disconfig
}

// SetEnviroment set viper in config
func (dconfig *DisConfig) SetEnviroment(viper *viper.Viper) {
	dconfig.Config = viper
}


// ReadLocationConfig get config content
func (dconfig *DisConfig) ReadLocationConfig(defaultPath string) (error,string) {
	nacosfile, err := ioutil.ReadFile(defaultPath)
	if err != nil {
		level.Error(logger).Log("msg", "Error on alert update", "err", err)
		return err,""
	}
	readcontent := string(nacosfile)
	for _,value := range os.Environ() {
		keyvalue := strings.Split(value,"=")
		if len(keyvalue)==2 {
			find := fmt.Sprintf("${%s}",keyvalue[0])
			if strings.Index(readcontent,find)>0 {
				readcontent = strings.ReplaceAll(readcontent,find,keyvalue[1])
			}
		}
	}
	return nil,readcontent
}
