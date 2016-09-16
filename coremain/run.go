package coremain

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/mholt/caddy"

	// Plug in CoreDNS
	"github.com/miekg/coredns/core"
)

func init() {
	caddy.TrapSignals()
	caddy.DefaultConfigFile = "Corefile"
	caddy.Quiet = true // don't show init stuff from caddy
	setVersion()

	flag.StringVar(&conf, "conf", "", "Corefile to load (default \""+caddy.DefaultConfigFile+"\")")
	flag.StringVar(&cpu, "cpu", "100%", "CPU cap")
	flag.BoolVar(&plugins, "plugins", false, "List installed plugins")
	flag.StringVar(&logfile, "log", "", "Process log file")
	flag.StringVar(&caddy.PidFile, "pidfile", "", "Path to write pid file")
	flag.BoolVar(&core.Quiet, "quiet", false, "Quiet mode (no initialization output)")
	flag.BoolVar(&version, "version", false, "Show version")

	caddy.RegisterCaddyfileLoader("flag", caddy.LoaderFunc(confLoader))
	caddy.SetDefaultCaddyfileLoader("default", caddy.LoaderFunc(defaultLoader))
}

// Run is CoreDNS's main() function.
func Run() {
	flag.Parse()

	caddy.AppName = coreName
	caddy.AppVersion = coreVersion

	// Set up process log before anything bad happens
	switch logfile {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	default:
		log.SetOutput(os.Stdout)
	}
	log.SetFlags(log.LstdFlags)

	if version {
		showVersion()
		os.Exit(0)
	}
	if plugins {
		fmt.Println(caddy.DescribePlugins())
		os.Exit(0)
	}

	// Set CPU cap
	if err := setCPU(cpu); err != nil {
		mustLogFatal(err)
	}

	// Get Corefile input
	corefile, err := caddy.LoadCaddyfile(serverType)
	if err != nil {
		mustLogFatal(err)
	}

	// Start your engines
	instance, err := caddy.Start(corefile)
	if err != nil {
		mustLogFatal(err)
	}

	logVersion()

	// Twiddle your thumbs
	instance.Wait()
}

// startNotification will log CoreDNS' version to the log.
func startupNotification() {
	if core.Quiet {
		return
	}
	logVersion()
}

func showVersion() {
	fmt.Printf("%s-%s\n", caddy.AppName, caddy.AppVersion)
	if devBuild && gitShortStat != "" {
		fmt.Printf("%s\n%s\n", gitShortStat, gitFilesModified)
	}
}

// logVersion logs the version that is starting.
func logVersion() {
	log.Printf("[INFO] %s-%s starting\n", caddy.AppName, caddy.AppVersion)
	if devBuild && gitShortStat != "" {
		log.Printf("[INFO] %s\n%s\n", gitShortStat, gitFilesModified)
	}
}

// mustLogFatal wraps log.Fatal() in a way that ensures the
// output is always printed to stderr so the user can see it
// if the user is still there, even if the process log was not
// enabled. If this process is an upgrade, however, and the user
// might not be there anymore, this just logs to the process
// log and exits.
func mustLogFatal(args ...interface{}) {
	if !caddy.IsUpgrade() {
		log.SetOutput(os.Stderr)
	}
	log.Fatal(args...)
}

// confLoader loads the Caddyfile using the -conf flag.
func confLoader(serverType string) (caddy.Input, error) {
	if conf == "" {
		return nil, nil
	}

	if conf == "stdin" {
		return caddy.CaddyfileFromPipe(os.Stdin, "dns")
	}

	contents, err := ioutil.ReadFile(conf)
	if err != nil {
		return nil, err
	}
	return caddy.CaddyfileInput{
		Contents:       contents,
		Filepath:       conf,
		ServerTypeName: serverType,
	}, nil
}

// defaultLoader loads the Corefile from the current working directory.
func defaultLoader(serverType string) (caddy.Input, error) {
	contents, err := ioutil.ReadFile(caddy.DefaultConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return caddy.CaddyfileInput{
		Contents:       contents,
		Filepath:       caddy.DefaultConfigFile,
		ServerTypeName: serverType,
	}, nil
}

// setVersion figures out the version information
// based on variables set by -ldflags.
func setVersion() {
	// A development build is one that's not at a tag or has uncommitted changes
	devBuild = gitTag == "" || gitShortStat != ""

	// Only set the appVersion if -ldflags was used
	if gitNearestTag != "" || gitTag != "" {
		if devBuild && gitNearestTag != "" {
			appVersion = fmt.Sprintf("%s (+%s %s)",
				strings.TrimPrefix(gitNearestTag, "v"), gitCommit, buildDate)
		} else if gitTag != "" {
			appVersion = strings.TrimPrefix(gitTag, "v")
		}
	}
}

// setCPU parses string cpu and sets GOMAXPROCS
// according to its value. It accepts either
// a number (e.g. 3) or a percent (e.g. 50%).
func setCPU(cpu string) error {
	var numCPU int

	availCPU := runtime.NumCPU()

	if strings.HasSuffix(cpu, "%") {
		// Percent
		var percent float32
		pctStr := cpu[:len(cpu)-1]
		pctInt, err := strconv.Atoi(pctStr)
		if err != nil || pctInt < 1 || pctInt > 100 {
			return errors.New("invalid CPU value: percentage must be between 1-100")
		}
		percent = float32(pctInt) / 100
		numCPU = int(float32(availCPU) * percent)
	} else {
		// Number
		num, err := strconv.Atoi(cpu)
		if err != nil || num < 1 {
			return errors.New("invalid CPU value: provide a number or percent greater than 0")
		}
		numCPU = num
	}

	if numCPU > availCPU {
		numCPU = availCPU
	}

	runtime.GOMAXPROCS(numCPU)
	return nil
}

// Flags that control program flow or startup
var (
	conf    string
	cpu     string
	logfile string
	version bool
	plugins bool
)

// Build information obtained with the help of -ldflags
var (
	appVersion = "(untracked dev build)" // inferred at startup
	devBuild   = true                    // inferred at startup

	buildDate        string // date -u
	gitTag           string // git describe --exact-match HEAD 2> /dev/null
	gitNearestTag    string // git describe --abbrev=0 --tags HEAD
	gitCommit        string // git rev-parse HEAD
	gitShortStat     string // git diff-index --shortstat
	gitFilesModified string // git diff-index --name-only HEAD
)