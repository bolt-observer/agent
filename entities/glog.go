package entities

import (
	"flag"
	"fmt"

	cli "github.com/urfave/cli"
)

func glogFlagShim(fakeVals map[string]string) {
	flag.VisitAll(func(fl *flag.Flag) {
		if val, ok := fakeVals[fl.Name]; ok {
			fl.Value.Set(val)
		}
	})
}

func GlogShim(c *cli.Context) {
	_ = flag.CommandLine.Parse([]string{})
	glogFlagShim(map[string]string{
		"v":                fmt.Sprint(c.Int("verbosity")),
		"logtostderr":      fmt.Sprint(c.Bool("logtostderr")),
		"stderrthreshold":  fmt.Sprint(c.Int("stderrthreshold")),
		"alsologtostderr":  fmt.Sprint(c.Bool("alsologtostderr")),
		"vmodule":          c.String("vmodule"),
		"log_dir":          c.String("log_dir"),
		"log_backtrace_at": c.String("log_backtrace_at"),
	})
}

var GlogFlags = []cli.Flag{
	cli.IntFlag{
		Name: "verbosity", Value: 0, Usage: "log level for V logs", Hidden: false,
	},
	cli.BoolFlag{
		Name: "logtostderr", Usage: "log to standard error instead of files", Hidden: true,
	},
	cli.IntFlag{
		Name:  "stderrthreshold",
		Usage: "logs at or above this threshold go to stderr", Hidden: true,
	},
	cli.BoolFlag{
		Name: "alsologtostderr", Usage: "log to standard error as well as files", Hidden: true,
	},
	cli.StringFlag{
		Name:  "vmodule",
		Usage: "comma-separated list of pattern=N settings for file-filtered logging", Hidden: true,
	},
	cli.StringFlag{
		Name: "log_dir", Usage: "If non-empty, write log files in this directory", Hidden: true,
	},
	cli.StringFlag{
		Name:  "log_backtrace_at",
		Usage: "when logging hits line file:N, emit a stack trace", Hidden: true,
		Value: ":0",
	},
}
