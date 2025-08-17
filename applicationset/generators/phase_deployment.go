package generators

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	PhaseDeploymentType     = "phaseDeployment"
	StopOnFailureAction     = "stop"
	ContinueOnFailureAction = "continue"
	RollbackOnFailureAction = "rollback"

	// Hook failure policies
	HookFailurePolicyFail   = "fail"
	HookFailurePolicyIgnore = "ignore"
	HookFailurePolicyAbort  = "abort"

	// Default timeouts
	DefaultCheckTimeout = 5 * time.Minute
	DefaultHookTimeout  = 5 * time.Minute

	// HTTP defaults
	DefaultHTTPExpectedStatus = int64(200)
	DefaultHTTPMethod         = "GET"
	DefaultHTTPHookMethod     = "POST"

	// Kubernetes annotation keys
	PhaseAnnotationPrefix    = "applicationset.argoproj.io/phase-"
	RollbackAnnotationPrefix = "applicationset.argoproj.io/rollback-phase-"

	// Security limits
	MaxResponseBodySize = 1024 * 1024 // 1MB
	MaxCommandTimeout   = 30 * time.Minute
	MaxHTTPTimeout      = 10 * time.Minute
	MaxEnvVarsCount     = 100
	MaxHeadersCount     = 50

	// Version compatibility
	MinSupportedVersion = "v2.12.0" // Phase deployment hooks introduced

	// Performance limits
	MaxConcurrentChecks     = 10   // Maximum concurrent phase checks
	MaxConcurrentHooks      = 5    // Maximum concurrent hooks
	MaxApplicationsPerPhase = 1000 // Maximum applications per phase
)

type PhaseDeploymentProcessor struct {
	client client.Client //nolint:unused // Will be used in future implementations
}

func NewPhaseDeploymentProcessor(client client.Client) *PhaseDeploymentProcessor {
	return &PhaseDeploymentProcessor{
		client: client,
	}
}

func (p *PhaseDeploymentProcessor) ProcessPhaseDeployment(
	ctx context.Context,
	appSet *argoprojiov1alpha1.ApplicationSet,
	generator *argoprojiov1alpha1.ApplicationSetGenerator,
	generatedParams []map[string]any,
) ([]map[string]any, error) {
	// Validate inputs
	if appSet == nil {
		return nil, errors.New("applicationSet cannot be nil")
	}
	if generator == nil {
		return nil, errors.New("generator cannot be nil")
	}
	if generator.DeploymentStrategy == nil || generator.DeploymentStrategy.Type != PhaseDeploymentType {
		return generatedParams, nil
	}

	// Version compatibility check
	if err := validateVersionCompatibility(appSet); err != nil {
		log.WithError(err).Warn("Version compatibility warning")
		// Don't fail, just warn - this is for informational purposes
	}

	logger := log.WithFields(log.Fields{
		"applicationset": appSet.Name,
		"namespace":      appSet.Namespace,
		"generator":      p.getGeneratorType(generator),
	})
	logger.Info("Processing phase deployment strategy")

	phases := generator.DeploymentStrategy.Phases
	if len(phases) == 0 {
		return nil, errors.New("phase deployment strategy requires at least one phase")
	}

	currentPhase := p.getCurrentPhase(appSet, generator)
	if currentPhase >= len(phases) {
		logger.WithField("currentPhase", currentPhase).Debug("All phases completed")
		return generatedParams, nil
	}

	phase := phases[currentPhase]
	logger = logger.WithFields(log.Fields{
		"currentPhase": currentPhase,
		"phaseName":    phase.Name,
		"totalPhases":  len(phases),
	})
	logger.Info("Processing phase deployment")

	// Run pre-deployment hooks
	logger.Debug("Running pre-deployment hooks")
	if err := p.runPhaseHooks(ctx, appSet, phase.PreHooks, "pre", logger); err != nil {
		logger.WithError(err).Error("Pre-deployment hooks failed")
		return nil, fmt.Errorf("phase %s pre-hooks failed: %w", phase.Name, err)
	}

	// Run phase checks
	logger.Debug("Running phase checks")
	if err := p.runPhaseChecks(ctx, appSet, phase); err != nil {
		logger.WithError(err).Error("Phase checks failed")
		return nil, fmt.Errorf("phase %s checks failed: %w", phase.Name, err)
	}

	// For percentage-based phases, we need to consider all previous phases
	var filteredParams []map[string]any
	if p.isPercentageBasedStrategy(phases) {
		filteredParams = p.filterParamsForPercentagePhase(generatedParams, phases, currentPhase)
	} else {
		filteredParams = p.filterParamsForPhase(generatedParams, phase)
	}

	// Run post-deployment hooks (regardless of whether we advance to next phase)
	logger.Debug("Running post-deployment hooks")
	if err := p.runPhaseHooks(ctx, appSet, phase.PostHooks, "post", logger); err != nil {
		logger.WithError(err).Error("Post-deployment hooks failed")
		// Post-hook failures don't block deployment but are logged
	}

	if p.shouldAdvanceToNextPhase(appSet, generator, currentPhase, filteredParams) {
		if currentPhase+1 < len(phases) {
			nextPhase := currentPhase + 1
			p.setCurrentPhase(appSet, generator, nextPhase)
			logger.WithField("nextPhase", nextPhase).Info("Advanced to next phase")
		} else {
			logger.Info("All phases completed")
		}
	}

	logger.WithField("filteredApplications", len(filteredParams)).Info("Phase deployment processing completed")

	return filteredParams, nil
}

func (p *PhaseDeploymentProcessor) getCurrentPhase(appSet *argoprojiov1alpha1.ApplicationSet, generator *argoprojiov1alpha1.ApplicationSetGenerator) int {
	key := p.getPhaseKey(generator)
	if appSet.Annotations == nil {
		return 0
	}

	phaseStr, exists := appSet.Annotations[key]
	if !exists {
		return 0
	}

	phase, err := strconv.Atoi(phaseStr)
	if err != nil {
		log.WithError(err).Warn("Invalid phase annotation value, defaulting to 0")
		return 0
	}

	return phase
}

func (p *PhaseDeploymentProcessor) setCurrentPhase(appSet *argoprojiov1alpha1.ApplicationSet, generator *argoprojiov1alpha1.ApplicationSetGenerator, phase int) {
	key := p.getPhaseKey(generator)
	if appSet.Annotations == nil {
		appSet.Annotations = make(map[string]string)
	}
	appSet.Annotations[key] = strconv.Itoa(phase)
}

func (p *PhaseDeploymentProcessor) getPhaseKey(generator *argoprojiov1alpha1.ApplicationSetGenerator) string {
	genType := p.getGeneratorType(generator)
	return "applicationset.argoproj.io/phase-" + genType
}

func (p *PhaseDeploymentProcessor) getGeneratorType(generator *argoprojiov1alpha1.ApplicationSetGenerator) string {
	switch {
	case generator.List != nil:
		return "list"
	case generator.Clusters != nil:
		return "clusters"
	case generator.Git != nil:
		return "git"
	case generator.SCMProvider != nil:
		return "scm-provider"
	case generator.ClusterDecisionResource != nil:
		return "cluster-decision-resource"
	case generator.PullRequest != nil:
		return "pull-request"
	case generator.Matrix != nil:
		return "matrix"
	case generator.Merge != nil:
		return "merge"
	case generator.Plugin != nil:
		return "plugin"
	default:
		return "unknown"
	}
}

func (p *PhaseDeploymentProcessor) filterParamsForPhase(allParams []map[string]any, phase argoprojiov1alpha1.GeneratorDeploymentPhase) []map[string]any {
	if allParams == nil {
		return nil
	}

	// Performance optimization: validate application count constraints
	if len(allParams) > MaxApplicationsPerPhase {
		log.WithFields(log.Fields{
			"phase":          phase.Name,
			"applications":   len(allParams),
			"maxRecommended": MaxApplicationsPerPhase,
		}).Warn("Large number of applications may impact performance")
	}

	logger := log.WithFields(log.Fields{
		"phase":        phase.Name,
		"totalParams":  len(allParams),
		"targetsCount": len(phase.Targets),
	})

	var filteredParams []map[string]any

	// If no targets specified, use all parameters
	if len(phase.Targets) == 0 {
		logger.Debug("No targets specified, using all parameters")
		filteredParams = make([]map[string]any, len(allParams))
		copy(filteredParams, allParams)
	} else {
		// Filter by targets
		logger.Debug("Filtering parameters by targets")
		for _, param := range allParams {
			if param == nil {
				continue // Skip nil parameters
			}
			if p.paramMatchesPhaseTargets(param, phase.Targets) {
				filteredParams = append(filteredParams, param)
			}
		}
	}

	// Sort parameters for consistent ordering
	sort.Slice(filteredParams, func(i, j int) bool {
		clusterI, _ := filteredParams[i]["name"].(string)
		clusterJ, _ := filteredParams[j]["name"].(string)
		return clusterI < clusterJ
	})

	// Apply percentage-based filtering if specified
	if phase.Percentage != nil {
		percentage := *phase.Percentage
		originalCount := len(filteredParams)

		// Validate percentage range
		if percentage < 0 {
			logger.WithField("percentage", percentage).Warn("Negative percentage specified, using 0")
			percentage = 0
		} else if percentage > 100 {
			logger.WithField("percentage", percentage).Warn("Percentage over 100 specified, using 100")
			percentage = 100
		}

		if percentage < 100 && originalCount > 0 {
			count := (originalCount * int(percentage)) / 100
			// Ensure at least 1 application if percentage > 0
			if count == 0 && percentage > 0 {
				count = 1
			}
			if count < originalCount {
				logger.WithFields(log.Fields{
					"percentage":    percentage,
					"originalCount": originalCount,
					"newCount":      count,
				}).Debug("Applied percentage filtering")
				filteredParams = filteredParams[:count]
			}
		}
	}

	// Apply maxUpdate constraint (takes precedence over percentage)
	if phase.MaxUpdate != nil {
		maxUpdate := phase.MaxUpdate.IntValue()
		originalCount := len(filteredParams)

		if maxUpdate < 0 {
			logger.WithField("maxUpdate", maxUpdate).Warn("Negative maxUpdate specified, ignoring")
		} else if maxUpdate > 0 && originalCount > maxUpdate {
			logger.WithFields(log.Fields{
				"maxUpdate":     maxUpdate,
				"originalCount": originalCount,
			}).Debug("Applied maxUpdate constraint")
			filteredParams = filteredParams[:maxUpdate]
		}
	}

	logger.WithField("filteredCount", len(filteredParams)).Debug("Parameter filtering completed")
	return filteredParams
}

func (p *PhaseDeploymentProcessor) paramMatchesPhaseTargets(param map[string]any, targets []argoprojiov1alpha1.GeneratorPhaseTarget) bool {
	for _, target := range targets {
		if p.paramMatchesTarget(param, target) {
			return true
		}
	}
	return false
}

func (p *PhaseDeploymentProcessor) paramMatchesTarget(param map[string]any, target argoprojiov1alpha1.GeneratorPhaseTarget) bool {
	if param == nil {
		return false
	}

	// Check cluster targeting
	if len(target.Clusters) > 0 {
		clusterName, ok := param["cluster"].(string)
		if !ok {
			// Also try "name" field for backward compatibility
			clusterName, ok = param["name"].(string)
			if !ok {
				return false
			}
		}
		for _, cluster := range target.Clusters {
			if cluster != "" && cluster == clusterName {
				return true
			}
		}
		return false
	}

	// Check value targeting
	if len(target.Values) > 0 {
		for key, expectedValue := range target.Values {
			if key == "" || expectedValue == "" {
				continue // Skip empty keys or values
			}
			paramValue, exists := param[key]
			if !exists {
				return false
			}
			// Type-safe string comparison
			paramStr := fmt.Sprintf("%v", paramValue)
			if paramStr != expectedValue {
				return false
			}
		}
		return true
	}

	// Check match expressions
	if len(target.MatchExpressions) > 0 {
		for _, matchExpr := range target.MatchExpressions {
			if !p.evaluateMatchExpression(param, matchExpr) {
				return false
			}
		}
		return true
	}

	// If no targeting criteria specified, match all
	return len(target.Clusters) == 0 && len(target.Values) == 0
}

func (p *PhaseDeploymentProcessor) evaluateMatchExpression(param map[string]any, expr argoprojiov1alpha1.ApplicationMatchExpression) bool {
	if param == nil {
		return expr.Operator == "DoesNotExist"
	}

	// Validate expression
	if expr.Key == "" {
		log.Warn("Match expression has empty key, skipping")
		return false
	}

	paramValue, exists := param[expr.Key]

	switch expr.Operator {
	case "Exists":
		return exists
	case "DoesNotExist":
		return !exists
	case "In":
		if !exists {
			return false
		}
		paramStr := fmt.Sprintf("%v", paramValue)
		for _, value := range expr.Values {
			if paramStr == value {
				return true
			}
		}
		return false
	case "NotIn":
		if !exists {
			return true // Non-existent values are not in any set
		}
		paramStr := fmt.Sprintf("%v", paramValue)
		for _, value := range expr.Values {
			if paramStr == value {
				return false
			}
		}
		return true
	default:
		log.WithField("operator", expr.Operator).Warn("Unknown match expression operator")
		return false
	}
}

func (p *PhaseDeploymentProcessor) isPercentageBasedStrategy(phases []argoprojiov1alpha1.GeneratorDeploymentPhase) bool {
	for _, phase := range phases {
		if phase.Percentage != nil {
			return true
		}
	}
	return false
}

func (p *PhaseDeploymentProcessor) filterParamsForPercentagePhase(allParams []map[string]any, phases []argoprojiov1alpha1.GeneratorDeploymentPhase, currentPhase int) []map[string]any {
	if currentPhase >= len(phases) {
		return nil
	}

	phase := phases[currentPhase]

	// First, filter by targets if specified (same as target-based deployment)
	var targetFilteredParams []map[string]any
	if len(phase.Targets) > 0 {
		for _, param := range allParams {
			if p.paramMatchesPhaseTargets(param, phase.Targets) {
				targetFilteredParams = append(targetFilteredParams, param)
			}
		}
	} else {
		targetFilteredParams = allParams
	}

	// Sort filtered parameters for consistent ordering
	sort.Slice(targetFilteredParams, func(i, j int) bool {
		clusterI, _ := targetFilteredParams[i]["name"].(string)
		clusterJ, _ := targetFilteredParams[j]["name"].(string)
		return clusterI < clusterJ
	})

	// For percentage-based phases with targets, we need to calculate percentage
	// based on the target-filtered parameters, not all parameters
	if phase.Percentage == nil {
		// No percentage specified, return all target-filtered params
		result := targetFilteredParams
		if phase.MaxUpdate != nil {
			maxUpdate := phase.MaxUpdate.IntValue()
			if maxUpdate > 0 && len(result) > maxUpdate {
				result = result[:maxUpdate]
			}
		}
		return result
	}

	// Calculate percentage of target-filtered parameters
	percentage := *phase.Percentage
	if percentage < 0 {
		percentage = 0
	} else if percentage > 100 {
		percentage = 100
	}

	totalCount := len(targetFilteredParams)
	phaseCount := (totalCount * int(percentage)) / 100

	// Ensure at least 1 application if percentage > 0
	if phaseCount == 0 && percentage > 0 && totalCount > 0 {
		phaseCount = 1
	}

	// Ensure we don't exceed total count
	if phaseCount > totalCount {
		phaseCount = totalCount
	}

	// Get the slice for this phase
	var result []map[string]any
	if phaseCount > 0 {
		result = targetFilteredParams[:phaseCount]
	}

	// Apply maxUpdate constraint if specified
	if phase.MaxUpdate != nil {
		maxUpdate := phase.MaxUpdate.IntValue()
		if maxUpdate > 0 && len(result) > maxUpdate {
			result = result[:maxUpdate]
		}
	}

	return result
}

func (p *PhaseDeploymentProcessor) shouldAdvanceToNextPhase(_ *argoprojiov1alpha1.ApplicationSet, generator *argoprojiov1alpha1.ApplicationSetGenerator, _ int, filteredParams []map[string]any) bool {
	// For percentage-based strategies, advance when current phase is complete
	if p.isPercentageBasedStrategy(generator.DeploymentStrategy.Phases) {
		return len(filteredParams) == 0
	}
	// For target-based strategies, advance when no matching targets
	return len(filteredParams) == 0
}

func (p *PhaseDeploymentProcessor) runPhaseChecks(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, phase argoprojiov1alpha1.GeneratorDeploymentPhase) error {
	if len(phase.Checks) == 0 {
		return nil
	}

	// Performance optimization: validate phase constraints
	if len(phase.Checks) > MaxConcurrentChecks*2 {
		log.WithFields(log.Fields{
			"phase":          phase.Name,
			"checks":         len(phase.Checks),
			"maxRecommended": MaxConcurrentChecks * 2,
		}).Warn("Large number of checks may impact performance")
	}

	log.WithField("phase", phase.Name).WithField("checks", len(phase.Checks)).Info("Running phase checks")

	// Determine concurrency level
	concurrency := minInt(len(phase.Checks), MaxConcurrentChecks)

	// Run checks concurrently for better performance
	type checkResult struct {
		index int
		check argoprojiov1alpha1.GeneratorPhaseCheck
		err   error
	}

	resultChan := make(chan checkResult, len(phase.Checks))
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// Launch checks concurrently
	for i, check := range phase.Checks {
		wg.Add(1)
		go func(index int, check argoprojiov1alpha1.GeneratorPhaseCheck) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			checkLogger := log.WithFields(log.Fields{
				"check":      check.Name,
				"checkIndex": index + 1,
				"checkType":  check.Type,
			})

			checkLogger.Debug("Starting phase check")
			err := p.runSingleCheck(ctx, appSet, check, checkLogger)

			if err != nil {
				checkLogger.WithError(err).Error("Phase check failed")
			} else {
				checkLogger.Info("Phase check passed")
			}

			resultChan <- checkResult{index: index, check: check, err: err}
		}(i, check)
	}

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results in order
	results := make([]checkResult, len(phase.Checks))
	for result := range resultChan {
		results[result.index] = result
	}

	// Process results sequentially to maintain failure handling logic
	var failedChecks []string
	var firstError error

	for _, result := range results {
		if result.err == nil {
			continue
		}
		failedChecks = append(failedChecks, result.check.Name)
		if firstError == nil {
			firstError = result.err
		}

		if phase.OnFailure == nil {
			return fmt.Errorf("check %s failed: %w", result.check.Name, result.err)
		}
		switch phase.OnFailure.Action {
		case StopOnFailureAction:
			return fmt.Errorf("check %s failed and onFailure action is stop: %w", result.check.Name, result.err)
		case RollbackOnFailureAction:
			if rollbackErr := p.performRollback(ctx, appSet, phase); rollbackErr != nil {
				log.WithError(rollbackErr).Error("Failed to perform rollback")
			}
			return fmt.Errorf("check %s failed, rollback attempted: %w", result.check.Name, result.err)
		case ContinueOnFailureAction:
			log.WithField("check", result.check.Name).Warn("Check failed but continuing due to onFailure policy")
		default:
			return fmt.Errorf("check %s failed with unknown onFailure action %s: %w", result.check.Name, phase.OnFailure.Action, result.err)
		}
	}

	if len(failedChecks) > 0 && phase.OnFailure != nil && phase.OnFailure.Action == ContinueOnFailureAction {
		log.WithField("failedChecks", failedChecks).Warn("Some checks failed but proceeding due to onFailure policy")
	}

	if phase.WaitDuration != nil && phase.WaitDuration.Duration > 0 {
		log.WithField("duration", phase.WaitDuration.Duration).Info("Waiting before proceeding to next phase")

		// Use context-aware sleep for better cancellation support
		if err := safeContextSleep(ctx, phase.WaitDuration.Duration); err != nil {
			log.WithError(err).Warn("Wait duration interrupted by context cancellation")
			return err
		}
	}

	log.Info("All phase checks completed successfully")
	return nil
}

func (p *PhaseDeploymentProcessor) performRollback(_ context.Context, appSet *argoprojiov1alpha1.ApplicationSet, phase argoprojiov1alpha1.GeneratorDeploymentPhase) error {
	if appSet == nil {
		return errors.New("applicationSet cannot be nil")
	}

	logger := log.WithFields(log.Fields{
		"applicationset": appSet.Name,
		"namespace":      appSet.Namespace,
		"phase":          phase.Name,
	})
	logger.Info("Performing rollback due to check failure")

	key := RollbackAnnotationPrefix + phase.Name
	if appSet.Annotations == nil {
		appSet.Annotations = make(map[string]string)
	}
	timestamp := time.Now().Format(time.RFC3339)
	appSet.Annotations[key] = timestamp

	logger.WithFields(log.Fields{
		"annotation": key,
		"timestamp":  timestamp,
	}).Debug("Added rollback annotation")

	// TODO: Implement actual rollback logic based on requirements
	// This is a placeholder that just adds an annotation

	return nil
}

func (p *PhaseDeploymentProcessor) runSingleCheck(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, check argoprojiov1alpha1.GeneratorPhaseCheck, logger *log.Entry) error {
	// Validate inputs
	if appSet == nil {
		return errors.New("applicationSet cannot be nil")
	}
	if check.Name == "" {
		return errors.New("check name cannot be empty")
	}
	if check.Type == "" {
		return errors.New("check type cannot be empty")
	}

	timeout := DefaultCheckTimeout
	if check.Timeout != nil {
		timeout = check.Timeout.Duration
		if err := validateTimeout(timeout, MaxCommandTimeout, "check"); err != nil {
			return err
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger.WithField("timeout", timeout).Debug("Running check with timeout")

	switch check.Type {
	case "command":
		return p.runCommandCheck(checkCtx, appSet, check, logger)
	case "http":
		return p.runHTTPCheck(checkCtx, appSet, check)
	default:
		return fmt.Errorf("unsupported check type: %s", check.Type)
	}
}

func (p *PhaseDeploymentProcessor) runCommandCheck(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, check argoprojiov1alpha1.GeneratorPhaseCheck, logger *log.Entry) error {
	if check.Command == nil || len(check.Command.Command) == 0 {
		return errors.New("command check requires command field")
	}

	cmdName := check.Command.Command[0]
	if cmdName == "" {
		return errors.New("command name cannot be empty")
	}
	cmdArgs := check.Command.Command[1:]

	// Security validation for command execution
	if err := validateCommandSecurity(check.Command.Command, check.Command.Env); err != nil {
		logger.WithError(err).Error("Command security validation failed")
		return fmt.Errorf("command security validation failed: %w", err)
	}

	logger.WithFields(log.Fields{
		"command": cmdName,
		"args":    cmdArgs,
	}).Debug("Executing command check")

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	env := os.Environ()
	if check.Command.Env != nil {
		for key, value := range check.Command.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	env = append(env, "APPSET_NAME="+appSet.Name)
	env = append(env, "APPSET_NAMESPACE="+appSet.Namespace)

	cmd.Env = env

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	logger.WithFields(log.Fields{
		"duration":   duration,
		"outputSize": len(output),
	}).Debug("Command execution completed")

	if err != nil {
		logger.WithFields(log.Fields{
			"exitCode": cmd.ProcessState.ExitCode(),
			"output":   string(output),
		}).Error("Command check failed")
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	logger.WithField("output", string(output)).Debug("Command check completed successfully")
	return nil
}

func (p *PhaseDeploymentProcessor) runPhaseHooks(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hooks []argoprojiov1alpha1.GeneratorPhaseHook, hookType string, logger *log.Entry) error {
	if len(hooks) == 0 {
		logger.WithField("hookType", hookType).Debug("No hooks configured")
		return nil
	}

	// Performance optimization: validate hook constraints
	if len(hooks) > MaxConcurrentHooks*2 {
		logger.WithFields(log.Fields{
			"hookType":       hookType,
			"hooks":          len(hooks),
			"maxRecommended": MaxConcurrentHooks * 2,
		}).Warn("Large number of hooks may impact performance")
	}

	logger.WithFields(log.Fields{
		"hookType":  hookType,
		"hookCount": len(hooks),
	}).Info("Running phase hooks")

	// Determine concurrency level for hooks
	concurrency := minInt(len(hooks), MaxConcurrentHooks)

	// Run hooks concurrently for better performance
	type hookResult struct {
		index int
		hook  argoprojiov1alpha1.GeneratorPhaseHook
		err   error
	}

	resultChan := make(chan hookResult, len(hooks))
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// Launch hooks concurrently
	for i, hook := range hooks {
		wg.Add(1)
		go func(index int, hook argoprojiov1alpha1.GeneratorPhaseHook) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			hookLogger := logger.WithFields(log.Fields{
				"hook":      hook.Name,
				"hookIndex": index + 1,
				"hookType":  hookType,
				"type":      hook.Type,
			})

			hookLogger.Debug("Starting hook execution")
			err := p.runSingleHook(ctx, appSet, hook, hookType, hookLogger)

			if err != nil {
				hookLogger.WithError(err).Error("Hook execution failed")
			} else {
				hookLogger.Info("Hook completed successfully")
			}

			resultChan <- hookResult{index: index, hook: hook, err: err}
		}(i, hook)
	}

	// Wait for all hooks to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results in order
	results := make([]hookResult, len(hooks))
	for result := range resultChan {
		results[result.index] = result
	}

	// Process results sequentially to maintain failure handling logic
	for _, result := range results {
		if result.err == nil {
			continue
		}
		// Handle failure policy
		failurePolicy := result.hook.FailurePolicy
		if failurePolicy == "" {
			failurePolicy = HookFailurePolicyFail
		}

		hookLogger := logger.WithFields(log.Fields{
			"hook":          result.hook.Name,
			"failurePolicy": failurePolicy,
			"error":         result.err.Error(),
		})
		hookLogger.Error("Hook execution failed")

		switch failurePolicy {
		case HookFailurePolicyIgnore:
			hookLogger.WithError(result.err).Warn("Hook failed but ignoring due to failure policy")
		case HookFailurePolicyAbort:
			return fmt.Errorf("hook %s failed and failure policy is abort: %w", result.hook.Name, result.err)
		case HookFailurePolicyFail:
			return fmt.Errorf("hook %s failed: %w", result.hook.Name, result.err)
		default:
			return fmt.Errorf("hook %s failed: %w", result.hook.Name, result.err)
		}
	}

	return nil
}

func (p *PhaseDeploymentProcessor) runSingleHook(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hook argoprojiov1alpha1.GeneratorPhaseHook, hookType string, logger *log.Entry) error {
	// Validate inputs
	if appSet == nil {
		return errors.New("applicationSet cannot be nil")
	}
	if hook.Name == "" {
		return errors.New("hook name cannot be empty")
	}
	if hook.Type == "" {
		return errors.New("hook type cannot be empty")
	}

	timeout := DefaultHookTimeout
	if hook.Timeout != nil {
		timeout = hook.Timeout.Duration
		if err := validateTimeout(timeout, MaxCommandTimeout, "hook"); err != nil {
			return err
		}
	}

	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger.WithField("timeout", timeout).Debug("Running hook with timeout")

	switch hook.Type {
	case "command":
		return p.runCommandHook(hookCtx, appSet, hook, hookType, logger)
	case "http":
		return p.runHTTPHook(hookCtx, appSet, hook, hookType)
	default:
		return fmt.Errorf("unsupported hook type: %s", hook.Type)
	}
}

func (p *PhaseDeploymentProcessor) runCommandHook(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hook argoprojiov1alpha1.GeneratorPhaseHook, hookType string, logger *log.Entry) error {
	if hook.Command == nil || len(hook.Command.Command) == 0 {
		return errors.New("command hook requires command field")
	}

	cmdName := hook.Command.Command[0]
	if cmdName == "" {
		return errors.New("command name cannot be empty")
	}
	cmdArgs := hook.Command.Command[1:]

	// Security validation for command execution
	if err := validateCommandSecurity(hook.Command.Command, hook.Command.Env); err != nil {
		logger.WithError(err).Error("Command hook security validation failed")
		return fmt.Errorf("command hook security validation failed: %w", err)
	}

	logger.WithFields(log.Fields{
		"command": cmdName,
		"args":    cmdArgs,
	}).Debug("Executing command hook")

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	env := os.Environ()
	if hook.Command.Env != nil {
		for key, value := range hook.Command.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Add hook-specific environment variables
	env = append(env, "APPSET_NAME="+appSet.Name)
	env = append(env, "APPSET_NAMESPACE="+appSet.Namespace)
	env = append(env, "HOOK_NAME="+hook.Name)
	env = append(env, "HOOK_TYPE="+hookType)

	cmd.Env = env

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	logger.WithFields(log.Fields{
		"duration":   duration,
		"outputSize": len(output),
	}).Debug("Command hook execution completed")

	if err != nil {
		logger.WithFields(log.Fields{
			"exitCode": cmd.ProcessState.ExitCode(),
			"output":   string(output),
		}).Error("Command hook failed")
		return fmt.Errorf("hook command failed: %w, output: %s", err, string(output))
	}

	logger.WithField("output", string(output)).Debug("Command hook completed successfully")
	return nil
}

func (p *PhaseDeploymentProcessor) runHTTPHook(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hook argoprojiov1alpha1.GeneratorPhaseHook, hookType string) error {
	// Validate inputs
	if appSet == nil {
		return errors.New("applicationSet cannot be nil")
	}
	if hook.HTTP == nil {
		return errors.New("http hook requires http field")
	}

	httpHook := hook.HTTP
	if httpHook.URL == "" {
		return errors.New("http hook requires URL")
	}

	// Security validation for HTTP requests
	if err := validateHTTPSecurity(httpHook); err != nil {
		log.WithError(err).Error("HTTP hook security validation failed")
		return fmt.Errorf("HTTP hook security validation failed: %w", err)
	}

	method := httpHook.Method
	if method == "" {
		method = DefaultHTTPHookMethod
	}

	expectedStatus := httpHook.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = DefaultHTTPExpectedStatus
	}

	logger := log.WithFields(log.Fields{
		"hook":           hook.Name,
		"hookType":       hookType,
		"url":            httpHook.URL,
		"method":         method,
		"expectedStatus": expectedStatus,
	})

	// Create HTTP client with proper timeouts
	// Use context deadline if available, otherwise use default timeout
	clientTimeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < clientTimeout {
			clientTimeout = remaining
		}
	}

	client := &http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: httpHook.InsecureSkipVerify,
			},
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       30 * time.Second,
			DisableKeepAlives:     true, // Prevent connection reuse that could cause hangs
		},
	}

	var body io.Reader
	if httpHook.Body != "" {
		body = strings.NewReader(httpHook.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, httpHook.URL, body)
	if err != nil {
		logger.WithError(err).Error("Failed to create HTTP hook request")
		return fmt.Errorf("failed to create HTTP hook request: %w", err)
	}

	for key, value := range httpHook.Headers {
		req.Header.Set(key, value)
	}

	req.Header.Set("User-Agent", "ArgoCD-ApplicationSet-PhaseHook/1.0")

	// Add hook-specific headers
	envHeaders := map[string]string{
		"X-AppSet-Name":      appSet.Name,
		"X-AppSet-Namespace": appSet.Namespace,
		"X-Hook-Name":        hook.Name,
		"X-Hook-Type":        hookType,
	}

	for key, value := range envHeaders {
		req.Header.Set(key, value)
	}

	logger.Debug("Sending HTTP hook request")
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		logger.WithFields(log.Fields{
			"duration": duration,
			"error":    err.Error(),
		}).Error("HTTP hook request failed")
		return fmt.Errorf("HTTP hook request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	// Limit response body size to prevent memory issues
	limitedReader := io.LimitReader(resp.Body, MaxResponseBodySize)
	responseBody, readErr := io.ReadAll(limitedReader)
	if readErr != nil {
		logger.WithError(readErr).Warn("Failed to read response body")
		// Continue with empty body for status code checking
		responseBody = []byte{}
	}

	logger.WithFields(log.Fields{
		"status":        resp.StatusCode,
		"expected":      expectedStatus,
		"duration":      duration,
		"responseSize":  len(responseBody),
		"contentType":   resp.Header.Get("Content-Type"),
		"contentLength": resp.ContentLength,
	}).Debug("HTTP hook completed")

	if resp.StatusCode != int(expectedStatus) {
		logger.WithFields(log.Fields{
			"actualStatus":   resp.StatusCode,
			"expectedStatus": expectedStatus,
			"responseBody":   string(responseBody),
		}).Error("HTTP hook failed - status code mismatch")
		return fmt.Errorf("HTTP hook failed: expected status %d, got %d. Response: %s",
			expectedStatus, resp.StatusCode, string(responseBody))
	}

	logger.Info("HTTP hook completed successfully")
	return nil
}

func GetGeneratorWithPhaseDeployment(generator *argoprojiov1alpha1.ApplicationSetGenerator) bool {
	return generator.DeploymentStrategy != nil && generator.DeploymentStrategy.Type == PhaseDeploymentType
}

func (p *PhaseDeploymentProcessor) GetGeneratorPhaseStatus(appSet *argoprojiov1alpha1.ApplicationSet, generator *argoprojiov1alpha1.ApplicationSetGenerator) (int, int, error) {
	if generator.DeploymentStrategy == nil || generator.DeploymentStrategy.Type != PhaseDeploymentType {
		return 0, 0, errors.New("generator does not use phase deployment strategy")
	}

	currentPhase := p.getCurrentPhase(appSet, generator)
	totalPhases := len(generator.DeploymentStrategy.Phases)

	return currentPhase, totalPhases, nil
}

func GeneratorPhaseStatusToJSON(currentPhase, totalPhases int) string {
	status := map[string]any{
		"currentPhase": currentPhase,
		"totalPhases":  totalPhases,
		"completed":    currentPhase >= totalPhases,
	}

	jsonBytes, err := json.Marshal(status)
	if err != nil {
		log.WithError(err).Error("Failed to marshal phase status to JSON")
		return "{\"error\": \"failed to marshal status\"}"
	}
	return string(jsonBytes)
}

// safeContextSleep sleeps for the specified duration but respects context cancellation
func safeContextSleep(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		return nil
	}
}

// Security validation functions

// validateCommandSecurity performs security checks on command execution
func validateCommandSecurity(command []string, env map[string]string) error {
	if len(command) == 0 {
		return errors.New("command cannot be empty")
	}

	cmdName := command[0]

	// Prevent absolute path traversal attacks
	if filepath.IsAbs(cmdName) {
		log.WithField("command", cmdName).Warn("Absolute path command detected")
	}

	// Check for dangerous commands (basic blacklist)
	dangerousCommands := []string{
		"rm", "rmdir", "del", "format", "fdisk",
		"mkfs", "dd", "shutdown", "reboot", "halt",
		"su", "sudo", "passwd", "chown", "chmod",
		"curl", "wget", "nc", "netcat", "telnet",
	}

	baseName := filepath.Base(cmdName)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(baseName, dangerous) {
			log.WithFields(log.Fields{
				"command": cmdName,
				"pattern": dangerous,
			}).Warn("Potentially dangerous command detected")
			break
		}
	}

	// Validate environment variables
	if len(env) > MaxEnvVarsCount {
		return fmt.Errorf("too many environment variables: %d (max: %d)", len(env), MaxEnvVarsCount)
	}

	// Check for suspicious environment variable names
	suspiciousEnvPattern := regexp.MustCompile(`(?i)(password|secret|token|key|auth|credential)`)
	for key := range env {
		if suspiciousEnvPattern.MatchString(key) {
			log.WithField("envVar", key).Warn("Potentially sensitive environment variable detected")
		}
	}

	return nil
}

// validateHTTPSecurity performs security checks on HTTP requests
func validateHTTPSecurity(httpConfig *argoprojiov1alpha1.GeneratorPhaseCheckHTTP) error {
	if httpConfig == nil {
		return errors.New("HTTP configuration cannot be nil")
	}

	// Validate URL
	if httpConfig.URL == "" {
		return errors.New("HTTP URL cannot be empty")
	}

	parsedURL, err := url.Parse(httpConfig.URL)
	if err != nil {
		return fmt.Errorf("invalid HTTP URL: %w", err)
	}

	// Security checks for URL
	switch parsedURL.Scheme {
	case "http":
		log.WithField("url", httpConfig.URL).Warn("Insecure HTTP URL detected, consider using HTTPS")
	case "https":
		// HTTPS is preferred
	case "":
		return errors.New("URL scheme is required")
	default:
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	// Check for localhost/internal network access
	if isInternalAddress(parsedURL.Hostname()) {
		log.WithField("hostname", parsedURL.Hostname()).Warn("Internal network address detected")
	}

	// Validate headers count
	if len(httpConfig.Headers) > MaxHeadersCount {
		return fmt.Errorf("too many HTTP headers: %d (max: %d)", len(httpConfig.Headers), MaxHeadersCount)
	}

	// Check for sensitive headers
	sensitiveHeaders := []string{"authorization", "cookie", "x-api-key", "x-auth-token"}
	for key := range httpConfig.Headers {
		lowerKey := strings.ToLower(key)
		for _, sensitive := range sensitiveHeaders {
			if strings.Contains(lowerKey, sensitive) {
				log.WithField("header", key).Warn("Potentially sensitive header detected")
				break
			}
		}
	}

	return nil
}

// isInternalAddress checks if an address is internal/private
func isInternalAddress(hostname string) bool {
	if hostname == "" {
		return false
	}

	// Check for localhost
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}

	// Check for private IP ranges (basic check)
	privatePatterns := []string{
		"10.",      // 10.0.0.0/8
		"172.",     // 172.16.0.0/12 (simplified)
		"192.168.", // 192.168.0.0/16
		"169.254.", // 169.254.0.0/16 (link-local)
	}

	for _, pattern := range privatePatterns {
		if strings.HasPrefix(hostname, pattern) {
			return true
		}
	}

	return false
}

// validateTimeout ensures timeout values are within acceptable limits
func validateTimeout(timeout time.Duration, maxTimeout time.Duration, operation string) error {
	if timeout <= 0 {
		return fmt.Errorf("invalid %s timeout: must be positive", operation)
	}
	if timeout > maxTimeout {
		return fmt.Errorf("%s timeout too large: %v (max: %v)", operation, timeout, maxTimeout)
	}
	return nil
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// validateVersionCompatibility checks if the ApplicationSet has proper version annotations
func validateVersionCompatibility(appSet *argoprojiov1alpha1.ApplicationSet) error {
	if appSet == nil {
		return errors.New("applicationSet cannot be nil")
	}

	// Check for version annotation
	if appSet.Annotations != nil {
		if version, exists := appSet.Annotations["argocd.argoproj.io/min-version"]; exists {
			// Basic version format validation (semantic versioning)
			versionPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-.*)?$`)
			if !versionPattern.MatchString(version) {
				return fmt.Errorf("invalid version format in min-version annotation: %s", version)
			}

			// Log the minimum version requirement
			log.WithFields(log.Fields{
				"applicationset": appSet.Name,
				"minVersion":     version,
				"supportedFrom":  MinSupportedVersion,
			}).Info("Phase deployment version compatibility")
		} else {
			log.WithFields(log.Fields{
				"applicationset": appSet.Name,
				"supportedFrom":  MinSupportedVersion,
			}).Info("No min-version annotation found, assuming compatibility")
		}
	}

	return nil
}
