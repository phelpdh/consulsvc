package consulsvc

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"strconv"

	"github.com/hashicorp/consul/api"
)

// IConsulService ... Interface for consul registration
// type IConsulService interface {
// 	configRegistration()
// 	submitRegistration() error
// }

// ConsulRegistration ... Struct to hold service registration info
type ConsulRegistration struct {
	SvcID        string   `yaml:"id"`
	SvcName      string   `yaml:"name"`
	SvcProtocol  string   `yaml:"protocol"`
	SvcIP        string   `yaml:"ip"`
	SvcPort      int      `yaml:"port"`
	SvcHealthURL string   `yaml:"healthUrl"`
	SvcTags      []string `yaml:"tags"`
	ConsulURL    string   `yaml:"consulUrl"`
	SvcSkipSSL   bool     `yaml:"skipSsl"`
	Registered   bool     `yaml:"registered"`
}

// GetRegistration ... get new registration struct with some defaults
func GetRegistration(name, id string, port int) *ConsulRegistration {
	var cr ConsulRegistration
	cr.SvcName = name
	cr.SvcID = id
	cr.SvcProtocol = "http"
	cr.SvcIP = "localhost"
	cr.SvcPort = port
	cr.SvcHealthURL = "/health"
	cr.ConsulURL = "localhost:8500"
	cr.SvcSkipSSL = true
	cr.Registered = false
	return &cr
}

// register ... Method for registering service with consul
func (cr *ConsulRegistration) register() error {
	config := api.DefaultConfig()
	config.Address = cr.ConsulURL
	client, err := api.NewClient(config)
	if err != nil {
		return err
	}
	agent := client.Agent()
	consulService := api.AgentServiceRegistration{
		ID:   cr.SvcID,
		Name: cr.SvcName,
		Tags: cr.SvcTags,
		Port: cr.SvcPort,
		Check: &api.AgentServiceCheck{
			TLSSkipVerify: cr.SvcSkipSSL,
			Interval:      "10s",
			Timeout:       "5s",
			HTTP:          cr.SvcProtocol + "://" + cr.SvcIP + ":" + strconv.Itoa(cr.SvcPort) + cr.SvcHealthURL,
			Status:        "passing",
		},
		Checks: api.AgentServiceChecks{},
	}
	err = agent.ServiceRegister(&consulService)
	if err != nil {
		return err
	}
	return nil
}

// deregister ... Method for de-registering service with consul
func (cr *ConsulRegistration) deregister() error {
	config := api.DefaultConfig()
	config.Address = cr.ConsulURL
	client, err := api.NewClient(config)
	if err != nil {
		return err
	}
	agent := client.Agent()
	return agent.ServiceDeregister(cr.SvcID)
}

// RegisterWithConsul ... Register service with consul
// autoDeregister true will cause deregistration on service shutdown
func (cr *ConsulRegistration) RegisterWithConsul(autoDeregister bool) error {
	hostIP, found := os.LookupEnv("HOST_IP")
	if found {
		if strings.Contains(cr.SvcHealthURL, "localhost") {
			cr.SvcIP = hostIP
			cr.SvcHealthURL = strings.Replace(cr.SvcHealthURL, "localhost", hostIP, 1)
		}
	}
	err := cr.register()
	if err != nil {
		log.Printf("Error registering with consul on %s - %s\n", cr.ConsulURL, err)
		return err
	}
	cr.Registered = true
	if autoDeregister {
		go cr.deregisterWithConsulOnSigTermOrInterrupt()
	}
	return nil
}

// DeregisterWithConsul ... Deregister service with consul
func (cr *ConsulRegistration) DeregisterWithConsul() error {
	if cr.Registered {
		log.Printf("Attempting de-register (%v) with consul on %s\n", cr.SvcID, cr.ConsulURL)
		err := cr.deregister()
		if err != nil {
			log.Printf("Error de-registering (%v) with consul on %s - %s\n", cr.SvcID, cr.ConsulURL, err)
			return err
		}
		log.Printf("Successfully de-registered (%v) with consul on %s \n", cr.SvcID, cr.ConsulURL)
	}
	return nil
}

// Hook SIGTERM and interrupt to deregister service
func (cr *ConsulRegistration) deregisterWithConsulOnSigTermOrInterrupt() {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGTERM, os.Interrupt)
	<-sigChannel
	cr.DeregisterWithConsul()
	os.Exit(0)
}
