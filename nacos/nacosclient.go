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

// NacosClient nacos client
type NacosClient struct {
	mx           sync.Mutex
	config       nacosConfig
	LocalHost    string
	nacosDir      string
	Port         string
	isNameStart  bool
	nameClient   naming_client.INamingClient
	configClient config_client.IConfigClient
}

//NewNacosClient instance object
func NewNacosClient(homeDir string,logging log.Logger) *NacosClient {
	client := &NacosClient{}
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
func (sdkClient *NacosClient) LoadGuideConfig() error {
	config := path.Join(sdkClient.nacosDir,"nacos.yml")
	err,readContent := ConfigInstance().ReadLocationConfig(config)
	if err != nil {
		level.Error(logger).Log("msg", "LoadGuideConfig", "err", err)
		return err
	}
	err = yaml.Unmarshal([]byte(readContent), &sdkClient.config)
	if err != nil {
		level.Error(logger).Log("msg", "LoadConfig", "err", err)
		return err
	}
	// 处理地址访问
	url := strings.Split(sdkClient.config.AccessAddress,":")
	if len(url) < 2 {
		return errors.New(sdkClient.config.AccessAddress+"nacos url is not match http URL")
	}
	if len(sdkClient.config.ServerConfig)< 1 {
		sdkClient.config.ServerConfig[0]= constant.ServerConfig{ContextPath:"/nacos"}
	}
	sdkClient.config.ServerConfig[0].IpAddr = url[0]
	port,_:= strconv.ParseUint(url[1],10, 64)
	sdkClient.config.ServerConfig[0].Port = port
	sdkClient.LocalHost = GetMatchLocalIP(sdkClient.config.LocalHost)
	return nil
}

//RemoteDiscoverConfig  load from server config center
func (sdkClient *NacosClient) RemoteDiscoverConfig() error {
	client := sdkClient.getConfigClient()
	if client == nil {
		level.Error(logger).Log("msg", "config client is nil")
		return errors.New("config provider is fail")
	}
	dsconfig := ConfigInstance()
	content, err := client.GetConfig(vo.ConfigParam{
		DataId: sdkClient.config.DataID,
		Group:  sdkClient.config.Group})
	if err != nil {
		level.Error(logger).Log("DataId: %s,Group: %s, NamespaceId: %s,error $s", sdkClient.config.DataID, sdkClient.config.Group, sdkClient.config.ClientConfig.NamespaceId, err.Error())
		return err
	}
	r := strings.NewReader(content)
	err = dsconfig.Config.ReadConfig(r)
	if err != nil {
		level.Error(logger).Log("read config :%s, error: %s", sdkClient.config.ServiceName, err.Error())
		fmt.Println(err.Error())
		return err
	}
	_ = client.ListenConfig(vo.ConfigParam{
		DataId: sdkClient.config.DataID,
		Group:  sdkClient.config.Group,
		OnChange: func(namespace, group, dataId, data string) {
			level.Info(logger).Log("config altermanger.yaml has changed in nacos")
			r := strings.NewReader(content)
			dsconfig.Config.MergeConfig(r)
		},
	})
	return nil
}

// RegisterService register local instance with service
func (sdkClient *NacosClient) RegisterService(listenPort string) {
	if sdkClient.isNameStart {
		return
	}
	sdkClient.Port = listenPort
	port, err := strconv.ParseUint(listenPort, 10, 64)
	sdkClient.nameClient = sdkClient.getNamingClient()
	if sdkClient.nameClient == nil {
		level.Error(logger).Log("namingClient is nil")
		return
	}
	success, err := sdkClient.nameClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          sdkClient.LocalHost,
		Port:        port,
		ServiceName: sdkClient.config.ServiceName,
		Weight:      1,
		ClusterName: sdkClient.config.ClusterName,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{},
	})
	if err != nil {
		level.Error(logger).Log("register service: %s, error: %s", sdkClient.config.ServiceName, err.Error())
	}
	sdkClient.isNameStart = true
	level.Info(logger).Log("register result: ", success)
}

//GetService select other service
func (sdkClient *NacosClient) GetService(serviceName string) string {
	if sdkClient.nameClient == nil {
		level.Error(logger).Log("namingClient is nil")
		return ""
	}
	svr, err := sdkClient.nameClient.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
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
func (sdkClient *NacosClient) suscribeService(name string) {
	client := sdkClient.nameClient
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
func (sdkClient *NacosClient) getConfigClient() config_client.IConfigClient {
	sdkClient.mx.Lock()
	defer sdkClient.mx.Unlock()
	if sdkClient.configClient == nil {
		client, err := clients.CreateConfigClient(map[string]interface{}{
			"serverConfigs": sdkClient.config.ServerConfig,
			"clientConfig":  sdkClient.config.ClientConfig,
		})
		if err != nil {
			level.Error(logger).Log("connect configClient error: ", err)
		}
		sdkClient.configClient = client
	}
	return sdkClient.configClient
}

func (sdkClient *NacosClient) getNamingClient() naming_client.INamingClient {
	sdkClient.mx.Lock()
	defer sdkClient.mx.Unlock()
	if sdkClient.nameClient == nil {
		client, err := clients.CreateNamingClient(map[string]interface{}{
			"serverConfigs": sdkClient.config.ServerConfig,
			"clientConfig":  sdkClient.config.ClientConfig,
		})
		if err != nil {
			level.Error(logger).Log("connect nameClient error: ", err)
		}
		sdkClient.nameClient = client
	}
	return sdkClient.nameClient
}

//GetNacosIP get local ip address
func (sdkClient *NacosClient) GetNacosIP() string {
	if sdkClient.config.ServerConfig != nil {
		return sdkClient.config.ServerConfig[0].IpAddr
	}
	return ""
}

//GetNacosPort get local port
func (sdkClient *NacosClient) GetNacosPort() string {
	if sdkClient.config.ServerConfig != nil && len(sdkClient.config.ServerConfig) >= 0 {
		return fmt.Sprintf("%d", sdkClient.config.ServerConfig[0].Port)
	}
	return ""
}

// GetNacosNamespaceID get local namespace
func (sdkClient *NacosClient) GetNacosNamespaceID() string {
	return sdkClient.config.ClientConfig.NamespaceId
}
