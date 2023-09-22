local os = require("os")

-- This action constructs a Workflow resource from a CronWorkflow resource, to enable creating a CronWorkflow instance
-- on demand.
-- It returns an array with a single member - a table with the operation to perform (create) and the Workflow resource.
-- It mimics the output of "argo submit --from=CronWorkflow/<CRON_WORKFLOW_NAME>" command, declaratively.

-- This code is written to mimic what the Argo Workflows API server does to create a Workflow from a CronWorkflow.
-- https://github.com/argoproj/argo-workflows/blob/873a58de7dd9dad76d5577b8c4294a58b52849b8/workflow/common/convert.go#L12

-- Deep-copying an object is a ChatGPT generated code.
-- Since empty tables are treated as empty arrays, the resulting k8s resource might be invalid (arrays instead of maps).
-- So empty tables are not cloned to the target object.
function deepCopy(object)
    local lookup_table = {}
    local function _copy(obj)
        if type(obj) ~= "table" then
            return obj
        elseif lookup_table[obj] then
            return lookup_table[obj]
        elseif next(obj) == nil then
            return nil
        else
            local new_table = {}
            lookup_table[obj] = new_table
            for key, value in pairs(obj) do
                new_table[_copy(key)] = _copy(value)
            end
            return setmetatable(new_table, getmetatable(obj))
        end
    end
    return _copy(object)
end

local workflow = {}
workflow.apiVersion = "argoproj.io/v1alpha1"
workflow.kind = "Workflow"

workflow.metadata = {}
workflow.metadata.name = obj.metadata.name .. "-" ..os.date("!%Y%m%d%H%M")
workflow.metadata.namespace = obj.metadata.namespace
workflow.metadata.labels = {}
workflow.metadata.annotations = {}
if (obj.spec.workflowMetadata ~= nil) then
    if (obj.spec.workflowMetadata.labels ~= nil) then
        workflow.metadata.labels = deepCopy(obj.spec.workflowMetadata.labels)
    end
    if (obj.spec.workflowMetadata.annotations ~= nil) then
        workflow.metadata.annotations = deepCopy(obj.spec.workflowMetadata.annotations)
    end
end
workflow.metadata.labels["workflows.argoproj.io/cron-workflow"] = obj.metadata.name
if (obj.metadata.labels ~= nil and obj.metadata.labels["workflows.argoproj.io/controller-instanceid"] ~= nil) then
    workflow.metadata.labels["workflows.argoproj.io/controller-instanceid"] = obj.metadata.labels["workflows.argoproj.io/controller-instanceid"]
end
workflow.metadata.annotations["workflows.argoproj.io/scheduled-time"] = os.date("!%Y-%m-%dT%d:%H:%MZ")

workflow.finalizers = {}
-- add all finalizers from obj.spec.workflowMetadata.finalizers
if (obj.spec.workflowMetadata ~= nil and obj.spec.workflowMetadata.finalizers ~= nil) then
    for i, finalizer in ipairs(obj.spec.workflowMetadata.finalizers) do
        workflow.finalizers[i] = finalizer
    end
end

local ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
workflow.metadata.ownerReferences = {}
workflow.metadata.ownerReferences[1] = ownerRef

workflow.spec = deepCopy(obj.spec.workflowSpec)

local impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = workflow
local result = {}
result[1] = impactedResource

return result
