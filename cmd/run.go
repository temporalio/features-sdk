package cmd

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/harness/go/harness"
	"github.com/temporalio/features/harness/go/history"
	"github.com/temporalio/features/sdkbuild"
	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/testsuite"
)

const (
	summaryListenAddr = "127.0.0.1:0"
	FeaturePassed     = "PASSED"
)

func runCmd() *cli.Command {
	var config RunConfig
	return &cli.Command{
		Name:  "run",
		Usage: "run a test or set of tests",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return NewRunner(config).Run(ctx.Context, ctx.Args().Slice())
		},
	}
}

type SummaryEntry struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
	Message string `json:"message"`
}

type Summary []SummaryEntry

// RunConfig is configuration for NewRunner.
type RunConfig struct {
	PrepareConfig
	Server              string
	Namespace           string
	ClientCertPath      string
	ClientKeyPath       string
	GenerateHistory     bool
	DisableHistoryCheck bool
	RetainTempDir       bool
	SummaryURI          string
}

// dockerRunFlags are a subset of flags that apply when running in a docker container
func (r *RunConfig) dockerRunFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "server",
			Usage:       "The host:port of the server (default is to create ephemeral in-memory server)",
			Destination: &r.Server,
		},
		&cli.StringFlag{
			Name:        "namespace",
			Usage:       "The namespace to use (default is random)",
			Destination: &r.Namespace,
		},
		&cli.StringFlag{
			Name:        "client-cert-path",
			Usage:       "Path of TLS client cert to use (optional)",
			Destination: &r.ClientCertPath,
		},
		&cli.StringFlag{
			Name:        "client-key-path",
			Usage:       "Path of TLS client key to use (optional)",
			Destination: &r.ClientKeyPath,
		},
	}
}

func (r *RunConfig) flags() []cli.Flag {
	return append([]cli.Flag{
		langFlag(&r.Lang),
		&cli.StringFlag{
			Name: "version",
			Usage: "SDK language version to run. Most languages support versions as paths. " +
				"Version cannot be present if prepared directory is.",
			Destination: &r.Version,
		},
		&cli.BoolFlag{
			Name:        "generate-history",
			Usage:       "Generate the history of the features that are run (overwrites any existing history)",
			Destination: &r.GenerateHistory,
		},
		&cli.BoolFlag{
			Name:        "no-history-check",
			Usage:       "Do not verify history matches",
			Destination: &r.DisableHistoryCheck,
		},
		&cli.BoolFlag{
			Name:        "retain-temp-dir",
			Usage:       "Do not delete the temp directory after the run",
			Destination: &r.RetainTempDir,
		},
		&cli.StringFlag{
			Name:        "prepared-dir",
			Usage:       "Relative directory already prepared. Cannot include version with this.",
			Destination: &r.DirName,
		},
	}, r.dockerRunFlags()...)
}

// Runner can run features.
type Runner struct {
	log    log.Logger
	config RunConfig
	// Root of the features repo
	rootDir    string
	createTime time.Time
	program    sdkbuild.Program
}

// NewRunner creates a new runner for the given config.
func NewRunner(config RunConfig) *Runner {
	return &Runner{
		// TODO(cretz): Configurable logger
		log:        harness.NewCLILogger(),
		config:     config,
		rootDir:    rootDir(),
		createTime: time.Now(),
	}
}

// Run runs all matching features for the given patterns (or all if no patterns
// given).
func (r *Runner) Run(ctx context.Context, patterns []string) error {
	var err error
	if r.config.Lang, err = normalizeLangName(r.config.Lang); err != nil {
		return err
	}

	// Cannot generate history if a version isn't provided explicitly
	if r.config.GenerateHistory && r.config.Version == "" {
		return fmt.Errorf("must have explicit version to generate history")
	}

	// If prepared dir given, validate
	if r.config.DirName != "" {
		if strings.ContainsAny(r.config.DirName, `\/`) {
			return fmt.Errorf("prepared directory must not have path separators, it is always relative to the SDK features root")
		} else if r.config.Version != "" {
			return fmt.Errorf("cannot provide version with prepared directory")
		}
		if _, err := os.Stat(filepath.Join(r.rootDir, r.config.DirName)); err != nil {
			return fmt.Errorf("failed checking prepared directory: %w", err)
		}
	}

	// If the namespace is not set, set it ourselves
	if r.config.Namespace == "" {
		r.config.Namespace = "features-ns-" + uuid.NewString()
	}

	// Collect features to run
	features, err := r.GlobFeatures(patterns)
	if err != nil {
		return err
	} else if len(features) == 0 {
		return fmt.Errorf("no features matched")
	} else if len(features) > 1 && r.config.GenerateHistory {
		return fmt.Errorf("can only specify a single feature when generating history")
	}
	// Aa task queue to every feature
	run := &cmd.Run{Features: make([]cmd.RunFeature, len(features))}
	for i, feature := range features {
		run.Features[i].Dir = feature.Dir
		run.Features[i].TaskQueue = fmt.Sprintf("features-%v-%v", feature.Dir, uuid.NewString())
		run.Features[i].Config = feature.Config
	}

	// If the server is not set, start it ourselves
	if r.config.Server == "" {
		server, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{
			// TODO(cretz): Configurable?
			LogLevel:      "error",
			ClientOptions: &client.Options{Namespace: r.config.Namespace},
			ExtraArgs: []string{
				"--dynamic-config-value", "system.forceSearchAttributesCacheRefreshOnRead=true",
				"--dynamic-config-value", "system.enableActivityEagerExecution=true",
				"--dynamic-config-value", "frontend.enableUpdateWorkflowExecution=true",
				"--dynamic-config-value", "frontend.enableUpdateWorkflowExecutionAsyncAccepted=true",
			},
		})
		if err != nil {
			return fmt.Errorf("failed starting devserver: %w", err)
		}
		defer server.Stop()
		r.config.Server = server.FrontendHostPort()
		r.log.Info("Started server", "HostPort", r.config.Server)
	} else {
		// Wait for namespace to become available
		err := harness.WaitNamespaceAvailable(ctx, r.log,
			r.config.Server, r.config.Namespace, r.config.ClientCertPath, r.config.ClientKeyPath)
		if err != nil {
			return err
		}
	}

	// Ensure any created temp dir is cleaned on ctrl-c or normal exit
	if r.config.DirName == "" && !r.config.RetainTempDir {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			r.destroyTempDir()
			os.Exit(1)
		}()
		defer r.destroyTempDir()
	}

	l, err := net.Listen("tcp", summaryListenAddr)
	if err != nil {
		return err
	}
	defer l.Close()
	summaryChan := make(chan Summary)
	go r.summaryServer(l, summaryChan)
	r.config.SummaryURI = "tcp://" + l.Addr().String()

	err = nil
	switch r.config.Lang {
	case "go":
		// If there's a version or prepared dir we run external, otherwise we run local
		if r.config.Version != "" || r.config.DirName != "" {
			if r.config.DirName != "" {
				r.program, err = sdkbuild.GoProgramFromDir(filepath.Join(r.rootDir, r.config.DirName))
			}
			if err == nil {
				err = r.RunGoExternal(ctx, run)
			}
		} else {
			err = cmd.NewRunner(cmd.RunConfig{
				Server:         r.config.Server,
				Namespace:      r.config.Namespace,
				ClientCertPath: r.config.ClientCertPath,
				ClientKeyPath:  r.config.ClientKeyPath,
				SummaryURI:     r.config.SummaryURI,
			}).Run(ctx, run)
		}
	case "java":
		if r.config.DirName != "" {
			r.program, err = sdkbuild.JavaProgramFromDir(filepath.Join(r.rootDir, r.config.DirName))
		}
		if err == nil {
			err = r.RunJavaExternal(ctx, run)
		}
	case "ts":
		if r.config.DirName != "" {
			r.program, err = sdkbuild.TypeScriptProgramFromDir(filepath.Join(r.rootDir, r.config.DirName))
		}
		if err == nil {
			err = r.RunTypeScriptExternal(ctx, run)
		}
	case "py":
		if r.config.DirName != "" {
			r.program, err = sdkbuild.PythonProgramFromDir(filepath.Join(r.rootDir, r.config.DirName))
		}
		if err == nil {
			err = r.RunPythonExternal(ctx, run)
		}
	case "cs":
		if r.config.DirName != "" {
			r.program, err = sdkbuild.DotNetProgramFromDir(filepath.Join(r.rootDir, r.config.DirName))
		}
		if err == nil {
			err = r.RunDotNetExternal(ctx, run)
		}
	default:
		err = fmt.Errorf("unrecognized language")
	}
	if err != nil {
		return err
	}
	l.Close()
	summary, ok := <-summaryChan
	if !ok {
		r.log.Debug("did not receive a test run summary - adopting legacy behavior of assuming no tests were skipped")
		for _, feature := range run.Features {
			summary = append(summary, SummaryEntry{Name: feature.Dir, Outcome: FeaturePassed})
		}
	}
	return r.handleHistory(ctx, run, summary)
}

func (r *Runner) handleHistory(ctx context.Context, run *cmd.Run, summary Summary) error {
	// Handle each
	var cl client.Client
	var failureCount int
	for _, feature := range run.Features {
		// We ignore history if there are no workflows
		if feature.Config.NoWorkflow {
			continue
		}
		entry, ok := summary.Find(feature.Dir)
		if !ok {
			r.log.Info("skipping history check because feature not listed in execution summary", "feature", feature.Dir)
			continue
		}
		if entry.Outcome == "SKIPPED" {
			r.log.Info("skipping history check because feature was skipped", "feature", feature.Dir, "reason", entry.Message)
			continue
		}

		// Dial client if not already done
		if cl == nil {
			opts := client.Options{
				HostPort:  r.config.Server,
				Namespace: r.config.Namespace,
				Logger:    r.log,
			}
			if r.config.ClientCertPath != "" {
				cert, err := tls.LoadX509KeyPair(r.config.ClientCertPath, r.config.ClientKeyPath)
				if err != nil {
					return fmt.Errorf("failed to load certs: %s", err)
				}
				opts.ConnectionOptions.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
			}
			var err error
			if cl, err = client.Dial(opts); err != nil {
				return fmt.Errorf("failed creating client: %w", err)
			}
			defer cl.Close()
		}

		// Check history
		if err := r.handleSingleHistory(ctx, cl, feature); err != nil {
			failureCount++
			r.log.Error("Feature history handling failed", "Feature", feature.Dir, "error", err)
		}
	}
	if failureCount > 0 {
		return fmt.Errorf("%v failure(s) reported", failureCount)
	}
	return nil
}

func (r *Runner) handleSingleHistory(ctx context.Context, client client.Client, feature cmd.RunFeature) error {
	// Obtain current history from the server even no history checking/generating
	fetcher := history.Fetcher{
		Client:         client,
		Namespace:      r.config.Namespace,
		TaskQueue:      feature.TaskQueue,
		FeatureStarted: r.createTime,
	}
	storage := history.Storage{Dir: filepath.Join(r.rootDir, "features", feature.Dir, "history"), Lang: r.config.Lang}
	// Load all histories from storage to validate against
	existingSet, err := storage.Load()
	if err != nil {
		return err
	}
	if !r.config.GenerateHistory && len(existingSet.ByVersion) == 0 {
		r.log.Info("Skipping history check since nothing to check against and not generating",
			"Feature", feature.Dir)
		return nil
	}
	currHist, err := fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed getting history: %w", err)
	}

	// Do a check against all scrubbed existing histories to ensure nothing
	// changed
	if !r.config.DisableHistoryCheck {

		// Check that all versions of history match the current one when scrubbed
		// TODO(cretz): Allow some versions to ignore histories from other versions
		currHistScrubbed := currHist.Clone()
		currHistScrubbed.ScrubRunSpecificFields()
		for version, existingHist := range existingSet.ByVersion {
			// Scrub, then check equality
			existingHist.ScrubRunSpecificFields()
			if !currHistScrubbed.Equals(existingHist) {
				// Convert both to JSON because it shows a better diff
				actualJSON, err := json.MarshalIndent(currHistScrubbed, "", "  ")
				if err != nil {
					return err
				}
				expectedJSON, err := json.MarshalIndent(existingHist, "", "  ")
				if err != nil {
					return err
				}
				// Technically, in Go, the version may be empty
				currVersion := r.config.Version
				if currVersion == "" {
					currVersion = "<current>"
				}
				// Use the same diff lib testify assertion uses
				diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
					A:        difflib.SplitLines(string(expectedJSON)),
					B:        difflib.SplitLines(string(actualJSON)),
					FromFile: feature.Dir + "/history/history." + r.config.Lang + "." + version + ".json",
					FromDate: "",
					ToFile:   feature.Dir + "/history/history." + r.config.Lang + "." + currVersion + ".json",
					ToDate:   "",
					Context:  10,
				})
				// We are going to just dump this to log since it has a multiline output
				// that Zap is not cool with in a tag
				// TODO(cretz): Make equality output more configurable?
				r.log.Error("History check failed, diff:\n" + diff)
				return fmt.Errorf("on feature %v, history with current version %v didn't match version %v",
					feature.Dir, currVersion, version)
			}
		}
	}

	// Store history, overwriting if necessary
	if r.config.GenerateHistory {
		err = storage.Store(&history.StoredSet{ByVersion: map[string]history.Histories{r.config.Version: currHist}})
		if err != nil {
			return fmt.Errorf("failed storing history for %v: %w", feature.Dir, err)
		}
	}
	return nil
}

// summaryServer uses the supplied listener to handle a single incoming
// connection that sends JSONL data describing the execution status of feature
// tests as determined by a lower level test execution harness. JSONL data items
// are expected to have the following fields
//   - Name (string) the name of the test
//   - Outcome (string) one of PASSED|FAILED|SKIPPED
//   - Message (string) a free text field
func (r *Runner) summaryServer(l net.Listener, out chan<- Summary) {
	conn, err := l.Accept()
	if err != nil {
		// Accept returns an error if the listener is closed
		close(out)
		return
	}
	summary := Summary{}
	rdr := bufio.NewReaderSize(conn, 4096)
	for {
		line, err := rdr.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			r.log.Error("error reading from summary socket", "error", err.Error())
			close(out)
			return
		}
		var entry SummaryEntry
		err = json.Unmarshal(line, &entry)
		if err != nil {
			r.log.Error("error unmarshalling summary entry", "error", err.Error(), "line", string(line))
			continue
		}
		summary = append(summary, entry)
	}
	out <- summary
}

func rootDir() string {
	_, currFile, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(currFile))
}

func (r *Runner) destroyTempDir() {
	if r.program != nil {
		_ = os.RemoveAll(r.program.Dir())
	}
}

func normalizeLangName(lang string) (string, error) {
	// Normalize to file extension
	switch lang {
	case "go", "java", "ts", "py", "cs":
	case "typescript":
		lang = "ts"
	case "python":
		lang = "py"
	case "dotnet", "csharp":
		lang = "cs"
	default:
		return "", fmt.Errorf("invalid language %q, must be one of: go or java or ts or py or cs", lang)
	}
	return lang, nil
}

func expandLangName(lang string) (string, error) {
	// Expand to lang name
	switch lang {
	case "go", "java", "typescript", "python":
	case "ts":
		lang = "typescript"
	case "py":
		lang = "python"
	case "cs":
		lang = "dotnet"
	default:
		return "", fmt.Errorf("invalid language %q, must be one of: go or java or ts or py or cs", lang)
	}
	return lang, nil
}

func langFlag(destination *string) *cli.StringFlag {
	return &cli.StringFlag{
		Name:        "lang",
		Usage:       "SDK language to run ('go' or 'java' or 'ts' or 'py' or 'cs')",
		Required:    true,
		Destination: destination,
	}
}

func (s Summary) Find(featureName string) (*SummaryEntry, bool) {
	for _, entry := range s {
		if entry.Name == featureName {
			return &entry, true
		}
	}
	return nil, false
}
