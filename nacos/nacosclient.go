package nacos

import (
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/utils"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

type nacosConfig struct {
	ClientConfig constant.ClientConfig   `yaml:"ClientConfig"`
	ServerConfig []constant.ServerConfig `yaml:"ServerConfig"`
	LocalHost    string                  `yaml:"LocalHost"`
	DataID       string                  `yaml:"DataId"`
	Group        string                  `yaml:"Group"`
	ServiceName  string                  `yaml:"ServiceName"`
	ClusterName  string                  `yaml:"ClusterName"`
	AccessAddress string                  `yaml:"AccessAddress"`
}

// Nacosclient nacos client
type Nacosclient struct {
	mx           sync.Mutex
	config       nacosConfig
	LocalHost    string
	nacosDir      string
	Port         string
	isNameStart  bool
	nameClient   naming_client.INamingClient
	configClient config_client.IConfigClient
}

//NewNacosclient instance object
func NewNacosclient(homeDir string,logging log.Logger) *Nacosclient {
	client := &Nacosclient{}
	client.nacosDir = homeDir
	if client.nacosDir == "" {
		client.nacosDir, _ = os.Getwd()
	}
	logger = logging
	dsconfig :=  ConfigInstance()
	dsconfig.Config.Set(HomeDir, client.nacosDir)
	dsconfig.Config.SetEnvPrefix(ModuleProductName)
	dsconfig.Config.SetConfigType("yaml")
	replacer := strings.NewReplacer(".", "_")
	dsconfig.Config.SetEnvKeyReplacer(replacer)
	return client
}

// LoadGuideConfig load from server config center
func (sdkclient *Nacosclient) LoadGuideConfig() error {
	config := path.Join(sdkclient.nacosDir,"nacos.yml")
	err,readContent := ConfigInstance().ReadLocationConfig(config)
	if err != nil {
		level.Error(logger).Log("msg", "LoadGuideConfig", "err", err)
		return err
	}
	err = yaml.Unmarshal([]byte(readContent), &sdkclient.config)
	if err != nil {
		level.Error(logger).Log("msg", "LoadConfig", "err", err)
		return err
	}
	// 处理地址访问
	url := strings.Split(sdkclient.config.AccessAddress,":")
	if len(url) < 2 {
		return errors.New(sdkclient.config.AccessAddress+"nacos url is not match http URL")
	}
	if len(sdkclient.config.ServerConfig)< 1 {
		sdkclient.config.ServerConfig[0]= constant.ServerConfig{ContextPath:"/nacos"}
	}
	sdkclient.config.ServerConfig[0].IpAddr = url[0]
	port,_:= strconv.ParseUint(url[1],10, 64)
	sdkclient.config.ServerConfig[0].Port = port
	sdkclient.LocalHost = GetMatchLocalIP(sdkclient.config.LocalHost)
	return nil
}

//RemoteDiscoverConfig  load from server config center
func (sdkclient *Nacosclient) RemoteDiscoverConfig() error {
	client := sdkclient.getConfigClient()
	if client == nil {
		level.Error(logger).Log("msg", "config client is nil")
		return errors.New("config provider is fail")
	}
	dsconfig := ConfigInstance()
	content, err := client.GetConfig(vo.ConfigParam{
		DataId: sdkclient.config.DataID,
		Group:  sdkclient.config.Group})
	if err != nil {
		level.Error(logger).Log("DataId: %s,Group: %s, NamespaceId: %s,error $s", sdkclient.config.DataID, sdkclient.config.Group, sdkclient.config.ClientConfig.NamespaceId, err.Error())
		return err
	}
	r := strings.NewReader(content)
	err = dsconfig.Config.ReadConfig(r)
	if err != nil {
		level.Error(logger).Log("read config :%s, error: %s", sdkclient.config.ServiceName, err.Error())
		fmt.Println(err.Error())
		return err
	}
	_ = client.ListenConfig(vo.ConfigParam{
		DataId: sdkclient.config.DataID,
		Group:  sdkclient.config.Group,
		OnChange: func(namespace, group, dataId, data string) {
			level.Info(logger).Log("config altermanger.yaml has changed in nacos")
			r := strings.NewReader(content)
			dsconfig.Config.MergeConfig(r)
		},
	})
	return nil
}

// RegisterService register local instance with service
func (sdkclient *Nacosclient) RegisterService(listenPort string) {
	if sdkclient.isNameStart {
		return
	}
	sdkclient.Port = listenPort
	port, err := strconv.ParseUint(listenPort, 10, 64)
	sdkclient.nameClient = sdkclient.getNamingClient()
	if sdkclient.nameClient == nil {
		level.Error(logger).Log("namingClient is nil")
		return
	}
	success, err := sdkclient.nameClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          sdkclient.LocalHost,
		Port:        port,
		ServiceName: sdkclient.config.ServiceName,
		Weight:      1,
		ClusterName: sdkclient.config.ClusterName,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{},
	})
	if err != nil {
		level.Error(logger).Log("register service: %s, error: %s", sdkclient.config.ServiceName, err.Error())
	}
	sdkclient.isNameStart = true
	level.Info(logger).Log("register result: ", success)
}

//GetService select other service
func (sdkclient *Nacosclient) GetService(serviceName string) string {
	if sdkclient.nameClient == nil {
		level.Error(logger).Log("namingClient is nil")
		return ""
	}
	svr, err := sdkclient.nameClient.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: serviceName,
	})

	if err != nil {
		level.Error(logger).Log("get service: %s, error: %s ", serviceName, err)
		return ""
	}
	if svr.Ip != "" {
		result := fmt.Sprintf("http://%s:%d", svr.Ip, svr.Port)
		return result
	}
	return ""
}

//suscribeService subsribe other service
func (sdkclient *Nacosclient) suscribeService(name string) {
	client := sdkclient.nameClient
	if client == nil {
		level.Error(logger).Log("namingClient is nil")
		return
	}
	_ = client.Subscribe(&vo.SubscribeParam{
		ServiceName: name,
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			level.Info(logger).Log("\n\n callback return services:%s \n\n", utils.ToJsonString(services))
		},
	})
}

//
func (sdkclient *Nacosclient) getConfigClient() config_client.IConfigClient {
	sdkclient.mx.Lock()
	defer sdkclient.mx.Unlock()
	if sdkclient.configClient == nil {
		client, err := clients.CreateConfigClient(map[string]interface{}{
			"serverConfigs": sdkclient.config.ServerConfig,
			"clientConfig":  sdkclient.config.ClientConfig,
		})
		if err != nil {
			level.Error(logger).Log("connect configClient error: ", err)
		}
		sdkclient.configClient = client
	}
	return sdkclient.configClient
}

func (sdkclient *Nacosclient) getNamingClient() naming_client.INamingClient {
	sdkclient.mx.Lock()
	defer sdkclient.mx.Unlock()
	if sdkclient.nameClient == nil {
		client, err := clients.CreateNamingClient(map[string]interface{}{
			"serverConfigs": sdkclient.config.ServerConfig,
			"clientConfig":  sdkclient.config.ClientConfig,
		})
		if err != nil {
			level.Error(logger).Log("connect nameClient error: ", err)
		}
		sdkclient.nameClient = client
	}
	return sdkclient.nameClient
}

//GetNacosIP get local ip address
func (sdkclient *Nacosclient) GetNacosIP() string {
	if sdkclient.config.ServerConfig != nil {
		return sdkclient.config.ServerConfig[0].IpAddr
	}
	return ""
}

//GetNacosPort get local port
func (sdkclient *Nacosclient) GetNacosPort() string {
	if sdkclient.config.ServerConfig != nil && len(sdkclient.config.ServerConfig) >= 0 {
		return fmt.Sprintf("%d", sdkclient.config.ServerConfig[0].Port)
	}
	return ""
}

// GetNacosNamespaceID get local namespace
func (sdkclient *Nacosclient) GetNacosNamespaceID() string {
	return sdkclient.config.ClientConfig.NamespaceId
}
