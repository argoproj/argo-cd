local os = require("os")

-- This action constructs a Workflow resource from a CronWorkflow resource, to enable creating a CronWorkflow instance
-- on demand.
-- It returns an array with a single member - a table with the operation to perform (create) and the Workflow resource.
-- It mimics the output of "argo submit --from=CronWorkflow/<CRON_WORKFLOW_NAME>" command, declaratively.

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

workflow = {}
workflow.apiVersion = "argoproj.io/v1alpha1"
workflow.kind = "Workflow"

workflow.metadata = {}
workflow.metadata.name = obj.metadata.name .. "-" ..os.date("!%Y%m%d%H%M")
workflow.metadata.namespace = obj.metadata.namespace

ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
workflow.metadata.ownerReferences = {}
workflow.metadata.ownerReferences[1] = ownerRef

workflow.spec = deepCopy(obj.spec.workflowSpec)

impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = workflow
result = {}
result[1] = impactedResource

return result