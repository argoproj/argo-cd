-- promotionStrategyRef, branch, and sha are immutable. Patching the live object
-- makes the merge patch contain only spec.blockEnvironment. Do not delete the object.
-- https://gitops-promoter.readthedocs.io/en/latest/advanced-usage/rolling-back/

if obj.status == nil or obj.status.activeSha == nil or obj.status.activeSha == "" then
    error("rollback has not completed", 0)
end
if obj.spec == nil then
    obj.spec = {}
end
obj.spec.blockEnvironment = false

local impactedResource = {}
impactedResource.operation = "patch"
impactedResource.resource = obj
local result = {}
result[1] = impactedResource
return result
