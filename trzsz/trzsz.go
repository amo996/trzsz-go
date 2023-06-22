/*
MIT License

Copyright (c) 2023 Lonny Wong <lonnywong@qq.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package trzsz

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

type trzszArgs struct {
	Help     bool
	Version  bool
	Relay    bool
	TraceLog bool
	DragFile bool
	Name     string
	Args     []string
}

func printVersion() {
	fmt.Printf("trzsz go %s\n", kTrzszVersion)
}

func printHelp() {
	fmt.Print("usage: trzsz [-h] [-v] [-r] [-t] [-d] command line\n\n" +
		"Wrapping command line to support trzsz ( trz / tsz ).\n\n" +
		"positional arguments:\n" +
		"  command line       the original command line\n\n" +
		"optional arguments:\n" +
		"  -h, --help         show this help message and exit\n" +
		"  -v, --version      show version number and exit\n" +
		"  -r, --relay        run as a trzsz relay server\n" +
		"  -t, --tracelog     eanble trace log for debugging\n" +
		"  -d, --dragfile     enable drag file(s) to upload\n")
}

func parseTrzszArgs() *trzszArgs {
	args := &trzszArgs{}
	var i int
	for i = 1; i < len(os.Args); i++ {
		if os.Args[i] == "-h" || os.Args[i] == "--help" {
			args.Help = true
			return args
		} else if os.Args[i] == "-v" || os.Args[i] == "--version" {
			args.Version = true
			return args
		} else if os.Args[i] == "-r" || os.Args[i] == "--relay" {
			args.Relay = true
		} else if os.Args[i] == "-t" || os.Args[i] == "--tracelog" {
			args.TraceLog = true
		} else if os.Args[i] == "-d" || os.Args[i] == "--dragfile" {
			args.DragFile = true
		} else {
			break
		}
	}
	if i >= len(os.Args) {
		args.Help = true
		return args
	}
	args.Name = os.Args[i]
	args.Args = os.Args[i+1:]
	return args
}

func handleSignal(pty *trzszPty, filter *TrzszFilter) {
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	go func() {
		for range sigterm {
			pty.Terminate()
		}
	}()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	go func() {
		for range sigint {
			filter.StopTransferringFiles()
		}
	}()
}

// TrzszMain is the main function of `trzsz` binary.
func TrzszMain() int {
	// parse command line arguments
	args := parseTrzszArgs()
	if args.Help {
		printHelp()
		return 0
	}
	if args.Version {
		printVersion()
		return 0
	}

	// spawn a pty
	pty, err := spawn(args.Name, args.Args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "spawn pty failed: %#v\r\n", err)
		return -1
	}
	defer func() { pty.Close() }()

	// set stdin in raw mode
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		state, err := term.MakeRaw(fd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stdin make raw failed: %#v\r\n", err)
			return -2
		}
		defer func() { _ = term.Restore(fd, state) }()
	}

	if args.Relay {
		// run as relay
		NewTrzszRelay(os.Stdin, os.Stdout, pty.Stdin, pty.Stdout, TrzszOptions{
			DetectTraceLog: args.TraceLog,
		})
	} else {
		// new trzsz filter
		columns, err := pty.GetColumns()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pty get columns failed: %#v\r\n", err)
			return -3
		}
		filter := NewTrzszFilter(os.Stdin, os.Stdout, pty.Stdin, pty.Stdout, TrzszOptions{
			TerminalColumns: columns,
			DetectDragFile:  args.DragFile,
			DetectTraceLog:  args.TraceLog,
		})
		pty.OnResize(filter.SetTerminalColumns)
		// handle signal
		go handleSignal(pty, filter)
	}

	// wait for exit
	pty.Wait()
	return pty.ExitCode()
}
