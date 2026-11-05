-- Creates a RestoreActiveCommit for one environment branch.
-- promotionStrategyRef, branch, and sha are immutable, so a different rollback is a new object.
-- https://gitops-promoter.readthedocs.io/en/latest/advanced-usage/rolling-back/

local branch = actionParams["branch"]
local sha = actionParams["sha"]
if branch == nil or branch == "" then
    error("branch is required", 0)
end
if sha == nil or sha == "" then
    error("sha is required", 0)
end

local restore = {}
restore.apiVersion = "promoter.argoproj.io/v1alpha1"
restore.kind = "RestoreActiveCommit"
restore.metadata = {}
restore.metadata.name = "restore-" .. sha
restore.metadata.namespace = obj.metadata.namespace
restore.spec = {}
restore.spec.promotionStrategyRef = {}
restore.spec.promotionStrategyRef.name = obj.metadata.name
restore.spec.branch = branch
restore.spec.sha = sha

local impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = restore
local result = {}
result[1] = impactedResource
return result
