package plugintest

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/configs"
	"github.com/hashicorp/terraform/configs/configload"
	"github.com/hashicorp/terraform/helper/logging"
	"github.com/hashicorp/terraform/plans"
	tfplugin "github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/terraform"
	"github.com/hashicorp/terraform/tfdiags"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	sdk "github.com/hashicorp/terraform-plugin-sdk"
	"github.com/hashicorp/terraform-plugin-sdk/plugintest/internal/initwd"
	pb "github.com/hashicorp/terraform-plugin-sdk/tfplugin5"
)

const testEnvVar = "TF_ACC"

// TestT is the interface used to handle the test lifecycle of a test.
//
// Users should just use a *testing.T object, which implements this.
type TestT interface {
	Error(args ...interface{})
	Fatal(args ...interface{})
	Skip(args ...interface{})
	Name() string
	Parallel()
	Helper()
}

// TestCase is a single acceptance test case used to test the apply/destroy
// lifecycle of a resource in a specific configuration.
//
// When the destroy plan is executed, the config from the last TestStep
// is used to plan it.
type TestCase struct {
	Providers map[string]sdk.Provider

	// IsUnitTest allows a test to run regardless of the TF_ACC
	// environment variable. This should be used with care - only for
	// fast tests on local resources (e.g. remote state with a local
	// backend) but can be used to increase confidence in correct
	// operation of Terraform without waiting for a full acctest run.
	IsUnitTest bool

	// PreCheck, if non-nil, will be called before any test steps are
	// executed. It will only be executed in the case that the steps
	// would run, so it can be used for some validation before running
	// acceptance tests, such as verifying that keys are setup.
	PreCheck func()

	// PreventPostDestroyRefresh can be set to true for cases where data sources
	// are tested alongside real resources
	PreventPostDestroyRefresh bool

	// CheckDestroy is called after the resource is finally destroyed
	// to allow the tester to test that the resource is truly gone.
	CheckDestroy TestCheckFunc

	// Steps are the apply sequences done within the context of the
	// same state. Each step can have its own check to verify correctness.
	Steps []TestStep
}

// TestCheckFunc is the callback type used with acceptance tests to check
// the state of a resource. The state passed in is the latest state known,
// or in the case of being after a destroy, it is the last known state when
// it was created.
type TestCheckFunc func(*states.State) error

// TestStep is a single apply sequence of a test, done within the
// context of a state.
//
// Multiple TestSteps can be sequenced in a Test to allow testing
// potentially complex update logic. In general, simply create/destroy
// tests will only need one step.
type TestStep struct {
	// PreConfig is called before the Config is applied to perform any per-step
	// setup that needs to happen. This is called regardless of "test mode"
	// below.
	PreConfig func()

	// Config a string of the configuration to give to Terraform. If this
	// is set, then the TestCase will execute this step with the same logic
	// as a `terraform apply`.
	Config string

	// Check is called after the Config is applied. Use this step to
	// make your own API calls to check the status of things, and to
	// inspect the format of the ResourceState itself.
	//
	// If an error is returned, the test will fail. In this case, a
	// destroy plan will still be attempted.
	//
	// If this is nil, no check is done on this step.
	Check TestCheckFunc

	// Destroy will create a destroy plan if set to true.
	Destroy bool

	// ExpectNonEmptyPlan can be set to true for specific types of tests that are
	// looking to verify that a diff occurs
	ExpectNonEmptyPlan bool

	// ExpectError allows the construction of test cases that we expect to fail
	// with an error. The specified regexp must match against the error for the
	// test to pass.
	ExpectError *regexp.Regexp

	// PlanOnly can be set to only run `plan` with this configuration, and not
	// actually apply it. This is useful for ensuring config changes result in
	// no-op plans
	PlanOnly bool

	// PreventPostDestroyRefresh can be set to true for cases where data sources
	// are tested alongside real resources
	PreventPostDestroyRefresh bool
}

// UnitTest is a helper to force the acceptance testing harness to run in the
// normal unit test suite. This should only be used for resource that don't
// have any external dependencies.
func UnitTest(t TestT, c TestCase) {
	t.Helper()

	c.IsUnitTest = true
	Test(t, c)
}

// Test performs an acceptance test on a resource.
//
// Tests are not run unless an environmental variable "TF_ACC" is
// set to some non-empty value. This is to avoid test cases surprising
// a user by creating real resources.
//
// Tests will fail unless the verbose flag (`go test -v`, or explicitly
// the "-test.v" flag) is set. Because some acceptance tests take quite
// long, we require the verbose flag so users are able to see progress
// output.
func Test(t TestT, c TestCase) {
	t.Helper()

	// We only run acceptance tests if an env var is set because they're
	// slow and generally require some outside configuration. You can opt out
	// of this with OverrideEnvVar on individual TestCases.
	if os.Getenv(testEnvVar) == "" && !c.IsUnitTest {
		t.Skip(fmt.Sprintf(
			"Acceptance tests skipped unless env '%s' set",
			testEnvVar))
		return
	}

	logWriter, err := LogOutput(t)
	if err != nil {
		t.Error(fmt.Errorf("error setting up logging: %s", err))
	}
	log.SetOutput(logWriter)

	// We require verbose mode so that the user knows what is going on.
	if !testing.Verbose() && !c.IsUnitTest {
		t.Fatal("Acceptance tests must be run with the -v flag on tests")
		return
	}

	// Run the PreCheck if we have it
	if c.PreCheck != nil {
		c.PreCheck()
	}

	// // get instances of all providers, so we can use the individual
	// // resources to shim the state during the tests.
	// providers := make(map[string]terraform.ResourceProvider)
	// for name, pf := range testProviderFactories(c) {
	// 	p, err := pf()
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	providers[name] = p
	// }

	providerResolver, err := testProviderResolver(c)
	if err != nil {
		t.Fatal(err)
	}

	opts := terraform.ContextOpts{ProviderResolver: providerResolver}

	// A single state variable to track the lifecycle, starting with no state
	var state *states.State

	// Go through each step and run it
	for i, step := range c.Steps {
		// // insert the providers into the step so we can get the resources for
		// // shimming the state
		// step.providers = providers

		var err error
		log.Printf("[DEBUG] Test: Executing step %d", i)

		if step.Config == "" {
			err = fmt.Errorf(
				"unknown test mode for step. Please see TestStep docs\n\n%#v",
				step)
		} else {
			state, err = testStep(opts, state, step)
		}

		// If we expected an error, but did not get one, fail
		if err == nil && step.ExpectError != nil {
			t.Error(fmt.Sprintf(
				"Step %d, no error received, but expected a match to:\n\n%s\n\n",
				i, step.ExpectError))
			break
		}

		// If there was an error, exit
		if err != nil {
			// Perhaps we expected an error? Check if it matches
			if step.ExpectError != nil {
				if !step.ExpectError.MatchString(err.Error()) {
					t.Error(fmt.Sprintf(
						"Step %d, expected error:\n\n%s\n\nTo match:\n\n%s\n\n",
						i, err, step.ExpectError))
					break
				}
			} else {
				t.Error(fmt.Sprintf("Step %d error: %s", i, detailedErrorMessage(err)))
				break
			}
		}
	}

	// If we have a state, then run the destroy
	if state != nil {
		lastStep := c.Steps[len(c.Steps)-1]
		destroyStep := TestStep{
			Config:                    lastStep.Config,
			Check:                     c.CheckDestroy,
			Destroy:                   true,
			PreventPostDestroyRefresh: c.PreventPostDestroyRefresh,
			//providers:                 providers,
		}

		log.Printf("[WARN] Test: Executing destroy step")
		state, err := testStep(opts, state, destroyStep)
		if err != nil {
			t.Error(fmt.Sprintf(
				"Error destroying resource! WARNING: Dangling resources\n"+
					"may exist. The full state and error is shown below.\n\n"+
					"Error: %s\n\nState: %s",
				err,
				state))
		}
	} else {
		log.Printf("[WARN] Skipping destroy test since there is no state.")
	}
}

func testStep(opts terraform.ContextOpts, state *states.State, step TestStep) (*states.State, error) {
	// if !step.Destroy {
	// 	if err := testStepTaint(state, step); err != nil {
	// 		return state, err
	// 	}
	// }

	cfg, err := testConfig(opts, step)
	if err != nil {
		return state, err
	}

	var stepDiags tfdiags.Diagnostics

	// Build the context
	opts.Config = cfg
	opts.State = state
	if err != nil {
		return nil, err
	}

	opts.Destroy = step.Destroy
	ctx, stepDiags := terraform.NewContext(&opts)
	if stepDiags.HasErrors() {
		return state, fmt.Errorf("Error initializing context: %s", stepDiags.Err())
	}
	if stepDiags := ctx.Validate(); len(stepDiags) > 0 {
		if stepDiags.HasErrors() {
			return state, errwrap.Wrapf("config is invalid: {{err}}", stepDiags.Err())
		}

		log.Printf("[WARN] Config warnings:\n%s", stepDiags)
	}

	// Refresh!
	state, stepDiags = ctx.Refresh()
	if stepDiags.HasErrors() {
		return state, newOperationError("refresh", stepDiags)
	}

	// If this step is a PlanOnly step, skip over this first Plan and subsequent
	// Apply, and use the follow up Plan that checks for perpetual diffs
	if !step.PlanOnly {
		// Plan!
		if p, stepDiags := ctx.Plan(); stepDiags.HasErrors() {
			return state, newOperationError("plan", stepDiags)
		} else {
			log.Printf("[WARN] Test: Step plan: %s", legacyPlanComparisonString(state, p.Changes))
		}

		// We need to keep a copy of the state prior to destroying
		// such that destroy steps can verify their behavior in the check
		// function
		stateBeforeApplication := state.DeepCopy()

		// Apply the diff, creating real resources.
		state, stepDiags = ctx.Apply()
		if stepDiags.HasErrors() {
			return state, newOperationError("apply", stepDiags)
		}

		// Run any configured checks
		if step.Check != nil {
			if step.Destroy {
				if err := step.Check(stateBeforeApplication); err != nil {
					return state, fmt.Errorf("Check failed: %s", err)
				}
			} else {
				if err := step.Check(state); err != nil {
					return state, fmt.Errorf("Check failed: %s", err)
				}
			}
		}
	}

	// Now, verify that Plan is now empty and we don't have a perpetual diff issue
	// We do this with TWO plans. One without a refresh.
	var p *plans.Plan
	if p, stepDiags = ctx.Plan(); stepDiags.HasErrors() {
		return state, newOperationError("follow-up plan", stepDiags)
	}
	if !p.Changes.Empty() {
		if step.ExpectNonEmptyPlan {
			log.Printf("[INFO] Got non-empty plan, as expected:\n\n%s", legacyPlanComparisonString(state, p.Changes))
		} else {
			return state, fmt.Errorf(
				"After applying this step, the plan was not empty:\n\n%s", legacyPlanComparisonString(state, p.Changes))
		}
	}

	// And another after a Refresh.
	if !step.Destroy || (step.Destroy && !step.PreventPostDestroyRefresh) {
		state, stepDiags = ctx.Refresh()
		if stepDiags.HasErrors() {
			return state, newOperationError("follow-up refresh", stepDiags)
		}
	}
	if p, stepDiags = ctx.Plan(); stepDiags.HasErrors() {
		return state, newOperationError("second follow-up refresh", stepDiags)
	}
	empty := p.Changes.Empty()

	// Data resources are tricky because they legitimately get instantiated
	// during refresh so that they will be already populated during the
	// plan walk. Because of this, if we have any data resources in the
	// config we'll end up wanting to destroy them again here. This is
	// acceptable and expected, and we'll treat it as "empty" for the
	// sake of this testing.
	if step.Destroy && !empty {
		empty = true
		for _, change := range p.Changes.Resources {
			if change.Addr.Resource.Resource.Mode != addrs.DataResourceMode {
				empty = false
				break
			}
		}
	}

	if !empty {
		if step.ExpectNonEmptyPlan {
			log.Printf("[INFO] Got non-empty plan, as expected:\n\n%s", legacyPlanComparisonString(state, p.Changes))
		} else {
			return state, fmt.Errorf(
				"After applying this step and refreshing, "+
					"the plan was not empty:\n\n%s", legacyPlanComparisonString(state, p.Changes))
		}
	}

	// Made it here, but expected a non-empty plan, fail!
	if step.ExpectNonEmptyPlan && empty {
		return state, fmt.Errorf("Expected a non-empty plan, but got an empty plan!")
	}

	// Made it here? Good job test step!
	return state, nil
}

func testConfig(opts terraform.ContextOpts, step TestStep) (*configs.Config, error) {
	if step.PreConfig != nil {
		step.PreConfig()
	}

	cfgPath, err := ioutil.TempDir("", "tf-test")
	if err != nil {
		return nil, fmt.Errorf("Error creating temporary directory for config: %s", err)
	}

	// if step.PreventDiskCleanup {
	// 	log.Printf("[INFO] Skipping defer os.RemoveAll call")
	// } else {
	defer os.RemoveAll(cfgPath)
	// }

	// Write the main configuration file
	err = ioutil.WriteFile(filepath.Join(cfgPath, "main.tf"), []byte(step.Config), os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("Error creating temporary file for config: %s", err)
	}

	// Create directory for our child modules, if any.
	modulesDir := filepath.Join(cfgPath, ".modules")
	err = os.Mkdir(modulesDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("Error creating child modules directory: %s", err)
	}

	inst := initwd.NewModuleInstaller(modulesDir, nil)
	_, installDiags := inst.InstallModules(cfgPath, true, initwd.ModuleInstallHooksImpl{})
	if installDiags.HasErrors() {
		return nil, installDiags.Err()
	}

	loader, err := configload.NewLoader(&configload.Config{
		ModulesDir: modulesDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create config loader: %s", err)
	}

	config, configDiags := loader.LoadConfig(cfgPath)
	if configDiags.HasErrors() {
		return nil, configDiags
	}

	return config, nil
}

// testProviderResolver is a helper to build a ResourceProviderResolver
// with pre instantiated ResourceProviders, so that we can reset them for the
// test, while only calling the factory function once.
// Any errors are stored so that they can be returned by the factory in
// terraform to match non-test behavior.
func testProviderResolver(c TestCase) (providers.Resolver, error) {
	newProviders := make(map[string]providers.Factory)

	for k, p := range c.Providers {
		p := p
		newProviders[k] = func() (providers.Interface, error) {
			return grpcTestProvider(p)
		}
	}

	return providers.ResolverFixed(newProviders), nil
}

func grpcTestProvider(p sdk.Provider) (providers.Interface, error) {
	listener := bufconn.Listen(256 * 1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(sdk.LoggingServerInterceptor()))

	grpcProvider := &sdk.GRPCProviderServer{
		Server: sdk.Server{
			Provider: p,
		},
	}

	pb.RegisterProviderServer(grpcServer, grpcProvider)

	go grpcServer.Serve(listener)

	conn, err := grpc.Dial("", grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	var pp tfplugin.GRPCProviderPlugin
	client, _ := pp.GRPCClient(context.Background(), nil, conn)

	grpcClient := client.(*tfplugin.GRPCProvider)
	//grpcClient.TestListener = listener

	return grpcClient, nil
}

const EnvLogPathMask = "TF_LOG_PATH_MASK"

func LogOutput(t TestT) (logOutput io.Writer, err error) {
	t.Helper()

	logOutput = ioutil.Discard

	logLevel := logging.LogLevel()
	if logLevel == "" {
		return
	}

	logOutput = os.Stderr

	if logPath := os.Getenv(logging.EnvLogFile); logPath != "" {
		var err error
		logOutput, err = os.OpenFile(logPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
	}

	if logPathMask := os.Getenv(EnvLogPathMask); logPathMask != "" {
		// Escape special characters which may appear if we have subtests
		testName := strings.Replace(t.Name(), "/", "__", -1)

		logPath := fmt.Sprintf(logPathMask, testName)
		var err error
		logOutput, err = os.OpenFile(logPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
	}

	// This was the default since the beginning
	logOutput = &logutils.LevelFilter{
		Levels:   logging.ValidLevels,
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   logOutput,
	}

	return
}
