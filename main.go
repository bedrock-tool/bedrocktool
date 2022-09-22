package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bedrock-tool/bedrocktool/bedrock-skin-bot/utils"
	"github.com/disgoorg/dislog"
	"github.com/disgoorg/snowflake"
	"golang.org/x/exp/slices"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

type Config struct {
	API struct {
		Server string
		Key    string
	}
	Discord struct {
		WebhookId    string
		WebhookToken string
	}
	Users []struct {
		Name            string
		ServerAddresses string
		Multi           bool
	}
	ServerBlacklist []string
}

func cleanup() {
	logrus.Info("\nCleaning up\n")
	for i := len(utils.G_cleanup_funcs) - 1; i >= 0; i-- { // go through cleanup functions reversed
		utils.G_cleanup_funcs[i]()
	}
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Errorf("Fatal Error occurred.")
			println("")
			println("--COPY FROM HERE--")
			logrus.Infof("Cmdline: %s", os.Args)
			logrus.Errorf("Error: %s", err)
			println("--END COPY HERE--")
			println("")
			println("if you want to report this error, please open an issue at")
			println("https://github.com/bedrock-tool/bedrocktool/issues")
			println("And attach the error info, describe what you did to get this error.")
			println("Thanks!\n")
			os.Exit(1)
		}
	}()

	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	flag.BoolVar(&utils.G_debug, "debug", false, "debug mode")
	flag.Parse()

	// exit cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		cleanup()
	}()

	var config Config

	{
		if _, err := os.Stat("config.toml"); err == nil {
			_, err := toml.DecodeFile("config.toml", &config)
			if err != nil {
				logrus.Fatal(err)
			}
		}

		{ // save config
			f, _ := os.Create("config.toml")
			defer f.Close()
			if err := toml.NewEncoder(f).Encode(&config); err != nil {
				logrus.Fatal(err)
			}
		}

		if config.API.Server == "" {
			logrus.Fatal("API.Server undefined")
		}
		if config.API.Key == "" {
			logrus.Fatal("API.Key undefined")
		}
		if len(config.Users) == 0 {
			logrus.Warn("No Users defined")
		}

		if config.Discord.WebhookId != "" {
			logrus.Info("Enabling discord Error logs")
			dlog, err := dislog.New(
				// Sets which logging levels to send to the webhook
				dislog.WithLogLevels(dislog.ErrorLevelAndAbove...),
				// Sets webhook id & token
				dislog.WithWebhookIDToken(snowflake.Snowflake(config.Discord.WebhookId), config.Discord.WebhookToken),
			)
			if err != nil {
				logrus.Fatal("error initializing dislog: ", err)
			}
			defer dlog.Close(ctx)
			logrus.StandardLogger().AddHook(dlog)
		}
	}

	{ // setup api client
		if err := utils.InitAPIClient(config.API.Server, config.API.Key); err != nil {
			logrus.Fatal(err)
		}
		if err := utils.APIClient.Start(); err != nil {
			logrus.Fatal(err)
		}
		// remove after exit
		defer utils.APIClient.Metrics.Pusher.Delete()
	}

	// starting the bots
	for _, v := range config.Users {
		addresses := strings.Split(v.ServerAddresses, " ")
		for _, address := range addresses {
			if !strings.Contains(address, ":") {
				address = address + ":19132"
			}
			for {
				IPs, err := utils.FindAllIps(address)
				if err != nil {
					logrus.Errorf("Failed to lookup ips %s", err)
					time.Sleep(30 * time.Second)
					continue
				}

				if !v.Multi {
					IPs = IPs[:1]
				}

				count := 0
				for i, ip := range IPs {
					if slices.Contains(config.ServerBlacklist, ip) {
						logrus.Info("Skipped %s for %s", ip, address)
						continue
					}
					b := NewBot(v.Name, ip, fmt.Sprintf("%s-%d", address, i))
					go b.Start(ctx)
					count += 1
				}
				logrus.Infof("Started %d Bots as %s on %s", count, v.Name, address)
				time.Sleep(10 * time.Second)
				break
			}
		}

	}

	<-ctx.Done()
	cleanup()
	os.Exit(0)
}

type TransCMD struct{}

func (*TransCMD) Name() string     { return "trans" }
func (*TransCMD) Synopsis() string { return "" }

func (c *TransCMD) SetFlags(f *flag.FlagSet) {}

func (c *TransCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *TransCMD) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	const (
		BLACK_FG = "\033[30m"
		BOLD     = "\033[1m"
		BLUE     = "\033[46m"
		PINK     = "\033[45m"
		WHITE    = "\033[47m"
		RESET    = "\033[0m"
	)
	fmt.Println(BLACK_FG + BOLD + BLUE + " Trans " + PINK + " Rights " + WHITE + " Are " + PINK + " Human " + BLUE + " Rights " + RESET)
	return 0
}

func init() {
	utils.RegisterCommand(&TransCMD{})
}
