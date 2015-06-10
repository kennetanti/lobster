package main

import "lobster"
import "lobster/lndynamic"
import "lobster/lobopenstack"
import "lobster/solusvm"
import "lobster/vmfake"

import "encoding/json"
import "io/ioutil"
import "log"
import "os"

type VmConfig struct {
	Name string `json:"name"`

	// one of solusvm, openstack, lndynamic, fake
	Type string `json:"type"`

	// API options (used by solusvm, lndynamic)
	ApiId string `json:"api_id"`
	ApiKey string `json:"api_key"`

	// URL (used by solusvm, openstack)
	Url string `json:"url"`

	// solusvm options
	VirtType string `json:"virt_type"`
	NodeGroup string `json:"node_group"`
	Insecure bool `json:"insecure"`

	// openstack options
	Username string `json:"username"`
	Password string `json:"password"`
	Tenant string `json:"tenant"`
	NetworkId string `json:"network_id"`

	// lndynamic options
	Region string `json:"region"`
}

type PaymentConfig struct {
	Name string `json:"name"`

	// one of paypal, coinbase, fake
	Type string `json:"type"`

	// paypal options
	Business string `json:"business"`
	ReturnUrl string `json:"return_url"`

	// coinbase options
	CallbackSecret string `json:"callback_secret"`

	// API options (used by coinbase)
	ApiKey string `json:"api_key"`
	ApiSecret string `json:"api_secret"`
}

type InterfaceConfig struct {
	Vm []*VmConfig `json:"vm"`
	Payment []*PaymentConfig `json:"payment"`
}

func main() {
	cfgPath := "lobster.cfg"
	if len(os.Args) >= 2 {
		cfgPath = os.Args[1]
	}
	app := lobster.MakeLobster(cfgPath)
	app.Init()

	// load interface configuration
	interfacePath := cfgPath + ".json"
	interfaceConfigBytes, err := ioutil.ReadFile(interfacePath)
	if err != nil {
		log.Fatalf("Error: failed to read interface configuration file %s: %s", interfacePath, err.Error())
	}
	var interfaceConfig InterfaceConfig
	err = json.Unmarshal(interfaceConfigBytes, &interfaceConfig)
	if err != nil {
		log.Fatalf("Error: failed to parse interface configuration: %s", err.Error())
	}

	for _, vm := range interfaceConfig.Vm {
		var vmi lobster.VmInterface
		if vm.Type == "openstack" {
			vmi = lobopenstack.MakeOpenStack(vm.Url, vm.Username, vm.Password, vm.Tenant, vm.NetworkId)
		} else if vm.Type == "solusvm" {
			vmi = &solusvm.SolusVM{
				Lobster: app,
				VirtType: vm.VirtType,
				NodeGroup: vm.NodeGroup,
				Api: &solusvm.API{
					Url: vm.Url,
					ApiId: vm.ApiId,
					ApiKey: vm.ApiKey,
					Insecure: vm.Insecure,
				},
			}
		} else if vm.Type == "lndynamic" {
			vmi = lndynamic.MakeLNDynamic(vm.Region, vm.ApiId, vm.ApiKey)
		} else if vm.Type == "fake" {
			vmi = new(vmfake.Fake)
		} else {
			log.Fatalf("Encountered unrecognized VM interface type %s", vm.Type)
		}
		app.RegisterVmInterface(vm.Name, vmi)
	}

	for _, payment := range interfaceConfig.Payment {
		var pi lobster.PaymentInterface
		if payment.Type == "paypal" {
			pi = lobster.MakePaypalPayment(app, payment.Business, payment.ReturnUrl)
		} else if payment.Type == "coinbase" {
			pi = lobster.MakeCoinbasePayment(app, payment.CallbackSecret, payment.ApiKey, payment.ApiSecret)
		} else if payment.Type == "fake" {
			pi = new(lobster.FakePayment)
		} else {
			log.Fatalf("Encountered unrecognized payment interface type %s", payment.Type)
		}
		app.RegisterPaymentInterface(payment.Name, pi)
	}

	app.Run()
}
