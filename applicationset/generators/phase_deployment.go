package generators

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	PhaseDeploymentType = "phaseDeployment"
	StopOnFailureAction = "stop"
	ContinueOnFailureAction = "continue"
	RollbackOnFailureAction = "rollback"
	
	// Hook failure policies
	HookFailurePolicyFail     = "fail"
	HookFailurePolicyIgnore   = "ignore"
	HookFailurePolicyAbort    = "abort"
)

type PhaseDeploymentProcessor struct {
	client client.Client
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
	if generator.DeploymentStrategy == nil || generator.DeploymentStrategy.Type != PhaseDeploymentType {
		return generatedParams, nil
	}

	log.WithField("applicationset", appSet.Name).Info("Processing phase deployment strategy")

	phases := generator.DeploymentStrategy.Phases
	if len(phases) == 0 {
		return generatedParams, fmt.Errorf("phase deployment strategy requires at least one phase")
	}

	currentPhase := p.getCurrentPhase(appSet, generator)
	if currentPhase >= len(phases) {
		return generatedParams, nil
	}

	phase := phases[currentPhase]

	// Run pre-deployment hooks
	if err := p.runPhaseHooks(ctx, appSet, phase.PreHooks, "pre"); err != nil {
		return nil, fmt.Errorf("phase %s pre-hooks failed: %w", phase.Name, err)
	}

	if err := p.runPhaseChecks(ctx, appSet, phase); err != nil {
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
	if err := p.runPhaseHooks(ctx, appSet, phase.PostHooks, "post"); err != nil {
		log.WithError(err).WithField("phase", phase.Name).Error("Post-deployment hooks failed")
		// Post-hook failures don't block deployment but are logged
	}

	if p.shouldAdvanceToNextPhase(appSet, generator, currentPhase, filteredParams) {
		if currentPhase+1 < len(phases) {
			p.setCurrentPhase(appSet, generator, currentPhase+1)
		}
	}

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
	return fmt.Sprintf("applicationset.argoproj.io/phase-%s", genType)
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
	var filteredParams []map[string]any

	// If no targets specified, use all parameters
	if len(phase.Targets) == 0 {
		filteredParams = allParams
	} else {
		// Filter by targets
		for _, param := range allParams {
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
		if percentage < 0 {
			percentage = 0
		} else if percentage > 100 {
			percentage = 100
		}
		
		if percentage < 100 {
			count := (len(filteredParams) * percentage) / 100
			if count < len(filteredParams) && percentage > 0 {
				// Ensure at least 1 application if percentage > 0
				if count == 0 {
					count = 1
				}
				filteredParams = filteredParams[:count]
			}
		}
	}

	// Apply maxUpdate constraint (takes precedence over percentage)
	if phase.MaxUpdate != nil {
		maxUpdate := phase.MaxUpdate.IntValue()
		if maxUpdate > 0 && len(filteredParams) > maxUpdate {
			filteredParams = filteredParams[:maxUpdate]
		}
	}

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
	if len(target.Clusters) > 0 {
		clusterName, ok := param["name"].(string)
		if !ok {
			return false
		}
		for _, cluster := range target.Clusters {
			if cluster == clusterName {
				return true
			}
		}
		return false
	}

	if len(target.Values) > 0 {
		for key, expectedValue := range target.Values {
			if paramValue, exists := param[key]; !exists || fmt.Sprintf("%v", paramValue) != expectedValue {
				return false
			}
		}
		return true
	}

	for _, matchExpr := range target.MatchExpressions {
		if !p.evaluateMatchExpression(param, matchExpr) {
			return false
		}
	}

	return len(target.MatchExpressions) > 0
}

func (p *PhaseDeploymentProcessor) evaluateMatchExpression(param map[string]any, expr argoprojiov1alpha1.ApplicationMatchExpression) bool {
	paramValue, exists := param[expr.Key]
	if !exists {
		return expr.Operator == "DoesNotExist"
	}

	paramStr := fmt.Sprintf("%v", paramValue)

	switch expr.Operator {
	case "In":
		for _, value := range expr.Values {
			if paramStr == value {
				return true
			}
		}
		return false
	case "NotIn":
		for _, value := range expr.Values {
			if paramStr == value {
				return false
			}
		}
		return true
	case "Exists":
		return true
	case "DoesNotExist":
		return false
	default:
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
	phaseCount := (totalCount * percentage) / 100

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

func (p *PhaseDeploymentProcessor) shouldAdvanceToNextPhase(appSet *argoprojiov1alpha1.ApplicationSet, generator *argoprojiov1alpha1.ApplicationSetGenerator, currentPhase int, filteredParams []map[string]any) bool {
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

	log.WithField("phase", phase.Name).WithField("checks", len(phase.Checks)).Info("Running phase checks")

	var failedChecks []string
	var firstError error

	for _, check := range phase.Checks {
		if err := p.runSingleCheck(ctx, appSet, check); err != nil {
			failedChecks = append(failedChecks, check.Name)
			if firstError == nil {
				firstError = err
			}

			log.WithError(err).WithField("check", check.Name).Error("Phase check failed")

			if phase.OnFailure != nil {
				switch phase.OnFailure.Action {
				case StopOnFailureAction:
					return fmt.Errorf("check %s failed and onFailure action is stop: %w", check.Name, err)
				case RollbackOnFailureAction:
					if rollbackErr := p.performRollback(ctx, appSet, phase); rollbackErr != nil {
						log.WithError(rollbackErr).Error("Failed to perform rollback")
					}
					return fmt.Errorf("check %s failed, rollback attempted: %w", check.Name, err)
				case ContinueOnFailureAction:
					log.WithField("check", check.Name).Warn("Check failed but continuing due to onFailure policy")
					continue
				default:
					return fmt.Errorf("check %s failed with unknown onFailure action %s: %w", check.Name, phase.OnFailure.Action, err)
				}
			} else {
				return fmt.Errorf("check %s failed: %w", check.Name, err)
			}
		}
		log.WithField("check", check.Name).Info("Phase check passed")
	}

	if len(failedChecks) > 0 && phase.OnFailure != nil && phase.OnFailure.Action == ContinueOnFailureAction {
		log.WithField("failedChecks", failedChecks).Warn("Some checks failed but proceeding due to onFailure policy")
	}

	if phase.WaitDuration != nil && phase.WaitDuration.Duration > 0 {
		log.WithField("duration", phase.WaitDuration.Duration).Info("Waiting before proceeding to next phase")
		time.Sleep(phase.WaitDuration.Duration)
	}

	return nil
}

func (p *PhaseDeploymentProcessor) performRollback(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, phase argoprojiov1alpha1.GeneratorDeploymentPhase) error {
	log.WithField("phase", phase.Name).Info("Performing rollback due to check failure")
	
	key := fmt.Sprintf("applicationset.argoproj.io/rollback-phase-%s", phase.Name)
	if appSet.Annotations == nil {
		appSet.Annotations = make(map[string]string)
	}
	appSet.Annotations[key] = time.Now().Format(time.RFC3339)
	
	return nil
}

func (p *PhaseDeploymentProcessor) runSingleCheck(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, check argoprojiov1alpha1.GeneratorPhaseCheck) error {
	timeout := 5 * time.Minute
	if check.Timeout != nil {
		timeout = check.Timeout.Duration
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch check.Type {
	case "command":
		return p.runCommandCheck(checkCtx, appSet, check)
	case "http":
		return p.runHTTPCheck(checkCtx, appSet, check)
	default:
		return fmt.Errorf("unsupported check type: %s", check.Type)
	}
}

func (p *PhaseDeploymentProcessor) runCommandCheck(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, check argoprojiov1alpha1.GeneratorPhaseCheck) error {
	if check.Command == nil || len(check.Command.Command) == 0 {
		return fmt.Errorf("command check requires command field")
	}

	cmdName := check.Command.Command[0]
	cmdArgs := check.Command.Command[1:]

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	env := os.Environ()
	if check.Command.Env != nil {
		for key, value := range check.Command.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	env = append(env, fmt.Sprintf("APPSET_NAME=%s", appSet.Name))
	env = append(env, fmt.Sprintf("APPSET_NAMESPACE=%s", appSet.Namespace))

	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	log.WithField("check", check.Name).WithField("output", string(output)).Debug("Command check completed successfully")
	return nil
}

func (p *PhaseDeploymentProcessor) runPhaseHooks(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hooks []argoprojiov1alpha1.GeneratorPhaseHook, hookType string) error {
	if len(hooks) == 0 {
		return nil
	}

	log.WithField("hookType", hookType).WithField("hooks", len(hooks)).Info("Running phase hooks")

	for _, hook := range hooks {
		if err := p.runSingleHook(ctx, appSet, hook, hookType); err != nil {
			// Handle failure policy
			failurePolicy := hook.FailurePolicy
			if failurePolicy == "" {
				failurePolicy = HookFailurePolicyFail
			}

			switch failurePolicy {
			case HookFailurePolicyIgnore:
				log.WithError(err).WithField("hook", hook.Name).Warn("Hook failed but ignoring due to failure policy")
				continue
			case HookFailurePolicyAbort:
				return fmt.Errorf("hook %s failed and failure policy is abort: %w", hook.Name, err)
			case HookFailurePolicyFail:
				fallthrough
			default:
				return fmt.Errorf("hook %s failed: %w", hook.Name, err)
			}
		}
		log.WithField("hook", hook.Name).WithField("type", hookType).Info("Hook completed successfully")
	}

	return nil
}

func (p *PhaseDeploymentProcessor) runSingleHook(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hook argoprojiov1alpha1.GeneratorPhaseHook, hookType string) error {
	timeout := 5 * time.Minute
	if hook.Timeout != nil {
		timeout = hook.Timeout.Duration
	}

	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch hook.Type {
	case "command":
		return p.runCommandHook(hookCtx, appSet, hook, hookType)
	case "http":
		return p.runHTTPHook(hookCtx, appSet, hook, hookType)
	default:
		return fmt.Errorf("unsupported hook type: %s", hook.Type)
	}
}

func (p *PhaseDeploymentProcessor) runCommandHook(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hook argoprojiov1alpha1.GeneratorPhaseHook, hookType string) error {
	if hook.Command == nil || len(hook.Command.Command) == 0 {
		return fmt.Errorf("command hook requires command field")
	}

	cmdName := hook.Command.Command[0]
	cmdArgs := hook.Command.Command[1:]

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	env := os.Environ()
	if hook.Command.Env != nil {
		for key, value := range hook.Command.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Add hook-specific environment variables
	env = append(env, fmt.Sprintf("APPSET_NAME=%s", appSet.Name))
	env = append(env, fmt.Sprintf("APPSET_NAMESPACE=%s", appSet.Namespace))
	env = append(env, fmt.Sprintf("HOOK_NAME=%s", hook.Name))
	env = append(env, fmt.Sprintf("HOOK_TYPE=%s", hookType))

	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook command failed: %w, output: %s", err, string(output))
	}

	log.WithField("hook", hook.Name).WithField("type", hookType).WithField("output", string(output)).Debug("Command hook completed successfully")
	return nil
}

func (p *PhaseDeploymentProcessor) runHTTPHook(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, hook argoprojiov1alpha1.GeneratorPhaseHook, hookType string) error {
	if hook.HTTP == nil {
		return fmt.Errorf("http hook requires http field")
	}

	httpHook := hook.HTTP
	method := httpHook.Method
	if method == "" {
		method = "POST"
	}

	expectedStatus := httpHook.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: httpHook.InsecureSkipVerify,
			},
		},
	}

	var body io.Reader
	if httpHook.Body != "" {
		body = strings.NewReader(httpHook.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, httpHook.URL, body)
	if err != nil {
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

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("HTTP hook request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	log.WithFields(log.Fields{
		"hook":           hook.Name,
		"hookType":       hookType,
		"url":            httpHook.URL,
		"method":         method,
		"status":         resp.StatusCode,
		"expected":       expectedStatus,
		"duration":       duration,
		"response_size":  len(responseBody),
	}).Debug("HTTP hook completed")

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("HTTP hook failed: expected status %d, got %d. Response: %s", 
			expectedStatus, resp.StatusCode, string(responseBody))
	}

	return nil
}

func GetGeneratorWithPhaseDeployment(generator *argoprojiov1alpha1.ApplicationSetGenerator) bool {
	return generator.DeploymentStrategy != nil && generator.DeploymentStrategy.Type == PhaseDeploymentType
}

func (p *PhaseDeploymentProcessor) GetGeneratorPhaseStatus(appSet *argoprojiov1alpha1.ApplicationSet, generator *argoprojiov1alpha1.ApplicationSetGenerator) (int, int, error) {
	if generator.DeploymentStrategy == nil || generator.DeploymentStrategy.Type != PhaseDeploymentType {
		return 0, 0, fmt.Errorf("generator does not use phase deployment strategy")
	}

	currentPhase := p.getCurrentPhase(appSet, generator)
	totalPhases := len(generator.DeploymentStrategy.Phases)

	return currentPhase, totalPhases, nil
}

func GeneratorPhaseStatusToJSON(currentPhase, totalPhases int) string {
	status := map[string]interface{}{
		"currentPhase": currentPhase,
		"totalPhases":  totalPhases,
		"completed":    currentPhase >= totalPhases,
	}

	jsonBytes, _ := json.Marshal(status)
	return string(jsonBytes)
}