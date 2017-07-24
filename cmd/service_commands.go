package cmd

import (
	"github.com/kardianos/service"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"time"
	"github.com/spf13/viper"
)

var (
	serviceName        string
	serviceDescription string
	serviceConfig      *service.Config
)

type nsgParserService struct {
	cmd *cobra.Command
}

func (p *nsgParserService) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	log.Infof("Start() service at %v", time.Now())
	go p.run()
	return nil
}

func (p *nsgParserService) run() {
	initClient()
	initFileClient()
	daemon = true
	processCmd.Run(processCmd, []string{})
}

func (p *nsgParserService) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	log.Infof("stopping service at %v", time.Now())
	return nil
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage nsg-parser service",
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		RootCmd.PersistentPreRun(RootCmd, args)
		initServiceParams()
	},
}

var installServiceCmd = &cobra.Command{
	Use:   "install",
	Short: "Install/Reinstall nsg-parser service",
	Run: func(cmd *cobra.Command, args []string) {
		if cfgFile, err := cfgFilePath(); err != nil {
			stdoutLog.WithField("config_file", cfgFile).
				Fatal("unable to load provided config file. exiting")
		} else {
			stdoutLog.WithField("config_file", cfgFile).
				Info("installing service with config")
			serviceConfig.Arguments = []string{"process", "service", "run", "--config", cfgFile}
		}
		prog := &nsgParserService{}
		prog.cmd = cmd
		s, err := service.New(prog, serviceConfig)
		if err != nil {
			stdoutLog.Fatal(err)
		}

		_ = s.Stop()
		_ = s.Uninstall()

		err = s.Install()

		if err != nil {
			stdoutLog.Fatalf("error while installing service %s", err)
		} else {
			stdoutLog.Infof("installed service")
		}

	},
}

var uninstallServiceCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall nsg-parser service",
	Run: func(cmd *cobra.Command, args []string) {
		prog := &nsgParserService{}
		prog.cmd = cmd

		s, err := service.New(prog, serviceConfig)
		if err != nil {
			stdoutLog.Fatal(err)
		}

		_ = s.Stop()
		err = s.Uninstall()
		if err != nil {
			stdoutLog.Fatalf("error while uninstalling service %s", err)
		} else {
			stdoutLog.Infof("uninstalled service")
		}
	},
}

var runServiceCmd = &cobra.Command{
	Use:   "run",
	Short: "run nsg-parser service",
	Run: func(cmd *cobra.Command, args []string) {
		prog := &nsgParserService{}
		prog.cmd = cmd

		s, err := service.New(prog, serviceConfig)
		if err != nil {
			stdoutLog.Fatal(err)
		}

		log.Infof("starting service at %v", time.Now())
		err = s.Run()
		if err != nil {
			log.Fatalf("error while running service %s", err)
		}
	},
}

func init() {
	serviceCmd.PersistentFlags().StringVar(&serviceName, "service_name", "nsg-parser", "Service Name")
	serviceCmd.PersistentFlags().StringVar(&serviceDescription, "service_description", "Parser for MS Azure NSG Flow Logs", "Service Description")

	processCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(installServiceCmd)
	serviceCmd.AddCommand(uninstallServiceCmd)
	serviceCmd.AddCommand(runServiceCmd)
}

func initServiceParams() {
	serviceName = viper.GetString("service_name")
	serviceDescription = viper.GetString("service_description")
	stdoutLog.Infof("configuring service [%s]", serviceName)
	serviceConfig = &service.Config{
		Name:        serviceName,
		DisplayName: serviceName,
		Description: serviceDescription,
	}
}
