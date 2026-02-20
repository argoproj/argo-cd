package upgrade

import (
	"fmt"

	"github.com/argoproj/argo-cd/v3/common"
)

type V2V3Check struct{}

func (v *V2V3Check) performChecks(u *Upgrade) (checklist []CheckResult, err error) {
	if !u.configMapExists(common.ArgoCDConfigMapName) {
		checklist = []CheckResult{}
		err = fmt.Errorf(
			"configmap `%s` not found - ensure the context is set to `argocd` namespace",
			common.ArgoCDConfigMapName)
		return checklist, err
	}

	checklist = append(checklist, v.check1(u))
	checklist = append(checklist, v.check2(u))

	return checklist, nil
}

// UPGRADE CHECKLIST 2.14-3.0
// https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/upgrading/2.14-3.0.md

func (v *V2V3Check) check1(u *Upgrade) (checkResult CheckResult) {
	argoConfigMap, _ := u.getConfigMap(common.ArgoCDConfigMapName)
	rbacExists := u.configMapExists(common.ArgoCDRBACConfigMapName)
	rbacConfigMap, _ := u.getConfigMap(common.ArgoCDRBACConfigMapName)

	checkResult = CheckResult{
		title: "Fine-Grained RBAC for application update and delete sub-resources",
		description: `The default behavior of fine-grained policies have changed so they no
longer apply to sub-resources. Prior to v3, policies granting update or delete
to an application also applied to any of its sub-resources.
Starting with v3, the update or delete actions only apply to the application
itself.
In v3, new policies must be defined to set the update/* or delete/* actions 
on an Application's managed resources.
Alternatively, v2 behavior can be preserved globally by setting the
following value in argocd-cm:
    server.rbac.disableApplicationFineGrainedRBACInheritance: "false"`,
	}

	r1 := Rule{
		title: "server.rbac.disableApplicationFineGrainedRBACInheritance value",
	}
	if u.configMapValueEqual(argoConfigMap,
		"server.rbac.disableApplicationFineGrainedRBACInheritance", "false") {
		r1.actions = append(r1.actions, "Found `false`, v2 behavior is set globally. No policy changes required.")
		r1.result = checkPass
		checkResult.rules = append(checkResult.rules, r1)
	} else {
		r1.actions = append(r1.actions, "Not `false`, v3 behavior will apply to application RBAC policies.")
		r1.result = checkInfo
		checkResult.rules = append(checkResult.rules, r1)
	}

	r2 := Rule{
		title: fmt.Sprintf("Check `%s` application update/delete policies", common.ArgoCDRBACConfigMapName),
	}
	if r1.result == checkPass {
		r2.actions = append(r2.actions, "Skipped, disableApplicationFineGrainedRBACInheritance sets v2 behavior.")
		r2.result = checkPass
		checkResult.rules = append(checkResult.rules, r2)
	} else {
		if !rbacExists {
			r2.actions = append(r2.actions, fmt.Sprintf("Skipped, `%s` does not exist", common.ArgoCDConfigMapName))
			r2.result = checkPass
			checkResult.rules = append(checkResult.rules, r2)
		} else {
			policyMatches := u.configMapValueRegex(rbacConfigMap, "policy.csv", `(?m)^.*p,.*applications,\s*(delete|update),.*$`)
			if len(policyMatches) > 0 {
				for _, policy := range policyMatches {
					r2.actions = append(r2.actions, "Review policy: \n        "+policy)
				}
				r2.result = checkWarn
				checkResult.rules = append(checkResult.rules, r2)
			} else {
				r2.actions = append(r2.actions, fmt.Sprintf("`%s` policy.csv has no application update or delete policies", common.ArgoCDConfigMapName))
				r2.result = checkPass
				checkResult.rules = append(checkResult.rules, r2)
			}
		}
	}

	return checkResult
}

func (v *V2V3Check) check2(u *Upgrade) (checkResult CheckResult) {
	argoConfigMap, _ := u.getConfigMap(common.ArgoCDConfigMapName)
	rbacExists := u.configMapExists(common.ArgoCDRBACConfigMapName)
	rbacConfigMap, _ := u.getConfigMap(common.ArgoCDRBACConfigMapName)

	checkResult = CheckResult{
		title: "Logs RBAC enforcement as a first-class RBAC citizen",
		description: fmt.Sprintf(`2.14 introduced logs as a new RBAC resource. In 2.13 and lower,
users with applications, get access automatically got logs access. In 2.14, it
became possible to enable logs RBAC enforcement with a flag in %s:

server.rbac.log.enforce.enable: 'true'

Starting from 3.0, this flag is removed and the logs RBAC is enforced by
default, meaning the logs tab on pod view will not be visible without
granting explicit logs, get permissions to the users/groups/roles requiring it.`, common.ArgoCDConfigMapName),
	}

	r1 := Rule{
		title: fmt.Sprintf("Check deprecated `server.rbac.log.enforce.enable` in `%s`", common.ArgoCDConfigMapName),
	}
	if u.configMapKeyExists(argoConfigMap,
		"rbac.log.enforce.enable") {
		r1.actions = append(r1.actions, fmt.Sprintf("Remove deprecated `rbac.log.enforce.enable` setting from `%s`", common.ArgoCDConfigMapName))
		r1.result = checkFail
		checkResult.rules = append(checkResult.rules, r1)
	} else {
		r1.actions = append(r1.actions, "Skipped, `rbac.log.enforce.enable` not found")
		r1.result = checkPass
		checkResult.rules = append(checkResult.rules, r1)
	}

	r2 := Rule{
		title: fmt.Sprintf("Check `%s` policy.default for a custom role", common.ArgoCDRBACConfigMapName),
	}
	if !rbacExists {
		r2.actions = append(r2.actions, fmt.Sprintf("Skipped, `%s` does not exist", common.ArgoCDConfigMapName))
		r2.result = checkPass
		checkResult.rules = append(checkResult.rules, r2)
	} else {
		policyDefault := rbacConfigMap.Data["policy.default"]
		if policyDefault != "" && policyDefault != "role:readonly" {
			r2.actions = append(r2.actions, fmt.Sprintf(`Quick Remediation (global): add 'logs, get' to the custom 
        'policy.default: %s' role in policy.csv
        Example: 'p, role:%s, logs, get, */*, allow'`, policyDefault, policyDefault))
			r2.result = checkWarn
			checkResult.rules = append(checkResult.rules, r2)
		} else {
			r2.actions = append(r2.actions, "Skipped, 'policy.default' is not a custom role")
			r2.result = checkPass
			checkResult.rules = append(checkResult.rules, r2)
		}
	}

	r3 := Rule{
		title: "Add explicit 'logs, get' policies to application roles",
	}
	r3.actions = append(r3.actions, `Recommended Remediation (per-policy): Explicitly add a 'logs, get'
        policy to every role that has a policy for 
        'applications, get' or 'applications, *' 
        This is the recommended way to maintain the principle of least privilege.
        More info: https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/upgrading/2.3-2.4.md#example-1`)
	policyMatches := u.configMapValueRegex(rbacConfigMap, "policy.csv", `(?m)^.*p,.*(applications, get|applications, \*),.*$`)
	if len(policyMatches) > 0 {
		for _, policy := range policyMatches {
			r3.actions = append(r3.actions, "Review policy: \n        "+policy)
		}
	}
	r3.result = checkWarn
	checkResult.rules = append(checkResult.rules, r3)

	return checkResult
}
