package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/klothoplatform/klotho/pkg/compiler/types"
	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/engine/constraints"
	"github.com/klothoplatform/klotho/pkg/filter"
	"github.com/klothoplatform/klotho/pkg/graph_loader"

	"github.com/fatih/color"
	"github.com/gojek/heimdall/v7"
	"github.com/gojek/heimdall/v7/httpclient"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/klothoplatform/klotho/pkg/analytics"
	"github.com/klothoplatform/klotho/pkg/auth"
	"github.com/klothoplatform/klotho/pkg/cli_config"
	"github.com/klothoplatform/klotho/pkg/closenicely"
	"github.com/klothoplatform/klotho/pkg/compiler"
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/input"
	"github.com/klothoplatform/klotho/pkg/logging"
	"github.com/klothoplatform/klotho/pkg/updater"
)

type KlothoMain struct {
	DefaultUpdateStream string
	Version             string
	VersionQualifier    string
	PluginSetup         func(*PluginSetBuilder) error
	// Authorizer is an optional authorizer override. If this also conforms to FlagsProvider, those flags will be added.
	Authorizer Authorizer
}
type Authorizer interface {

	// Authorize tries to authorize the user. The LoginInfo may have content even if the error is non-nil. That means
	// that we got information from the user, but were not able to verify it. succeeds. You can use
	// that information provisionally (and specifically, in analytics) even if the error is non-nil, as long as you
	// don't rely on it for anything related to things like security or privacy.
	Authorize() (auth.LoginInfo, error)
}

type FlagsProvider interface {
	SetUpCliFlags(flags *pflag.FlagSet)
}

var cfg struct {
	verbose            bool
	config             string
	constructGraph     string
	guardrails         string
	outDir             string
	ast                bool
	caps               bool
	provider           string
	appName            string
	strict             bool
	disableLogo        bool
	internalDebug      bool
	version            bool
	uploadSource       bool
	update             bool
	cfgFormat          string
	setOption          map[string]string
	login              bool
	logout             bool
	skipInfrastructure bool
	skipVisualization  bool
	skipUpdateCheck    bool
	jsonLog            bool
}

const defaultDisableLogo = false

var hadWarnings = atomic.NewBool(false)
var hadErrors = atomic.NewBool(false)

const consoleEncoderName = "klotho-cli"

func init() {
	err := zap.RegisterEncoder(consoleEncoderName, func(zcfg zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return logging.NewConsoleEncoder(cfg.verbose, hadWarnings, hadErrors), nil
	})

	if err != nil {
		panic(err)
	}
}

const (
	defaultOutDir = "compiled"
)

func (km KlothoMain) Main() {
	if km.Authorizer == nil {
		km.Authorizer = auth.Auth0Authorizer{}
	}

	var root = &cobra.Command{
		Use: "klotho",
	}

	compilerCmd := &cobra.Command{
		Use:   "compile [path]",
		Short: "Compile a klotho application",
		RunE:  km.run,
	}
	root.AddCommand(compilerCmd)

	flags := compilerCmd.Flags()

	flags.BoolVarP(&cfg.verbose, "verbose", "v", false, "Verbose flag")
	flags.StringVarP(&cfg.config, "config", "c", "", "Config file")
	flags.StringVar(&cfg.constructGraph, "construct-graph", "", "Construct Graph file")
	flags.StringVar(&cfg.guardrails, "guardrails", "", "Guardrails file")
	flags.StringVarP(&cfg.outDir, "outDir", "o", defaultOutDir, "Output directory")
	flags.BoolVar(&cfg.ast, "ast", false, "Print the AST to a companion file")
	flags.BoolVar(&cfg.caps, "caps", false, "Print the capabilities to a companion file")
	flags.StringVarP(&cfg.cfgFormat, "cfg-format", "F", "yaml", "The format for the compiled config file (if --config is not specified). Supports: yaml, toml, json")
	flags.StringVar(&cfg.appName, "app", "", "Application name")
	flags.StringVarP(&cfg.provider, "provider", "p", "", fmt.Sprintf("Provider to compile to. Supported: %v", "aws"))
	flags.BoolVar(&cfg.strict, "strict", false, "Fail the compilation on warnings")
	flags.BoolVar(&cfg.disableLogo, "disable-logo", defaultDisableLogo, "Disable printing the Klotho logo")
	flags.BoolVar(&cfg.uploadSource, "upload-source", false, "Upload the compressed source folder for debugging")
	flags.BoolVar(&cfg.internalDebug, "internalDebug", false, "Enable debugging for compiler")
	flags.BoolVar(&cfg.version, "version", false, "Print the version")
	flags.BoolVar(&cfg.update, "update", false, "update the cli to the latest version")
	flags.StringToStringVar(&cfg.setOption, "set-option", nil, "Sets a CLI option")
	flags.BoolVar(&cfg.login, "login", false, "Login to Klotho with email.")
	flags.BoolVar(&cfg.logout, "logout", false, "Logout of current klotho account.")
	flags.BoolVar(&cfg.skipInfrastructure, "skip-infrastructure", false, "Skip Klotho's IaC generation stage.")
	flags.BoolVar(&cfg.skipVisualization, "skip-visualization", false, "Skip Klotho's visualization stage.")
	flags.BoolVar(&cfg.skipUpdateCheck, "skip-update-check", false, "Skip Klotho's update check.")
	flags.BoolVar(&cfg.jsonLog, "json-log", false, "Output logs in JSON format.")

	if authFlags, hasFlags := km.Authorizer.(FlagsProvider); hasFlags {
		authFlags.SetUpCliFlags(flags)
	}

	_ = flags.MarkHidden("skip-infrastructure")
	_ = flags.MarkHidden("skip-update-check")
	_ = flags.MarkHidden("internalDebug")
	_ = flags.MarkHidden("construct-graph")

	err := root.Execute()
	if err != nil {
		if cfg.internalDebug {
			zap.S().With(logging.SendEntryMessage).Errorf("%+v", err)
		} else if !root.SilenceErrors {
			zap.S().With(logging.SendEntryMessage).Errorf("%v", err)
		}
		zap.S().With(logging.SendEntryMessage).Error("Klotho compilation failed")
		os.Exit(1)
	}
	if hadWarnings.Load() && cfg.strict {
		os.Exit(1)
	}
}

func setupLogger(analyticsClient *analytics.Client) (*zap.Logger, error) {
	var zapCfg zap.Config
	if cfg.verbose {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}
	if cfg.jsonLog {
		zapCfg.Encoding = "json"
	} else {
		zapCfg.Encoding = consoleEncoderName
	}
	return zapCfg.Build(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		trackingCore := analyticsClient.NewFieldListener(zapcore.WarnLevel)
		return zapcore.NewTee(core, trackingCore)
	}))
}

func readConfig(args []string) (appCfg config.Application, err error) {
	if cfg.config != "" {
		appCfg, err = config.ReadConfig(cfg.config)
		if err != nil {
			return
		}
	} else {
		appCfg.Format = cfg.cfgFormat
	}
	appCfg.EnsureMapsExist()
	// TODO debug logging for when config file is overwritten by CLI flags
	if cfg.appName != "" {
		appCfg.AppName = cfg.appName
	}
	if cfg.provider != "" {
		appCfg.Provider = cfg.provider
	}
	if len(args) > 0 {
		appCfg.Path = args[0]
	}
	if cfg.outDir != "" {
		if appCfg.OutDir == "" || cfg.outDir != defaultOutDir {
			appCfg.OutDir = cfg.outDir
		}
	}

	return
}

func (km KlothoMain) run(cmd *cobra.Command, args []string) (err error) {
	// Save any config options. This should go before anything else, so that it always takes effect before any code
	// that uses it (for example, we should save an update.stream option before we use it below to perform the update).
	err = SetOptions(cfg.setOption)
	if err != nil {
		return err
	}
	options, err := ReadOptions()
	if err != nil {
		return err
	}

	showLogo := !(options.UI.DisableLogo.OrDefault(defaultDisableLogo) || cfg.disableLogo)
	// color.NoColor is set if we're not a terminal that
	// supports color
	if !color.NoColor && showLogo {
		color.New(color.FgHiGreen).Println(Logo)
	}

	// create config directory if necessary, must run
	// before calling analytics for first time
	if err := cli_config.CreateKlothoConfigPath(); err != nil {
		zap.S().Warnf("failed to create .klotho directory: %v", err)
	}

	// Set up analytics, and hook them up to the logs
	analyticsClient := analytics.NewClient()
	analyticsClient.AppendProperties(map[string]any{
		"version": km.Version,
		"strict":  cfg.strict,
		"edition": km.DefaultUpdateStream,
	})
	z, err := setupLogger(analyticsClient)
	if err != nil {
		return err
	}
	defer closenicely.FuncOrDebug(z.Sync)
	zap.ReplaceGlobals(z)

	// Set up user if login is specified
	if cfg.login {
		err := auth.Login(func(err error) error {
			zap.L().Warn(`Couldn't log in. You may be able to continue using klotho without logging in for now, but this may break in the future. Please contact us if this continues.`)
			// Set an empty token. This will mean that the user doesn't get prompted to log in. The login token is still
			// invalid, but it'll fail-open (at least for now).
			_ = auth.WriteIDToken("")
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	}
	// Set up user if login is specified
	if cfg.logout {
		err := auth.CallLogoutEndpoint()
		if err != nil {
			return err
		}
		return nil
	}

	errHandler := ErrorHandler{
		InternalDebug: cfg.internalDebug,
		Verbose:       cfg.verbose,
		PostPrintHook: func() {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
		},
	}
	defer analyticsClient.PanicHandler(&err, errHandler)

	updateStream := options.Update.Stream.OrDefault(km.DefaultUpdateStream)
	analyticsClient.AppendProperty("updateStream", updateStream)

	if cfg.version {
		var versionQualifier string
		if km.VersionQualifier != "" {
			versionQualifier = fmt.Sprintf("(%s)", km.VersionQualifier)
		}
		zap.S().Infof("Version: %s-%s-%s%s", km.Version, updater.OS, updater.Arch, versionQualifier)
		return nil
	}
	klothoName := "klotho"
	if km.VersionQualifier != "" {
		analyticsClient.AppendProperty(km.VersionQualifier, true)
	}

	// if update is specified do the update in place
	var klothoUpdater = updater.Updater{
		ServerURL:     updater.DefaultServer,
		Stream:        updateStream,
		CurrentStream: km.DefaultUpdateStream,
		Client: httpclient.NewClient(
			httpclient.WithHTTPTimeout(5*time.Minute),
			httpclient.WithRetryCount(3),
			httpclient.WithRetrier(heimdall.NewRetrier(heimdall.NewExponentialBackoff(10*time.Millisecond, time.Second, 1.5, 100*time.Millisecond))),
		),
	}
	if cfg.update {
		if err := klothoUpdater.Update(km.Version); err != nil {
			analyticsClient.Error(klothoName + " failed to update")
			return err
		}
		analyticsClient.Info(klothoName + " was updated successfully")
		return nil
	}

	if !cfg.skipUpdateCheck && ShouldCheckForUpdate(updateStream, km.DefaultUpdateStream, km.Version) {
		// check daily for new updates and notify users if found
		needsUpdate, err := klothoUpdater.CheckUpdate(km.Version)
		if err != nil {
			analyticsClient.Error(fmt.Sprintf(klothoName+"failed to check for updates: %v", err))
			zap.S().Warnf("failed to check for updates: %v", err)
		}
		if needsUpdate {
			analyticsClient.Info(klothoName + " update is available")
			zap.L().Info("new update is available, please run klotho --update to get the latest version")
		}
	} else {
		zap.S().Infof("Klotho is pinned to version: %s", options.Update.Stream)
	}

	if len(cfg.setOption) > 0 {
		// Options were set above, and used to perform or check for update. Nothing else to do.
		// We want to exit early, so that the user doesn't get an error about path not being provided.
		return nil
	}

	// Needs to go after the --version and --update checks
	claims, err := km.Authorizer.Authorize()
	analyticsClient.AttachAuthorizations(claims)
	if err != nil {
		if errors.Is(err, auth.ErrNoCredentialsFile) {
			return errors.New(`Failed to get credentials for user. Please run "klotho --login"`)
		}
		if errors.Is(err, auth.ErrEmailUnverified) {
			zap.L().Warn(
				`You have not verified your email. You may continue using klotho for now, but this may break in the future. Please check your email to complete registration.`,
				zap.Error(err),
				logging.SendEntryMessage)
		} else {
			// Fail-open. See also the error handler at auth.Login(...) above (you should change that to not write the
			// empty token, if this fail-open ever changes).
			zap.L().Warn(
				`Not logged in. You may be able to continue using klotho without logging in for now, but this may break in the future. Please contact us if this continues.`,
				zap.Error(err),
				logging.SendEntryMessage)
		}
	}

	appCfg, err := readConfig(args)
	if err != nil {
		return errors.Wrapf(err, "could not read config '%s'", cfg.config)
	}

	if appCfg.Path == "" {
		return errors.New("'path' required")
	}

	if appCfg.AppName == "" {
		return errors.New("'app' required")
	} else if len(appCfg.AppName) > 50 {
		analyticsClient.Error("Klotho parameter check failed. 'app' must be less than 50 characters in length")
		return fmt.Errorf("'app' must be less than 50 characters in length. 'app' was %s", appCfg.AppName)
	}
	match, err := regexp.MatchString(`^[\w-.:/]+$`, appCfg.AppName)
	if err != nil {
		return err
	} else if !match {
		analyticsClient.Error("Klotho parameter check failed. 'app' can only contain alphanumeric, -, _, ., :, and /.")
		return fmt.Errorf("'app' can only contain alphanumeric, -, _, ., :, and /. 'app' was %s", appCfg.AppName)
	}

	if appCfg.Provider == "" {
		return errors.New("'provider' required")
	}

	// Update analytics with app configs
	analyticsClient.AppendProperties(map[string]any{
		"provider": appCfg.Provider,
		"app":      appCfg.AppName,
	})

	analyticsClient.Info(klothoName + " pre-compile")

	input, err := input.ReadOSDir(appCfg, cfg.config)
	if err != nil {
		return errors.Wrapf(err, "could not read root path %s", appCfg.Path)
	}

	if cfg.ast {
		if err = OutputAST(input, appCfg.OutDir); err != nil {
			return errors.Wrap(err, "could not output helpers")
		}
	}
	if cfg.caps {
		if err = OutputCapabilities(input, appCfg.OutDir); err != nil {
			return errors.Wrap(err, "could not output helpers")
		}
	}
	var guardrails []byte
	if cfg.guardrails != "" {
		f, err := os.ReadFile(cfg.guardrails)
		if err != nil {
			return err
		}
		guardrails = f
	}
	plugins := &PluginSetBuilder{
		Cfg:        &appCfg,
		GuardRails: guardrails,
	}
	if cfg.constructGraph != "" {
		err = plugins.AddEngine()
		if err != nil {
			return err
		}
		err = plugins.AddPulumi()
		if err != nil {
			return err
		}
		err = plugins.AddVisualizerPlugin()
		if err != nil {
			return err
		}
	} else {
		err = km.PluginSetup(plugins)

		if err != nil {
			return err
		}
	}

	document := &compiler.CompilationDocument{
		InputFiles:       input,
		FileDependencies: &types.FileDependencies{},
		Constructs:       construct.NewConstructGraph(),
		Configuration:    &appCfg,
		OutputOptions:    options.Output,
	}

	// disable unnecessary plugins
	iacPlugins := plugins.IaC
	if cfg.skipVisualization {
		iacPlugins = filterVisualizerPlugin(iacPlugins)
	}
	if cfg.skipInfrastructure {
		iacPlugins = filterInfrastructure(iacPlugins)
	}

	klothoCompiler := compiler.Compiler{
		AnalysisAndTransformationPlugins: plugins.AnalysisAndTransform,
		IaCPlugins:                       iacPlugins,
		Engine:                           plugins.Engine,
		Document:                         document,
	}
	klothoCompiler.Engine.Context.InitialState = document.Constructs

	if cfg.constructGraph != "" {
		klothoCompiler.AnalysisAndTransformationPlugins = nil
		cg, err := graph_loader.LoadConstructGraphFromFile(cfg.constructGraph)
		if err != nil {
			return errors.Errorf("failed to load construct graph: %s", err.Error())
		}
		document.Constructs = cg
		c, err := constraints.LoadConstraintsFromFile(cfg.constructGraph)
		if err != nil {
			return errors.Errorf("failed to load constraints: %s", err.Error())
		}

		klothoCompiler.Engine.LoadContext(document.Constructs, c, cfg.appName)
		dag, err := klothoCompiler.Engine.Run()
		if err != nil {
			return errors.Errorf("failed to run engine: %s", err.Error())
		}
		zap.S().Debugf("Finished running engine")
		files, err := klothoCompiler.Engine.VisualizeViews()
		if err != nil {
			return errors.Errorf("failed to run engine viz: %s", err.Error())
		}
		document.OutputFiles = append(document.OutputFiles, files...)
		document.Resources = dag
		document.DeploymentOrder = klothoCompiler.Engine.GetDeploymentOrderGraph(dag)
		err = klothoCompiler.Document.Resources.OutputResourceGraph(cfg.outDir)
		if err != nil {
			return err
		}
		err = document.OutputTo(appCfg.OutDir)
		if err != nil {
			return err
		}
	} else {
		document.Resources = construct.NewResourceGraph()
	}

	analyticsClient.Info(klothoName + " compiling")

	err = klothoCompiler.Compile()
	if err != nil || hadErrors.Load() {
		if err != nil {
			errHandler.PrintErr(err)
		} else {
			err = errors.New("Failed run of klotho invocation")
		}
		analyticsClient.Error(klothoName + " compiling failed")

		return err
	}

	if cfg.uploadSource {
		analyticsClient.UploadSource(input)
	}

	resourceCounts, err := document.OutputResources()
	if err != nil {
		return err
	}
	CloseTreeSitter(document.Constructs)
	analyticsClient.AppendProperties(map[string]any{
		"resource_types": GetResourceTypeCount(document.Constructs, &appCfg),
		"languages":      GetLanguagesUsed(document.Constructs),
		"resources":      GetResourceCount(resourceCounts),
	})
	analyticsClient.Info(klothoName + " compile complete")

	return nil
}

func filterInfrastructure(plugins []compiler.IaCPlugin) []compiler.IaCPlugin {
	return filter.NewSimpleFilter[compiler.IaCPlugin](func(p compiler.IaCPlugin) bool {
		return !strings.HasPrefix(p.Name(), "pulumi")
	}).Apply(plugins...)
}

func filterVisualizerPlugin(plugins []compiler.IaCPlugin) []compiler.IaCPlugin {
	return filter.NewSimpleFilter[compiler.IaCPlugin](func(p compiler.IaCPlugin) bool {
		return p.Name() != "visualizer"
	}).Apply(plugins...)
}
